package billing

import (
	"errors"
	"fmt"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
	"time"
)

const (
	errorInvalidObjectId = "invalid bson object id"
	errorRequiredField   = "field \"%s\" is required to convert object %s"
)

type MgoMultiLang struct {
	Lang  string `bson:"lang"`
	Value string `bson:"value"`
}

type MgoVat struct {
	Id          bson.ObjectId `bson:"_id" json:"id"`
	Country     *Country      `bson:"country" json:"country"`
	Subdivision string        `bson:"subdivision_code" json:"subdivision_code,omitempty"`
	Vat         float64       `bson:"vat" json:"vat"`
	IsActive    bool          `bson:"is_active" json:"is_active"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updated_at,omitempty"`
}

type MgoSystemFee struct {
	Percent         float64 `bson:"percent"`
	PercentCurrency string  `bson:"percent_currency"`
	FixAmount       float64 `bson:"fix_amount"`
	FixCurrency     string  `bson:"fix_currency"`
}

type MgoFeeSet struct {
	MinAmounts       []*MinAmount  `bson:"min_amounts"`
	TransactionCost  *MgoSystemFee `bson:"transaction_cost"`
	AuthorizationFee *MgoSystemFee `bson:"authorization_fee"`
}

type MgoSystemFees struct {
	Id        bson.ObjectId `bson:"_id"`
	MethodId  bson.ObjectId `bson:"method_id"`
	Region    string        `bson:"region"`
	CardBrand string        `bson:"card_brand"`
	Fees      []*MgoFeeSet  `bson:"fees"`
	UserId    bson.ObjectId `bson:"user_id"`
	CreatedAt time.Time     `bson:"created_at"`
	IsActive  bool          `bson:"is_active"`
}

type MgoProject struct {
	Id                       bson.ObjectId   `bson:"_id"`
	MerchantId               bson.ObjectId   `bson:"merchant_id"`
	Name                     []*MgoMultiLang `bson:"name"`
	CallbackCurrency         string          `bson:"callback_currency"`
	CallbackProtocol         string          `bson:"callback_protocol"`
	CreateOrderAllowedUrls   []string        `bson:"create_order_allowed_urls"`
	AllowDynamicNotifyUrls   bool            `bson:"allow_dynamic_notify_urls"`
	AllowDynamicRedirectUrls bool            `bson:"allow_dynamic_redirect_urls"`
	LimitsCurrency           string          `bson:"limits_currency"`
	MinPaymentAmount         float64         `bson:"min_payment_amount"`
	MaxPaymentAmount         float64         `bson:"max_payment_amount"`
	NotifyEmails             []string        `bson:"notify_emails"`
	IsProductsCheckout       bool            `bson:"is_products_checkout"`
	SecretKey                string          `bson:"secret_key"`
	SignatureRequired        bool            `bson:"signature_required"`
	SendNotifyEmail          bool            `bson:"send_notify_email"`
	UrlCheckAccount          string          `bson:"url_check_account"`
	UrlProcessPayment        string          `bson:"url_process_payment"`
	UrlRedirectFail          string          `bson:"url_redirect_fail"`
	UrlRedirectSuccess       string          `bson:"url_redirect_success"`
	Status                   int32           `bson:"status"`
	CreatedAt                time.Time       `bson:"created_at"`
	UpdatedAt                time.Time       `bson:"updated_at"`
	ProductsCount            int32           `bson:"products_count"`
	IdString                 string          `bson:"id_string"`
	UrlChargebackPayment     string          `bson:"url_chargeback_payment"`
	UrlCancelPayment         string          `bson:"url_cancel_payment"`
	UrlFraudPayment          string          `bson:"url_fraud_payment"`
	UrlRefundPayment         string          `bson:"url_refund_payment"`
}

type MgoMerchantLastPayout struct {
	Date   time.Time `bson:"date"`
	Amount float64   `bson:"amount"`
}

type MgoMerchantPaymentMethodIdentification struct {
	Id   bson.ObjectId `bson:"id"`
	Name string        `bson:"name"`
}

type MgoMerchantPaymentMethod struct {
	PaymentMethod *MgoMerchantPaymentMethodIdentification `bson:"payment_method"`
	Commission    *MerchantPaymentMethodCommissions       `bson:"commission"`
	Integration   *MerchantPaymentMethodIntegration       `bson:"integration"`
	IsActive      bool                                    `bson:"is_active"`
}

type MgoMerchant struct {
	Id                        bson.ObjectId                        `bson:"_id"`
	User                      *MerchantUser                        `bson:"user"`
	Name                      string                               `bson:"name"`
	AlternativeName           string                               `bson:"alternative_name"`
	Website                   string                               `bson:"website"`
	Country                   *Country                             `bson:"country"`
	State                     string                               `bson:"state"`
	Zip                       string                               `bson:"zip"`
	City                      string                               `bson:"city"`
	Address                   string                               `bson:"address"`
	AddressAdditional         string                               `bson:"address_additional"`
	RegistrationNumber        string                               `bson:"registration_number"`
	TaxId                     string                               `bson:"tax_id"`
	Contacts                  *MerchantContact                     `bson:"contacts"`
	Banking                   *MerchantBanking                     `bson:"banking"`
	Status                    int32                                `bson:"status"`
	CreatedAt                 time.Time                            `bson:"created_at"`
	UpdatedAt                 time.Time                            `bson:"updated_at"`
	FirstPaymentAt            time.Time                            `bson:"first_payment_at"`
	IsVatEnabled              bool                                 `bson:"is_vat_enabled"`
	IsCommissionToUserEnabled bool                                 `bson:"is_commission_to_user_enabled"`
	HasMerchantSignature      bool                                 `bson:"has_merchant_signature"`
	HasPspSignature           bool                                 `bson:"has_psp_signature"`
	LastPayout                *MgoMerchantLastPayout               `bson:"last_payout"`
	IsSigned                  bool                                 `bson:"is_signed"`
	PaymentMethods            map[string]*MgoMerchantPaymentMethod `bson:"payment_methods"`
	AgreementType             int32                                `bson:"agreement_type"`
	AgreementSentViaMail      bool                                 `bson:"agreement_sent_via_mail"`
	MailTrackingLink          string                               `bson:"mail_tracking_link"`
	S3AgreementName           string                               `bson:"s3_agreement_name"`
}

type MgoCurrencyRate struct {
	Id           bson.ObjectId `bson:"_id"`
	CurrencyFrom int32         `bson:"currency_from"`
	CurrencyTo   int32         `bson:"currency_to"`
	Rate         float64       `bson:"rate"`
	Date         time.Time     `bson:"date"`
	IsActive     bool          `bson:"is_active"`
	CreatedAt    time.Time     `bson:"created_at"`
}

type MgoCommission struct {
	Id struct {
		PaymentMethodId bson.ObjectId `bson:"pm_id"`
		ProjectId       bson.ObjectId `bson:"project_id"`
	} `bson:"_id"`
	PaymentMethodCommission float64   `bson:"pm_commission"`
	PspCommission           float64   `bson:"psp_commission"`
	ToUserCommission        float64   `bson:"total_commission_to_user"`
	StartDate               time.Time `bson:"start_date"`
}

