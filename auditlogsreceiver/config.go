package auditlogsreceiver

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
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
	Filters         FilterConfig           `mapstructure:"filters"`
}

type FilterConfig struct {
	ClusterID *string `mapstructure:"cluster_id,omitempty"`
}

type InMemoryStorageConfig struct {
	BackFromNowSec int `mapstructure:"back_from_now_sec"`
}

type PersistentStorageConfig struct {
	Filename string `mapstructure:"filename"`
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

	if c.PageLimit < 10 || 1000 < c.PageLimit {
		return errors.New("page limit must be within 10...1000 interval")
	}

	if c.Filters.ClusterID != nil && *c.Filters.ClusterID != "" {
		_, err := uuid.Parse(*c.Filters.ClusterID)
		if err != nil {
			return errors.New("cluster id must be a valid UUID")
		}
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

	switch storageType {
	case "in-memory":
		var storageConfig InMemoryStorageConfig
		err = mapstructure.Decode(c.Storage, &storageConfig)
		if err != nil {
			return fmt.Errorf("decoding in-memory storage configuration: %w", err)
		}
	case "persistent":
		var storageConfig PersistentStorageConfig
		err = mapstructure.Decode(c.Storage, &storageConfig)
		if err != nil {
			return fmt.Errorf("decoding persistent storage configuration: %w", err)
		}

		if storageConfig.Filename == "" {
			return fmt.Errorf("file name must be provided in persistent storage configuration")
		}
	default:
		return errors.New("unsupported storage type provided")
	}

	return nil
}
