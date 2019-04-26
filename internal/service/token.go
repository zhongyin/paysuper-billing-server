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
	"net"
)

const (
	customerNotFound                 = "customer by specified data not found"
	tokenErrorUserIdentityRequired   = "request must contain one or more parameters with user information"
	tokenErrorSettingsItemsRequired  = "field settings.items required and can't be empty"
	tokenErrorSettingsAmountRequired = "field settings.amount required and must be greater than 0"
)

var (
	customerErrNotFound = errors.New(customerNotFound)
)

func (s *Service) CreateToken(
	ctx context.Context,
	req *grpc.TokenRequest,
	rsp *grpc.TokenResponse,
) error {
	identityExist := req.User.Id != "" || (req.User.Email != nil && req.User.Email.Value != "") ||
		(req.User.Phone != nil && req.User.Phone.Value != "")

	if identityExist == false {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = tokenErrorUserIdentityRequired

		return nil
	}

	project, ok := s.projectCache[req.Settings.ProjectId]

	if !ok {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = projectErrorNotFound

		return nil
	}

	var err error

	if project.IsProductsCheckout == true {
		if len(req.Settings.Items) <= 0 {
			rsp.Status = pkg.ResponseStatusBadData
			rsp.Message = tokenErrorSettingsItemsRequired

			return nil
		}

		req.Settings.ProductsIds, err = s.processTokenProducts(req)

		if err != nil {
			rsp.Status = pkg.ResponseStatusBadData
			rsp.Message = err.Error()

			return nil
		}
	} else {
		if req.Settings.Amount <= 0 {
			rsp.Status = pkg.ResponseStatusBadData
			rsp.Message = tokenErrorSettingsAmountRequired

			return nil
		}
	}

	customer, err := s.getCustomer(req, project)

	if err != nil && err != customerErrNotFound {
		rsp.Status = pkg.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	if customer == nil {
		customer, err = s.createCustomer(req, project)
	} else {
		customer, err = s.updateCustomer(req, project, customer)
	}

	if err != nil {
		rsp.Status = pkg.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	token, err := s.createToken(req, customer)

	if err != nil {
		rsp.Status = pkg.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = token

	return nil
}

func (s *Service) createToken(
	req *grpc.TokenRequest,
	customer *billing.Customer,
) (*billing.Token, error) {
	token := &billing.Token{
		Id:         bson.NewObjectId().Hex(),
		CustomerId: customer.Id,
		User:       req.User,
		Settings:   req.Settings,
		CreatedAt:  ptypes.TimestampNow(),
		UpdatedAt:  ptypes.TimestampNow(),
	}

	b, _ := json.Marshal(token)

	hash := sha256.New()
	hash.Write(b)
	token.Token = hex.EncodeToString(hash.Sum(nil))

	err := s.db.Collection(pkg.CollectionCustomerToken).Insert(token)

	if err != nil {
		s.logError("Query to create token failed", []interface{}{"error", err.Error(), "data", token})
		return nil, errors.New(orderErrorUnknown)
	}

	return token, nil
}

func (s *Service) getCustomer(
	req *grpc.TokenRequest,
	project *billing.Project,
) (*billing.Customer, error) {
	var subQuery []bson.M
	var subQueryItem bson.M

	if req.User.Id != "" {
		subQueryItem = bson.M{
			"identity": bson.M{
				"$elemMatch": bson.M{
					"type":        pkg.UserIdentityTypeExternal,
					"merchant_id": bson.ObjectIdHex(project.MerchantId),
					"value":       req.User.Id,
				},
			},
		}

		subQuery = append(subQuery, subQueryItem)
	}

	if req.User.Email != nil && req.User.Email.Value != "" {
		subQueryItem = bson.M{
			"identity": bson.M{
				"$elemMatch": bson.M{
					"type":        pkg.UserIdentityTypeEmail,
					"merchant_id": bson.ObjectIdHex(project.MerchantId),
					"value":       req.User.Email.Value,
				},
			},
		}

		subQuery = append(subQuery, subQueryItem)
	}

	if req.User.Phone != nil && req.User.Phone.Value != "" {
		subQueryItem = bson.M{
			"identity": bson.M{
				"$elemMatch": bson.M{
					"type":        pkg.UserIdentityTypePhone,
					"merchant_id": bson.ObjectIdHex(project.MerchantId),
					"value":       req.User.Phone.Value,
				},
			},
		}

		subQuery = append(subQuery, subQueryItem)
	}

	query := make(bson.M)
	customer := new(billing.Customer)

	if len(subQuery) > 1 {
		query["$or"] = subQuery
	} else {
		query = subQuery[0]
	}

	err := s.db.Collection(pkg.CollectionCustomer).Find(query).One(&customer)

	if err != nil {
		if err != mgo.ErrNotFound {
			s.logError("Query to find customer failed", []interface{}{"err", err.Error(), "query", query})
			return nil, errors.New(orderErrorUnknown)
		}

		return nil, customerErrNotFound
	}

	return customer, nil
}

func (s *Service) createCustomer(
	req *grpc.TokenRequest,
	project *billing.Project,
) (*billing.Customer, error) {
	id := bson.NewObjectId().Hex()

	customer := &billing.Customer{
		Id:        id,
		TechEmail: id + pkg.TechEmailDomain,
		Metadata:  req.User.Metadata,
		CreatedAt: ptypes.TimestampNow(),
		UpdatedAt: ptypes.TimestampNow(),
	}
	s.processCustomer(req, project, customer)

	err := s.db.Collection(pkg.CollectionCustomer).Insert(customer)

	if err != nil {
		s.logError("Query to create new customer failed", []interface{}{"error", err.Error(), "data", customer})
		return nil, errors.New(orderErrorUnknown)
	}

	return customer, nil
}

func (s *Service) updateCustomer(
	req *grpc.TokenRequest,
	project *billing.Project,
	customer *billing.Customer,
) (*billing.Customer, error) {
	s.processCustomer(req, project, customer)
	err := s.db.Collection(pkg.CollectionCustomer).UpdateId(bson.ObjectIdHex(customer.Id), customer)

	if err != nil {
		s.logError("Query to update customer data failed", []interface{}{"error", err.Error(), "data", customer})
		return nil, errors.New(orderErrorUnknown)
	}

	return customer, nil
}

func (s *Service) processCustomer(
	req *grpc.TokenRequest,
	project *billing.Project,
	customer *billing.Customer,
) {
	user := req.User

	if user.Id != "" && user.Id != customer.ExternalId {
		customer.ExternalId = user.Id
		identity := &billing.CustomerIdentity{
			MerchantId: project.MerchantId,
			ProjectId:  project.Id,
			Type:       pkg.UserIdentityTypeExternal,
			Value:      user.Id,
			Verified:   true,
			CreatedAt:  ptypes.TimestampNow(),
		}

		customer.Identity = s.processCustomerIdentity(customer.Identity, identity)
	}

	if user.Email != nil && customer.Email != user.Email.Value {
		customer.Email = user.Email.Value
		customer.EmailVerified = user.Email.Verified
		identity := &billing.CustomerIdentity{
			MerchantId: project.MerchantId,
			ProjectId:  project.Id,
			Type:       pkg.UserIdentityTypeEmail,
			Value:      user.Email.Value,
			Verified:   user.Email.Verified,
			CreatedAt:  ptypes.TimestampNow(),
		}

		customer.Identity = s.processCustomerIdentity(customer.Identity, identity)
	}

	if user.Phone != nil && customer.Phone != user.Phone.Value {
		customer.Phone = user.Phone.Value
		customer.PhoneVerified = user.Phone.Verified
		identity := &billing.CustomerIdentity{
			MerchantId: project.MerchantId,
			ProjectId:  project.Id,
			Type:       pkg.UserIdentityTypePhone,
			Value:      user.Phone.Value,
			Verified:   user.Phone.Verified,
			CreatedAt:  ptypes.TimestampNow(),
		}

		customer.Identity = s.processCustomerIdentity(customer.Identity, identity)
	}

	if user.Name != nil && customer.Name != user.Name.Value {
		customer.Name = user.Name.Value
	}

	if user.Ip != nil {
		ip := net.IP(customer.Ip)

		if ip.String() != user.Ip.Value {
			history := &billing.CustomerIpHistory{
				Ip:        customer.Ip,
				CreatedAt: ptypes.TimestampNow(),
			}
			customer.Ip = net.ParseIP(user.Ip.Value)

			customer.IpHistory = append(customer.IpHistory, history)
		}
	}

	if user.Locale != nil && customer.Locale != user.Locale.Value {
		history := &billing.CustomerStringValueHistory{
			Value:     customer.Locale,
			CreatedAt: ptypes.TimestampNow(),
		}
		customer.Locale = user.Locale.Value

		customer.LocaleHistory = append(customer.LocaleHistory, history)
	}

	if user.Address != nil && customer.Address != user.Address {
		history := &billing.CustomerAddressHistory{
			Country:    customer.Address.Country,
			City:       customer.Address.City,
			PostalCode: customer.Address.PostalCode,
			State:      customer.Address.State,
			CreatedAt:  ptypes.TimestampNow(),
		}
		customer.Address = user.Address

		customer.AddressHistory = append(customer.AddressHistory, history)
	}
}

func (s *Service) processCustomerIdentity(
	currentIdentities []*billing.CustomerIdentity,
	newIdentity *billing.CustomerIdentity,
) []*billing.CustomerIdentity {
	if len(currentIdentities) <= 0 {
		return append(currentIdentities, newIdentity)
	}

	isNewIdentity := true

	for _, v := range currentIdentities {
		needChange := v.Type == newIdentity.Type && v.ProjectId == newIdentity.ProjectId &&
			v.MerchantId == newIdentity.MerchantId && v.Value == newIdentity.Value

		if needChange == false {
			continue
		}

		v = newIdentity
		isNewIdentity = false
	}

	if isNewIdentity == true {
		currentIdentities = append(currentIdentities, newIdentity)
	}

	return currentIdentities
}
