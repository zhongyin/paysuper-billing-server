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
)

type SystemFeesTestSuite struct {
	suite.Suite
	service       *Service
	log           *zap.Logger
	project       *billing.Project
	paymentMethod *billing.PaymentMethod
	pmWebMoney    *billing.PaymentMethod
	AdminUserId   string
}

var systemFeeExample = &billing.SystemFee{
	Percent:         0.3,
	PercentCurrency: "EUR",
	FixAmount:       0.20,
	FixCurrency:     "EUR",
}

func Test_SystemFees(t *testing.T) {
	suite.Run(t, new(SystemFeesTestSuite))
}

func (suite *SystemFeesTestSuite) SetupTest() {
	cfg, err := config.NewConfig()

	assert.NoError(suite.T(), err, "Config load failed")

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
	usd := &billing.Currency{
		CodeInt:  840,
		CodeA3:   "USD",
		Name:     &billing.Name{Ru: "Доллар США", En: "US Dollar"},
		IsActive: true,
	}

	currency := []interface{}{rub, usd}

	err = db.Collection(pkg.CollectionCurrency).Insert(currency...)

	us := &billing.Country{
		CodeInt:  840,
		CodeA2:   "US",
		CodeA3:   "USA",
		Name:     &billing.Name{Ru: "США", En: "USA"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCountry).Insert([]interface{}{us}...)
	assert.NoError(suite.T(), err, "Insert country test data failed")

	if err != nil {
		suite.FailNow("Insert currency test data failed", "%v", err)
	}

	pmBankCard := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "Bank card",
		Group:            "BANKCARD",
		MinPaymentAmount: 100,
		MaxPaymentAmount: 15000,
		Currency:         rub,
		Currencies:       []int32{643, 840, 980},
		Params:           &billing.PaymentMethodParams{},
		Type:             "bank_card",
		IsActive:         true,
		AccountRegexp:    "^(?:4[0-9]{12}(?:[0-9]{3})?|[25][1-7][0-9]{14}|6(?:011|5[0-9][0-9])[0-9]{12}|3[47][0-9]{13}|3(?:0[0-5]|[68][0-9])[0-9]{11}|(?:2131|1800|35\\d{3})\\d{11})$",
		PaymentSystem: &billing.PaymentSystem{
			Id: bson.NewObjectId().Hex(),
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
		Params:           &billing.PaymentMethodParams{},
		Type:             "ewallet",
		IsActive:         true,
		PaymentSystem: &billing.PaymentSystem{
			Id: bson.NewObjectId().Hex(),
		},
	}

	pms := []interface{}{pmBankCard, pmWebMoney}

	err = db.Collection(pkg.CollectionPaymentMethod).Insert(pms...)

	if err != nil {
		suite.FailNow("Insert payment methods test data failed", "%v", err)
	}

	cardBrands := []string{"MASTERCARD", "VISA"}
	regions := []string{"", "EU"}
	adminUserId := bson.NewObjectId().Hex()

	for _, r := range pms {
		pm := r.(*billing.PaymentMethod)
		for _, reg := range regions {

			systemFee := &billing.SystemFees{
				Id:        bson.NewObjectId().Hex(),
				MethodId:  pm.Id,
				Region:    reg,
				CardBrand: "",
				UserId:    adminUserId,
				CreatedAt: ptypes.TimestampNow(),
				IsActive:  true,
				Fees: []*billing.FeeSet{
					{
						MinAmounts: map[string]float64{"EUR": 0, "USD": 0, "RUB": 0},
						TransactionCost: &billing.SystemFee{
							Percent:         1.15,
							PercentCurrency: "EUR",
							FixAmount:       0.10,
							FixCurrency:     "EUR",
						},
						AuthorizationFee: &billing.SystemFee{
							Percent:         0,
							PercentCurrency: "EUR",
							FixAmount:       0.05,
							FixCurrency:     "EUR",
						},
					},
					{
						MinAmounts: map[string]float64{"EUR": 100, "USD": 113.09, "RUB": 7235.63},
						TransactionCost: &billing.SystemFee{
							Percent:         2.15,
							PercentCurrency: "EUR",
							FixAmount:       0.20,
							FixCurrency:     "EUR",
						},
						AuthorizationFee: &billing.SystemFee{
							Percent:         0.3,
							PercentCurrency: "EUR",
							FixAmount:       0.10,
							FixCurrency:     "EUR",
						},
					},
				},
			}

			if pm.IsBankCard() {
				for _, cb := range cardBrands {
					systemFee.Id = bson.NewObjectId().Hex()
					systemFee.CardBrand = cb
					err = db.Collection(pkg.CollectionSystemFees).Insert(systemFee)
					if err != nil {
						suite.FailNow("Insert system fees test data failed", "%v", err)
					}
				}
			} else {
				err = db.Collection(pkg.CollectionSystemFees).Insert(systemFee)
				if err != nil {
					suite.FailNow("Insert system fees test data failed", "%v", err)
				}
			}
		}
	}

	suite.log, err = zap.NewProduction()
	assert.NoError(suite.T(), err, "Logger initialization failed")

	broker, err := rabbitmq.NewBroker(cfg.BrokerAddress)
	assert.NoError(suite.T(), err, "Creating RabbitMQ publisher failed")

	suite.service = NewBillingService(
		db,
		cfg,
		make(chan bool, 1),
		mock.NewGeoIpServiceTestOk(),
		mock.NewRepositoryServiceOk(),
		mock.NewTaxServiceOkMock(),
		broker,
		nil,
	)
	err = suite.service.Init()
	assert.NoError(suite.T(), err, "Billing service initialization failed")

	suite.AdminUserId = adminUserId
	suite.paymentMethod = pmBankCard
	suite.pmWebMoney = pmWebMoney
}

func (suite *SystemFeesTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *SystemFeesTestSuite) TestSystemFees_AddSystemFees() {

	query := bson.M{
		"method_id":  bson.ObjectIdHex(suite.paymentMethod.Id),
		"region":     "",
		"card_brand": "MASTERCARD",
		"is_active":  true,
	}

	var fees []*billing.SystemFees

	// get existing fees

	err := suite.service.db.Collection(pkg.CollectionSystemFees).Find(query).All(&fees)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), len(fees) == 1)
	assert.True(suite.T(), len(fees[0].Fees) == 2)
	assert.True(suite.T(), fees[0].Fees[0].TransactionCost.Percent == float64(1.15))

	// add new worldwide Mastercard

	req1 := &billing.AddSystemFeesRequest{
		MethodId:  suite.paymentMethod.Id,
		CardBrand: "MASTERCARD",
		Fees: []*billing.FeeSet{
			{
				MinAmounts: map[string]float64{"EUR": 0, "USD": 0},
				TransactionCost: &billing.SystemFee{
					Percent:         2.35,
					PercentCurrency: "EUR",
					FixAmount:       0.20,
					FixCurrency:     "EUR",
				},
				AuthorizationFee: systemFeeExample,
			},
		},
		UserId: suite.AdminUserId,
	}

	err = suite.service.AddSystemFees(context.TODO(), req1, &grpc.EmptyResponse{})

	assert.NoError(suite.T(), err)

	// get fees again with new actual values

	err = suite.service.db.Collection(pkg.CollectionSystemFees).Find(query).All(&fees)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), len(fees) == 1)

	assert.True(suite.T(), len(fees[0].Fees) == 1)
	assert.True(suite.T(), fees[0].Fees[0].TransactionCost.Percent == float64(2.35))
}

