package service

import (
	"fmt"
	"github.com/ProtocolONE/payone-billing-service/internal/config"
	"github.com/ProtocolONE/payone-billing-service/internal/database"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"testing"
)

type ProjectTestSuite struct {
	suite.Suite
	service *Service

	projectId string
}

func Test_Project(t *testing.T) {
	suite.Run(t, new(ProjectTestSuite))
}

func (suite *ProjectTestSuite) SetupTest() {
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

	logger, err := zap.NewProduction()

	if err != nil {
		suite.FailNow("Logger initialization failed", "%v", err)
	}

	vat := &billing.Vat{Country: "US", Subdivision: "CA", Vat: 10.25, IsActive: true}

	err = db.Collection(collectionVat).Insert(vat)

	if err != nil {
		suite.FailNow("Insert VAT test data failed", "%v", err)
	}

	rub := &billing.Currency{
		CodeInt:  643,
		CodeA3:   "RUB",
		Name:     &billing.Name{Ru: "Российский рубль", En: "Russian ruble"},
		IsActive: true,
	}

	err = db.Collection(collectionCurrency).Insert(rub)

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

	err = db.Collection(collectionCurrencyRate).Insert(rate)

	if err != nil {
		suite.FailNow("Insert rates test data failed", "%v", err)
	}

	project := &billing.Project{
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

	err = db.Collection(collectionProject).Insert(project)

	if err != nil {
		suite.FailNow("Insert project test data failed", "%v", err)
	}

	suite.projectId = project.Id

	suite.service = NewBillingService(db, logger.Sugar(), cfg.CacheConfig, make(chan bool, 1))

	err = suite.service.Init()

	if err != nil {
		suite.FailNow("Billing service initialization failed", "%v", err)
	}
}

func (suite *ProjectTestSuite) TearDownTest() {
	if err := suite.service.db.Drop(); err != nil {
		suite.FailNow("Database deletion failed", "%v", err)
	}

	suite.service.db.Close()

	if err := suite.service.log.Sync(); err != nil {
		suite.FailNow("Logger sync failed", "%v", err)
	}
}

func (suite *ProjectTestSuite) TestProject_GetProjectByIdOk() {
	project, err := suite.service.GetProjectById(suite.projectId)

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), project)
	assert.Equal(suite.T(), suite.projectId, project.Id)
}

func (suite *ProjectTestSuite) TestProject_GetProjectByIdError() {
	project, err := suite.service.GetProjectById(bson.NewObjectId().Hex())

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), project)
	assert.Equal(suite.T(), fmt.Sprintf(errorNotFound, collectionProject), err.Error())
}