type MgoCommissionBilling struct {
	Id                      bson.ObjectId `bson:"_id"`
	PaymentMethodId         bson.ObjectId `bson:"pm_id"`
	ProjectId               bson.ObjectId `bson:"project_id"`
	PaymentMethodCommission float64       `bson:"pm_commission"`
	PspCommission           float64       `bson:"psp_commission"`
	TotalCommissionToUser   float64       `bson:"total_commission_to_user"`
	StartDate               time.Time     `bson:"start_date"`
	CreatedAt               time.Time     `bson:"created_at"`
	UpdatedAt               time.Time     `bson:"updated_at"`
}

type MgoOrderProject struct {
	Id                   bson.ObjectId     `bson:"_id" `
	MerchantId           bson.ObjectId     `bson:"merchant_id"`
	Name                 map[string]string `bson:"name"`
	UrlSuccess           string            `bson:"url_success"`
	UrlFail              string            `bson:"url_fail"`
	NotifyEmails         []string          `bson:"notify_emails"`
	SecretKey            string            `bson:"secret_key"`
	SendNotifyEmail      bool              `bson:"send_notify_email"`
	UrlCheckAccount      string            `bson:"url_check_account"`
	UrlProcessPayment    string            `bson:"url_process_payment"`
	CallbackProtocol     string            `bson:"callback_protocol"`
	UrlChargebackPayment string            `bson:"url_chargeback_payment"`
	UrlCancelPayment     string            `bson:"url_cancel_payment"`
	UrlFraudPayment      string            `bson:"url_fraud_payment"`
	UrlRefundPayment     string            `bson:"url_refund_payment"`
	Status               int32             `bson:"status"`
}

type MgoOrderNotificationPaymentMethod struct {
	Id            bson.ObjectId        `bson:"_id"`
	Name          string               `bson:"name"`
	Params        *PaymentMethodParams `bson:"params"`
	PaymentSystem *PaymentSystem       `bson:"payment_system"`
	Group         string               `bson:"group_alias"`
}

type MgoOrder struct {
	Id                                      bson.ObjectId                      `bson:"_id"`
	IdString                                string                             `bson:"id_string"`
	Project                                 *MgoOrderProject                   `bson:"project"`
	ProjectOrderId                          string                             `bson:"project_order_id"`
	ProjectAccount                          string                             `bson:"project_account"`
	Description                             string                             `bson:"description"`
	ProjectIncomeAmount                     float64                            `bson:"project_income_amount"`
	ProjectIncomeCurrency                   *Currency                          `bson:"project_income_currency"`
	ProjectOutcomeAmount                    float64                            `bson:"project_outcome_amount"`
	ProjectOutcomeCurrency                  *Currency                          `bson:"project_outcome_currency"`
	ProjectLastRequestedAt                  time.Time                          `bson:"project_last_requested_at"`
	ProjectParams                           map[string]string                  `bson:"project_params"`
	PaymentMethod                           *MgoOrderNotificationPaymentMethod `bson:"payment_method"`
	PaymentMethodTerminalId                 string                             `bson:"pm_terminal_id"`
	PaymentMethodOrderId                    string                             `bson:"pm_order_id"`
	PaymentMethodOutcomeAmount              float64                            `bson:"pm_outcome_amount"`
	PaymentMethodOutcomeCurrency            *Currency                          `bson:"pm_outcome_currency"`
	PaymentMethodIncomeAmount               float64                            `bson:"pm_income_amount"`
	PaymentMethodIncomeCurrency             *Currency                          `bson:"pm_income_currency"`
	PaymentMethodOrderClosedAt              time.Time                          `bson:"pm_order_close_date"`
	Status                                  int32                              `bson:"status"`
	IsJsonRequest                           bool                               `bson:"created_by_json"`
	AmountInPspAccountingCurrency           float64                            `bson:"amount_psp_ac"`
	AmountInMerchantAccountingCurrency      float64                            `bson:"amount_in_merchant_ac"`
	AmountOutMerchantAccountingCurrency     float64                            `bson:"amount_out_merchant_ac"`
	AmountInPaymentSystemAccountingCurrency float64                            `bson:"amount_ps_ac"`
	PaymentMethodPayerAccount               string                             `bson:"pm_account"`
	PaymentMethodTxnParams                  map[string]string                  `bson:"pm_txn_params"`
	FixedPackage                            *FixedPackage                      `bson:"fixed_package"`
	PaymentRequisites                       map[string]string                  `bson:"payment_requisites"`
	PspFeeAmount                            *OrderFeePsp                       `bson:"psp_fee_amount"`
	ProjectFeeAmount                        *OrderFee                          `bson:"project_fee_amount"`
	ToPayerFeeAmount                        *OrderFee                          `bson:"to_payer_fee_amount"`
	VatAmount                               *OrderFee                          `bson:"vat_amount"`
	PaymentSystemFeeAmount                  *OrderFeePaymentSystem             `bson:"ps_fee_amount"`
	UrlSuccess                              string                             `bson:"url_success"`
	UrlFail                                 string                             `bson:"url_fail"`
	CreatedAt                               time.Time                          `bson:"created_at"`
	UpdatedAt                               time.Time                          `bson:"updated_at"`
	Products                                []string                           `bson:"products"`
	Items                                   []*OrderItem                       `bson:"items"`
	Amount                                  float64                            `bson:"amount"`
	Currency                                string                             `bson:"currency"`

	Uuid                    string               `bson:"uuid"`
	ExpireDateToFormInput   time.Time            `bson:"expire_date_to_form_input"`
	Tax                     *OrderTax            `bson:"tax"`
	TotalPaymentAmount      float64              `bson:"total_payment_amount"`
	UserAddressDataRequired bool                 `bson:"user_address_data_required"`
	BillingAddress          *OrderBillingAddress `bson:"billing_address"`
	User                    *OrderUser           `bson:"user"`

	RefundedAt         time.Time                `bson:"refunded_at"`
	CanceledAt         time.Time                `bson:"canceled_at"`
	CancellationReason string                   `bson:"cancellation_reason"`
	AgreementVersion   string                   `bson:"agreement_version"`
	AgreementAccepted  bool                     `bson:"agreement_accepted"`
	NotifySale         bool                     `bson:"notify_sale"`
	NotifySaleEmail    string                   `bson:"notify_sale_email"`
	Issuer             *OrderIssuer             `bson:"issuer"`
	Refund             *OrderNotificationRefund `bson:"refund"`
}

type MgoPaymentSystem struct {
	Id                 bson.ObjectId `bson:"_id"`
	Name               string        `bson:"name"`
	Country            *Country      `bson:"country"`
	AccountingCurrency *Currency     `bson:"accounting_currency"`
	AccountingPeriod   string        `bson:"accounting_period"`
	IsActive           bool          `bson:"is_active"`
	CreatedAt          time.Time     `bson:"created_at"`
	UpdatedAt          time.Time     `bson:"updated_at"`
}

