package service

import (
	"context"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
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

	pmBankCard *billing.PaymentMethod
	pmQiwi     *billing.PaymentMethod
}

func Test_Onboarding(t *testing.T) {
	suite.Run(t, new(OnboardingTestSuite))
}

func (suite *OnboardingTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	cfg.AccountingCurrency = "RUB"
	cfg.CardPayApiUrl = "https://sandbox.cardpay.com"

	assert.NoError(suite.T(), err, "Config load failed")

	settings := database.Connection{
		Host:     cfg.MongoHost,
		Database: cfg.MongoDatabase,
		User:     cfg.MongoUser,
		Password: cfg.MongoPassword,
	}

	db, err := database.NewDatabase(settings)
	assert.NoError(suite.T(), err, "Database connection failed")

	rub := &billing.Currency{
		CodeInt:  643,
		CodeA3:   "RUB",
		Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCurrency).Insert(rub)
	assert.NoError(suite.T(), err, "Insert currency test data failed")

	rate := &billing.CurrencyRate{
		CurrencyFrom: 643,
		CurrencyTo:   840,
		Rate:         64,
		Date:         ptypes.TimestampNow(),
		IsActive:     true,
	}

	err = db.Collection(pkg.CollectionCurrencyRate).Insert(rate)
	assert.NoError(suite.T(), err, "Insert rates test data failed")

	country := &billing.Country{
		CodeInt:  643,
		CodeA2:   "RU",
		CodeA3:   "RUS",
		Name:     &billing.Name{Ru: "Россия", En: "Russia (Russian Federation)"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCountry).Insert(country)
	assert.NoError(suite.T(), err, "Insert country test data failed")

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

	pmQiwi := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "QIWI",
		Group:            "QIWI",
		MinPaymentAmount: 100,
		MaxPaymentAmount: 15000,
		Currency:         rub,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:          "cardpay",
			Terminal:         "15985",
			Password:         "A1tph4I6BD0f",
			CallbackPassword: "0V1rJ7t4jCRv",
			ExternalId:       "QIWI",
		},
		Type:     "ewallet",
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
	assert.NoError(suite.T(), err, "Generate merchant date failed")

	merchant := &billing.Merchant{
		Id: bson.NewObjectId().Hex(),
		User: &billing.MerchantUser{
			Id:    uuid.New().String(),
			Email: "test@unit.test",
		},
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

	date, err = ptypes.TimestampProto(time.Now().Add(time.Hour * -360))
	assert.NoError(suite.T(), err, "Generate merchant date failed")

	merchantAgreement := &billing.Merchant{
		Id: bson.NewObjectId().Hex(),
		User: &billing.MerchantUser{
			Id:    uuid.New().String(),
			Email: "test_agreement@unit.test",
		},
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
		Id: bson.NewObjectId().Hex(),
		User: &billing.MerchantUser{
			Id:    uuid.New().String(),
			Email: "test_merchant1@unit.test",
		},
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
	assert.NoError(suite.T(), err, "Insert merchant test data failed")

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
	assert.NoError(suite.T(), err, "Insert project test data failed")

	err = db.Collection(pkg.CollectionPaymentMethod).Insert([]interface{}{pmBankCard, pmQiwi}...)
	assert.NoError(suite.T(), err, "Insert payment methods test data failed")

	commissionStartDate, err := ptypes.TimestampProto(time.Now().Add(time.Minute * -10))
	assert.NoError(suite.T(), err, "Commission start date conversion failed")

	commission := &billing.Commission{
		PaymentMethodId:         pmBankCard.Id,
		ProjectId:               project.Id,
		PaymentMethodCommission: 1,
		PspCommission:           2,
		TotalCommissionToUser:   1,
		StartDate:               commissionStartDate,
	}

	err = db.Collection(pkg.CollectionCommission).Insert(commission)
	assert.NoError(suite.T(), err, "Insert commission test data failed")

	suite.log, err = zap.NewProduction()
	assert.NoError(suite.T(), err, "Logger initialization failed")

	suite.service = NewBillingService(db, cfg, make(chan bool, 1), nil, nil, nil, nil)
	err = suite.service.Init()
	assert.NoError(suite.T(), err, "Billing service initialization failed")

	suite.merchant = merchant
	suite.merchantAgreement = merchantAgreement
	suite.merchant1 = merchant1

	suite.pmBankCard = pmBankCard
	suite.pmQiwi = pmQiwi
}

func (suite *OnboardingTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchant_NewMerchant_Ok() {
	var merchant *billing.Merchant

	req := &grpc.OnboardingRequest{
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
	assert.True(suite.T(), len(rsp.Id) > 0)
	assert.Equal(suite.T(), pkg.MerchantStatusDraft, rsp.Status)
	assert.Equal(suite.T(), req.Website, rsp.Website)
	assert.Equal(suite.T(), req.Contacts.Authorized.Position, rsp.Contacts.Authorized.Position)
	assert.Equal(suite.T(), req.Banking.Name, rsp.Banking.Name)

	err = suite.service.db.Collection(pkg.CollectionMerchant).Find(bson.M{"_id": bson.ObjectIdHex(rsp.Id)}).One(&merchant)

	assert.NotNil(suite.T(), merchant)
	assert.Equal(suite.T(), rsp.Status, merchant.Status)
	assert.Equal(suite.T(), rsp.Contacts.Authorized.Position, merchant.Contacts.Authorized.Position)
	assert.Equal(suite.T(), rsp.Banking.Name, merchant.Banking.Name)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchant_UpdateMerchant_Ok() {
	req := &grpc.OnboardingRequest{
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
	assert.Equal(suite.T(), req.Website, rsp.Website)
	assert.Equal(suite.T(), req.Contacts.Authorized.Phone, rsp.Contacts.Authorized.Phone)
	assert.Equal(suite.T(), req.Banking.AccountNumber, rsp.Banking.AccountNumber)

	var merchant *billing.Merchant
	err = suite.service.db.Collection(pkg.CollectionMerchant).Find(bson.M{"_id": bson.ObjectIdHex(rsp.Id)}).One(&merchant)

	assert.NotNil(suite.T(), merchant)
	assert.Equal(suite.T(), rsp.Status, merchant.Status)
	assert.Equal(suite.T(), rsp.Contacts.Authorized.Phone, merchant.Contacts.Authorized.Phone)
	assert.Equal(suite.T(), rsp.Banking.AccountNumber, merchant.Banking.AccountNumber)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchant_UpdateMerchantNotAllowed_Error() {
	req := &grpc.OnboardingRequest{
		Id: suite.merchantAgreement.Id,
		User: &billing.MerchantUser{
			Id:    bson.NewObjectId().Hex(),
			Email: "test@unit.test",
		},
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

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantById_MerchantId_Ok() {
	req := &grpc.GetMerchantByRequest{
		MerchantId: suite.merchant.Id,
	}

	rsp := &grpc.MerchantGetMerchantResponse{}
	err := suite.service.GetMerchantBy(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.True(suite.T(), len(rsp.Item.Id) > 0)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Item.Id)
	assert.Equal(suite.T(), suite.merchant.Website, rsp.Item.Website)
	assert.Equal(suite.T(), suite.merchant.Name, rsp.Item.Name)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantById_UserId_Ok() {
	req := &grpc.GetMerchantByRequest{
		UserId: suite.merchant.User.Id,
	}

	rsp := &grpc.MerchantGetMerchantResponse{}
	err := suite.service.GetMerchantBy(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.True(suite.T(), len(rsp.Item.Id) > 0)
	assert.Equal(suite.T(), suite.merchant.Id, rsp.Item.Id)
	assert.Equal(suite.T(), suite.merchant.Website, rsp.Item.Website)
	assert.Equal(suite.T(), suite.merchant.Name, rsp.Item.Name)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantById_Error() {
	req := &grpc.GetMerchantByRequest{
		MerchantId: bson.NewObjectId().Hex(),
	}

	rsp := &grpc.MerchantGetMerchantResponse{}
	err := suite.service.GetMerchantBy(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusNotFound, rsp.Status)
	assert.Equal(suite.T(), merchantErrorNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantBy_IncorrectRequest_Error() {
	req := &grpc.GetMerchantByRequest{}
	rsp := &grpc.MerchantGetMerchantResponse{}
	err := suite.service.GetMerchantBy(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), merchantErrorBadData, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
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

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_QuickSearchQuery_Ok() {
	req := &grpc.MerchantListingRequest{
		QuickSearch: "test_agreement",
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 1)
	assert.Equal(suite.T(), suite.merchantAgreement.Id, rsp.Merchants[0].Id)
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
		IsSigned: 1,
	}
	rsp := &grpc.Merchants{}

	err := suite.service.ListMerchants(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.Merchants, 1)
	assert.Equal(suite.T(), suite.merchant1.Id, rsp.Merchants[0].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchants_IsAgreementTrueQuery_Ok() {
	req := &grpc.MerchantListingRequest{
		IsSigned: 2,
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusAgreementRequested,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.MerchantStatusAgreementRequested, rspChangeStatus.Status)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_DraftToDraft_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusDraft,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorStatusDraft, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementRequested_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusAgreementRequested,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorAgreementRequested, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_OnReview_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusOnReview,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorOnReview, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_Approved_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusApproved,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorReturnFromReview, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_Rejected_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusRejected,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorReturnFromReview, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigning_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusAgreementSigning,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigning, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_IncorrectBeforeStatus_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigned, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_NotHaveTwoSignature_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigned, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_NotHaveMerchantSignature_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigned, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_NotHavePspSignature_Error() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), merchantErrorSigned, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantStatus_AgreementSigned_Ok() {
	req := &grpc.OnboardingRequest{
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
		MerchantId: rsp.Id,
		Status:     pkg.MerchantStatusAgreementSigned,
	}

	rspChangeStatus := &billing.Merchant{}
	err = suite.service.ChangeMerchantStatus(context.TODO(), reqChangeStatus, rspChangeStatus)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), rspChangeStatus.IsSigned)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchantPaymentMethods_MerchantPaymentMethodsEmpty_Ok() {
	var merchant *billing.Merchant
	err := suite.service.db.Collection(pkg.CollectionMerchant).FindId(bson.ObjectIdHex(suite.merchant1.Id)).One(&merchant)

	assert.NotNil(suite.T(), merchant)
	assert.Len(suite.T(), merchant.PaymentMethods, 0)

	req := &grpc.ListMerchantPaymentMethodsRequest{
		MerchantId: suite.merchant1.Id,
	}
	rsp := &grpc.ListingMerchantPaymentMethod{}
	err = suite.service.ListMerchantPaymentMethods(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.PaymentMethods) > 0)
	assert.Len(suite.T(), rsp.PaymentMethods, len(suite.service.paymentMethodIdCache))

	for _, v := range rsp.PaymentMethods {
		assert.True(suite.T(), v.PaymentMethod.Id != "")
		assert.True(suite.T(), v.PaymentMethod.Name != "")
		assert.True(suite.T(), v.Commission.Fee == 0)
		assert.NotNil(suite.T(), v.Commission.PerTransaction)
		assert.True(suite.T(), v.Commission.PerTransaction.Fee == 0)
		assert.True(suite.T(), v.Commission.PerTransaction.Currency == "")
		assert.True(suite.T(), v.Integration.TerminalId == "")
		assert.True(suite.T(), v.Integration.TerminalPassword == "")
		assert.False(suite.T(), v.Integration.Integrated)
		assert.True(suite.T(), v.IsActive)
	}
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchantPaymentMethods_ExistMerchantHasPaymentMethod_Ok() {
	req := &grpc.ListMerchantPaymentMethodsRequest{
		MerchantId: suite.merchant.Id,
	}
	rsp := &grpc.ListingMerchantPaymentMethod{}
	err := suite.service.ListMerchantPaymentMethods(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.PaymentMethods) > 0)
	assert.Len(suite.T(), rsp.PaymentMethods, len(suite.service.paymentMethodIdCache))

	_, ok := suite.service.merchantPaymentMethods[suite.merchant.Id]
	assert.True(suite.T(), ok)

	for _, v := range rsp.PaymentMethods {
		if v.PaymentMethod.Id != suite.pmBankCard.Id {
			continue
		}

		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].PaymentMethod.Id, v.PaymentMethod.Id)
		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].PaymentMethod.Name, v.PaymentMethod.Name)
		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].Commission.Fee, v.Commission.Fee)
		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].Commission.PerTransaction.Fee, v.Commission.PerTransaction.Fee)
		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].Commission.PerTransaction.Currency, v.Commission.PerTransaction.Currency)
		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].Integration.TerminalId, v.Integration.TerminalId)
		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].Integration.TerminalPassword, v.Integration.TerminalPassword)
		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].Integration.Integrated, v.Integration.Integrated)
		assert.Equal(suite.T(), suite.merchant.PaymentMethods[suite.pmBankCard.Id].IsActive, v.IsActive)
	}
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchantPaymentMethods_NewMerchant_Ok() {
	req := &grpc.OnboardingRequest{
		Name:               "New merchant unit test",
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
	assert.Nil(suite.T(), rsp.PaymentMethods)

	reqListMerchantPaymentMethods := &grpc.ListMerchantPaymentMethodsRequest{
		MerchantId: rsp.Id,
	}
	rspListMerchantPaymentMethods := &grpc.ListingMerchantPaymentMethod{}
	err = suite.service.ListMerchantPaymentMethods(context.TODO(), reqListMerchantPaymentMethods, rspListMerchantPaymentMethods)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rspListMerchantPaymentMethods.PaymentMethods) > 0)
	assert.Len(suite.T(), rspListMerchantPaymentMethods.PaymentMethods, len(suite.service.paymentMethodIdCache))

	_, ok := suite.service.merchantPaymentMethods[rsp.Id]
	assert.False(suite.T(), ok)

	for _, v := range rspListMerchantPaymentMethods.PaymentMethods {
		assert.True(suite.T(), v.PaymentMethod.Id != "")
		assert.True(suite.T(), v.PaymentMethod.Name != "")
		assert.True(suite.T(), v.Commission.Fee == 0)
		assert.NotNil(suite.T(), v.Commission.PerTransaction)
		assert.True(suite.T(), v.Commission.PerTransaction.Fee == 0)
		assert.True(suite.T(), v.Commission.PerTransaction.Currency == "")
		assert.True(suite.T(), v.Integration.TerminalId == "")
		assert.True(suite.T(), v.Integration.TerminalPassword == "")
		assert.False(suite.T(), v.Integration.Integrated)
		assert.True(suite.T(), v.IsActive)
	}

	reqMerchantPaymentMethodAdd := &grpc.MerchantPaymentMethodRequest{
		MerchantId: rsp.Id,
		PaymentMethod: &billing.MerchantPaymentMethodIdentification{
			Id:   suite.pmBankCard.Id,
			Name: suite.pmBankCard.Name,
		},
		Commission: &billing.MerchantPaymentMethodCommissions{
			Fee: 5,
			PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
				Fee:      100,
				Currency: "RUB",
			},
		},
		Integration: &billing.MerchantPaymentMethodIntegration{
			TerminalId:       "1234567890",
			TerminalPassword: "0987654321",
			Integrated:       true,
		},
		IsActive: true,
	}
	rspMerchantPaymentMethodAdd := &grpc.MerchantPaymentMethodResponse{}
	err = suite.service.ChangeMerchantPaymentMethod(context.TODO(), reqMerchantPaymentMethodAdd, rspMerchantPaymentMethodAdd)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rspMerchantPaymentMethodAdd.Status)
	assert.NotNil(suite.T(), rspMerchantPaymentMethodAdd.Item)
	assert.True(suite.T(), len(rspMerchantPaymentMethodAdd.Item.PaymentMethod.Id) > 0)

	_, ok = suite.service.merchantPaymentMethods[rsp.Id]
	assert.True(suite.T(), ok)
	assert.True(suite.T(), len(suite.service.merchantPaymentMethods[rsp.Id]) > 0)
	pm, ok := suite.service.merchantPaymentMethods[rsp.Id][suite.pmBankCard.Id]
	assert.True(suite.T(), ok)

	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.PaymentMethod.Id, pm.PaymentMethod.Id)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.PaymentMethod.Name, pm.PaymentMethod.Name)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Commission.Fee, pm.Commission.Fee)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Commission.PerTransaction.Fee, pm.Commission.PerTransaction.Fee)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Commission.PerTransaction.Currency, pm.Commission.PerTransaction.Currency)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Integration.TerminalId, pm.Integration.TerminalId)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Integration.TerminalPassword, pm.Integration.TerminalPassword)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Integration.Integrated, pm.Integration.Integrated)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.IsActive, pm.IsActive)

	var merchant *billing.Merchant
	err = suite.service.db.Collection(pkg.CollectionMerchant).FindId(bson.ObjectIdHex(rsp.Id)).One(&merchant)
	assert.NotNil(suite.T(), merchant)
	assert.True(suite.T(), len(merchant.PaymentMethods) > 0)

	pm1, ok := merchant.PaymentMethods[suite.pmBankCard.Id]
	assert.True(suite.T(), ok)

	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.PaymentMethod.Id, pm1.PaymentMethod.Id)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.PaymentMethod.Name, pm1.PaymentMethod.Name)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Commission.Fee, pm1.Commission.Fee)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Commission.PerTransaction.Fee, pm1.Commission.PerTransaction.Fee)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Commission.PerTransaction.Currency, pm1.Commission.PerTransaction.Currency)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Integration.TerminalId, pm1.Integration.TerminalId)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Integration.TerminalPassword, pm1.Integration.TerminalPassword)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.Integration.Integrated, pm1.Integration.Integrated)
	assert.Equal(suite.T(), reqMerchantPaymentMethodAdd.IsActive, pm1.IsActive)

	assert.Equal(suite.T(), pm.PaymentMethod.Id, pm1.PaymentMethod.Id)
	assert.Equal(suite.T(), pm.PaymentMethod.Name, pm1.PaymentMethod.Name)
	assert.Equal(suite.T(), pm.Commission.Fee, pm1.Commission.Fee)
	assert.Equal(suite.T(), pm.Commission.PerTransaction.Fee, pm1.Commission.PerTransaction.Fee)
	assert.Equal(suite.T(), pm.Commission.PerTransaction.Currency, pm1.Commission.PerTransaction.Currency)
	assert.Equal(suite.T(), pm.Integration.TerminalId, pm1.Integration.TerminalId)
	assert.Equal(suite.T(), pm.Integration.TerminalPassword, pm1.Integration.TerminalPassword)
	assert.Equal(suite.T(), pm.Integration.Integrated, pm1.Integration.Integrated)
	assert.Equal(suite.T(), pm.IsActive, pm1.IsActive)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchantPaymentMethods_PaymentMethodsIsEmpty_Ok() {
	_, err := suite.service.db.Collection(pkg.CollectionPaymentMethod).RemoveAll(bson.M{})

	req := &grpc.ListMerchantPaymentMethodsRequest{
		MerchantId: suite.merchant1.Id,
	}
	rsp := &grpc.ListingMerchantPaymentMethod{}
	err = suite.service.ListMerchantPaymentMethods(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.PaymentMethods, 0)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListMerchantPaymentMethods_Filter_Ok() {
	req := &grpc.ListMerchantPaymentMethodsRequest{
		MerchantId:        suite.merchant.Id,
		PaymentMethodName: "iwi",
	}
	rsp := &grpc.ListingMerchantPaymentMethod{}
	err := suite.service.ListMerchantPaymentMethods(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp.PaymentMethods, 1)

	pm := rsp.PaymentMethods[0]

	assert.Equal(suite.T(), suite.pmQiwi.Id, pm.PaymentMethod.Id)
	assert.Equal(suite.T(), suite.pmQiwi.Name, pm.PaymentMethod.Name)
	assert.Equal(suite.T(), float64(0), pm.Commission.Fee)
	assert.Equal(suite.T(), float64(0), pm.Commission.PerTransaction.Fee)
	assert.Equal(suite.T(), "", pm.Commission.PerTransaction.Currency)
	assert.Equal(suite.T(), "", pm.Integration.TerminalId)
	assert.Equal(suite.T(), "", pm.Integration.TerminalPassword)
	assert.False(suite.T(), pm.Integration.Integrated)
	assert.True(suite.T(), pm.IsActive)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantPaymentMethod_ExistPaymentMethod_Ok() {
	req := &grpc.GetMerchantPaymentMethodRequest{
		MerchantId:      suite.merchant.Id,
		PaymentMethodId: suite.pmBankCard.Id,
	}
	rsp := &billing.MerchantPaymentMethod{}
	err := suite.service.GetMerchantPaymentMethod(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), rsp.PaymentMethod)
	assert.NotNil(suite.T(), rsp.Commission)
	assert.NotNil(suite.T(), rsp.Commission.PerTransaction)
	assert.NotNil(suite.T(), rsp.Integration)
	assert.True(suite.T(), rsp.IsActive)

	pm, ok := suite.merchant.PaymentMethods[suite.pmBankCard.Id]
	assert.True(suite.T(), ok)

	assert.Equal(suite.T(), pm.PaymentMethod.Id, rsp.PaymentMethod.Id)
	assert.Equal(suite.T(), pm.PaymentMethod.Name, rsp.PaymentMethod.Name)
	assert.Equal(suite.T(), pm.Commission.Fee, rsp.Commission.Fee)
	assert.Equal(suite.T(), pm.Commission.PerTransaction.Fee, rsp.Commission.PerTransaction.Fee)
	assert.Equal(suite.T(), pm.Commission.PerTransaction.Currency, rsp.Commission.PerTransaction.Currency)
	assert.Equal(suite.T(), pm.Integration.TerminalId, rsp.Integration.TerminalId)
	assert.Equal(suite.T(), pm.Integration.TerminalPassword, rsp.Integration.TerminalPassword)
	assert.Equal(suite.T(), pm.Integration.Integrated, rsp.Integration.Integrated)
	assert.Equal(suite.T(), pm.IsActive, rsp.IsActive)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantPaymentMethod_NotExistPaymentMethod_Ok() {
	req := &grpc.GetMerchantPaymentMethodRequest{
		MerchantId:      suite.merchant.Id,
		PaymentMethodId: suite.pmQiwi.Id,
	}
	rsp := &billing.MerchantPaymentMethod{}
	err := suite.service.GetMerchantPaymentMethod(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), rsp.PaymentMethod)
	assert.NotNil(suite.T(), rsp.Commission)
	assert.NotNil(suite.T(), rsp.Commission.PerTransaction)
	assert.NotNil(suite.T(), rsp.Integration)
	assert.True(suite.T(), rsp.IsActive)

	assert.Equal(suite.T(), suite.pmQiwi.Id, rsp.PaymentMethod.Id)
	assert.Equal(suite.T(), suite.pmQiwi.Name, rsp.PaymentMethod.Name)
	assert.Equal(suite.T(), float64(0), rsp.Commission.Fee)
	assert.Equal(suite.T(), float64(0), rsp.Commission.PerTransaction.Fee)
	assert.Equal(suite.T(), "", rsp.Commission.PerTransaction.Currency)
	assert.Equal(suite.T(), "", rsp.Integration.TerminalId)
	assert.Equal(suite.T(), "", rsp.Integration.TerminalPassword)
	assert.False(suite.T(), rsp.Integration.Integrated)
	assert.True(suite.T(), rsp.IsActive)
}

func (suite *OnboardingTestSuite) TestOnboarding_GetMerchantPaymentMethod_Error() {
	req := &grpc.GetMerchantPaymentMethodRequest{
		MerchantId:      suite.merchant.Id,
		PaymentMethodId: bson.NewObjectId().Hex(),
	}
	rsp := &billing.MerchantPaymentMethod{}
	err := suite.service.GetMerchantPaymentMethod(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotFound, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantPaymentMethod_PaymentMethodNotFound_Error() {
	req := &grpc.MerchantPaymentMethodRequest{
		MerchantId: suite.merchant.Id,
		PaymentMethod: &billing.MerchantPaymentMethodIdentification{
			Id:   bson.NewObjectId().Hex(),
			Name: "Unit test",
		},
		Commission: &billing.MerchantPaymentMethodCommissions{
			Fee: 5,
			PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
				Fee:      10,
				Currency: "RUB",
			},
		},
		Integration: &billing.MerchantPaymentMethodIntegration{
			TerminalId:       "1234567890",
			TerminalPassword: "0987654321",
			Integrated:       true,
		},
		IsActive: true,
	}
	rsp := &grpc.MerchantPaymentMethodResponse{}
	err := suite.service.ChangeMerchantPaymentMethod(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)

}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantPaymentMethod_CurrencyNotFound_Error() {
	req := &grpc.MerchantPaymentMethodRequest{
		MerchantId: suite.merchant.Id,
		PaymentMethod: &billing.MerchantPaymentMethodIdentification{
			Id:   suite.pmBankCard.Id,
			Name: suite.pmBankCard.Name,
		},
		Commission: &billing.MerchantPaymentMethodCommissions{
			Fee: 5,
			PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
				Fee:      10,
				Currency: "USD",
			},
		},
		Integration: &billing.MerchantPaymentMethodIntegration{
			TerminalId:       "1234567890",
			TerminalPassword: "0987654321",
			Integrated:       true,
		},
		IsActive: true,
	}
	rsp := &grpc.MerchantPaymentMethodResponse{}
	err := suite.service.ChangeMerchantPaymentMethod(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorCurrencyNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *OnboardingTestSuite) TestOnboarding_CreateNotification_Ok() {
	var notification *billing.Notification

	userId := bson.NewObjectId().Hex()

	query := bson.M{
		"merchant_id": bson.ObjectIdHex(suite.merchant.Id),
		"user_id":     bson.ObjectIdHex(userId),
	}
	err := suite.service.db.Collection(pkg.CollectionNotification).Find(query).One(&notification)
	assert.Nil(suite.T(), notification)

	req := &grpc.NotificationRequest{
		MerchantId: suite.merchant.Id,
		UserId:     userId,
		Title:      "Unit test title",
		Message:    "Unit test message",
	}
	rsp := &billing.Notification{}

	err = suite.service.CreateNotification(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)
	assert.Equal(suite.T(), req.MerchantId, rsp.MerchantId)
	assert.Equal(suite.T(), req.UserId, rsp.UserId)
	assert.Equal(suite.T(), req.Title, rsp.Title)
	assert.Equal(suite.T(), req.Message, rsp.Message)

	err = suite.service.db.Collection(pkg.CollectionNotification).Find(query).One(&notification)
	assert.NotNil(suite.T(), notification)
	assert.Equal(suite.T(), rsp.Id, notification.Id)
	assert.Equal(suite.T(), rsp.MerchantId, notification.MerchantId)
	assert.Equal(suite.T(), rsp.UserId, notification.UserId)
	assert.Equal(suite.T(), rsp.Title, notification.Title)
	assert.Equal(suite.T(), rsp.Message, notification.Message)
}

func (suite *OnboardingTestSuite) TestOnboarding_CreateNotification_UserIdEmpty_Error() {
	req := &grpc.NotificationRequest{
		MerchantId: suite.merchant.Id,
		Title:      "Unit test title",
		Message:    "Unit test message",
	}
	rsp := &billing.Notification{}

	err := suite.service.CreateNotification(context.TODO(), req, rsp)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), notificationErrorUserIdIncorrect, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_CreateNotification_MessageEmpty_Error() {
	req := &grpc.NotificationRequest{
		MerchantId: suite.merchant.Id,
		UserId:     bson.NewObjectId().Hex(),
		Title:      "Unit test title",
	}
	rsp := &billing.Notification{}

	err := suite.service.CreateNotification(context.TODO(), req, rsp)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), notificationErrorMessageIsEmpty, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_CreateNotification_AddNotification_Error() {
	req := &grpc.NotificationRequest{
		MerchantId: "bad_bson_id",
		UserId:     bson.NewObjectId().Hex(),
		Title:      "Unit test title",
		Message:    "Unit test message",
	}
	rsp := &billing.Notification{}

	err := suite.service.CreateNotification(context.TODO(), req, rsp)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), notificationErrorMerchantIdIncorrect, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_GetNotification_Ok() {
	req := &grpc.NotificationRequest{
		MerchantId: suite.merchant.Id,
		UserId:     bson.NewObjectId().Hex(),
		Title:      "Unit test title",
		Message:    "Unit test message",
	}
	rsp := &billing.Notification{}

	err := suite.service.CreateNotification(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	reqGetNotification := &grpc.GetNotificationRequest{
		MerchantId:     suite.merchant.Id,
		NotificationId: rsp.Id,
	}
	rspGetNotification := &billing.Notification{}
	err = suite.service.GetNotification(context.TODO(), reqGetNotification, rspGetNotification)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), rsp.Id, rspGetNotification.Id)
	assert.Equal(suite.T(), rsp.MerchantId, rspGetNotification.MerchantId)
	assert.Equal(suite.T(), rsp.UserId, rspGetNotification.UserId)
	assert.Equal(suite.T(), rsp.Title, rspGetNotification.Title)
	assert.Equal(suite.T(), rsp.Message, rspGetNotification.Message)
	assert.NotNil(suite.T(), rspGetNotification.CreatedAt)
	assert.NotNil(suite.T(), rspGetNotification.UpdatedAt)
}

func (suite *OnboardingTestSuite) TestOnboarding_NotFound_Error() {
	reqGetNotification := &grpc.GetNotificationRequest{
		MerchantId:     bson.NewObjectId().Hex(),
		NotificationId: bson.NewObjectId().Hex(),
	}
	rspGetNotification := &billing.Notification{}
	err := suite.service.GetNotification(context.TODO(), reqGetNotification, rspGetNotification)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), notificationErrorNotFound, err.Error())
}

