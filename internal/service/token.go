package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"math/rand"
	"net"
	"strings"
	"time"
)

const (
	customerNotFound                 = "customer by specified data not found"
	tokenErrorNotFound               = "token not found"
	tokenErrorUserIdentityRequired   = "request must contain one or more parameters with user information"
	tokenErrorSettingsItemsRequired  = "field settings.items required and can't be empty"
	tokenErrorSettingsAmountRequired = "field settings.amount required and must be greater than 0"

	tokenStorageMask   = "paysuper:token:%s"
	tokenLetterBytes   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	tokenLetterIdxBits = uint(6)
	tokenLetterIdxMask = uint64(1<<tokenLetterIdxBits - 1)
	tokenLetterIdxMax  = 63 / tokenLetterIdxBits
)

var (
	customerErrNotFound = errors.New(customerNotFound)

	tokenRandSource = rand.NewSource(time.Now().UnixNano())
)

type Token struct {
	CustomerId string                 `json:"customer_id"`
	User       *billing.TokenUser     `json:"user"`
	Settings   *billing.TokenSettings `json:"settings"`
}

type tokenRepository struct {
	token   *Token
	service *Service
}

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

	customer, err := s.findCustomer(req, project)

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
	rsp.Token = token

	return nil
}

func (s *Service) createToken(req *grpc.TokenRequest, customer *billing.Customer) (string, error) {
	tokenRep := &tokenRepository{
		service: s,
		token: &Token{
			CustomerId: customer.Id,
			User:       req.User,
			Settings:   req.Settings,
		},
	}
	token := tokenRep.service.getTokenString(s.cfg.GetCustomerTokenLength())
	err := tokenRep.setToken(token)

	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Service) getTokenBy(token string) (*Token, error) {
	tokenRep := &tokenRepository{
		service: s,
		token:   &Token{},
	}
	err := tokenRep.getToken(token)

	if err != nil {
		return nil, err
	}

	return tokenRep.token, nil
}

func (s *Service) getCustomerById(id string) (*billing.Customer, error) {
	var customer *billing.Customer
	err := s.db.Collection(pkg.CollectionCustomer).FindId(bson.ObjectIdHex(id)).One(&customer)

	if err != nil {
		if err != mgo.ErrNotFound {
			return nil, errors.New(orderErrorUnknown)
		}

		return nil, errors.New(customerNotFound)
	}

	return customer, nil
}

