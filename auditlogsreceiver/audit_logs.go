package auditlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/samber/lo"
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
	logger       *zap.Logger
	pollInterval time.Duration
	pageLimit    int
	wg           *sync.WaitGroup
	doneChan     chan bool
	storage      storage.Storage
	rest         *resty.Client
	consumer     consumer.Logs
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

	ctx, cancel := context.WithCancel(ctx)

	t := time.NewTicker(a.pollInterval)
	defer t.Stop()

	for {
		err := a.poll(ctx, cancel)
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

func (a *auditLogsReceiver) poll(ctx context.Context, cancel context.CancelFunc) error {
	// It is OK to have long durations (to - from) as backend will handle it through pagination & page limit.
	data := a.storage.Get()

	fmt.Printf("--> HERE 0.1 %v\n", data)

	// ToDate is present when exporter is restarted in the middle of pagination; ToDate is shifted with every page.
	if data.ToDate == nil {
		data.ToDate = lo.ToPtr(time.Now().UTC())
		data.NextCheckPoint = data.ToDate

		// Saving state as here fromDate and toDate are known.
		err := a.storage.Save(data)
		if err != nil {
			return err
		}

		fmt.Printf("--> HERE 0.2 %v\n", data)
	}

	var queryParams map[string]string
	for {
		if queryParams == nil {
			queryParams = map[string]string{
				"page.limit": strconv.Itoa(a.pageLimit),
				"toDate":     data.ToDate.Format(timestampLayout),
				"fromDate":   data.CheckPoint.Format(timestampLayout),
			}
		}

		fmt.Printf("--> HERE 0.3 %v\n", queryParams)

		resp, err := a.rest.R().
			SetContext(ctx).
			SetQueryParams(queryParams).
			Get("")
		if err != nil {
			return err
		}
		if resp.StatusCode() > 399 {
			switch resp.StatusCode() {
			case 401, 403:
				// Shutdown collector if unable to authenticate to the api.
				cancel()
				err = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
				if err != nil {
					return err
				}
				return fmt.Errorf("invalid api token, response code: %d", resp.StatusCode())
			default:
				a.logger.Warn("unexpected response from audit logs api:", zap.Any("response_code", resp.StatusCode()))
				return fmt.Errorf("got non 200 status code %d", resp.StatusCode())
			}
		}

		auditLogsMap, lastAuditLogTimestamp, err := a.processResponseBody(ctx, resp.Body())
		if err != nil {
			return err
		}

		// Shifting ToDate towards the current check point with every processed page.
		data.ToDate = lastAuditLogTimestamp
		err = a.storage.Save(data)
		if err != nil {
			return err
		}

		c, ok := auditLogsMap["nextCursor"]
		if !ok {
			// Cursor data is not provided, so it is the last page.
			break
		}

		// Creating query parameters based on cursor as there is more data to be fetched.
		cursor, ok := c.(string)
		if !ok {
			a.logger.Warn("invalid cursor type is returned, skipping")
			break
		}
		if cursor == "" {
			break
		}

		queryParams = map[string]string{
			"page.limit":  strconv.Itoa(a.pageLimit),
			"page.cursor": cursor,
		}
	}

	// Storing state about Audit Logs export position.
	data.CheckPoint = *data.NextCheckPoint
	data.ToDate = nil
	data.NextCheckPoint = nil
	err := a.storage.Save(data)
	if err != nil {
		return err
	}

	return nil
}

func (a *auditLogsReceiver) processResponseBody(ctx context.Context, body []byte) (map[string]interface{}, *time.Time, error) {
	var auditLogsMap map[string]interface{}
	err := json.Unmarshal(body, &auditLogsMap)
	if err != nil {
		return nil, nil, fmt.Errorf("unexpected body in response: %v", body)
	}

	lastAuditLogTimestamp, err := a.processAuditLogs(ctx, auditLogsMap)
	if err != nil {
		return nil, nil, fmt.Errorf("processing audit logs items: %w", err)
	}

	return auditLogsMap, lastAuditLogTimestamp, nil
}

func (a *auditLogsReceiver) processAuditLogs(ctx context.Context, auditLogsMap map[string]interface{}) (lastAuditLogTimestamp *time.Time, err error) {
	its, ok := auditLogsMap["items"]
	if !ok {
		a.logger.Warn("no audit logs items found in the response, skipping", zap.Any("response", auditLogsMap))
		return
	}

	items, ok := its.([]interface{})
	if !ok {
		a.logger.Warn("invalid items type in the response, skipping", zap.Any("items", its))
		return
	}

	logs := plog.NewLogs()
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

		// It may fail due to an invalid type used in attributesMap; in that case, nothing can be done so entry is skipped.
		err = logRecord.Attributes().FromRaw(attributesMap)
		if err != nil {
			return nil, err
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
		lastAuditLogTimestamp = &auditLogTimestamp

		observedTime := pcommon.NewTimestampFromTime(time.Now().UTC())
		logRecord.SetObservedTimestamp(observedTime)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(auditLogTimestamp))
	}

	if logs.LogRecordCount() > 0 {
		if err = a.consumer.ConsumeLogs(ctx, logs); err != nil {
			return nil, fmt.Errorf("consuming logs: %w", err)
		}
	}

	return
}
