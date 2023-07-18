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
		logger:       settings.Logger,
		pollInterval: time.Second * time.Duration(cfg.PollIntervalSec),
		pageLimit:    cfg.PageLimit,
		wg:           &sync.WaitGroup{},
		doneChan:     make(chan bool),
		storage:      newStorage(settings.Logger, cfg),
		rest:         newRestyClient(cfg),
		consumer:     consumer,
	}, nil
}

func newStorage(logger *zap.Logger, cfg *Config) storage.Storage {
	// Configuration validation is done in config.validate method, so it is safe to use configuration without validations here.
	storageType := cfg.Storage["type"].(string)

	switch storageType {
	case "in-memory":
		backFromNowSec := cfg.Storage["back_from_now_sec"].(int)
		from := time.Now().UTC().Add(time.Second * time.Duration(-backFromNowSec))
		logger.Info("creating an in-memory storage used for keeping timestamps by audit logs receiver", zap.Time("from", from))
		return storage.NewInMemoryStorage(logger, storage.PollData{
			// TODO: read from config
			CheckPoint: time.Now().UTC(),
		})
	case "persistent":
		filename := cfg.Storage["filename"].(string)
		logger.Info("creating a persistent storage used for keeping timestamps by audit logs receiver", zap.String("filename", filename))
		return storage.NewPersistentStorage(logger, filename)
	default:
		logger.Fatal("invalid storage type provided for audit logs exporter", zap.String("type", storageType))
		return nil
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
