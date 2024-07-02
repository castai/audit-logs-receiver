package auditlogsreceiver

import (
	"context"
	"fmt"
	"github.com/castai/audit-logs-receiver/audit-logs/internal/metadata"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/castai/audit-logs-receiver/audit-logs/storage"
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
	settings receiver.Settings,
	cc component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg, ok := cc.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type")
	}

	// This is where logger may be adjusted if needed.
	logger := settings.Logger

	st, err := newStorage(settings.Logger, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating storage: %w", err)
	}

	return &auditLogsReceiver{
		logger:       logger,
		pollInterval: time.Second * time.Duration(cfg.PollIntervalSec),
		pageLimit:    cfg.PageLimit,
		filter: filters{
			clusterID: cfg.Filters.ClusterID,
		},
		wg:          &sync.WaitGroup{},
		stopPolling: func() {},
		storage:     st,
		rest:        newRestyClient(cfg),
		consumer:    consumer,
	}, nil
}

func newStorage(logger *zap.Logger, cfg *Config) (storage.Storage, error) {
	// Configuration validation is done in config.validate method, so it is safe to use configuration without validations here.
	storageType := cfg.Storage["type"].(string)

	// TODO: Consider reuse with config.go
	switch storageType {
	case "in-memory":
		var storageConfig InMemoryStorageConfig
		err := mapstructure.Decode(cfg.Storage, &storageConfig)
		if err != nil {
			return nil, fmt.Errorf("decoding in-memory storage configuration: %w", err)
		}

		return storage.NewInMemoryStorage(logger, storageConfig.BackFromNowSec), nil
	case "persistent":
		var storageConfig PersistentStorageConfig
		err := mapstructure.Decode(cfg.Storage, &storageConfig)
		if err != nil {
			return nil, fmt.Errorf("decoding persistent storage configuration: %w", err)
		}

		return storage.NewPersistentStorage(logger, storageConfig.Filename)
	default:
		return nil, fmt.Errorf("invalid storage type provided for audit logs exporter: %v", storageType)
	}
}

func newRestyClient(cfg *Config) *resty.Client {
	return resty.New().
		// TODO: look up version during build process
		SetHeader("User-Agent", "castai/audit-logs-receiver/0.1.0").
		SetHeader("Content-Type", "application/json").
		SetRetryCount(1).
		SetTimeout(time.Minute).
		SetBaseURL(strings.TrimSuffix(cfg.API.Url, "/")+"/v1/audit").
		SetHeader("X-API-Key", cfg.API.Key)
}
