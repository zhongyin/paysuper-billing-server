package pkg

const (
	ServiceName    = "p1paybilling"
	ServiceVersion = "latest"

	CollectionCurrency                     = "currency"
	CollectionCountry                      = "country"
	CollectionProject                      = "project"
	CollectionCurrencyRate                 = "currency_rate"
	CollectionOrder                        = "order"
	CollectionPaymentMethod                = "payment_method"
	CollectionCommission                   = "commission"
	CollectionBinData                      = "bank_bin"
	CollectionMerchant                     = "merchant"
	CollectionNotification                 = "notification"
	CollectionRefund                       = "refund"
	CollectionProduct                      = "product"
	CollectionSystemFees                   = "system_fees"
	CollectionMerchantPaymentMethodHistory = "payment_method_history"
	CollectionCustomer                     = "customer"

	CardPayPaymentResponseStatusInProgress = "IN_PROGRESS"
	CardPayPaymentResponseStatusPending    = "PENDING"
	CardPayPaymentResponseStatusRefunded   = "REFUNDED"
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
	PaymentCreateFieldUserCountry     = "country"
	PaymentCreateFieldUserCity        = "city"
	PaymentCreateFieldUserZip         = "zip"

	TxnParamsFieldBankCardEmissionCountry = "emission_country"
	TxnParamsFieldBankCardToken           = "token"
	TxnParamsFieldBankCardIs3DS           = "is_3ds"
	TxnParamsFieldBankCardRrn             = "rrn"
	TxnParamsFieldDeclineCode             = "decline_code"
	TxnParamsFieldDeclineReason           = "decline_reason"
	TxnParamsFieldCryptoTransactionId     = "transaction_id"
	TxnParamsFieldCryptoAmount            = "amount_crypto"
	TxnParamsFieldCryptoCurrency          = "currency_crypto"

	StatusOK              = int32(0)
	StatusErrorValidation = int32(1)
	StatusErrorSystem     = int32(2)
	StatusTemporary       = int32(4)

	MerchantStatusDraft              = int32(0)
	MerchantStatusAgreementRequested = int32(1)
	MerchantStatusOnReview           = int32(2)
	MerchantStatusAgreementSigning   = int32(3)
	MerchantStatusAgreementSigned    = int32(4)
	MerchantStatusDeleted            = int32(5)

	ResponseStatusOk          = int32(200)
	ResponseStatusBadData     = int32(400)
	ResponseStatusNotFound    = int32(404)
	ResponseStatusSystemError = int32(500)
	ResponseStatusTemporary   = int32(410)

	SystemUserId = "000000000000000000000000"

	RefundStatusCreated               = int32(0)
	RefundStatusRejected              = int32(1)
	RefundStatusInProgress            = int32(2)
	RefundStatusCompleted             = int32(3)
	RefundStatusPaymentSystemDeclined = int32(4)
	RefundStatusPaymentSystemCanceled = int32(5)

	PaymentSystemErrorCreateRefundFailed   = "refund can't be create. try request later"
	PaymentSystemErrorCreateRefundRejected = "refund create request rejected"

	PaymentSystemHandlerCardPay = "cardpay"

	MerchantAgreementTypeESign = 2

	ProjectStatusDraft         = int32(0)
	ProjectStatusTestCompleted = int32(1)
	ProjectStatusTestFailed    = int32(2)
	ProjectStatusInProduction  = int32(3)
	ProjectStatusDeleted       = int32(4)

	ProjectCallbackProtocolEmpty   = "empty"
	ProjectCallbackProtocolDefault = "default"

	ObjectTypeUser = "user"

	UserIdentityTypeEmail    = "email"
	UserIdentityTypePhone    = "phone"
	UserIdentityTypeExternal = "external"

	TechEmailDomain        = "@paysuper.com"
	OrderInlineFormUrlMask = "%s://%s/order/%s"
)

var (
	CountryPhoneCodes = map[int32]string{
		7:    "RU",
		375:  "BY",
		994:  "AZ",
		91:   "IN",
		77:   "KZ",
		380:  "UA",
		44:   "GB",
		9955: "GE",
		370:  "LT",
		992:  "TJ",
		66:   "TH",
		998:  "UZ",
		507:  "PA",
		374:  "AM",
		371:  "LV",
		90:   "TR",
		373:  "MD",
		972:  "IL",
		84:   "VN",
		372:  "EE",
		82:   "KR",
		996:  "KG",
	}
)
