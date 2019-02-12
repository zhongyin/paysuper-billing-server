package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ProtocolONE/geoip-service/pkg/proto"
	"github.com/ProtocolONE/payone-billing-service/internal/config"
	"github.com/ProtocolONE/payone-billing-service/internal/database"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/ProtocolONE/payone-repository/pkg/constant"
	"github.com/ProtocolONE/payone-repository/tools"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/micro/go-micro/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"sort"
	"strings"
	"testing"
	"time"
)

type OrderTestSuite struct {
	suite.Suite
	service                                *Service
	project                                *billing.Project
	inactiveProject                        *billing.Project
	projectWithoutPaymentMethods           *billing.Project
	projectIncorrectPaymentMethodId        *billing.Project
	projectEmptyPaymentMethodTerminal      *billing.Project
	projectUahLimitCurrency                *billing.Project
	paymentMethod                          *billing.PaymentMethod
	inactivePaymentMethod                  *billing.PaymentMethod
	paymentMethodWithInactivePaymentSystem *billing.PaymentMethod
}

type GeoIpServiceTestOk struct{}
type GeoIpServiceTestOkWithoutSubdivision struct{}
type GeoIpServiceTestError struct{}

func newGeoIpServiceTestOkWithoutSubdivision() proto.GeoIpService {
	return &GeoIpServiceTestOkWithoutSubdivision{}
}
func newGeoIpServiceTestOk() proto.GeoIpService {
	return &GeoIpServiceTestOk{}
}

func newGeoIpServiceTestError() proto.GeoIpService {
	return &GeoIpServiceTestError{}
}

func (s *GeoIpServiceTestOk) GetIpData(
	ctx context.Context,
	in *proto.GeoIpDataRequest,
	opts ...client.CallOption,
) (*proto.GeoIpDataResponse, error) {
	data := &proto.GeoIpDataResponse{
		Country: &proto.GeoIpCountry{
			IsoCode: "RU",
			Names:   map[string]string{"en": "Russia", "ru": "Россия"},
		},
		City: &proto.GeoIpCity{
			Names: map[string]string{"en": "St.Petersburg", "ru": "Санкт-Петербург"},
		},
		Location: &proto.GeoIpLocation{
			TimeZone: "Europe/Moscow",
		},
		Subdivisions: []*proto.GeoIpSubdivision{
			{
				GeoNameID: uint32(1),
				IsoCode:   "SPE",
				Names:     map[string]string{"en": "St.Petersburg", "ru": "Санкт-Петербург"},
			},
		},
	}

	return data, nil
}

func (s *GeoIpServiceTestOkWithoutSubdivision) GetIpData(
	ctx context.Context,
	in *proto.GeoIpDataRequest,
	opts ...client.CallOption,
) (*proto.GeoIpDataResponse, error) {
	data := &proto.GeoIpDataResponse{
		Country: &proto.GeoIpCountry{
			IsoCode: "RU",
			Names:   map[string]string{"en": "Russia", "ru": "Россия"},
		},
		City: &proto.GeoIpCity{
			Names: map[string]string{"en": "St.Petersburg", "ru": "Санкт-Петербург"},
		},
		Location: &proto.GeoIpLocation{
			TimeZone: "Europe/Moscow",
		},
	}

	return data, nil
}

func (s *GeoIpServiceTestError) GetIpData(
	ctx context.Context,
	in *proto.GeoIpDataRequest,
	opts ...client.CallOption,
) (*proto.GeoIpDataResponse, error) {
	return &proto.GeoIpDataResponse{}, errors.New("some error")
}

func Test_Order(t *testing.T) {
	suite.Run(t, new(OrderTestSuite))
}

