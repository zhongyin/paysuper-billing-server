package service

import (
	"context"
	"github.com/ProtocolONE/rabbitmq/pkg"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mock"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"testing"
	"time"
)

type CustomerTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger

	project *billing.Project
}

func Test_Customer(t *testing.T) {
	suite.Run(t, new(CustomerTestSuite))
}

func (suite *CustomerTestSuite) SetupTest() {
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

	rate := []interface{}{
		&billing.CurrencyRate{
			CurrencyFrom: 643,
			CurrencyTo:   643,
			Rate:         1,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
	}

	err = db.Collection(pkg.CollectionCurrencyRate).Insert(rate...)

	if err != nil {
		suite.FailNow("Insert rates test data failed", "%v", err)
	}

	ru := &billing.Country{
		CodeInt:  643,
		CodeA2:   "RU",
		CodeA3:   "RUS",
		Name:     &billing.Name{Ru: "Россия", En: "Russia (Russian Federation)"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCountry).Insert(ru)
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
		Type:          "bank_card",
		IsActive:      true,
		AccountRegexp: "^(?:4[0-9]{12}(?:[0-9]{3})?|[25][1-7][0-9]{14}|6(?:011|5[0-9][0-9])[0-9]{12}|3[47][0-9]{13}|3(?:0[0-5]|[68][0-9])[0-9]{11}|(?:2131|1800|35\\d{3})\\d{11})$",
		PaymentSystem: &billing.PaymentSystem{
			Id:                 bson.NewObjectId().Hex(),
			Name:               "CardPay",
			AccountingCurrency: rub,
			AccountingPeriod:   "every-day",
			Country:            &billing.Country{},
			IsActive:           true,
		},
	}

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -360))

	if err != nil {
		suite.FailNow("Generate merchant date failed", "%v", err)
	}

	merchant := &billing.Merchant{
		Id:      bson.NewObjectId().Hex(),
		Name:    "Unit test",
		Country: ru,
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
					TerminalId:               "15985",
					TerminalPassword:         "A1tph4I6BD0f",
					TerminalCallbackPassword: "0V1rJ7t4jCRv",
					Integrated:               true,
				},
				IsActive: true,
			},
		},
	}

	err = db.Collection(pkg.CollectionMerchant).Insert(merchant)

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
		IsProductsCheckout:       true,
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
		FixedPackage: map[string]*billing.FixedPackages{
			"RU": {
				FixedPackage: []*billing.FixedPackage{
					{
						Id:       "id_0",
						Name:     "package 0",
						Currency: rub,
						Price:    10,
						IsActive: true,
					},
				},
			},
			"US": {FixedPackage: []*billing.FixedPackage{}},
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

	bin := &BinData{
		Id:                bson.NewObjectId(),
		CardBin:           400000,
		CardBrand:         "MASTERCARD",
		CardType:          "DEBIT",
		CardCategory:      "WORLD",
		BankName:          "ALFA BANK",
		BankCountryName:   "UKRAINE",
		BankCountryCodeA2: "US",
	}

	err = db.Collection(pkg.CollectionBinData).Insert(bin)

	if err != nil {
		suite.FailNow("Insert BIN test data failed", "%v", err)
	}

	suite.log, err = zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	broker, err := rabbitmq.NewBroker(cfg.BrokerAddress)

	if err != nil {
		suite.FailNow("Creating RabbitMQ publisher failed", "%v", err)
	}

	suite.service = NewBillingService(
		db,
		cfg,
		make(chan bool, 1),
		mock.NewGeoIpServiceTestOk(),
		mock.NewRepositoryServiceOk(),
		mock.NewTaxServiceOkMock(),
		broker,
	)
	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.project = project
}

