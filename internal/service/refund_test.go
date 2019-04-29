package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"github.com/ProtocolONE/rabbitmq/pkg"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mock"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"testing"
	"time"
)

type RefundTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger

	project    *billing.Project
	pmBankCard *billing.PaymentMethod
}

func Test_Refund(t *testing.T) {
	suite.Run(t, new(RefundTestSuite))
}

func (suite *RefundTestSuite) SetupTest() {
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
	assert.NoError(suite.T(), err, "Database connection failed")

	suite.log, err = zap.NewProduction()
	assert.NoError(suite.T(), err, "Logger initialization failed")

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
		CurrencyTo:   643,
		Rate:         1,
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
		IsVatEnabled:              false,
		IsCommissionToUserEnabled: false,
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

	project := &billing.Project{
		Id:                       bson.NewObjectId().Hex(),
		CallbackCurrency:         rub.CodeA3,
		CallbackProtocol:         "default",
		LimitsCurrency:           rub.CodeA3,
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       false,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   pkg.ProjectStatusInProduction,
		MerchantId:               merchant.Id,
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
			Handler:    "mock_error",
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
	assert.NoError(suite.T(), err, "Insert payment methods test data failed")

	commissionStartDate, err := ptypes.TimestampProto(time.Now().Add(time.Minute * -10))
	assert.NoError(suite.T(), err, "Commission start date conversion failed")

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
	assert.NoError(suite.T(), err, "Insert commission test data failed")

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
	assert.NoError(suite.T(), err, "Insert merchant test data failed")

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

	suite.project = project
	suite.pmBankCard = pmBankCard
}

func (suite *RefundTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *RefundTestSuite) TestRefund_CreateRefund_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
	assert.NotNil(suite.T(), rsp2.Item)
	assert.NotEmpty(suite.T(), rsp2.Item.Id)
	assert.NotEmpty(suite.T(), rsp2.Item.ExternalId)
	assert.Equal(suite.T(), pkg.RefundStatusInProgress, rsp2.Item.Status)

	var refund *billing.Refund
	err = suite.service.db.Collection(pkg.CollectionRefund).FindId(bson.ObjectIdHex(rsp2.Item.Id)).One(&refund)
	assert.NotNil(suite.T(), refund)
	assert.Equal(suite.T(), pkg.RefundStatusInProgress, refund.Status)
}

func (suite *RefundTestSuite) TestRefund_CreateRefund_AmountLess_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   order.Uuid,
		Amount:    50,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Equal(suite.T(), pkg.RefundStatusInProgress, rsp2.Item.Status)

	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Equal(suite.T(), pkg.RefundStatusInProgress, rsp2.Item.Status)

	rsp3 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp3.Status)
	assert.Equal(suite.T(), refundErrorPaymentAmountLess, rsp3.Message)
	assert.Empty(suite.T(), rsp3.Item)
}

func (suite *RefundTestSuite) TestRefund_CreateRefund_PaymentSystemNotExists_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "not_exist_payment_system"
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp2.Status)
	assert.Equal(suite.T(), paymentSystemErrorHandlerNotFound, rsp2.Message)
	assert.Empty(suite.T(), rsp2.Item)
}

func (suite *RefundTestSuite) TestRefund_CreateRefund_PaymentSystemReturnError_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_error"
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp2.Status)
	assert.Equal(suite.T(), pkg.PaymentSystemErrorCreateRefundFailed, rsp2.Message)
	assert.Empty(suite.T(), rsp2.Item)
}

func (suite *RefundTestSuite) TestRefund_CreateRefundProcessor_ProcessOrder_OrderNotFound_Error() {
	processor := &createRefundProcessor{
		service: suite.service,
		request: &grpc.CreateRefundRequest{
			OrderId:   bson.NewObjectId().Hex(),
			Amount:    10,
			CreatorId: bson.NewObjectId().Hex(),
			Reason:    "unit test",
		},
		checked: &createRefundChecked{},
	}

	err := processor.processOrder()
	assert.Error(suite.T(), err)

	err1, ok := err.(*RefundError)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), pkg.ResponseStatusNotFound, err1.status)
	assert.Equal(suite.T(), orderErrorNotFound, err1.err)
}

