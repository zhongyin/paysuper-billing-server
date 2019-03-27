package service

import (
	"context"
	"errors"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
)

func (s *Service) CreateOrUpdateProduct(ctx context.Context, req *grpc.Product, res *grpc.Product) error {
	var (
		err     error
		product = &grpc.Product{}
		isNew   = req.Id == ""
		now     = ptypes.TimestampNow()
	)

	if isNew {
		req.Id = bson.NewObjectId().Hex()
		req.CreatedAt = now
	} else {
		err = s.GetProduct(ctx, &grpc.RequestProduct{Id: req.Id, MerchantId: req.MerchantId}, product)
		if err != nil {
			s.logError("Product that requested to change is not found", []interface{}{"err", err.Error(), "data", req})
			return err
		}
		req.CreatedAt = product.CreatedAt
	}
	req.UpdatedAt = now
	req.Deleted = false

	if !req.IsPricesContainDefaultCurrency() {
		s.logError("No price in default currency", []interface{}{"data", req})
		return errors.New("no price in default currency")
	}

	if isNew {
		err = s.db.Collection(pkg.CollectionProduct).Insert(req)
	} else {
		err = s.db.Collection(pkg.CollectionProduct).UpdateId(bson.ObjectIdHex(req.Id), req)
	}

	if err != nil {
		s.logError("Query to create/update product failed", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	res.Id = req.Id
	res.Object = req.Object
	res.Type = req.Type
	res.Sku = req.Sku
	res.Name = req.Name
	res.DefaultCurrency = req.DefaultCurrency
	res.Enabled = req.Enabled
	res.Prices = req.Prices
	res.Description = req.Description
	res.LongDescription = req.LongDescription
	res.Images = req.Images
	res.Url = req.Url
	res.Metadata = req.Metadata
	res.CreatedAt = req.CreatedAt
	res.UpdatedAt = req.UpdatedAt
	res.Deleted = req.Deleted
	res.MerchantId = req.MerchantId

	return nil
}

func (s *Service) ListProducts(ctx context.Context, req *grpc.ListProductsRequest, res *grpc.ListProductsResponse) error {

	query := bson.M{"merchant_id": bson.ObjectIdHex(req.MerchantId), "deleted": false}

	if req.Sku != "" {
		query["sku"] = bson.RegEx{req.Sku, "i"}
	}
	if req.Name != "" {
		query["name"] = bson.M{"$elemMatch": bson.M{"value": bson.RegEx{req.Name, "i"}}}
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

func (s *Service) GetProduct(ctx context.Context, req *grpc.RequestProduct, res *grpc.Product) error {

	query := bson.M{
		"_id":         bson.ObjectIdHex(req.Id),
		"merchant_id": bson.ObjectIdHex(req.MerchantId),
		"deleted":     false,
	}
	err := s.db.Collection(pkg.CollectionProduct).Find(query).One(&res)

	if err != nil {
		s.logError("Query to find refund by id failed", []interface{}{"err", err.Error(), "query", query})
		return err
	}

	return nil
}

func (s *Service) DeleteProduct(ctx context.Context, req *grpc.RequestProduct, res *grpc.EmptyResponse) error {

	product := &grpc.Product{}

	err := s.GetProduct(ctx, &grpc.RequestProduct{Id: req.Id, MerchantId: req.MerchantId}, product)
	if err != nil {
		s.logError("Product that requested to delete is not found", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	product.Deleted = true
	product.UpdatedAt = ptypes.TimestampNow()

	err = s.db.Collection(pkg.CollectionProduct).UpdateId(bson.ObjectIdHex(product.Id), product)

	if err != nil {
		s.logError("Query to delete product failed", []interface{}{"err", err.Error(), "data", req})
		return err
	}

	return nil
}
