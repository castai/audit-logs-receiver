package auditlogs

import (
	"context"
	"errors"
	"fmt"

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
		receiver.WithLogs(CreateTcpStatsReceiver, component.StabilityLevelDevelopment),
	)
}

func CreateTcpStatsReceiver(
	_ context.Context,
	settings receiver.CreateSettings,
	cc component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg, ok := cc.(*Config)
	if !ok {
		return nil, errInvalidConfig
	}

	fmt.Printf("--> HERE 1.1 %v\n", cfg)

	return &auditLogsReceiver{}, nil
}

type auditLogsReceiver struct {
}

func (a *auditLogsReceiver) Start(ctx context.Context, host component.Host) error {
	fmt.Printf("--> HERE 1.2\n")

	return nil
}

func (a *auditLogsReceiver) Shutdown(ctx context.Context) error {
	fmt.Printf("--> HERE 1.3\n")

	return nil
}
