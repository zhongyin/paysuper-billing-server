package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	"github.com/paysuper/paysuper-recurring-repository/tools"
	"github.com/stoewer/go-strcase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

type OrderTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger

	project                                *billing.Project
	projectFixedAmount                     *billing.Project
	inactiveProject                        *billing.Project
	projectWithoutPaymentMethods           *billing.Project
	projectIncorrectPaymentMethodId        *billing.Project
	projectEmptyPaymentMethodTerminal      *billing.Project
	projectUahLimitCurrency                *billing.Project
	paymentMethod                          *billing.PaymentMethod
	inactivePaymentMethod                  *billing.PaymentMethod
	paymentMethodWithInactivePaymentSystem *billing.PaymentMethod
	pmWebMoney                             *billing.PaymentMethod
	pmBitcoin1                             *billing.PaymentMethod
	productIds                             []string
	merchantDefaultCurrency                string
}

func Test_Order(t *testing.T) {
	suite.Run(t, new(OrderTestSuite))
}

func (suite *OrderTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	if err != nil {
		suite.FailNow("Config load failed", "%v", err)
	}
	cfg.AccountingCurrency = "RUB"
	cfg.CardPayApiUrl = "https://sandbox.cardpay.com"

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
	uah := &billing.Currency{
		CodeInt:  980,
		CodeA3:   "UAH",
		Name:     &billing.Name{Ru: "Украинская гривна", En: "Ukrainian Hryvnia"},
		IsActive: true,
	}
	eur := &billing.Currency{
		CodeInt:  978,
		CodeA3:   "EUR",
		Name:     &billing.Name{Ru: "Евро", En: "Euro"},
		IsActive: true,
	}
	amd := &billing.Currency{
		CodeInt:  51,
		CodeA3:   "AMD",
		Name:     &billing.Name{Ru: "Армянский драм", En: "Armenian dram"},
		IsActive: true,
	}

	currency := []interface{}{rub, usd, uah, amd}

	err = db.Collection(pkg.CollectionCurrency).Insert(currency...)

	if err != nil {
		suite.FailNow("Insert currency test data failed", "%v", err)
	}

	rate := []interface{}{
		&billing.CurrencyRate{
			CurrencyFrom: 840,
			CurrencyTo:   643,
			Rate:         0.015625,
			Date:         ptypes.TimestampNow(),
			IsActive:     true,
		},
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
		&billing.CurrencyRate{
			CurrencyFrom: 643,
			CurrencyTo:   51,
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
	us := &billing.Country{
		CodeInt:  840,
		CodeA2:   "US",
		CodeA3:   "USA",
		Name:     &billing.Name{Ru: "США", En: "USA"},
		IsActive: true,
	}
	by := &billing.Country{
		CodeInt:  111,
		CodeA2:   "BY",
		CodeA3:   "BYR",
		Name:     &billing.Name{Ru: "Белоруссия", En: "Belarus"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCountry).Insert([]interface{}{ru, us, by}...)
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

	pmBitcoin1 := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "Bitcoin",
		Group:            "BITCOIN_1",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		Currency:         rub,
		Currencies:       []int32{643, 840, 980},
		Params: &billing.PaymentMethodParams{
			Handler:    "unit_test",
			Terminal:   "16007",
			ExternalId: "BITCOIN",
		},
		Type:     "crypto",
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
		Type:          "ewallet",
		IsActive:      true,
		AccountRegexp: "^\\d{1,15}",
		PaymentSystem: &billing.PaymentSystem{
			Id:                 bson.NewObjectId().Hex(),
			Name:               "CardPay 2",
			AccountingCurrency: uah,
			AccountingPeriod:   "every-day",
			Country:            &billing.Country{},
			IsActive:           false,
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
			pmBitcoin1.Id: {
				PaymentMethod: &billing.MerchantPaymentMethodIdentification{
					Id:   pmBitcoin1.Id,
					Name: pmBitcoin1.Name,
				},
				Commission: &billing.MerchantPaymentMethodCommissions{
					Fee: 3.5,
					PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
						Fee:      300,
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
			pmQiwi.Id: {
				PaymentMethod: &billing.MerchantPaymentMethodIdentification{
					Id:   pmQiwi.Id,
					Name: pmQiwi.Name,
				},
				Commission: &billing.MerchantPaymentMethodCommissions{
					Fee: 3.5,
					PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
						Fee:      300,
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
			Amount: 100000,
		},
		IsSigned: false,
	}

	err = db.Collection(pkg.CollectionMerchant).Insert([]interface{}{merchant, merchantAgreement, merchant1}...)

	if err != nil {
		suite.FailNow("Insert merchant test data failed", "%v", err)
	}

	project := &billing.Project{
		Id:                       bson.NewObjectId().Hex(),
		CallbackCurrency:         rub,
		CallbackProtocol:         "default",
		LimitsCurrency:           usd,
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     "test project 1",
		IsProductsCheckout:       false,
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
	projectFixedAmount := &billing.Project{
		Id:                       bson.NewObjectId().Hex(),
		CallbackCurrency:         rub,
		CallbackProtocol:         "default",
		LimitsCurrency:           usd,
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
		IsActive: true,
		Merchant: merchant,
	}
	projectUahLimitCurrency := &billing.Project{
		Id:                 bson.NewObjectId().Hex(),
		CallbackCurrency:   rub,
		CallbackProtocol:   "default",
		LimitsCurrency:     uah,
		MaxPaymentAmount:   15000,
		MinPaymentAmount:   0,
		Name:               "project uah limit currency",
		IsProductsCheckout: true,
		SecretKey:          "project uah limit currency secret key",
		PaymentMethods: map[string]*billing.ProjectPaymentMethod{
			"BANKCARD": {
				Id:        pmBankCard.Id,
				Terminal:  "terminal",
				Password:  "password",
				CreatedAt: ptypes.TimestampNow(),
			},
		},
		IsActive: true,
		Merchant: &billing.Merchant{
			Id:   bson.NewObjectId().Hex(),
			Name: "Unit test",
			Country: &billing.Country{
				CodeInt:  643,
				CodeA2:   "RU",
				CodeA3:   "RUS",
				Name:     &billing.Name{Ru: "Россия", En: "Russia (Russian Federation)"},
				IsActive: true,
			},
			Zip:  "190000",
			City: "St.Petersburg",
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
				Currency: amd,
				Name:     "Bank name",
			},
			IsVatEnabled:              true,
			IsCommissionToUserEnabled: true,
			Status:                    1,
		},
	}
	projectIncorrectPaymentMethodId := &billing.Project{
		Id:                 bson.NewObjectId().Hex(),
		CallbackCurrency:   rub,
		CallbackProtocol:   "default",
		LimitsCurrency:     rub,
		MaxPaymentAmount:   15000,
		MinPaymentAmount:   0,
		Name:               "project incorrect payment method id",
		IsProductsCheckout: true,
		SecretKey:          "project incorrect payment method id secret key",
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
			Id:   bson.NewObjectId().Hex(),
			Name: "Unit test",
			Country: &billing.Country{
				CodeInt:  643,
				CodeA2:   "RU",
				CodeA3:   "RUS",
				Name:     &billing.Name{Ru: "Россия", En: "Russia (Russian Federation)"},
				IsActive: true,
			},
			Zip:  "190000",
			City: "St.Petersburg",
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
				Currency: uah,
				Name:     "Bank name",
			},
			IsVatEnabled:              true,
			IsCommissionToUserEnabled: true,
			Status:                    1,
		},
	}
	projectEmptyPaymentMethodTerminal := &billing.Project{
		Id:                 bson.NewObjectId().Hex(),
		CallbackCurrency:   rub,
		CallbackProtocol:   "default",
		LimitsCurrency:     rub,
		MaxPaymentAmount:   15000,
		MinPaymentAmount:   0,
		Name:               "project incorrect payment method id",
		IsProductsCheckout: false,
		SecretKey:          "project incorrect payment method id secret key",
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
			Id:   bson.NewObjectId().Hex(),
			Name: "Unit test",
			Country: &billing.Country{
				CodeInt:  643,
				CodeA2:   "RU",
				CodeA3:   "RUS",
				Name:     &billing.Name{Ru: "Россия", En: "Russia (Russian Federation)"},
				IsActive: true,
			},
			Zip:  "190000",
			City: "St.Petersburg",
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
				Currency: uah,
				Name:     "Bank name",
			},
			IsVatEnabled:              false,
			IsCommissionToUserEnabled: false,
			Status:                    1,
		},
	}
	projectWithoutPaymentMethods := &billing.Project{
		Id:                 bson.NewObjectId().Hex(),
		CallbackCurrency:   rub,
		CallbackProtocol:   "default",
		LimitsCurrency:     rub,
		MaxPaymentAmount:   15000,
		MinPaymentAmount:   0,
		Name:               "test project 1",
		IsProductsCheckout: true,
		SecretKey:          "test project 1 secret key",
		IsActive:           true,
	}
	inactiveProject := &billing.Project{
		Id:                 bson.NewObjectId().Hex(),
		CallbackCurrency:   rub,
		CallbackProtocol:   "xsolla",
		LimitsCurrency:     rub,
		MaxPaymentAmount:   15000,
		MinPaymentAmount:   0,
		Name:               "test project 2",
		IsProductsCheckout: true,
		SecretKey:          "test project 2 secret key",
		IsActive:           false,
	}

	projects := []interface{}{
		project,
		projectFixedAmount,
		inactiveProject,
		projectWithoutPaymentMethods,
		projectIncorrectPaymentMethodId,
		projectEmptyPaymentMethodTerminal,
		projectUahLimitCurrency,
	}

	err = db.Collection(pkg.CollectionProject).Insert(projects...)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
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
	pmWebMoneyWME := &billing.PaymentMethod{
		Id:               bson.NewObjectId().Hex(),
		Name:             "WebMoney WME",
		Group:            "WEBMONEY_WME",
		MinPaymentAmount: 0,
		MaxPaymentAmount: 0,
		Currency:         eur,
		Currencies:       []int32{978},
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

	pms := []interface{}{pmBankCard, pmQiwi, pmBitcoin, pmWebMoney, pmWebMoneyWME, pmBitcoin1}

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
		&billing.Commission{
			PaymentMethodId:         pmWebMoney.Id,
			ProjectId:               project.Id,
			PaymentMethodCommission: 1,
			PspCommission:           2,
			TotalCommissionToUser:   3,
			StartDate:               commissionStartDate,
		},
		&billing.Commission{
			PaymentMethodId:         pmBitcoin1.Id,
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

	err = db.Collection(pkg.CollectionCommission).Insert(commissions...)

	if err != nil {
		suite.FailNow("Insert commission test data failed", "%v", err)
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
	zap.ReplaceGlobals(suite.log)

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

	var productIds []string

	names := []string{"Madalin Stunt Cars M2", "Plants vs Zombies"}

	for i, n := range names {
		req := &grpc.Product{
			Object:          "product",
			Type:            "simple_product",
			Sku:             "ru_" + strconv.Itoa(i) + "_" + strcase.SnakeCase(n),
			Name:            map[string]string{"en": n},
			DefaultCurrency: "USD",
			Enabled:         true,
			Description:     map[string]string{"en": n + " description"},
			MerchantId:      projectFixedAmount.Merchant.Id,
			ProjectId:       projectFixedAmount.Id,
		}

		baseAmount := 37.00 * float64(i+1) // base amount in product's default currency

		req.Prices = append(req.Prices, &grpc.ProductPrice{
			Currency: "USD",
			Amount:   baseAmount,
		})
		req.Prices = append(req.Prices, &grpc.ProductPrice{
			Currency: "RUB",
			Amount:   baseAmount * 65.13,
		})

		prod := grpc.Product{}

		assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req, &prod))

		productIds = append(productIds, prod.Id)
	}

	suite.project = project
	suite.projectFixedAmount = projectFixedAmount
	suite.inactiveProject = inactiveProject
	suite.projectWithoutPaymentMethods = projectWithoutPaymentMethods
	suite.projectIncorrectPaymentMethodId = projectIncorrectPaymentMethodId
	suite.projectEmptyPaymentMethodTerminal = projectEmptyPaymentMethodTerminal
	suite.projectUahLimitCurrency = projectUahLimitCurrency
	suite.paymentMethod = pmBankCard
	suite.inactivePaymentMethod = pmBitcoin
	suite.paymentMethodWithInactivePaymentSystem = pmQiwi
	suite.pmWebMoney = pmWebMoney
	suite.pmBitcoin1 = pmBitcoin1
	suite.productIds = productIds
	suite.merchantDefaultCurrency = "USD"

}

func (suite *OrderTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
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
	assert.NotEmpty(suite.T(), processor.checked.payerData.State)
	assert.Empty(suite.T(), processor.checked.payerData.Email)
	assert.Empty(suite.T(), processor.checked.payerData.Phone)
}

func (suite *OrderTestSuite) TestOrder_ProcessPayerData_EmptySubdivision_Ok() {
	suite.service.geo = mock.NewGeoIpServiceTestOkWithoutSubdivision()

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
	assert.Empty(suite.T(), processor.checked.payerData.State)

	suite.service.geo = mock.NewGeoIpServiceTestOk()
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
	suite.service.geo = mock.NewGeoIpServiceTestError()

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

func (suite *OrderTestSuite) TestOrder_ValidateProductsForOrder_Ok() {
	_, err := suite.service.GetOrderProducts(suite.projectFixedAmount.Id, suite.productIds)
	assert.Nil(suite.T(), err)
}

func (suite *OrderTestSuite) TestOrder_ValidateProductsForOrder_AnotherProject_Fail() {
	_, err := suite.service.GetOrderProducts(suite.project.Id, suite.productIds)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorProductsInvalid, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ValidateProductsForOrder_OneProductIsInactive_Fail() {

	n := "Bubble Hunter"
	baseAmount := 7.00

	req := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_3_" + strcase.SnakeCase(n),
		Name:            map[string]string{"en": n},
		DefaultCurrency: "USD",
		Enabled:         false,
		Description:     map[string]string{"en": n + " description"},
		MerchantId:      suite.projectFixedAmount.Merchant.Id,
		ProjectId:       suite.projectFixedAmount.Id,
		Prices: []*grpc.ProductPrice{
			{
				Currency: "USD",
				Amount:   baseAmount,
			},
			{
				Currency: "RUB",
				Amount:   baseAmount * 65.13,
			},
		},
	}

	inactiveProd := grpc.Product{}
	if assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req, &inactiveProd)) {
		products := []string{suite.productIds[0], inactiveProd.Id}
		_, err := suite.service.GetOrderProducts(suite.projectFixedAmount.Id, products)
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), orderErrorProductsInvalid, err.Error())
	}
}

