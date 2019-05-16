package config

import (
	"crypto/rsa"
	"encoding/base64"
	"github.com/dgrijalva/jwt-go"
	"github.com/kelseyhightower/envconfig"
	"time"
)

type CacheConfig struct {
	CurrencyTimeout      int64 `envconfig:"CACHE_CURRENCY_TIMEOUT" default:"15552000"`
	CountryTimeout       int64 `envconfig:"CACHE_COUNTRY_TIMEOUT" default:"15552000"`
	ProjectTimeout       int64 `envconfig:"CACHE_PROJECT_TIMEOUT" default:"10800"`
	CurrencyRateTimeout  int64 `envconfig:"CACHE_CURRENCY_RATE_TIMEOUT" default:"86400"`
	PaymentMethodTimeout int64 `envconfig:"CACHE_PAYMENT_METHOD_TIMEOUT" default:"2592000"`
	CommissionTimeout    int64 `envconfig:"CACHE_COMMISSION_TIMEOUT" default:"86400"`
	OrderProductsTimeout int64 `envconfig:"CACHE_ORDER_PRODUCTS_TIMEOUT" default:"86400"`
	SystemFeesTimeout    int64 `envconfig:"CACHE_SYSTEM_FEES_TIMEOUT" default:"86400"`
}

type PaymentSystemConfig struct {
	CardPayApiUrl string `envconfig:"CARD_PAY_API_URL" required:"true"`
}

type CustomerTokenConfig struct {
	Length   int   `envconfig:"CUSTOMER_TOKEN_LENGTH" default:"32"`
	LifeTime int64 `envconfig:"CUSTOMER_TOKEN_LIFETIME" default:"2592000"`

	CookiePublicKeyBase64  string `envconfig:"CUSTOMER_COOKIE_PUBLIC_KEY" required:"true"`
	CookiePrivateKeyBase64 string `envconfig:"CUSTOMER_COOKIE_PRIVATE_KEY" required:"true"`
	CookiePublicKey        *rsa.PublicKey
	CookiePrivateKey       *rsa.PrivateKey
}

type Config struct {
	MongoHost          string `envconfig:"MONGO_HOST" required:"true"`
	MongoDatabase      string `envconfig:"MONGO_DB" required:"true"`
	MongoUser          string `envconfig:"MONGO_USER" default:""`
	MongoPassword      string `envconfig:"MONGO_PASSWORD" default:""`
	AccountingCurrency string `envconfig:"PSP_ACCOUNTING_CURRENCY" default:"EUR"`
	MetricsPort        string `envconfig:"METRICS_PORT" required:"false" default:"8086"`
	Environment        string `envconfig:"ENVIRONMENT" default:"dev"`
	RedisHost          string `envconfig:"REDIS_HOST" default:"127.0.0.1:6379"`
	RedisPassword      string `envconfig:"REDIS_PASSWORD" default:""`

	CentrifugoSecret string `envconfig:"CENTRIFUGO_SECRET" required:"true"`
	CentrifugoURL    string `envconfig:"CENTRIFUGO_URL" required:"false" default:"http://127.0.0.1:8000"`
	BrokerAddress    string `envconfig:"BROKER_ADDRESS" default:"amqp://127.0.0.1:5672"`

	MicroRegistry string `envconfig:"MICRO_REGISTRY" required:"false"`

	*CacheConfig
	*PaymentSystemConfig
	*CustomerTokenConfig
}

func NewConfig() (*Config, error) {
	cfg := &Config{}
	err := envconfig.Process("", cfg)

	pem, err := base64.StdEncoding.DecodeString(cfg.CookiePublicKeyBase64)

	if err != nil {
		return nil, err
	}

	cfg.CookiePublicKey, err = jwt.ParseRSAPublicKeyFromPEM(pem)

	if err != nil {
		return nil, err
	}

	pem, err = base64.StdEncoding.DecodeString(cfg.CookiePrivateKeyBase64)

	if err != nil {
		return nil, err
	}

	cfg.CookiePrivateKey, err = jwt.ParseRSAPrivateKeyFromPEM(pem)

	if err != nil {
		return nil, err
	}

	return cfg, err
}

func (cfg *Config) GetCustomerTokenLength() int {
	return cfg.CustomerTokenConfig.Length
}

func (cfg *Config) GetCustomerTokenExpire() time.Duration {
	return time.Second * time.Duration(cfg.CustomerTokenConfig.LifeTime)
}