func (suite *OrderTestSuite) SetupTest() {
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
	usd := &billing.Currency{
		CodeInt:  840,
		CodeA3:   "USD",
		Name:     &billing.Name{Ru: "Доллар США", En: "US Dollar"},
		IsActive: true,
	}
	uah := &billing.Currency{
		CodeInt:  980,
		CodeA3:   "UAH",
		Name:     &billing.Name{Ru: "Украинская гривна", En: "Ukrainian Hryvnia"},
		IsActive: true,
	}

	currency := []interface{}{rub, usd, uah}

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
		&billing.CurrencyRate{
			CurrencyFrom: 643,
			CurrencyTo:   643,
			Rate:         1,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
	}

	err = db.Collection(collectionCurrencyRate).Insert(rate...)

	if err != nil {
		suite.FailNow("Insert rates test data failed", "%v", err)
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
			Handler:    "cardpay",
			Terminal:   "15985",
			ExternalId: "BANKCARD",
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

	project := &billing.Project{
		Id:               bson.NewObjectId().Hex(),
		CallbackCurrency: rub,
		CallbackProtocol: "default",
		LimitsCurrency:   usd,
		MaxPaymentAmount: 15000,
		MinPaymentAmount: 1,
		Name:             "test project 1",
		OnlyFixedAmounts: true,
		SecretKey:        "test project 1 secret key",
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
						Id:         "id_0",
						Name:       "package 0",
						CurrencyA3: "RUB",
						Price:      10,
						IsActive:   true,
					},
					{
						Id:         "id_1",
						Name:       "package 1",
						CurrencyA3: "RUB",
						Price:      100,
						IsActive:   true,
					},
					{
						Id:         "id_2",
						Name:       "package 2",
						CurrencyA3: "RUB",
						Price:      300,
						IsActive:   false,
					},
					{
						Id:         "id_3",
						Name:       "package 3",
						CurrencyA3: "AUD",
						Price:      500,
						IsActive:   true,
					},
					{
						Id:         "id_4",
						Name:       "package 4",
						CurrencyA3: "RUB",
						Price:      1000,
						IsActive:   true,
					},
				},
			},
			"US": {FixedPackage: []*billing.FixedPackage{}},
		},
		IsActive: true,
		Merchant: &billing.Merchant{
			Id:                        bson.NewObjectId().Hex(),
			ExternalId:                bson.NewObjectId().Hex(),
			Currency:                  usd,
			IsVatEnabled:              true,
			IsCommissionToUserEnabled: true,
			Status:                    1,
		},
	}
	projectUahLimitCurrency := &billing.Project{
		Id:               bson.NewObjectId().Hex(),
		CallbackCurrency: rub,
		CallbackProtocol: "default",
		LimitsCurrency:   uah,
		MaxPaymentAmount: 15000,
		MinPaymentAmount: 0,
		Name:             "project uah limit currency",
		OnlyFixedAmounts: true,
		SecretKey:        "project uah limit currency secret key",
		PaymentMethods: map[string]*billing.ProjectPaymentMethod{
			"BANKCARD": {
				Id:        pmBankCard.Id,
				Terminal:  "terminal",
				Password:  "password",
				CreatedAt: ptypes.TimestampNow(),
			},
		},
		IsActive: true,
		FixedPackage: map[string]*billing.FixedPackages{
			"RU": {
				FixedPackage: []*billing.FixedPackage{
					{
						Id:         "id_1",
						Name:       "package 1",
						CurrencyA3: "RUB",
						Price:      100,
						IsActive:   true,
					},
					{
						Id:         "id_2",
						Name:       "package 2",
						CurrencyA3: "RUB",
						Price:      300,
						IsActive:   false,
					},
					{
						Id:         "id_3",
						Name:       "package 3",
						CurrencyA3: "AUD",
						Price:      500,
						IsActive:   true,
					},
					{
						Id:         "id_4",
						Name:       "package 4",
						CurrencyA3: "RUB",
						Price:      1000,
						IsActive:   true,
					},
				},
			},
			"US": {FixedPackage: []*billing.FixedPackage{}},
		},
		Merchant: &billing.Merchant{
			Id:                        bson.NewObjectId().Hex(),
			ExternalId:                bson.NewObjectId().Hex(),
			Currency:                  uah,
			IsVatEnabled:              true,
			IsCommissionToUserEnabled: true,
			Status:                    1,
		},
	}
	projectIncorrectPaymentMethodId := &billing.Project{
		Id:               bson.NewObjectId().Hex(),
		CallbackCurrency: rub,
		CallbackProtocol: "default",
		LimitsCurrency:   rub,
		MaxPaymentAmount: 15000,
		MinPaymentAmount: 0,
		Name:             "project incorrect payment method id",
		OnlyFixedAmounts: true,
		SecretKey:        "project incorrect payment method id secret key",
		PaymentMethods: map[string]*billing.ProjectPaymentMethod{
			"BANKCARD": {
				Id:        bson.NewObjectId().Hex(),
				Terminal:  "terminal",
				Password:  "password",
				CreatedAt: ptypes.TimestampNow(),
			},
		},
		IsActive: true,
		Merchant: &billing.Merchant{
			Id:                        bson.NewObjectId().Hex(),
			ExternalId:                bson.NewObjectId().Hex(),
			Currency:                  uah,
			IsVatEnabled:              true,
			IsCommissionToUserEnabled: true,
			Status:                    1,
		},
	}
	projectEmptyPaymentMethodTerminal := &billing.Project{
		Id:               bson.NewObjectId().Hex(),
		CallbackCurrency: rub,
		CallbackProtocol: "default",
		LimitsCurrency:   rub,
		MaxPaymentAmount: 15000,
		MinPaymentAmount: 0,
		Name:             "project incorrect payment method id",
		OnlyFixedAmounts: false,
		SecretKey:        "project incorrect payment method id secret key",
		PaymentMethods: map[string]*billing.ProjectPaymentMethod{
			"BANKCARD": {
				Id:        pmBankCard.Id,
				Terminal:  "",
				Password:  "password",
				CreatedAt: ptypes.TimestampNow(),
			},
		},
		IsActive: true,
		Merchant: &billing.Merchant{
			Id:                        bson.NewObjectId().Hex(),
			ExternalId:                bson.NewObjectId().Hex(),
			Currency:                  uah,
			IsVatEnabled:              false,
			IsCommissionToUserEnabled: false,
			Status:                    1,
		},
	}
	projectWithoutPaymentMethods := &billing.Project{
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
	inactiveProject := &billing.Project{
		Id:               bson.NewObjectId().Hex(),
		CallbackCurrency: rub,
		CallbackProtocol: "xsolla",
		LimitsCurrency:   rub,
		MaxPaymentAmount: 15000,
		MinPaymentAmount: 0,
		Name:             "test project 2",
		OnlyFixedAmounts: true,
		SecretKey:        "test project 2 secret key",
		IsActive:         false,
	}

	projects := []interface{}{
		project,
		inactiveProject,
		projectWithoutPaymentMethods,
		projectIncorrectPaymentMethodId,
		projectEmptyPaymentMethodTerminal,
		projectUahLimitCurrency,
	}

	err = db.Collection(collectionProject).Insert(projects...)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
	}

	pmQiwi := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "Qiwi",
		Group:            "QIWI",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		Currency:         rub,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:    "cardpay",
			Terminal:   "15993",
			ExternalId: "QIWI",
		},
		Type:     "ewallet",
		IsActive: true,
		PaymentSystem: &billing.PaymentSystem{
			Id:                 bson.NewObjectId().Hex(),
			Name:               "CardPay 2",
			AccountingCurrency: uah,
			AccountingPeriod:   "every-day",
			Country:            &billing.Country{},
			IsActive:           false,
		},
	}
	pmWebMoney := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "WebMoney",
		Group:            "WEBMONEY",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		Currency:         rub,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:    "cardpay",
			Terminal:   "15985",
			ExternalId: "WEBMONEY",
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
	pmBitcoin := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "Bitcoin",
		Group:            "BITCOIN",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		Currency:         rub,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:    "cardpay",
			Terminal:   "16007",
			ExternalId: "BITCOIN",
		},
		Type:     "crypto",
		IsActive: false,
	}

	pms := []interface{}{pmBankCard, pmQiwi, pmBitcoin, pmWebMoney}

	err = db.Collection(collectionPaymentMethod).Insert(pms...)

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
		&billing.Commission{
			PaymentMethodId:         pmBankCard.Id,
			ProjectId:               projectIncorrectPaymentMethodId.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   1,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmQiwi.Id,
			ProjectId:               projectIncorrectPaymentMethodId.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   2,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBitcoin.Id,
			ProjectId:               projectIncorrectPaymentMethodId.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   3,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBankCard.Id,
			ProjectId:               projectEmptyPaymentMethodTerminal.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   1,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmQiwi.Id,
			ProjectId:               projectEmptyPaymentMethodTerminal.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   2,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBitcoin.Id,
			ProjectId:               projectEmptyPaymentMethodTerminal.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   3,
			StartDate:               commissionStartDate,
		},
	}

	err = db.Collection(collectionCommission).Insert(commissions...)

	if err != nil {
		suite.FailNow("Insert commission test data failed", "%v", err)
	}

	logger, err := zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	suite.service = NewBillingService(db, logger.Sugar(), cfg.CacheConfig, make(chan bool, 1), newGeoIpServiceTestOk(), "dev", "USD")
	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}

	suite.project = project
	suite.inactiveProject = inactiveProject
	suite.projectWithoutPaymentMethods = projectWithoutPaymentMethods
	suite.projectIncorrectPaymentMethodId = projectIncorrectPaymentMethodId
	suite.projectEmptyPaymentMethodTerminal = projectEmptyPaymentMethodTerminal
	suite.projectUahLimitCurrency = projectUahLimitCurrency
	suite.paymentMethod = pmBankCard
	suite.inactivePaymentMethod = pmBitcoin
	suite.paymentMethodWithInactivePaymentSystem = pmQiwi
}