func (suite *OrderTestSuite) TestOrder_ValidateProductsForOrder_SomeProductsIsNotFound_Fail() {
	products := []string{suite.productIds[0], bson.NewObjectId().Hex()}
	_, err := suite.service.GetOrderProducts(suite.projectFixedAmount.Id, products)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorProductsInvalid, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ValidateProductsForOrder_EmptyProducts_Fail() {
	_, err := suite.service.GetOrderProducts("", []string{})
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorProductsEmpty, err.Error())
}

func (suite *OrderTestSuite) TestOrder_GetProductsOrderAmount_Ok() {
	p, err := suite.service.GetOrderProducts(suite.projectFixedAmount.Id, suite.productIds)
	assert.Nil(suite.T(), err)

	amount, err := suite.service.GetOrderProductsAmount(p, suite.merchantDefaultCurrency)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), amount, float64(111))
}

func (suite *OrderTestSuite) TestOrder_GetProductsOrderAmount_EmptyProducts_Fail() {
	_, err := suite.service.GetOrderProductsAmount([]*grpc.Product{}, suite.merchantDefaultCurrency)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorProductsEmpty, err.Error())
}

func (suite *OrderTestSuite) TestOrder_GetProductsOrderAmount_DifferentCurrencies_Fail() {
	n1 := "Bubble Hunter"
	baseAmount1 := 7.00
	req1 := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_4_" + strcase.SnakeCase(n1),
		Name:            map[string]string{"en": n1},
		DefaultCurrency: "USD",
		Enabled:         false,
		Description:     map[string]string{"en": n1 + " description"},
		MerchantId:      suite.projectFixedAmount.Merchant.Id,
		ProjectId:       suite.projectFixedAmount.Id,
		Prices: []*grpc.ProductPrice{
			{
				Currency: "USD",
				Amount:   baseAmount1,
			},
			{
				Currency: "RUB",
				Amount:   baseAmount1 * 0.89,
			},
		},
	}
	prod1 := grpc.Product{}
	assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req1, &prod1))

	n2 := "Scary Maze"
	baseAmount2 := 8.00
	req2 := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_5_" + strcase.SnakeCase(n2),
		Name:            map[string]string{"en": n2},
		DefaultCurrency: "USD",
		Enabled:         false,
		Description:     map[string]string{"en": n2 + " description"},
		MerchantId:      suite.projectFixedAmount.Merchant.Id,
		ProjectId:       suite.projectFixedAmount.Id,
		Prices: []*grpc.ProductPrice{
			{
				Currency: "USD",
				Amount:   baseAmount2,
			},
			{
				Currency: "EUR",
				Amount:   baseAmount2 * 0.89,
			},
		},
	}
	prod2 := grpc.Product{}
	assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req2, &prod2))

	p := []*grpc.Product{&prod1, &prod2}

	_, err := suite.service.GetOrderProductsAmount(p, "RUB")
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorNoProductsCommonCurrency, err.Error())
}

