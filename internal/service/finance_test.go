package service

import (
	"fmt"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"testing"
	"time"
)

type FinanceTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger

	project       *billing.Project
	paymentMethod *billing.PaymentMethod
}

func Test_Finance(t *testing.T) {
	suite.Run(t, new(FinanceTestSuite))
}

func (suite *FinanceTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}
	cfg.AccountingCurrency = "RUB"

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

	rub := &billing.Currency{
		CodeInt:  643,
		CodeA3:   "RUB",
		Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
		IsActive: true,
	}

	currency := []interface{}{rub}

	err = db.Collection(pkg.CollectionCurrency).Insert(currency...)

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

	err = db.Collection(pkg.CollectionCurrencyRate).Insert(rate...)

	if err != nil {
		suite.FailNow("Insert rates test data failed", "%v", err)
	}

	pmBankCard := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "Bank card",
		Group:            "BANKCARD",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:    "cardpay",
			Terminal:   "15985",
			ExternalId: "BANKCARD",
		},
		Type:     "bank_card",
		IsActive: true,
	}

	country := &billing.Country{
		CodeInt:  643,
		CodeA2:   "RU",
		CodeA3:   "RUS",
		Name:     &billing.Name{Ru: "Россия", En: "Russia (Russian Federation)"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCountry).Insert(country)
	assert.NoError(suite.T(), err, "Insert country test data failed")

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -360))
	assert.NoError(suite.T(), err, "Generate merchant date failed")

	merchant := &billing.Merchant{
		Id:      bson.NewObjectId().Hex(),
		Name:    "Unit test",
		Country: country,
		Zip:     "190000",
		City:    "St.Petersburg",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "123456789",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "123456789",
			},
		},
		Banking: &billing.MerchantBanking{
			Currency: rub,
			Name:     "Bank name",
		},
		IsVatEnabled:              true,
		IsCommissionToUserEnabled: true,
		Status:                    pkg.MerchantStatusDraft,
		LastPayout: &billing.MerchantLastPayout{
			Date:   date,
			Amount: 999999,
		},
		IsSigned: true,
		PaymentMethods: map[string]*billing.MerchantPaymentMethod{
			pmBankCard.Id: {
				PaymentMethod: &billing.MerchantPaymentMethodIdentification{
					Id:   pmBankCard.Id,
					Name: pmBankCard.Name,
				},
				Commission: &billing.MerchantPaymentMethodCommissions{
					Fee: 2.5,
					PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
						Fee:      30,
						Currency: rub.CodeA3,
					},
				},
				Integration: &billing.MerchantPaymentMethodIntegration{
					TerminalId:       "1234567890",
					TerminalPassword: "0987654321",
					Integrated:       true,
				},
				IsActive: true,
			},
		},
	}

	err = db.Collection(pkg.CollectionMerchant).Insert(merchant)
	assert.NoError(suite.T(), err, "Insert merchant test data failed")

	project := &billing.Project{
		Id:                 bson.NewObjectId().Hex(),
		CallbackCurrency:   rub.CodeA3,
		CallbackProtocol:   "default",
		LimitsCurrency:     rub.CodeA3,
		MaxPaymentAmount:   15000,
		MinPaymentAmount:   0,
		Name:               map[string]string{"en": "test project 1"},
		IsProductsCheckout: true,
		SecretKey:          "test project 1 secret key",
		Status:             pkg.ProjectStatusInProduction,
		MerchantId:         merchant.Id,
	}

	err = db.Collection(pkg.CollectionProject).Insert(project)
	assert.NoError(suite.T(), err, "Insert project test data failed")

	pmQiwi := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "Qiwi",
		Group:            "QIWI",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:    "cardpay",
			Terminal:   "15993",
			ExternalId: "QIWI",
		},
		Type:     "ewallet",
		IsActive: true,
	}
	pmBitcoin := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "Bitcoin",
		Group:            "BITCOIN",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:    "cardpay",
			Terminal:   "16007",
			ExternalId: "BITCOIN",
		},
		Type:     "crypto",
		IsActive: true,
	}

	pms := []interface{}{pmBankCard, pmQiwi, pmBitcoin}

	err = db.Collection(pkg.CollectionPaymentMethod).Insert(pms...)

	if err != nil {
		suite.FailNow("Insert payment methods test data failed", "%v", err)
	}

	commissionStartDate, err := ptypes.TimestampProto(time.Now().Add(time.Minute * -10))

	if err != nil {
		suite.FailNow("Commission start date conversion failed", "%v", err)
	}

	commissions := []interface{}{
		&billing.Commission{
			PaymentMethodId:         pmBankCard.Id,
			ProjectId:               project.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   1,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmQiwi.Id,
			ProjectId:               project.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   2,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBitcoin.Id,
			ProjectId:               project.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   3,
			StartDate:               commissionStartDate,
		},
	}

	err = db.Collection(pkg.CollectionCommission).Insert(commissions...)

	if err != nil {
		suite.FailNow("Insert commission test data failed", "%v", err)
	}

	suite.log, err = zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	suite.service = NewBillingService(db, cfg, make(chan bool, 1), nil, nil, nil, nil, nil)
	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.project = project
	suite.paymentMethod = pmBankCard
}

func (suite *FinanceTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
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
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrency), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_ConvertOk() {
	origin := float64(1000)
	expect := 15.63

	amount, err := suite.service.Convert(643, 840, origin)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), amount > 0)
	assert.Equal(suite.T(), expect, amount)
}

func (suite *FinanceTestSuite) TestFinance_ConvertCurrencyFromError() {
	amount, err := suite.service.Convert(980, 840, 1000)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrencyRate), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_ConvertCurrencyToError() {
	amount, err := suite.service.Convert(643, 980, 1000)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrencyRate), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_CalculateCommissionOk() {
	amount := float64(100)

	commission, err := suite.service.CalculatePmCommission(suite.project.Id, suite.paymentMethod.Id, amount)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), commission)
	assert.True(suite.T(), commission > 0)
	assert.Equal(suite.T(), float64(2.5), commission)
}

func (suite *FinanceTestSuite) TestFinance_CalculateCommissionProjectError() {
	commission, err := suite.service.CalculatePmCommission(bson.NewObjectId().Hex(), suite.paymentMethod.Id, float64(100))

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), float64(0), commission)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCommission), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_CalculateCommissionPaymentMethodError() {
	commission, err := suite.service.CalculatePmCommission(suite.project.Id, bson.NewObjectId().Hex(), float64(100))

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), float64(0), commission)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCommission), err.Error())
}
