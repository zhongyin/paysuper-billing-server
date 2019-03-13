package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
)

const (
	refundErrorNotAllowed        = "create refund for order not allowed"
	refundErrorAlreadyRefunded   = "amount by order was fully refunded"
	refundErrorPaymentAmountLess = "refund unavailable, because payment amount less than total refunds amount"

	refundDefaultReasonMask = "Refund by order #%s"
)

type RefundError struct {
	err    string
	status int32
}

type createRefundChecked struct {
	order *billing.Order
}

type createRefundProcessor struct {
	service *Service
	request *grpc.CreateRefundRequest
	checked *createRefundChecked
}

func (s *Service) CreateRefund(
	ctx context.Context,
	req *grpc.CreateRefundRequest,
	rsp *grpc.CreateRefundResponse,
) error {
	processor := &createRefundProcessor{
		service: s,
		request: req,
		checked: &createRefundChecked{},
	}

	refund, err := processor.processCreateRefund()

	if err != nil {
		rsp.Status = err.(*RefundError).status
		rsp.Message = err.(*RefundError).err

		return nil
	}

	h, err := s.NewPaymentSystem(s.cfg.PaymentSystemConfig, processor.checked.order)

	if err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = err.Error()

		return nil
	}

	err = h.Refund(refund)

	if err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = err.Error()

		return nil
	}

	err = s.db.Collection(pkg.CollectionRefund).UpdateId(bson.ObjectIdHex(refund.Id), refund)

	if err != nil {
		s.logError("Query to update refund failed", []interface{}{"err", err.Error(), "data", refund})

		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = orderErrorUnknown

		return nil
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = refund

	return nil
}

func (p *createRefundProcessor) processCreateRefund() (*billing.Refund, error) {
	err := p.processOrder()

	if err != nil {
		return nil, err
	}

	err = p.processRefundsByOrder()

	if err != nil {
		return nil, err
	}

	refund := &billing.Refund{
		Id:        bson.NewObjectId().Hex(),
		OrderId:   p.checked.order.Id,
		Amount:    p.request.Amount,
		CreatorId: p.request.CreatorId,
		Reason:    fmt.Sprintf(refundDefaultReasonMask, p.checked.order.Id),
		Currency:  p.checked.order.PaymentMethodIncomeCurrency,
		Status:    pkg.RefundStatusCreated,
		CreatedAt: ptypes.TimestampNow(),
		UpdatedAt: ptypes.TimestampNow(),
	}

	if p.request.Reason != "" {
		refund.Reason = p.request.Reason
	}

	err = p.service.db.Collection(pkg.CollectionRefund).Insert(refund)

	if err != nil {
		p.service.logError("Query to insert refund failed", []interface{}{"err", err.Error(), "data", refund})
		return nil, p.service.NewRefundError(orderErrorUnknown, pkg.ResponseStatusBadData)
	}

	return refund, nil
}

func (p *createRefundProcessor) processOrder() error {
	order, err := p.service.getOrderById(p.request.OrderId)

	if err != nil {
		return p.service.NewRefundError(err.Error(), pkg.ResponseStatusNotFound)
	}

	if order.RefundAllowed() == false {
		return p.service.NewRefundError(refundErrorNotAllowed, pkg.ResponseStatusBadData)
	}

	if order.Status == constant.OrderStatusRefund {
		return p.service.NewRefundError(refundErrorAlreadyRefunded, pkg.ResponseStatusBadData)
	}

	p.checked.order = order

	return nil
}

func (p *createRefundProcessor) processRefundsByOrder() error {
	refundedAmount, err := p.getRefundedAmount(p.checked.order)

	if err != nil {
		return p.service.NewRefundError(err.Error(), pkg.ResponseStatusBadData)
	}

	if p.checked.order.PaymentMethodIncomeAmount < (refundedAmount + p.request.Amount) {
		return p.service.NewRefundError(refundErrorPaymentAmountLess, pkg.ResponseStatusBadData)
	}

	return nil
}

func (p *createRefundProcessor) getRefundedAmount(order *billing.Order) (float64, error) {
	var res struct {
		Id     bson.ObjectId `bson:"_id"`
		Amount float64       `bson:"amount"`
	}

	query := []bson.M{
		{
			"$match": bson.M{
				"status":   bson.M{"$nin": []int32{pkg.RefundStatusRejected}},
				"order_id": bson.ObjectIdHex(order.Id),
			},
		},
		{"$group": bson.M{"_id": "$order_id", "amount": bson.M{"$sum": "$amount"}}},
	}

	err := p.service.db.Collection(pkg.CollectionRefund).Pipe(query).One(&res)

	if err != nil && !p.service.IsDbNotFoundError(err) {
		p.service.logError("Query to calculate refunded amount by order failed", []interface{}{"err", err.Error(), "query", query})
		return 0, errors.New(orderErrorUnknown)
	}

	return res.Amount, nil
}

func (s *Service) NewRefundError(text string, status int32) error {
	return &RefundError{err: text, status: status}
}

func (e *RefundError) Error() string {
	return e.err
}

func (e *RefundError) Status() int32 {
	return e.status
}
