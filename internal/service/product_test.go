package service

import (
	"context"
	"github.com/ProtocolONE/rabbitmq/pkg"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/internal/mock"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/stoewer/go-strcase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"strconv"
	"testing"
)

var (
	createdProductId string
	initialName      string = "Double Yeti"
	newName          string = "Double Yeti Reload"
)

type ProductTestSuite struct {
	suite.Suite
	service *Service
	log     *zap.Logger

	project    *billing.Project
	pmBankCard *billing.PaymentMethod
}

func Test_Product(t *testing.T) {
	suite.Run(t, new(ProductTestSuite))
}

func (suite *ProductTestSuite) SetupTest() {
	cfg, err := config.NewConfig()
	cfg.AccountingCurrency = "RUB"

	assert.NoError(suite.T(), err, "Config load failed")

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

	if err != nil {
		suite.FailNow("Insert currency test data failed", "%v", err)
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
	)
	err = suite.service.Init()
	assert.NoError(suite.T(), err, "Billing service initialization failed")
}

func (suite *ProductTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()
}

func (suite *ProductTestSuite) TestProduct_CRUDProduct_Ok() {

	// Create product OK

	req := &grpc.Product{
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_double_yeti",
		Name:            initialName,
		DefaultCurrency: "USD",
		Enabled:         true,
		Description:     "blah-blah-blah",
		LongDescription: "Super game steam keys",
		Url:             "http://test.ru/dffdsfsfs",
		Images:          []string{"/home/image.jpg"},
		Metadata: map[string]string{
			"SomeKey": "SomeValue",
		},
	}

	req.Prices = append(req.Prices, &grpc.ProductPrice{
		Currency: "USD",
		Amount:   1005.00,
	})

	res := grpc.Product{}

	err := suite.service.CreateOrUpdateProduct(context.TODO(), req, &res)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Name, initialName)
	assert.Equal(suite.T(), len(res.Prices), 1)

	createdProductId = res.Id

	// Get product OK

	req2 := &grpc.RequestProductById{
		Id: createdProductId,
	}

	res2 := grpc.Product{}

	err = suite.service.GetProduct(context.TODO(), req2, &res2)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res2.Name, initialName)
	assert.Equal(suite.T(), len(res2.Prices), 1)

	// Update product OK

	req3 := &grpc.Product{
		Id:              createdProductId,
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_double_yeti_rel",
		Name:            newName,
		DefaultCurrency: "USD",
		Enabled:         true,
		Description:     "Yet another cool game",
		LongDescription: "Super game steam keys",
		Url:             "http://mygame.ru/duoble_yeti",
		Images:          []string{"/home/image.jpg"},
		Metadata: map[string]string{
			"SomeKey": "SomeValue",
		},
	}

	req3.Prices = append(req3.Prices, &grpc.ProductPrice{
		Currency: "USD",
		Amount:   1010.23,
	})
	req3.Prices = append(req3.Prices, &grpc.ProductPrice{
		Currency: "RUB",
		Amount:   65010.23,
	})

	res3 := grpc.Product{}

	err = suite.service.CreateOrUpdateProduct(context.TODO(), req3, &res3)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res3.Name, newName)
	assert.Equal(suite.T(), len(res3.Prices), 2)

	// Delete product Ok

	req4 := &grpc.RequestProductById{
		Id: createdProductId,
	}

	err = suite.service.DeleteProduct(context.TODO(), req4, &grpc.EmptyResponse{})

	assert.NoError(suite.T(), err)

	// Product not found after deletion

	req5 := &grpc.RequestProductById{
		Id: createdProductId,
	}
	err = suite.service.GetProduct(context.TODO(), req5, &grpc.Product{})

	assert.Error(suite.T(), err)

	// Product cant be updated after deletion

	req6 := &grpc.Product{
		Id:              createdProductId,
		Object:          "product",
		Type:            "simple_product",
		Sku:             "ru_double_yeti_rel",
		Name:            newName,
		DefaultCurrency: "USD",
		Enabled:         true,
		Description:     "Yet another cool game",
		LongDescription: "Super game steam keys",
		Url:             "http://mygame.ru/duoble_yeti",
		Images:          []string{"/home/image.jpg"},
		Metadata: map[string]string{
			"SomeKey": "SomeValue",
		},
	}

	req6.Prices = append(req6.Prices, &grpc.ProductPrice{
		Currency: "USD",
		Amount:   1010.23,
	})
	req6.Prices = append(req6.Prices, &grpc.ProductPrice{
		Currency: "RUB",
		Amount:   65010.23,
	})

	err = suite.service.CreateOrUpdateProduct(context.TODO(), req6, &grpc.Product{})

	assert.Error(suite.T(), err)
}

func (suite *ProductTestSuite) TestProduct_ListProduct_Ok() {

	names := []string{"Madalin Stunt Cars M2", "Plants vs Zombies", "Bubble Hunter", "Deer Hunter",
		"Madalin Cars Multiplayer", "Scary Maze"}

	for i, n := range names {
		req := &grpc.Product{
			Object:          "product",
			Type:            "simple_product",
			Sku:             "ru_" + strconv.Itoa(i) + "_" + strcase.SnakeCase(n),
			Name:            n,
			DefaultCurrency: "USD",
			Enabled:         true,
			Description:     n + " description",
		}

		req.Prices = append(req.Prices, &grpc.ProductPrice{
			Currency: "USD",
			Amount:   123.00 * float32(i+1),
		})

		assert.NoError(suite.T(), suite.service.CreateOrUpdateProduct(context.TODO(), req, &grpc.Product{}))
	}

	// get all (first 2 will be shown and total number will be Limit)

	res := grpc.ListProductsResponse{}

	err := suite.service.ListProducts(context.TODO(), &grpc.ListProductsRequest{
		Limit: 2,
	}, &res)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Total, int32(6))
	assert.Equal(suite.T(), res.Limit, int32(2))
	assert.Equal(suite.T(), len(res.Products), 2)

	// get all with offset

	res2 := grpc.ListProductsResponse{}

	err = suite.service.ListProducts(context.TODO(), &grpc.ListProductsRequest{
		Limit:  2,
		Offset: 1,
	}, &res2)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res.Products[1], res2.Products[0])

	// search by part of name

	res3 := grpc.ListProductsResponse{}

	err = suite.service.ListProducts(context.TODO(), &grpc.ListProductsRequest{
		Limit: 2,
		Name:  "cAr",
	}, &res3)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res3.Total, int32(3))

	// search by name with space

	res4 := grpc.ListProductsResponse{}

	err = suite.service.ListProducts(context.TODO(), &grpc.ListProductsRequest{
		Limit: 2,
		Name:  "Cars M",
	}, &res4)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res4.Total, int32(2))

	// search by sku

	res5 := grpc.ListProductsResponse{}

	err = suite.service.ListProducts(context.TODO(), &grpc.ListProductsRequest{
		Limit: 2,
		Sku:   "_cars_",
	}, &res5)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res5.Total, int32(2))

	// search both by name and sku

	res6 := grpc.ListProductsResponse{}

	err = suite.service.ListProducts(context.TODO(), &grpc.ListProductsRequest{
		Limit: 2,
		Name:  "cAr",
		Sku:   "ru_0_",
	}, &res6)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), res6.Total, int32(1))
}
