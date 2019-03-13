package service

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
	"github.com/paysuper/paysuper-recurring-repository/tools"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cardPayRequestFieldGrantType    = "grant_type"
	cardPayRequestFieldTerminalCode = "terminal_code"
	cardPayRequestFieldPassword     = "password"
	cardPayRequestFieldRefreshToken = "refresh_token"

	cardPayGrantTypePassword     = "password"
	cardPayGrantTypeRefreshToken = "refresh_token"

	cardPayActionAuthenticate     = "auth"
	cardPayActionRefresh          = "refresh"
	cardPayActionCreatePayment    = "create_payment"
	cardPayActionRecurringPayment = "recurring_payment"
	cardPayActionRefund           = "refund"

	cardPayDateFormat          = "2006-01-02T15:04:05Z"
	cardPayInitiatorCardholder = "cit"
)

var (
	cardPayTokens = map[string]*cardPayToken{}
	cardPayPaths  = map[string]*Path{
		cardPayActionAuthenticate: {
			path:   "/api/auth/token",
			method: http.MethodPost,
		},
		cardPayActionRefresh: {
			path:   "/api/auth/token",
			method: http.MethodPost,
		},
		cardPayActionCreatePayment: {
			path:   "/api/payments",
			method: http.MethodPost,
		},
		cardPayActionRecurringPayment: {
			path:   "/api/recurrings",
			method: http.MethodPost,
		},
		cardPayActionRefund: {
			path:   "/api/refunds",
			method: http.MethodPost,
		},
	}

	successRefundResponseStatuses = map[string]bool{
		pkg.CardPayPaymentResponseStatusAuthorized: true,
		pkg.CardPayPaymentResponseStatusInProgress: true,
		pkg.CardPayPaymentResponseStatusPending:    true,
		pkg.CardPayPaymentResponseStatusRefunded:   true,
		pkg.CardPayPaymentResponseStatusCompleted:  true,
	}
)

type cardPay struct {
	processor *paymentProcessor
	mu        sync.Mutex
}

type cardPayTransport struct {
	Transport http.RoundTripper
	processor *paymentProcessor
}

type cardPayContextKey struct {
	name string
}

type cardPayToken struct {
	TokenType              string `json:"token_type"`
	AccessToken            string `json:"access_token"`
	RefreshToken           string `json:"refresh_token"`
	AccessTokenExpire      int    `json:"expires_in"`
	RefreshTokenExpire     int    `json:"refresh_expires_in"`
	AccessTokenExpireTime  time.Time
	RefreshTokenExpireTime time.Time
}

type CardPayBankCardAccount struct {
	Pan        string `json:"pan"`
	HolderName string `json:"holder"`
	Cvv        string `json:"security_code"`
	Expire     string `json:"expiration"`
}

type CardPayEWalletAccount struct {
	Id string `json:"id"`
}

type CardPayRecurringDataFiling struct {
	Id string `json:"id"`
}

type CardPayPaymentData struct {
	Currency   string  `json:"currency"`
	Amount     float64 `json:"amount"`
	Descriptor string  `json:"dynamic_descriptor"`
	Note       string  `json:"note"`
}

type CardPayRecurringData struct {
	Currency   string                      `json:"currency"`
	Amount     float64                     `json:"amount"`
	Filing     *CardPayRecurringDataFiling `json:"filing,omitempty"`
	Descriptor string                      `json:"dynamic_descriptor"`
	Note       string                      `json:"note"`
	Initiator  string                      `json:"initiator"`
}

type CardPayCustomer struct {
	Email   string `json:"email"`
	Ip      string `json:"ip"`
	Account string `json:"id"`
}

type CardPayItem struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Count       int     `json:"count"`
	Price       float64 `json:"price"`
}

type CardPayRequest struct {
	Id   string `json:"id"`
	Time string `json:"time"`
}

