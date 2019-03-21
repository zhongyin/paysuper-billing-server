package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/ProtocolONE/geoip-service/pkg/proto"
	"github.com/ProtocolONE/rabbitmq/pkg"
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/internal/database"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/paysuper/paysuper-recurring-repository/pkg/proto/repository"
	"github.com/paysuper/paysuper-tax-service/proto"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"
)

const (
	errorNotFound                   = "[PAYONE_BILLING] %s not found"
	errorQueryMask                  = "[PAYONE_BILLING] Query from collection \"%s\" failed"
	errorAccountingCurrencyNotFound = "[PAYONE_BILLING] Accounting currency not found"

	errorBbNotFoundMessage = "not found"

	environmentProd = "prod"

	HeaderContentType   = "Content-Type"
	HeaderAuthorization = "Authorization"
	HeaderContentLength = "Content-Length"

	MIMEApplicationForm = "application/x-www-form-urlencoded"
	MIMEApplicationJSON = "application/json"

	DefaultPaymentMethodFee               = 3
	DefaultPaymentMethodPerTransactionFee = 2
	DefaultPaymentMethodCurrency          = "RUB"

	CountryCodeUSA = "US"
)

var (
	handlers = map[string]func(*Service) Cacher{
		pkg.CollectionCurrency:      newCurrencyHandler,
		pkg.CollectionCountry:       newCountryHandler,
		pkg.CollectionProject:       newProjectHandler,
		pkg.CollectionCurrencyRate:  newCurrencyRateHandler,
		pkg.CollectionPaymentMethod: newPaymentMethodHandler,
		pkg.CollectionCommission:    newCommissionHandler,
		pkg.CollectionMerchant:      newMerchantHandler,
	}
)

type Service struct {
	db     *database.Source
	mx     sync.Mutex
	cfg    *config.Config
	exitCh chan bool
	ctx    context.Context
	geo    proto.GeoIpService
	rep    repository.RepositoryService
	tax    tax_service.TaxService
	broker *rabbitmq.Broker

	accountingCurrency *billing.Currency

	currencyCache        map[string]*billing.Currency
	countryCache         map[string]*billing.Country
	projectCache         map[string]*billing.Project
	currencyRateCache    map[int32]map[int32]*billing.CurrencyRate
	paymentMethodCache   map[string]map[int32]*billing.PaymentMethod
	paymentMethodIdCache map[string]*billing.PaymentMethod

	merchantPaymentMethods map[string]map[string]*billing.MerchantPaymentMethod

	commissionCache           map[string]map[string]*billing.MerchantPaymentMethodCommissions
	projectPaymentMethodCache map[string][]*billing.PaymentFormPaymentMethod

	rebuild      bool
	rebuildError error
}

type Cacher interface {
	getAll() ([]interface{}, error)
	setCache([]interface{})
}

func NewBillingService(
	db *database.Source,
	cfg *config.Config,
	exitCh chan bool,
	geo proto.GeoIpService,
	rep repository.RepositoryService,
	tax tax_service.TaxService,
	broker *rabbitmq.Broker,
) *Service {
	return &Service{
		db:     db,
		cfg:    cfg,
		exitCh: exitCh,
		geo:    geo,
		rep:    rep,
		tax:    tax,
		broker: broker,
	}
}

func (s *Service) Init() (err error) {
	err = s.initCache()

	if err != nil {
		return
	}

	s.projectPaymentMethodCache = make(map[string][]*billing.PaymentFormPaymentMethod)
	s.accountingCurrency, err = s.GetCurrencyByCodeA3(s.cfg.AccountingCurrency)

	if err != nil {
		return errors.New(errorAccountingCurrencyNotFound)
	}

	go s.reBuildCache()

	return
}

