package service

import (
	"fmt"
	"github.com/ProtocolONE/payone-billing-service/internal/config"
	"github.com/ProtocolONE/payone-billing-service/internal/database"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"testing"
)

type FinanceTestSuite struct {
	suite.Suite
	service *Service
}

func Test_Finance(t *testing.T) {
	suite.Run(t, new(FinanceTestSuite))
}

func (suite *FinanceTestSuite) SetupTest() {
	cfg, err := config.NewConfig()

	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}

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

	vat := []interface{}{
		&billing.Vat{Country: "RU", Subdivision: "", Vat: 20, IsActive: true},
		&billing.Vat{Country: "US", Subdivision: "CA", Vat: 10.25, IsActive: true},
	}

	err = db.Collection(collectionVat).Insert(vat...)

	if err != nil {
		suite.FailNow("Insert VAT test data failed", "%v", err)
	}

	rub := &billing.Currency{
		CodeInt:  643,
		CodeA3:   "RUB",
		Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
		IsActive: true,
	}

	currency := []interface{}{rub}

	err = db.Collection(collectionCurrency).Insert(currency...)

	if err != nil {
		suite.FailNow("Insert currency test data failed", "%v", err)
	}

	rate := []interface{}{
		&billing.CurrencyRate{
			CurrencyFrom: 643,
			CurrencyTo:   840,
			Rate:         64,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
	}

	err = db.Collection(collectionCurrencyRate).Insert(rate...)

	if err != nil {
		suite.FailNow("Insert rates test data failed", "%v", err)
	}

	project := &billing.Project{
		CallbackCurrency: rub,
		CallbackProtocol: "default",
		LimitsCurrency:   rub,
		MaxPaymentAmount: 15000,
		MinPaymentAmount: 0,
		Name:             "test project 1",
		OnlyFixedAmounts: true,
		SecretKey:        "test project 1 secret key",
		IsActive:         true,
	}

	err = db.Collection(collectionProject).Insert(project)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
	}

	logger, err := zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	suite.service = NewBillingService(db, logger.Sugar(), cfg.CacheConfig, make(chan bool, 1))
	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}
}

func (suite *FinanceTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()

	if err := suite.service.log.Sync(); err != nil {
		suite.FailNow("Logger sync failed", "%v", err)
	}
}

func (suite *FinanceTestSuite) TestFinance_GetCurrencyByCodeA3Ok() {
	c, err := suite.service.GetCurrencyByCodeA3("RUB")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), c)
	assert.Equal(suite.T(), int32(643), c.CodeInt)
}

func (suite *FinanceTestSuite) TestFinance_GetCurrencyByCodeA3Error() {
	c, err := suite.service.GetCurrencyByCodeA3("AUD")

	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), c)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrency), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_ConvertOk() {
	origin := float64(1000)
	expect := 15.625

	amount, err := suite.service.Convert(643, 840, origin)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), amount > 0)
	assert.Equal(suite.T(), expect, amount)
}

func (suite *FinanceTestSuite) TestFinance_ConvertCurrencyFromError() {
	amount, err := suite.service.Convert(960, 840, 1000)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_ConvertCurrencyToError() {
	amount, err := suite.service.Convert(643, 960, 1000)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_CalculateVatWithoutSubdivisionOk() {
	origin := float64(1000)
	expect := float64(200)

	amount1, err := suite.service.CalculateVat(origin, "RU", "SPE")

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), amount1 > 0)
	assert.Equal(suite.T(), expect, amount1)

	amount2, err := suite.service.CalculateVat(origin, "RU", "")

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), amount2 > 0)
	assert.Equal(suite.T(), expect, amount2)

	assert.Equal(suite.T(), amount1, amount2)
}

func (suite *FinanceTestSuite) TestFinance_CalculateVatWithSubdivisionOk() {
	origin := float64(1000)
	expect := float64(102.5)

	amount, err := suite.service.CalculateVat(origin, "US", "CA")

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), amount > 0)
	assert.Equal(suite.T(), expect, amount)
}

func (suite *FinanceTestSuite) TestFinance_CalculateVatCountryError() {
	amount, err := suite.service.CalculateVat(float64(1000), "AU", "")

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionVat), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_CalculateVatSubdivisionError() {
	amount, err := suite.service.CalculateVat(float64(1000), "US", "AL")

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionVat), err.Error())
}
