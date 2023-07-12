package auditlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/castai/otel-receivers/audit-logs/storage"
)

const (
	timestampLayout = "2006-01-02T15:04:05.999999Z"
)

type auditLogsReceiver struct {
	logger        *zap.Logger
	pollInterval  time.Duration
	pageLimit     int
	nextStartTime time.Time
	wg            *sync.WaitGroup
	doneChan      chan bool
	store         storage.Store
	rest          *resty.Client
	consumer      consumer.Logs
}

func (a *auditLogsReceiver) Start(ctx context.Context, _ component.Host) error {
	a.logger.Debug("starting audit logs receiver")
	a.wg.Add(1)
	go a.startPolling(ctx)

	return nil
}

func (a *auditLogsReceiver) Shutdown(_ context.Context) error {
	a.logger.Debug("shutting down audit logs receiver")
	close(a.doneChan)
	a.wg.Wait()

	return nil
}

func (a *auditLogsReceiver) startPolling(ctx context.Context) {
	defer a.wg.Done()

	t := time.NewTicker(a.pollInterval)
	defer t.Stop()

	for {
		err := a.poll(ctx)
		if err != nil {
			a.logger.Error("there was an error during the poll", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-a.doneChan:
			return
		case <-t.C:
			continue
		}
	}
}

func (a *auditLogsReceiver) poll(ctx context.Context) error {
	// It is OK to have long durations (to - from) as backend will handle it through pagination & page limit.
	fromDate := a.store.GetFromDate()
	toDate := time.Now()

	var queryParams map[string]string
	for true {
		if queryParams == nil {
			queryParams = map[string]string{
				"page.limit": strconv.Itoa(a.pageLimit),
				"fromDate":   fromDate.Format(timestampLayout),
				"toDate":     toDate.Format(timestampLayout),
			}
		}

		resp, err := a.rest.R().
			SetContext(ctx).
			SetQueryParams(queryParams).
			Get("")
		if err != nil {
			return err
		}
		if resp.StatusCode()/100 != 2 { // nolint:gomnd
			return fmt.Errorf("unexpected response from audit logs api: code=%d, payload='%v'", resp.StatusCode(), string(resp.Body()))
		}

		auditLogsMap, err := a.processResponseBody(ctx, resp.Body(), toDate)
		c, ok := auditLogsMap["nextCursor"]
		if !ok {
			// Cursor data is not provided, so it is the last page.
			break
		}

		// Creating query parameters based on cursor as there is more data to be fetched.
		cursor, ok := c.(string)
		if !ok {
			a.logger.Warn("invalid empty cursor type is returned, skipping")
			break
		}
		if cursor == "" {
			a.logger.Warn("empty cursor is returned, skipping")
			break
		}

		queryParams = map[string]string{
			"page.limit":  strconv.Itoa(a.pageLimit),
			"page.cursor": cursor,
		}
	}

	return nil
}

func (a *auditLogsReceiver) processResponseBody(ctx context.Context, body []byte, toDate time.Time) (map[string]interface{}, error) {
	var auditLogsMap map[string]interface{}
	err := json.Unmarshal(body, &auditLogsMap)
	if err != nil {
		return nil, fmt.Errorf("unexpected body in response: %v", body)
	}

	err = a.processAuditLogs(ctx, auditLogsMap, toDate)
	if err != nil {
		return nil, fmt.Errorf("processing audit logs items: %w", err)
	}

	return auditLogsMap, nil
}

func (a *auditLogsReceiver) processAuditLogs(ctx context.Context, auditLogsMap map[string]interface{}, toDate time.Time) (err error) {
	logs := plog.NewLogs()
	fromDate := a.store.GetFromDate()
	defer func() {
		if err == nil {
			a.store.PutFromDate(toDate)
		}
	}()
	if len(auditLogsMap) == 0 {
		// TODO: test edges
		a.store.PutFromDate(fromDate.Add(a.pollInterval))
		return nil
	}

	its, ok := auditLogsMap["items"]
	if !ok {
		a.logger.Warn("no audit logs items found in the response, skipping", zap.Any("response", auditLogsMap))
		return nil
	}

	items, ok := its.([]interface{})
	if !ok {
		a.logger.Warn("invalid items type in the response, skipping", zap.Any("items", its))
		return nil
	}

	for _, it := range items {
		item, ok := it.(map[string]interface{})
		if !ok {
			a.logger.Warn("invalid item type among items, skipping", zap.Any("item", it))
			continue
		}

		// Dumping content of the Audit Logs requires setting logger DEBUG level in collector configuration.
		a.logger.Debug("processing new audit log", zap.Any("data", item))

		attributesMap := map[string]interface{}{
			"id":          item["id"],
			"eventType":   item["eventType"],
			"initiatedBy": item["initiatedBy"],
			"labels":      item["labels"],
			"event":       item["event"],
		}

		resourceLog := logs.ResourceLogs().AppendEmpty()
		logRecord := resourceLog.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		// It may fail due to invalid type used in attributesMap; in that case nothing can be done so entry is skipped.
		err = logRecord.Attributes().FromRaw(attributesMap)
		if err != nil {
			return err
		}

		str, ok := item["time"].(string)
		if !ok {
			a.logger.Warn("invalid item's time type, skipping", zap.Any("time", str))
			continue
		}

		var auditLogTimestamp time.Time
		auditLogTimestamp, err = time.Parse(timestampLayout, str)
		if err != nil {
			a.logger.Warn("item's time was not recognized, skipping", zap.Any("time", str), zap.Error(err))
			continue
		}

		observedTime := pcommon.NewTimestampFromTime(time.Now())
		logRecord.SetObservedTimestamp(observedTime)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(auditLogTimestamp))
	}

	if logs.LogRecordCount() > 0 {
		if err = a.consumer.ConsumeLogs(ctx, logs); err != nil {
			return fmt.Errorf("consuming logs: %w", err)
		}
	}

	return nil
}
