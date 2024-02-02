package config

import (
	"github.com/caarlos0/env/v6"
)

// Config is the global config.
type Config struct {
	UnsignedAPI string `env:"UNSIGNED_API"`
	SignedAPI   string `env:"SIGNED_API"`
}

// LoadConfig load config from environment.
func LoadConfig() (*Config, error) {
	config := Config{}
	if err := env.Parse(&config); err != nil {
		return nil, err
	}
	return &config, nil
}
