package service

import (
	"errors"
	"github.com/ProtocolONE/payone-billing-service/internal/config"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"go.uber.org/zap"
)

const (
	paymentSystemHandlerCardPay = "cardpay"

	paymentSystemErrorHandlerNotFound                = "handler for specified payment system not found"
	paymentSystemErrorSettingsNotFound               = "payment system settings not found"
	paymentSystemErrorAuthenticateFailed             = "authentication failed"
	paymentSystemErrorUnknownPaymentMethod           = "unknown payment method"
	paymentSystemErrorCreateRequestFailed            = "order can't be create. try request later"
	paymentSystemErrorEWalletIdentifierIsInvalid     = "wallet identifier is invalid"
	paymentSystemErrorCryptoCurrencyAddressIsInvalid = "crypto currency address is invalid"
	paymentSystemErrorRequestSignatureIsInvalid      = "request signature is invalid"
	paymentSystemErrorRequestTimeFieldIsInvalid      = "time field in request is invalid"
	paymentSystemErrorRequestStatusIsInvalid         = "status is invalid"
	paymentSystemErrorRequestPaymentMethodIsInvalid  = "payment method from request not equal value in order"
	paymentSystemErrorRequestTemporarySkipped        = "notification skipped with temporary status"
)

var paymentSystemHandlers = map[string]func(*paymentProcessor) PaymentSystem{
	paymentSystemHandlerCardPay: newCardPayHandler,
}

type Path struct {
	path   string
	method string
}

type Authenticate struct {
	Terminal         string
	Password         string
	CallbackPassword string
}

type PaymentSystem interface {
	CreatePayment(order *billing.Order, requisites map[string]string) (string, error)
	ProcessPayment() error
}

type paymentProcessor struct {
	cfg  *config.PaymentSystemConfig
	log  *zap.SugaredLogger
	auth *Authenticate
}

func (s *Service) NewPaymentSystem(
	cfg *config.PaymentSystemConfig,
	log *zap.SugaredLogger,
	auth *Authenticate,
	order *billing.Order,
	requisites map[string]string,
) (PaymentSystem, error) {
	h, ok := paymentSystemHandlers[order.PaymentMethod.Params.Handler]

	if !ok {
		return nil, errors.New(paymentSystemErrorHandlerNotFound)
	}

	processor := &paymentProcessor{cfg: cfg, log: log, auth: auth}

	return h(processor), nil
}