func (suite *OrderTestSuite) TestOrder_GetProductsOrderAmount_DifferentCurrenciesWithFallback_Fail() {
	n1 := "Bubble Hunter"
	baseAmount1 := 7.00
	req1 := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_6_" + strcase.SnakeCase(n1),
		Name:            map[string]string{"en": n1},
		DefaultCurrency: "EUR",
		Enabled:         false,
		Description:     map[string]string{"en": n1 + " description"},
		MerchantId:      suite.projectFixedAmount.Merchant.Id,
		ProjectId:       suite.projectFixedAmount.Id,
		Prices: []*grpc.ProductPrice{
			{
				Currency: "EUR",
				Amount:   baseAmount1,
			},
			{
				Currency: "UAH",
				Amount:   baseAmount1 * 30.21,
			},
		},
	}
	prod1 := grpc.Product{}
	assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req1, &prod1))

	n2 := "Scary Maze"
	baseAmount2 := 8.00
	req2 := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_7_" + strcase.SnakeCase(n2),
		Name:            map[string]string{"en": n2},
		DefaultCurrency: "EUR",
		Enabled:         false,
		Description:     map[string]string{"en": n2 + " description"},
		MerchantId:      suite.projectFixedAmount.Merchant.Id,
		ProjectId:       suite.projectFixedAmount.Id,
		Prices: []*grpc.ProductPrice{
			{
				Currency: "EUR",
				Amount:   baseAmount2,
			},
			{
				Currency: "UAH",
				Amount:   baseAmount2 * 30.21,
			},
		},
	}
	prod2 := grpc.Product{}
	assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req2, &prod2))

	p := []*grpc.Product{&prod1, &prod2}

	_, err := suite.service.GetOrderProductsAmount(p, "RUB")
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorNoProductsCommonCurrency, err.Error())
}

func (suite *OrderTestSuite) TestOrder_GetOrderProductsItems_Ok() {
	p, err := suite.service.GetOrderProducts(suite.projectFixedAmount.Id, suite.productIds)
	assert.Nil(suite.T(), err)

	items, err := suite.service.GetOrderProductsItems(p, DefaultLanguage, suite.merchantDefaultCurrency)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), len(items), 2)
}

func (suite *OrderTestSuite) TestOrder_GetOrderProductsItems_EmptyProducts_Fail() {
	_, err := suite.service.GetOrderProductsItems([]*grpc.Product{}, DefaultLanguage, suite.merchantDefaultCurrency)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), orderErrorProductsEmpty, err.Error())
}

func (suite *OrderTestSuite) TestOrder_GetOrderProductsItems_DifferentCurrencies_Fail() {
	n1 := "Bubble Hunter"
	baseAmount1 := 7.00
	req1 := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_8_" + strcase.SnakeCase(n1),
		Name:            map[string]string{"en": n1},
		DefaultCurrency: "USD",
		Enabled:         false,
		Description:     map[string]string{"en": n1 + " description"},
		MerchantId:      suite.projectFixedAmount.Merchant.Id,
		ProjectId:       suite.projectFixedAmount.Id,
		Prices: []*grpc.ProductPrice{
			{
				Currency: "USD",
				Amount:   baseAmount1,
			},
			{
				Currency: "RUB",
				Amount:   baseAmount1 * 0.89,
			},
		},
	}
	prod1 := grpc.Product{}
	assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req1, &prod1))

	n2 := "Scary Maze"
	baseAmount2 := 8.00
	req2 := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_9_" + strcase.SnakeCase(n2),
		Name:            map[string]string{"en": n2},
		DefaultCurrency: "USD",
		Enabled:         false,
		Description:     map[string]string{"en": n2 + " description"},
		MerchantId:      suite.projectFixedAmount.Merchant.Id,
		ProjectId:       suite.projectFixedAmount.Id,
		Prices: []*grpc.ProductPrice{
			{
				Currency: "USD",
				Amount:   baseAmount2,
			},
			{
				Currency: "EUR",
				Amount:   baseAmount2 * 0.89,
			},
		},
	}
	prod2 := grpc.Product{}
	assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req2, &prod2))

	p := []*grpc.Product{&prod1, &prod2}

	_, err := suite.service.GetOrderProductsItems(p, DefaultLanguage, "EUR")
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf("no price in currency %s", "EUR"), err.Error())
}

func (suite *OrderTestSuite) TestOrder_GetOrderProductsItems_ProductHasNoDescInSelectedLanguageButFallback_Fail() {
	n1 := "Bubble Hunter"
	baseAmount1 := 7.00
	req1 := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_8_" + strcase.SnakeCase(n1),
		Name:            map[string]string{"en": n1},
		DefaultCurrency: "USD",
		Enabled:         false,
		Description:     map[string]string{"en": n1 + " description"},
		MerchantId:      suite.projectFixedAmount.Merchant.Id,
		ProjectId:       suite.projectFixedAmount.Id,
		Prices: []*grpc.ProductPrice{
			{
				Currency: "USD",
				Amount:   baseAmount1,
			},
			{
				Currency: "RUB",
				Amount:   baseAmount1 * 0.89,
			},
		},
	}
	prod1 := grpc.Product{}
	assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req1, &prod1))

	p := []*grpc.Product{&prod1}

	items, err := suite.service.GetOrderProductsItems(p, "ru", suite.merchantDefaultCurrency)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), len(items), 1)
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

	err = processor.processProducts()
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
		AmountInMerchantAccountingCurrency: tools.FormatAmount(req.Amount),
		PaymentMethodOutcomeAmount:         req.Amount,
		PaymentMethodOutcomeCurrency:       processor.checked.currency,
		PaymentMethodIncomeAmount:          req.Amount,
		PaymentMethodIncomeCurrency:        processor.checked.currency,
	}

	err = suite.service.db.Collection(pkg.CollectionOrder).Insert(order)
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

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.paymentMethod)
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

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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

	suite.service.cfg.Environment = environmentProd

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotAllowed, err.Error())

	suite.service.cfg.Environment = "dev"
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

	suite.service.cfg.Environment = environmentProd

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotAllowed, err.Error())

	suite.service.cfg.Environment = "dev"
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

	suite.service.cfg.Environment = environmentProd

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodIncompatible, err.Error())

	suite.service.cfg.Environment = "dev"
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

	suite.service.cfg.Environment = environmentProd

	err := processor.processProject()
	assert.Nil(suite.T(), err)

	err = processor.processCurrency()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodEmptySettings, err.Error())

	suite.service.cfg.Environment = "dev"
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

	processor.processAmount()

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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

	processor.processAmount()

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrencyRate), err.Error())
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

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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

	processor.processAmount()

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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

	processor.processAmount()

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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

	processor.processAmount()

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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
		ProjectId:   suite.projectFixedAmount.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
		UrlSuccess:  "https://unit.test",
		UrlFail:     "https://unit.test",
		Products:    suite.productIds,
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)

	order, err := processor.prepareOrder()
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), order)
	assert.Equal(suite.T(), req.UrlFail, order.Project.UrlFail)
	assert.Equal(suite.T(), req.UrlSuccess, order.Project.UrlSuccess)
}

