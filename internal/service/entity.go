package service

import (
	"fmt"
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
)

type Project Currency
type PaymentMethod Currency
type Country Currency
type Merchant Currency

func newProjectHandler(svc *Service) Cacher {
	c := &Project{svc: svc}

	return c
}

func (h *Project) setCache(recs []interface{}) {
	h.svc.projectCache = make(map[string]*billing.Project, len(recs))

	for _, r := range recs {
		project := r.(*billing.Project)
		h.svc.projectCache[project.Id] = project
	}
}

func (h *Project) getAll() (recs []interface{}, err error) {
	var data []*billing.Project

	err = h.svc.db.Collection(pkg.CollectionProject).Find(bson.M{}).All(&data)

	if data != nil {
		for _, v := range data {
			recs = append(recs, v)
		}
	}

	return
}

func (s *Service) GetProjectById(id string) (*billing.Project, error) {
	rec, ok := s.projectCache[id]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, pkg.CollectionProject)
	}

	return rec, nil
}

func newPaymentMethodHandler(svc *Service) Cacher {
	c := &PaymentMethod{svc: svc}

	return c
}

func (h *PaymentMethod) setCache(recs []interface{}) {
	recsLen := len(recs)

	h.svc.paymentMethodCache = make(map[string]map[int32]*billing.PaymentMethod, recsLen)
	h.svc.paymentMethodIdCache = make(map[string]*billing.PaymentMethod, recsLen)

	for _, r := range recs {
		pm := r.(*billing.PaymentMethod)

		if _, ok := h.svc.paymentMethodCache[pm.Group]; !ok {
			h.svc.paymentMethodCache[pm.Group] = make(map[int32]*billing.PaymentMethod, len(pm.Currencies))
		}

		for _, v := range pm.Currencies {
			h.svc.paymentMethodCache[pm.Group][v] = pm
		}

		h.svc.paymentMethodIdCache[pm.Id] = pm
	}
}

func (h *PaymentMethod) getAll() (recs []interface{}, err error) {
	var data []*billing.PaymentMethod

	err = h.svc.db.Collection(pkg.CollectionPaymentMethod).Find(bson.M{}).All(&data)

	if data != nil {
		for _, v := range data {
			recs = append(recs, v)
		}
	}

	return
}

func (s *Service) GetPaymentMethodByGroupAndCurrency(group string, currency int32) (*billing.PaymentMethod, error) {
	pmGroup, ok := s.paymentMethodCache[group]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, pkg.CollectionPaymentMethod)
	}

	rec, ok := pmGroup[currency]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, pkg.CollectionPaymentMethod)
	}

	return rec, nil
}

func (s *Service) GetPaymentMethodById(id string) (*billing.PaymentMethod, error) {
	rec, ok := s.paymentMethodIdCache[id]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, pkg.CollectionPaymentMethod)
	}

	return rec, nil
}

func newCountryHandler(svc *Service) Cacher {
	c := &Country{svc: svc}

	return c
}

func (h *Country) setCache(recs []interface{}) {
	h.svc.countryCache = make(map[string]*billing.Country, len(recs))

	for _, r := range recs {
		country := r.(*billing.Country)
		h.svc.countryCache[country.CodeA2] = country
	}
}

func (h *Country) getAll() (recs []interface{}, err error) {
	var data []*billing.Country

	err = h.svc.db.Collection(pkg.CollectionCountry).Find(bson.M{"is_active": true}).All(&data)

	if data != nil {
		for _, v := range data {
			recs = append(recs, v)
		}
	}

	return
}

func (s *Service) GetCountryByCodeA2(id string) (*billing.Country, error) {
	rec, ok := s.countryCache[id]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, pkg.CollectionCountry)
	}

	return rec, nil
}

func newMerchantHandler(svc *Service) Cacher {
	c := &Merchant{svc: svc}

	return c
}

func (h *Merchant) setCache(recs []interface{}) {
	h.svc.merchantPaymentMethods = make(map[string]map[string]*billing.MerchantPaymentMethod)

	for _, r := range recs {
		m := r.(*billing.Merchant)

		if _, ok := h.svc.merchantPaymentMethods[m.Id]; !ok {
			h.svc.merchantPaymentMethods[m.Id] = make(map[string]*billing.MerchantPaymentMethod)
		}

		if len(m.PaymentMethods) <= 0 {
			continue
		}

		for k, v := range m.PaymentMethods {
			h.svc.merchantPaymentMethods[m.Id][k] = v
		}
	}
}

func (h *Merchant) getAll() (recs []interface{}, err error) {
	var data []*billing.Merchant

	err = h.svc.db.Collection(pkg.CollectionMerchant).Find(bson.M{}).All(&data)

	if data != nil {
		for _, v := range data {
			recs = append(recs, v)
		}
	}

	return
}