type CardPayAddress struct {
	Country string `json:"country"`
	City    string `json:"city,omitempty"`
	Phone   string `json:"phone,omitempty"`
	State   string `json:"state,omitempty"`
	Street  string `json:",omitempty"`
	Zip     string `json:"zip,omitempty"`
}

type CardPayMerchantOrder struct {
	Id              string          `json:"id" validate:"required,hexadecimal"`
	Description     string          `json:"description,omitempty"`
	Items           []*CardPayItem  `json:"items,omitempty"`
	ShippingAddress *CardPayAddress `json:"shipping_address,omitempty"`
}

type CardPayCardAccount struct {
	BillingAddress *CardPayAddress         `json:"billing_address,omitempty"`
	Card           *CardPayBankCardAccount `json:"card"`
	Token          string                  `json:"token,omitempty"`
}

type CardPayCryptoCurrencyAccount struct {
	RollbackAddress string `json:"rollback_address"`
}

type CardPayReturnUrls struct {
	CancelUrl  string `json:"cancel_url,omitempty"`
	DeclineUrl string `json:"decline_url,omitempty"`
	SuccessUrl string `json:"success_url,omitempty"`
}

type CardPayOrder struct {
	Request               *CardPayRequest               `json:"request"`
	MerchantOrder         *CardPayMerchantOrder         `json:"merchant_order"`
	Description           string                        `json:"description"`
	PaymentMethod         string                        `json:"payment_method"`
	PaymentData           *CardPayPaymentData           `json:"payment_data,omitempty"`
	RecurringData         *CardPayRecurringData         `json:"recurring_data,omitempty"`
	CardAccount           *CardPayCardAccount           `json:"card_account,omitempty"`
	Customer              *CardPayCustomer              `json:"customer"`
	EWalletAccount        *CardPayEWalletAccount        `json:"ewallet_account,omitempty"`
	CryptoCurrencyAccount *CardPayCryptoCurrencyAccount `json:"cryptocurrency_account,omitempty"`
	ReturnUrls            *CardPayReturnUrls            `json:"return_urls,omitempty"`
}

type CardPayOrderResponse struct {
	RedirectUrl string `json:"redirect_url"`
}

type CardPayRefundData struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type CardPayRefundRequest struct {
	Request       *CardPayRequest             `json:"request"`
	MerchantOrder *CardPayMerchantOrder       `json:"merchant_order"`
	PaymentData   *CardPayRecurringDataFiling `json:"payment_data"`
	RefundData    *CardPayRefundData          `json:"refund_data"`
}