func (suite *OrderTestSuite) TestOrder_PrepareOrder_PaymentMethod_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectFixedAmount.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
		Products:      suite.productIds,
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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
}

func (suite *OrderTestSuite) TestOrder_PrepareOrder_UrlVerify_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.projectFixedAmount.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
		UrlNotify:   "https://unit.test",
		UrlVerify:   "https://unit.test",
		Products:    suite.productIds,
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)

	order, err := processor.prepareOrder()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order)
	assert.Equal(suite.T(), orderErrorDynamicNotifyUrlsNotAllowed, err.Error())
}

func (suite *OrderTestSuite) TestOrder_PrepareOrder_UrlRedirect_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.projectFixedAmount.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
		UrlFail:     "https://unit.test",
		UrlSuccess:  "https://unit.test",
		Products:    suite.productIds,
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)

	processor.checked.project = suite.projectUahLimitCurrency

	order, err := processor.prepareOrder()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order)
	assert.Equal(suite.T(), orderErrorDynamicRedirectUrlsNotAllowed, err.Error())
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	processor.checked.project.Merchant.Banking.Currency = &billing.Currency{
		CodeInt:  980,
		CodeA3:   "UAH",
		Name:     &billing.Name{Ru: "Украинская гривна", En: "Ukrainian Hryvnia"},
		IsActive: true,
	}

	order, err := processor.prepareOrder()
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), order)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrencyRate), err.Error())
}

func (suite *OrderTestSuite) TestOrder_PrepareOrder_Commission_Error() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectFixedAmount.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
		Products:      suite.productIds,
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	err = processor.processProjectOrderId()
	assert.Nil(suite.T(), err)

	err = processor.processLimitAmounts()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
	assert.Nil(suite.T(), err)
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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
	assert.Nil(suite.T(), order.Tax)

	err = processor.processOrderCommissions(order)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), order.PaymentSystemFeeAmount)

	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountPaymentMethodCurrency > 0)
	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountMerchantCurrency > 0)
	assert.True(suite.T(), order.PaymentSystemFeeAmount.AmountPaymentSystemCurrency > 0)
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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

	err = processor.processProducts()
	assert.Nil(suite.T(), err)

	pm, err := suite.service.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)
	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), pm)

	err = processor.processPaymentMethod(pm)
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
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCommission), err.Error())
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
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrencyRate), err.Error())
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

	suite.service.geo = mock.NewGeoIpServiceTestError()

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
			Ip:          req.PayerIp,
			Country:     "RU",
			CountryName: &billing.Name{En: "Russia", Ru: "Россия"},
			City:        &billing.Name{En: "St.Petersburg", Ru: "Санкт-Петербург"},
			State:       "",
			Timezone:    "Europe/Moscow",
		},
		Status:        constant.OrderStatusNew,
		CreatedAt:     ptypes.TimestampNow(),
		IsJsonRequest: false,

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

	err := suite.service.db.Collection(pkg.CollectionOrder).Insert(order)
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
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, pkg.CollectionCurrencyRate), err.Error())

	assert.Len(suite.T(), rsp.Id, 0)
	assert.Nil(suite.T(), rsp.Project)
	assert.Nil(suite.T(), rsp.PaymentMethod)
	assert.Nil(suite.T(), rsp.PaymentSystemFeeAmount)
}

func (suite *OrderTestSuite) TestOrder_ProcessRenderFormPaymentMethods_DevEnvironment_Ok() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(order.Id) > 0)

	processor := &PaymentFormProcessor{
		service: suite.service,
		order:   order,
		request: &grpc.PaymentFormJsonDataRequest{
			OrderId: order.Id,
			Scheme:  "http",
			Host:    "unit.test",
		},
	}

	pms, err := processor.processRenderFormPaymentMethods()

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(pms) > 0)
}

func (suite *OrderTestSuite) TestOrder_ProcessRenderFormPaymentMethods_Cache_Ok() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(order.Id) > 0)

	processor := &PaymentFormProcessor{
		service: suite.service,
		order:   order,
		request: &grpc.PaymentFormJsonDataRequest{
			OrderId: order.Id,
			Scheme:  "http",
			Host:    "unit.test",
		},
	}

	_, ok := suite.service.projectPaymentMethodCache[order.Project.Id]
	assert.False(suite.T(), ok)

	pms, err := processor.processRenderFormPaymentMethods()

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(pms) > 0)

	cachePms, ok := suite.service.projectPaymentMethodCache[order.Project.Id]
	assert.True(suite.T(), ok)
	assert.True(suite.T(), len(cachePms) > 0)

	pms1, err := processor.processRenderFormPaymentMethods()

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(pms1) > 0)
	assert.Equal(suite.T(), pms, pms1)
}

func (suite *OrderTestSuite) TestOrder_ProcessRenderFormPaymentMethods_ProdEnvironment_Ok() {
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

	suite.service.cfg.Environment = "prod"

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(order.Id) > 0)

	processor := &PaymentFormProcessor{
		service: suite.service,
		order:   order,
		request: &grpc.PaymentFormJsonDataRequest{
			OrderId: order.Id,
			Scheme:  "http",
			Host:    "unit.test",
		},
	}
	pms, err := processor.processRenderFormPaymentMethods()

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(pms) > 0)

	suite.service.cfg.Environment = "dev"
}

func (suite *OrderTestSuite) TestOrder_ProcessRenderFormPaymentMethods_ProjectNotFound_Error() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(order.Id) > 0)

	order.Project.Id = bson.NewObjectId().Hex()

	processor := &PaymentFormProcessor{service: suite.service, order: order}
	pms, err := processor.processRenderFormPaymentMethods()

	assert.Error(suite.T(), err)
	assert.Len(suite.T(), pms, 0)
	assert.Equal(suite.T(), orderErrorProjectNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessRenderFormPaymentMethods_ProjectNotHavePaymentMethods_Error() {
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

	suite.service.cfg.Environment = environmentProd

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(order.Id) > 0)

	order.Project.Id = suite.projectWithoutPaymentMethods.Id

	processor := &PaymentFormProcessor{service: suite.service, order: order}
	pms, err := processor.processRenderFormPaymentMethods()

	assert.Error(suite.T(), err)
	assert.Len(suite.T(), pms, 0)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotAllowed, err.Error())

	suite.service.cfg.Environment = "dev"
}

