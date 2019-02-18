package billing

import "github.com/ProtocolONE/payone-billing-service/pkg"

func (m *CardPayPaymentCallback) IsPaymentAllowedStatus() bool {
	return m.PaymentData.Status == pkg.CardPayPaymentResponseStatusCompleted || m.PaymentData.Status == pkg.CardPayPaymentResponseStatusDeclined ||
		m.PaymentData.Status == pkg.CardPayPaymentResponseStatusCancelled || m.PaymentData.Status == pkg.CardPayPaymentResponseStatusAuthorized
}

func (m *CardPayPaymentCallback) GetBankCardTxnParams() map[string]string {
	params := make(map[string]string)

	params[pkg.PaymentCreateFieldPan] = m.CardAccount.MaskedPan
	params[pkg.PaymentCreateFieldHolder] = m.CardAccount.Holder
	params[pkg.TxnParamsFieldBankCardEmissionCountry] = m.CardAccount.IssuingCountryCode
	params[pkg.TxnParamsFieldBankCardToken] = m.CardAccount.Token
	params[pkg.TxnParamsFieldBankCardRrn] = m.PaymentData.Rrn

	params[pkg.TxnParamsFieldBankCardIs3DS] = "0"

	if m.PaymentData.Is_3D == true {
		params[pkg.TxnParamsFieldBankCardIs3DS] = "1"
	}

	if m.PaymentData.Status == pkg.CardPayPaymentResponseStatusDeclined {
		params[pkg.TxnParamsFieldDeclineCode] = m.PaymentData.DeclineCode
		params[pkg.TxnParamsFieldDeclineReason] = m.PaymentData.DeclineReason
	}

	return params
}

func (m *CardPayPaymentCallback) GetEWalletTxnParams() map[string]string {
	params := make(map[string]string)

	params[pkg.PaymentCreateFieldEWallet] = m.EwalletAccount.Id

	if m.PaymentData.Status == pkg.CardPayPaymentResponseStatusDeclined {
		params[pkg.TxnParamsFieldDeclineCode] = m.PaymentData.DeclineCode
		params[pkg.TxnParamsFieldDeclineReason] = m.PaymentData.DeclineReason
	}

	return params
}

func (m *CardPayPaymentCallback) GetCryptoCurrencyTxnParams() map[string]string {
	params := make(map[string]string)

	params[pkg.PaymentCreateFieldCrypto] = m.CryptocurrencyAccount.CryptoAddress
	params[pkg.TxnParamsFieldCryptoTransactionId] = m.CryptocurrencyAccount.CryptoTransactionId
	params[pkg.TxnParamsFieldCryptoAmount] = m.CryptocurrencyAccount.PrcAmount
	params[pkg.TxnParamsFieldCryptoCurrency] = m.CryptocurrencyAccount.PrcCurrency

	if m.PaymentData.Status == pkg.CardPayPaymentResponseStatusDeclined {
		params[pkg.TxnParamsFieldDeclineCode] = m.PaymentData.DeclineCode
		params[pkg.TxnParamsFieldDeclineReason] = m.PaymentData.DeclineReason
	}

	return params
}