func (suite *OnboardingTestSuite) TestOnboarding_ListNotifications_Merchant_Ok() {
	req1 := &grpc.NotificationRequest{
		MerchantId: suite.merchant.Id,
		UserId:     bson.NewObjectId().Hex(),
		Title:      "Unit test title 1",
		Message:    "Unit test message 1",
	}
	rsp1 := &billing.Notification{}

	err := suite.service.CreateNotification(context.TODO(), req1, rsp1)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp1.Id) > 0)

	req2 := &grpc.NotificationRequest{
		MerchantId: suite.merchant.Id,
		UserId:     bson.NewObjectId().Hex(),
		Title:      "Unit test title 1",
		Message:    "Unit test message 1",
	}
	rsp2 := &billing.Notification{}

	err = suite.service.CreateNotification(context.TODO(), req2, rsp2)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp1.Id) > 0)

	req3 := &grpc.ListingNotificationRequest{
		MerchantId: suite.merchant.Id,
		Limit:      10,
		Offset:     0,
	}
	rsp3 := &grpc.Notifications{}
	err = suite.service.ListNotifications(context.TODO(), req3, rsp3)
	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp3.Notifications, 2)
	assert.Equal(suite.T(), rsp1.Id, rsp3.Notifications[0].Id)
	assert.Equal(suite.T(), rsp2.Id, rsp3.Notifications[1].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_ListNotifications_User_Ok() {
	userId := bson.NewObjectId().Hex()

	req1 := &grpc.NotificationRequest{
		MerchantId: bson.NewObjectId().Hex(),
		UserId:     userId,
		Title:      "Unit test title 1",
		Message:    "Unit test message 1",
	}
	rsp1 := &billing.Notification{}

	err := suite.service.CreateNotification(context.TODO(), req1, rsp1)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp1.Id) > 0)

	req2 := &grpc.NotificationRequest{
		MerchantId: bson.NewObjectId().Hex(),
		UserId:     userId,
		Title:      "Unit test title 2",
		Message:    "Unit test message 2",
	}
	rsp2 := &billing.Notification{}

	err = suite.service.CreateNotification(context.TODO(), req2, rsp2)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp1.Id) > 0)

	req3 := &grpc.NotificationRequest{
		MerchantId: bson.NewObjectId().Hex(),
		UserId:     userId,
		Title:      "Unit test title 3",
		Message:    "Unit test message 3",
	}
	rsp3 := &billing.Notification{}

	err = suite.service.CreateNotification(context.TODO(), req3, rsp3)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp1.Id) > 0)

	req4 := &grpc.ListingNotificationRequest{
		UserId: userId,
		Limit:  10,
		Offset: 0,
	}
	rsp4 := &grpc.Notifications{}
	err = suite.service.ListNotifications(context.TODO(), req4, rsp4)
	assert.Nil(suite.T(), err)
	assert.Len(suite.T(), rsp4.Notifications, 3)
	assert.Equal(suite.T(), rsp1.Id, rsp4.Notifications[0].Id)
	assert.Equal(suite.T(), rsp2.Id, rsp4.Notifications[1].Id)
	assert.Equal(suite.T(), rsp3.Id, rsp4.Notifications[2].Id)
}