func (suite *OrderTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()

	if err := suite.service.log.Sync(); err != nil {
		suite.FailNow("Logger sync failed", "%v", err)
	}
}

func (suite *OrderTestSuite) TestOrder_ProcessProject_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.project.Id,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.project)

	err := processor.processProject()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.project)
	assert.Equal(suite.T(), processor.checked.project.Id, suite.project.Id)
}

func (suite *OrderTestSuite) TestOrder_ProcessProject_NotFound() {
	req := &billing.OrderCreateRequest{
		ProjectId: "5bf67ebd46452d00062c7cc1",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.project)

	err := processor.processProject()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Equal(suite.T(), orderErrorProjectNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessProject_InactiveProject() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.inactiveProject.Id,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.project)

	err := processor.processProject()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Equal(suite.T(), orderErrorProjectInactive, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessCurrency_Ok() {
	req := &billing.OrderCreateRequest{
		Currency: "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.currency)

	err := processor.processCurrency()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.currency)
	assert.Equal(suite.T(), req.Currency, processor.checked.currency.CodeA3)
}

func (suite *OrderTestSuite) TestOrder_ProcessCurrency_Error() {
	req := &billing.OrderCreateRequest{
		Currency: "EUR",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.currency)

	err := processor.processCurrency()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.currency)
	assert.Equal(suite.T(), orderErrorCurrencyNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPayerData_EmptyEmailAndPhone_Ok() {
	req := &billing.OrderCreateRequest{
		PayerIp: "127.0.0.1",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.payerData)

	err := processor.processPayerData()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.payerData)
	assert.NotEmpty(suite.T(), processor.checked.payerData.Subdivision)
	assert.Empty(suite.T(), processor.checked.payerData.Email)
	assert.Empty(suite.T(), processor.checked.payerData.Phone)
}

func (suite *OrderTestSuite) TestOrder_ProcessPayerData_EmptySubdivision_Ok() {
	suite.service.geo = newGeoIpServiceTestOkWithoutSubdivision()

	req := &billing.OrderCreateRequest{
		PayerIp: "127.0.0.1",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.payerData)

	err := processor.processPayerData()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.payerData)
	assert.Empty(suite.T(), processor.checked.payerData.Subdivision)

	suite.service.geo = newGeoIpServiceTestOk()
}

func (suite *OrderTestSuite) TestOrder_ProcessPayerData_NotEmptyEmailAndPhone_Ok() {
	req := &billing.OrderCreateRequest{
		PayerIp:    "127.0.0.1",
		PayerEmail: "some_email@unit.com",
		PayerPhone: "123456789",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.payerData)

	err := processor.processPayerData()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.payerData)
	assert.Equal(suite.T(), req.PayerEmail, processor.checked.payerData.Email)
	assert.Equal(suite.T(), req.PayerPhone, processor.checked.payerData.Phone)
}

func (suite *OrderTestSuite) TestOrder_ProcessPayerData_Error() {
	suite.service.geo = newGeoIpServiceTestError()

	req := &billing.OrderCreateRequest{
		PayerIp: "127.0.0.1",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.payerData)

	err := processor.processPayerData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.payerData)
	assert.Equal(suite.T(), orderErrorPayerRegionUnknown, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessFixedPackage_RegionFromRequest_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.project.Id,
		Region:    "RU",
		Amount:    100,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.fixedPackage)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.fixedPackage)
	assert.Equal(suite.T(), suite.project.FixedPackage["RU"].FixedPackage[1].Id, processor.checked.fixedPackage.Id)
	assert.Equal(suite.T(), suite.project.FixedPackage["RU"].FixedPackage[1].Name, processor.checked.fixedPackage.Name)
}

func (suite *OrderTestSuite) TestOrder_ProcessFixedPackage_RegionFromPayerData_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.project.Id,
		Amount:    1000,
		PayerIp:   "127.0.0.1",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.fixedPackage)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.fixedPackage)
	assert.Equal(suite.T(), suite.project.FixedPackage["RU"].FixedPackage[len(suite.project.FixedPackage["RU"].FixedPackage)-1].Id, processor.checked.fixedPackage.Id)
	assert.Equal(suite.T(), suite.project.FixedPackage["RU"].FixedPackage[len(suite.project.FixedPackage["RU"].FixedPackage)-1].Name, processor.checked.fixedPackage.Name)
}

