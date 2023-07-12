package auditlogs

import (
	"errors"
	"net/url"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the TCP stats receiver.
type Config struct {
	Url             string `mapstructure:"castai_api_url"`
	Token           string `mapstructure:"castai_api_token"`
	PollIntervalSec int    `mapstructure:"castai_poll_interval_sec"`
	PageLimit       int    `mapstructure:"castai_page_limit"`
}

var (
	errEmptyURL        = errors.New("apir url must be specified")
	errInvalidURL      = errors.New("api url must be in the form of <scheme>://<hostname>:<port>")
	errEmptyToken      = errors.New("api token cannot be empty")
	errInvalidInterval = errors.New("poll interval must be positive number")
)

func newDefaultConfig() component.Config {
	// Default parameters.
	return &Config{
		Url:             "https://api.cast.ai",
		PollIntervalSec: 10,
		PageLimit:       100,
	}
}

func (c Config) Validate() error {
	if c.Url == "" {
		return errEmptyURL
	}

	_, err := url.ParseRequestURI(c.Url)
	if err != nil {
		return errInvalidURL
	}

	if c.Token == "" {
		return errEmptyToken
	}

	if c.PollIntervalSec <= 0 {
		return errInvalidInterval
	}

	// Capping to 1000 records per page which is max supported by the backend.
	if c.PageLimit > 1000 {
		c.PageLimit = 1000
	}

	return nil
}
