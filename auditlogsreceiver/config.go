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
	PageLimit       int    `mapstructure:"castai_page_limit"`
}

func newDefaultConfig() component.Config {
	// Default parameters.
	return &Config{
		Url:             "https://api.cast.ai",
		PollIntervalSec: 10,
		PageLimit:       100,
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

	// Capping to 1000 records per page which is max supported by the backend.
	if c.PageLimit > 1000 {
		c.PageLimit = 1000
	}

	return nil
}
