package service

import (
	"errors"
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

type BillingServiceTestSuite struct {
	suite.Suite
	db   *database.Source
	log  *zap.Logger
	cfg  *config.Config
	exCh chan bool
}

type getAllEmptyResultTest Currency
type getAllErrorTest Currency

func newGetAllEmptyResultTest(svc *Service) Cacher {
	return &getAllEmptyResultTest{svc: svc}
}

func (h *getAllEmptyResultTest) setCache(recs []interface{}) {
	return
}

func (h *getAllEmptyResultTest) getAll() (recs []interface{}, err error) {
	return
}

func newGetAllErrorTest(svc *Service) Cacher {
	return &getAllErrorTest{svc: svc}
}

func (h *getAllErrorTest) setCache(recs []interface{}) {
	return
}

func (h *getAllErrorTest) getAll() (recs []interface{}, err error) {
	err = errors.New("unit test")

	return
}

func Test_BillingService(t *testing.T) {
	suite.Run(t, new(BillingServiceTestSuite))
}

func (suite *BillingServiceTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	cfg.AccountingCurrency = "RUB"

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
				CodeInt:  804,
				CodeA2:   "UA",
				CodeA3:   "UKR",
				Name:     &billing.Name{Ru: "Украина", En: "Ukraine"},
				IsActive: true,
			},
			Subdivision: "",
			Vat:         22,
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
			Subdivision: "AL",
			Vat:         13.5,
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

	err = suite.db.Collection(pkg.CollectionVat).Insert(vat...)

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

	err = suite.db.Collection(pkg.CollectionCurrency).Insert(currency...)

	if err != nil {
		suite.FailNow("Insert currency test data failed", "%v", err)
	}

	projectDefault := &billing.Project{
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
	projectXsolla := &billing.Project{
		Id:               bson.NewObjectId().Hex(),
		CallbackCurrency: rub,
		CallbackProtocol: "xsolla",
		LimitsCurrency:   rub,
		MaxPaymentAmount: 15000,
		MinPaymentAmount: 0,
		Name:             "test project 2",
		OnlyFixedAmounts: true,
		SecretKey:        "test project 2 secret key",
		IsActive:         true,
	}
	projectCardpay := &billing.Project{
		Id:               bson.NewObjectId().Hex(),
		CallbackCurrency: rub,
		CallbackProtocol: "cardpay",
		LimitsCurrency:   rub,
		MaxPaymentAmount: 15000,
		MinPaymentAmount: 0,
		Name:             "test project 3",
		OnlyFixedAmounts: true,
		SecretKey:        "test project 3 secret key",
		IsActive:         true,
	}

	project := []interface{}{projectDefault, projectXsolla, projectCardpay}

	err = suite.db.Collection(pkg.CollectionProject).Insert(project...)

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

	err = suite.db.Collection(pkg.CollectionCurrencyRate).Insert(rate...)

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

	err = suite.db.Collection(pkg.CollectionPaymentMethod).Insert(pms...)

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
			ProjectId:               projectDefault.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   1,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmQiwi.Id,
			ProjectId:               projectDefault.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   2,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBitcoin.Id,
			ProjectId:               projectDefault.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   3,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBankCard.Id,
			ProjectId:               projectXsolla.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   1,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmQiwi.Id,
			ProjectId:               projectXsolla.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   2,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBitcoin.Id,
			ProjectId:               projectXsolla.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   3,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBankCard.Id,
			ProjectId:               projectCardpay.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   1,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmQiwi.Id,
			ProjectId:               projectCardpay.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   2,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBitcoin.Id,
			ProjectId:               projectCardpay.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   3,
			StartDate:               commissionStartDate,
		},
	}

	err = suite.db.Collection(pkg.CollectionCommission).Insert(commissions...)

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

	date, err := ptypes.TimestampProto(time.Now().Add(time.Hour * -480))

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

	suite.exCh = make(chan bool, 1)
}

func (suite *BillingServiceTestSuite) TearDownTest() {
	if err := suite.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.db.Close()
}

func (suite *BillingServiceTestSuite) TestNewBillingService() {
	service := NewBillingService(suite.db, suite.cfg, suite.exCh, nil, nil, nil)

	if _, ok := handlers["unit"]; ok {
		delete(handlers, "unit")
	}

	err := service.Init()

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(service.currencyCache) > 0)
	assert.True(suite.T(), len(service.projectCache) > 0)
	assert.True(suite.T(), len(service.currencyRateCache) > 0)
	assert.True(suite.T(), len(service.vatCache) > 0)
	assert.True(suite.T(), len(service.paymentMethodCache) > 0)
	assert.True(suite.T(), len(service.commissionCache) > 0)
}