func (suite *OrderTestSuite) TestOrder_ProcessRenderFormPaymentMethods_EmptyPaymentMethods_Error() {
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

	suite.service.cfg.Environment = environmentProd

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(order.Id) > 0)

	order.Project.Id = suite.projectEmptyPaymentMethodTerminal.Id

	processor := &PaymentFormProcessor{service: suite.service, order: order}
	pms, err := processor.processRenderFormPaymentMethods()

	assert.Error(suite.T(), err)
	assert.Len(suite.T(), pms, 0)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotAllowed, err.Error())

	suite.service.cfg.Environment = "dev"
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethodsData_SavedCards_Ok() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	processor := &PaymentFormProcessor{service: suite.service, order: order}

	pm := &billing.PaymentFormPaymentMethod{
		Id:            suite.paymentMethod.Id,
		Name:          suite.paymentMethod.Id,
		Icon:          suite.paymentMethod.Name,
		Type:          suite.paymentMethod.Type,
		Group:         suite.paymentMethod.Group,
		AccountRegexp: suite.paymentMethod.AccountRegexp,
	}

	assert.True(suite.T(), len(pm.SavedCards) <= 0)

	err = processor.processPaymentMethodsData(pm)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), pm.HasSavedCards)
	assert.True(suite.T(), len(pm.SavedCards) > 0)
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethodsData_EmptySavedCards_Ok() {
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

	suite.service.rep = mock.NewRepositoryServiceEmpty()

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	processor := &PaymentFormProcessor{service: suite.service, order: order}

	pm := &billing.PaymentFormPaymentMethod{
		Id:            suite.paymentMethod.Id,
		Name:          suite.paymentMethod.Id,
		Icon:          suite.paymentMethod.Name,
		Type:          suite.paymentMethod.Type,
		Group:         suite.paymentMethod.Group,
		AccountRegexp: suite.paymentMethod.AccountRegexp,
	}

	assert.True(suite.T(), len(pm.SavedCards) <= 0)

	err = processor.processPaymentMethodsData(pm)
	assert.Nil(suite.T(), err)
	assert.False(suite.T(), pm.HasSavedCards)
	assert.Len(suite.T(), pm.SavedCards, 0)
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethodsData_NotBankCard_Ok() {
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

	suite.service.rep = mock.NewRepositoryServiceEmpty()

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	processor := &PaymentFormProcessor{service: suite.service, order: order}

	pm := &billing.PaymentFormPaymentMethod{
		Id:            suite.paymentMethod.Id,
		Name:          suite.paymentMethodWithInactivePaymentSystem.Id,
		Icon:          suite.paymentMethodWithInactivePaymentSystem.Name,
		Type:          suite.paymentMethodWithInactivePaymentSystem.Type,
		Group:         suite.paymentMethodWithInactivePaymentSystem.Group,
		AccountRegexp: suite.paymentMethodWithInactivePaymentSystem.AccountRegexp,
	}

	assert.True(suite.T(), len(pm.SavedCards) <= 0)

	err = processor.processPaymentMethodsData(pm)
	assert.Nil(suite.T(), err)
	assert.False(suite.T(), pm.HasSavedCards)
	assert.Len(suite.T(), pm.SavedCards, 0)
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentMethodsData_GetSavedCards_Error() {
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

	suite.service.rep = mock.NewRepositoryServiceError()

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	processor := &PaymentFormProcessor{service: suite.service, order: order}

	pm := &billing.PaymentFormPaymentMethod{
		Id:            suite.paymentMethod.Id,
		Name:          suite.paymentMethod.Id,
		Icon:          suite.paymentMethod.Name,
		Type:          suite.paymentMethod.Type,
		Group:         suite.paymentMethod.Group,
		AccountRegexp: suite.paymentMethod.AccountRegexp,
	}

	err = processor.processPaymentMethodsData(pm)
	assert.Nil(suite.T(), err)
	assert.False(suite.T(), pm.HasSavedCards)
	assert.Len(suite.T(), pm.SavedCards, 0)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormJsonDataProcess_Ok() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	req1 := &grpc.PaymentFormJsonDataRequest{
		OrderId: order.Uuid,
		Scheme:  "https",
		Host:    "unit.test",
	}
	rsp := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req1, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.PaymentMethods) > 0)
	assert.True(suite.T(), len(rsp.PaymentMethods[0].Id) > 0)
	assert.Equal(suite.T(), len(rsp.Items), 0)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormJsonDataProcessWithProducts_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:     suite.projectFixedAmount.Id,
		PaymentMethod: suite.paymentMethod.Group,
		Currency:      "RUB",
		Amount:        100,
		Account:       "unit test",
		Description:   "unit test",
		OrderId:       bson.NewObjectId().Hex(),
		PayerEmail:    "test@unit.unit",
		PayerIp:       "127.0.0.1",
		Products:      suite.productIds,
	}

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	req1 := &grpc.PaymentFormJsonDataRequest{
		OrderId: order.Uuid,
		Scheme:  "https",
		Host:    "unit.test",
	}
	rsp := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req1, rsp)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.PaymentMethods) > 0)
	assert.True(suite.T(), len(rsp.PaymentMethods[0].Id) > 0)
	assert.Equal(suite.T(), len(rsp.Items), 2)
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_BankCard_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldPan:             "4000000000000002",
		pkg.PaymentCreateFieldCvv:             "123",
		pkg.PaymentCreateFieldMonth:           "02",
		pkg.PaymentCreateFieldYear:            "2100",
		pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.order)
	assert.NotNil(suite.T(), processor.checked.project)
	assert.NotNil(suite.T(), processor.checked.paymentMethod)

	bankBrand, ok := processor.checked.order.PaymentRequisites[paymentCreateBankCardFieldBrand]

	assert.True(suite.T(), ok)
	assert.True(suite.T(), len(bankBrand) > 0)
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_Ewallet_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.pmWebMoney.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldEWallet:         "ewallet_account",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.order)
	assert.NotNil(suite.T(), processor.checked.project)
	assert.NotNil(suite.T(), processor.checked.paymentMethod)
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_Bitcoin_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.pmBitcoin1.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.order)
	assert.NotNil(suite.T(), processor.checked.project)
	assert.NotNil(suite.T(), processor.checked.paymentMethod)
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_OrderIdEmpty_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldPaymentMethodId: suite.pmBitcoin1.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorCreatePaymentRequiredFieldIdNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_PaymentMethodEmpty_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId: rsp.Id,
		pkg.PaymentCreateFieldEmail:   "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:  "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorCreatePaymentRequiredFieldPaymentMethodNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_EmailEmpty_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.pmBitcoin1.Id,
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorCreatePaymentRequiredFieldEmailNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_OrderNotFound_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         bson.NewObjectId().Hex(),
		pkg.PaymentCreateFieldPaymentMethodId: suite.pmBitcoin1.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_OrderHasEndedStatus_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	rsp.Status = constant.OrderStatusProjectComplete
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.pmBitcoin1.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorOrderAlreadyComplete, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_ProjectProcess_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	rsp.Project.Id = suite.inactiveProject.Id
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.pmBitcoin1.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorProjectInactive, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_PaymentMethodNotFound_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: bson.NewObjectId().Hex(),
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotFound, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_PaymentMethodProcess_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.inactivePaymentMethod.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorPaymentMethodInactive, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_AmountLimitProcess_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	rsp.ProjectIncomeAmount = 10
	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(rsp.Id), rsp)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), orderErrorAmountLowerThanMinAllowed, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_BankCardNumberInvalid_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldPan:             "fake_bank_card_number",
		pkg.PaymentCreateFieldCvv:             "123",
		pkg.PaymentCreateFieldMonth:           "02",
		pkg.PaymentCreateFieldYear:            "2100",
		pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), bankCardPanIsInvalid, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_GetBinData_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldPan:             "5555555555554444",
		pkg.PaymentCreateFieldCvv:             "123",
		pkg.PaymentCreateFieldMonth:           "02",
		pkg.PaymentCreateFieldYear:            "2100",
		pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
	}

	suite.service.rep = mock.NewRepositoryServiceError()

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.order)
	assert.NotNil(suite.T(), processor.checked.project)
	assert.NotNil(suite.T(), processor.checked.paymentMethod)

	bankBrand, ok := processor.checked.order.PaymentRequisites[paymentCreateBankCardFieldBrand]

	assert.False(suite.T(), ok)
	assert.Len(suite.T(), bankBrand, 0)

	suite.service.rep = mock.NewRepositoryServiceOk()
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_AccountEmpty_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.pmBitcoin1.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldCrypto:          "",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), processor.checked.order)
	assert.Nil(suite.T(), processor.checked.project)
	assert.Nil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), paymentSystemErrorEWalletIdentifierIsInvalid, err.Error())
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_ChangePaymentSystemTerminal_Ok() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: order.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "ru-RU",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	suite.service.cfg.Environment = environmentProd
	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         order.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusOK, rsp.Status)
	assert.Len(suite.T(), rsp.Error, 0)
	assert.True(suite.T(), len(rsp.RedirectUrl) > 0)

	var check *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(order.Id)).One(&check)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), check)
	assert.Equal(
		suite.T(),
		suite.project.PaymentMethods[constant.PaymentSystemGroupAliasBankCard].Terminal,
		check.PaymentMethod.Params.Terminal,
	)

	suite.service.cfg.Environment = "dev"
}