func (suite *SystemFeesTestSuite) TestSystemFees_AddSystemFees_Ok() {

	req := &billing.AddSystemFeesRequest{
		MethodId:  suite.paymentMethod.Id,
		Region:    "US",
		CardBrand: "MASTERCARD",
		Fees: []*billing.FeeSet{
			{
				MinAmounts: map[string]float64{"EUR": 0, "USD": 0},
				TransactionCost: &billing.SystemFee{
					Percent:         2.35,
					PercentCurrency: "EUR",
					FixAmount:       0.20,
					FixCurrency:     "EUR",
				},
				AuthorizationFee: systemFeeExample,
			},
		},
		UserId: suite.AdminUserId,
	}

	err := suite.service.AddSystemFees(context.TODO(), req, &grpc.EmptyResponse{})

	assert.NoError(suite.T(), err)
}

func (suite *SystemFeesTestSuite) TestSystemFees_AddSystemFees_Fail_CardBrandRequired() {

	req := &billing.AddSystemFeesRequest{
		MethodId:  suite.paymentMethod.Id,
		Region:    "",
		CardBrand: "",
		Fees: []*billing.FeeSet{
			{
				MinAmounts:       map[string]float64{"EUR": 0, "USD": 0},
				TransactionCost:  systemFeeExample,
				AuthorizationFee: systemFeeExample,
			},
		},
		UserId: suite.AdminUserId,
	}

	err := suite.service.AddSystemFees(context.TODO(), req, &grpc.EmptyResponse{})

	assert.EqualError(suite.T(), err, errorSystemFeeCardBrandRequired)
}

func (suite *SystemFeesTestSuite) TestSystemFees_AddSystemFees_Fail_CardBrandInvalid() {

	req := &billing.AddSystemFeesRequest{
		MethodId:  suite.paymentMethod.Id,
		CardBrand: "BLA-BLA-BLA",
		Fees: []*billing.FeeSet{
			{
				MinAmounts:       map[string]float64{"EUR": 0, "USD": 0},
				TransactionCost:  systemFeeExample,
				AuthorizationFee: systemFeeExample,
			},
		},
		UserId: suite.AdminUserId,
	}

	err := suite.service.AddSystemFees(context.TODO(), req, &grpc.EmptyResponse{})

	assert.EqualError(suite.T(), err, errorSystemFeeCardBrandInvalid)
}

