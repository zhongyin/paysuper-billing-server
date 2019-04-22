package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	customerFieldExternalId     = "ExternalId"
	customerFieldName           = "Name"
	customerFieldEmail          = "Email"
	customerFieldEmailVerified  = "EmailVerified"
	customerFieldPhone          = "Phone"
	customerFieldPhoneVerified  = "PhoneVerified"
	customerFieldIp             = "Ip"
	customerFieldLocale         = "Locale"
	customerFieldAddress        = "Address"
	customerFieldAcceptLanguage = "AcceptLanguage"
	customerFieldUserAgent      = "UserAgent"

	customerErrorNotFound = "customer with specified data not found"
)

var (
	ErrCustomerNotFound        = errors.New(customerErrorNotFound)
	ErrCustomerProjectNotFound = errors.New(orderErrorProjectNotFound)
	ErrCustomerGeoIncorrect    = errors.New(orderErrorPayerRegionUnknown)
)

func (s *Service) ChangeCustomer(
	ctx context.Context,
	req *billing.Customer,
	rsp *grpc.ChangeCustomerResponse,
) error {
	customer, err := s.changeCustomer(req)

	if err != nil {
		rsp.Status = pkg.ResponseStatusSystemError

		if err == ErrCustomerProjectNotFound || err == ErrCustomerGeoIncorrect {
			rsp.Status = pkg.ResponseStatusBadData
		}

		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = customer

	return nil
}

func (s *Service) getCustomerBy(query bson.M) (customer *billing.Customer, err error) {
	err = s.db.Collection(pkg.CollectionCustomer).Find(query).One(&customer)

	if err != nil {
		if err == mgo.ErrNotFound {
			return customer, ErrCustomerNotFound
		}

		s.logError("Query to find customer failed", []interface{}{"err", err.Error(), "query", query})
		return customer, errors.New(orderErrorUnknown)
	}

	return
}

func (s *Service) changeCustomer(req *billing.Customer) (*billing.Customer, error) {
	var customer *billing.Customer
	var err error

	if req.IsEmptyRequest() == false {
		query := bson.M{"project_id": bson.ObjectIdHex(req.ProjectId)}

		if req.ExternalId != "" || req.Email != "" || req.Phone != "" {
			var subQuery []bson.M

			if req.ExternalId != "" {
				subQuery = append(subQuery, bson.M{"external_id": req.ExternalId})
			}

			if req.Email != "" {
				subQuery = append(subQuery, bson.M{"email": req.Email})
			}

			if req.Phone != "" {
				subQuery = append(subQuery, bson.M{"phone": req.Phone})
			}

			query["$or"] = subQuery
		} else {
			if req.Token != "" {
				query["token"] = req.Token
			}
		}

		err = s.db.Collection(pkg.CollectionCustomer).Find(query).One(&customer)

		if err != nil && err != mgo.ErrNotFound {
			s.logError("Query to find customer failed", []interface{}{"error", err.Error(), "query", query})
			return nil, errors.New(orderErrorUnknown)
		}
	}

	if customer == nil {
		customer, err = s.createCustomer(req)
	} else {
		customer, err = s.updateCustomer(req, customer)
	}

	if err != nil {
		return nil, err
	}

	return customer, nil
}

func (s *Service) createCustomer(req *billing.Customer) (*billing.Customer, error) {
	customer := &billing.Customer{
		Id:             bson.NewObjectId().Hex(),
		ProjectId:      req.ProjectId,
		ExternalId:     req.ExternalId,
		Name:           req.Name,
		Email:          req.Email,
		EmailVerified:  req.EmailVerified,
		Phone:          req.Phone,
		PhoneVerified:  req.PhoneVerified,
		Ip:             req.Ip,
		Locale:         req.Locale,
		Address:        req.Address,
		AcceptLanguage: req.AcceptLanguage,
		UserAgent:      req.UserAgent,
		Metadata:       req.Metadata,
		CreatedAt:      ptypes.TimestampNow(),
	}

	processor := &OrderCreateRequestProcessor{
		Service: s,
		request: &billing.OrderCreateRequest{
			ProjectId: req.ProjectId,
			User:      customer,
		},
		checked: &orderCreateRequestProcessorChecked{},
	}

	if req.MerchantId == "" {
		if err := processor.processProject(); err != nil {
			return nil, ErrCustomerProjectNotFound
		}

		customer.MerchantId = processor.checked.project.MerchantId
	} else {
		customer.MerchantId = req.MerchantId
	}

	if customer.Address == nil && customer.Ip != "" {
		err := processor.processPayerData()

		if err != nil {
			return nil, ErrCustomerGeoIncorrect
		}

		customer.Address = processor.getBillingAddress()
	}

	s.customerTokenUpdate(customer)

	err := s.db.Collection(pkg.CollectionCustomer).Insert(customer)

	if err != nil {
		s.logError("Query to create new customer failed", []interface{}{"error", err.Error(), "data", customer})
		return nil, errors.New(orderErrorUnknown)
	}

	return customer, nil
}

func (s *Service) updateCustomer(req, customer *billing.Customer) (*billing.Customer, error) {
	changes := s.getCustomerChanges(req, customer)

	if len(changes) > 0 {
		err := s.saveCustomerHistory(customer.Id, changes)

		if err != nil {
			return nil, errors.New(orderErrorUnknown)
		}
	}

	if customer.IsTokenExpired() == true {
		s.customerTokenUpdate(customer)
	}

	err := s.db.Collection(pkg.CollectionCustomer).UpdateId(bson.ObjectIdHex(customer.Id), customer)

	if err != nil {
		s.logError("Query to update customer data failed", []interface{}{"error", err.Error(), "data", customer})
		return nil, errors.New(orderErrorUnknown)
	}

	return customer, nil
}

func (s *Service) customerTokenUpdate(c *billing.Customer) {
	c.UpdatedAt = ptypes.TimestampNow()
	c.ExpireAt, _ = ptypes.TimestampProto(time.Now().Add(time.Second * pkg.DefaultCustomerTokenLifetime))

	b, _ := json.Marshal(c)

	hash := sha256.New()
	hash.Write(b)
	c.Token = hex.EncodeToString(hash.Sum(nil))

	return
}

func (s *Service) getCustomerChanges(newData, oldData *billing.Customer) map[string]interface{} {
	changes := make(map[string]interface{})

	if newData.ExternalId != oldData.ExternalId {
		changes[customerFieldExternalId] = oldData.ExternalId
		oldData.ExternalId = newData.ExternalId
	}

	if newData.Name != oldData.Name {
		changes[customerFieldName] = oldData.Name
		oldData.Name = newData.Name
	}

	if newData.Email != oldData.Email {
		changes[customerFieldEmail] = oldData.Email
		oldData.Email = newData.Email
	}

	if newData.EmailVerified != oldData.EmailVerified {
		changes[customerFieldEmailVerified] = oldData.EmailVerified
		oldData.EmailVerified = newData.EmailVerified
	}

	if newData.Phone != oldData.Phone {
		changes[customerFieldPhone] = oldData.Phone
		oldData.Phone = newData.Phone
	}

	if newData.PhoneVerified != oldData.PhoneVerified {
		changes[customerFieldPhoneVerified] = oldData.PhoneVerified
		oldData.PhoneVerified = newData.PhoneVerified
	}

	if newData.Ip != oldData.Ip {
		changes[customerFieldIp] = oldData.Ip
		oldData.Ip = newData.Ip
	}

	if newData.Address != oldData.Address {
		changes[customerFieldAddress] = oldData.Address
		oldData.Address = newData.Address
	}

	if newData.Locale != oldData.Locale {
		changes[customerFieldLocale] = oldData.Locale
		oldData.Locale = newData.Locale
	}

	if newData.AcceptLanguage != oldData.AcceptLanguage {
		changes[customerFieldAcceptLanguage] = oldData.AcceptLanguage
		oldData.AcceptLanguage = newData.AcceptLanguage

		if newData.Locale == oldData.Locale {
			changes[customerFieldLocale] = oldData.Locale
			oldData.Locale, _ = s.getCountryFromAcceptLanguage(oldData.AcceptLanguage)
		}
	}

	if newData.UserAgent != oldData.UserAgent {
		changes[customerFieldUserAgent] = oldData.UserAgent
		oldData.UserAgent = newData.UserAgent
	}

	oldData.Metadata = newData.Metadata

	return changes
}

func (s *Service) saveCustomerHistory(customerId string, changes map[string]interface{}) error {
	customerHistory := &billing.MgoCustomerHistory{
		Id:         bson.NewObjectId(),
		CustomerId: bson.ObjectIdHex(customerId),
		Changes:    changes,
		CreatedAt:  time.Now(),
	}

	err := s.db.Collection(pkg.CollectionCustomerHistory).Insert(customerHistory)

	if err != nil {
		s.logError("Query to insert customer history failed", []interface{}{"error", err.Error(), "data", customerHistory})
		return errors.New(orderErrorUnknown)
	}

	return nil
}

func (s *Service) changeCustomerPaymentFormData(
	customer *billing.Customer,
	ip, acceptLanguage, userAgent, email string,
	address *billing.OrderBillingAddress,
) (*billing.Customer, error) {
	isHeaderDataMatch := customer.Ip == ip && customer.AcceptLanguage == acceptLanguage && customer.UserAgent == userAgent
	isUserIdentityMatch := (email == "" || customer.Email == email) && (address == nil || customer.Address == address)

	if isHeaderDataMatch == true && isUserIdentityMatch {
		return customer, nil
	}

	if email != "" && customer.Email != email {
		customer.Email = email
	}

	if ip != "" && customer.Ip != ip {
		processor := &OrderCreateRequestProcessor{
			Service: s,
			request: &billing.OrderCreateRequest{
				User: &billing.Customer{Ip: ip},
			},
			checked: &orderCreateRequestProcessorChecked{},
		}

		err := processor.processPayerData()

		if err != nil {
			return nil, err
		}

		customer.Ip = ip
		customer.Address = &billing.OrderBillingAddress{
			Country:    processor.checked.payerData.Country,
			City:       processor.checked.payerData.City.En,
			PostalCode: processor.checked.payerData.Zip,
			State:      processor.checked.payerData.State,
		}
	}

	if address != nil && customer.Address != address {
		customer.Address = address
	}

	if acceptLanguage != "" && customer.AcceptLanguage != acceptLanguage {
		customer.AcceptLanguage = acceptLanguage
		customer.Locale, _ = s.getCountryFromAcceptLanguage(acceptLanguage)
	}

	if userAgent != "" && customer.UserAgent != userAgent {
		customer.UserAgent = userAgent
	}

	customer, err := s.changeCustomer(customer)

	if err != nil {
		return nil, err
	}

	return customer, nil
}
