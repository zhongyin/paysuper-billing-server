package grpc

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
