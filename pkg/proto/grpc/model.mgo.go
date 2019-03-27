package grpc

import (
	"errors"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"time"
)

const (
	errorInvalidObjectId = "invalid bson object id"
)

type MgoProduct struct {
	Id              bson.ObjectId         `bson:"_id" json:"id"`
	Object          string                `bson:"object" json:"object"`
	Type            string                `bson:"type" json:"type"`
	Sku             string                `bson:"sku" json:"sku"`
	Name            []*I18NTextSearchable `bson:"name" json:"name"`
	DefaultCurrency string                `bson:"default_currency" json:"default_currency"`
	Enabled         bool                  `bson:"enabled" json:"enabled"`
	Prices          []*ProductPrice       `bson:"prices" json:"prices"`
	Description     map[string]string     `bson:"description" json:"description"`
	LongDescription map[string]string     `bson:"long_description,omitempty" json:"long_description"`
	CreatedAt       time.Time             `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time             `bson:"updated_at" json:"updated_at"`
	Images          []string              `bson:"images,omitempty" json:"images"`
	Url             string                `bson:"url,omitempty" json:"url"`
	Metadata        map[string]string     `bson:"metadata,omitempty" json:"metadata"`
	Deleted         bool                  `bson:"deleted" json:"deleted"`
	MerchantId      bson.ObjectId         `bson:"merchant_id" json:"-"`
}

func (p *Product) SetBSON(raw bson.Raw) error {
	decoded := new(MgoProduct)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	p.Id = decoded.Id.Hex()
	p.Object = decoded.Object
	p.Type = decoded.Type
	p.Sku = decoded.Sku
	p.DefaultCurrency = decoded.DefaultCurrency
	p.Enabled = decoded.Enabled
	p.Prices = decoded.Prices
	p.Description = decoded.Description
	p.LongDescription = decoded.LongDescription
	p.Images = decoded.Images
	p.Url = decoded.Url
	p.Metadata = decoded.Metadata
	p.Deleted = decoded.Deleted
	p.MerchantId = decoded.MerchantId.Hex()

	p.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	p.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	p.Name = map[string]string{}
	for _, i := range decoded.Name {
		p.Name[i.Lang] = i.Value
	}

	return nil
}

func (p *Product) GetBSON() (interface{}, error) {
	st := &MgoProduct{
		Object:          p.Object,
		Type:            p.Type,
		Sku:             p.Sku,
		DefaultCurrency: p.DefaultCurrency,
		Enabled:         p.Enabled,
		Prices:          p.Prices,
		Description:     p.Description,
		LongDescription: p.LongDescription,
		Images:          p.Images,
		Url:             p.Url,
		Metadata:        p.Metadata,
		Deleted:         p.Deleted,
	}

	if len(p.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(p.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(p.Id)
	}

	if len(p.Id) <= 0 {
		return nil, errors.New(errorInvalidObjectId)
	} else {
		if bson.IsObjectIdHex(p.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.MerchantId = bson.ObjectIdHex(p.MerchantId)
	}

	if p.CreatedAt != nil {
		t, err := ptypes.Timestamp(p.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if p.UpdatedAt != nil {
		t, err := ptypes.Timestamp(p.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	st.Name = []*I18NTextSearchable{}
	for k, v := range p.Name {
		st.Name = append(st.Name, &I18NTextSearchable{Lang: k, Value: v})
	}

	return st, nil
}