func (suite *OrderTestSuite) TestOrder_ProcessPaymentFormData_ChangeProjectAccount_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
	}

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), "", rsp.ProjectAccount)

	data := map[string]string{
		pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
		pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
		pkg.PaymentCreateFieldEmail:           "test@unit.unit",
		pkg.PaymentCreateFieldPan:             "4000000000000002",
		pkg.PaymentCreateFieldCvv:             "123",
		pkg.PaymentCreateFieldMonth:           "02",
		pkg.PaymentCreateFieldYear:            "2100",
		pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
	}

	processor := &PaymentCreateProcessor{service: suite.service, data: data}
	err = processor.processPaymentFormData()

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), processor.checked.order)
	assert.NotNil(suite.T(), processor.checked.project)
	assert.NotNil(suite.T(), processor.checked.paymentMethod)
	assert.Equal(suite.T(), "test@unit.unit", processor.checked.order.ProjectAccount)
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_Ok() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: order.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "ru-RU",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         order.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusOK, rsp.Status)
	assert.True(suite.T(), len(rsp.RedirectUrl) > 0)
	assert.Len(suite.T(), rsp.Error, 0)

	var order1 *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(order.Id)).One(&order1)
	assert.NotNil(suite.T(), order1)

	commission, ok := suite.service.commissionCache[order1.Project.Id][order1.PaymentMethod.Id]
	assert.True(suite.T(), ok)
	assert.NotNil(suite.T(), commission)

	rate, ok := suite.service.currencyRateCache[order1.PaymentMethodOutcomeCurrency.CodeInt][order1.Project.Merchant.GetPayoutCurrency().CodeInt]
	assert.True(suite.T(), ok)
	assert.NotNil(suite.T(), rate)

	pmCommission := tools.FormatAmount(order1.ProjectIncomeAmount * (commission.Fee / 100))

	assert.Equal(suite.T(), pmCommission, order1.PaymentSystemFeeAmount.AmountPaymentMethodCurrency)
	assert.Equal(suite.T(), pmCommission, order1.PaymentSystemFeeAmount.AmountPaymentSystemCurrency)

	pmCommission1 := tools.FormatAmount(pmCommission / rate.Rate)
	assert.Equal(suite.T(), pmCommission1, order1.PaymentSystemFeeAmount.AmountMerchantCurrency)
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_ProcessValidation_Error() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         order.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusErrorValidation, rsp.Status)
	assert.Len(suite.T(), rsp.RedirectUrl, 0)
	assert.True(suite.T(), len(rsp.Error) > 0)
	assert.Equal(suite.T(), bankCardExpireYearIsRequired, rsp.Error)
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_ChangeTerminalData_Ok() {
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

	suite.service.cfg.Environment = environmentProd

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: order.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "ru-RU",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         order.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusOK, rsp.Status)
	assert.True(suite.T(), len(rsp.RedirectUrl) > 0)
	assert.Len(suite.T(), rsp.Error, 0)

	suite.service.cfg.Environment = "dev"
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_CreatePaymentSystemHandler_Error() {
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

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         order.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.pmBitcoin1.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldCrypto:          "bitcoin_address",
		},
	}

	rsp := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusErrorSystem, rsp.Status)
	assert.Len(suite.T(), rsp.RedirectUrl, 0)
	assert.True(suite.T(), len(rsp.Error) > 0)
	assert.Equal(suite.T(), paymentSystemErrorHandlerNotFound, rsp.Error)
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_FormInputTimeExpired_Error() {
	req1 := &billing.OrderCreateRequest{
		ProjectId:   suite.project.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
	}

	rsp1 := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)

	var order *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(rsp1.Id)).One(&order)
	assert.NotNil(suite.T(), order)

	order.ExpireDateToFormInput, err = ptypes.TimestampProto(time.Now().Add(time.Minute * -40))
	assert.NoError(suite.T(), err)

	err = suite.service.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	expireYear := time.Now().AddDate(1, 0, 0)

	req2 := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp1.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp2 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusErrorValidation, rsp2.Status)
	assert.Equal(suite.T(), orderErrorFormInputTimeExpired, rsp2.Error)
}

func (suite *OrderTestSuite) TestOrder_PaymentCallbackProcess_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.projectFixedAmount.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
		Products:    suite.productIds,
	}

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: order.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "ru-RU",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         order.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusOK, rsp.Status)

	var order1 *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(order.Id)).One(&order1)
	suite.NotNil(suite.T(), order1)

	callbackRequest := &billing.CardPayPaymentCallback{
		PaymentMethod: suite.paymentMethod.Params.ExternalId,
		CallbackTime:  time.Now().Format("2006-01-02T15:04:05Z"),
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id:          order.Id,
			Description: order.Description,
			Items: []*billing.CardPayItem{
				{
					Name:        order.Items[0].Name,
					Description: order.Items[0].Name,
					Count:       1,
					Price:       order.Items[0].Amount,
				},
			},
		},
		CardAccount: &billing.CallbackCardPayBankCardAccount{
			Holder:             order.PaymentRequisites[pkg.PaymentCreateFieldHolder],
			IssuingCountryCode: "RU",
			MaskedPan:          order.PaymentRequisites[pkg.PaymentCreateFieldPan],
			Token:              bson.NewObjectId().Hex(),
		},
		Customer: &billing.CardPayCustomer{
			Email:  order.PayerData.Email,
			Ip:     order.PayerData.Ip,
			Id:     order.ProjectAccount,
			Locale: "Europe/Moscow",
		},
		PaymentData: &billing.CallbackCardPayPaymentData{
			Id:          bson.NewObjectId().Hex(),
			Amount:      order1.TotalPaymentAmount,
			Currency:    order1.PaymentMethodOutcomeCurrency.CodeA3,
			Description: order.Description,
			Is_3D:       true,
			Rrn:         bson.NewObjectId().Hex(),
			Status:      pkg.CardPayPaymentResponseStatusCompleted,
		},
	}

	buf, err := json.Marshal(callbackRequest)
	assert.Nil(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(buf) + order1.PaymentMethod.Params.CallbackPassword))

	callbackData := &grpc.PaymentNotifyRequest{
		OrderId:   order.Id,
		Request:   buf,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}

	callbackResponse := &grpc.PaymentNotifyResponse{}
	err = suite.service.PaymentCallbackProcess(context.TODO(), callbackData, callbackResponse)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusOK, callbackResponse.Status)

	var order2 *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(order.Id)).One(&order2)
	suite.NotNil(suite.T(), order2)

	assert.Equal(suite.T(), int32(constant.OrderStatusPaymentSystemComplete), order2.Status)
	assert.Equal(suite.T(), callbackRequest.GetId(), order2.PaymentMethodOrderId)
	assert.Equal(suite.T(), callbackRequest.GetAmount(), order2.PaymentMethodIncomeAmount)
	assert.Equal(suite.T(), callbackRequest.GetCurrency(), order2.PaymentMethodIncomeCurrency.CodeA3)
}

