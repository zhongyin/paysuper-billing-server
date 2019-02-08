package config

import (
	"github.com/kelseyhightower/envconfig"
)

type CacheConfig struct {
	CurrencyTimeout int64 `envconfig:"CACHE_CURRENCY_TIMEOUT" default:"86400"`
}

type Config struct {
	MongoHost          string `envconfig:"MONGO_HOST" required:"true"`
	MongoDatabase      string `envconfig:"MONGO_DB" required:"true"`
	MongoUser          string `envconfig:"MONGO_USER" required:"true"`
	MongoPassword      string `envconfig:"MONGO_PASSWORD" required:"true"`
	AccountingCurrency string `envconfig:"PSP_ACCOUNTING_CURRENCY" required:"true" default:"EUR"`
	MetricsPort        string `envconfig:"METRICS_PORT" required:"false" default:"8085"`
	Environment        string `envconfig:"ENVIRONMENT" default:"dev"`

	*CacheConfig
}

func NewConfig() (*Config, error) {
	cfg := &Config{}
	err := envconfig.Process("", cfg)

	return cfg, err
}
