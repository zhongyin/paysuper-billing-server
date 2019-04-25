package billing

import (
	"github.com/globalsign/mgo/bson"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
	"strconv"
	"time"
)

var (
	orderRefundAllowedStatuses = map[int32]bool{
		constant.OrderStatusPaymentSystemComplete: true,
		constant.OrderStatusProjectInProgress:     true,
		constant.OrderStatusProjectComplete:       true,
		constant.OrderStatusProjectPending:        true,
	}

	orderStatusPublicMapping = map[int32]string{
		constant.OrderStatusNew:                         constant.OrderPublicStatusCreated,
		constant.OrderStatusPaymentSystemCreate:         constant.OrderPublicStatusCreated,
		constant.OrderStatusPaymentSystemCanceled:       constant.OrderPublicStatusCanceled,
		constant.OrderStatusPaymentSystemRejectOnCreate: constant.OrderPublicStatusRejected,
		constant.OrderStatusPaymentSystemReject:         constant.OrderPublicStatusRejected,
		constant.OrderStatusProjectReject:               constant.OrderPublicStatusRejected,
		constant.OrderStatusPaymentSystemDeclined:       constant.OrderPublicStatusRejected,
		constant.OrderStatusPaymentSystemComplete:       constant.OrderPublicStatusProcessed,
		constant.OrderStatusProjectComplete:             constant.OrderPublicStatusProcessed,
		constant.OrderStatusRefund:                      constant.OrderPublicStatusRefunded,
		constant.OrderStatusChargeback:                  constant.OrderPublicStatusChargeback,
	}

	bankCardBrandUnknown = "unknown or unsupported credit card brand"
)

type digits [6]int

// At returns the digits from the start to the given length
func (d *digits) At(i int) int {
	return d[i-1]
}

func getCardBrand(pan string) (string, error) {
	ccLen := len(pan)
	ccDigits := digits{}

	for i := 0; i < 6; i++ {
		if i < ccLen {
			ccDigits[i], _ = strconv.Atoi(pan[:i+1])
		}
	}

	switch {
	case ccDigits.At(2) == 62:
		return "UNIONPAY", nil
	case ccDigits.At(4) >= 3528 && ccDigits.At(4) <= 3589:
		return "JCB", nil
	case ccDigits.At(2) >= 51 && ccDigits.At(2) <= 55:
		return "MASTERCARD", nil
	case ccDigits.At(1) == 4:
		return "VISA", nil
	default:
		return "", errors.New(bankCardBrandUnknown)
	}
}

func (m *Merchant) ChangesAllowed() bool {
	return m.Status == pkg.MerchantStatusDraft
}

func (m *Merchant) GetPayoutCurrency() *Currency {
	if m.Banking == nil {
		return nil
	}

	return m.Banking.Currency
}

func (m *Merchant) NeedMarkESignAgreementAsSigned() bool {
	return m.HasMerchantSignature == true && m.HasPspSignature == true &&
		m.Status != pkg.MerchantStatusAgreementSigned
}

func (m *Merchant) CanGenerateAgreement() bool {
	return (m.Status == pkg.MerchantStatusOnReview || m.Status == pkg.MerchantStatusAgreementSigning ||
		m.Status == pkg.MerchantStatusAgreementSigned) && m.Banking != nil && m.Country != nil &&
		m.Contacts != nil && m.Contacts.Authorized != nil
}

func (m *Merchant) CanChangeStatusToSigning() bool {
	return m.Status == pkg.MerchantStatusOnReview && m.Banking != nil && m.Country != nil &&
		m.Contacts != nil && m.Contacts.Authorized != nil
}