type MgoPaymentMethod struct {
	Id               bson.ObjectId        `bson:"_id"`
	Name             string               `bson:"name"`
	Group            string               `bson:"group_alias"`
	Currency         *Currency            `bson:"currency"`
	MinPaymentAmount float64              `bson:"min_payment_amount"`
	MaxPaymentAmount float64              `bson:"max_payment_amount"`
	Params           *PaymentMethodParams `bson:"params"`
	Icon             string               `bson:"icon"`
	IsActive         bool                 `bson:"is_active"`
	CreatedAt        time.Time            `bson:"created_at"`
	UpdatedAt        time.Time            `bson:"updated_at"`
	PaymentSystem    *MgoPaymentSystem    `bson:"payment_system"`
	Currencies       []int32              `bson:"currencies"`
	Type             string               `bson:"type"`
	AccountRegexp    string               `bson:"account_regexp"`
}

type MgoNotification struct {
	Id         bson.ObjectId               `bson:"_id"`
	Title      string                      `bson:"title"`
	Message    string                      `bson:"message"`
	MerchantId bson.ObjectId               `bson:"merchant_id"`
	UserId     bson.ObjectId               `bson:"user_id"`
	IsSystem   bool                        `bson:"is_system"`
	IsRead     bool                        `bson:"is_read"`
	CreatedAt  time.Time                   `bson:"created_at"`
	UpdatedAt  time.Time                   `bson:"updated_at"`
	Statuses   *SystemNotificationStatuses `bson:"statuses"`
}

type MgoRefundOrder struct {
	Id   bson.ObjectId `bson:"id"`
	Uuid string        `bson:"uuid"`
}

type MgoRefund struct {
	Id         bson.ObjectId    `bson:"_id"`
	Order      *MgoRefundOrder  `bson:"order"`
	ExternalId string           `bson:"external_id"`
	Amount     float64          `bson:"amount"`
	CreatorId  bson.ObjectId    `bson:"creator_id"`
	Currency   *Currency        `bson:"currency"`
	Status     int32            `bson:"status"`
	CreatedAt  time.Time        `bson:"created_at"`
	UpdatedAt  time.Time        `bson:"updated_at"`
	PayerData  *RefundPayerData `bson:"payer_data"`
	SalesTax   float32          `bson:"sales_tax"`
}

type MgoMerchantPaymentMethodHistory struct {
	Id            bson.ObjectId             `bson:"_id"`
	MerchantId    bson.ObjectId             `bson:"merchant_id"`
	PaymentMethod *MgoMerchantPaymentMethod `bson:"payment_method"`
	CreatedAt     time.Time                 `bson:"created_at" json:"created_at"`
	UserId        bson.ObjectId             `bson:"user_id"`
}

type MgoCustomerIdentity struct {
	MerchantId bson.ObjectId `bson:"merchant_id"`
	ProjectId  bson.ObjectId `bson:"project_id"`
	Type       string        `bson:"type"`
	Value      string        `bson:"value"`
	Verified   bool          `bson:"verified"`
	CreatedAt  time.Time     `bson:"created_at"`
}

type MgoCustomerIpHistory struct {
	Ip        []byte    `bson:"ip"`
	CreatedAt time.Time `bson:"created_at"`
}

type MgoCustomerAddressHistory struct {
	Country    string    `bson:"country"`
	City       string    `bson:"city"`
	PostalCode string    `bson:"postal_code"`
	State      string    `bson:"state"`
	CreatedAt  time.Time `bson:"created_at"`
}

type MgoCustomerStringValueHistory struct {
	Value     string    `bson:"value"`
	CreatedAt time.Time `bson:"created_at"`
}

type MgoCustomer struct {
	Id                    bson.ObjectId                    `bson:"_id"`
	TechEmail             string                           `bson:"tech_email"`
	ExternalId            string                           `bson:"external_id"`
	Email                 string                           `bson:"email"`
	EmailVerified         bool                             `bson:"email_verified"`
	Phone                 string                           `bson:"phone"`
	PhoneVerified         bool                             `bson:"phone_verified"`
	Name                  string                           `bson:"name"`
	Ip                    []byte                           `bson:"ip"`
	Locale                string                           `bson:"locale"`
	AcceptLanguage        string                           `bson:"accept_language"`
	UserAgent             string                           `bson:"user_agent"`
	Address               *OrderBillingAddress             `bson:"address"`
	Identity              []*MgoCustomerIdentity           `bson:"identity"`
	IpHistory             []*MgoCustomerIpHistory          `bson:"ip_history"`
	AddressHistory        []*MgoCustomerAddressHistory     `bson:"address_history"`
	LocaleHistory         []*MgoCustomerStringValueHistory `bson:"locale_history"`
	AcceptLanguageHistory []*MgoCustomerStringValueHistory `bson:"accept_language_history"`
	Metadata              map[string]string                `bson:"metadata"`
	CreatedAt             time.Time                        `bson:"created_at"`
	UpdatedAt             time.Time                        `bson:"updated_at"`
}