func (suite *OrderTestSuite) TestOrder_PaymentCallbackProcess_Recurring_Ok() {
	req := &billing.OrderCreateRequest{
		ProjectId:   suite.projectFixedAmount.Id,
		Currency:    "RUB",
		Amount:      100,
		Account:     "unit test",
		Description: "unit test",
		OrderId:     bson.NewObjectId().Hex(),
		PayerEmail:  "test@unit.unit",
		PayerIp:     "127.0.0.1",
		Products:    suite.productIds,
	}

	order := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, order)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: order.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "ru-RU",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	createPaymentRequest := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         order.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
			pkg.PaymentCreateFieldStoreData:       "1",
		},
	}

	rsp := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), createPaymentRequest, rsp)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusOK, rsp.Status)

	var order1 *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(order.Id)).One(&order1)
	suite.NotNil(suite.T(), order1)

	callbackRequest := &billing.CardPayPaymentCallback{
		PaymentMethod: suite.paymentMethod.Params.ExternalId,
		CallbackTime:  time.Now().Format("2006-01-02T15:04:05Z"),
		MerchantOrder: &billing.CardPayMerchantOrder{
			Id:          order.Id,
			Description: order.Description,
			Items: []*billing.CardPayItem{
				{
					Name:        order.Items[0].Name,
					Description: order.Items[0].Name,
					Count:       1,
					Price:       order.Items[0].Amount,
				},
			},
		},
		CardAccount: &billing.CallbackCardPayBankCardAccount{
			Holder:             order.PaymentRequisites[pkg.PaymentCreateFieldHolder],
			IssuingCountryCode: "RU",
			MaskedPan:          order.PaymentRequisites[pkg.PaymentCreateFieldPan],
			Token:              bson.NewObjectId().Hex(),
		},
		Customer: &billing.CardPayCustomer{
			Email:  order.PayerData.Email,
			Ip:     order.PayerData.Ip,
			Id:     order.ProjectAccount,
			Locale: "Europe/Moscow",
		},
		RecurringData: &billing.CardPayCallbackRecurringData{
			Id:          bson.NewObjectId().Hex(),
			Amount:      order1.TotalPaymentAmount,
			Currency:    order1.PaymentMethodOutcomeCurrency.CodeA3,
			Description: order.Description,
			Is_3D:       true,
			Rrn:         bson.NewObjectId().Hex(),
			Status:      pkg.CardPayPaymentResponseStatusCompleted,
			Filing: &billing.CardPayCallbackRecurringDataFilling{
				Id: bson.NewObjectId().Hex(),
			},
		},
	}

	buf, err := json.Marshal(callbackRequest)
	assert.Nil(suite.T(), err)

	hash := sha512.New()
	hash.Write([]byte(string(buf) + order1.PaymentMethod.Params.CallbackPassword))

	callbackData := &grpc.PaymentNotifyRequest{
		OrderId:   order.Id,
		Request:   buf,
		Signature: hex.EncodeToString(hash.Sum(nil)),
	}

	callbackResponse := &grpc.PaymentNotifyResponse{}
	err = suite.service.PaymentCallbackProcess(context.TODO(), callbackData, callbackResponse)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusOK, callbackResponse.Status)

	var order2 *billing.Order
	err = suite.service.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(order.Id)).One(&order2)
	suite.NotNil(suite.T(), order2)

	assert.Equal(suite.T(), int32(constant.OrderStatusPaymentSystemComplete), order2.Status)
	assert.Equal(suite.T(), callbackRequest.GetId(), order2.PaymentMethodOrderId)
	assert.Equal(suite.T(), callbackRequest.GetAmount(), order2.PaymentMethodIncomeAmount)
	assert.Equal(suite.T(), callbackRequest.GetCurrency(), order2.PaymentMethodIncomeCurrency.CodeA3)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormLanguageChanged_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "en-US",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	req1 := &grpc.PaymentFormUserChangeLangRequest{
		OrderId: rsp.Uuid,
		Lang:    "by",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormLanguageChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.True(suite.T(), rsp1.Item.UserAddressDataRequired)
	assert.Equal(suite.T(), rsp.PayerData.Country, rsp1.Item.UserIpData.Country)
	assert.Equal(suite.T(), rsp.PayerData.Zip, rsp1.Item.UserIpData.Zip)
	assert.Equal(suite.T(), rsp.PayerData.City.En, rsp1.Item.UserIpData.City)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormLanguageChanged_OrderNotFound_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangeLangRequest{
		OrderId: uuid.New().String(),
		Lang:    "en",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormLanguageChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), orderErrorNotFound, rsp1.Message)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormLanguageChanged_NoChanges_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "en-US",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	req1 := &grpc.PaymentFormUserChangeLangRequest{
		OrderId: rsp.Uuid,
		Lang:    "en",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormLanguageChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.False(suite.T(), rsp1.Item.UserAddressDataRequired)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_BankCard_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "en-US",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: suite.paymentMethod.Id,
		Account:  "4000000000000002",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.True(suite.T(), rsp1.Item.UserAddressDataRequired)
	assert.Equal(suite.T(), "US", rsp1.Item.UserIpData.Country)
	assert.Equal(suite.T(), rsp.PayerData.Zip, rsp1.Item.UserIpData.Zip)
	assert.Equal(suite.T(), rsp.PayerData.City.En, rsp1.Item.UserIpData.City)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_Qiwi_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "localhost",
		Locale:  "en-US",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: suite.paymentMethodWithInactivePaymentSystem.Id,
		Account:  "375444190039",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.True(suite.T(), rsp1.Item.UserAddressDataRequired)
	assert.Equal(suite.T(), "BY", rsp1.Item.UserIpData.Country)
	assert.Equal(suite.T(), rsp.PayerData.Zip, rsp1.Item.UserIpData.Zip)
	assert.Equal(suite.T(), rsp.PayerData.City.En, rsp1.Item.UserIpData.City)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_OrderNotFound_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  uuid.New().String(),
		MethodId: suite.paymentMethod.Id,
		Account:  "4000000000000002",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), orderErrorNotFound, rsp1.Message)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_PaymentMethodNotFound_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: bson.NewObjectId().Hex(),
		Account:  "4000000000000002",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), orderErrorPaymentMethodNotFound, rsp1.Message)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_AccountIncorrect_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: suite.paymentMethod.Id,
		Account:  "some_account",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), orderErrorPaymentAccountIncorrect, rsp1.Message)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_BinDataNotFound_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: suite.paymentMethod.Id,
		Account:  "5555555555554444",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), orderErrorCountryByPaymentAccountNotFound, rsp1.Message)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_QiwiAccountIncorrect_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: suite.paymentMethodWithInactivePaymentSystem.Id,
		Account:  "some_account",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), orderErrorPaymentAccountIncorrect, rsp1.Message)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_QiwiAccountCountryNotFound_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: suite.paymentMethodWithInactivePaymentSystem.Id,
		Account:  "244636739467",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), orderErrorCountryByPaymentAccountNotFound, rsp1.Message)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_Bitcoin_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: suite.pmBitcoin1.Id,
		Account:  "some_account",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.False(suite.T(), rsp1.Item.UserAddressDataRequired)
}

