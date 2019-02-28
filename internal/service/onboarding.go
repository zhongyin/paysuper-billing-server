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
	"time"
)

const (
	merchantErrorChangeNotAllowed   = "merchant data changing not allowed"
	merchantErrorCountryNotFound    = "merchant country not found"
	merchantErrorCurrencyNotFound   = "merchant bank accounting currency not found"
	merchantErrorStatusDraft        = "merchant status can't be set to draft. draft status allowed only for new merchant"
	merchantErrorAgreementRequested = "agreement for merchant can't be requested"
	merchantErrorOnReview           = "merchant hasn't allowed status for review"
	merchantErrorReturnFromReview   = "this action is impossible by workflow"
	merchantErrorSigning            = "signing unapproved merchant is impossible"
	merchantErrorSigned             = "document can't be mark as signed"
	merchantErrorUnknown            = "request processing failed. try request later"
	merchantErrorNotFound           = "merchant with specified identifier not found"
)

var (
	ErrMerchantNotFound = errors.New(merchantErrorNotFound)
)

func (s *Service) GetMerchantById(ctx context.Context, req *grpc.FindByIdRequest, rsp *billing.Merchant) error {
	merchant, err := s.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(req.Id)})

	if err != nil {
		return err
	}

	s.mapMerchantData(rsp, merchant)

	return nil
}

func (s *Service) GetMerchantByExternalId(
	ctx context.Context,
	req *grpc.FindByIdRequest,
	rsp *billing.Merchant,
) error {
	merchant, err := s.getMerchantBy(bson.M{"external_id": req.Id})

	if err != nil {
		return err
	}

	s.mapMerchantData(rsp, merchant)

	return nil
}

func (s *Service) ListMerchants(ctx context.Context, req *grpc.MerchantListingRequest, rsp *grpc.Merchants) error {
	var merchants []*billing.Merchant

	query := make(bson.M)

	if req.Name != "" {
		query["name"] = bson.RegEx{Pattern: ".*" + req.Name + ".*", Options: "i"}
	}

	if req.LastPayoutDateFrom > 0 || req.LastPayoutDateTo > 0 {
		payoutDates := make(bson.M)

		if req.LastPayoutDateFrom > 0 {
			payoutDates["$gte"] = time.Unix(req.LastPayoutDateFrom, 0)
		}

		if req.LastPayoutDateTo > 0 {
			payoutDates["$lte"] = time.Unix(req.LastPayoutDateTo, 0)
		}

		query["last_payout.date"] = payoutDates
	}

	if req.IsAgreement > 0 {
		if req.IsAgreement == 1 {
			query["is_agreement"] = false
		} else {
			query["is_agreement"] = true
		}
	}

	if req.LastPayoutAmount > 0 {
		query["last_payout.amount"] = req.LastPayoutAmount
	}

	err := s.db.Collection(pkg.CollectionMerchant).Find(query).Sort(req.Sort...).Limit(int(req.Limit)).
		Skip(int(req.Offset)).All(&merchants)

	if err != nil {
		s.logError("Query to find merchants failed", []interface{}{"err", err.Error(), "query", query})
		return errors.New(merchantErrorUnknown)
	}

	if len(merchants) > 0 {
		rsp.Merchants = merchants
	}

	return nil
}

func (s *Service) ChangeMerchant(ctx context.Context, req *grpc.OnboardingRequest, rsp *billing.Merchant) (err error) {
	isNew := false
	merchant, err := s.getMerchantBy(bson.M{"external_id": req.ExternalId})

	if err != nil {
		if err != ErrMerchantNotFound {
			return err
		}

		isNew = true

		merchant = &billing.Merchant{
			Id:        bson.NewObjectId().Hex(),
			Status:    pkg.MerchantStatusDraft,
			CreatedAt: ptypes.TimestampNow(),
		}
	}

	if merchant.ChangesAllowed() == false {
		return errors.New(merchantErrorChangeNotAllowed)
	}

	country, err := s.GetCountryByCodeA2(req.Country)

	if err != nil {
		s.logError("Get country for merchant failed", []interface{}{"err", err.Error(), "request", req})
		return errors.New(merchantErrorCountryNotFound)
	}

	currency, err := s.GetCurrencyByCodeA3(req.Banking.Currency)

	if err != nil {
		s.logError("Get currency for merchant failed", []interface{}{"err", err.Error(), "request", req})
		return errors.New(merchantErrorCurrencyNotFound)
	}

	merchant.ExternalId = req.ExternalId
	merchant.AccountEmail = req.AccountingEmail
	merchant.CompanyName = req.Name
	merchant.AlternativeName = req.AlternativeName
	merchant.Website = req.Website
	merchant.Country = country
	merchant.State = req.State
	merchant.Zip = req.Zip
	merchant.City = req.City
	merchant.Address = req.Address
	merchant.AddressAdditional = req.AddressAdditional
	merchant.RegistrationNumber = req.RegistrationNumber
	merchant.TaxId = req.TaxId
	merchant.Contacts = req.Contacts
	merchant.Banking = &billing.MerchantBanking{
		Currency:      currency,
		Name:          req.Banking.Name,
		Address:       req.Banking.Address,
		AccountNumber: req.Banking.AccountNumber,
		Swift:         req.Banking.Swift,
		Details:       req.Banking.Details,
	}

	if isNew {
		err = s.db.Collection(pkg.CollectionMerchant).Insert(merchant)
	} else {
		err = s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)
	}

	if err != nil {
		s.logError("Query to change merchant data failed", []interface{}{"err", err.Error(), "data", merchant})
		return errors.New(merchantErrorUnknown)
	}

	s.mapMerchantData(rsp, merchant)

	return
}

