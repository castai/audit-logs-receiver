package otelcolreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr = "otelcolreceiver"
)

func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsReceiver(_ context.Context, params receiver.CreateSettings, baseCfg component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	return nil, nil
}

// NewFactory creates a factory for tailtracer receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelAlpha))
}
