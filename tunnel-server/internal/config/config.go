package config

import "github.com/caarlos0/env/v11"

type Config struct {
	BaseScheme string `env:"ISKNDR_BASE_SCHEME" envDefault:"http"`
	BaseDomain string `env:"ISKNDR_BASE_DOMAIN" envDefault:"localhost.direct:8080"`
	Port       int    `env:"ISKNDR_PORT" envDefault:"8080"`
	Logging    bool   `env:"ISKNDR_LOGGING" envDefault:"true"`
}

func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
