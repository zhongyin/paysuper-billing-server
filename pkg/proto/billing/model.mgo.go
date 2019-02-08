package billing

import (
	"errors"
	"fmt"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"time"
)

const (
	errorInvalidObjectId = "invalid bson object id"
	errorRequiredField   = "field \"%s\" is required to convert object %s"
)

type MgoVat struct {
	Id          bson.ObjectId `bson:"_id" json:"id"`
	Country     string        `bson:"country" json:"country"`
	Subdivision string        `bson:"subdivision_code" json:"subdivision_code,omitempty"`
	Vat         float64       `bson:"vat" json:"vat"`
	IsActive    bool          `bson:"is_active" json:"is_active"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updated_at,omitempty"`
}

type MgoProject struct {
	Id                       bson.ObjectId                    `bson:"_id" json:"id"`
	CallbackCurrency         *Currency                        `bson:"callback_currency" json:"callback_currency"`
	CallbackProtocol         string                           `bson:"callback_protocol" json:"callback_protocol"`
	CreateInvoiceAllowedUrls []string                         `bson:"create_invoice_allowed_urls" json:"create_invoice_allowed_urls"`
	Merchant                 *Merchant                        `bson:"merchant" json:"-"`
	AllowDynamicNotifyUrls   bool                             `bson:"is_allow_dynamic_notify_urls" json:"allow_dynamic_notify_urls"`
	AllowDynamicRedirectUrls bool                             `bson:"is_allow_dynamic_redirect_urls" json:"allow_dynamic_redirect_urls"`
	LimitsCurrency           *Currency                        `bson:"limits_currency" json:"limits_currency"`
	MaxPaymentAmount         float64                          `bson:"max_payment_amount" json:"max_payment_amount"`
	MinPaymentAmount         float64                          `bson:"min_payment_amount" json:"min_payment_amount"`
	Name                     string                           `bson:"name" json:"name"`
	NotifyEmails             []string                         `bson:"notify_emails" json:"notify_emails"`
	OnlyFixedAmounts         bool                             `bson:"only_fixed_amounts" json:"only_fixed_amounts"`
	SecretKey                string                           `bson:"secret_key" json:"secret_key"`
	SendNotifyEmail          bool                             `bson:"send_notify_email" json:"send_notify_email"`
	UrlCheckAccount          string                           `bson:"url_check_account" json:"url_check_account"`
	UrlProcessPayment        string                           `bson:"url_process_payment" json:"url_process_payment"`
	UrlRedirectFail          string                           `bson:"url_redirect_fail" json:"url_redirect_fail"`
	UrlRedirectSuccess       string                           `bson:"url_redirect_success" json:"url_redirect_success"`
	IsActive                 bool                             `bson:"is_active" json:"is_active"`
	CreatedAt                time.Time                        `bson:"created_at" json:"created_at"`
	UpdatedAt                time.Time                        `bson:"updated_at" json:"-"`
	FixedPackage             map[string][]*FixedPackage       `bson:"fixed_package" json:"fixed_package,omitempty"`
	PaymentMethods           map[string]*ProjectPaymentMethod `bson:"payment_methods" json:"payment_methods,omitempty"`
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

func (m *Project) GetBSON() (interface{}, error) {
	st := &MgoProject{
		CallbackCurrency:         m.CallbackCurrency,
		CreateInvoiceAllowedUrls: m.CreateInvoiceAllowedUrls,
		Merchant:                 m.Merchant,
		AllowDynamicNotifyUrls:   m.AllowDynamicNotifyUrls,
		AllowDynamicRedirectUrls: m.AllowDynamicRedirectUrls,
		LimitsCurrency:           m.LimitsCurrency,
		MaxPaymentAmount:         m.MaxPaymentAmount,
		MinPaymentAmount:         m.MinPaymentAmount,
		Name:                     m.Name,
		NotifyEmails:             m.NotifyEmails,
		OnlyFixedAmounts:         m.OnlyFixedAmounts,
		SecretKey:                m.SecretKey,
		SendNotifyEmail:          m.SendNotifyEmail,
		UrlCheckAccount:          m.UrlCheckAccount,
		UrlProcessPayment:        m.UrlProcessPayment,
		UrlRedirectFail:          m.UrlRedirectFail,
		UrlRedirectSuccess:       m.UrlRedirectSuccess,
		IsActive:                 m.IsActive,
		PaymentMethods:           m.PaymentMethods,
	}

	if len(m.Id) <= 0 {
		st.Id = bson.NewObjectId()
	} else {
		if bson.IsObjectIdHex(m.Id) == false {
			return nil, errors.New(errorInvalidObjectId)
		}

		st.Id = bson.ObjectIdHex(m.Id)
	}

	if len(m.CallbackProtocol) <= 0 {
		st.CallbackProtocol = "default"
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

	if len(m.FixedPackage) > 0 {
		fps := make(map[string][]*FixedPackage)

		for c, ofp := range m.FixedPackage {
			fps[c] = ofp.FixedPackage
		}

		st.FixedPackage = fps
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
	m.CallbackCurrency = decoded.CallbackCurrency
	m.CallbackProtocol = decoded.CallbackProtocol
	m.CreateInvoiceAllowedUrls = decoded.CreateInvoiceAllowedUrls
	m.Merchant = decoded.Merchant
	m.AllowDynamicNotifyUrls = decoded.AllowDynamicNotifyUrls
	m.AllowDynamicRedirectUrls = decoded.AllowDynamicRedirectUrls
	m.LimitsCurrency = decoded.LimitsCurrency
	m.MaxPaymentAmount = decoded.MaxPaymentAmount
	m.MinPaymentAmount = decoded.MinPaymentAmount
	m.Name = decoded.Name
	m.NotifyEmails = decoded.NotifyEmails
	m.OnlyFixedAmounts = decoded.OnlyFixedAmounts
	m.SecretKey = decoded.SecretKey
	m.SendNotifyEmail = decoded.SendNotifyEmail
	m.UrlCheckAccount = decoded.UrlCheckAccount
	m.UrlProcessPayment = decoded.UrlProcessPayment
	m.UrlRedirectFail = decoded.UrlRedirectFail
	m.UrlRedirectSuccess = decoded.UrlRedirectSuccess
	m.PaymentMethods = decoded.PaymentMethods
	m.IsActive = decoded.IsActive

	if len(decoded.FixedPackage) > 0 {
		fps := make(map[string]*FixedPackages)

		for c, fp := range decoded.FixedPackage {
			fps[c] = &FixedPackages{FixedPackage: fp}
		}

		m.FixedPackage = fps
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
