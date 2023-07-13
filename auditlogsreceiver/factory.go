package auditlogs

import (
	"context"
	"errors"
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

var errInvalidConfig = errors.New("invalid config for tcpstatsreceiver")

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
		return nil, errInvalidConfig
	}

	return &auditLogsReceiver{
		logger:        settings.Logger,
		pollInterval:  time.Second * time.Duration(cfg.PollIntervalSec),
		pageLimit:     cfg.PageLimit,
		nextStartTime: time.Now().Add(time.Duration(cfg.PollIntervalSec)),
		wg:            &sync.WaitGroup{},
		doneChan:      make(chan bool),
		storage:       newStorage(settings.Logger, cfg),
		rest:          newRestyClient(cfg),
		consumer:      consumer,
	}, nil
}

func newStorage(logger *zap.Logger, cfg *Config) storage.Storage {
	// Configuration validation is done in config.validate method, so it is safe to use configuration without validations here.
	storageType := cfg.Storage["type"].(string)

	// TODO: introduce possibility to use Persistent Storage based on configuration.
	switch storageType {
	case "in-memory":
		backFromNowSec := cfg.Storage["back_from_now_sec"].(int)
		from := time.Now().Add(time.Second * time.Duration(-backFromNowSec))
		logger.Info("creating an in-memory storage used for keeping timestamps by audit logs receiver", zap.Time("from", from))
		return storage.NewInMemoryStorage(from)
	case "persistent":
		backFromNowSec := cfg.Storage["back_from_now_sec"].(int)
		from := time.Now().Add(time.Second * time.Duration(-backFromNowSec))
		logger.Info("creating a persistent storage used for keeping timestamps by audit logs receiver", zap.Time("from", from))
		return storage.NewPersistentStorage(from)
	default:
		logger.Fatal("invalid storage type provided for audit logs exporter", zap.String("type", storageType))
		return nil
	}
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