func (s *Service) reBuildCache() {
	var err error
	var key string

	curTicker := time.NewTicker(time.Second * time.Duration(s.cfg.CurrencyTimeout))
	countryTicker := time.NewTicker(time.Second * time.Duration(s.cfg.CountryTimeout))
	projectTicker := time.NewTicker(time.Second * time.Duration(s.cfg.ProjectTimeout))
	currencyRateTicker := time.NewTicker(time.Second * time.Duration(s.cfg.CurrencyRateTimeout))
	paymentMethodTicker := time.NewTicker(time.Second * time.Duration(s.cfg.PaymentMethodTimeout))
	commissionTicker := time.NewTicker(time.Second * time.Duration(s.cfg.CommissionTimeout))
	projectPaymentMethodTimer := time.NewTicker(time.Second * time.Duration(s.cfg.ProjectPaymentMethodTimeout))

	s.rebuild = true

	for {
		select {
		case <-curTicker.C:
			err = s.cache(pkg.CollectionCurrency, handlers[pkg.CollectionCurrency](s))
			key = pkg.CollectionCurrency
		case <-countryTicker.C:
			err = s.cache(pkg.CollectionCountry, handlers[pkg.CollectionCountry](s))
			key = pkg.CollectionCountry
		case <-projectTicker.C:
			err = s.cache(pkg.CollectionProject, handlers[pkg.CollectionProject](s))
			key = pkg.CollectionProject
		case <-currencyRateTicker.C:
			err = s.cache(pkg.CollectionCurrencyRate, handlers[pkg.CollectionCurrencyRate](s))
			key = pkg.CollectionCurrencyRate
		case <-paymentMethodTicker.C:
			err = s.cache(pkg.CollectionPaymentMethod, handlers[pkg.CollectionPaymentMethod](s))
			key = pkg.CollectionPaymentMethod
		case <-commissionTicker.C:
			err = s.cache(pkg.CollectionCommission, handlers[pkg.CollectionCommission](s))
			key = pkg.CollectionCommission
		case <-projectPaymentMethodTimer.C:
			s.mx.Lock()
			s.projectPaymentMethodCache = make(map[string][]*billing.PaymentFormPaymentMethod)
			s.mx.Unlock()
		case <-s.exitCh:
			s.rebuild = false
			return
		}

		if err != nil {
			s.rebuild = false
			s.rebuildError = err

			zap.S().Errorw("Rebuild cache failed", "error", err, "cached_collection", key)
		}
	}
}

func (s *Service) cache(key string, handler Cacher) error {
	rec, err := handler.getAll()

	if err != nil {
		return err
	}

	s.mx.Lock()
	defer s.mx.Unlock()

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
	return s.cfg.Environment == environmentProd
}

func (s *Service) logError(msg string, data []interface{}) {
	zap.S().Errorw(fmt.Sprintf("[PAYSUPER_BILLING] %s", msg), data...)
}

func (s *Service) RebuildCache(ctx context.Context, req *grpc.EmptyRequest, res *grpc.EmptyResponse) error {
	return nil
}

func (s *Service) UpdateOrder(ctx context.Context, req *billing.Order, rsp *grpc.EmptyResponse) error {
	err := s.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(req.Id), req)

	if err != nil {
		s.logError("Update order failed", []interface{}{"error", err.Error(), "order", req})
	}

	return nil
}

func (s *Service) UpdateMerchant(ctx context.Context, req *billing.Merchant, rsp *grpc.EmptyResponse) error {
	err := s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(req.Id), req)

	if err != nil {
		s.logError("Update merchant failed", []interface{}{"error", err.Error(), "order", req})
	}

	return nil
}

func (s *Service) GetConvertRate(ctx context.Context, req *grpc.ConvertRateRequest, rsp *grpc.ConvertRateResponse) error {
	rate, err := s.Convert(req.From, req.To, 1)

	if err != nil {
		s.logError("Get convert rate failed", []interface{}{"error", err.Error(), "from", req.From, "to", req.To})
	} else {
		rsp.Rate = rate
	}

	return nil
}

func (s *Service) IsDbNotFoundError(err error) bool {
	return err.Error() == errorBbNotFoundMessage
}

func (s *Service) getCountryFromAcceptLanguage(acceptLanguage string) (string, string) {
	it := strings.Split(acceptLanguage, ",")

	if strings.Index(it[0], "-") == -1 {
		return "", ""
	}

	it = strings.Split(it[0], "-")

	return strings.ToLower(it[0]), strings.ToUpper(it[1])
}
