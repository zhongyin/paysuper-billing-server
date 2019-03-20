package service

import (
	"fmt"
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-recurring-repository/tools"
)

type Currency struct {
	svc *Service
}

type CurrencyRate Currency
type Vat Currency
type Commission Currency

func newCurrencyHandler(svc *Service) Cacher {
	return &Currency{svc: svc}
}

func (h *Currency) setCache(recs []interface{}) {
	h.svc.currencyCache = make(map[string]*billing.Currency, len(recs))

	if len(recs) <= 0 {
		return
	}

	for _, c := range recs {
		cur := c.(*billing.Currency)
		h.svc.currencyCache[cur.CodeA3] = cur
	}
}

func (h *Currency) getAll() (recs []interface{}, err error) {
	var data []*billing.Currency

	err = h.svc.db.Collection(pkg.CollectionCurrency).Find(bson.M{"is_active": true}).All(&data)

	if data != nil {
		for _, v := range data {
			recs = append(recs, v)
		}
	}

	return
}

func (s *Service) GetCurrencyByCodeA3(code string) (*billing.Currency, error) {
	rec, ok := s.currencyCache[code]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, pkg.CollectionCurrency)
	}

	return rec, nil
}

func newCurrencyRateHandler(svc *Service) Cacher {
	return &CurrencyRate{svc: svc}
}

func (h *CurrencyRate) setCache(recs []interface{}) {
	h.svc.currencyRateCache = make(map[int32]map[int32]*billing.CurrencyRate, len(recs))

	if len(recs) <= 0 {
		return
	}

	for _, c := range recs {
		rate := c.(*billing.CurrencyRate)

		if _, ok := h.svc.currencyRateCache[rate.CurrencyFrom]; !ok {
			h.svc.currencyRateCache[rate.CurrencyFrom] = make(map[int32]*billing.CurrencyRate, len(recs))
		}

		h.svc.currencyRateCache[rate.CurrencyFrom][rate.CurrencyTo] = rate
	}
}

func (h *CurrencyRate) getAll() (recs []interface{}, err error) {
	var data []*billing.CurrencyRate

	err = h.svc.db.Collection(pkg.CollectionCurrencyRate).Find(bson.M{"is_active": true}).All(&data)

	if data != nil {
		for _, v := range data {
			recs = append(recs, v)
		}
	}

	return
}

func (s *Service) Convert(from int32, to int32, value float64) (float64, error) {
	fRates, ok := s.currencyRateCache[from]

	if !ok {
		return 0, fmt.Errorf(errorNotFound, pkg.CollectionCurrencyRate)
	}

	rec, ok := fRates[to]

	if !ok {
		return 0, fmt.Errorf(errorNotFound, pkg.CollectionCurrencyRate)
	}

	value = value / rec.Rate

	return tools.FormatAmount(value), nil
}

func newCommissionHandler(svc *Service) Cacher {
	return &Commission{svc: svc}
}

func (h *Commission) setCache(recs []interface{}) {
	h.svc.commissionCache = make(map[string]map[string]*billing.MerchantPaymentMethodCommissions, len(recs))

	if len(recs) <= 0 {
		return
	}

	for _, v := range recs {
		h.svc.commissionCache = v.(map[string]map[string]*billing.MerchantPaymentMethodCommissions)
	}
}

func (h *Commission) getAll() (recs []interface{}, err error) {
	var merchants []*billing.Merchant
	var projects []*billing.Project

	err = h.svc.db.Collection(pkg.CollectionMerchant).Find(bson.M{}).All(&merchants)

	if err != nil {
		return
	}

	for _, v := range merchants {
		query := bson.M{"merchant._id": bson.ObjectIdHex(v.Id)}
		err = h.svc.db.Collection(pkg.CollectionProject).Find(query).All(&projects)

		if err != nil {
			continue
		}

		for _, v1 := range projects {
			commission := make(map[string]map[string]*billing.MerchantPaymentMethodCommissions)

			_, ok := commission[v1.Id]

			if !ok {
				commission[v1.Id] = make(map[string]*billing.MerchantPaymentMethodCommissions)
			}

			for k, v2 := range v.PaymentMethods {
				commission[v1.Id][k] = v2.Commission
			}

			if len(h.svc.paymentMethodIdCache) != len(commission[v1.Id]) {
				for k := range h.svc.paymentMethodIdCache {
					_, ok := commission[v1.Id][k]

					if ok {
						continue
					}

					commission[v1.Id][k] = &billing.MerchantPaymentMethodCommissions{
						Fee: DefaultPaymentMethodFee,
						PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
							Fee:      DefaultPaymentMethodPerTransactionFee,
							Currency: DefaultPaymentMethodCurrency,
						},
					}
				}
			}

			recs = append(recs, commission)
		}
	}

	return
}

func (s *Service) CalculatePmCommission(projectId, pmId string, amount float64) (float64, error) {
	prjCom, ok := s.commissionCache[projectId]

	if !ok {
		return 0, fmt.Errorf(errorNotFound, pkg.CollectionCommission)
	}

	prjPmCom, ok := prjCom[pmId]

	if !ok {
		return 0, fmt.Errorf(errorNotFound, pkg.CollectionCommission)
	}

	return tools.FormatAmount(amount * (prjPmCom.Fee / 100)), nil
}