func (suite *CustomerTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *CustomerTestSuite) TestCustomer_ChangeCustomer_Ok() {
	req := &billing.Customer{
		ProjectId:  suite.project.Id,
		ExternalId: bson.NewObjectId().Hex(),
		Email:      "test@unit.test",
		Ip:         "127.0.0.1",
		Locale:     "ru",
		Metadata: map[string]string{
			"field1": "value1",
			"field2": "value2",
		},
	}
	rsp := &grpc.ChangeCustomerResponse{}
	err := suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.Equal(suite.T(), req.ProjectId, rsp.Item.ProjectId)
	assert.Equal(suite.T(), req.ExternalId, rsp.Item.ExternalId)
	assert.Equal(suite.T(), req.Ip, rsp.Item.Ip)
	assert.NotNil(suite.T(), rsp.Item.Address)

	var customer *billing.Customer
	err = suite.service.db.Collection(pkg.CollectionCustomer).
		Find(bson.M{"project_id": bson.ObjectIdHex(req.ProjectId), "external_id": req.ExternalId}).One(&customer)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), rsp.Item.Id, customer.Id)
	assert.Equal(suite.T(), rsp.Item.ProjectId, customer.ProjectId)
	assert.Equal(suite.T(), rsp.Item.ExternalId, customer.ExternalId)
	assert.Equal(suite.T(), rsp.Item.Ip, customer.Ip)
	assert.Equal(suite.T(), rsp.Item.Address, customer.Address)
	assert.NotNil(suite.T(), customer.ExpireAt)
	assert.True(suite.T(), len(customer.Token) > 0)
}

func (suite *CustomerTestSuite) TestCustomer_ChangeCustomer_WithHistory_Ok() {
	req := &billing.Customer{
		ProjectId:  suite.project.Id,
		ExternalId: bson.NewObjectId().Hex(),
		Email:      "test@unit.test",
		Ip:         "127.0.0.1",
		Locale:     "ru",
		Metadata: map[string]string{
			"field1": "value1",
			"field2": "value2",
		},
	}
	rsp := &grpc.ChangeCustomerResponse{}
	err := suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.Len(suite.T(), rsp.Item.Id, 24)

	count, err := suite.service.db.Collection(pkg.CollectionCustomerHistory).
		Find(bson.M{"customer_id": bson.ObjectIdHex(rsp.Item.Id)}).Count()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)

	req.Ip = "127.0.0.2"
	req.EmailVerified = true
	err = suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	req.Ip = "127.0.0.3"
	req.Phone = "1234567890"
	err = suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	req.Ip = "127.0.0.4"
	req.PhoneVerified = true
	err = suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	count, err = suite.service.db.Collection(pkg.CollectionCustomerHistory).
		Find(bson.M{"customer_id": bson.ObjectIdHex(rsp.Item.Id)}).Count()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

func (suite *CustomerTestSuite) TestCustomer_ChangeCustomer_TokenWithHistory_Ok() {
	req := &billing.Customer{
		ProjectId:  suite.project.Id,
		ExternalId: bson.NewObjectId().Hex(),
		Email:      "test@unit.test",
		Ip:         "127.0.0.1",
		Locale:     "ru",
		Metadata: map[string]string{
			"field1": "value1",
			"field2": "value2",
		},
	}
	rsp := &grpc.ChangeCustomerResponse{}
	err := suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.NotNil(suite.T(), rsp.Item)
	assert.Len(suite.T(), rsp.Item.Id, 24)

	count, err := suite.service.db.Collection(pkg.CollectionCustomerHistory).
		Find(bson.M{"customer_id": bson.ObjectIdHex(rsp.Item.Id)}).Count()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)

	req.Token = rsp.Item.Token
	req.Ip = "127.0.0.2"
	req.EmailVerified = true
	err = suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	req.Ip = "127.0.0.3"
	req.Phone = "1234567890"
	err = suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	req.Ip = "127.0.0.4"
	req.PhoneVerified = true
	err = suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	count, err = suite.service.db.Collection(pkg.CollectionCustomerHistory).
		Find(bson.M{"customer_id": bson.ObjectIdHex(rsp.Item.Id)}).Count()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 3, count)
}

func (suite *CustomerTestSuite) TestCustomer_ChangeCustomer_ProjectNotFound_Error() {
	req := &billing.Customer{
		ProjectId:  bson.NewObjectId().Hex(),
		ExternalId: bson.NewObjectId().Hex(),
		Email:      "test@unit.test",
		Ip:         "127.0.0.1",
		Locale:     "ru",
		Metadata: map[string]string{
			"field1": "value1",
			"field2": "value2",
		},
	}
	rsp := &grpc.ChangeCustomerResponse{}
	err := suite.service.ChangeCustomer(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), orderErrorProjectNotFound, rsp.Message)
}
