package grpc

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	productNoPriceInCurrency           = "no price in currency %s"
	productNoNameInLanguage            = "no name in language %s"
	productNoDescriptionInLanguage     = "no description in language %s"
	productNoLongDescriptionInLanguage = "no long description in language %s"
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

func (p *Product) GetLocalizedName(lang string) (string, error) {
	v, ok := p.Description[lang]
	if !ok {
		return "", errors.New(fmt.Sprintf(productNoNameInLanguage, lang))
	}
	return v, nil
}

func (p *Product) GetLocalizedDescription(lang string) (string, error) {
	v, ok := p.Description[lang]
	if !ok {
		return "", errors.New(fmt.Sprintf(productNoDescriptionInLanguage, lang))
	}
	return v, nil
}

func (p *Product) GetLocalizedLongDescription(lang string) (string, error) {
	v, ok := p.LongDescription[lang]
	if !ok {
		return "", errors.New(fmt.Sprintf(productNoLongDescriptionInLanguage, lang))
	}
	return v, nil
}

func (m *PaymentFormJsonDataRequest) HasUserCookie(regex string) bool {
	if m.Cookie == "" {
		return false
	}

	match, _ := regexp.MatchString(regex, m.Cookie)

	return match
}
