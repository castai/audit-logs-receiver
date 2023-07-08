package auditlogs

import (
	"context"
	"encoding/json"
	"fmt"
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

type auditLogsReceiver struct {
	logger        *zap.Logger
	pollInterval  time.Duration
	nextStartTime time.Time
	wg            *sync.WaitGroup
	doneChan      chan bool
	store         storage.Store
	rest          *resty.Client
	consumer      consumer.Logs
}

func (a *auditLogsReceiver) Start(ctx context.Context, host component.Host) error {
	fmt.Printf("--> HERE 1.1\n")

	a.logger.Debug("starting audit logs receiver")
	a.wg.Add(1)
	go a.startPolling(ctx)

	return nil
}

func (a *auditLogsReceiver) Shutdown(ctx context.Context) error {
	fmt.Printf("--> HERE 1.2\n")

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
	// TODO: a.store should be used for handling timestamps.
	queryParams := map[string]string{
		"page.limit": "10",
		"fromDate":   "2023-05-07T13:42:08.654Z",
		"toDate":     "2023-07-08T13:42:08.654Z",
	}
	resp, err := a.rest.R().
		SetContext(ctx).
		SetQueryParams(queryParams).
		Get("")
	if err != nil {
		return err
	}
	if resp.StatusCode()/100 != 2 { // nolint:gomnd
		return fmt.Errorf("unexpected response from audit logs api: code=%d, payload=%v", resp.StatusCode(), string(resp.Body()))
	}

	auditLogsMap, err := a.processResponseBody(ctx, resp.Body())
	c, ok := auditLogsMap["cursor"]
	if !ok {
		// Cursor data is not provided so no next page.
		return nil
	}

	// TODO: WIP - handle pagination
	cursor, ok := c.(string)
	if !ok {
	}
	if cursor != "" {
	}

	return err
}

func (a *auditLogsReceiver) processResponseBody(ctx context.Context, body []byte) (map[string]interface{}, error) {
	var auditLogsMap map[string]interface{}
	err := json.Unmarshal(body, &auditLogsMap)
	if err != nil {
		return nil, fmt.Errorf("unexpected body in response: %v", body)
	}

	logs, err := a.processAuditLogs(auditLogsMap)
	if err != nil {
		return nil, fmt.Errorf("processing audit logs items: %w", err)
	}

	if logs.LogRecordCount() > 0 {
		if err = a.consumer.ConsumeLogs(ctx, logs); err != nil {
			return nil, fmt.Errorf("consuming logs: %w", err)
		}
	}

	return auditLogsMap, nil
}

func (a *auditLogsReceiver) processAuditLogs(auditLogsMap map[string]interface{}) (plog.Logs, error) {
	logs := plog.NewLogs()
	if len(auditLogsMap) == 0 {
		return logs, nil
	}

	its, ok := auditLogsMap["items"]
	if !ok {
		a.logger.Warn("no audit logs items found, response is skipped")
		return logs, nil
	}

	items, ok := its.([]interface{})
	if !ok {
		return logs, fmt.Errorf("invalid response from audit logs api: %v", auditLogsMap)
	}

	for _, it := range items {
		item, ok := it.(map[string]interface{})
		if !ok {
			a.logger.Warn("no audit logs item found, item is skipped")
			continue
		}

		fmt.Printf("--> HERE 2.1 %v\n", item)

		attributesMap := map[string]interface{}{
			"id":          item["id"],
			"eventType":   item["eventType"],
			"initiatedBy": item["initiatedBy"],
			"labels":      item["labels"],
			"event":       item["event"],
		}

		resourceLog := logs.ResourceLogs().AppendEmpty()
		logRecord := resourceLog.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		err := logRecord.Attributes().FromRaw(attributesMap)
		if err != nil {
			return logs, err
		}

		str, ok := item["time"].(string)
		if !ok {
			a.logger.Error("--> HERE 2.2 ", zap.Error(err))
		}

		layout := "2006-01-02T15:04:05.999999Z"
		auditLogTimestamp, err := time.Parse(layout, str)
		if err != nil {
			a.logger.Error("--> HERE 2.3 ", zap.Error(err))
		}

		observedTime := pcommon.NewTimestampFromTime(time.Now())
		logRecord.SetObservedTimestamp(observedTime)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(auditLogTimestamp))
	}

	return logs, nil
}
