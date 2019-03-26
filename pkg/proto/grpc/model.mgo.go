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
	Id              bson.ObjectId     `bson:"_id" json:"id"`
	Object          string            `bson:"object" json:"object"`
	Type            string            `bson:"type" json:"type"`
	Sku             string            `bson:"sku" json:"sku"`
	Name            string            `bson:"name" json:"name"`
	DefaultCurrency string            `bson:"default_currency" json:"default_currency"`
	Enabled         bool              `bson:"enabled" json:"enabled"`
	Prices          []*ProductPrice   `bson:"prices" json:"prices"`
	Description     string            `bson:"description" json:"description"`
	LongDescription string            `bson:"long_description" json:"long_description"`
	CreatedAt       time.Time         `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time         `bson:"updated_at" json:"updated_at"`
	Images          []string          `bson:"images" json:"images"`
	Url             string            `bson:"url" json:"url"`
	Metadata        map[string]string `bson:"metadata" json:"metadata"`
	Deleted         bool              `bson:"deleted" json:"deleted"`
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
	p.Name = decoded.Name
	p.DefaultCurrency = decoded.DefaultCurrency
	p.Enabled = decoded.Enabled
	p.Prices = decoded.Prices
	p.Description = decoded.Description
	p.LongDescription = decoded.LongDescription
	p.Images = decoded.Images
	p.Url = decoded.Url
	p.Metadata = decoded.Metadata
	p.Deleted = decoded.Deleted

	p.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	p.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (p *Product) GetBSON() (interface{}, error) {
	st := &MgoProduct{
		Object:          p.Object,
		Type:            p.Type,
		Sku:             p.Sku,
		Name:            p.Name,
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

	return st, nil
}