func (suite *OrderTestSuite) TestOrder_PaymentFormPaymentAccountChanged_NoChanges_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.PaymentFormUserChangePaymentAccountRequest{
		OrderId:  rsp.Uuid,
		MethodId: suite.paymentMethodWithInactivePaymentSystem.Id,
		Account:  "79211234567",
	}
	rsp1 := &grpc.PaymentFormDataChangeResponse{}
	err = suite.service.PaymentFormPaymentAccountChanged(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.False(suite.T(), rsp1.Item.UserAddressDataRequired)
}

func (suite *OrderTestSuite) TestOrder_OrderReCalculateAmounts_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "127.0.0.1",
		Locale:  "ru-Ru",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	order, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), order.BillingAddress)

	req1 := &grpc.ProcessBillingAddressRequest{
		OrderId: rsp.Uuid,
		Country: "US",
		City:    "Washington",
		Zip:     "98001",
	}
	rsp1 := &grpc.ProcessBillingAddressResponse{}
	err = suite.service.ProcessBillingAddress(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.True(suite.T(), rsp1.Item.HasVat)
	assert.True(suite.T(), rsp1.Item.Vat > 0)
	assert.True(suite.T(), rsp1.Item.Amount > 0)
	assert.True(suite.T(), rsp1.Item.TotalAmount > 0)

	assert.NotEqual(suite.T(), order.Tax.Amount, rsp1.Item.Vat)
	assert.NotEqual(suite.T(), float32(order.TotalPaymentAmount), rsp1.Item.TotalAmount)

	order1, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order1.BillingAddress)

	assert.Equal(suite.T(), order1.Tax.Amount, rsp1.Item.Vat)
	assert.Equal(suite.T(), float32(order1.TotalPaymentAmount), rsp1.Item.TotalAmount)
}

func (suite *OrderTestSuite) TestOrder_OrderReCalculateAmounts_OrderNotFound_Error() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(rsp.Id) > 0)

	req1 := &grpc.ProcessBillingAddressRequest{
		OrderId: uuid.New().String(),
		Country: "US",
		City:    "Washington",
		Zip:     "98001",
	}
	rsp1 := &grpc.ProcessBillingAddressResponse{}
	err = suite.service.ProcessBillingAddress(context.TODO(), req1, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp1.Status)
	assert.Equal(suite.T(), orderErrorNotFound, rsp1.Message)
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_UserAddressDataRequired_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "127.0.0.1",
		Locale:  "ru-Ru",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	order, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order)
	assert.Nil(suite.T(), order.BillingAddress)

	order.UserAddressDataRequired = true
	err = suite.service.updateOrder(order)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	req1 := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
			pkg.PaymentCreateFieldUserCountry:     "US",
			pkg.PaymentCreateFieldUserCity:        "Washington",
			pkg.PaymentCreateFieldUserZip:         "98001",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), req1, rsp1)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusOK, rsp.Status)
	assert.True(suite.T(), len(rsp1.RedirectUrl) > 0)
	assert.Len(suite.T(), rsp1.Error, 0)

	order1, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order1)

	assert.True(suite.T(), order.Tax.Amount > order1.Tax.Amount)
	assert.True(suite.T(), order.TotalPaymentAmount > order1.TotalPaymentAmount)
	assert.NotNil(suite.T(), order1.BillingAddress)
	assert.Equal(suite.T(), "US", order1.BillingAddress.Country)
	assert.Equal(suite.T(), "Washington", order1.BillingAddress.City)
	assert.Equal(suite.T(), "98001", order1.BillingAddress.PostalCode)
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_UserAddressDataRequired_CountryFieldNotFound_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "127.0.0.1",
		Locale:  "ru-Ru",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	order, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order)
	assert.Nil(suite.T(), order.BillingAddress)

	order.UserAddressDataRequired = true
	err = suite.service.updateOrder(order)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	req1 := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), req1, rsp1)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusErrorValidation, rsp1.Status)
	assert.Empty(suite.T(), rsp1.RedirectUrl)
	assert.Equal(suite.T(), orderErrorCreatePaymentRequiredFieldUserCountryNotFound, rsp1.Error)

	order1, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order1)

	assert.Equal(suite.T(), order.Tax.Amount, order1.Tax.Amount)
	assert.Equal(suite.T(), order.TotalPaymentAmount, order1.TotalPaymentAmount)
	assert.Nil(suite.T(), order1.BillingAddress)
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_UserAddressDataRequired_CityFieldNotFound_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "127.0.0.1",
		Locale:  "ru-Ru",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	order, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order)
	assert.Nil(suite.T(), order.BillingAddress)

	order.UserAddressDataRequired = true
	err = suite.service.updateOrder(order)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	req1 := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
			pkg.PaymentCreateFieldUserCountry:     "US",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), req1, rsp1)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusErrorValidation, rsp1.Status)
	assert.Empty(suite.T(), rsp1.RedirectUrl)
	assert.Equal(suite.T(), orderErrorCreatePaymentRequiredFieldUserCityNotFound, rsp1.Error)

	order1, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order1)

	assert.Equal(suite.T(), order.Tax.Amount, order1.Tax.Amount)
	assert.Equal(suite.T(), order.TotalPaymentAmount, order1.TotalPaymentAmount)
	assert.Nil(suite.T(), order1.BillingAddress)
}

func (suite *OrderTestSuite) TestOrder_PaymentCreateProcess_UserAddressDataRequired_ZipFieldNotFound_Ok() {
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

	rsp := &billing.Order{}
	err := suite.service.OrderCreateProcess(context.TODO(), req, rsp)
	assert.Nil(suite.T(), err)

	req2 := &grpc.PaymentFormJsonDataRequest{
		OrderId: rsp.Uuid,
		Scheme:  "http",
		Host:    "127.0.0.1",
		Locale:  "ru-Ru",
		Ip:      "127.0.0.1",
	}
	rsp2 := &grpc.PaymentFormJsonDataResponse{}
	err = suite.service.PaymentFormJsonDataProcess(context.TODO(), req2, rsp2)
	assert.NoError(suite.T(), err)

	order, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order)
	assert.Nil(suite.T(), order.BillingAddress)

	order.UserAddressDataRequired = true
	err = suite.service.updateOrder(order)
	assert.NoError(suite.T(), err)

	expireYear := time.Now().AddDate(1, 0, 0)

	req1 := &grpc.PaymentCreateRequest{
		Data: map[string]string{
			pkg.PaymentCreateFieldOrderId:         rsp.Uuid,
			pkg.PaymentCreateFieldPaymentMethodId: suite.paymentMethod.Id,
			pkg.PaymentCreateFieldEmail:           "test@unit.unit",
			pkg.PaymentCreateFieldPan:             "4000000000000002",
			pkg.PaymentCreateFieldCvv:             "123",
			pkg.PaymentCreateFieldMonth:           "02",
			pkg.PaymentCreateFieldYear:            expireYear.Format("2006"),
			pkg.PaymentCreateFieldHolder:          "Mr. Card Holder",
			pkg.PaymentCreateFieldUserCountry:     "US",
			pkg.PaymentCreateFieldUserCity:        "Washington",
		},
	}

	rsp1 := &grpc.PaymentCreateResponse{}
	err = suite.service.PaymentCreateProcess(context.TODO(), req1, rsp1)

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), pkg.StatusErrorValidation, rsp1.Status)
	assert.Empty(suite.T(), rsp1.RedirectUrl)
	assert.Equal(suite.T(), orderErrorCreatePaymentRequiredFieldUserZipNotFound, rsp1.Error)

	order1, err := suite.service.getOrderByUuid(rsp.Uuid)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), order1)

	assert.Equal(suite.T(), order.Tax.Amount, order1.Tax.Amount)
	assert.Equal(suite.T(), order.TotalPaymentAmount, order1.TotalPaymentAmount)
	assert.Nil(suite.T(), order1.BillingAddress)
}
