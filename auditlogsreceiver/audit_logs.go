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
	// Important note: backend prefers timestamps in UTC, hence the layout; in case
	// this format is applied for timestamps based on time.Now() then UTC location must be
	// specified (for example: tm := time.Now().UTC().Format(timestampLayout))
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
		err := a.poll(ctx, func() {
			// Stop function is called in case of critical errors (error that cannot be restored from).
			cancel()

			// TODO: reconsider this approach based on Open Telemetry practices.
			err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
			if err != nil {
				a.logger.Error("sending sigterm signal", zap.Error(err))
			}
		})
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

func (a *auditLogsReceiver) poll(ctx context.Context, stopFunc func()) error {
	// It is OK to have long durations (to - from) as backend will handle it through pagination & page limit.
	pollData := a.storage.Get()

	// ToDate is present when exporter is restarted in the middle of pagination; ToDate is shifted with every page.
	if pollData.ToDate == nil {
		pollData.ToDate = lo.ToPtr(time.Now())
		pollData.NextCheckPoint = pollData.ToDate

		// Saving state, as fromDate and toDate are fixed from now on.
		err := a.storage.Save(pollData)
		if err != nil {
			return err
		}
	}

	// Logging polling data, which is helpful for debugging.
	a.logger.Debug("polling for audit logs", zap.Any("poll_data", pollData))

	var queryParams map[string]string
	for {
		if queryParams == nil {
			queryParams = map[string]string{
				"page.limit": strconv.Itoa(a.pageLimit),
				"toDate":     pollData.ToDate.UTC().Format(timestampLayout),
				"fromDate":   pollData.CheckPoint.UTC().Format(timestampLayout),
			}
		}

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
				// Authentication error is treated as critical error hence calling a stop function.
				stopFunc()
				return fmt.Errorf("invalid api access key, response code: %d", resp.StatusCode())
			default:
				a.logger.Warn("unexpected response from audit logs api:", zap.Any("response_code", resp.StatusCode()))
				return fmt.Errorf("got non 200 status code %d", resp.StatusCode())
			}
		}

		auditLogsMap, lastAuditLogTimestamp, err := a.processResponseBody(ctx, resp.Body())
		if err != nil {
			return err
		}

		// if lastAuditLogTimestamp is not returned, then there were no valid items found in the response
		if lastAuditLogTimestamp == nil {
			break
		}

		// Shifting ToDate towards the current check point with every processed page.
		pollData.ToDate = lastAuditLogTimestamp
		err = a.storage.Save(pollData)
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
	pollData.CheckPoint = *pollData.NextCheckPoint
	pollData.ToDate = nil
	pollData.NextCheckPoint = nil
	err := a.storage.Save(pollData)
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

		// Dumping content of the Audit Logs to the console.
		a.logger.Info("processing new audit log", zap.Any("data", item))

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

		observedTime := pcommon.NewTimestampFromTime(time.Now())
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
