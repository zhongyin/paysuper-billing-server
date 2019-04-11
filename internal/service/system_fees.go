package service

import (
	"context"
	"errors"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"sort"
)

func (s *Service) AddSystemFees(
	ctx context.Context,
	req *billing.AddSystemFeesRequest,
	res *grpc.EmptyResponse,
) error {

	fees := &billing.SystemFees{
		Id:        bson.NewObjectId().Hex(),
		MethodId:  req.MethodId,
		Region:    req.Region,
		CardBrand: req.CardBrand,
		Fees:      req.Fees,
		UserId:    req.UserId,
		CreatedAt: ptypes.TimestampNow(),
		IsActive:  true,
	}

	method, err := s.GetPaymentMethodById(req.MethodId)
	if err != nil {
		s.logError("GetPaymentMethodById failed", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	if method.IsBankCard() == true {
		if fees.CardBrand == "" {
			err = errors.New("card brand required for this method")
			s.logError("Card brand required for this method", []interface{}{"err", err.Error(), "data", req})
			return err
		}
	} else {
		fees.CardBrand = ""
	}

	query := bson.M{"method_id": bson.ObjectIdHex(req.MethodId), "region": req.Region, "card_brand": req.CardBrand, "is_active": true}
	err = s.db.Collection(pkg.CollectionSystemFees).Update(query, bson.M{"$set": bson.M{"is_active": false}})

	if err != nil && !s.IsDbNotFoundError(err) {
		s.logError("Query to disable old fees failed", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	err = s.db.Collection(pkg.CollectionSystemFees).Insert(fees)

	if err != nil {
		s.logError("Query to add fees failed", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	// updating a cache
	s.mx.Lock()
	defer s.mx.Unlock()

	if _, ok := s.systemFeesCache[fees.MethodId]; !ok {
		s.systemFeesCache[fees.MethodId] = make(map[string]map[string]*billing.SystemFees)
	}
	if _, ok := s.systemFeesCache[fees.MethodId][fees.Region]; !ok {
		s.systemFeesCache[fees.MethodId][fees.Region] = make(map[string]*billing.SystemFees)
	}
	s.systemFeesCache[fees.MethodId][fees.Region][fees.CardBrand] = fees

	return nil
}

func (s *Service) GetSystemFeesForPayment(
	ctx context.Context,
	req *billing.GetSystemFeesRequest,
	res *billing.FeeSet,
) error {
	systemFees, ok := s.systemFeesCache[req.MethodId][req.Region][req.CardBrand]

	if !ok {
		return errors.New(errorSystemFeeNotFound)
	}

	var matchedAmounts []kv

	for k, f := range systemFees.Fees {
		minA, ok := f.MinAmounts[req.Currency]
		if !ok {
			continue
		}
		if req.Amount >= minA {
			matchedAmounts = append(matchedAmounts, kv{k, minA})
		}
	}

	if len(matchedAmounts) == 0 {
		return errors.New(errorSystemFeeMatchedMinAmountNotFound)
	}

	sort.Slice(matchedAmounts, func(i, j int) bool {
		return matchedAmounts[i].Value > matchedAmounts[j].Value
	})

	f := systemFees.Fees[matchedAmounts[0].Key]
	res.MinAmounts = f.MinAmounts
	res.TransactionCost = f.TransactionCost
	res.AuthorizationFee = f.AuthorizationFee
	return nil
}

func (s *Service) GetActualSystemFeesList(
	ctx context.Context,
	req *grpc.EmptyRequest,
	res *billing.SystemFeesList,
) error {
	var fees []*billing.SystemFees
	e := s.db.Collection(pkg.CollectionSystemFees).Find(bson.M{"is_active": true}).All(&fees)
	if e != nil {
		s.logError("Get System fees failed", []interface{}{"err", e.Error()})
		return e
	}
	res.SystemFees = fees
	return nil
}