func (s *Service) findCustomer(
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

	if len(subQuery) <= 0 {
		return nil, customerErrNotFound
	}

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

	if user.Email != nil && (customer.Email != user.Email.Value || customer.EmailVerified != user.Email.Verified) {
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

	if user.Phone != nil && (customer.Phone != user.Phone.Value || customer.PhoneVerified != user.Phone.Verified) {
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
		if customer.Address != nil {
			history := &billing.CustomerAddressHistory{
				Country:    customer.Address.Country,
				City:       customer.Address.City,
				PostalCode: customer.Address.PostalCode,
				State:      customer.Address.State,
				CreatedAt:  ptypes.TimestampNow(),
			}
			customer.AddressHistory = append(customer.AddressHistory, history)
		}

		customer.Address = user.Address
	}

	if user.UserAgent != "" && customer.UserAgent != user.UserAgent {
		customer.UserAgent = user.UserAgent
	}

	if user.AcceptLanguage != "" && customer.AcceptLanguage != user.AcceptLanguage {
		history := &billing.CustomerStringValueHistory{
			Value:     customer.AcceptLanguage,
			CreatedAt: ptypes.TimestampNow(),
		}
		customer.AcceptLanguage = user.AcceptLanguage
		customer.AcceptLanguageHistory = append(customer.AcceptLanguageHistory, history)
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

	for k, v := range currentIdentities {
		needChange := v.Type == newIdentity.Type && v.ProjectId == newIdentity.ProjectId &&
			v.MerchantId == newIdentity.MerchantId && v.Value == newIdentity.Value && v.Verified != newIdentity.Verified

		if needChange == false {
			continue
		}

		currentIdentities[k] = newIdentity
		isNewIdentity = false
	}

	if isNewIdentity == true {
		currentIdentities = append(currentIdentities, newIdentity)
	}

	return currentIdentities
}

func (s *Service) transformOrderUser2TokenRequest(user *billing.OrderUser) *grpc.TokenRequest {
	tokenReq := &grpc.TokenRequest{User: &billing.TokenUser{}}

	if user.ExternalId != "" {
		tokenReq.User.Id = user.ExternalId
	}

	if user.Name != "" {
		tokenReq.User.Name = &billing.TokenUserValue{Value: user.Name}
	}

	if user.Email != "" {
		tokenReq.User.Email = &billing.TokenUserEmailValue{
			Value:    user.Email,
			Verified: user.EmailVerified,
		}
	}

	if user.Phone != "" {
		tokenReq.User.Phone = &billing.TokenUserPhoneValue{
			Value:    user.Phone,
			Verified: user.PhoneVerified,
		}
	}

	if user.Ip != "" {
		tokenReq.User.Ip = &billing.TokenUserIpValue{Value: user.Ip}
	}

	if user.Locale != "" {
		tokenReq.User.Locale = &billing.TokenUserLocaleValue{Value: user.Locale}
	}

	if user.Address != nil {
		tokenReq.User.Address = user.Address
	}

	if len(user.Metadata) > 0 {
		tokenReq.User.Metadata = user.Metadata
	}

	return tokenReq
}

func (r *tokenRepository) getToken(token string) error {
	data, err := r.service.redis.Get(r.getKey(token)).Bytes()

	if err != nil {
		r.service.logError("Get customer token from Redis failed", []interface{}{"error", err.Error()})
		return errors.New(tokenErrorNotFound)
	}

	err = json.Unmarshal(data, &r.token)

	if err != nil {
		r.service.logError("Unmarshal customer token failed", []interface{}{"error", err.Error()})
		return errors.New(tokenErrorNotFound)
	}

	return nil
}

func (r *tokenRepository) setToken(token string) error {
	b, err := json.Marshal(r.token)

	if err != nil {
		r.service.logError("Marshal customer token failed", []interface{}{"error", err.Error()})
		return errors.New(orderErrorUnknown)
	}

	return r.service.redis.Set(r.getKey(token), b, r.service.cfg.GetCustomerTokenExpire()).Err()
}

func (r *tokenRepository) getKey(token string) string {
	return fmt.Sprintf(tokenStorageMask, token)
}

func (s *Service) getTokenString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)

	for i, cache, remain := n-1, tokenRandSource.Int63(), tokenLetterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = tokenRandSource.Int63(), tokenLetterIdxMax
		}

		if idx := int(uint64(cache) & tokenLetterIdxMask); idx < len(tokenLetterBytes) {
			sb.WriteByte(tokenLetterBytes[idx])
			i--
		}

		cache >>= tokenLetterIdxBits
		remain--
	}

	return sb.String()
}

func (s *Service) updateCustomerFromRequest(
	order *billing.Order,
	req *grpc.TokenRequest,
	ip, acceptLanguage, userAgent string,
) error {
	customer, err := s.getCustomerById(order.User.Id)
	project := &billing.Project{Id: order.Project.Id, MerchantId: order.Project.MerchantId}

	if err != nil {
		return err
	}

	req.User.Ip = &billing.TokenUserIpValue{Value: ip}
	req.User.AcceptLanguage = acceptLanguage
	req.User.UserAgent = userAgent

	_, err = s.updateCustomer(req, project, customer)

	return err
}

func (s *Service) updateCustomerFromRequestLocale(
	order *billing.Order,
	ip, acceptLanguage, userAgent, locale string,
) {
	tokenReq := &grpc.TokenRequest{
		User: &billing.TokenUser{
			Locale: &billing.TokenUserLocaleValue{Value: locale},
		},
	}

	err := s.updateCustomerFromRequest(order, tokenReq, ip, acceptLanguage, userAgent)

	if err != nil {
		s.logError("Update customer data by request failed", []interface{}{"error", err})
	}
}

func (s *Service) updateCustomerFromRequestAddress(
	order *billing.Order,
	ip, acceptLanguage, userAgent string,
	address *billing.OrderBillingAddress,
) {
	tokenReq := &grpc.TokenRequest{
		User: &billing.TokenUser{Address: address},
	}

	err := s.updateCustomerFromRequest(order, tokenReq, ip, acceptLanguage, userAgent)

	if err != nil {
		s.logError("Update customer data by request failed", []interface{}{"error", err})
	}
}