func (s *Service) ChangeMerchantStatus(
	ctx context.Context,
	req *grpc.MerchantChangeStatusRequest,
	rsp *billing.Merchant,
) error {
	merchant, err := s.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(req.Id)})

	if err != nil {
		return err
	}

	if req.Status == pkg.MerchantStatusDraft {
		return errors.New(merchantErrorStatusDraft)
	}

	if req.Status == pkg.MerchantStatusAgreementRequested && merchant.Status != pkg.MerchantStatusDraft &&
		merchant.Status != pkg.MerchantStatusRejected {
		return errors.New(merchantErrorAgreementRequested)
	}

	if req.Status == pkg.MerchantStatusOnReview && merchant.Status != pkg.MerchantStatusAgreementRequested {
		return errors.New(merchantErrorOnReview)
	}

	if (req.Status == pkg.MerchantStatusApproved || req.Status == pkg.MerchantStatusRejected) &&
		merchant.Status != pkg.MerchantStatusOnReview {
		return errors.New(merchantErrorReturnFromReview)
	}

	if req.Status == pkg.MerchantStatusAgreementSigning && merchant.Status != pkg.MerchantStatusApproved {
		return errors.New(merchantErrorSigning)
	}

	if req.Status == pkg.MerchantStatusAgreementSigned && (merchant.Status != pkg.MerchantStatusAgreementSigning ||
		merchant.HasMerchantSignature != true || merchant.HasPspSignature != true) {
		return errors.New(merchantErrorSigned)
	}

	merchant.Status = req.Status

	err = s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)

	if err != nil {
		s.logError("Query to change merchant data failed", []interface{}{"err", err.Error(), "data", rsp})
		return errors.New(merchantErrorUnknown)
	}

	s.mapMerchantData(rsp, merchant)

	return nil
}

func (s *Service) getMerchantBy(query bson.M) (merchant *billing.Merchant, err error) {
	err = s.db.Collection(pkg.CollectionMerchant).Find(query).One(&merchant)

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Query to find merchant by id failed", []interface{}{"err", err.Error(), "query", query})

		return merchant, errors.New(merchantErrorUnknown)
	}

	if merchant == nil {
		return merchant, ErrMerchantNotFound
	}

	return
}

func (s *Service) mapMerchantData(rsp *billing.Merchant, merchant *billing.Merchant) {
	rsp.Id = merchant.Id
	rsp.Status = merchant.Status
	rsp.CreatedAt = merchant.CreatedAt
	rsp.ExternalId = merchant.ExternalId
	rsp.AccountEmail = merchant.AccountEmail
	rsp.CompanyName = merchant.CompanyName
	rsp.AlternativeName = merchant.AlternativeName
	rsp.Website = merchant.Website
	rsp.Country = merchant.Country
	rsp.State = merchant.State
	rsp.Zip = merchant.Zip
	rsp.City = merchant.City
	rsp.Address = merchant.Address
	rsp.AddressAdditional = merchant.AddressAdditional
	rsp.RegistrationNumber = merchant.RegistrationNumber
	rsp.TaxId = merchant.TaxId
	rsp.Contacts = merchant.Contacts
	rsp.Banking = merchant.Banking
	rsp.HasMerchantSignature = merchant.HasMerchantSignature
	rsp.HasPspSignature = merchant.HasPspSignature
	rsp.LastPayout = merchant.LastPayout
}
