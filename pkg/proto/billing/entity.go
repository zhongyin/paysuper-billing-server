package billing

import (
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
)

var (
	orderRefundAllowedStatuses = map[int32]bool{
		constant.OrderStatusPaymentSystemComplete: true,
		constant.OrderStatusProjectInProgress:     true,
		constant.OrderStatusProjectComplete:       true,
		constant.OrderStatusProjectPending:        true,
	}
)

func (m *Merchant) ChangesAllowed() bool {
	return m.Status == pkg.MerchantStatusDraft || m.Status == pkg.MerchantStatusRejected
}

func (m *Merchant) GetPayoutCurrency() *Currency {
	if m.Banking == nil {
		return nil
	}

	return m.Banking.Currency
}

func (m *PaymentMethodOrder) GetAccountingCurrency() *Currency {
	return m.PaymentSystem.AccountingCurrency
}

func (m *Order) HasEndedStatus() bool {
	return m.Status == constant.OrderStatusPaymentSystemReject || m.Status == constant.OrderStatusProjectComplete ||
		m.Status == constant.OrderStatusProjectReject || m.Status == constant.OrderStatusRefund ||
		m.Status == constant.OrderStatusChargeback
}

func (m *Order) RefundAllowed() bool {
	v, ok := orderRefundAllowedStatuses[m.Status]

	return ok && v == true
}
