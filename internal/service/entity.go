package service

import (
	"fmt"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/globalsign/mgo/bson"
)

type Project Currency
type PaymentMethod Currency

func newProjectHandler(svc *Service) Cacher {
	c := &Project{svc: svc}

	return c
}

func (h *Project) setCache(recs []interface{}) {
	h.svc.projectCache = make(map[string]*billing.Project)

	for _, r := range recs {
		project := r.(*billing.Project)

		h.svc.mx.Lock()
		h.svc.projectCache[project.Id] = project
		h.svc.mx.Unlock()
	}
}

func (h *Project) getAll() (recs []interface{}, err error) {
	var data []*billing.Project

	err = h.svc.db.Collection(collectionProject).Find(bson.M{}).All(&data)

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
		return nil, fmt.Errorf(errorNotFound, collectionProject)
	}

	return rec, nil
}

func newPaymentMethodHandler(svc *Service) Cacher {
	c := &PaymentMethod{svc: svc}

	return c
}

func (h *PaymentMethod) setCache(recs []interface{}) {
	h.svc.paymentMethodCache = make(map[string]map[int32]*billing.PaymentMethod)

	for _, r := range recs {
		pm := r.(*billing.PaymentMethod)

		h.svc.mx.Lock()

		if _, ok := h.svc.paymentMethodCache[pm.Group]; !ok {
			h.svc.paymentMethodCache[pm.Group] = make(map[int32]*billing.PaymentMethod)
		}

		for _, v := range pm.Currencies  {
			h.svc.paymentMethodCache[pm.Group][v] = pm
		}

		h.svc.mx.Unlock()
	}
}

func (h *PaymentMethod) getAll() (recs []interface{}, err error) {
	var data []*billing.PaymentMethod

	err = h.svc.db.Collection(collectionPaymentMethod).Find(bson.M{}).All(&data)

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
		return nil, fmt.Errorf(errorNotFound, collectionPaymentMethod)
	}

	rec, ok := pmGroup[currency]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, collectionPaymentMethod)
	}

	return rec, nil
}
