package service

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"gopkg.in/mgo.v2/bson"
)

type PaymentSystemMockOk struct {
	processor *paymentProcessor
}
type PaymentSystemMockError struct {
	processor *paymentProcessor
}

func NewPaymentSystemMockOk(processor *paymentProcessor) PaymentSystem {
	return &PaymentSystemMockOk{processor: processor}
}

func NewPaymentSystemMockError(processor *paymentProcessor) PaymentSystem {
	return &PaymentSystemMockError{processor: processor}
}

func (m *PaymentSystemMockOk) CreatePayment(map[string]string) (string, error) {
	return "", nil
}

func (m *PaymentSystemMockOk) ProcessPayment(request proto.Message, rawRequest string, signature string) error {
	return nil
}

func (m *PaymentSystemMockOk) IsRecurringCallback(request proto.Message) bool {
	return false
}

func (m *PaymentSystemMockOk) GetRecurringId(request proto.Message) string {
	return ""
}

func (m *PaymentSystemMockOk) CreateRefund(refund *billing.Refund) error {
	refund.Status = pkg.RefundStatusInProgress
	refund.ExternalId = bson.NewObjectId().Hex()

	return nil
}

func (m *PaymentSystemMockOk) ProcessRefund(
	refund *billing.Refund,
	message proto.Message,
	raw,
	signature string,
) (err error) {
	refund.Status = pkg.RefundStatusCompleted
	refund.ExternalId = bson.NewObjectId().Hex()

	return
}

func (m *PaymentSystemMockError) CreatePayment(map[string]string) (string, error) {
	return "", nil
}

func (m *PaymentSystemMockError) ProcessPayment(request proto.Message, rawRequest string, signature string) error {
	return nil
}

func (m *PaymentSystemMockError) IsRecurringCallback(request proto.Message) bool {
	return false
}

func (m *PaymentSystemMockError) GetRecurringId(request proto.Message) string {
	return ""
}

func (m *PaymentSystemMockError) CreateRefund(refund *billing.Refund) error {
	refund.Status = pkg.RefundStatusRejected
	return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
}

func (m *PaymentSystemMockError) ProcessRefund(
	refund *billing.Refund,
	message proto.Message,
	raw,
	signature string,
) (err error) {
	return NewError(paymentSystemErrorRefundRequestAmountOrCurrencyIsInvalid, pkg.ResponseStatusBadData)
}
