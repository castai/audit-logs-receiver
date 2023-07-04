package otelcolreceiver

import "errors"

type Config struct {
	Token string `mapstructure:"token"`
}

var (
	errNoToken = errors.New("no CAST AI token was specified")
)

func (c *Config) Validate() error {
	if c.Token == "" {
		return errNoToken
	}

	return nil
}
