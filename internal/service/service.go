package service

import (
	"fmt"
	"github.com/ProtocolONE/payone-billing-service/internal/database"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"go.uber.org/zap"
	"sync"
	"time"
)

const (
	collectionCurrency = "currency"

	errorNotFound          = "[PAYONE_BILLING] %s not found"
	initCacheErrorNotFound = "[PAYONE_BILLING] %s query result is empty"
)

var (
	handlers = map[string]func(*Service) Cacher{
		collectionCurrency: newCurrencyHandler,
	}
)

type CacheConfig struct {
	CurrencyTimeout int64 `envconfig:"CACHE_CURRENCY_TIMEOUT" default:"86400"`
}

type Service struct {
	db     *database.Source
	log    *zap.SugaredLogger
	mx     sync.Mutex
	cCfg   *CacheConfig
	exitCh chan bool

	curCache map[string]*billing.Currency
}

type Cacher interface {
	getAll() ([]interface{}, error)
	setCache([]interface{})
}

func NewBillingService(
	db *database.Source,
	log *zap.SugaredLogger,
	cCfg *CacheConfig,
	exitCh chan bool,
) (svc *Service, err error) {
	svc = &Service{
		db:     db,
		log:    log,
		cCfg:   cCfg,
		exitCh: exitCh,
	}

	err = svc.initCache()

	return
}

func (s *Service) reBuildCache() {
	var err error
	var key string

	curTicker := time.NewTicker(time.Second * time.Duration(s.cCfg.CurrencyTimeout))

	for {
		select {
		case <-curTicker.C:
			err = s.cache(collectionCurrency, handlers[collectionCurrency](s))
			key = collectionCurrency
		case <-s.exitCh:
			return
		}

		if err != nil {
			s.log.Fatalw("Rebuild cache failed", "error", err, "cached_collection", key)
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

	go s.reBuildCache()

	return nil
}