func (suite *OrderTestSuite) TestOrder_ProcessFixedPackage_EmptyFixedPackages_Error() {
	req := &billing.OrderCreateRequest{
		Amount: 100,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.fixedPackage)

	processor.checked.project = suite.inactiveProject

	err := processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.fixedPackage)
	assert.Equal(suite.T(), orderErrorFixedPackagesIsEmpty, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessFixedPackage_EmptyRegionFixedPackage_Error() {
	req := &billing.OrderCreateRequest{
		Amount: 100,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.fixedPackage)

	processor.checked.project = suite.project
	processor.checked.payerData = &billing.PayerData{CountryCodeA2: "US"}

	err := processor.processFixedPackage()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.fixedPackage)
	assert.Equal(suite.T(), orderErrorFixedPackageForRegionNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessFixedPackage_EmptyRegion_Error() {
	req := &billing.OrderCreateRequest{
		Amount: 100,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.fixedPackage)

	processor.checked.project = suite.project
	processor.checked.payerData = &billing.PayerData{CountryCodeA2: ""}

	err := processor.processFixedPackage()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.fixedPackage)
	assert.Equal(suite.T(), orderErrorPayerRegionUnknown, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessFixedPackage_FixedPackageNotFound_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.project.Id,
		Amount:    3000,
		PayerIp:   "127.0.0.1",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.fixedPackage)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.fixedPackage)
	assert.Equal(suite.T(), orderErrorFixedPackageNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessFixedPackage_FixedPackageCurrencyNotFound_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.project.Id,
		Amount:    500,
		PayerIp:   "127.0.0.1",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.fixedPackage)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.fixedPackage)
	assert.Equal(suite.T(), orderErrorFixedPackageUnknownCurrency, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessProjectOrderId_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.project.Id,
		Amount:    100,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)
}

func (suite *OrderTestSuite) TestOrder_ProcessProjectOrderId_Duplicate_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.project.Id,
		Amount:    100,
		OrderId:   "1234567890",
		Account:   "unit-test",
		Currency:  "RUB",
		Other:     make(map[string]string),
		PayerIp:   "127.0.0.1",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()
	assert.Nil(suite.T(), err)

	id := bson.NewObjectId().Hex()

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                processor.checked.project.Id,
			Name:              processor.checked.project.Name,
			UrlSuccess:        processor.checked.project.UrlRedirectSuccess,
			UrlFail:           processor.checked.project.UrlRedirectFail,
			SendNotifyEmail:   processor.checked.project.SendNotifyEmail,
			NotifyEmails:      processor.checked.project.NotifyEmails,
			SecretKey:         processor.checked.project.SecretKey,
			UrlCheckAccount:   processor.checked.project.UrlCheckAccount,
			UrlProcessPayment: processor.checked.project.UrlProcessPayment,
			CallbackProtocol:  processor.checked.project.CallbackProtocol,
			Merchant:          processor.checked.project.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     req.OrderId,
		ProjectAccount:                     req.Account,
		ProjectIncomeAmount:                req.Amount,
		ProjectIncomeCurrency:              processor.checked.currency,
		ProjectOutcomeAmount:               req.Amount,
		ProjectOutcomeCurrency:             processor.checked.project.CallbackCurrency,
		ProjectParams:                      req.Other,
		PayerData:                          processor.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      false,
		FixedPackage:                       processor.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
	}

	err = suite.service.db.Collection(collectionOrder).Insert(order)
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorProjectOrderIdIsDuplicate, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethod_Ok() {
	req := &billing.OrderCreateRequest{
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.paymentMethod)
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethod_PaymentMethodNotFound_Error() {
	req := &billing.OrderCreateRequest{
		PaymentMethod: "some_payment_method_from_my_head",
		Currency:      "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethod_PaymentMethodInactive_Error() {
	req := &billing.OrderCreateRequest{
		PaymentMethod: suite.inactivePaymentMethod.Group,
		Currency:      "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodInactive, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethod_PaymentSystemInactive_Error() {
	req := &billing.OrderCreateRequest{
		PaymentMethod: suite.paymentMethodWithInactivePaymentSystem.Group,
		Currency:      "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentSystemInactive, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethod_ProductionPaymentMethodEmpty_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectWithoutPaymentMethods.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	suite.service.env = environmentProd

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotAllowed, err.Error())

	suite.service.env = "dev"
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethod_ProductionPaymentMethodNotAllowed_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: "WEBMONEY",
		Currency:      "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	suite.service.env = environmentProd

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotAllowed, err.Error())

	suite.service.env = "dev"
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethod_ProductionPaymentMethodIncorrectId_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectIncorrectPaymentMethodId.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	suite.service.env = environmentProd

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodIncompatible, err.Error())

	suite.service.env = "dev"
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethod_ProductionPaymentMethodEmptyTerminal_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectEmptyPaymentMethodTerminal.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	suite.service.env = environmentProd

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodEmptySettings, err.Error())

	suite.service.env = "dev"
}

func (suite *OrderTestSuite) TestOrder_ProcessLimitAmounts_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)
}

func (suite *OrderTestSuite) TestOrder_ProcessLimitAmounts_ConvertAmount_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)
}

func (suite *OrderTestSuite) TestOrder_ProcessLimitAmounts_ConvertAmount_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectUahLimitCurrency.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessLimitAmounts_ProjectMinAmount_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        1,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorAmountLowerThanMinAllowed, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessLimitAmounts_ProjectMaxAmount_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        10000000,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorAmountGreaterThanMaxAllowed, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessLimitAmounts_PaymentMethodMinAmount_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        99,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorAmountLowerThanMinAllowedPaymentMethod, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessLimitAmounts_PaymentMethodMaxAmount_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        15001,
	}
	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorAmountGreaterThanMaxAllowedPaymentMethod, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessSignature_Form_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
	}

	req.RawParams = map[string]string{
		"PO_PROJECT_ID":     req.ProjectId,
		"PO_PAYMENT_METHOD": req.PaymentMethod,
		"PO_CURRENCY":       req.Currency,
		"PO_AMOUNT":         fmt.Sprintf("%f", req.Amount),
		"PO_ACCOUNT":        req.Account,
		"PO_DESCRIPTION":    req.Description,
		"PO_ORDER_ID":       req.OrderId,
		"PO_PAYER_EMAIL":    req.PayerEmail,
	}

	var keys []string
	var elements []string

	for k := range req.RawParams {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		value := k + "=" + req.RawParams[k]
		elements = append(elements, value)
	}

	hashString := strings.Join(elements, "") + suite.project.SecretKey

	h := sha512.New()
	h.Write([]byte(hashString))

	req.Signature = hex.EncodeToString(h.Sum(nil))

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processSignature()
	assert.Nil(suite.T(), err)
}

