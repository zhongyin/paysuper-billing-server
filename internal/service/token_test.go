package service

import (
	"context"
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"net"
	"testing"
)

type TokenTestSuite struct {
	suite.Suite
	service *Service

	project             *billing.Project
	projectWithProducts *billing.Project

	product1 *grpc.Product
	product2 *grpc.Product
}

func Test_Token(t *testing.T) {
	suite.Run(t, new(TokenTestSuite))
}

func (suite *TokenTestSuite) SetupTest() {
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

	rub := &billing.Currency{
		CodeInt:  643,
		CodeA3:   "RUB",
		Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
		IsActive: true,
	}

	err = db.Collection(pkg.CollectionCurrency).Insert(rub)
	assert.NoError(suite.T(), err, "Insert currency test data failed")

	project := &billing.Project{
		Id:                       bson.NewObjectId().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         pkg.ProjectCallbackProtocolEmpty,
		LimitsCurrency:           "RUB",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       false,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   pkg.ProjectStatusInProduction,
		MerchantId:               bson.NewObjectId().Hex(),
	}
	projectWithProducts := &billing.Project{
		Id:                       bson.NewObjectId().Hex(),
		CallbackCurrency:         "RUB",
		CallbackProtocol:         pkg.ProjectCallbackProtocolEmpty,
		LimitsCurrency:           "RUB",
		MaxPaymentAmount:         15000,
		MinPaymentAmount:         1,
		Name:                     map[string]string{"en": "test project 1"},
		IsProductsCheckout:       true,
		AllowDynamicRedirectUrls: true,
		SecretKey:                "test project 1 secret key",
		Status:                   pkg.ProjectStatusInProduction,
		MerchantId:               bson.NewObjectId().Hex(),
	}

	err = db.Collection(pkg.CollectionProject).Insert([]interface{}{project, projectWithProducts}...)
	assert.NoError(suite.T(), err, "Insert project test data failed")

	product1 := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_double_yeti",
		Name:            map[string]string{"en": initialName},
		DefaultCurrency: "USD",
		Enabled:         true,
		Description:     map[string]string{"en": "blah-blah-blah"},
		LongDescription: map[string]string{"en": "Super game steam keys"},
		Url:             "http://test.ru/dffdsfsfs",
		Images:          []string{"/home/image.jpg"},
		MerchantId:      projectWithProducts.MerchantId,
		ProjectId:       projectWithProducts.Id,
		Metadata: map[string]string{
			"SomeKey": "SomeValue",
		},
		Prices: []*grpc.ProductPrice{{Currency: "USD", Amount: 1005.00}},
	}
	product2 := &grpc.Product{
		Object:          "product1",
		Type:            "simple_product",
		Sku:             "ru_double_yeti1",
		Name:            map[string]string{"en": initialName},
		DefaultCurrency: "USD",
		Enabled:         true,
		Description:     map[string]string{"en": "blah-blah-blah"},
		LongDescription: map[string]string{"en": "Super game steam keys"},
		Url:             "http://test.ru/dffdsfsfs",
		Images:          []string{"/home/image.jpg"},
		MerchantId:      projectWithProducts.MerchantId,
		ProjectId:       projectWithProducts.Id,
		Metadata: map[string]string{
			"SomeKey": "SomeValue",
		},
		Prices: []*grpc.ProductPrice{{Currency: "USD", Amount: 1005.00}},
	}

	err = db.Collection(pkg.CollectionProduct).Insert([]interface{}{product1, product2}...)
	assert.NoError(suite.T(), err, "Insert product test data failed")

	suite.service = NewBillingService(db, cfg, make(chan bool, 1), nil, nil, nil, nil)
	err = suite.service.Init()
	assert.NoError(suite.T(), err, "Billing service initialization failed")

	suite.project = project
	suite.projectWithProducts = projectWithProducts
	suite.product1 = product1
	suite.product2 = product2
}

