package service

import (
	"context"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2"
	"testing"
	"time"
)

type OnboardingTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger

	merchant          *billing.Merchant
	merchantAgreement *billing.Merchant
	merchant1         *billing.Merchant
}

func Test_Onboarding(t *testing.T) {
	suite.Run(t, new(OnboardingTestSuite))
}

func (suite *OnboardingTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	cfg.AccountingCurrency = "RUB"
	cfg.CardPayApiUrl = "https://sandbox.cardpay.com"

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

	vat := &billing.Vat{
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
	}

	err = db.Collection(pkg.CollectionVat).Insert(vat)

	if err != nil {
		suite.FailNow("Insert VAT test data failed", "%v", err)
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

	pmBankCard := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "Bank card",
		Group:            "BANKCARD",
		MinPaymentAmount: 100,
		MaxPaymentAmount: 15000,
		Currency:         rub,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:          "cardpay",
			Terminal:         "15985",
			Password:         "A1tph4I6BD0f",
			CallbackPassword: "0V1rJ7t4jCRv",
			ExternalId:       "BANKCARD",
		},
		Type:     "bank_card",
		IsActive: true,
		PaymentSystem: &billing.PaymentSystem{
			Id:                 bson.NewObjectId().Hex(),
			Name:               "CardPay",
			AccountingCurrency: rub,
			AccountingPeriod:   "every-day",
			Country:            &billing.Country{},
			IsActive:           true,
		},
	}

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -480))

	if err != nil {
		suite.FailNow("Generate merchant date failed", "%v", err)
	}

	merchant := &billing.Merchant{
		Id:           bson.NewObjectId().Hex(),
		ExternalId:   bson.NewObjectId().Hex(),
		AccountEmail: "test@unit.test",
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
		IsAgreement: true,
	}

	date, err = ptypes.TimestampProto(time.Now().Add(time.Hour * -360))

	if err != nil {
		suite.FailNow("Generate merchant date failed", "%v", err)
	}

	merchantAgreement := &billing.Merchant{
		Id:           bson.NewObjectId().Hex(),
		ExternalId:   bson.NewObjectId().Hex(),
		AccountEmail: "test@unit.test",
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
		IsAgreement: true,
	}
	merchant1 := &billing.Merchant{
		Id:           bson.NewObjectId().Hex(),
		ExternalId:   bson.NewObjectId().Hex(),
		AccountEmail: "test@unit.test",
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
		IsAgreement: false,
	}

	err = db.Collection(pkg.CollectionMerchant).Insert([]interface{}{merchant, merchantAgreement, merchant1}...)

	if err != nil {
		suite.FailNow("Insert merchant test data failed", "%v", err)
	}

	project := &billing.Project{
		Id:                       bson.NewObjectId().Hex(),
		CallbackCurrency:         rub,
		CallbackProtocol:         "default",
		LimitsCurrency:           rub,
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     "test project 1",
		OnlyFixedAmounts:         true,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		PaymentMethods: map[string]*billing.ProjectPaymentMethod{
			"BANKCARD": {
				Id:        pmBankCard.Id,
				Terminal:  "terminal",
				Password:  "password",
				CreatedAt: ptypes.TimestampNow(),
			},
		},
		IsActive: true,
		Merchant: merchant,
	}

	err = db.Collection(pkg.CollectionProject).Insert(project)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
	}

	err = db.Collection(pkg.CollectionPaymentMethod).Insert(pmBankCard)

	if err != nil {
		suite.FailNow("Insert payment methods test data failed", "%v", err)
	}

	commissionStartDate, err := ptypes.TimestampProto(time.Now().Add(time.Minute * -10))

	if err != nil {
		suite.FailNow("Commission start date conversion failed", "%v", err)
	}

	commission := &billing.Commission{
		PaymentMethodId:         pmBankCard.Id,
		ProjectId:               project.Id,
		PaymentMethodCommission: 1,
		PspCommission:           2,
		TotalCommissionToUser:   1,
		StartDate:               commissionStartDate,
	}

	err = db.Collection(pkg.CollectionCommission).Insert(commission)

	if err != nil {
		suite.FailNow("Insert commission test data failed", "%v", err)
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

	suite.merchant = merchant
	suite.merchantAgreement = merchantAgreement
	suite.merchant1 = merchant1
}

