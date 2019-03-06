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
	cfg.AccountingCurrency = "RUB"

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
		&billing.Vat{
			Country: &billing.Country{
				CodeInt:  643,
				CodeA2:   "RU",
				CodeA3:   "RUS",
				Name:     &billing.Name{Ru: "Россия", En: "Russia (Russian Federation)"},
				IsActive: true,
			},
			Subdivision: "",
			Vat:         20,
			IsActive:    true,
		},
		&billing.Vat{
			Country: &billing.Country{
				CodeInt:  840,
				CodeA2:   "US",
				CodeA3:   "USA",
				Name:     &billing.Name{Ru: "Соединенные Штаты Америки", En: "United States of America"},
				IsActive: true,
			},
			Subdivision: "CA",
			Vat:         10.25,
			IsActive:    true,
		},
	}

	err = db.Collection(pkg.CollectionVat).Insert(vat...)

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

	project := &billing.Project{
		Id:               bson.NewObjectId().Hex(),
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

	err = db.Collection(pkg.CollectionProject).Insert(project)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
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

	country := &billing.Country{
		CodeInt:  643,
		CodeA2:   "RU",
		CodeA3:   "RUS",
		Name:     &billing.Name{Ru: "Россия", En: "Russia (Russian Federation)"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCountry).Insert(country)

	if err != nil {
		suite.FailNow("Insert country test data failed", "%v", err)
	}

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -360))

	if err != nil {
		suite.FailNow("Generate merchant date failed", "%v", err)
	}

	merchant := &billing.Merchant{
		Id:           bson.NewObjectId().Hex(),
		CompanyName:  "Unit test",
		Country:      country,
		Zip:          "190000",
		City:         "St.Petersburg",
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

	merchantAgreement := &billing.Merchant{
		Id:           bson.NewObjectId().Hex(),
		CompanyName:  "Unit test status Agreement",
		Country:      country,
		Zip:          "190000",
		City:         "St.Petersburg",
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
		Status:                    pkg.MerchantStatusAgreementRequested,
		LastPayout: &billing.MerchantLastPayout{
			Date:   date,
			Amount: 10000,
		},
		IsSigned: true,
	}
	merchant1 := &billing.Merchant{
		Id:           bson.NewObjectId().Hex(),
		CompanyName:  "merchant1",
		Country:      country,
		Zip:          "190000",
		City:         "St.Petersburg",
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
			Amount: 100000,
		},
		IsSigned: false,
	}

	err = db.Collection(pkg.CollectionMerchant).Insert([]interface{}{merchant, merchantAgreement, merchant1}...)

	if err != nil {
		suite.FailNow("Insert merchant test data failed", "%v", err)
	}

	suite.log, err = zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	suite.service = NewBillingService(db, cfg, make(chan bool, 1), nil, nil, nil)
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
	amount, err := suite.service.Convert(960, 840, 1000)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrencyRate), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_ConvertCurrencyToError() {
	amount, err := suite.service.Convert(643, 960, 1000)

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrencyRate), err.Error())
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
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionVat), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_CalculateVatSubdivisionError() {
	amount, err := suite.service.CalculateVat(float64(1000), "US", "AL")

	assert.Error(suite.T(), err)
	assert.True(suite.T(), amount == 0)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionVat), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_CalculateCommissionOk() {
	amount := float64(100)

	commission, err := suite.service.CalculateCommission(suite.project.Id, suite.paymentMethod.Id, amount)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), commission)
	assert.True(suite.T(), commission.PMCommission > 0)
	assert.True(suite.T(), commission.PspCommission > 0)
	assert.True(suite.T(), commission.ToUserCommission > 0)
	assert.Equal(suite.T(), float64(1), commission.PMCommission)
	assert.Equal(suite.T(), float64(2), commission.PspCommission)
	assert.Equal(suite.T(), float64(1), commission.ToUserCommission)
}

func (suite *FinanceTestSuite) TestFinance_CalculateCommissionProjectError() {
	commission, err := suite.service.CalculateCommission("5bf67ebd46452d00062c7cc1", suite.paymentMethod.Id, float64(100))

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), commission)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCommission), err.Error())
}

func (suite *FinanceTestSuite) TestFinance_CalculateCommissionPaymentMethodError() {
	commission, err := suite.service.CalculateCommission(suite.project.Id, "5bf67ebd46452d00062c7cc1", float64(100))

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), commission)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCommission), err.Error())
}
