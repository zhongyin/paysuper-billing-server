package billing

import "github.com/paysuper/paysuper-billing-server/pkg"

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