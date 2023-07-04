package auditlogs

import (
	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the TCP stats receiver.
type Config struct {
	Url   string `mapstructure:"castai_api_url"`
	Token string `mapstructure:"castai_api_token"`
}

func newDefaultConfig() component.Config {
	return &Config{}
}
