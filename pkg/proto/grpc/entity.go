package grpc

import (
	"errors"
	"fmt"
)

var (
	productNoPriceInCurrency = "no price in currency %s"
)

func (m *MerchantPaymentMethodRequest) GetPerTransactionCurrency() string {
	return m.Commission.PerTransaction.Currency
}

func (m *MerchantPaymentMethodRequest) GetPerTransactionFee() float64 {
	return m.Commission.PerTransaction.Fee
}

func (m *MerchantPaymentMethodRequest) HasPerTransactionCurrency() bool {
	return m.Commission.PerTransaction.Currency != ""
}

func (m *MerchantPaymentMethodRequest) HasIntegration() bool {
	return m.Integration.TerminalId != "" && m.Integration.TerminalPassword != "" &&
		m.Integration.TerminalCallbackPassword != ""
}

func (p *Product) IsPricesContainDefaultCurrency() bool {
	_, err := p.GetPriceInCurrency(p.DefaultCurrency)
	return err == nil
}

func (p *Product) GetPriceInCurrency(currency string) (float64, error) {
	for _, price := range p.Prices {
		if price.Currency == currency {
			return price.Amount, nil
		}
	}
	return 0, errors.New(fmt.Sprintf(productNoPriceInCurrency, currency))
}
