package service

import (
	"context"
	"errors"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
    "github.com/paysuper/paysuper-recurring-repository/tools"
    "sort"
)

type kv struct {
	Key   int
	Value float64
}

const (
	errorSystemFeeCardBrandRequired        = "card brand required for this method"
	errorSystemFeeCardBrandNotAllowed      = "card brand not allowed for this method"
	errorSystemFeeCardBrandInvalid         = "card brand invalid or not supported"
	errorSystemFeeNotFound                 = "system fee not found"
	errorSystemFeeMatchedMinAmountNotFound = "system fee matched min amount not found"
	errorSystemFeeDuplicatedActive         = "duplicated active system fee"
	errorSystemFeeRegionInvalid            = "system fee region invalid"
	errorSystemFeeRequiredFeeset           = "system fees require alt least one fee set in request"
)

var CardBrands = []string{
	"JCB",
	"MASTERCARD",
	"UNIONPAY",
	"VISA",
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}

func (s *Service) AddSystemFees(
	ctx context.Context,
	req *billing.AddSystemFeesRequest,
	res *grpc.EmptyResponse,
) error {

	if req.Region != "" && req.Region != "EU" {
		_, err := s.GetCountryByCodeA2(req.Region)
		if err != nil {
			s.logError(errorSystemFeeRegionInvalid, []interface{}{"data", req})
			return errors.New(errorSystemFeeRegionInvalid)
		}
	}

	method, err := s.GetPaymentMethodById(req.MethodId)
	if err != nil {
		s.logError("GetPaymentMethodById failed", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	if method.IsBankCard() == true {
		if req.CardBrand == "" {
			s.logError(errorSystemFeeCardBrandRequired, []interface{}{"data", req})
			return errors.New(errorSystemFeeCardBrandRequired)
		}
		if !contains(CardBrands, req.CardBrand) {
			s.logError(errorSystemFeeCardBrandInvalid, []interface{}{"data", req})
			return errors.New(errorSystemFeeCardBrandInvalid)
		}
	} else {
		if req.CardBrand != "" {
			s.logError(errorSystemFeeCardBrandNotAllowed, []interface{}{"data", req})
			return errors.New(errorSystemFeeCardBrandNotAllowed)
		}
	}

	if len(req.Fees) == 0 {
		s.logError(errorSystemFeeRequiredFeeset, []interface{}{"data", req})
		return errors.New(errorSystemFeeRequiredFeeset)
	}

    // formatting values
	for _, f := range req.Fees {
        f.TransactionCost.Percent = tools.FormatAmount(f.TransactionCost.Percent)
        f.TransactionCost.FixAmount = tools.FormatAmount(f.TransactionCost.FixAmount)
        f.AuthorizationFee.Percent = tools.FormatAmount(f.AuthorizationFee.Percent)
        f.AuthorizationFee.FixAmount = tools.FormatAmount(f.AuthorizationFee.FixAmount)

        for c, v := range f.MinAmounts {
            f.MinAmounts[c] = tools.FormatAmount(v)
        }
    }

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

	query := bson.M{"method_id": bson.ObjectIdHex(req.MethodId), "region": req.Region, "card_brand": req.CardBrand, "is_active": true}
	err = s.db.Collection(pkg.CollectionSystemFees).Update(query, bson.M{"$set": bson.M{"is_active": false}})

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Query to disable old fees failed", []interface{}{"err", err.Error(), "query", query, "req", req})
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
	sf, ok := s.systemFeesCache[req.MethodId]
	if !ok {
		return errors.New(errorSystemFeeNotFound)
	}

	sfr, ok := sf[req.Region]
	if !ok {
		return errors.New(errorSystemFeeNotFound)
	}

	systemFees, ok := sfr[req.CardBrand]
	if !ok {
		return errors.New(errorSystemFeeNotFound)
	}

	var matchedAmounts []*kv

	for k, f := range systemFees.Fees {
		minA, ok := f.MinAmounts[req.Currency]
		if !ok {
			continue
		}
		if req.Amount >= minA {
			matchedAmounts = append(matchedAmounts, &kv{k, minA})
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
	var (
		fees  []*billing.SystemFees
		query = bson.M{"is_active": true}
	)
	e := s.db.Collection(pkg.CollectionSystemFees).Find(query).All(&fees)
	if e != nil {
		s.logError("Get System fees failed", []interface{}{"err", e.Error(), "query", query})
		return e
	}
	res.SystemFees = fees
	return nil
}
