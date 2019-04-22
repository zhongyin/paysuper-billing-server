package billing

import (
	"encoding/json"
	"github.com/golang/protobuf/ptypes/timestamp"
)

type JsonRefund struct {
	Id         string               `json:"id"`
	OrderId    string               `json:"order_id"`
	ExternalId string               `json:"external_id"`
	Amount     float64              `json:"amount"`
	CreatorId  string               `json:"creator_id"`
	Reason     string               `json:"reason"`
	Currency   string               `json:"currency"`
	Status     int32                `json:"status"`
	CreatedAt  *timestamp.Timestamp `json:"created_at"`
	UpdatedAt  *timestamp.Timestamp `json:"updated_at"`
	PayerData  *RefundPayerData     `json:"payer_data"`
	SalesTax   float32              `json:"sales_tax"`
}

func (m *Refund) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		&JsonRefund{
			Id:         m.Id,
			OrderId:    m.Order.Uuid,
			ExternalId: m.ExternalId,
			Amount:     m.Amount,
			CreatorId:  m.CreatorId,
			Reason:     m.Reason,
			Currency:   m.Currency.CodeA3,
			Status:     m.Status,
			CreatedAt:  m.CreatedAt,
			UpdatedAt:  m.UpdatedAt,
			PayerData:  m.PayerData,
			SalesTax:   m.SalesTax,
		},
	)
}