func (suite *RefundTestSuite) TestRefund_CreateRefund_RefundNotAllowed_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp2.Status)
	assert.Equal(suite.T(), refundErrorNotAllowed, rsp2.Message)
}

func (suite *RefundTestSuite) TestRefund_CreateRefund_WasRefunded_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusRefund
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp2.Status)
	assert.Equal(suite.T(), refundErrorAlreadyRefunded, rsp2.Message)
}

func (suite *RefundTestSuite) TestRefund_ListRefunds_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusProjectComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
	assert.NotEmpty(suite.T(), rsp2.Item)

	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
	assert.NotEmpty(suite.T(), rsp2.Item)

	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
	assert.NotEmpty(suite.T(), rsp2.Item)

	req3 := &grpc.ListRefundsRequest{
		OrderId: order.Uuid,
		Limit:   100,
		Offset:  0,
	}
	rsp3 := &grpc.ListRefundsResponse{}
	err = suite.service.ListRefunds(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int32(3), rsp3.Count)
	assert.Len(suite.T(), rsp3.Items, int(rsp3.Count))
	assert.Equal(suite.T(), rsp2.Item.Id, rsp3.Items[2].Id)
}

func (suite *RefundTestSuite) TestRefund_ListRefunds_Limit_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusProjectComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
	assert.NotEmpty(suite.T(), rsp2.Item)

	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
	assert.NotEmpty(suite.T(), rsp2.Item)

	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
	assert.NotEmpty(suite.T(), rsp2.Item)

	req3 := &grpc.ListRefundsRequest{
		OrderId: order.Uuid,
		Limit:   1,
		Offset:  0,
	}
	rsp3 := &grpc.ListRefundsResponse{}
	err = suite.service.ListRefunds(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int32(3), rsp3.Count)
	assert.Len(suite.T(), rsp3.Items, int(req3.Limit))
}

func (suite *RefundTestSuite) TestRefund_ListRefunds_NoResults_Ok() {
	req3 := &grpc.ListRefundsRequest{
		OrderId: bson.NewObjectId().Hex(),
		Limit:   100,
		Offset:  0,
	}
	rsp3 := &grpc.ListRefundsResponse{}
	err := suite.service.ListRefunds(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int32(0), rsp3.Count)
	assert.Len(suite.T(), rsp3.Items, 0)
}

func (suite *RefundTestSuite) TestRefund_GetRefund_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusProjectComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)
	assert.NotEmpty(suite.T(), rsp2.Item)

	req3 := &grpc.GetRefundRequest{
		OrderId:  order.Uuid,
		RefundId: rsp2.Item.Id,
	}
	rsp3 := &grpc.CreateRefundResponse{}
	err = suite.service.GetRefund(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp3.Status)
	assert.Empty(suite.T(), rsp3.Message)
	assert.Equal(suite.T(), req3.OrderId, rsp3.Item.Order.Uuid)
	assert.Equal(suite.T(), req3.RefundId, rsp3.Item.Id)
}

