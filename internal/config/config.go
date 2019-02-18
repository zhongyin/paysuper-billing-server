package config

import (
	"github.com/kelseyhightower/envconfig"
)

type CacheConfig struct {
	CurrencyTimeout             int64 `envconfig:"CACHE_CURRENCY_TIMEOUT" default:"15552000"`
	ProjectTimeout              int64 `envconfig:"CACHE_PROJECT_TIMEOUT" default:"10800"`
	CurrencyRateTimeout         int64 `envconfig:"CACHE_CURRENCY_RATE_TIMEOUT" default:"86400"`
	VatTimeout                  int64 `envconfig:"CACHE_VAT_TIMEOUT" default:"2592000"`
	PaymentMethodTimeout        int64 `envconfig:"CACHE_PAYMENT_METHOD_TIMEOUT" default:"2592000"`
	CommissionTimeout           int64 `envconfig:"CACHE_COMMISSION_TIMEOUT" default:"86400"`
	ProjectPaymentMethodTimeout int64 `envconfig:"CACHE_PROJECT_PAYMENT_METHOD_TIMEOUT" default:"86400"`
}

type PaymentSystemConfig struct {
	CardPayOrderCreateUrl string `envconfig:"CARD_PAY_ORDER_CREATE_URL" required:"false"`
}

type Config struct {
	MongoHost          string `envconfig:"MONGO_HOST" required:"true"`
	MongoDatabase      string `envconfig:"MONGO_DB" required:"true"`
	MongoUser          string `envconfig:"MONGO_USER" required:"true"`
	MongoPassword      string `envconfig:"MONGO_PASSWORD" required:"true"`
	AccountingCurrency string `envconfig:"PSP_ACCOUNTING_CURRENCY" required:"true" default:"EUR"`
	MetricsPort        string `envconfig:"METRICS_PORT" required:"false" default:"8086"`
	Environment        string `envconfig:"ENVIRONMENT" default:"dev"`

	CentrifugoSecret string `envconfig:"CENTRIFUGO_SECRET" required:"true"`
	BrokerAddress    string `envconfig:"BROKER_ADDRESS" required:"true" default:"amqp://127.0.0.1:5672"`

	*CacheConfig
	*PaymentSystemConfig
}

func NewConfig() (*Config, error) {
	cfg := &Config{}
	err := envconfig.Process("", cfg)

	return cfg, err
}