type CardPayRefundResponseRefundData struct {
	Id       string  `json:"id"`
	Created  string  `json:"created"`
	Status   string  `json:"status"`
	AuthCode string  `json:"auth_code"`
	Is3d     bool    `json:"is_3d"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type CardPayRefundResponsePaymentData struct {
	Id              string  `json:"id"`
	RemainingAmount float64 `json:"remaining_amount"`
}

type CardPayRefundResponseCustomer struct {
	Id    string `json:"id"`
	Email string `json:"email"`
}

type CardPayRefundResponse struct {
	PaymentMethod  string                            `json:"payment_method"`
	MerchantOrder  *CardPayMerchantOrder             `json:"merchant_order"`
	RefundData     *CardPayRefundResponseRefundData  `json:"refund_data"`
	PaymentData    *CardPayRefundResponsePaymentData `json:"payment_data"`
	Customer       *CardPayRefundResponseCustomer    `json:"customer"`
	CardAccount    interface{}                       `json:"card_account,omitempty"`
	EwalletAccount interface{}                       `json:"ewallet_account,omitempty"`
}

func (m *CardPayRefundResponse) IsSuccessStatus() bool {
	v, ok := successRefundResponseStatuses[m.RefundData.Status]
	return ok && v == true
}

func newCardPayHandler(processor *paymentProcessor) PaymentSystem {
	return &cardPay{processor: processor}
}

func (h *cardPay) CreatePayment(requisites map[string]string) (url string, err error) {
	if err = h.auth(h.processor.order.PaymentMethod.Params.ExternalId); err != nil {
		return
	}

	cpOrder, err := h.getCardPayOrder(h.processor.order, requisites)

	if err != nil {
		return
	}

	action := cardPayActionCreatePayment

	if cpOrder.RecurringData != nil {
		action = cardPayActionRecurringPayment
	}

	qUrl, err := h.getUrl(action)

	if err != nil {
		return
	}

	h.processor.order.Status = constant.OrderStatusPaymentSystemRejectOnCreate

	b, _ := json.Marshal(cpOrder)

	client := &http.Client{
		Transport: &cardPayTransport{},
		Timeout:   time.Duration(defaultHttpClientTimeout * time.Second),
	}

	req, err := http.NewRequest(cardPayPaths[action].method, qUrl, bytes.NewBuffer(b))

	token := h.getToken(h.processor.order.PaymentMethod.Params.ExternalId)
	auth := strings.Title(token.TokenType) + " " + token.AccessToken

	req.Header.Add(HeaderContentType, MIMEApplicationJSON)
	req.Header.Add(HeaderAuthorization, auth)

	resp, err := client.Do(req)

	if err != nil {
		zap.L().Error(
			fmt.Sprintf("[PAYONE_BILLING] %s", "CardPay create payment failer"),
			zap.Error(err),
			zap.Any("request", cpOrder),
		)
	}

	if err != nil || resp.StatusCode != http.StatusOK {
		return "", errors.New(paymentSystemErrorCreateRequestFailed)
	}

	if b, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	cpResponse := &CardPayOrderResponse{}

	if err = json.Unmarshal(b, &cpResponse); err != nil {
		return
	}

	h.processor.order.Status = constant.OrderStatusPaymentSystemCreate
	url = cpResponse.RedirectUrl

	return
}

func (h *cardPay) ProcessPayment(message proto.Message, raw, signature string) (err error) {
	req := message.(*billing.CardPayPaymentCallback)
	order := h.processor.order
	order.Status = constant.OrderStatusPaymentSystemReject

	err = h.checkCallbackRequestSignature(raw, signature)

	if err != nil {
		return
	}

	if !req.IsPaymentAllowedStatus() {
		return NewError(paymentSystemErrorRequestStatusIsInvalid, pkg.StatusErrorValidation)
	}

	if req.IsRecurring() && (req.RecurringData.Filing == nil || req.RecurringData.Filing.Id == "") {
		return NewError(paymentSystemErrorRequestRecurringIdFieldIsInvalid, pkg.StatusErrorValidation)
	}

	t, err := time.Parse(cardPayDateFormat, req.CallbackTime)

	if err != nil {
		return NewError(paymentSystemErrorRequestTimeFieldIsInvalid, pkg.StatusErrorValidation)
	}

	ts, err := ptypes.TimestampProto(t)

	if err != nil {
		return NewError(paymentSystemErrorRequestTimeFieldIsInvalid, pkg.StatusErrorValidation)
	}

	if req.PaymentMethod != h.processor.order.PaymentMethod.Params.ExternalId {
		return NewError(paymentSystemErrorRequestPaymentMethodIsInvalid, pkg.StatusErrorValidation)
	}

	reqAmount := req.GetAmount()

	if reqAmount != order.PaymentMethodOutcomeAmount ||
		req.GetCurrency() != order.PaymentMethodOutcomeCurrency.CodeA3 {
		return NewError(paymentSystemErrorRequestAmountOrCurrencyIsInvalid, pkg.StatusErrorValidation)
	}

	switch req.PaymentMethod {
	case constant.PaymentSystemGroupAliasBankCard:
		order.PaymentMethodPayerAccount = req.CardAccount.MaskedPan
		order.PaymentMethodTxnParams = req.GetBankCardTxnParams()
		break
	case constant.PaymentSystemGroupAliasQiwi,
		constant.PaymentSystemGroupAliasWebMoney,
		constant.PaymentSystemGroupAliasNeteller,
		constant.PaymentSystemGroupAliasAlipay:
		order.PaymentMethodPayerAccount = req.EwalletAccount.Id
		order.PaymentMethodTxnParams = req.GetEWalletTxnParams()
		break
	case constant.PaymentSystemGroupAliasBitcoin:
		order.PaymentMethodPayerAccount = req.CryptocurrencyAccount.CryptoAddress
		order.PaymentMethodTxnParams = req.GetCryptoCurrencyTxnParams()
		break
	default:
		return NewError(paymentSystemErrorRequestPaymentMethodIsInvalid, pkg.StatusErrorValidation)
	}

	switch req.GetStatus() {
	case pkg.CardPayPaymentResponseStatusDeclined:
		order.Status = constant.OrderStatusPaymentSystemDeclined
		break
	case pkg.CardPayPaymentResponseStatusCancelled:
		order.Status = constant.OrderStatusPaymentSystemCanceled
		break
	case pkg.CardPayPaymentResponseStatusCompleted:
		order.Status = constant.OrderStatusPaymentSystemComplete
		break
	default:
		return NewError(paymentSystemErrorRequestTemporarySkipped, pkg.StatusTemporary)
	}

	order.PaymentMethodOrderId = req.GetId()
	order.PaymentMethodOrderClosedAt = ts
	order.PaymentMethodIncomeAmount = reqAmount
	order.PaymentMethodIncomeCurrency = order.PaymentMethodOutcomeCurrency

	return
}

func (h *cardPay) IsRecurringCallback(request proto.Message) bool {
	req := request.(*billing.CardPayPaymentCallback)
	return req.PaymentMethod == constant.PaymentSystemGroupAliasBankCard && req.IsRecurring()
}

func (h *cardPay) GetRecurringId(request proto.Message) string {
	return request.(*billing.CardPayPaymentCallback).RecurringData.Filing.Id
}

func (h *cardPay) auth(pmKey string) error {
	if token := h.getToken(pmKey); token != nil {
		return nil
	}

	data := url.Values{
		cardPayRequestFieldGrantType:    []string{cardPayGrantTypePassword},
		cardPayRequestFieldTerminalCode: []string{h.processor.order.PaymentMethod.Params.Terminal},
		cardPayRequestFieldPassword:     []string{h.processor.order.PaymentMethod.Params.Password},
	}

	qUrl, err := h.getUrl(cardPayActionAuthenticate)

	if err != nil {
		return err
	}

	client := tools.NewLoggedHttpClient(zap.S())
	req, err := http.NewRequest(cardPayPaths[cardPayActionAuthenticate].method, qUrl, strings.NewReader(data.Encode()))

	if err != nil {
		return err
	}

	req.Header.Add(HeaderContentType, MIMEApplicationForm)
	req.Header.Add(HeaderContentLength, strconv.Itoa(len(data.Encode())))

	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			return
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return errors.New(paymentSystemErrorAuthenticateFailed)
	}

	b, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	if err := h.setToken(b, pmKey); err != nil {
		return err
	}

	return nil
}

func (h *cardPay) refresh(pmKey string) error {
	data := url.Values{
		cardPayRequestFieldGrantType:    []string{cardPayGrantTypeRefreshToken},
		cardPayRequestFieldTerminalCode: []string{h.processor.order.PaymentMethod.Params.Terminal},
		cardPayRequestFieldRefreshToken: []string{cardPayTokens[pmKey].RefreshToken},
	}

	qUrl, err := h.getUrl(cardPayActionRefresh)

	if err != nil {
		return err
	}

	client := tools.NewLoggedHttpClient(zap.S())
	req, err := http.NewRequest(cardPayPaths[cardPayActionRefresh].method, qUrl, strings.NewReader(data.Encode()))

	if err != nil {
		return err
	}

	req.Header.Add(HeaderContentType, MIMEApplicationForm)
	req.Header.Add(HeaderContentLength, strconv.Itoa(len(data.Encode())))

	resp, err := client.Do(req)

	if err != nil {
		return err
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			return
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return errors.New(paymentSystemErrorAuthenticateFailed)
	}

	b, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	if err := h.setToken(b, pmKey); err != nil {
		return err
	}

	return nil
}

func (h *cardPay) getUrl(action string) (string, error) {
	u, err := url.ParseRequestURI(h.processor.cfg.CardPayApiUrl)

	if err != nil {
		return "", err
	}

	u.Path = cardPayPaths[action].path

	return u.String(), nil
}

func (h *cardPay) setToken(b []byte, pmKey string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var token *cardPayToken

	if err := json.Unmarshal(b, &token); err != nil {
		return err
	}

	token.AccessTokenExpireTime = time.Now().Add(time.Second * time.Duration(token.AccessTokenExpire))
	token.RefreshTokenExpireTime = time.Now().Add(time.Second * time.Duration(token.RefreshTokenExpire))

	cardPayTokens[pmKey] = token

	return nil
}

func (h *cardPay) getToken(pmKey string) *cardPayToken {
	token, ok := cardPayTokens[pmKey]

	if !ok {
		return nil
	}

	tn := time.Now().Unix()

	if token.AccessTokenExpire > 0 && token.AccessTokenExpireTime.Unix() >= tn {
		return token
	}

	if token.RefreshTokenExpire <= 0 || token.RefreshTokenExpireTime.Unix() < tn {
		return nil
	}

	if err := h.refresh(pmKey); err != nil {
		return nil
	}

	return cardPayTokens[pmKey]
}

func (h *cardPay) getCardPayOrder(order *billing.Order, requisites map[string]string) (*CardPayOrder, error) {
	cardPayOrder := &CardPayOrder{
		Request: &CardPayRequest{
			Id:   order.Id,
			Time: time.Now().UTC().Format(cardPayDateFormat),
		},
		MerchantOrder: &CardPayMerchantOrder{
			Id:          order.Id,
			Description: order.Description,
			Items: []*CardPayItem{
				{
					Name:        order.FixedPackage.Name,
					Description: order.FixedPackage.Name,
					Count:       1,
					Price:       order.FixedPackage.Price,
				},
			},
		},
		Description:   order.Description,
		PaymentMethod: order.PaymentMethod.Params.ExternalId,
		Customer: &CardPayCustomer{
			Email:   order.PayerData.Email,
			Ip:      order.PayerData.Ip,
			Account: order.ProjectAccount,
		},
	}

	if order.Project.UrlSuccess != "" || order.Project.UrlFail != "" {
		cardPayOrder.ReturnUrls = &CardPayReturnUrls{}

		if order.Project.UrlSuccess != "" {
			cardPayOrder.ReturnUrls.SuccessUrl = order.Project.UrlSuccess
		}

		if order.Project.UrlFail != "" {
			cardPayOrder.ReturnUrls.DeclineUrl = order.Project.UrlFail
			cardPayOrder.ReturnUrls.CancelUrl = order.Project.UrlFail
		}
	}

	storeData, okStoreData := requisites[pkg.PaymentCreateFieldStoreData]
	recurringId, okRecurringId := requisites[pkg.PaymentCreateFieldRecurringId]

	if order.PaymentMethod.IsBankCard() && (okStoreData && storeData == "1") ||
		(okRecurringId && recurringId != "") {
		cardPayOrder.RecurringData = &CardPayRecurringData{
			Currency:  order.PaymentMethodOutcomeCurrency.CodeA3,
			Amount:    order.PaymentMethodOutcomeAmount,
			Initiator: cardPayInitiatorCardholder,
		}

		if okRecurringId == true && recurringId != "" {
			cardPayOrder.RecurringData.Filing = &CardPayRecurringDataFiling{
				Id: recurringId,
			}

			return cardPayOrder, nil
		}
	} else {
		cardPayOrder.PaymentData = &CardPayPaymentData{
			Currency: order.PaymentMethodOutcomeCurrency.CodeA3,
			Amount:   order.PaymentMethodOutcomeAmount,
		}
	}

	switch order.PaymentMethod.Params.ExternalId {
	case constant.PaymentSystemGroupAliasBankCard:
		h.geBankCardCardPayOrder(cardPayOrder, requisites)
		break
	case constant.PaymentSystemGroupAliasQiwi,
		constant.PaymentSystemGroupAliasWebMoney,
		constant.PaymentSystemGroupAliasNeteller,
		constant.PaymentSystemGroupAliasAlipay:
		h.getEWalletCardPayOrder(cardPayOrder, requisites)
		break
	case constant.PaymentSystemGroupAliasBitcoin:
		h.getCryptoCurrencyCardPayOrder(cardPayOrder, requisites)
		break
	default:
		return nil, errors.New(paymentSystemErrorUnknownPaymentMethod)
	}

	return cardPayOrder, nil
}

func (h *cardPay) geBankCardCardPayOrder(cpo *CardPayOrder, requisites map[string]string) {
	expire := requisites[pkg.PaymentCreateFieldMonth] + "/" + requisites[pkg.PaymentCreateFieldYear]

	cpo.CardAccount = &CardPayCardAccount{
		Card: &CardPayBankCardAccount{
			Pan:        requisites[pkg.PaymentCreateFieldPan],
			HolderName: requisites[pkg.PaymentCreateFieldHolder],
			Cvv:        requisites[pkg.PaymentCreateFieldCvv],
			Expire:     expire,
		},
	}
}

func (h *cardPay) getEWalletCardPayOrder(cpo *CardPayOrder, requisites map[string]string) {
	cpo.EWalletAccount = &CardPayEWalletAccount{
		Id: requisites[pkg.PaymentCreateFieldEWallet],
	}
}

func (h *cardPay) getCryptoCurrencyCardPayOrder(cpo *CardPayOrder, requisites map[string]string) {
	cpo.CryptoCurrencyAccount = &CardPayCryptoCurrencyAccount{
		RollbackAddress: requisites[pkg.PaymentCreateFieldCrypto],
	}
}

func (h *cardPay) checkCallbackRequestSignature(raw, signature string) error {
	hash := sha512.New()
	hash.Write([]byte(raw + h.processor.order.PaymentMethod.Params.CallbackPassword))

	if hex.EncodeToString(hash.Sum(nil)) != signature {
		return NewError(paymentSystemErrorRequestSignatureIsInvalid, pkg.StatusErrorValidation)
	}

	return nil
}

func (t *cardPayTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := context.WithValue(req.Context(), &cardPayContextKey{name: "CardPayRequestStart"}, time.Now())
	req = req.WithContext(ctx)

	var reqBody []byte

	if req.Body != nil {
		reqBody, _ = ioutil.ReadAll(req.Body)
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

	resp, err := t.transport().RoundTrip(req)
	if err != nil {
		return resp, err
	}

	t.log(req.URL.Path, req.Header, reqBody, resp)

	return resp, err
}

func (t *cardPayTransport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}

	return http.DefaultTransport
}

func (t *cardPayTransport) log(reqUrl string, reqHeader http.Header, reqBody []byte, resp *http.Response) {
	var resBody []byte

	if resp.Body != nil {
		resBody, _ = ioutil.ReadAll(resp.Body)
	}
	resp.Body = ioutil.NopCloser(bytes.NewBuffer(resBody))

	cpOrder := &CardPayOrder{}
	err := json.Unmarshal(reqBody, cpOrder)

	if err != nil {
		return
	}

	if cpOrder.CardAccount != nil {
		cpOrder.CardAccount.Card.Pan = tools.MaskBankCardNumber(cpOrder.CardAccount.Card.Pan)
		cpOrder.CardAccount.Card.Cvv = "***"
	}

	request, err := json.Marshal(cpOrder)

	if err != nil {
		return
	}

	zap.L().Info(
		reqUrl,
		zap.String("request_headers", t.processor.httpHeadersToString(reqHeader)),
		zap.String("request_body", string(request)),
		zap.Int("response_status", resp.StatusCode),
		zap.String("response_headers", t.processor.httpHeadersToString(resp.Header)),
		zap.String("response_body", t.processor.cutBytes(resBody, defaultResponseBodyLimit)),
	)
}

func (h *cardPay) Refund(refund *billing.Refund) error {
	err := h.auth(h.processor.order.PaymentMethod.Params.ExternalId)

	if err != nil {
		h.processor.service.logError(
			"Auth in api failed on refund action",
			[]interface{}{
				"error", err.Error(),
				"handler", paymentSystemHandlerCardPay,
			},
		)

		return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
	}

	qUrl, err := h.getUrl(cardPayActionRefund)

	if err != nil {
		return err
	}

	data := &CardPayRefundRequest{
		Request: &CardPayRequest{
			Id:   refund.Id,
			Time: time.Now().UTC().Format(cardPayDateFormat),
		},
		MerchantOrder: &CardPayMerchantOrder{
			Id:          refund.Id,
			Description: refund.Reason,
		},
		PaymentData: &CardPayRecurringDataFiling{
			Id: h.processor.order.Id,
		},
		RefundData: &CardPayRefundData{
			Amount:   refund.Amount,
			Currency: refund.Currency.CodeA3,
		},
	}

	b, err := json.Marshal(data)

	if err != nil {
		h.processor.service.logError(
			"Marshal refund request failed",
			[]interface{}{
				"error", err.Error(),
				"handler", paymentSystemHandlerCardPay,
				"req", data,
			},
		)
		return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
	}

	client := tools.NewLoggedHttpClient(zap.S())
	req, err := http.NewRequest(cardPayPaths[cardPayActionRefund].method, qUrl, bytes.NewBuffer(b))

	if err != nil {
		h.processor.service.logError(
			"Refund request building failed",
			[]interface{}{
				"error", err.Error(),
				"handler", paymentSystemHandlerCardPay,
				"req", data,
			},
		)
		return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
	}

	token := h.getToken(h.processor.order.PaymentMethod.Params.ExternalId)
	auth := strings.Title(token.TokenType) + " " + token.AccessToken

	req.Header.Add(HeaderContentType, MIMEApplicationJSON)
	req.Header.Add(HeaderAuthorization, auth)

	refund.Status = pkg.RefundStatusRejected
	resp, err := client.Do(req)

	if err != nil || resp.StatusCode != http.StatusOK {
		if err != nil {
			h.processor.service.logError(
				"Refund request failed",
				[]interface{}{
					"error", err.Error(),
					"handler", paymentSystemHandlerCardPay,
					"req", data,
				},
			)
		}

		return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
	}

	b, err = ioutil.ReadAll(resp.Body)

	if err != nil {
		h.processor.service.logError(
			"Refund response can't be read",
			[]interface{}{
				"error", err.Error(),
				"handler", paymentSystemHandlerCardPay,
				"req", data,
			},
		)

		return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
	}

	rsp := &CardPayRefundResponse{}
	err = json.Unmarshal(b, &rsp)

	if err != nil {
		h.processor.service.logError(
			"Refund response can't be unmarshal",
			[]interface{}{
				"error", err.Error(),
				"handler", paymentSystemHandlerCardPay,
				"req", string(b),
			},
		)

		return errors.New(pkg.PaymentSystemErrorCreateRefundFailed)
	}

	if rsp.IsSuccessStatus() == false {
		return errors.New(pkg.PaymentSystemErrorCreateRefundRejected)
	}

	refund.Status = pkg.RefundStatusInProgress
	refund.ExternalId = rsp.RefundData.Id

	return nil
}