func (suite *TokenTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *TokenTestSuite) TestToken_CreateToken_NewCustomer_Ok() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Email: &billing.TokenUserEmailValue{
				Value:    "test@unit.test",
				Verified: true,
			},
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    100,
			Currency:  "RUB",
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.Id)
	assert.NotEmpty(suite.T(), rsp.Item.Token)
	assert.NotEmpty(suite.T(), rsp.Item.CustomerId)
	assert.Equal(suite.T(), req.User.Id, rsp.Item.User.Id)
	assert.Equal(suite.T(), req.User.Locale.Value, rsp.Item.User.Locale.Value)
	assert.Equal(suite.T(), req.Settings.ProjectId, rsp.Item.Settings.ProjectId)
	assert.Equal(suite.T(), req.Settings.Amount, rsp.Item.Settings.Amount)
	assert.Equal(suite.T(), req.Settings.Currency, rsp.Item.Settings.Currency)
	assert.NotNil(suite.T(), rsp.Item.User.Email)
	assert.Nil(suite.T(), rsp.Item.User.Phone)
	assert.Empty(suite.T(), rsp.Item.Settings.ProductsIds)

	var customer *billing.Customer
	err = suite.service.db.Collection(pkg.CollectionCustomer).FindId(bson.ObjectIdHex(rsp.Item.CustomerId)).One(&customer)
	assert.NotNil(suite.T(), customer)

	assert.Equal(suite.T(), req.User.Id, customer.ExternalId)
	assert.Equal(suite.T(), customer.Id+pkg.TechEmailDomain, customer.TechEmail)
	assert.Equal(suite.T(), req.User.Email.Value, customer.Email)
	assert.Equal(suite.T(), req.User.Email.Verified, customer.EmailVerified)
	assert.Empty(suite.T(), customer.Phone)
	assert.False(suite.T(), customer.PhoneVerified)
	assert.Empty(suite.T(), customer.Name)
	assert.Empty(suite.T(), customer.Ip)
	assert.Equal(suite.T(), req.User.Locale.Value, customer.Locale)
	assert.Empty(suite.T(), customer.AcceptLanguage)
	assert.Empty(suite.T(), customer.UserAgent)
	assert.Nil(suite.T(), customer.Address)
	assert.Empty(suite.T(), customer.IpHistory)
	assert.Empty(suite.T(), customer.AddressHistory)
	assert.Empty(suite.T(), customer.AcceptLanguageHistory)
	assert.Empty(suite.T(), customer.Metadata)

	assert.Len(suite.T(), customer.Identity, 2)
	assert.Equal(suite.T(), customer.Identity[0].Value, customer.ExternalId)
	assert.True(suite.T(), customer.Identity[0].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeExternal, customer.Identity[0].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[0].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[0].MerchantId)

	assert.Equal(suite.T(), customer.Identity[1].Value, customer.Email)
	assert.True(suite.T(), customer.Identity[1].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeEmail, customer.Identity[1].Type)
	assert.Equal(suite.T(), suite.project.Id, customer.Identity[1].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customer.Identity[1].MerchantId)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ExistCustomer_Ok() {
	email := "test_exist_customer@unit.test"

	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Email: &billing.TokenUserEmailValue{
				Value:    email,
				Verified: true,
			},
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    100,
			Currency:  "RUB",
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.Id)

	req.User.Phone = &billing.TokenUserPhoneValue{
		Value: "1234567890",
	}
	req.User.Email = &billing.TokenUserEmailValue{
		Value: "test_exist_customer_1@unit.test",
	}
	rsp1 := &grpc.TokenResponse{}
	err = suite.service.CreateToken(context.TODO(), req, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.NotEmpty(suite.T(), rsp1.Item.Id)
	assert.Equal(suite.T(), rsp.Item.CustomerId, rsp1.Item.CustomerId)

	var customers []*billing.Customer
	err = suite.service.db.Collection(pkg.CollectionCustomer).FindId(bson.ObjectIdHex(rsp.Item.CustomerId)).All(&customers)
	assert.Len(suite.T(), customers, 1)

	assert.Len(suite.T(), customers[0].Identity, 4)
	assert.Equal(suite.T(), customers[0].Identity[3].Value, customers[0].Phone)
	assert.False(suite.T(), customers[0].Identity[3].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypePhone, customers[0].Identity[3].Type)
	assert.Equal(suite.T(), suite.project.Id, customers[0].Identity[3].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customers[0].Identity[3].MerchantId)

	assert.Equal(suite.T(), customers[0].Identity[2].Value, customers[0].Email)
	assert.False(suite.T(), customers[0].Identity[2].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeEmail, customers[0].Identity[2].Type)
	assert.Equal(suite.T(), suite.project.Id, customers[0].Identity[2].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customers[0].Identity[2].MerchantId)

	assert.Equal(suite.T(), email, customers[0].Identity[1].Value)
	assert.True(suite.T(), customers[0].Identity[1].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeEmail, customers[0].Identity[1].Type)
	assert.Equal(suite.T(), suite.project.Id, customers[0].Identity[1].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customers[0].Identity[1].MerchantId)

	assert.Equal(suite.T(), customers[0].Identity[0].Value, customers[0].ExternalId)
	assert.True(suite.T(), customers[0].Identity[0].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeExternal, customers[0].Identity[0].Type)
	assert.Equal(suite.T(), suite.project.Id, customers[0].Identity[0].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customers[0].Identity[0].MerchantId)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ExistCustomer_UpdateExistIdentity_Ok() {
	email := "test_exist_customer_update_exist_identity@unit.test"
	address := &billing.OrderBillingAddress{
		Country:    "UA",
		City:       "NewYork",
		PostalCode: "000000",
	}

	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Email: &billing.TokenUserEmailValue{
				Value: email,
			},
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
			Address: address,
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    100,
			Currency:  "RUB",
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.NotEmpty(suite.T(), rsp.Item.Id)

	var customer *billing.Customer
	err = suite.service.db.Collection(pkg.CollectionCustomer).FindId(bson.ObjectIdHex(rsp.Item.CustomerId)).One(&customer)
	assert.NotNil(suite.T(), customer)
	assert.False(suite.T(), customer.Identity[1].Verified)

	req.User.Phone = &billing.TokenUserPhoneValue{
		Value: "1234567890",
	}
	req.User.Email = &billing.TokenUserEmailValue{
		Value:    "test_exist_customer_update_exist_identity@unit.test",
		Verified: true,
	}
	req.User.Name = &billing.TokenUserValue{Value: "Unit test"}
	req.User.Ip = &billing.TokenUserIpValue{Value: "127.0.0.1"}
	req.User.Locale = &billing.TokenUserLocaleValue{Value: "ru"}
	req.User.Address = &billing.OrderBillingAddress{
		Country:    "RU",
		City:       "St.Petersburg",
		PostalCode: "190000",
	}
	rsp1 := &grpc.TokenResponse{}
	err = suite.service.CreateToken(context.TODO(), req, rsp1)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp1.Status)
	assert.Empty(suite.T(), rsp1.Message)
	assert.NotNil(suite.T(), rsp1.Item)
	assert.NotEmpty(suite.T(), rsp1.Item.Id)
	assert.Equal(suite.T(), rsp.Item.CustomerId, rsp1.Item.CustomerId)

	var customers []*billing.Customer
	err = suite.service.db.Collection(pkg.CollectionCustomer).FindId(bson.ObjectIdHex(rsp.Item.CustomerId)).All(&customers)
	assert.Len(suite.T(), customers, 1)

	assert.Len(suite.T(), customers[0].Identity, 3)
	assert.Equal(suite.T(), customers[0].Identity[2].Value, customers[0].Phone)
	assert.False(suite.T(), customers[0].Identity[2].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypePhone, customers[0].Identity[2].Type)
	assert.Equal(suite.T(), suite.project.Id, customers[0].Identity[2].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customers[0].Identity[2].MerchantId)

	assert.Equal(suite.T(), email, customers[0].Identity[1].Value)
	assert.True(suite.T(), customers[0].Identity[1].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeEmail, customers[0].Identity[1].Type)
	assert.Equal(suite.T(), suite.project.Id, customers[0].Identity[1].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customers[0].Identity[1].MerchantId)

	assert.Equal(suite.T(), customers[0].Identity[0].Value, customers[0].ExternalId)
	assert.True(suite.T(), customers[0].Identity[0].Verified)
	assert.Equal(suite.T(), pkg.UserIdentityTypeExternal, customers[0].Identity[0].Type)
	assert.Equal(suite.T(), suite.project.Id, customers[0].Identity[0].ProjectId)
	assert.Equal(suite.T(), suite.project.MerchantId, customers[0].Identity[0].MerchantId)

	assert.Equal(suite.T(), req.User.Name.Value, customers[0].Name)
	assert.Equal(suite.T(), req.User.Ip.Value, net.IP(customers[0].Ip).String())
	assert.Equal(suite.T(), req.User.Locale.Value, customers[0].Locale)
	assert.Equal(suite.T(), req.User.Address, customers[0].Address)

	assert.NotEmpty(suite.T(), customers[0].IpHistory)
	assert.NotEmpty(suite.T(), customers[0].LocaleHistory)
	assert.NotEmpty(suite.T(), customers[0].AddressHistory)

	assert.Equal(suite.T(), address.Country, customers[0].AddressHistory[0].Country)
	assert.Equal(suite.T(), address.City, customers[0].AddressHistory[0].City)
	assert.Equal(suite.T(), address.PostalCode, customers[0].AddressHistory[0].PostalCode)
}