func (suite *RefundTestSuite) TestRefund_GetRefund_NotFound_Error() {
	req3 := &grpc.GetRefundRequest{
		OrderId:  bson.NewObjectId().Hex(),
		RefundId: bson.NewObjectId().Hex(),
	}
	rsp3 := &grpc.CreateRefundResponse{}
	err := suite.service.GetRefund(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusNotFound, rsp3.Status)
	assert.Equal(suite.T(), refundErrorNotFound, rsp3.Message)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = pkg.PaymentSystemHandlerCardPay
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	refundReq := &billing.CardPayRefundCallback{
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id: rsp2.Item.Id,
		},
		PaymentMethod: order.PaymentMethod.Group,
		PaymentData: &billing.CardPayRefundCallbackPaymentData{
			Id:              rsp2.Item.Id,
			RemainingAmount: 90,
		},
		RefundData: &billing.CardPayRefundCallbackRefundData{
			Amount:   10,
			Created:  time.Now().Format(cardPayDateFormat),
			Id:       bson.NewObjectId().Hex(),
			Currency: rsp2.Item.Currency.CodeA3,
			Status:   pkg.CardPayPaymentResponseStatusCompleted,
			AuthCode: bson.NewObjectId().Hex(),
			Is_3D:    true,
			Rrn:      bson.NewObjectId().Hex(),
		},
		CallbackTime: time.Now().Format(cardPayDateFormat),
		Customer: &billing.CardPayCustomer{
			Email: order.PayerData.Email,
			Id:    order.PayerData.Email,
		},
	}

	b, err := json.Marshal(refundReq)
	assert.NoError(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(b) + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   pkg.PaymentSystemHandlerCardPay,
		Body:      b,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp3.Status)
	assert.Empty(suite.T(), rsp3.Error)

	var refund *billing.Refund
	err = suite.service.db.Collection(pkg.CollectionRefund).FindId(bson.ObjectIdHex(rsp2.Item.Id)).One(&refund)
	assert.NotNil(suite.T(), refund)
	assert.Equal(suite.T(), pkg.RefundStatusCompleted, refund.Status)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_UnmarshalError() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = pkg.PaymentSystemHandlerCardPay
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	refundReq := `{"some_field": "some_value"}`

	hash := sha512.New()
	hash.Write([]byte(refundReq + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   pkg.PaymentSystemHandlerCardPay,
		Body:      []byte(refundReq),
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp3.Status)
	assert.Equal(suite.T(), callbackRequestIncorrect, rsp3.Error)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_UnknownHandler_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = pkg.PaymentSystemHandlerCardPay
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	refundReq := &billing.CardPayRefundCallback{
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id: rsp2.Item.Id,
		},
		PaymentMethod: order.PaymentMethod.Group,
		PaymentData: &billing.CardPayRefundCallbackPaymentData{
			Id:              rsp2.Item.Id,
			RemainingAmount: 90,
		},
		RefundData: &billing.CardPayRefundCallbackRefundData{
			Amount:   10,
			Created:  time.Now().Format(cardPayDateFormat),
			Id:       bson.NewObjectId().Hex(),
			Currency: rsp2.Item.Currency.CodeA3,
			Status:   pkg.CardPayPaymentResponseStatusCompleted,
			AuthCode: bson.NewObjectId().Hex(),
			Is_3D:    true,
			Rrn:      bson.NewObjectId().Hex(),
		},
		CallbackTime: time.Now().Format(cardPayDateFormat),
		Customer: &billing.CardPayCustomer{
			Email: order.PayerData.Email,
			Id:    order.PayerData.Email,
		},
	}

	b, err := json.Marshal(refundReq)
	assert.NoError(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(b) + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   "fake_payment_system_handler",
		Body:      b,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp3.Status)
	assert.Equal(suite.T(), callbackHandlerIncorrect, rsp3.Error)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_RefundNotFound_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = pkg.PaymentSystemHandlerCardPay
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	refundReq := &billing.CardPayRefundCallback{
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id: bson.NewObjectId().Hex(),
		},
		PaymentMethod: order.PaymentMethod.Group,
		PaymentData: &billing.CardPayRefundCallbackPaymentData{
			Id:              bson.NewObjectId().Hex(),
			RemainingAmount: 90,
		},
		RefundData: &billing.CardPayRefundCallbackRefundData{
			Amount:   10,
			Created:  time.Now().Format(cardPayDateFormat),
			Id:       bson.NewObjectId().Hex(),
			Currency: rsp2.Item.Currency.CodeA3,
			Status:   pkg.CardPayPaymentResponseStatusCompleted,
			AuthCode: bson.NewObjectId().Hex(),
			Is_3D:    true,
			Rrn:      bson.NewObjectId().Hex(),
		},
		CallbackTime: time.Now().Format(cardPayDateFormat),
		Customer: &billing.CardPayCustomer{
			Email: order.PayerData.Email,
			Id:    order.PayerData.Email,
		},
	}

	b, err := json.Marshal(refundReq)
	assert.NoError(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(b) + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   pkg.PaymentSystemHandlerCardPay,
		Body:      b,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusNotFound, rsp3.Status)
	assert.Equal(suite.T(), refundErrorNotFound, rsp3.Error)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_OrderNotFound_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = pkg.PaymentSystemHandlerCardPay
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	var refund *billing.Refund
	err = suite.service.db.Collection(pkg.CollectionRefund).FindId(bson.ObjectIdHex(rsp2.Item.Id)).One(&refund)
	assert.NotNil(suite.T(), refund)

	refund.Order = &billing.RefundOrder{Id: bson.NewObjectId().Hex(), Uuid: uuid.New().String()}
	err = suite.service.db.Collection(pkg.CollectionRefund).UpdateId(bson.ObjectIdHex(refund.Id), refund)

	refundReq := &billing.CardPayRefundCallback{
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id: rsp2.Item.Id,
		},
		PaymentMethod: order.PaymentMethod.Group,
		PaymentData: &billing.CardPayRefundCallbackPaymentData{
			Id:              rsp2.Item.Id,
			RemainingAmount: 90,
		},
		RefundData: &billing.CardPayRefundCallbackRefundData{
			Amount:   10,
			Created:  time.Now().Format(cardPayDateFormat),
			Id:       bson.NewObjectId().Hex(),
			Currency: rsp2.Item.Currency.CodeA3,
			Status:   pkg.CardPayPaymentResponseStatusCompleted,
			AuthCode: bson.NewObjectId().Hex(),
			Is_3D:    true,
			Rrn:      bson.NewObjectId().Hex(),
		},
		CallbackTime: time.Now().Format(cardPayDateFormat),
		Customer: &billing.CardPayCustomer{
			Email: order.PayerData.Email,
			Id:    order.PayerData.Email,
		},
	}

	b, err := json.Marshal(refundReq)
	assert.NoError(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(b) + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   pkg.PaymentSystemHandlerCardPay,
		Body:      b,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusNotFound, rsp3.Status)
	assert.Equal(suite.T(), refundErrorOrderNotFound, rsp3.Error)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_UnknownPaymentSystemHandler_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = "fake_payment_system_handler"
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	refundReq := &billing.CardPayRefundCallback{
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id: rsp2.Item.Id,
		},
		PaymentMethod: order.PaymentMethod.Group,
		PaymentData: &billing.CardPayRefundCallbackPaymentData{
			Id:              rsp2.Item.Id,
			RemainingAmount: 90,
		},
		RefundData: &billing.CardPayRefundCallbackRefundData{
			Amount:   10,
			Created:  time.Now().Format(cardPayDateFormat),
			Id:       bson.NewObjectId().Hex(),
			Currency: rsp2.Item.Currency.CodeA3,
			Status:   pkg.CardPayPaymentResponseStatusCompleted,
			AuthCode: bson.NewObjectId().Hex(),
			Is_3D:    true,
			Rrn:      bson.NewObjectId().Hex(),
		},
		CallbackTime: time.Now().Format(cardPayDateFormat),
		Customer: &billing.CardPayCustomer{
			Email: order.PayerData.Email,
			Id:    order.PayerData.Email,
		},
	}

	b, err := json.Marshal(refundReq)
	assert.NoError(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(b) + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   pkg.PaymentSystemHandlerCardPay,
		Body:      b,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusSystemError, rsp3.Status)
	assert.Equal(suite.T(), orderErrorUnknown, rsp3.Error)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_ProcessRefundError() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = pkg.PaymentSystemHandlerCardPay
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	refundReq := &billing.CardPayRefundCallback{
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id: rsp2.Item.Id,
		},
		PaymentMethod: order.PaymentMethod.Group,
		PaymentData: &billing.CardPayRefundCallbackPaymentData{
			Id:              rsp2.Item.Id,
			RemainingAmount: 90,
		},
		RefundData: &billing.CardPayRefundCallbackRefundData{
			Amount:   10000,
			Created:  time.Now().Format(cardPayDateFormat),
			Id:       bson.NewObjectId().Hex(),
			Currency: rsp2.Item.Currency.CodeA3,
			Status:   pkg.CardPayPaymentResponseStatusCompleted,
			AuthCode: bson.NewObjectId().Hex(),
			Is_3D:    true,
			Rrn:      bson.NewObjectId().Hex(),
		},
		CallbackTime: time.Now().Format(cardPayDateFormat),
		Customer: &billing.CardPayCustomer{
			Email: order.PayerData.Email,
			Id:    order.PayerData.Email,
		},
	}

	b, err := json.Marshal(refundReq)
	assert.NoError(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(b) + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   pkg.PaymentSystemHandlerCardPay,
		Body:      b,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp3.Status)
	assert.Equal(suite.T(), paymentSystemErrorRefundRequestAmountOrCurrencyIsInvalid, rsp3.Error)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_TemporaryStatus_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    10,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = pkg.PaymentSystemHandlerCardPay
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	refundReq := &billing.CardPayRefundCallback{
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id: rsp2.Item.Id,
		},
		PaymentMethod: order.PaymentMethod.Group,
		PaymentData: &billing.CardPayRefundCallbackPaymentData{
			Id:              rsp2.Item.Id,
			RemainingAmount: 90,
		},
		RefundData: &billing.CardPayRefundCallbackRefundData{
			Amount:   10,
			Created:  time.Now().Format(cardPayDateFormat),
			Id:       bson.NewObjectId().Hex(),
			Currency: rsp2.Item.Currency.CodeA3,
			Status:   pkg.CardPayPaymentResponseStatusAuthorized,
			AuthCode: bson.NewObjectId().Hex(),
			Is_3D:    true,
			Rrn:      bson.NewObjectId().Hex(),
		},
		CallbackTime: time.Now().Format(cardPayDateFormat),
		Customer: &billing.CardPayCustomer{
			Email: order.PayerData.Email,
			Id:    order.PayerData.Email,
		},
	}

	b, err := json.Marshal(refundReq)
	assert.NoError(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(b) + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   pkg.PaymentSystemHandlerCardPay,
		Body:      b,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp3.Status)
	assert.Equal(suite.T(), paymentSystemErrorRequestTemporarySkipped, rsp3.Error)

	var refund *billing.Refund
	err = suite.service.db.Collection(pkg.CollectionRefund).FindId(bson.ObjectIdHex(rsp2.Item.Id)).One(&refund)
	assert.NotNil(suite.T(), refund)
	assert.Equal(suite.T(), pkg.RefundStatusInProgress, refund.Status)
}