func (suite *BillingServiceTestSuite) TestBillingService_GetAllEmptyResult() {
	svc := NewBillingService(suite.db, suite.cfg, suite.exCh, nil, nil, nil)

	key := "unit"
	err := svc.cache(key, newGetAllEmptyResultTest(svc))

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf(initCacheErrorNotFound, key), err.Error())
}

func (suite *BillingServiceTestSuite) TestBillingService_GetAllError() {
	svc := NewBillingService(suite.db, suite.cfg, suite.exCh, nil, nil, nil)

	key := "unit"
	err := svc.cache(key, newGetAllErrorTest(svc))

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), "unit test", err.Error())
}

func (suite *BillingServiceTestSuite) TestBillingService_InitCacheError() {
	svc := NewBillingService(suite.db, suite.cfg, suite.exCh, nil, nil, nil)

	key := "unit"
	handlers[key] = newGetAllEmptyResultTest

	err := svc.Init()

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf(initCacheErrorNotFound, key), err.Error())
}

func (suite *BillingServiceTestSuite) TestBillingService_RebuildCacheExit() {
	service := NewBillingService(suite.db, suite.cfg, suite.exCh, nil, nil, nil)

	if _, ok := handlers["unit"]; ok {
		delete(handlers, "unit")
	}

	err := service.Init()

	assert.Nil(suite.T(), err)
	time.Sleep(time.Second * 1)
	assert.True(suite.T(), service.rebuild)

	tp := time.NewTimer(time.Second * 2)
	exit := make(chan bool, 1)

	select {
	case <-tp.C:
		suite.exCh <- true
		exit <- true
	}
	<-exit

	time.Sleep(time.Second * 1)
	assert.False(suite.T(), service.rebuild)
	assert.Nil(suite.T(), service.rebuildError)
}

func (suite *BillingServiceTestSuite) TestBillingService_RebuildCacheByTimer() {
	cfg := suite.cfg
	cfg.CacheConfig.CurrencyTimeout = 3

	service := NewBillingService(suite.db, cfg, suite.exCh, nil, nil, nil)

	if _, ok := handlers["unit"]; ok {
		delete(handlers, "unit")
	}

	err := service.Init()
	assert.Nil(suite.T(), err)

	c := &billing.Currency{
		CodeInt:  826,
		CodeA3:   "GBP",
		Name:     &billing.Name{Ru: "Фунт стерлингов Соединенного королевства", En: "British Pound Sterling"},
		IsActive: true,
	}

	err = suite.db.Collection(pkg.CollectionCurrency).Insert(c)
	assert.Nil(suite.T(), err)

	_, ok := service.currencyCache[c.CodeA3]
	assert.False(suite.T(), ok)

	time.Sleep(time.Second * time.Duration(cfg.CurrencyTimeout+1))

	_, ok = service.currencyCache[c.CodeA3]
	assert.True(suite.T(), ok)
	assert.True(suite.T(), service.rebuild)
	assert.Nil(suite.T(), service.rebuildError)
}

func (suite *BillingServiceTestSuite) TestBillingService_RebuildCacheByTimerError() {
	cfg := suite.cfg
	cfg.CurrencyTimeout = 3

	service := NewBillingService(suite.db, cfg, suite.exCh, nil, nil, nil)

	if _, ok := handlers["unit"]; ok {
		delete(handlers, "unit")
	}

	err := service.Init()
	assert.Nil(suite.T(), err)

	time.Sleep(time.Second * 1)

	assert.True(suite.T(), service.rebuild)
	assert.Nil(suite.T(), service.rebuildError)

	assert.Nil(suite.T(), suite.db.Collection(pkg.CollectionCurrency).DropCollection())

	time.Sleep(time.Second * time.Duration(cfg.CurrencyTimeout+1))

	assert.False(suite.T(), service.rebuild)
	assert.Error(suite.T(), service.rebuildError)
}

func (suite *BillingServiceTestSuite) TestBillingService_AccountingCurrencyInitError() {
	cfg, err := config.NewConfig()
	cfg.AccountingCurrency = "AUD"

	service := NewBillingService(suite.db, cfg, suite.exCh, nil, nil, nil)

	if _, ok := handlers["unit"]; ok {
		delete(handlers, "unit")
	}

	err = service.Init()
	assert.Error(suite.T(), err)
}

func (suite *BillingServiceTestSuite) TestBillingService_IsProductionEnvironment() {
	service := NewBillingService(suite.db, suite.cfg, suite.exCh, nil, nil, nil)

	if _, ok := handlers["unit"]; ok {
		delete(handlers, "unit")
	}

	err := service.Init()
	assert.Nil(suite.T(), err)

	isProd := service.isProductionEnvironment()
	assert.False(suite.T(), isProd)
}
