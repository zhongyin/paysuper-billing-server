package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
)

type Project Currency
type PaymentMethod Currency
type Country Currency
type Merchant Currency
type SystemFee Currency

func newProjectHandler(svc *Service) Cacher {
	c := &Project{svc: svc}

	return c
}

func (h *Project) setCache(recs []interface{}) {
	h.svc.projectCache = make(map[string]*billing.Project, len(recs))

	if len(recs) <= 0 {
		return
	}

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

	if len(recs) <= 0 {
		return
	}

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

	if len(recs) <= 0 {
		return
	}

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
	h.svc.merchantCache = make(map[string]*billing.Merchant)

	if len(recs) <= 0 {
		return
	}

	for _, r := range recs {
		m := r.(*billing.Merchant)
		h.svc.merchantCache[m.Id] = m

		if _, ok := h.svc.merchantPaymentMethods[m.Id]; !ok {
			h.svc.merchantPaymentMethods[m.Id] = make(map[string]*billing.MerchantPaymentMethod)
		}

		if len(m.PaymentMethods) > 0 {
			for k, v := range m.PaymentMethods {
				h.svc.merchantPaymentMethods[m.Id][k] = v
			}
		}

		if len(h.svc.merchantPaymentMethods[m.Id]) != len(h.svc.paymentMethodIdCache) {
			for k, v := range h.svc.paymentMethodIdCache {
				_, ok := h.svc.merchantPaymentMethods[m.Id][k]

				if ok {
					continue
				}

				h.svc.merchantPaymentMethods[m.Id][k] = &billing.MerchantPaymentMethod{
					PaymentMethod: &billing.MerchantPaymentMethodIdentification{
						Id:   k,
						Name: v.Name,
					},
					Commission: &billing.MerchantPaymentMethodCommissions{
						Fee: DefaultPaymentMethodFee,
						PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
							Fee:      DefaultPaymentMethodPerTransactionFee,
							Currency: DefaultPaymentMethodCurrency,
						},
					},
					Integration: &billing.MerchantPaymentMethodIntegration{},
					IsActive:    true,
				}
			}
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

func (s *Service) getMerchantPaymentMethod(merchantId, pmId string) (*billing.MerchantPaymentMethod, error) {
	pms, ok := s.merchantPaymentMethods[merchantId]

	if !ok {
		return nil, errors.New(orderErrorPaymentMethodNotAllowed)
	}

	pm, ok := pms[pmId]

	if !ok {
		return nil, errors.New(orderErrorPaymentMethodNotAllowed)
	}

	return pm, nil
}

func (s *Service) getMerchantPaymentMethodTerminalId(merchantId, pmId string) (string, error) {
	pm, err := s.getMerchantPaymentMethod(merchantId, pmId)

	if err != nil {
		return "", err
	}

	if pm.Integration == nil || pm.Integration.TerminalId == "" {
		return "", errors.New(orderErrorPaymentMethodEmptySettings)
	}

	return pm.Integration.TerminalId, nil
}

func (s *Service) getMerchantPaymentMethodTerminalPassword(merchantId, pmId string) (string, error) {
	pm, err := s.getMerchantPaymentMethod(merchantId, pmId)

	if err != nil {
		return "", err
	}

	if pm.Integration == nil || pm.Integration.TerminalPassword == "" {
		return "", errors.New(orderErrorPaymentMethodEmptySettings)
	}

	return pm.Integration.TerminalPassword, nil
}

func (s *Service) getMerchantPaymentMethodTerminalCallbackPassword(merchantId, pmId string) (string, error) {
	pm, err := s.getMerchantPaymentMethod(merchantId, pmId)

	if err != nil {
		return "", err
	}

	if pm.Integration == nil || pm.Integration.TerminalCallbackPassword == "" {
		return "", errors.New(orderErrorPaymentMethodEmptySettings)
	}

	return pm.Integration.TerminalCallbackPassword, nil
}

func newSystemFeeHandler(svc *Service) Cacher {
	c := &SystemFee{svc: svc}

	return c
}

func (h *SystemFee) getAll() (recs []interface{}, err error) {
	list := &billing.SystemFeesList{}
	e := h.svc.GetActualSystemFeesList(context.TODO(), &grpc.EmptyRequest{}, list)
	if e != nil {
		h.svc.logError("Get System fees failed", []interface{}{"err", e.Error()})
		return nil, e
	}

	for _, f := range list.SystemFees {
		recs = append(recs, f)
	}
	return
}

func (h *SystemFee) setCache(recs []interface{}) {
	h.svc.systemFeesCache = make(map[string]map[string]map[string]*billing.SystemFees)

	if len(recs) <= 0 {
		return
	}

	for _, r := range recs {
		f := r.(*billing.SystemFees)

		if _, ok := h.svc.systemFeesCache[f.MethodId]; !ok {
			h.svc.systemFeesCache[f.MethodId] = make(map[string]map[string]*billing.SystemFees)
		}

		if _, ok := h.svc.systemFeesCache[f.MethodId][f.Region]; !ok {
			h.svc.systemFeesCache[f.MethodId][f.Region] = make(map[string]*billing.SystemFees)
		}

		if ff, ok := h.svc.systemFeesCache[f.MethodId][f.Region][f.CardBrand]; ok && ff != nil {
			h.svc.logError(errorSystemFeeDuplicatedActive, []interface{}{"fee", ff})
			return
		}

		h.svc.systemFeesCache[f.MethodId][f.Region][f.CardBrand] = f
	}
}