func (suite *RefundTestSuite) TestRefund_ProcessRefundCallback_OrderFullyRefunded_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "some_email@unit.com",
		PayerIp:     "127.0.0.1",
		PayerPhone:  "123456789",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBankCard.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.Status = constant.OrderStatusPaymentSystemComplete
	order.PaymentMethod.Params.Handler = "mock_ok"
	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Rate:     20,
		Amount:   10,
		Currency: "RUB",
	}
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	req2 := &grpc.CreateRefundRequest{
		OrderId:   rsp.Uuid,
		Amount:    100,
		CreatorId: bson.NewObjectId().Hex(),
		Reason:    "unit test",
	}
	rsp2 := &grpc.CreateRefundResponse{}
	err = suite.service.CreateRefund(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp2.Status)
	assert.Empty(suite.T(), rsp2.Message)

	order.PaymentMethod.Params.Handler = pkg.PaymentSystemHandlerCardPay
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	refundReq := &billing.CardPayRefundCallback{
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id: rsp2.Item.Id,
		},
		PaymentMethod: order.PaymentMethod.Group,
		PaymentData: &billing.CardPayRefundCallbackPaymentData{
			Id:              rsp2.Item.Id,
			RemainingAmount: 0,
		},
		RefundData: &billing.CardPayRefundCallbackRefundData{
			Amount:   100,
			Created:  time.Now().Format(cardPayDateFormat),
			Id:       bson.NewObjectId().Hex(),
			Currency: rsp2.Item.Currency.CodeA3,
			Status:   pkg.CardPayPaymentResponseStatusCompleted,
			AuthCode: bson.NewObjectId().Hex(),
			Is_3D:    true,
			Rrn:      bson.NewObjectId().Hex(),
		},
		CallbackTime: time.Now().Format(cardPayDateFormat),
		Customer: &billing.CardPayCustomer{
			Email: order.PayerData.Email,
			Id:    order.PayerData.Email,
		},
	}

	b, err := json.Marshal(refundReq)
	assert.NoError(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(b) + order.PaymentMethod.Params.CallbackPassword))

	req3 := &grpc.CallbackRequest{
		Handler:   pkg.PaymentSystemHandlerCardPay,
		Body:      b,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}
	rsp3 := &grpc.PaymentNotifyResponse{}
	err = suite.service.ProcessRefundCallback(context.TODO(), req3, rsp3)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp3.Status)
	assert.Empty(suite.T(), rsp3.Error)

	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp.Id)).One(&order)
	assert.Equal(suite.T(), int32(constant.OrderStatusRefund), order.Status)
}