func (suite *OrderTestSuite) TestOrder_ProcessSignature_Json_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		IsJson:        true,
	}

	req.RawBody = `{"project":"` + suite.project.Id + `","amount":` + fmt.Sprintf("%f", req.Amount) +
		`,"currency":"` + req.Currency + `","account":"` + req.Account + `","order_id":"` + req.OrderId +
		`","description":"` + req.Description + `","payment_method":"` + req.PaymentMethod + `","payer_email":"` + req.PayerEmail + `"}`
	hashString := req.RawBody + suite.project.SecretKey

	h := sha512.New()
	h.Write([]byte(hashString))

	req.Signature = hex.EncodeToString(h.Sum(nil))

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processSignature()
	assert.Nil(suite.T(), err)
}

func (suite *OrderTestSuite) TestOrder_ProcessSignature_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		IsJson:        true,
	}

	req.RawBody = `{"project":"` + suite.project.Id + `","amount":` + fmt.Sprintf("%f", req.Amount) +
		`,"currency":"` + req.Currency + `","account":"` + req.Account + `","order_id":"` + req.OrderId +
		`","description":"` + req.Description + `","payment_method":"` + req.PaymentMethod + `","payer_email":"` + req.PayerEmail + `"}`

	fakeBody := `{"project":"` + suite.project.Id + `","amount":` + fmt.Sprintf("%f", req.Amount) +
		`,"currency":"` + req.Currency + `","account":"fake_account","order_id":"` + req.OrderId +
		`","description":"` + req.Description + `","payment_method":"` + req.PaymentMethod + `","payer_email":"` + req.PayerEmail + `"}`
	hashString := fakeBody + suite.project.SecretKey

	h := sha512.New()
	h.Write([]byte(hashString))

	req.Signature = hex.EncodeToString(h.Sum(nil))

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}
	assert.Nil(suite.T(), processor.checked.paymentMethod)

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processSignature()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorSignatureInvalid, err.Error())
}

func (suite *OrderTestSuite) TestOrder_PrepareOrder_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)

	order, err := processor.prepareOrder()
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), order)
}

func (suite *OrderTestSuite) TestOrder_PrepareOrder_PaymentMethod_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	order, err := processor.prepareOrder()
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), order)

	assert.NotNil(suite.T(), order.PaymentMethod)
	assert.Equal(suite.T(), processor.checked.paymentMethod.Id, order.PaymentMethod.Id)

	assert.NotNil(suite.T(), order.PaymentSystemFeeAmount)
	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountMerchantCurrency > 0)
	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountPaymentSystemCurrency > 0)
	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountPaymentMethodCurrency > 0)

	assert.NotNil(suite.T(), order.PspFeeAmount)
	assert.True(suite.T(), order.PspFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.PspFeeAmount.AmountMerchantCurrency > 0)
	assert.True(suite.T(), order.PspFeeAmount.AmountPspCurrency > 0)

	assert.NotNil(suite.T(), order.ToPayerFeeAmount)
	assert.True(suite.T(), order.ToPayerFeeAmount.AmountMerchantCurrency > 0)
	assert.True(suite.T(), order.ToPayerFeeAmount.AmountPaymentMethodCurrency > 0)

	assert.NotNil(suite.T(), order.ProjectFeeAmount)
	assert.True(suite.T(), order.ProjectFeeAmount.AmountMerchantCurrency > 0)
	assert.True(suite.T(), order.ProjectFeeAmount.AmountPaymentMethodCurrency > 0)

	assert.True(suite.T(), order.VatAmount > 0)
}

func (suite *OrderTestSuite) TestOrder_PrepareOrder_Convert_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.projectUahLimitCurrency.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	order, err := processor.prepareOrder()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())
}

func (suite *OrderTestSuite) TestOrder_PrepareOrder_Commission_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	processor.checked.payerData.CountryCodeA2 = "AU"

	order, err := processor.prepareOrder()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionVat), err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessOrderCommissions_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		PayerIp:       "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	id := bson.NewObjectId().Hex()

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                processor.checked.project.Id,
			Name:              processor.checked.project.Name,
			UrlSuccess:        processor.checked.project.UrlRedirectSuccess,
			UrlFail:           processor.checked.project.UrlRedirectFail,
			SendNotifyEmail:   processor.checked.project.SendNotifyEmail,
			NotifyEmails:      processor.checked.project.NotifyEmails,
			SecretKey:         processor.checked.project.SecretKey,
			UrlCheckAccount:   processor.checked.project.UrlCheckAccount,
			UrlProcessPayment: processor.checked.project.UrlProcessPayment,
			CallbackProtocol:  processor.checked.project.CallbackProtocol,
			Merchant:          processor.checked.project.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     req.OrderId,
		ProjectAccount:                     req.Account,
		ProjectIncomeAmount:                req.Amount,
		ProjectIncomeCurrency:              processor.checked.currency,
		ProjectOutcomeAmount:               req.Amount,
		ProjectOutcomeCurrency:             processor.checked.project.CallbackCurrency,
		ProjectParams:                      req.Other,
		PayerData:                          processor.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      false,
		FixedPackage:                       processor.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
		PaymentMethod: &billing.PaymentMethodOrder{
			Id:            processor.checked.paymentMethod.Id,
			Name:          processor.checked.paymentMethod.Name,
			Params:        processor.checked.paymentMethod.Params,
			PaymentSystem: processor.checked.paymentMethod.PaymentSystem,
			Group:         processor.checked.paymentMethod.Group,
		},
	}

	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)
	assert.Equal(suite.T(), float64(0), order.VatAmount)

	err = processor.processOrderCommissions(order)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), order.ProjectFeeAmount)
	assert.NotNil(suite.T(), order.PspFeeAmount)
	assert.NotNil(suite.T(), order.PaymentSystemFeeAmount)

	assert.True(suite.T(), order.ProjectFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.ProjectFeeAmount.AmountMerchantCurrency > 0)

	assert.True(suite.T(), order.PspFeeAmount.AmountMerchantCurrency > 0)
	assert.True(suite.T(), order.PspFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.PspFeeAmount.AmountPspCurrency > 0)

	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountMerchantCurrency > 0)
	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountPaymentSystemCurrency > 0)

	assert.True(suite.T(), order.VatAmount > 0)
}

