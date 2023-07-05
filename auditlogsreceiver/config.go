package auditlogs

import (
	"errors"
	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the TCP stats receiver.
type Config struct {
	Url             string `mapstructure:"castai_api_url"`
	Token           string `mapstructure:"castai_api_token"`
	PollIntervalSec int    `mapstructure:"castai_poll_interval_sec"`
}

func newDefaultConfig() component.Config {
	return &Config{
		Url:             "https://api.cast.ai",
		PollIntervalSec: 10,
	}
}

func (c Config) Validate() error {
	// TODO: Validate URL and trim last '/' if present

	if c.Token == "" {
		return errors.New("api token cannot be empty")
	}

	if c.PollIntervalSec <= 0 {
		return errors.New("poll interval must be positive number")
	}

	// TODO: implement API ping to validate URL & Token

	return nil
}
