package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/ProtocolONE/geoip-service/pkg/proto"
	"github.com/ProtocolONE/payone-billing-service/internal/config"
	"github.com/ProtocolONE/payone-billing-service/internal/database"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/grpc"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	collectionCurrency      = "currency"
	collectionProject       = "project"
	collectionCurrencyRate  = "currency_rate"
	collectionVat           = "vat"
	collectionOrder         = "order"
	collectionPaymentMethod = "payment_method"
	collectionCommission    = "commission"

	errorNotFound                   = "[PAYONE_BILLING] %s not found"
	initCacheErrorNotFound          = "[PAYONE_BILLING] %s query result is empty"
	errorQueryMask                  = "[PAYONE_BILLING] Query from collection \"%s\" failed"
	errorAccountingCurrencyNotFound = "[PAYONE_BILLING] Accounting currency not found"

	environmentProd = "prod"
)

var (
	handlers = map[string]func(*Service) Cacher{
		collectionCurrency:      newCurrencyHandler,
		collectionProject:       newProjectHandler,
		collectionCurrencyRate:  newCurrencyRateHandler,
		collectionVat:           newVatHandler,
		collectionPaymentMethod: newPaymentMethodHandler,
		collectionCommission:    newCommissionHandler,
	}

	vatBySubdivisionCountries = map[string]bool{"US": true, "CA": true}
)

type Service struct {
	db     *database.Source
	log    *zap.SugaredLogger
	mx     sync.Mutex
	cCfg   *config.CacheConfig
	exitCh chan bool
	ctx    context.Context
	geo    proto.GeoIpService
	env    string

	accountingCurrencyA3 string
	accountingCurrency   *billing.Currency

	currencyCache      map[string]*billing.Currency
	projectCache       map[string]*billing.Project
	currencyRateCache  map[int32]map[int32]*billing.CurrencyRate
	vatCache           map[string]map[string]*billing.Vat
	paymentMethodCache map[string]map[int32]*billing.PaymentMethod
	commissionCache    map[string]map[string]*billing.MgoCommission

	rebuild      bool
	rebuildError error
}

type Cacher interface {
	getAll() ([]interface{}, error)
	setCache([]interface{})
}

func NewBillingService(
	db *database.Source,
	log *zap.SugaredLogger,
	cCfg *config.CacheConfig,
	exitCh chan bool,
	geo proto.GeoIpService,
	env string,
	accountingCurrency string,
) *Service {
	return &Service{
		db:                   db,
		log:                  log,
		cCfg:                 cCfg,
		exitCh:               exitCh,
		geo:                  geo,
		env:                  env,
		accountingCurrencyA3: accountingCurrency,
	}
}

func (s *Service) Init() (err error) {
	s.accountingCurrency, err = s.GetCurrencyByCodeA3(s.accountingCurrencyA3)

	if err != nil {
		return errors.New(errorAccountingCurrencyNotFound)
	}

	err = s.initCache()

	if err == nil {
		go s.reBuildCache()
	}

	return
}

func (s *Service) reBuildCache() {
	var err error
	var key string

	curTicker := time.NewTicker(time.Second * time.Duration(s.cCfg.CurrencyTimeout))
	s.rebuild = true

	for {
		select {
		case <-curTicker.C:
			err = s.cache(collectionCurrency, handlers[collectionCurrency](s))
			key = collectionCurrency
		case <-s.exitCh:
			s.rebuild = false
			return
		}

		if err != nil {
			s.rebuild = false
			s.rebuildError = err

			s.log.Errorw("Rebuild cache failed", "error", err, "cached_collection", key)
		}
	}
}

func (s *Service) cache(key string, handler Cacher) error {
	rec, err := handler.getAll()

	if err != nil {
		return err
	}

	if rec == nil || len(rec) <= 0 {
		return fmt.Errorf(initCacheErrorNotFound, key)
	}

	handler.setCache(rec)

	return nil
}

func (s *Service) initCache() error {
	for k, handler := range handlers {
		err := s.cache(k, handler(s))

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) isProductionEnvironment() bool {
	return s.env == environmentProd
}

func (s *Service) RebuildCache(ctx context.Context, req *grpc.EmptyRequest, res *grpc.EmptyResponse) error {
	return nil
}