func (suite *OrderTestSuite) TestOrder_ProcessOrderCommissions_VatNotFound_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		PayerIp:       "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	id := bson.NewObjectId().Hex()

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                processor.checked.project.Id,
			Name:              processor.checked.project.Name,
			UrlSuccess:        processor.checked.project.UrlRedirectSuccess,
			UrlFail:           processor.checked.project.UrlRedirectFail,
			SendNotifyEmail:   processor.checked.project.SendNotifyEmail,
			NotifyEmails:      processor.checked.project.NotifyEmails,
			SecretKey:         processor.checked.project.SecretKey,
			UrlCheckAccount:   processor.checked.project.UrlCheckAccount,
			UrlProcessPayment: processor.checked.project.UrlProcessPayment,
			CallbackProtocol:  processor.checked.project.CallbackProtocol,
			Merchant:          processor.checked.project.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     req.OrderId,
		ProjectAccount:                     req.Account,
		ProjectIncomeAmount:                req.Amount,
		ProjectIncomeCurrency:              processor.checked.currency,
		ProjectOutcomeAmount:               req.Amount,
		ProjectOutcomeCurrency:             processor.checked.project.CallbackCurrency,
		ProjectParams:                      req.Other,
		PayerData:                          processor.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      false,
		FixedPackage:                       processor.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
		PaymentMethod: &billing.PaymentMethodOrder{
			Id:            processor.checked.paymentMethod.Id,
			Name:          processor.checked.paymentMethod.Name,
			Params:        processor.checked.paymentMethod.Params,
			PaymentSystem: processor.checked.paymentMethod.PaymentSystem,
			Group:         processor.checked.paymentMethod.Group,
		},
	}

	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)

	processor.checked.payerData.CountryCodeA2 = "AU"

	err = processor.processOrderCommissions(order)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionVat), err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessOrderCommissions_CommissionNotFound_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectUahLimitCurrency.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		PayerIp:       "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processFixedPackage()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	id := bson.NewObjectId().Hex()

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                processor.checked.project.Id,
			Name:              processor.checked.project.Name,
			UrlSuccess:        processor.checked.project.UrlRedirectSuccess,
			UrlFail:           processor.checked.project.UrlRedirectFail,
			SendNotifyEmail:   processor.checked.project.SendNotifyEmail,
			NotifyEmails:      processor.checked.project.NotifyEmails,
			SecretKey:         processor.checked.project.SecretKey,
			UrlCheckAccount:   processor.checked.project.UrlCheckAccount,
			UrlProcessPayment: processor.checked.project.UrlProcessPayment,
			CallbackProtocol:  processor.checked.project.CallbackProtocol,
			Merchant:          processor.checked.project.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     req.OrderId,
		ProjectAccount:                     req.Account,
		ProjectIncomeAmount:                req.Amount,
		ProjectIncomeCurrency:              processor.checked.currency,
		ProjectOutcomeAmount:               req.Amount,
		ProjectOutcomeCurrency:             processor.checked.project.CallbackCurrency,
		ProjectParams:                      req.Other,
		PayerData:                          processor.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      false,
		FixedPackage:                       processor.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
		PaymentMethod: &billing.PaymentMethodOrder{
			Id:            processor.checked.paymentMethod.Id,
			Name:          processor.checked.paymentMethod.Name,
			Params:        processor.checked.paymentMethod.Params,
			PaymentSystem: processor.checked.paymentMethod.PaymentSystem,
			Group:         processor.checked.paymentMethod.Group,
		},
	}

	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)

	err = processor.processOrderCommissions(order)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCommission), err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessOrderCommissions_CommissionToUserEnableConvert_Error() {
	req := &billing.OrderCreateRequest{
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		PayerIp:       "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	id := bson.NewObjectId().Hex()

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                suite.projectIncorrectPaymentMethodId.Id,
			Name:              suite.projectIncorrectPaymentMethodId.Name,
			UrlSuccess:        suite.projectIncorrectPaymentMethodId.UrlRedirectSuccess,
			UrlFail:           suite.projectIncorrectPaymentMethodId.UrlRedirectFail,
			SendNotifyEmail:   suite.projectIncorrectPaymentMethodId.SendNotifyEmail,
			NotifyEmails:      suite.projectIncorrectPaymentMethodId.NotifyEmails,
			SecretKey:         suite.projectIncorrectPaymentMethodId.SecretKey,
			UrlCheckAccount:   suite.projectIncorrectPaymentMethodId.UrlCheckAccount,
			UrlProcessPayment: suite.projectIncorrectPaymentMethodId.UrlProcessPayment,
			CallbackProtocol:  suite.projectIncorrectPaymentMethodId.CallbackProtocol,
			Merchant:          suite.projectIncorrectPaymentMethodId.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     req.OrderId,
		ProjectAccount:                     req.Account,
		ProjectIncomeAmount:                req.Amount,
		ProjectIncomeCurrency:              processor.checked.currency,
		ProjectOutcomeAmount:               req.Amount,
		ProjectOutcomeCurrency:             suite.projectIncorrectPaymentMethodId.CallbackCurrency,
		ProjectParams:                      req.Other,
		PayerData:                          processor.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      false,
		FixedPackage:                       processor.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
		PaymentMethod: &billing.PaymentMethodOrder{
			Id:            processor.checked.paymentMethod.Id,
			Name:          processor.checked.paymentMethod.Name,
			Params:        processor.checked.paymentMethod.Params,
			PaymentSystem: processor.checked.paymentMethod.PaymentSystem,
			Group:         processor.checked.paymentMethod.Group,
		},
	}

	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)

	processor.checked.project = suite.projectIncorrectPaymentMethodId

	err = processor.processOrderCommissions(order)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessOrderCommissions_MerchantAccountingCurrencyConvert_Error() {
	req := &billing.OrderCreateRequest{
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		PayerIp:       "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	id := bson.NewObjectId().Hex()

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                suite.projectEmptyPaymentMethodTerminal.Id,
			Name:              suite.projectEmptyPaymentMethodTerminal.Name,
			UrlSuccess:        suite.projectEmptyPaymentMethodTerminal.UrlRedirectSuccess,
			UrlFail:           suite.projectEmptyPaymentMethodTerminal.UrlRedirectFail,
			SendNotifyEmail:   suite.projectEmptyPaymentMethodTerminal.SendNotifyEmail,
			NotifyEmails:      suite.projectEmptyPaymentMethodTerminal.NotifyEmails,
			SecretKey:         suite.projectEmptyPaymentMethodTerminal.SecretKey,
			UrlCheckAccount:   suite.projectEmptyPaymentMethodTerminal.UrlCheckAccount,
			UrlProcessPayment: suite.projectEmptyPaymentMethodTerminal.UrlProcessPayment,
			CallbackProtocol:  suite.projectEmptyPaymentMethodTerminal.CallbackProtocol,
			Merchant:          suite.projectEmptyPaymentMethodTerminal.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     req.OrderId,
		ProjectAccount:                     req.Account,
		ProjectIncomeAmount:                req.Amount,
		ProjectIncomeCurrency:              processor.checked.currency,
		ProjectOutcomeAmount:               req.Amount,
		ProjectOutcomeCurrency:             suite.projectEmptyPaymentMethodTerminal.CallbackCurrency,
		ProjectParams:                      req.Other,
		PayerData:                          processor.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      false,
		FixedPackage:                       processor.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
		PaymentMethod: &billing.PaymentMethodOrder{
			Id:            processor.checked.paymentMethod.Id,
			Name:          processor.checked.paymentMethod.Name,
			Params:        processor.checked.paymentMethod.Params,
			PaymentSystem: processor.checked.paymentMethod.PaymentSystem,
			Group:         processor.checked.paymentMethod.Group,
		},
	}

	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)

	processor.checked.project = suite.projectEmptyPaymentMethodTerminal

	err = processor.processOrderCommissions(order)
	assert.Error(suite.T(), err)

	assert.NotNil(suite.T(), order.ProjectFeeAmount)
	assert.True(suite.T(), order.ProjectFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.Equal(suite.T(), float64(0), order.ProjectFeeAmount.AmountMerchantCurrency)

	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessOrderCommissions_PspAccountingCurrencyConvert_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		PayerIp:       "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	err = processor.processPaymentMethod()
	assert.Nil(suite.T(), err)

	id := bson.NewObjectId().Hex()

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                processor.checked.project.Id,
			Name:              processor.checked.project.Name,
			UrlSuccess:        processor.checked.project.UrlRedirectSuccess,
			UrlFail:           processor.checked.project.UrlRedirectFail,
			SendNotifyEmail:   processor.checked.project.SendNotifyEmail,
			NotifyEmails:      processor.checked.project.NotifyEmails,
			SecretKey:         processor.checked.project.SecretKey,
			UrlCheckAccount:   processor.checked.project.UrlCheckAccount,
			UrlProcessPayment: processor.checked.project.UrlProcessPayment,
			CallbackProtocol:  processor.checked.project.CallbackProtocol,
			Merchant:          processor.checked.project.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     req.OrderId,
		ProjectAccount:                     req.Account,
		ProjectIncomeAmount:                req.Amount,
		ProjectIncomeCurrency:              processor.checked.currency,
		ProjectOutcomeAmount:               req.Amount,
		ProjectOutcomeCurrency:             processor.checked.project.CallbackCurrency,
		ProjectParams:                      req.Other,
		PayerData:                          processor.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      false,
		FixedPackage:                       processor.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
		PaymentMethod: &billing.PaymentMethodOrder{
			Id:            processor.checked.paymentMethod.Id,
			Name:          processor.checked.paymentMethod.Name,
			Params:        processor.checked.paymentMethod.Params,
			PaymentSystem: processor.checked.paymentMethod.PaymentSystem,
			Group:         processor.checked.paymentMethod.Group,
		},
	}

	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)

	suite.service.accountingCurrency = &billing.Currency{
		CodeInt:  980,
		CodeA3:   "UAH",
		Name:     &billing.Name{Ru: "Украинская гривна", En: "Ukrainian Hryvnia"},
		IsActive: true,
	}

	err = processor.processOrderCommissions(order)
	assert.Error(suite.T(), err)

	assert.NotNil(suite.T(), order.ProjectFeeAmount)
	assert.True(suite.T(), order.ProjectFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.ProjectFeeAmount.AmountMerchantCurrency > 0)

	assert.NotNil(suite.T(), order.PspFeeAmount)
	assert.True(suite.T(), order.PspFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.PspFeeAmount.AmountMerchantCurrency > 0)
	assert.Equal(suite.T(), float64(0), order.PspFeeAmount.AmountPspCurrency)

	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())

	suite.service.accountingCurrency = &billing.Currency{
		CodeInt:  840,
		CodeA3:   "USD",
		Name:     &billing.Name{Ru: "Доллар США", En: "US Dollar"},
		IsActive: true,
	}
}

