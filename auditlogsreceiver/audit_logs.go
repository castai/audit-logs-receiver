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
)

type auditLogsReceiver struct {
	logger       *zap.Logger
	pollInterval time.Duration

	nextStartTime time.Time
	wg            *sync.WaitGroup
	doneChan      chan bool
	client        *resty.Client
	consumer      consumer.Logs
}

func newAuditLogsReceiver(cfg *Config, logger *zap.Logger, consumer consumer.Logs) *auditLogsReceiver {
	client := resty.New()
	client.SetBaseURL(cfg.Url + "/v1/audit")
	client.SetHeader("X-API-Key", cfg.Token)
	client.SetHeader("Content-Type", "application/json")
	client.SetRetryCount(1)
	client.SetTimeout(time.Second * 10)

	return &auditLogsReceiver{
		logger:        logger,
		pollInterval:  time.Second * time.Duration(cfg.PollIntervalSec),
		nextStartTime: time.Now().Add(time.Duration(cfg.PollIntervalSec)),
		wg:            &sync.WaitGroup{},
		doneChan:      make(chan bool),
		client:        client,
		consumer:      consumer,
	}
}

func (a *auditLogsReceiver) Start(ctx context.Context, host component.Host) error {
	fmt.Printf("--> HERE 1.1\n")

	a.logger.Debug("starting audit logs receiver")
	a.wg.Add(1)
	go a.startPolling(ctx)

	return nil
}

func (a *auditLogsReceiver) Shutdown(ctx context.Context) error {
	fmt.Printf("--> HERE 1.3\n")

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
	resp, err := a.client.R().SetContext(ctx).Get("")
	if err != nil {
		return err
	}
	if resp.StatusCode()/100 != 2 { // nolint:gomnd
		return fmt.Errorf("unexpected response: code=%d, payload=%v", resp.StatusCode(), string(resp.Body()))
	}

	var m map[string]interface{}
	err = json.Unmarshal(resp.Body(), &m)
	if err != nil {
		return fmt.Errorf("unexpected body in response: %v", resp.Body())
	}

	logs, err := a.processAuditLogs(m)
	if err != nil {
		return fmt.Errorf("processing audit logs items: %w", err)
	}

	if logs.LogRecordCount() > 0 {
		if err = a.consumer.ConsumeLogs(ctx, logs); err != nil {
			return fmt.Errorf("consuming logs: %w", err)
		}
	}

	return nil
}

func (a *auditLogsReceiver) processAuditLogs(auditLogsMap map[string]interface{}) (plog.Logs, error) {
	logs := plog.NewLogs()
	if len(auditLogsMap) == 0 {
		return logs, nil
	}

	i, ok := auditLogsMap["items"]
	if !ok {
		a.logger.Warn("no audit logs items found, response is skipped")
		return logs, nil
	}

	items := i.([]interface{})
	if !ok {
		return logs, fmt.Errorf("invalid response: %v", auditLogsMap)
	}

	// TODO: WIP below
	observedTime := pcommon.NewTimestampFromTime(time.Now())
	for _, it := range items {
		item, ok := it.(map[string]interface{})
		if !ok {
			a.logger.Warn("no audit logs item found, item is skipped")
			continue
		}

		fmt.Printf("--> HERE 1.3 %v\n", item)

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

		str := item["time"].(string)
		layout := "2006-01-02T15:04:05.999999Z"
		t, err := time.Parse(layout, str)
		if err != nil {
			a.logger.Error("--> HERE 1.2 ", zap.Error(err))
		}

		logRecord.SetObservedTimestamp(observedTime)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(t))
	}

	return logs, nil
}
