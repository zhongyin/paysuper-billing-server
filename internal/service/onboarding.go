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
)

const (
	merchantErrorChangeNotAllowed = "merchant data changing not allowed"
	merchantErrorCountryNotFound  = "merchant country not found"
	merchantErrorCurrencyNotFound = "merchant bank accounting currency not found"
	merchantErrorUnknown          = "request processing failed. try request later"
)

func (s *Service) GetMerchantById(ctx context.Context, req *grpc.FindByIdRequest, rsp *billing.Merchant) error {
	return nil
}

func (s *Service) ListMerchants(ctx context.Context, req *grpc.CountParams, rsp *grpc.Merchants) error {
	return nil
}

func (s *Service) ChangeMerchant(ctx context.Context, req *grpc.OnboardingRequest, rsp *billing.Merchant) (err error) {
	var merchant *billing.Merchant

	query := bson.M{"external_id": req.ExternalId}
	err = s.db.Collection(pkg.CollectionMerchant).Find(query).One(&merchant)

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Query to find merchant failed", []interface{}{"err", err.Error(), "query", query})
		return errors.New(merchantErrorUnknown)
	}

	isNew := false

	if merchant == nil {
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

	return
}
