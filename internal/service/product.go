package service

import (
	"context"
	"errors"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"gopkg.in/go-playground/validator.v9"
)

func (s *Service) CreateOrUpdateProduct(ctx context.Context, req *grpc.Product, res *grpc.Product) error {
	var (
		err     error
		product grpc.Product
		isNew   bool                 = req.Id == ""
		now     *timestamp.Timestamp = ptypes.TimestampNow()
	)

	if isNew {
		req.Id = bson.NewObjectId().Hex()
		req.CreatedAt = now
	} else {
		err = s.GetProduct(ctx, &grpc.RequestProductById{Id: req.Id}, &product)
		if err != nil {
			s.logError("Product that requested to change is not found", []interface{}{"err", err.Error(), "data", req})
			return err
		}
		req.CreatedAt = product.CreatedAt
	}
	req.UpdatedAt = now
	req.Deleted = false

	validate := validator.New()
	err = validate.Struct(req)
	if err != nil {
		s.logError("Request is invalid", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	if !pricesContainsDefaultCurrency(req.Prices, req.DefaultCurrency) {
		s.logError("No price in default currency", []interface{}{"data", req})
		return errors.New("No price in default currency")
	}

	if isNew {
		err = s.db.Collection(pkg.CollectionProduct).Insert(req)
	} else {
		err = s.db.Collection(pkg.CollectionProduct).UpdateId(req.Id, req)
	}

	if err != nil {
		s.logError("Query to create/update product failed", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	*res = *req

	return nil
}

func (s *Service) ListProducts(ctx context.Context, req *grpc.ListProductsRequest, res *grpc.ListProductsResponse) error {

	if req.Limit == 0 {
		s.logError("Count is required param and must be gt 0", []interface{}{"data", req})
		return errors.New("Count is required param and must be gt 0")
	}

	query := bson.M{"deleted": false}

	if req.Sku != "" {
		query["sku"] = bson.RegEx{req.Sku, "i"}
	}
	if req.Name != "" {
		query["name"] = bson.RegEx{req.Name, "i"}
	}

	total, err := s.db.Collection(pkg.CollectionProduct).Find(query).Count()
	if err != nil {
		s.logError("Query to find refund by id failed", []interface{}{"err", err.Error(), "req", req})
		return err
	}

	res.Limit = req.Limit
	res.Offset = req.Offset
	res.Total = int32(total)
	res.Products = []*grpc.Product{}

	if res.Offset > res.Total {
		return nil
	}

	items := []*grpc.Product{}

	err = s.db.Collection(pkg.CollectionProduct).Find(query).Skip(int(req.Offset)).Limit(int(req.Limit)).All(&items)

	if err != nil {
		s.logError("Query to find refund by id failed", []interface{}{"err", err.Error(), "req", req})
		return err
	}

	res.Products = items
	return nil
}

func (s *Service) GetProduct(ctx context.Context, req *grpc.RequestProductById, res *grpc.Product) error {

	if req.Id == "" {
		s.logError("Id is required param", []interface{}{"data", req})
		return errors.New("Id is required param")
	}

	query := bson.M{"_id": req.Id, "deleted": false}
	err := s.db.Collection(pkg.CollectionProduct).Find(query).One(&res)

	if err != nil {
		s.logError("Query to find refund by id failed", []interface{}{"err", err.Error(), "query", query})
		return err
	}

	return nil
}

func (s *Service) DeleteProduct(ctx context.Context, req *grpc.RequestProductById, res *grpc.EmptyResponse) error {

	if req.Id == "" {
		s.logError("Id is required param", []interface{}{"data", req})
		return errors.New("Id is required param")
	}

	var product grpc.Product

	err := s.GetProduct(ctx, &grpc.RequestProductById{Id: req.Id}, &product)
	if err != nil {
		s.logError("Product that requested to delete is not found", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	product.Deleted = true
	product.UpdatedAt = ptypes.TimestampNow()

	err = s.db.Collection(pkg.CollectionProduct).UpdateId(product.Id, product)

	if err != nil {
		s.logError("Query to delete product failed", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	return nil
}

func pricesContainsDefaultCurrency(prices []*grpc.ProductPrice, defaultCurrency string) bool {
	for _, price := range prices {
		if price.Currency == defaultCurrency {
			return true
		}
	}
	return false
}
