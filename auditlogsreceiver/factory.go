package auditlogs

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/castai/otel-receivers/audit-logs/internal/metadata"
	"github.com/castai/otel-receivers/audit-logs/storage"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		newDefaultConfig,
		receiver.WithLogs(NewAuditLogsReceiver, component.StabilityLevelDevelopment),
	)
}

func NewAuditLogsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg, ok := cc.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type")
	}

	st, err := newStorage(settings.Logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating storage: %w", err)
	}

	return &auditLogsReceiver{
		logger:       settings.Logger,
		pollInterval: time.Second * time.Duration(cfg.PollIntervalSec),
		pageLimit:    cfg.PageLimit,
		wg:           &sync.WaitGroup{},
		doneChan:     make(chan bool),
		storage:      st,
		rest:         newRestyClient(cfg),
		consumer:     consumer,
	}, nil
}

func newStorage(logger *zap.Logger, cfg *Config) (storage.Storage, error) {
	// Configuration validation is done in config.validate method, so it is safe to use configuration without validations here.
	storageType := cfg.Storage["type"].(string)

	switch storageType {
	case "in-memory":
		backFromNowSec := 0
		b, ok := cfg.Storage["back_from_now_sec"].(int)
		if ok {
			backFromNowSec = b
		}

		return storage.NewInMemoryStorage(logger, backFromNowSec), nil
	case "persistent":
		return storage.NewPersistentStorage(logger, cfg.Storage["filename"].(string))
	default:
		return nil, fmt.Errorf("invalid storage type provided for audit logs exporter: %v", storageType)
	}
}

func newRestyClient(cfg *Config) *resty.Client {
	return resty.New().
		SetHeader("Content-Type", "application/json").
		SetRetryCount(1).
		SetTimeout(time.Second*10).
		SetBaseURL(strings.TrimSuffix(cfg.Url, "/")+"/v1/audit").
		SetHeader("X-API-Key", cfg.Token)
}
