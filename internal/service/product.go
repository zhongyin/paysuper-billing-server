package service

import (
	"context"
	"errors"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"gopkg.in/mgo.v2"
)

const (
	productErrorNotFound         = "products with specified SKUs not found"
	productErrorCountNotMatch    = "request products count and products in system count not match"
	productErrorAmountNotMatch   = "one or more products amount not match"
	productErrorCurrencyNotMatch = "one or more products currency not match"
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

		if req.MerchantId != product.MerchantId {
			s.logError("MerchantId mismatch", []interface{}{"data", req})
			return errors.New("merchantId mismatch")
		}

		if req.ProjectId != product.ProjectId {
			s.logError("ProjectId mismatch", []interface{}{"data", req})
			return errors.New("projectId mismatch")
		}

		req.CreatedAt = product.CreatedAt
	}
	req.UpdatedAt = now
	req.Deleted = false

	if !req.IsPricesContainDefaultCurrency() {
		s.logError("No price in default currency", []interface{}{"data", req})
		return errors.New("no price in default currency")
	}

	if _, err := req.GetLocalizedName(DefaultLanguage); err != nil {
		s.logError("No name in default language", []interface{}{"data", req})
		return err
	}

	if _, err := req.GetLocalizedDescription(DefaultLanguage); err != nil {
		s.logError("No description in default language", []interface{}{"data", req})
		return err
	}

	// Prevent duplicated products (by projectId+sku)
	dupQuery := bson.M{"project_id": bson.ObjectIdHex(req.ProjectId), "sku": req.Sku, "deleted": false}
	found, err := s.db.Collection(pkg.CollectionProduct).Find(dupQuery).Count()
	if err != nil {
		s.logError("Query to find duplicates failed", []interface{}{"err", err.Error(), "req", req})
		return err
	}
	allowed := 1
	if isNew {
		allowed = 0
	}
	if found > allowed {
		s.logError("Pair projectId+Sku already exists", []interface{}{"data", req})
		return errors.New("pair projectId+Sku already exists")
	}

	_, err = s.db.Collection(pkg.CollectionProduct).UpsertId(bson.ObjectIdHex(req.Id), req)

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
	res.ProjectId = req.ProjectId

	return nil
}

func (s *Service) GetProductsForOrder(ctx context.Context, req *grpc.GetProductsForOrderRequest, res *grpc.ListProductsResponse) error {
	if len(req.Ids) == 0 {
		s.logError("Ids list is empty", []interface{}{"data", req})
		return errors.New("ids list is empty")
	}
	query := bson.M{"enabled": true, "deleted": false, "project_id": bson.ObjectIdHex(req.ProjectId)}
	var items = []bson.ObjectId{}
	for _, id := range req.Ids {
		items = append(items, bson.ObjectIdHex(id))
	}
	query["_id"] = bson.M{"$in": items}

	found := []*grpc.Product{}

	err := s.db.Collection(pkg.CollectionProduct).Find(query).All(&found)

	if err != nil {
		s.logError("Query to find refund by id failed", []interface{}{"err", err.Error(), "req", req})
		return err
	}

	res.Limit = int32(len(found))
	res.Offset = 0
	res.Total = res.Limit
	res.Products = found
	return nil
}

func (s *Service) ListProducts(ctx context.Context, req *grpc.ListProductsRequest, res *grpc.ListProductsResponse) error {

	query := bson.M{"merchant_id": bson.ObjectIdHex(req.MerchantId), "deleted": false}

	if req.ProjectId != "" {
		query["project_id"] = bson.ObjectIdHex(req.ProjectId)
	}

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

	if res.Total == 0 || res.Offset > res.Total {
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

func (s *Service) getProductsCountByProject(projectId string) int32 {
	query := bson.M{"project_id": bson.ObjectIdHex(projectId), "deleted": false}
	count, err := s.db.Collection(pkg.CollectionProduct).Find(query).Count()

	if err != nil {
		s.logError("Query to get project products count failed", []interface{}{"err", err.Error(), "query", query})
	}

	return int32(count)
}

func (s *Service) processTokenProducts(req *grpc.TokenRequest) ([]string, error) {
	var sku []string
	var products []*grpc.Product
	skuItemsMap := make(map[string]*billing.TokenSettingsItem, len(req.Settings.Items))

	for _, v := range req.Settings.Items {
		sku = append(sku, v.Sku)
		skuItemsMap[v.Sku] = v
	}

	query := bson.M{
		"project_id": bson.ObjectIdHex(req.Settings.ProjectId),
		"sku":        bson.M{"$in": sku},
		"deleted":    false,
	}
	err := s.db.Collection(pkg.CollectionProduct).Find(query).All(&products)

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Query to find project products failed", []interface{}{"err", err.Error(), "query", query})
		return nil, errors.New(orderErrorUnknown)
	}

	if len(products) <= 0 {
		return nil, errors.New(productErrorNotFound)
	}

	if len(sku) != len(products) {
		return nil, errors.New(productErrorCountNotMatch)
	}

	var productsIds []string

	for _, v := range products {
		item, _ := skuItemsMap[v.Sku]

		matchAmount := false
		matchCurrency := false

		for _, v1 := range v.Prices {
			if item.Amount != v1.Amount && item.Currency != v1.Currency {
				continue
			}

			if item.Amount == v1.Amount {
				matchAmount = true
			}

			if item.Currency == v1.Currency {
				matchCurrency = true
			}
		}

		if matchAmount == false {
			return nil, errors.New(productErrorAmountNotMatch)
		}

		if matchCurrency == false && v.DefaultCurrency != item.Currency {
			return nil, errors.New(productErrorCurrencyNotMatch)
		}

		productsIds = append(productsIds, v.Id)
	}

	return productsIds, nil
}
