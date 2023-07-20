package auditlogs

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/component"
)

type API struct {
	Url string `mapstructure:"url"`
	Key string `mapstructure:"key"`
}

// Config defines the configuration for the TCP stats receiver.
type Config struct {
	API             API                    `mapstructure:"api"`
	PollIntervalSec int                    `mapstructure:"poll_interval_sec"`
	PageLimit       int                    `mapstructure:"page_limit"`
	Storage         map[string]interface{} `mapstructure:"storage"`
}

func newDefaultConfig() component.Config {
	// Default parameters.
	return &Config{
		API: API{
			Url: "https://api.cast.ai",
			Key: "",
		},
		PollIntervalSec: 10,
		PageLimit:       100,
	}
}

func (c Config) Validate() error {
	if c.API.Url == "" {
		return errors.New("api url must be specified")
	}

	_, err := url.ParseRequestURI(c.API.Url)
	if err != nil {
		return errors.New("api url must be in the form of <scheme>://<hostname>:<port>")
	}

	if c.API.Key == "" {
		return errors.New("api access key cannot be empty")
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