func (suite *SystemFeesTestSuite) TestSystemFees_AddSystemFees_Fail_CardBrandNotAllowed() {

	req := &billing.AddSystemFeesRequest{
		MethodId:  suite.pmWebMoney.Id,
		CardBrand: "VISA",
		Fees: []*billing.FeeSet{
			{
				MinAmounts:       map[string]float64{"EUR": 0, "USD": 0},
				TransactionCost:  systemFeeExample,
				AuthorizationFee: systemFeeExample,
			},
		},
		UserId: suite.AdminUserId,
	}

	err := suite.service.AddSystemFees(context.TODO(), req, &grpc.EmptyResponse{})

	assert.EqualError(suite.T(), err, errorSystemFeeCardBrandNotAllowed)
}

func (suite *SystemFeesTestSuite) TestSystemFees_AddSystemFees_Fail_RegionInvalid() {

	req := &billing.AddSystemFeesRequest{
		MethodId: suite.pmWebMoney.Id,
		Region:   "BLAH",
		Fees: []*billing.FeeSet{
			{
				MinAmounts:       map[string]float64{"EUR": 0, "USD": 0},
				TransactionCost:  systemFeeExample,
				AuthorizationFee: systemFeeExample,
			},
		},
		UserId: suite.AdminUserId,
	}

	err := suite.service.AddSystemFees(context.TODO(), req, &grpc.EmptyResponse{})

	assert.EqualError(suite.T(), err, errorSystemFeeRegionInvalid)
}

func (suite *SystemFeesTestSuite) TestSystemFees_AddSystemFees_Fail_FeesetRequired() {

	req := &billing.AddSystemFeesRequest{
		MethodId: suite.pmWebMoney.Id,
		Fees:     []*billing.FeeSet{},
		UserId:   suite.AdminUserId,
	}

	err := suite.service.AddSystemFees(context.TODO(), req, &grpc.EmptyResponse{})

	assert.EqualError(suite.T(), err, errorSystemFeeRequiredFeeset)
}

func (suite *SystemFeesTestSuite) TestSystemFees_GetSystemFeesForPayment() {

	var sf = billing.FeeSet{}

	req := &billing.GetSystemFeesRequest{
		MethodId:  suite.paymentMethod.Id,
		Region:    "",
		CardBrand: "MASTERCARD",
		Amount:    1.01,
		Currency:  "USD",
	}
	err := suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), sf.TransactionCost.Percent == float64(1.15))

	req = &billing.GetSystemFeesRequest{
		MethodId:  suite.paymentMethod.Id,
		Region:    "",
		CardBrand: "MASTERCARD",
		Amount:    200,
		Currency:  "USD",
	}
	err = suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), sf.TransactionCost.Percent == float64(2.15))

	req = &billing.GetSystemFeesRequest{
		MethodId:  suite.paymentMethod.Id,
		Region:    "EU",
		CardBrand: "MASTERCARD",
		Amount:    200,
		Currency:  "USD",
	}
	err = suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), sf.TransactionCost.Percent == float64(2.15))

	req = &billing.GetSystemFeesRequest{
		MethodId:  suite.pmWebMoney.Id,
		Region:    "",
		CardBrand: "",
		Amount:    200,
		Currency:  "USD",
	}
	err = suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), sf.TransactionCost.Percent == float64(2.15))

	req = &billing.GetSystemFeesRequest{
		MethodId:  suite.pmWebMoney.Id,
		Region:    "US",
		CardBrand: "VISA",
		Amount:    200,
		Currency:  "USD",
	}
	err = suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.EqualError(suite.T(), err, errorSystemFeeNotFound)

	req = &billing.GetSystemFeesRequest{
		MethodId:  suite.pmWebMoney.Id,
		Region:    "US",
		CardBrand: "QWERTY",
		Amount:    200,
		Currency:  "USD",
	}
	err = suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.EqualError(suite.T(), err, errorSystemFeeNotFound)

	req = &billing.GetSystemFeesRequest{
		MethodId:  suite.pmWebMoney.Id,
		Region:    "US",
		CardBrand: "QWERTY",
		Amount:    200,
		Currency:  "BLA",
	}
	err = suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.EqualError(suite.T(), err, errorSystemFeeNotFound)

	req = &billing.GetSystemFeesRequest{
		MethodId:  suite.pmWebMoney.Id,
		Region:    "BLAH",
		CardBrand: "VISA",
		Amount:    200,
		Currency:  "USD",
	}
	err = suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.EqualError(suite.T(), err, errorSystemFeeNotFound)

	req = &billing.GetSystemFeesRequest{
		MethodId:  suite.paymentMethod.Id,
		Region:    "",
		CardBrand: "MASTERCARD",
		Amount:    200,
		Currency:  "BYR",
	}
	err = suite.service.GetSystemFeesForPayment(context.TODO(), req, &sf)
	assert.EqualError(suite.T(), err, errorSystemFeeMatchedMinAmountNotFound)
}