func (suite *OnboardingTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchant_NewMerchant_Ok() {
	var merchant *billing.Merchant

	id := bson.NewObjectId().Hex()
	err := suite.service.db.Collection(pkg.CollectionMerchant).Find(bson.M{"external_id": id}).One(&merchant)

	assert.Equal(suite.T(), mgo.ErrNotFound, err)
	assert.Nil(suite.T(), merchant)

	req := &grpc.OnboardingRequest{
		ExternalId:         id,
		AccountingEmail:    "test@unit.test",
		Name:               "Unit test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err = suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)
	assert.Equal(suite.T(), req.ExternalId, rsp.ExternalId)
	assert.Equal(suite.T(), req.Contacts.Authorized.Position, rsp.Contacts.Authorized.Position)
	assert.Equal(suite.T(), req.Banking.Name, rsp.Banking.Name)

	err = suite.service.db.Collection(pkg.CollectionMerchant).Find(bson.M{"external_id": id}).One(&merchant)

	assert.NotNil(suite.T(), merchant)
	assert.Equal(suite.T(), rsp.Status, merchant.Status)
	assert.Equal(suite.T(), rsp.Contacts.Authorized.Position, merchant.Contacts.Authorized.Position)
	assert.Equal(suite.T(), rsp.Banking.Name, merchant.Banking.Name)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchant_UpdateMerchant_Ok() {
	req := &grpc.OnboardingRequest{
		ExternalId:         suite.merchant.ExternalId,
		AccountingEmail:    "test@unit.test",
		Name:               "Unit test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "Moscow",
		Zip:                "190000",
		City:               "Moscow",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "0987654321",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "0987654321",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "0987654321",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)
	assert.Equal(suite.T(), req.ExternalId, rsp.ExternalId)
	assert.Equal(suite.T(), req.Contacts.Authorized.Phone, rsp.Contacts.Authorized.Phone)
	assert.Equal(suite.T(), req.Banking.AccountNumber, rsp.Banking.AccountNumber)

	var merchant *billing.Merchant
	err = suite.service.db.Collection(pkg.CollectionMerchant).Find(bson.M{"external_id": req.ExternalId}).One(&merchant)

	assert.NotNil(suite.T(), merchant)
	assert.Equal(suite.T(), rsp.Status, merchant.Status)
	assert.Equal(suite.T(), rsp.Contacts.Authorized.Phone, merchant.Contacts.Authorized.Phone)
	assert.Equal(suite.T(), rsp.Banking.AccountNumber, merchant.Banking.AccountNumber)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchant_UpdateMerchantNotAllowed_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         suite.merchantAgreement.ExternalId,
		AccountingEmail:    "test@unit.test",
		Name:               "Unit test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "Moscow",
		Zip:                "190000",
		City:               "Moscow",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "0987654321",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "0987654321",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "0987654321",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorChangeNotAllowed, err.Error())
	assert.Len(suite.T(), rsp.Id, 0)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchant_CreateMerchant_CountryNotFound_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Unit test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "US",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorCountryNotFound, err.Error())
	assert.Len(suite.T(), rsp.Id, 0)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchant_CreateMerchant_CurrencyNotFound_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Unit test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "USD",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorCurrencyNotFound, err.Error())
	assert.Len(suite.T(), rsp.Id, 0)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantById_Ok() {
	req := &grpc.FindByIdRequest{
		Id: suite.merchant.Id,
	}

	rsp := &billing.Merchant{}
	err := suite.service.GetMerchantById(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Id)
	assert.Equal(suite.T(), suite.merchant.ExternalId, rsp.ExternalId)
	assert.Equal(suite.T(), suite.merchant.CompanyName, rsp.CompanyName)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantById_Error() {
	req := &grpc.FindByIdRequest{
		Id: bson.NewObjectId().Hex(),
	}

	rsp := &billing.Merchant{}
	err := suite.service.GetMerchantById(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Len(suite.T(), rsp.Id, 0)
	assert.Equal(suite.T(), ErrMerchantNotFound, err)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantByExternalId_Ok() {
	req := &grpc.FindByIdRequest{
		Id: suite.merchant.ExternalId,
	}

	rsp := &billing.Merchant{}
	err := suite.service.GetMerchantByExternalId(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Id)
	assert.Equal(suite.T(), suite.merchant.ExternalId, rsp.ExternalId)
	assert.Equal(suite.T(), suite.merchant.CompanyName, rsp.CompanyName)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantByExternalId_Error() {
	req := &grpc.FindByIdRequest{
		Id: bson.NewObjectId().Hex(),
	}

	rsp := &billing.Merchant{}
	err := suite.service.GetMerchantByExternalId(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Len(suite.T(), rsp.Id, 0)
	assert.Equal(suite.T(), ErrMerchantNotFound, err)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_EmptyQuery_Ok() {
	req := &grpc.MerchantListingRequest{}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 3)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_NameQuery_Ok() {
	req := &grpc.MerchantListingRequest{
		Name: "test",
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 2)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_PayoutDateFromQuery_Ok() {
	date := time.Now().Add(time.Hour * -450)

	req := &grpc.MerchantListingRequest{
		LastPayoutDateFrom: date.Unix(),
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 2)
	assert.Equal(suite.T(), suite.merchantAgreement.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_PayoutDateToQuery_Ok() {
	date := time.Now()

	req := &grpc.MerchantListingRequest{
		LastPayoutDateTo: date.Unix(),
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 3)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_PayoutDateFromToQuery_Ok() {
	req := &grpc.MerchantListingRequest{
		LastPayoutDateFrom: time.Now().Add(time.Hour * -500).Unix(),
		LastPayoutDateTo:   time.Now().Add(time.Hour * -400).Unix(),
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 1)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_PayoutAmountQuery_Ok() {
	req := &grpc.MerchantListingRequest{
		LastPayoutAmount: 999999,
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 1)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_IsAgreementFalseQuery_Ok() {
	req := &grpc.MerchantListingRequest{
		IsAgreement: 1,
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 1)
	assert.Equal(suite.T(), suite.merchant1.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_IsAgreementTrueQuery_Ok() {
	req := &grpc.MerchantListingRequest{
		IsAgreement: 2,
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 2)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_Limit_Ok() {
	req := &grpc.MerchantListingRequest{
		Limit: 2,
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 2)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_Offset_Ok() {
	req := &grpc.MerchantListingRequest{
		Offset: 1,
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 2)
	assert.Equal(suite.T(), suite.merchantAgreement.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_Sort_Ok() {
	req := &grpc.MerchantListingRequest{
		Limit: 2,
		Sort:  []string{"-_id"},
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 2)
	assert.Equal(suite.T(), suite.merchant1.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_EmptyResult_Ok() {
	req := &grpc.MerchantListingRequest{
		Name: bson.NewObjectId().Hex(),
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 0)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_Ok() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusAgreementRequested,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusAgreementRequested, rspChangeStatus.Status)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_DraftToDraft_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusDraft,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorStatusDraft, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementRequested_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	rsp.Status = pkg.MerchantStatusOnReview
	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusAgreementRequested,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorAgreementRequested, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_OnReview_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	rsp.Status = pkg.MerchantStatusApproved
	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusOnReview,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorOnReview, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_Approved_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusApproved,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorReturnFromReview, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_Rejected_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusRejected,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorReturnFromReview, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigning_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	rsp.Status = pkg.MerchantStatusRejected
	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusAgreementSigning,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigning, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_IncorrectBeforeStatus_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	rsp.Status = pkg.MerchantStatusOnReview
	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigned, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_NotHaveTwoSignature_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	rsp.Status = pkg.MerchantStatusAgreementSigning
	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigned, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_NotHaveMerchantSignature_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	rsp.Status = pkg.MerchantStatusAgreementSigning
	rsp.HasPspSignature = true

	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigned, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_NotHavePspSignature_Error() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	rsp.Status = pkg.MerchantStatusAgreementSigning
	rsp.HasMerchantSignature = true

	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigned, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_Ok() {
	req := &grpc.OnboardingRequest{
		ExternalId:         bson.NewObjectId().Hex(),
		AccountingEmail:    "test@unit.test",
		Name:               "Change status test",
		AlternativeName:    "",
		Website:            "https://unit.test",
		Country:            "RU",
		State:              "St.Petersburg",
		Zip:                "190000",
		City:               "St.Petersburg",
		Address:            "",
		AddressAdditional:  "",
		RegistrationNumber: "",
		TaxId:              "",
		Contacts: &billing.MerchantContact{
			Authorized: &billing.MerchantContactAuthorized{
				Name:     "Unit Test",
				Email:    "test@unit.test",
				Phone:    "1234567890",
				Position: "Unit Test",
			},
			Technical: &billing.MerchantContactTechnical{
				Name:  "Unit Test",
				Email: "test@unit.test",
				Phone: "1234567890",
			},
		},
		Banking: &grpc.OnboardingBanking{
			Currency:      "RUB",
			Name:          "Bank name",
			Address:       "Unknown",
			AccountNumber: "1234567890",
			Swift:         "TEST",
			Details:       "",
		},
	}

	rsp := &billing.Merchant{}
	err := suite.service.ChangeMerchant(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)

	rsp.Status = pkg.MerchantStatusAgreementSigning
	rsp.HasMerchantSignature = true
	rsp.HasPspSignature = true

	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	reqChangeStatus := &grpc.MerchantChangeStatusRequest{
		Id:     rsp.Id,
		Status: pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), rspChangeStatus.IsAgreement)
}