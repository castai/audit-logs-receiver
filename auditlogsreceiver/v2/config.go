package auditlogsreceiver

import (
	"errors"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
)

// List of default values and configuration options.
const (
	defaultAPIURL       = "https://api.cast.ai"
	defaultTimeout      = 30 * time.Second
	defaultPollInterval = 10 * time.Second
	defaultPageLimit    = 100
)

// APIConfig holds API configuration.
type APIConfig struct {
	URL     string        `mapstructure:"url"`
	Key     string        `mapstructure:"key"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// FilterConfig is audit logs filter configuration.
type FilterConfig struct {
	Search    string   `mapstructure:"search"`
	Clusters  []string `mapstructure:"clusters"`
	Domains   []string `mapstructure:"domains"`
	Resources []string `mapstructure:"resources"`
	Actions   []string `mapstructure:"actions"`
	Sources   []string `mapstructure:"sources"`
	Severity  []string `mapstructure:"severity"`
}

// ToFilters converts FilterConfig to client Filters.
func (f FilterConfig) ToFilters() Filters {
	return Filters{
		Search:    f.Search,
		Clusters:  f.Clusters,
		Domains:   f.Domains,
		Resources: f.Resources,
		Actions:   f.Actions,
		Sources:   f.Sources,
		Severity:  f.Severity,
	}
}

// Config is the audit logs receiver configuration.
type Config struct {
	API            APIConfig     `mapstructure:"api"`
	PollInterval   time.Duration `mapstructure:"poll_interval"`
	PageLimit      int           `mapstructure:"page_limit"`
	Lookback       time.Duration `mapstructure:"lookback"`
	CheckpointFile string        `mapstructure:"checkpoint_file"`
	Filters        FilterConfig  `mapstructure:"filters"`
}

// CreateDefaultConfig returns configuration defaults as component.Config.
func CreateDefaultConfig() component.Config {
	return &Config{
		API: APIConfig{
			URL:     defaultAPIURL,
			Timeout: defaultTimeout,
		},
		PollInterval: defaultPollInterval,
		PageLimit:    defaultPageLimit,
	}
}

// Validate will examine c and return an appropriate error if any of its fields
// hold unexpected or unsupported values.
func (c *Config) Validate() error {
	if c.API.URL == "" {
		return errors.New("api.url must be specified")
	}
	if _, err := url.ParseRequestURI(c.API.URL); err != nil {
		return errors.New("api.url must be in the form <scheme>://<hostname>:<port>")
	}
	if c.API.Key == "" {
		return errors.New("api.key cannot be empty")
	}
	if c.API.Timeout <= 0 {
		return errors.New("api.timeout must be greater than zero")
	}
	if c.PollInterval <= 0 {
		return errors.New("poll_interval must be greater than zero")
	}
	if c.PageLimit < 1 || c.PageLimit > 250 {
		return errors.New("page_limit must be between 1 and 250")
	}
	if c.Lookback < 0 {
		return errors.New("lookback must not be negative")
	}

	return nil
}
