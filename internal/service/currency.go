package service

import (
	"fmt"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/globalsign/mgo/bson"
)

type Currency struct {
	svc *Service
}

func newCurrencyHandler(svc *Service) Cacher {
	c := &Currency{svc: svc}

	return c
}

func (h *Currency) setCache(recs []interface{}) {
	h.svc.curCache = make(map[string]*billing.Currency)

	for _, c := range recs {
		cur := c.(*billing.Currency)

		h.svc.mx.Lock()
		h.svc.curCache[cur.CodeA3] = cur
		h.svc.mx.Unlock()
	}
}

func (h *Currency) getAll() (recs []interface{}, err error) {
	var data []*billing.Currency

	err = h.svc.db.Collection(collectionCurrency).Find(bson.M{"is_active": true}).All(&data)

	if data != nil {
		for _, v := range data {
			recs = append(recs, v)
		}
	}

	return
}

func (s *Service) GetCurrencyByCodeA3(code string) (*billing.Currency, error) {
	rec, ok := s.curCache[code]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, collectionCurrency)
	}

	return rec, nil
}
