package auditlogs

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the TCP stats receiver.
type Config struct {
	Url             string                 `mapstructure:"castai_api_url"`
	Token           string                 `mapstructure:"castai_api_token"`
	PollIntervalSec int                    `mapstructure:"castai_poll_interval_sec"`
	PageLimit       int                    `mapstructure:"castai_page_limit"`
	Storage         map[string]interface{} `mapstructure:"castai_storage"`
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
	if c.Url == "" {
		return errors.New("api url must be specified")
	}

	_, err := url.ParseRequestURI(c.Url)
	if err != nil {
		return errors.New("api url must be in the form of <scheme>://<hostname>:<port>")
	}

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

	// Validating storage configuration based on its type.
	t, ok := c.Storage["type"]
	if !ok {
		return errors.New("storage type is not defined")
	}
	storageType, ok := t.(string)
	if !ok {
		return errors.New("invalid storage type")
	}

	// TODO: values may be empty
	switch storageType {
	case "in-memory":
		// This is an optional parameter.
		b, ok := c.Storage["back_from_now_sec"]
		if ok {
			_, ok = b.(int)
			if !ok {
				return fmt.Errorf("invalid back_from_now_sec type")
			}
		}
	case "persistent":
		filename, ok := c.Storage["filename"]
		if !ok {
			return fmt.Errorf("filename is missing in persistent storage configuration")
		}

		_, ok = filename.(string)
		if !ok {
			return fmt.Errorf("invalid filename type in persistent storage configuration")
		}
	default:
		return errors.New("unsupported storage type provided")
	}

	return nil
}