func (m *Vat) GetBSON() (interface{}, error) {
	st := &MgoVat{
		Country:     m.Country,
		Subdivision: m.Subdivision,
		Vat:         m.Vat,
		IsActive:    m.IsActive,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (m *Vat) SetBSON(raw bson.Raw) error {
	decoded := new(MgoVat)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Country = decoded.Country
	m.Subdivision = decoded.Subdivision
	m.Vat = decoded.Vat
	m.IsActive = decoded.IsActive

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *SystemFees) GetBSON() (interface{}, error) {
	st := &MgoSystemFees{
		Id:        bson.ObjectIdHex(m.Id),
		MethodId:  bson.ObjectIdHex(m.MethodId),
		Region:    m.Region,
		CardBrand: m.CardBrand,
		IsActive:  m.IsActive,
		UserId:    bson.ObjectIdHex(m.UserId),
	}

	for _, f := range m.Fees {
		fs := &MgoFeeSet{}

		for c, a := range f.MinAmounts {
			fs.MinAmounts = append(fs.MinAmounts, &MinAmount{Amount: a, Currency: c})
		}

		fs.TransactionCost = &MgoSystemFee{
			Percent:         f.TransactionCost.Percent,
			PercentCurrency: f.TransactionCost.PercentCurrency,
			FixAmount:       f.TransactionCost.FixAmount,
			FixCurrency:     f.TransactionCost.FixCurrency,
		}

		fs.AuthorizationFee = &MgoSystemFee{
			Percent:         f.AuthorizationFee.Percent,
			PercentCurrency: f.AuthorizationFee.PercentCurrency,
			FixAmount:       f.AuthorizationFee.FixAmount,
			FixCurrency:     f.AuthorizationFee.FixCurrency,
		}

		st.Fees = append(st.Fees, fs)
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	return st, nil
}

func (m *SystemFees) SetBSON(raw bson.Raw) error {
	decoded := new(MgoSystemFees)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.MethodId = decoded.MethodId.Hex()
	m.Region = decoded.Region
	m.CardBrand = decoded.CardBrand
	m.IsActive = decoded.IsActive

	m.Fees = []*FeeSet{}

	for _, f := range decoded.Fees {

		fs := &FeeSet{}

		fs.MinAmounts = make(map[string]float64)
		for _, i := range f.MinAmounts {
			fs.MinAmounts[i.Currency] = i.Amount
		}

		fs.TransactionCost = &SystemFee{
			Percent:         f.TransactionCost.Percent,
			PercentCurrency: f.TransactionCost.PercentCurrency,
			FixAmount:       f.TransactionCost.FixAmount,
			FixCurrency:     f.TransactionCost.FixCurrency,
		}

		fs.AuthorizationFee = &SystemFee{
			Percent:         f.AuthorizationFee.Percent,
			PercentCurrency: f.AuthorizationFee.PercentCurrency,
			FixAmount:       f.AuthorizationFee.FixAmount,
			FixCurrency:     f.AuthorizationFee.FixCurrency,
		}

		m.Fees = append(m.Fees, fs)
	}

	m.UserId = decoded.UserId.Hex()
	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (m *Project) GetBSON() (interface{}, error) {
	st := &MgoProject{
		MerchantId:               bson.ObjectIdHex(m.MerchantId),
		CallbackCurrency:         m.CallbackCurrency,
		CallbackProtocol:         m.CallbackProtocol,
		CreateOrderAllowedUrls:   m.CreateOrderAllowedUrls,
		AllowDynamicNotifyUrls:   m.AllowDynamicNotifyUrls,
		AllowDynamicRedirectUrls: m.AllowDynamicRedirectUrls,
		LimitsCurrency:           m.LimitsCurrency,
		MaxPaymentAmount:         m.MaxPaymentAmount,
		MinPaymentAmount:         m.MinPaymentAmount,
		NotifyEmails:             m.NotifyEmails,
		IsProductsCheckout:       m.IsProductsCheckout,
		SecretKey:                m.SecretKey,
		SignatureRequired:        m.SignatureRequired,
		SendNotifyEmail:          m.SendNotifyEmail,
		UrlCheckAccount:          m.UrlCheckAccount,
		UrlProcessPayment:        m.UrlProcessPayment,
		UrlRedirectFail:          m.UrlRedirectFail,
		UrlRedirectSuccess:       m.UrlRedirectSuccess,
		Status:                   m.Status,
		UrlChargebackPayment:     m.UrlChargebackPayment,
		UrlCancelPayment:         m.UrlCancelPayment,
		UrlFraudPayment:          m.UrlFraudPayment,
		UrlRefundPayment:         m.UrlRefundPayment,
	}

	if len(m.Name) > 0 {
		for k, v := range m.Name {
			st.Name = append(st.Name, &MgoMultiLang{Lang: k, Value: v})
		}
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	st.IdString = st.Id.Hex()

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (m *Project) SetBSON(raw bson.Raw) error {
	decoded := new(MgoProject)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.MerchantId = decoded.MerchantId.Hex()
	m.CallbackCurrency = decoded.CallbackCurrency
	m.CallbackProtocol = decoded.CallbackProtocol
	m.CreateOrderAllowedUrls = decoded.CreateOrderAllowedUrls
	m.AllowDynamicNotifyUrls = decoded.AllowDynamicNotifyUrls
	m.AllowDynamicRedirectUrls = decoded.AllowDynamicRedirectUrls
	m.LimitsCurrency = decoded.LimitsCurrency
	m.MaxPaymentAmount = decoded.MaxPaymentAmount
	m.MinPaymentAmount = decoded.MinPaymentAmount
	m.NotifyEmails = decoded.NotifyEmails
	m.IsProductsCheckout = decoded.IsProductsCheckout
	m.SecretKey = decoded.SecretKey
	m.SignatureRequired = decoded.SignatureRequired
	m.SendNotifyEmail = decoded.SendNotifyEmail
	m.UrlCheckAccount = decoded.UrlCheckAccount
	m.UrlProcessPayment = decoded.UrlProcessPayment
	m.UrlRedirectFail = decoded.UrlRedirectFail
	m.UrlRedirectSuccess = decoded.UrlRedirectSuccess
	m.Status = decoded.Status
	m.UrlChargebackPayment = decoded.UrlChargebackPayment
	m.UrlCancelPayment = decoded.UrlCancelPayment
	m.UrlFraudPayment = decoded.UrlFraudPayment
	m.UrlRefundPayment = decoded.UrlRefundPayment

	nameLen := len(decoded.Name)

	if nameLen > 0 {
		m.Name = make(map[string]string, nameLen)

		for _, v := range decoded.Name {
			m.Name[v.Lang] = v.Value
		}
	}

	if decoded.ProductsCount > 0 {
		m.ProductsCount = decoded.ProductsCount
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *CurrencyRate) GetBSON() (interface{}, error) {
	st := &MgoCurrencyRate{
		CurrencyFrom: m.CurrencyFrom,
		CurrencyTo:   m.CurrencyTo,
		Rate:         m.Rate,
		IsActive:     m.IsActive,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.Date == nil {
		return nil, fmt.Errorf(errorRequiredField, "Date", "CurrencyRate")
	}

	t, err := ptypes.Timestamp(m.Date)

	if err != nil {
		return nil, err
	}

	st.Date = t

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	return st, nil
}

func (m *CurrencyRate) SetBSON(raw bson.Raw) error {
	decoded := new(MgoCurrencyRate)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.CurrencyFrom = decoded.CurrencyFrom
	m.CurrencyTo = decoded.CurrencyTo
	m.Rate = decoded.Rate
	m.IsActive = decoded.IsActive

	m.Date, err = ptypes.TimestampProto(decoded.Date)

	if err != nil {
		return err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	return err
}

func (m *Commission) GetBSON() (interface{}, error) {
	st := &MgoCommissionBilling{
		PaymentMethodId:         bson.ObjectIdHex(m.PaymentMethodId),
		ProjectId:               bson.ObjectIdHex(m.ProjectId),
		PaymentMethodCommission: m.PaymentMethodCommission,
		PspCommission:           m.PspCommission,
		TotalCommissionToUser:   m.TotalCommissionToUser,
	}

	t, err := ptypes.Timestamp(m.StartDate)

	if err != nil {
		return nil, err
	}

	st.StartDate = t

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (m *Commission) SetBSON(raw bson.Raw) error {
	decoded := new(MgoCommissionBilling)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.PaymentMethodId = decoded.PaymentMethodId.Hex()
	m.ProjectId = decoded.ProjectId.Hex()
	m.PaymentMethodCommission = decoded.PaymentMethodCommission
	m.PspCommission = decoded.PspCommission
	m.TotalCommissionToUser = decoded.TotalCommissionToUser

	m.StartDate, err = ptypes.TimestampProto(decoded.StartDate)

	if err != nil {
		return err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	return err
}

func (m *Order) GetBSON() (interface{}, error) {
	st := &MgoOrder{
		Project: &MgoOrderProject{
			Id:                bson.ObjectIdHex(m.Project.Id),
			MerchantId:        bson.ObjectIdHex(m.Project.MerchantId),
			Name:              m.Project.Name,
			UrlSuccess:        m.Project.UrlSuccess,
			UrlFail:           m.Project.UrlFail,
			NotifyEmails:      m.Project.NotifyEmails,
			SendNotifyEmail:   m.Project.SendNotifyEmail,
			SecretKey:         m.Project.SecretKey,
			UrlCheckAccount:   m.Project.UrlCheckAccount,
			UrlProcessPayment: m.Project.UrlProcessPayment,
			CallbackProtocol:  m.Project.CallbackProtocol,
			Status:            m.Project.Status,
		},
		ProjectOrderId:                          m.ProjectOrderId,
		ProjectAccount:                          m.ProjectAccount,
		Description:                             m.Description,
		ProjectIncomeAmount:                     m.ProjectIncomeAmount,
		ProjectIncomeCurrency:                   m.ProjectIncomeCurrency,
		ProjectOutcomeAmount:                    m.ProjectOutcomeAmount,
		ProjectOutcomeCurrency:                  m.ProjectOutcomeCurrency,
		ProjectParams:                           m.ProjectParams,
		PaymentMethodOrderId:                    m.PaymentMethodOrderId,
		PaymentMethodOutcomeAmount:              m.PaymentMethodOutcomeAmount,
		PaymentMethodOutcomeCurrency:            m.PaymentMethodOutcomeCurrency,
		PaymentMethodIncomeAmount:               m.PaymentMethodIncomeAmount,
		PaymentMethodIncomeCurrency:             m.PaymentMethodIncomeCurrency,
		Status:                                  m.Status,
		IsJsonRequest:                           m.IsJsonRequest,
		AmountInPspAccountingCurrency:           m.AmountInPspAccountingCurrency,
		AmountInMerchantAccountingCurrency:      m.AmountInMerchantAccountingCurrency,
		AmountOutMerchantAccountingCurrency:     m.AmountOutMerchantAccountingCurrency,
		AmountInPaymentSystemAccountingCurrency: m.AmountInPaymentSystemAccountingCurrency,
		PaymentMethodPayerAccount:               m.PaymentMethodPayerAccount,
		PaymentMethodTxnParams:                  m.PaymentMethodTxnParams,
		Products:                                m.Products,
		Items:                                   m.Items,
		Amount:                                  m.Amount,
		Currency:                                m.Currency,
		PaymentRequisites:                       m.PaymentRequisites,
		PspFeeAmount:                            m.PspFeeAmount,
		ProjectFeeAmount:                        m.ProjectFeeAmount,
		ToPayerFeeAmount:                        m.ToPayerFeeAmount,
		PaymentSystemFeeAmount:                  m.PaymentSystemFeeAmount,

		Uuid:                    m.Uuid,
		Tax:                     m.Tax,
		TotalPaymentAmount:      m.TotalPaymentAmount,
		UserAddressDataRequired: m.UserAddressDataRequired,
		BillingAddress:          m.BillingAddress,
		User:                    m.User,

		CancellationReason: m.CancellationReason,
		AgreementVersion:   m.AgreementVersion,
		AgreementAccepted:  m.AgreementAccepted,
		NotifySale:         m.NotifySale,
		NotifySaleEmail:    m.NotifySaleEmail,
		Issuer:             m.Issuer,
		Refund:             m.Refund,
	}

	if m.PaymentMethod != nil {
		st.PaymentMethod = &MgoOrderNotificationPaymentMethod{
			Id:            bson.ObjectIdHex(m.PaymentMethod.Id),
			Name:          m.PaymentMethod.Name,
			Params:        m.PaymentMethod.Params,
			PaymentSystem: m.PaymentMethod.PaymentSystem,
			Group:         m.PaymentMethod.Group,
		}
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	st.IdString = st.Id.Hex()

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	if m.ProjectLastRequestedAt != nil {
		t, err := ptypes.Timestamp(m.ProjectLastRequestedAt)

		if err != nil {
			return nil, err
		}

		st.ProjectLastRequestedAt = t
	}

	if m.PaymentMethodOrderClosedAt != nil {
		t, err := ptypes.Timestamp(m.PaymentMethodOrderClosedAt)

		if err != nil {
			return nil, err
		}

		st.PaymentMethodOrderClosedAt = t
	}

	if m.ExpireDateToFormInput != nil {
		t, err := ptypes.Timestamp(m.ExpireDateToFormInput)

		if err != nil {
			return nil, err
		}

		st.ExpireDateToFormInput = t
	} else {
		st.ExpireDateToFormInput = time.Now()
	}

	if m.CanceledAt != nil {
		t, err := ptypes.Timestamp(m.CanceledAt)

		if err != nil {
			return nil, err
		}

		st.CanceledAt = t
	}

	if m.RefundedAt != nil {
		t, err := ptypes.Timestamp(m.RefundedAt)

		if err != nil {
			return nil, err
		}

		st.RefundedAt = t
	}

	return st, nil
}

func (m *Order) SetBSON(raw bson.Raw) error {
	decoded := new(MgoOrder)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Project = &ProjectOrder{
		Id:                decoded.Project.Id.Hex(),
		MerchantId:        decoded.Project.MerchantId.Hex(),
		Name:              decoded.Project.Name,
		UrlSuccess:        decoded.Project.UrlSuccess,
		UrlFail:           decoded.Project.UrlFail,
		NotifyEmails:      decoded.Project.NotifyEmails,
		SendNotifyEmail:   decoded.Project.SendNotifyEmail,
		SecretKey:         decoded.Project.SecretKey,
		UrlCheckAccount:   decoded.Project.UrlCheckAccount,
		UrlProcessPayment: decoded.Project.UrlProcessPayment,
		CallbackProtocol:  decoded.Project.CallbackProtocol,
		Status:            decoded.Project.Status,
	}

	m.ProjectOrderId = decoded.ProjectOrderId
	m.ProjectAccount = decoded.ProjectAccount
	m.Description = decoded.Description
	m.ProjectIncomeAmount = decoded.ProjectIncomeAmount
	m.ProjectIncomeCurrency = decoded.ProjectIncomeCurrency
	m.ProjectOutcomeAmount = decoded.ProjectOutcomeAmount
	m.ProjectOutcomeCurrency = decoded.ProjectOutcomeCurrency
	m.ProjectParams = decoded.ProjectParams

	if decoded.PaymentMethod != nil {
		m.PaymentMethod = &PaymentMethodOrder{
			Id:            decoded.PaymentMethod.Id.Hex(),
			Name:          decoded.PaymentMethod.Name,
			Params:        decoded.PaymentMethod.Params,
			PaymentSystem: decoded.PaymentMethod.PaymentSystem,
			Group:         decoded.PaymentMethod.Group,
		}
	}

	m.PaymentMethodOrderId = decoded.PaymentMethodOrderId
	m.PaymentMethodOutcomeAmount = decoded.PaymentMethodOutcomeAmount
	m.PaymentMethodOutcomeCurrency = decoded.PaymentMethodOutcomeCurrency
	m.PaymentMethodIncomeAmount = decoded.PaymentMethodIncomeAmount
	m.PaymentMethodIncomeCurrency = decoded.PaymentMethodIncomeCurrency
	m.Status = decoded.Status
	m.IsJsonRequest = decoded.IsJsonRequest
	m.AmountInPspAccountingCurrency = decoded.AmountInPspAccountingCurrency
	m.AmountInMerchantAccountingCurrency = decoded.AmountInMerchantAccountingCurrency
	m.AmountOutMerchantAccountingCurrency = decoded.AmountOutMerchantAccountingCurrency
	m.AmountInPaymentSystemAccountingCurrency = decoded.AmountInPaymentSystemAccountingCurrency
	m.PaymentMethodPayerAccount = decoded.PaymentMethodPayerAccount
	m.PaymentMethodTxnParams = decoded.PaymentMethodTxnParams
	m.Products = decoded.Products
	m.Items = decoded.Items
	m.Amount = decoded.Amount
	m.Currency = decoded.Currency
	m.PaymentRequisites = decoded.PaymentRequisites
	m.PspFeeAmount = decoded.PspFeeAmount
	m.ProjectFeeAmount = decoded.ProjectFeeAmount
	m.ToPayerFeeAmount = decoded.ToPayerFeeAmount
	m.PaymentSystemFeeAmount = decoded.PaymentSystemFeeAmount

	m.Uuid = decoded.Uuid
	m.Tax = decoded.Tax
	m.TotalPaymentAmount = decoded.TotalPaymentAmount
	m.UserAddressDataRequired = decoded.UserAddressDataRequired
	m.BillingAddress = decoded.BillingAddress
	m.User = decoded.User

	m.CancellationReason = decoded.CancellationReason
	m.AgreementVersion = decoded.AgreementVersion
	m.AgreementAccepted = decoded.AgreementAccepted
	m.NotifySale = decoded.NotifySale
	m.NotifySaleEmail = decoded.NotifySaleEmail
	m.Issuer = decoded.Issuer
	m.Refund = decoded.Refund

	m.PaymentMethodOrderClosedAt, err = ptypes.TimestampProto(decoded.PaymentMethodOrderClosedAt)

	if err != nil {
		return err
	}

	m.ProjectLastRequestedAt, err = ptypes.TimestampProto(decoded.ProjectLastRequestedAt)

	if err != nil {
		return err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	m.CanceledAt, err = ptypes.TimestampProto(decoded.CanceledAt)

	if err != nil {
		return err
	}

	m.RefundedAt, err = ptypes.TimestampProto(decoded.RefundedAt)

	if err != nil {
		return err
	}

	m.ExpireDateToFormInput, err = ptypes.TimestampProto(decoded.ExpireDateToFormInput)

	if err != nil {
		return err
	}

	return nil
}

func (m *PaymentMethod) GetBSON() (interface{}, error) {
	st := &MgoPaymentMethod{
		Name:             m.Name,
		Group:            m.Group,
		Currency:         m.Currency,
		MinPaymentAmount: m.MinPaymentAmount,
		MaxPaymentAmount: m.MaxPaymentAmount,
		Params:           m.Params,
		Icon:             m.Icon,
		Currencies:       m.Currencies,
		Type:             m.Type,
		AccountRegexp:    m.AccountRegexp,
		IsActive:         m.IsActive,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.PaymentSystem != nil {
		st.PaymentSystem = &MgoPaymentSystem{
			Id:                 bson.ObjectIdHex(m.PaymentSystem.Id),
			Name:               m.PaymentSystem.Name,
			Country:            m.PaymentSystem.Country,
			AccountingCurrency: m.PaymentSystem.AccountingCurrency,
			AccountingPeriod:   m.PaymentSystem.AccountingPeriod,
			IsActive:           m.PaymentSystem.IsActive,
		}

		if m.PaymentSystem.CreatedAt != nil {
			t, err := ptypes.Timestamp(m.PaymentSystem.CreatedAt)

			if err != nil {
				return nil, err
			}

			st.PaymentSystem.CreatedAt = t
		} else {
			st.PaymentSystem.CreatedAt = time.Now()
		}

		if m.PaymentSystem.UpdatedAt != nil {
			t, err := ptypes.Timestamp(m.PaymentSystem.UpdatedAt)

			if err != nil {
				return nil, err
			}

			st.PaymentSystem.UpdatedAt = t
		} else {
			st.PaymentSystem.UpdatedAt = time.Now()
		}
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (m *PaymentMethod) SetBSON(raw bson.Raw) error {
	decoded := new(MgoPaymentMethod)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Name = decoded.Name
	m.Group = decoded.Group
	m.Currency = decoded.Currency
	m.Currencies = decoded.Currencies
	m.MinPaymentAmount = decoded.MinPaymentAmount
	m.MaxPaymentAmount = decoded.MaxPaymentAmount
	m.Params = decoded.Params
	m.Icon = decoded.Icon
	m.Type = decoded.Type
	m.AccountRegexp = decoded.AccountRegexp
	m.IsActive = decoded.IsActive

	if decoded.PaymentSystem != nil {
		m.PaymentSystem = &PaymentSystem{
			Id:                 decoded.PaymentSystem.Id.Hex(),
			Name:               decoded.PaymentSystem.Name,
			Country:            decoded.PaymentSystem.Country,
			AccountingCurrency: decoded.PaymentSystem.AccountingCurrency,
			AccountingPeriod:   decoded.PaymentSystem.AccountingPeriod,
			IsActive:           decoded.PaymentSystem.IsActive,
		}

		m.PaymentSystem.CreatedAt, err = ptypes.TimestampProto(decoded.PaymentSystem.CreatedAt)

		if err != nil {
			return err
		}

		m.PaymentSystem.UpdatedAt, err = ptypes.TimestampProto(decoded.PaymentSystem.UpdatedAt)

		if err != nil {
			return err
		}
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *PaymentSystem) GetBSON() (interface{}, error) {
	st := &MgoPaymentSystem{
		Name:               m.Name,
		Country:            m.Country,
		AccountingCurrency: m.AccountingCurrency,
		AccountingPeriod:   m.AccountingPeriod,
		IsActive:           m.IsActive,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (m *PaymentSystem) SetBSON(raw bson.Raw) error {
	decoded := new(MgoPaymentSystem)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Name = decoded.Name
	m.Country = decoded.Country
	m.AccountingCurrency = decoded.AccountingCurrency
	m.AccountingPeriod = decoded.AccountingPeriod
	m.IsActive = decoded.IsActive

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *Merchant) GetBSON() (interface{}, error) {
	st := &MgoMerchant{
		User:                      m.User,
		Name:                      m.Name,
		AlternativeName:           m.AlternativeName,
		Website:                   m.Website,
		Country:                   m.Country,
		State:                     m.State,
		Zip:                       m.Zip,
		City:                      m.City,
		Address:                   m.Address,
		AddressAdditional:         m.AddressAdditional,
		RegistrationNumber:        m.RegistrationNumber,
		TaxId:                     m.TaxId,
		Contacts:                  m.Contacts,
		Banking:                   m.Banking,
		Status:                    m.Status,
		IsVatEnabled:              m.IsVatEnabled,
		IsCommissionToUserEnabled: m.IsCommissionToUserEnabled,
		HasMerchantSignature:      m.HasMerchantSignature,
		HasPspSignature:           m.HasPspSignature,
		IsSigned:                  m.IsSigned,
		AgreementType:             m.AgreementType,
		AgreementSentViaMail:      m.AgreementSentViaMail,
		MailTrackingLink:          m.MailTrackingLink,
		S3AgreementName:           m.S3AgreementName,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.FirstPaymentAt != nil {
		t, err := ptypes.Timestamp(m.FirstPaymentAt)

		if err != nil {
			return nil, err
		}

		st.FirstPaymentAt = t
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	if m.LastPayout != nil {
		st.LastPayout = &MgoMerchantLastPayout{
			Amount: m.LastPayout.Amount,
		}

		t, err := ptypes.Timestamp(m.LastPayout.Date)

		if err != nil {
			return nil, err
		}

		st.LastPayout.Date = t
	}

	if len(m.PaymentMethods) > 0 {
		st.PaymentMethods = make(map[string]*MgoMerchantPaymentMethod, len(m.PaymentMethods))

		for k, v := range m.PaymentMethods {
			st.PaymentMethods[k] = &MgoMerchantPaymentMethod{
				PaymentMethod: &MgoMerchantPaymentMethodIdentification{
					Id:   bson.ObjectIdHex(v.PaymentMethod.Id),
					Name: v.PaymentMethod.Name,
				},
				Commission:  v.Commission,
				Integration: v.Integration,
				IsActive:    v.IsActive,
			}
		}
	}

	return st, nil
}

func (m *Merchant) SetBSON(raw bson.Raw) error {
	decoded := new(MgoMerchant)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.User = decoded.User
	m.Name = decoded.Name
	m.AlternativeName = decoded.AlternativeName
	m.Website = decoded.Website
	m.Country = decoded.Country
	m.State = decoded.State
	m.Zip = decoded.Zip
	m.City = decoded.City
	m.Address = decoded.Address
	m.AddressAdditional = decoded.AddressAdditional
	m.RegistrationNumber = decoded.RegistrationNumber
	m.TaxId = decoded.TaxId
	m.Contacts = decoded.Contacts
	m.Banking = decoded.Banking
	m.Status = decoded.Status
	m.IsVatEnabled = decoded.IsVatEnabled
	m.IsCommissionToUserEnabled = decoded.IsCommissionToUserEnabled
	m.HasMerchantSignature = decoded.HasMerchantSignature
	m.HasPspSignature = decoded.HasPspSignature
	m.IsSigned = decoded.IsSigned
	m.AgreementType = decoded.AgreementType
	m.AgreementSentViaMail = decoded.AgreementSentViaMail
	m.MailTrackingLink = decoded.MailTrackingLink
	m.S3AgreementName = decoded.S3AgreementName

	m.FirstPaymentAt, err = ptypes.TimestampProto(decoded.FirstPaymentAt)

	if err != nil {
		return err
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	if decoded.LastPayout != nil {
		m.LastPayout = &MerchantLastPayout{
			Amount: decoded.LastPayout.Amount,
		}

		m.LastPayout.Date, err = ptypes.TimestampProto(decoded.LastPayout.Date)

		if err != nil {
			return err
		}
	}

	if len(decoded.PaymentMethods) > 0 {
		m.PaymentMethods = make(map[string]*MerchantPaymentMethod, len(decoded.PaymentMethods))

		for k, v := range decoded.PaymentMethods {
			m.PaymentMethods[k] = &MerchantPaymentMethod{
				PaymentMethod: &MerchantPaymentMethodIdentification{},
				Commission:    v.Commission,
				Integration:   v.Integration,
				IsActive:      v.IsActive,
			}

			if v.PaymentMethod != nil {
				m.PaymentMethods[k].PaymentMethod.Id = v.PaymentMethod.Id.Hex()
				m.PaymentMethods[k].PaymentMethod.Name = v.PaymentMethod.Name
			}
		}
	}

	return nil
}

func (m *Notification) GetBSON() (interface{}, error) {
	st := &MgoNotification{
		Title:      m.Title,
		Message:    m.Message,
		IsSystem:   m.IsSystem,
		IsRead:     m.IsRead,
		MerchantId: bson.ObjectIdHex(m.MerchantId),
		UserId:     bson.ObjectIdHex(m.UserId),
		Statuses:   m.Statuses,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (m *Notification) SetBSON(raw bson.Raw) error {
	decoded := new(MgoNotification)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Title = decoded.Title
	m.Message = decoded.Message
	m.IsSystem = decoded.IsSystem
	m.IsRead = decoded.IsRead
	m.MerchantId = decoded.MerchantId.Hex()
	m.UserId = decoded.UserId.Hex()
	m.Statuses = decoded.Statuses

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *Refund) GetBSON() (interface{}, error) {
	st := &MgoRefund{
		Order: &MgoRefundOrder{
			Id:   bson.ObjectIdHex(m.Order.Id),
			Uuid: m.Order.Uuid,
		},
		ExternalId: m.ExternalId,
		Amount:     m.Amount,
		CreatorId:  bson.ObjectIdHex(m.CreatorId),
		Currency:   m.Currency,
		Status:     m.Status,
		PayerData:  m.PayerData,
		SalesTax:   m.SalesTax,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (m *Refund) SetBSON(raw bson.Raw) error {
	decoded := new(MgoRefund)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.Order = &RefundOrder{
		Id:   decoded.Order.Id.Hex(),
		Uuid: decoded.Order.Uuid,
	}
	m.ExternalId = decoded.ExternalId
	m.Amount = decoded.Amount
	m.CreatorId = decoded.CreatorId.Hex()
	m.Currency = decoded.Currency
	m.Status = decoded.Status
	m.PayerData = decoded.PayerData
	m.SalesTax = decoded.SalesTax

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}

func (m *PaymentFormPaymentMethod) IsBankCard() bool {
	return m.Group == constant.PaymentSystemGroupAliasBankCard
}

func (m *PaymentMethod) IsBankCard() bool {
	return m.Group == constant.PaymentSystemGroupAliasBankCard
}

func (m *PaymentMethodOrder) IsBankCard() bool {
	return m.Group == constant.PaymentSystemGroupAliasBankCard
}

func (m *PaymentMethodOrder) IsCryptoCurrency() bool {
	return m.Group == constant.PaymentSystemGroupAliasBitcoin
}

func (m *MerchantPaymentMethodHistory) SetBSON(raw bson.Raw) error {
	decoded := new(MgoMerchantPaymentMethodHistory)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.MerchantId = decoded.MerchantId.Hex()
	m.UserId = decoded.UserId.Hex()
	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)
	if err != nil {
		return err
	}

	m.PaymentMethod = &MerchantPaymentMethod{
		PaymentMethod: &MerchantPaymentMethodIdentification{
			Id:   bson.ObjectId(decoded.PaymentMethod.PaymentMethod.Id).Hex(),
			Name: decoded.PaymentMethod.PaymentMethod.Name,
		},
		Commission:  decoded.PaymentMethod.Commission,
		Integration: decoded.PaymentMethod.Integration,
		IsActive:    decoded.PaymentMethod.IsActive,
	}

	return nil
}

func (p *MerchantPaymentMethodHistory) GetBSON() (interface{}, error) {
	st := &MgoMerchantPaymentMethodHistory{}

	if len(p.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(p.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(p.Id)
	}

	if len(p.MerchantId) <= 0 {
		return nil, errors.New(errorInvalidObjectId)
	} else {
		if bson.IsObjectIdHex(p.MerchantId) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.MerchantId = bson.ObjectIdHex(p.MerchantId)
	}

	if len(p.UserId) <= 0 {
		return nil, errors.New(errorInvalidObjectId)
	} else {
		if bson.IsObjectIdHex(p.UserId) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.UserId = bson.ObjectIdHex(p.UserId)
	}

	if p.CreatedAt != nil {
		t, err := ptypes.Timestamp(p.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	st.PaymentMethod = &MgoMerchantPaymentMethod{
		PaymentMethod: &MgoMerchantPaymentMethodIdentification{
			Id:   bson.ObjectIdHex(p.PaymentMethod.PaymentMethod.Id),
			Name: p.PaymentMethod.PaymentMethod.Name,
		},
		Commission:  p.PaymentMethod.Commission,
		Integration: p.PaymentMethod.Integration,
		IsActive:    p.PaymentMethod.IsActive,
	}

	return st, nil
}

func (m *Customer) GetBSON() (interface{}, error) {
	st := &MgoCustomer{
		Id:                    bson.ObjectIdHex(m.Id),
		TechEmail:             m.TechEmail,
		ExternalId:            m.ExternalId,
		Email:                 m.Email,
		EmailVerified:         m.EmailVerified,
		Phone:                 m.Phone,
		PhoneVerified:         m.PhoneVerified,
		Name:                  m.Name,
		Ip:                    m.Ip,
		Locale:                m.Locale,
		AcceptLanguage:        m.AcceptLanguage,
		UserAgent:             m.UserAgent,
		Address:               m.Address,
		Metadata:              m.Metadata,
		Identity:              []*MgoCustomerIdentity{},
		IpHistory:             []*MgoCustomerIpHistory{},
		AddressHistory:        []*MgoCustomerAddressHistory{},
		LocaleHistory:         []*MgoCustomerStringValueHistory{},
		AcceptLanguageHistory: []*MgoCustomerStringValueHistory{},
	}

	for _, v := range m.Identity {
		mgoIdentity := &MgoCustomerIdentity{
			MerchantId: bson.ObjectIdHex(v.MerchantId),
			ProjectId:  bson.ObjectIdHex(v.ProjectId),
			Type:       v.Type,
			Value:      v.Value,
			Verified:   v.Verified,
		}

		mgoIdentity.CreatedAt, _ = ptypes.Timestamp(v.CreatedAt)
		st.Identity = append(st.Identity, mgoIdentity)
	}

	for _, v := range m.IpHistory {
		mgoIdentity := &MgoCustomerIpHistory{Ip: v.Ip}
		mgoIdentity.CreatedAt, _ = ptypes.Timestamp(v.CreatedAt)
		st.IpHistory = append(st.IpHistory, mgoIdentity)
	}

	for _, v := range m.AddressHistory {
		mgoIdentity := &MgoCustomerAddressHistory{
			Country:    v.Country,
			City:       v.City,
			PostalCode: v.PostalCode,
			State:      v.State,
		}
		mgoIdentity.CreatedAt, _ = ptypes.Timestamp(v.CreatedAt)
		st.AddressHistory = append(st.AddressHistory, mgoIdentity)
	}

	for _, v := range m.LocaleHistory {
		mgoIdentity := &MgoCustomerStringValueHistory{Value: v.Value}
		mgoIdentity.CreatedAt, _ = ptypes.Timestamp(v.CreatedAt)
		st.LocaleHistory = append(st.LocaleHistory, mgoIdentity)
	}

	for _, v := range m.AcceptLanguageHistory {
		mgoIdentity := &MgoCustomerStringValueHistory{Value: v.Value}
		mgoIdentity.CreatedAt, _ = ptypes.Timestamp(v.CreatedAt)
		st.AcceptLanguageHistory = append(st.AcceptLanguageHistory, mgoIdentity)
	}

	if m.CreatedAt != nil {
		t, err := ptypes.Timestamp(m.CreatedAt)

		if err != nil {
			return nil, err
		}

		st.CreatedAt = t
	} else {
		st.CreatedAt = time.Now()
	}

	if m.UpdatedAt != nil {
		t, err := ptypes.Timestamp(m.UpdatedAt)

		if err != nil {
			return nil, err
		}

		st.UpdatedAt = t
	} else {
		st.UpdatedAt = time.Now()
	}

	return st, nil
}

func (m *Customer) SetBSON(raw bson.Raw) error {
	decoded := new(MgoCustomer)
	err := raw.Unmarshal(decoded)

	if err != nil {
		return err
	}

	m.Id = decoded.Id.Hex()
	m.TechEmail = decoded.TechEmail
	m.ExternalId = decoded.ExternalId
	m.Email = decoded.Email
	m.EmailVerified = decoded.EmailVerified
	m.Phone = decoded.Phone
	m.PhoneVerified = decoded.PhoneVerified
	m.Name = decoded.Name
	m.Ip = decoded.Ip
	m.Locale = decoded.Locale
	m.AcceptLanguage = decoded.AcceptLanguage
	m.UserAgent = decoded.UserAgent
	m.Address = decoded.Address
	m.Identity = []*CustomerIdentity{}
	m.IpHistory = []*CustomerIpHistory{}
	m.AddressHistory = []*CustomerAddressHistory{}
	m.LocaleHistory = []*CustomerStringValueHistory{}
	m.AcceptLanguageHistory = []*CustomerStringValueHistory{}
	m.Metadata = decoded.Metadata

	for _, v := range decoded.Identity {
		identity := &CustomerIdentity{
			MerchantId: v.MerchantId.Hex(),
			ProjectId:  v.ProjectId.Hex(),
			Type:       v.Type,
			Value:      v.Value,
			Verified:   v.Verified,
		}

		identity.CreatedAt, _ = ptypes.TimestampProto(v.CreatedAt)
		m.Identity = append(m.Identity, identity)
	}

	for _, v := range decoded.IpHistory {
		identity := &CustomerIpHistory{Ip: v.Ip}
		identity.CreatedAt, _ = ptypes.TimestampProto(v.CreatedAt)
		m.IpHistory = append(m.IpHistory, identity)
	}

	for _, v := range decoded.AddressHistory {
		identity := &CustomerAddressHistory{
			Country:    v.Country,
			City:       v.City,
			PostalCode: v.PostalCode,
			State:      v.State,
		}
		identity.CreatedAt, _ = ptypes.TimestampProto(v.CreatedAt)
		m.AddressHistory = append(m.AddressHistory, identity)
	}

	for _, v := range decoded.LocaleHistory {
		identity := &CustomerStringValueHistory{Value: v.Value}
		identity.CreatedAt, _ = ptypes.TimestampProto(v.CreatedAt)
		m.LocaleHistory = append(m.LocaleHistory, identity)
	}

	for _, v := range decoded.AcceptLanguageHistory {
		identity := &CustomerStringValueHistory{Value: v.Value}
		identity.CreatedAt, _ = ptypes.TimestampProto(v.CreatedAt)
		m.AcceptLanguageHistory = append(m.AcceptLanguageHistory, identity)
	}

	m.CreatedAt, err = ptypes.TimestampProto(decoded.CreatedAt)

	if err != nil {
		return err
	}

	m.UpdatedAt, err = ptypes.TimestampProto(decoded.UpdatedAt)

	if err != nil {
		return err
	}

	return nil
}