func (m *Merchant) IsDeleted() bool {
	return m.Status == pkg.MerchantStatusDeleted
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

func (m *Order) FormInputTimeIsEnded() bool {
	t, err := ptypes.Timestamp(m.ExpireDateToFormInput)

	return err != nil || t.Before(time.Now())
}

func (m *Project) IsProduction() bool {
	return m.Status == pkg.ProjectStatusInProduction
}

func (m *Project) IsDeleted() bool {
	return m.Status == pkg.ProjectStatusDeleted
}

func (m *Project) NeedChangeStatusToDraft(req *Project) bool {
	if m.Status != pkg.ProjectStatusTestCompleted &&
		m.Status != pkg.ProjectStatusInProduction {
		return false
	}

	if m.CallbackProtocol == pkg.ProjectCallbackProtocolEmpty &&
		req.CallbackProtocol == pkg.ProjectCallbackProtocolDefault {
		return true
	}

	if req.UrlCheckAccount != "" &&
		req.UrlCheckAccount != m.UrlCheckAccount {
		return true
	}

	if req.UrlProcessPayment != "" &&
		req.UrlProcessPayment != m.UrlProcessPayment {
		return true
	}

	return false
}

func (m *OrderUser) IsIdentified() bool {
	return m.Id != "" && bson.IsObjectIdHex(m.Id) == true
}

func (m *Order) GetPublicStatus() string {
	st, ok := orderStatusPublicMapping[m.Status]
	if !ok {
		return constant.OrderPublicStatusPending
	}
	return st
}

func (m *Order) GetOrderNotification() (*OrderNotification, error) {

	pf := &CalculatedFeeItem{
		Amount:        m.ProjectFeeAmount.AmountMerchantCurrency,
		Currency:      m.ProjectFeeAmount.MerchantCurrencyA3,
		EffectiveRate: m.ProjectFeeAmount.EffectiveRateMerchantCurrency,
	}

	method := &OrderNotificationPaymentMethod{
		Type:  m.PaymentMethod.Group,
		Title: m.PaymentMethod.Name,
		Saved: m.PaymentMethodTxnParams["saved"] == "1",
		// Fee: todo: add fee after system fees applies will be implemented
	}

	if m.PaymentMethod.IsBankCard() {

		secure3D := false
		if is3ds, ok := m.PaymentMethodTxnParams["is_3ds"]; ok {
			secure3D = is3ds == "1"
		}

		first6 := ""
		last4 := ""
		pan, ok := m.PaymentMethodTxnParams["pan"]
		if !ok {
			pan, ok = m.PaymentRequisites["pan"]
			if !ok {
				pan = ""
			}
		}
		if pan != "" {
			first6 = string(pan[0:6])
			last4 = string(pan[len(pan)-4:])
		}
		month := m.PaymentRequisites["month"]
		year := m.PaymentRequisites["year"]

		cardBrand, _ := getCardBrand(pan)

		method.Card = &OrderNotificationPaymentMethodCard{
			Masked:      pan,
			First6:      first6,
			Last4:       last4,
			ExpiryMonth: month,
			ExpiryYear:  year,
			Brand:       cardBrand,
			Secure3D:    secure3D,
		}

		b, err := json.Marshal(method.Card)
		if err != nil {
			h := sha256.New()
			h.Write([]byte(string(b)))
			method.Card.Fingerprint = hex.EncodeToString(h.Sum(nil))
		}

	} else {
		if m.PaymentMethod.IsCryptoCurrency() {
			method.CryptoCurrency = &OrderNotificationPaymentMethodCrypto{
				Address: m.PaymentRequisites["address"],
				Brand:   m.PaymentMethod.Name,
			}
		} else {
			method.Wallet = &OrderNotificationPaymentMethodWallet{
				Account: m.PaymentRequisites["ewallet"],
				Brand:   m.PaymentMethod.Name,
			}
		}
	}

	on := &OrderNotification{
		Id:                 m.Uuid,
		Transaction:        m.PaymentMethodOrderId,
		Object:             "order",
		Status:             m.GetPublicStatus(),
		Description:        m.Description,
		CreatedAt:          m.CreatedAt,
		UpdatedAt:          m.UpdatedAt,
		Amount:             m.TotalPaymentAmount,
		Currency:           m.Currency,
		Items:              m.Items,
		ReceiptEmail:       m.PayerData.Email,
		ReceiptPhone:       m.PayerData.Phone,
		ReceiptNumber:      m.PayerData.ReceiptNumber,
		ReceiptUrl:         m.PayerData.ReceiptUrl,
		AgreementVersion:   m.AgreementVersion,
		AgreementAccepted:  m.AgreementAccepted,
		NotifySale:         m.NotifySale,
		NotifySaleEmail:    m.NotifySaleEmail,
		Issuer:             m.Issuer,
		User:               m.User,
		PlatformFee:        pf,
		BillingAddress:     m.BillingAddress,
		Tax:                m.Tax,
		Method:             method,
		Canceled:           m.Status == constant.OrderStatusPaymentSystemCanceled,
		CanceledAt:         m.CanceledAt,
		CancellationReason: m.CancellationReason,
		Refunded:           m.Status == constant.OrderStatusRefund,
		RefundedAt:         m.RefundedAt,
		Refund:             m.Refund,
		Metadata:           m.Metadata,
	}

	return on, nil
}

func (m *Order) GetCountry() string {
	if m.BillingAddress != nil && m.BillingAddress.Country != "" {
		return m.BillingAddress.Country
	}
	if m.User != nil && m.User.Address != nil && m.User.Address.Country != "" {
		return m.User.Address.Country
	}
	if m.PayerData != nil && m.PayerData.Country != "" {
		return m.PayerData.Country
	}
	return ""
}

func (m *Order) GetState() string {
	if m.BillingAddress != nil && m.BillingAddress.State != "" {
		return m.BillingAddress.State
	}
	if m.User != nil && m.User.Address != nil && m.User.Address.State != "" {
		return m.User.Address.State
	}
	if m.PayerData != nil && m.PayerData.State != "" {
		return m.PayerData.State
	}
	return ""
}