func (suite *OrderTestSuite) TestOrder_ProcessOrderCommissions_PaymentSystemAccountingCurrencyConvert_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId: suite.project.Id,
		Currency:  "RUB",
		Amount:    100,
		PayerIp:   "127.0.0.1",
	}

	processor := &OrderCreateRequestProcessor{
		Service: suite.service,
		request: req,
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processPayerData()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	id := bson.NewObjectId().Hex()

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                processor.checked.project.Id,
			Name:              processor.checked.project.Name,
			UrlSuccess:        processor.checked.project.UrlRedirectSuccess,
			UrlFail:           processor.checked.project.UrlRedirectFail,
			SendNotifyEmail:   processor.checked.project.SendNotifyEmail,
			NotifyEmails:      processor.checked.project.NotifyEmails,
			SecretKey:         processor.checked.project.SecretKey,
			UrlCheckAccount:   processor.checked.project.UrlCheckAccount,
			UrlProcessPayment: processor.checked.project.UrlProcessPayment,
			CallbackProtocol:  processor.checked.project.CallbackProtocol,
			Merchant:          processor.checked.project.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     req.OrderId,
		ProjectAccount:                     req.Account,
		ProjectIncomeAmount:                req.Amount,
		ProjectIncomeCurrency:              processor.checked.currency,
		ProjectOutcomeAmount:               req.Amount,
		ProjectOutcomeCurrency:             processor.checked.project.CallbackCurrency,
		ProjectParams:                      req.Other,
		PayerData:                          processor.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      false,
		FixedPackage:                       processor.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
		PaymentMethod: &billing.PaymentMethodOrder{
			Id:            suite.paymentMethodWithInactivePaymentSystem.Id,
			Name:          suite.paymentMethodWithInactivePaymentSystem.Name,
			Params:        suite.paymentMethodWithInactivePaymentSystem.Params,
			PaymentSystem: suite.paymentMethodWithInactivePaymentSystem.PaymentSystem,
			Group:         suite.paymentMethodWithInactivePaymentSystem.Group,
		},
	}

	assert.Nil(suite.T(), order.ProjectFeeAmount)
	assert.Nil(suite.T(), order.PspFeeAmount)
	assert.Nil(suite.T(), order.PaymentSystemFeeAmount)

	processor.checked.paymentMethod = suite.paymentMethodWithInactivePaymentSystem

	err = processor.processOrderCommissions(order)
	assert.Error(suite.T(), err)

	assert.NotNil(suite.T(), order.ProjectFeeAmount)
	assert.True(suite.T(), order.ProjectFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.ProjectFeeAmount.AmountMerchantCurrency > 0)

	assert.NotNil(suite.T(), order.PspFeeAmount)
	assert.True(suite.T(), order.PspFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.PspFeeAmount.AmountMerchantCurrency > 0)
	assert.True(suite.T(), order.PspFeeAmount.AmountPspCurrency > 0)

	assert.NotNil(suite.T(), order.PaymentSystemFeeAmount)
	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.Equal(suite.T(), float64(0), order.PaymentSystemFeeAmount.AmountMerchantCurrency)
	assert.Equal(suite.T(), float64(0), order.PaymentSystemFeeAmount.AmountMerchantCurrency)

	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)
	assert.NotNil(suite.T(), rsp.Project)
	assert.NotNil(suite.T(), rsp.PaymentMethod)
	assert.NotNil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_ProjectInactive_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.inactiveProject.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorProjectInactive, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_SignatureInvalid_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
		IsJson:        true,
	}

	req.RawBody = `{"project":"` + suite.project.Id + `","amount":` + fmt.Sprintf("%f", req.Amount) +
		`,"currency":"` + req.Currency + `","account":"` + req.Account + `","order_id":"` + req.OrderId +
		`","description":"` + req.Description + `","payment_method":"` + req.PaymentMethod + `","payer_email":"` + req.PayerEmail + `"}`

	fakeBody := `{"project":"` + suite.project.Id + `","amount":` + fmt.Sprintf("%f", req.Amount) +
		`,"currency":"` + req.Currency + `","account":"fake_account","order_id":"` + req.OrderId +
		`","description":"` + req.Description + `","payment_method":"` + req.PaymentMethod + `","payer_email":"` + req.PayerEmail + `"}`
	hashString := fakeBody + suite.project.SecretKey

	h := sha512.New()
	h.Write([]byte(hashString))

	req.Signature = hex.EncodeToString(h.Sum(nil))

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorSignatureInvalid, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_PayerDataInvalid_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	suite.service.geo = newGeoIpServiceTestError()

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorPayerRegionUnknown, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_CurrencyInvalid_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "AUD",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorCurrencyNotFound, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_FixedPackageInvalid_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "USD",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorFixedPackageNotFound, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_CurrencyEmpty_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectEmptyPaymentMethodTerminal.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorCurrencyIsRequired, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_DuplicateProjectOrderId_Error() {
	orderId := bson.NewObjectId().Hex()

	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       orderId,
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	order := &billing.Order{
		Id: bson.NewObjectId().Hex(),
		Project: &billing.ProjectOrder{
			Id:                suite.project.Id,
			Name:              suite.project.Name,
			UrlSuccess:        suite.project.UrlRedirectSuccess,
			UrlFail:           suite.project.UrlRedirectFail,
			SendNotifyEmail:   suite.project.SendNotifyEmail,
			NotifyEmails:      suite.project.NotifyEmails,
			SecretKey:         suite.project.SecretKey,
			UrlCheckAccount:   suite.project.UrlCheckAccount,
			UrlProcessPayment: suite.project.UrlProcessPayment,
			CallbackProtocol:  suite.project.CallbackProtocol,
			Merchant:          suite.project.Merchant,
		},
		Description:         fmt.Sprintf(orderDefaultDescription, orderId),
		ProjectOrderId:      req.OrderId,
		ProjectAccount:      req.Account,
		ProjectIncomeAmount: req.Amount,
		ProjectIncomeCurrency: &billing.Currency{
			CodeInt:  643,
			CodeA3:   "RUB",
			Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
			IsActive: true,
		},
		ProjectOutcomeAmount: req.Amount,
		ProjectOutcomeCurrency: &billing.Currency{
			CodeInt:  643,
			CodeA3:   "RUB",
			Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
			IsActive: true,
		},
		ProjectParams: req.Other,
		PayerData: &billing.PayerData{
			Ip:            req.PayerIp,
			CountryCodeA2: "RU",
			CountryName:   &billing.Name{En: "Russia", Ru: "Россия"},
			City:          &billing.Name{En: "St.Petersburg", Ru: "Санкт-Петербург"},
			Subdivision:   "",
			Timezone:      "Europe/Moscow",
		},
		Status:        constant.OrderStatusNew,
		CreatedAt:     ptypes.TimestampNow(),
		IsJsonRequest: false,
		FixedPackage: &billing.FixedPackage{
			Id:         "id_1",
			Name:       "package 1",
			CurrencyA3: "RUB",
			Price:      100,
			IsActive:   true,
		},
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency: &billing.Currency{
			CodeInt:  643,
			CodeA3:   "RUB",
			Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
			IsActive: true,
		},
		PaymentMethodIncomeAmount: req.Amount,
		PaymentMethodIncomeCurrency: &billing.Currency{
			CodeInt:  643,
			CodeA3:   "RUB",
			Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
			IsActive: true,
		},
	}

	err := suite.service.db.Collection(collectionOrder).Insert(order)
	assert.Nil(suite.T(), err)

	rsp := &billing.Order{}
	err = suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorProjectOrderIdIsDuplicate, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_PaymentMethodInvalid_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.inactivePaymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorPaymentMethodInactive, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_AmountInvalid_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.project.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        10,
		Account:       "unit test",
		Description:   "unit test",
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorAmountLowerThanMinAllowed, err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderCreateProcess_PrepareOrderInvalid_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectEmptyPaymentMethodTerminal.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionCurrencyRate), err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}
