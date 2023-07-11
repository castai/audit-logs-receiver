package auditlogs

import (
	"context"
	"errors"
	"github.com/castai/otel-receivers/audit-logs/storage"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/castai/otel-receivers/audit-logs/internal/metadata"
)

var errInvalidConfig = errors.New("invalid config for tcpstatsreceiver")

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		newDefaultConfig,
		receiver.WithLogs(CreateAuditLogsReceiver, component.StabilityLevelDevelopment),
	)
}

func CreateAuditLogsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg, ok := cc.(*Config)
	if !ok {
		return nil, errInvalidConfig
	}

	rest := resty.New()
	rest.SetBaseURL(cfg.Url + "/v1/audit")
	rest.SetHeader("X-API-Key", cfg.Token)
	rest.SetHeader("Content-Type", "application/json")
	rest.SetRetryCount(1)
	rest.SetTimeout(time.Second * 10)

	// TODO: introduce possibility to use Persistent Store based on configuration.
	store := storage.NewEphemeralStore(time.Now().Add(-1 * time.Duration(cfg.PollIntervalSec) * time.Second))

	return &auditLogsReceiver{
		logger:        settings.Logger,
		pollInterval:  time.Second * time.Duration(cfg.PollIntervalSec),
		pageLimit:     cfg.PageLimit,
		nextStartTime: time.Now().Add(time.Duration(cfg.PollIntervalSec)),
		wg:            &sync.WaitGroup{},
		doneChan:      make(chan bool),
		store:         store,
		rest:          rest,
		consumer:      consumer,
	}, nil
}
