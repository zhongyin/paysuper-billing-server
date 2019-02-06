package service

import (
	"github.com/ProtocolONE/payone-billing-service/internal/config"
	"github.com/ProtocolONE/payone-billing-service/internal/database"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"testing"
)

type BillingServiceTestSuite struct {
	suite.Suite
	db   *database.Source
	log  *zap.SugaredLogger
	cfg  *config.Config
	exCh chan bool
}

func Test_BillingService(t *testing.T) {
	suite.Run(t, new(BillingServiceTestSuite))
}

func (suite *BillingServiceTestSuite) SetupTest() {
	cfg, err := config.NewConfig()

	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

	suite.cfg = cfg

	settings := database.Connection{
		Host:     cfg.MongoHost,
		Database: cfg.MongoDatabase,
		User:     cfg.MongoUser,
		Password: cfg.MongoPassword,
	}

	db, err := database.NewDatabase(settings)

	if err != nil {
		suite.FailNow("Database connection failed", "%v", err)
	}

	suite.db = db

	vat := []interface{}{
		&billing.Vat{Country: "RU", Subdivision: "", Vat: 20, IsActive: true},
		&billing.Vat{Country: "UA", Subdivision: "", Vat: 22, IsActive: true},
		&billing.Vat{Country: "US", Subdivision: "AL", Vat: 13.5, IsActive: true},
		&billing.Vat{Country: "US", Subdivision: "CA", Vat: 10.25, IsActive: true},
	}

	err = suite.db.Collection(collectionVat).Insert(vat...)

	if err != nil {
		suite.FailNow("Insert VAT test data failed", "%v", err)
	}

	rub := &billing.Currency{
		CodeInt:  643,
		CodeA3:   "RUB",
		Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
		IsActive: true,
	}

	currency := []interface{}{
		&billing.Currency{
			CodeInt:  840,
			CodeA3:   "USD",
			Name:     &billing.Name{Ru: "Доллар США", En: "US Dollar"},
			IsActive: true,
		},
		rub,
		&billing.Currency{
			CodeInt:  980,
			CodeA3:   "UAH",
			Name:     &billing.Name{Ru: "Украинская гривна", En: "Ukrainian Hryvnia"},
			IsActive: true,
		},
	}

	err = suite.db.Collection(collectionCurrency).Insert(currency...)

	if err != nil {
		suite.FailNow("Insert currency test data failed", "%v", err)
	}

	project := []interface{}{
		&billing.Project{
			CallbackCurrency: rub,
			CallbackProtocol: "default",
			LimitsCurrency:   rub,
			MaxPaymentAmount: 15000,
			MinPaymentAmount: 0,
			Name:             "test project 1",
			OnlyFixedAmounts: true,
			SecretKey:        "test project 1 secret key",
			IsActive:         true,
		},
		&billing.Project{
			CallbackCurrency: rub,
			CallbackProtocol: "xsolla",
			LimitsCurrency:   rub,
			MaxPaymentAmount: 15000,
			MinPaymentAmount: 0,
			Name:             "test project 2",
			OnlyFixedAmounts: true,
			SecretKey:        "test project 2 secret key",
			IsActive:         true,
		},
		&billing.Project{
			CallbackCurrency: rub,
			CallbackProtocol: "cardpay",
			LimitsCurrency:   rub,
			MaxPaymentAmount: 15000,
			MinPaymentAmount: 0,
			Name:             "test project 3",
			OnlyFixedAmounts: true,
			SecretKey:        "test project 3 secret key",
			IsActive:         true,
		},
	}

	err = suite.db.Collection(collectionProject).Insert(project...)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
	}

	rate := []interface{}{
		&billing.CurrencyRate{
			CurrencyFrom: 980,
			CurrencyTo:   643,
			Rate:         0.411128442,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
		&billing.CurrencyRate{
			CurrencyFrom: 980,
			CurrencyTo:   840,
			Rate:         27.13085922,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
		&billing.CurrencyRate{
			CurrencyFrom: 980,
			CurrencyTo:   978,
			Rate:         30.96446748,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
		&billing.CurrencyRate{
			CurrencyFrom: 840,
			CurrencyTo:   980,
			Rate:         0.034680066,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
		&billing.CurrencyRate{
			CurrencyFrom: 840,
			CurrencyTo:   643,
			Rate:         0.01469893,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
		&billing.CurrencyRate{
			CurrencyFrom: 840,
			CurrencyTo:   840,
			Rate:         1.00000000,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
		&billing.CurrencyRate{
			CurrencyFrom: 643,
			CurrencyTo:   840,
			Rate:         64.01146400,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
		&billing.CurrencyRate{
			CurrencyFrom: 643,
			CurrencyTo:   643,
			Rate:         1,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
		&billing.CurrencyRate{
			CurrencyFrom: 643,
			CurrencyTo:   980,
			Rate:         2.2885792,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
	}

	err = suite.db.Collection(collectionCurrencyRate).Insert(rate...)

	if err != nil {
		suite.FailNow("Insert rates test data failed", "%v", err)
	}

	logger, err := zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	suite.log = logger.Sugar()
	suite.exCh = make(chan bool, 1)
}

func (suite *BillingServiceTestSuite) TearDownTest() {
	if err := suite.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.db.Close()

	if err := suite.log.Sync(); err != nil {
		suite.FailNow("Logger sync failed", "%v", err)
	}
}

func (suite *BillingServiceTestSuite) TestNewBillingService() {
	service, err := NewBillingService(suite.db, suite.log, suite.cfg.CacheConfig, suite.exCh)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(service.currencyCache) > 0)
	assert.True(suite.T(), len(service.projectCache) > 0)
	assert.True(suite.T(), len(service.currencyRateCache) > 0)
	assert.True(suite.T(), len(service.vatCache) > 0)
}