func (suite *TokenTestSuite) TestToken_CreateToken_CustomerIdentityInformationNotFound_Error() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    100,
			Currency:  "RUB",
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorUserIdentityRequired, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectNotFound_Error() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: bson.NewObjectId().Hex(),
			Amount:    100,
			Currency:  "RUB",
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), projectErrorNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *TokenTestSuite) TestToken_CreateToken_AmountIncorrect_Error() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.project.Id,
			Amount:    -100,
			Currency:  "RUB",
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsAmountRequired, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectIsProductCheckout_WithoutItems_Error() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.projectWithProducts.Id,
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), tokenErrorSettingsItemsRequired, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectIsProductCheckout_ProductsNotFound_Error() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.projectWithProducts.Id,
			Items: []*billing.TokenSettingsItem{
				{
					Sku:      bson.NewObjectId().Hex(),
					Amount:   100,
					Currency: "UAH",
				},
				{
					Sku:      bson.NewObjectId().Hex(),
					Amount:   200,
					Currency: "UAH",
				},
				{
					Sku:      bson.NewObjectId().Hex(),
					Amount:   300,
					Currency: "UAH",
				},
			},
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), productErrorNotFound, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectIsProductCheckout_ProductsCountNotMatch_Error() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.projectWithProducts.Id,
			Items: []*billing.TokenSettingsItem{
				{
					Sku:      bson.NewObjectId().Hex(),
					Amount:   100,
					Currency: "UAH",
				},
				{
					Sku:      suite.product1.Sku,
					Amount:   suite.product1.Prices[0].Amount,
					Currency: suite.product1.Prices[0].Currency,
				},
				{
					Sku:      suite.product2.Sku,
					Amount:   suite.product2.Prices[0].Amount,
					Currency: suite.product2.Prices[0].Currency,
				},
			},
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), productErrorCountNotMatch, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectIsProductCheckout_ProductAmountNotMatch_Error() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.projectWithProducts.Id,
			Items: []*billing.TokenSettingsItem{
				{
					Sku:      suite.product1.Sku,
					Amount:   100,
					Currency: suite.product1.Prices[0].Currency,
				},
				{
					Sku:      suite.product2.Sku,
					Amount:   suite.product2.Prices[0].Amount,
					Currency: suite.product2.Prices[0].Currency,
				},
			},
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), productErrorAmountNotMatch, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectIsProductCheckout_ProductCurrencyNotMatch_Error() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.projectWithProducts.Id,
			Items: []*billing.TokenSettingsItem{
				{
					Sku:      suite.product2.Sku,
					Amount:   suite.product2.Prices[0].Amount,
					Currency: "UAH",
				},
			},
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusBadData, rsp.Status)
	assert.Equal(suite.T(), productErrorCurrencyNotMatch, rsp.Message)
	assert.Nil(suite.T(), rsp.Item)
}

func (suite *TokenTestSuite) TestToken_CreateToken_ProjectIsProductCheckout_Ok() {
	req := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Id: bson.NewObjectId().Hex(),
			Locale: &billing.TokenUserLocaleValue{
				Value: "en",
			},
		},
		Settings: &billing.TokenSettings{
			ProjectId: suite.projectWithProducts.Id,
			Items: []*billing.TokenSettingsItem{
				{
					Sku:      suite.product1.Sku,
					Amount:   suite.product1.Prices[0].Amount,
					Currency: suite.product1.Prices[0].Currency,
				},
				{
					Sku:      suite.product2.Sku,
					Amount:   suite.product2.Prices[0].Amount,
					Currency: suite.product2.Prices[0].Currency,
				},
			},
		},
	}
	rsp := &grpc.TokenResponse{}
	err := suite.service.CreateToken(context.TODO(), req, rsp)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), pkg.ResponseStatusOk, rsp.Status)
	assert.Empty(suite.T(), rsp.Message)
	assert.NotNil(suite.T(), rsp.Item)
	assert.Len(suite.T(), rsp.Item.Settings.ProductsIds, 2)
}
