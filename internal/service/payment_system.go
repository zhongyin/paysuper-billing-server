package service

import (
	"errors"
	"github.com/golang/protobuf/proto"
	"github.com/paysuper/paysuper-billing-server/internal/config"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
)

const (
	paymentSystemHandlerCardPay = "cardpay"

	paymentSystemErrorHandlerNotFound                  = "handler for specified payment system not found"
	paymentSystemErrorAuthenticateFailed               = "authentication failed"
	paymentSystemErrorUnknownPaymentMethod             = "unknown payment method"
	paymentSystemErrorCreateRequestFailed              = "order can't be create. try request later"
	paymentSystemErrorEWalletIdentifierIsInvalid       = "wallet identifier is invalid"
	paymentSystemErrorRequestSignatureIsInvalid        = "request signature is invalid"
	paymentSystemErrorRequestTimeFieldIsInvalid        = "time field in request is invalid"
	paymentSystemErrorRequestRecurringIdFieldIsInvalid = "recurring id field in request is invalid"
	paymentSystemErrorRequestStatusIsInvalid           = "status is invalid"
	paymentSystemErrorRequestPaymentMethodIsInvalid    = "payment method from request not match with value in order"
	paymentSystemErrorRequestAmountOrCurrencyIsInvalid = "amount or currency from request not match with value in order"
	paymentSystemErrorRequestTemporarySkipped          = "notification skipped with temporary status"
	paymentSystemErrorCreateRefundFailed               = "refund can't be create. try request later"
	paymentSystemErrorCreateRefundRejected             = "refund create request rejected"

	defaultHttpClientTimeout = 10
	defaultResponseBodyLimit = 512
)

var paymentSystemHandlers = map[string]func(*paymentProcessor) PaymentSystem{
	paymentSystemHandlerCardPay: newCardPayHandler,
}

type Error struct {
	err    string
	status int32
}

type Path struct {
	path   string
	method string
}

type PaymentSystem interface {
	CreatePayment(map[string]string) (string, error)
	ProcessPayment(request proto.Message, rawRequest string, signature string) error
	IsRecurringCallback(request proto.Message) bool
	GetRecurringId(request proto.Message) string
	Refund(refund *billing.Refund) error
}

type paymentProcessor struct {
	cfg     *config.PaymentSystemConfig
	order   *billing.Order
	service *Service
}

func (s *Service) NewPaymentSystem(
	cfg *config.PaymentSystemConfig,
	order *billing.Order,
) (PaymentSystem, error) {
	h, ok := paymentSystemHandlers[order.PaymentMethod.Params.Handler]

	if !ok {
		return nil, errors.New(paymentSystemErrorHandlerNotFound)
	}

	processor := &paymentProcessor{cfg: cfg, order: order, service: s}

	return h(processor), nil
}

func NewError(text string, status int32) error {
	return &Error{err: text, status: status}
}

func (e *Error) Error() string {
	return e.err
}

func (e *Error) Status() int32 {
	return e.status
}

func (h *paymentProcessor) cutBytes(body []byte, limit int) string {
	sBody := string(body)
	r := []rune(sBody)

	if len(r) > limit {
		return string(r[:limit])
	}

	return sBody
}

func (h *paymentProcessor) httpHeadersToString(headers map[string][]string) string {
	var out string

	for k, v := range headers {
		out += k + ":" + v[0] + "\n "
	}

	return out
}
