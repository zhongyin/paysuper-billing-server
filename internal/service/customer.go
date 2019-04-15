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
	customerFieldId         = "Id"
	customerFieldToken      = "Token"
	customerFieldProjectId  = "ProjectId"
	customerFieldMerchantId = "MerchantId"
	customerFieldMetadata   = "Metadata"
	customerFieldExpireAt   = "ExpireAt"
	customerFieldCreatedAt  = "CreatedAt"
	customerFieldUpdatedAt  = "UpdatedAt"

	customerErrorNotFound = "customer with specified data not found"
)

var (
	ErrCustomerNotFound = errors.New(customerErrorNotFound)

	customerHistoryExcludedFields = map[string]bool{
		customerFieldId:         true,
		customerFieldToken:      true,
		customerFieldProjectId:  true,
		customerFieldMerchantId: true,
		customerFieldMetadata:   true,
		customerFieldExpireAt:   true,
		customerFieldCreatedAt:  true,
		customerFieldUpdatedAt:  true,
	}
)

func (s *Service) ChangeCustomer(
	ctx context.Context,
	req *billing.Customer,
	rsp *grpc.ChangeCustomerResponse,
) error {
	processor := &OrderCreateRequestProcessor{
		Service: s,
		request: &billing.OrderCreateRequest{
			ProjectId: req.ProjectId,
		},
		checked: &orderCreateRequestProcessorChecked{},
	}

	err := processor.processProject()

	if err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = err.Error()

		return nil
	}

	customer, err := s.changeCustomer(req, processor.checked.project.Merchant.Id)

	if err != nil {
		rsp.Status = pkg.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = customer

	return nil
}

func (s *Service) getCustomerBy(query bson.M) (customer *billing.Customer, err error) {
	err = s.db.Collection(pkg.CollectionCustomer).Find(query).One(&customer)

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Query to find customer failed", []interface{}{"err", err.Error(), "query", query})
		return customer, errors.New(orderErrorUnknown)
	}

	if customer == nil {
		return customer, ErrCustomerNotFound
	}

	return
}

func (s *Service) changeCustomer(req *billing.Customer, merchantId string) (*billing.Customer, error) {
	var customer *billing.Customer
	var isNew bool
	var err error

	if req.IsEmptyRequest() == false {
		query := bson.M{"project_id": bson.ObjectIdHex(req.ProjectId)}

		if req.Token != "" {
			query["token"] = req.Token
		} else {
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
			}
		}

		err = s.db.Collection(pkg.CollectionCustomer).Find(query).One(&customer)

		if err != nil && err != mgo.ErrNotFound {
			s.logError("Query to find customer failed", []interface{}{"error", err.Error(), "query", query})
			return nil, errors.New(orderErrorUnknown)
		}
	}

	if customer == nil {
		isNew = true
		customer = &billing.Customer{
			Id:            bson.NewObjectId().Hex(),
			ProjectId:     req.ProjectId,
			MerchantId:    merchantId,
			ExternalId:    req.ExternalId,
			Name:          req.Name,
			Email:         req.Email,
			EmailVerified: req.EmailVerified,
			Phone:         req.Phone,
			PhoneVerified: req.PhoneVerified,
			Ip:            req.Ip,
			Locale:        req.Locale,
			Address:       req.Address,
			Metadata:      req.Metadata,
			CreatedAt:     ptypes.TimestampNow(),
		}
	} else {
		changes := s.getCustomerChanges(req, customer)

		if len(changes) > 0 {
			err = s.saveCustomerHistory(customer.Id, changes)

			if err != nil {
				return nil, errors.New(orderErrorUnknown)
			}
		}
	}

	if customer.IsTokenExpired() == true {
		s.customerTokenUpdate(customer)
	}

	if isNew == true {
		err = s.db.Collection(pkg.CollectionCustomer).Insert(customer)
	} else {
		err = s.db.Collection(pkg.CollectionCustomer).UpdateId(bson.ObjectIdHex(customer.Id), customer)
	}

	if err != nil {
		s.logError("Query to save customer data failed", []interface{}{"error", err.Error(), "data", customer})
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
	newDataMap := pkg.NewStructureConverter(newData).Map()
	oldDataMap := pkg.NewStructureConverter(oldData).Map()

	changes := make(map[string]interface{})

	for k, v := range newDataMap {
		if vv, ok := customerHistoryExcludedFields[k]; ok && vv == true {
			continue
		}

		vv, _ := oldDataMap[k]

		if vv == v {
			continue
		}

		changes[k] = vv
	}

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