func (suite *OnboardingTestSuite) TestOnboarding_MarkNotificationAsRead_Ok() {
	req1 := &grpc.NotificationRequest{
		MerchantId: bson.NewObjectId().Hex(),
		UserId:     bson.NewObjectId().Hex(),
		Title:      "Unit test title 1",
		Message:    "Unit test message 1",
	}
	rsp1 := &billing.Notification{}

	err := suite.service.CreateNotification(context.TODO(), req1, rsp1)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp1.Id) > 0)
	assert.False(suite.T(), rsp1.IsRead)

	req2 := &grpc.GetNotificationRequest{
		MerchantId:     req1.MerchantId,
		NotificationId: rsp1.Id,
	}
	rsp2 := &billing.Notification{}
	err = suite.service.MarkNotificationAsRead(context.TODO(), req2, rsp2)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), rsp2.IsRead)
	assert.Equal(suite.T(), rsp1.Id, rsp2.Id)

	var notification *billing.Notification
	err = suite.service.db.Collection(pkg.CollectionNotification).FindId(bson.ObjectIdHex(rsp1.Id)).One(&notification)
	assert.NotNil(suite.T(), notification)

	assert.True(suite.T(), notification.IsRead)
}

func (suite *OnboardingTestSuite) TestOnboarding_MarkNotificationAsRead_NotFound_Error() {
	req1 := &grpc.NotificationRequest{
		MerchantId: bson.NewObjectId().Hex(),
		UserId:     bson.NewObjectId().Hex(),
		Title:      "Unit test title 1",
		Message:    "Unit test message 1",
	}
	rsp1 := &billing.Notification{}

	err := suite.service.CreateNotification(context.TODO(), req1, rsp1)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp1.Id) > 0)
	assert.False(suite.T(), rsp1.IsRead)

	req2 := &grpc.GetNotificationRequest{
		MerchantId:     bson.NewObjectId().Hex(),
		NotificationId: bson.NewObjectId().Hex(),
	}
	rsp2 := &billing.Notification{}
	err = suite.service.MarkNotificationAsRead(context.TODO(), req2, rsp2)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), notificationErrorNotFound, err.Error())
	assert.False(suite.T(), rsp2.IsRead)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantAgreementType_Ok() {
	req := &grpc.OnboardingRequest{
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

	merchant, err := suite.service.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(rsp.Id)})
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), merchant)

	merchant.Status = pkg.MerchantStatusApproved
	err = suite.service.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)
	assert.NoError(suite.T(), err)

	req1 := &grpc.ChangeMerchantAgreementTypeRequest{
		MerchantId:    merchant.Id,
		AgreementType: 1,
	}
	rsp1 := &grpc.ChangeMerchantAgreementTypeResponse{}
	err = suite.service.ChangeMerchantAgreementType(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.Equal(suite.T(), req1.MerchantId, rsp1.Item.Id)
	assert.Equal(suite.T(), req1.AgreementType, rsp1.Item.AgreementType)
	assert.Equal(suite.T(), pkg.MerchantStatusAgreementSigning, rsp1.Item.Status)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantAgreementType_MerchantNotFound_Error() {
	req1 := &grpc.ChangeMerchantAgreementTypeRequest{
		MerchantId:    bson.NewObjectId().Hex(),
		AgreementType: 1,
	}
	rsp1 := &grpc.ChangeMerchantAgreementTypeResponse{}
	err := suite.service.ChangeMerchantAgreementType(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusNotFound, rsp1.Status)
	assert.Equal(suite.T(), merchantErrorNotFound, rsp1.Message)
	assert.Nil(suite.T(), rsp1.Item)
}

func (suite *OnboardingTestSuite) TestOnboarding_ChangeMerchantAgreementType_StatusNotAllowed_Error() {
	req := &grpc.OnboardingRequest{
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

	req1 := &grpc.ChangeMerchantAgreementTypeRequest{
		MerchantId:    rsp.Id,
		AgreementType: 2,
	}
	rsp1 := &grpc.ChangeMerchantAgreementTypeResponse{}
	err = suite.service.ChangeMerchantAgreementType(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), merchantErrorAgreementTypeSelectNotAllow, rsp1.Message)
	assert.Nil(suite.T(), rsp1.Item)
}
