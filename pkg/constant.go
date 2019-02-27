package pkg

const (
	ServiceName    = "p1paybilling"
	ServiceVersion = "latest"

	CollectionCurrency      = "currency"
	CollectionCountry       = "country"
	CollectionProject       = "project"
	CollectionCurrencyRate  = "currency_rate"
	CollectionVat           = "vat"
	CollectionOrder         = "order"
	CollectionPaymentMethod = "payment_method"
	CollectionCommission    = "commission"
	CollectionBinData       = "bank_bin"
	CollectionMerchant      = "merchant"

	CardPayPaymentResponseStatusDeclined   = "DECLINED"
	CardPayPaymentResponseStatusAuthorized = "AUTHORIZED"
	CardPayPaymentResponseStatusCompleted  = "COMPLETED"
	CardPayPaymentResponseStatusCancelled  = "CANCELLED"

	PaymentCreateFieldOrderId         = "order_id"
	PaymentCreateFieldPaymentMethodId = "payment_method_id"
	PaymentCreateFieldEmail           = "email"
	PaymentCreateFieldPan             = "pan"
	PaymentCreateFieldCvv             = "cvv"
	PaymentCreateFieldMonth           = "month"
	PaymentCreateFieldYear            = "year"
	PaymentCreateFieldHolder          = "card_holder"
	PaymentCreateFieldEWallet         = "ewallet"
	PaymentCreateFieldCrypto          = "address"
	PaymentCreateFieldStoreData       = "store_data"
	PaymentCreateFieldRecurringId     = "recurring_id"
	PaymentCreateFieldStoredCardId    = "stored_card_id"

	TxnParamsFieldBankCardEmissionCountry = "emission_country"
	TxnParamsFieldBankCardToken           = "token"
	TxnParamsFieldBankCardIs3DS           = "is_3ds"
	TxnParamsFieldBankCardRrn             = "rrn"
	TxnParamsFieldDeclineCode             = "decline_code"
	TxnParamsFieldDeclineReason           = "decline_reason"
	TxnParamsFieldCryptoTransactionId     = "transaction_id"
	TxnParamsFieldCryptoAmount            = "amount_crypto"
	TxnParamsFieldCryptoCurrency          = "currency_crypto"

	StatusOK                 = int32(0)
	StatusErrorValidation    = int32(1)
	StatusErrorSystem        = int32(2)
	StatusErrorPaymentSystem = int32(3)
	StatusTemporary          = int32(4)

	MerchantStatusDraft              = int32(0)
	MerchantStatusAgreementRequested = int32(1)
	MerchantStatusVerification       = int32(2)
	MerchantStatusApproved           = int32(3)
	MerchantStatusRejected           = int32(4)
	MerchantStatusAgreementSigning   = int32(5)
	MerchantStatusAgreementSigned    = int32(6)
)
