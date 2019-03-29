package service

import (
	"context"
	"errors"
	"github.com/globalsign/mgo"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"gopkg.in/mgo.v2/bson"
)

func (s *Service) ChangeCustomer(ctx context.Context, req *billing.Customer, rsp *billing.Customer) error {
	return nil
}

func (s *Service) changeCustomer(req *billing.Customer) (*billing.Customer, error) {
	var customer *billing.Customer

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

		err := s.db.Collection(pkg.CollectionCustomer).Find(query).One(&customer)

		if err != nil {
			if err != mgo.ErrNotFound {
				s.logError("Query to find customer failed", []interface{}{"error", err.Error(), "query", query})

				return nil, errors.New(orderErrorUnknown)
			}
		}
	}

	return &billing.Customer{}, nil
}
