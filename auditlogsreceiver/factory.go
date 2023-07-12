package auditlogs

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/castai/otel-receivers/audit-logs/storage"

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
		rest:          newRestyClient(cfg),
		consumer:      consumer,
	}, nil
}

func newRestyClient(cfg *Config) *resty.Client {
	client := resty.New().
		SetHeader("Content-Type", "application/json").
		SetRetryCount(1).
		SetTimeout(time.Second*10).
		SetBaseURL(strings.TrimSuffix(cfg.Url, "/")+"/v1/audit").
		SetHeader("X-API-Key", cfg.Token)
	return client
}
