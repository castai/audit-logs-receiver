package auditlogsreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		config := &Config{
			API:          APIConfig{URL: "https://api.cast.ai", Key: "foobar", Timeout: defaultTimeout},
			PollInterval: defaultPollInterval,
			PageLimit:    defaultPageLimit,
		}

		require.NoError(t, config.Validate())
	})

	t.Run("Error", func(t *testing.T) {
		tests := map[string]struct {
			config  *Config
			errText string
		}{
			"MissingURL": {
				config: &Config{
					API:          APIConfig{URL: "", Timeout: defaultTimeout},
					PollInterval: defaultPollInterval,
					PageLimit:    defaultPageLimit,
				},
				errText: "api.url must be specified",
			},
			"InvalidURL": {
				config: &Config{
					API:          APIConfig{URL: "cast", Timeout: defaultTimeout},
					PollInterval: defaultPollInterval,
					PageLimit:    defaultPageLimit,
				},
				errText: "api.url must be in the form <scheme>://<hostname>:<port>",
			},
			"MissingAPIKey": {
				config: &Config{
					API:          APIConfig{URL: "https://api.cast.ai", Key: "", Timeout: defaultTimeout},
					PollInterval: defaultPollInterval,
					PageLimit:    defaultPageLimit,
				},
				errText: "api.key cannot be empty",
			},
			"InvalidTimeout": {
				config: &Config{
					API:          APIConfig{URL: "https://api.cast.ai", Key: "foobar", Timeout: 0},
					PollInterval: defaultPollInterval,
					PageLimit:    defaultPageLimit,
				},
				errText: "api.timeout must be greater than zero",
			},
			"InvalidPollInterval": {
				config: &Config{
					API:          APIConfig{URL: "https://api.cast.ai", Key: "foobar", Timeout: defaultTimeout},
					PollInterval: 0,
					PageLimit:    defaultPageLimit,
				},
				errText: "poll_interval must be greater than zero",
			},
			"InvalidPageLimit": {
				config: &Config{
					API:          APIConfig{URL: "https://api.cast.ai", Key: "foobar", Timeout: defaultTimeout},
					PollInterval: defaultPollInterval,
					PageLimit:    0,
				},
				errText: "page_limit must be between 1 and 250",
			},
			"PageLimitTooHigh": {
				config: &Config{
					API:          APIConfig{URL: "https://api.cast.ai", Key: "foobar", Timeout: defaultTimeout},
					PollInterval: defaultPollInterval,
					PageLimit:    255,
				},
				errText: "page_limit must be between 1 and 250",
			},
			"NegativeLookback": {
				config: &Config{
					API:          APIConfig{URL: "https://api.cast.ai", Key: "foobar", Timeout: defaultTimeout},
					PollInterval: defaultPollInterval,
					PageLimit:    defaultPageLimit,
					Lookback:     -1 * time.Second,
				},
				errText: "lookback must not be negative",
			},
		}
		for name, tt := range tests {
			t.Run(name, func(t *testing.T) {
				err := tt.config.Validate()

				require.Error(t, err)
				require.ErrorContains(t, err, tt.errText)
			})
		}
	})
}
