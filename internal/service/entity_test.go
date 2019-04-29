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

type ProjectTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger

	projectId     string
	paymentMethod *billing.PaymentMethod
}

func Test_Project(t *testing.T) {
	suite.Run(t, new(ProjectTestSuite))
}

func (suite *ProjectTestSuite) SetupTest() {
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

	suite.log, err = zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	rub := &billing.Currency{
		CodeInt:  643,
		CodeA3:   "RUB",
		Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCurrency).Insert(rub)

	if err != nil {
		suite.FailNow("Insert currency test data failed", "%v", err)
	}

	rate := &billing.CurrencyRate{
		CurrencyFrom: 643,
		CurrencyTo:   840,
		Rate:         64,
		Date:         ptypes.TimestampNow(),
		IsActive:     true,
	}

	err = db.Collection(pkg.CollectionCurrencyRate).Insert(rate)

	if err != nil {
		suite.FailNow("Insert rates test data failed", "%v", err)
	}

	project := &billing.Project{
		Id:                 bson.NewObjectId().Hex(),
		MerchantId:         bson.NewObjectId().Hex(),
		CallbackCurrency:   rub.CodeA3,
		CallbackProtocol:   "default",
		LimitsCurrency:     rub.CodeA3,
		MaxPaymentAmount:   15000,
		MinPaymentAmount:   0,
		Name:               map[string]string{"en": "test project 1"},
		IsProductsCheckout: true,
		SecretKey:          "test project 1 secret key",
		Status:             pkg.ProjectStatusInProduction,
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

	merchantAgreement := &billing.Merchant{
		Id:      bson.NewObjectId().Hex(),
		Name:    "Unit test status Agreement",
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
		Status:                    pkg.MerchantStatusAgreementRequested,
		LastPayout: &billing.MerchantLastPayout{
			Date:   date,
			Amount: 10000,
		},
		IsSigned: true,
	}
	merchant1 := &billing.Merchant{
		Id:      bson.NewObjectId().Hex(),
		Name:    "merchant1",
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
			Amount: 100000,
		},
		IsSigned: false,
	}

	err = db.Collection(pkg.CollectionMerchant).Insert([]interface{}{merchant, merchantAgreement, merchant1}...)

	if err != nil {
		suite.FailNow("Insert merchant test data failed", "%v", err)
	}

	suite.projectId = project.Id

	suite.service = NewBillingService(db, cfg, make(chan bool, 1), nil, nil, nil, nil, nil)
	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.paymentMethod = pmBankCard
}

func (suite *ProjectTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *ProjectTestSuite) TestProject_GetProjectByIdOk() {
	project, err := suite.service.GetProjectById(suite.projectId)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), project)
	assert.Equal(suite.T(), suite.projectId, project.Id)
}

func (suite *ProjectTestSuite) TestProject_GetProjectByIdError() {
	project, err := suite.service.GetProjectById(bson.NewObjectId().Hex())

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), project)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionProject), err.Error())
}

func (suite *ProjectTestSuite) TestProject_GetGetPaymentMethodByGroupAndCurrencyOk() {
	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(suite.paymentMethod.Group, 643)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)
	assert.Equal(suite.T(), suite.paymentMethod.Id, pm.Id)
	assert.Equal(suite.T(), suite.paymentMethod.Group, pm.Group)
}

func (suite *ProjectTestSuite) TestProject_GetGetPaymentMethodByGroupAndCurrency_GroupError() {
	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency("group_from_my_head", 643)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), pm)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionPaymentMethod), err.Error())
}

func (suite *ProjectTestSuite) TestProject_GetGetPaymentMethodByGroupAndCurrency_CurrencyError() {
	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(suite.paymentMethod.Group, 960)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), pm)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionPaymentMethod), err.Error())
}
