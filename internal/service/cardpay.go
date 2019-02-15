package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/ProtocolONE/payone-repository/pkg/constant"
	"github.com/ProtocolONE/payone-repository/tools"
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

	cardPayActionAuthenticate  = "auth"
	cardPayActionRefresh       = "refresh"
	cardPayActionCreatePayment = "create_payment"

	cardPayDateFormat = "2006-01-02T15:04:05Z"
)

var cardPayTokens = map[string]*cardPayToken{}

var cardPayPaths = map[string]*Path{
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
}

type cardPay struct {
	processor *paymentProcessor
	mu        sync.Mutex
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

type CardPayPaymentData struct {
	Currency   string  `json:"currency"`
	Amount     float64 `json:"amount"`
	Descriptor string  `json:"dynamic_descriptor"`
	Note       string  `json:"note"`
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
	PaymentData           *CardPayPaymentData           `json:"payment_data"`
	CardAccount           *CardPayCardAccount           `json:"card_account,omitempty"`
	Customer              *CardPayCustomer              `json:"customer"`
	EWalletAccount        *CardPayEWalletAccount        `json:"ewallet_account,omitempty"`
	CryptoCurrencyAccount *CardPayCryptoCurrencyAccount `json:"cryptocurrency_account,omitempty"`
	ReturnUrls            *CardPayReturnUrls            `json:"return_urls,omitempty"`
}

type CardPayOrderResponse struct {
	RedirectUrl string `json:"redirect_url"`
}

func newCardPayHandler(processor *paymentProcessor) PaymentSystem {
	return &cardPay{processor: processor}
}

func (h *cardPay) CreatePayment(order *billing.Order, requisites map[string]string) (url string, err error) {
	if err = h.auth(order.PaymentMethod.Params.ExternalId); err != nil {
		return
	}

	qUrl, err := h.getUrl(cardPayActionCreatePayment)

	if err != nil {
		return
	}

	cpOrder, err := h.getCardPayOrder(order, requisites)

	if err != nil {
		return
	}

	order.Status = constant.OrderStatusPaymentSystemRejectOnCreate

	b, _ := json.Marshal(cpOrder)

	client := tools.NewLoggedHttpClient(h.processor.log)
	req, err := http.NewRequest(cardPayPaths[cardPayActionCreatePayment].method, qUrl, bytes.NewBuffer(b))

	token := h.getToken(order.PaymentMethod.Params.ExternalId)
	auth := strings.Title(token.TokenType) + " " + token.AccessToken

	req.Header.Add(HeaderContentType, MIMEApplicationJSON)
	req.Header.Add(HeaderAuthorization, auth)

	resp, err := client.Do(req)

	if resp.StatusCode != http.StatusOK {
		return "", errors.New(paymentSystemErrorCreateRequestFailed)
	}

	if b, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	cpResponse := &CardPayOrderResponse{}

	if err = json.Unmarshal(b, &cpResponse); err != nil {
		return
	}

	order.Status = constant.OrderStatusPaymentSystemCreate
	url = cpResponse.RedirectUrl

	return
}

func (h *cardPay) ProcessPayment() (err error) {
	return
}

func (h *cardPay) auth(pmKey string) error {
	if token := h.getToken(pmKey); token != nil {
		return nil
	}

	data := url.Values{
		cardPayRequestFieldGrantType:    []string{cardPayGrantTypePassword},
		cardPayRequestFieldTerminalCode: []string{h.processor.auth.Terminal},
		cardPayRequestFieldPassword:     []string{h.processor.auth.Password},
	}

	qUrl, err := h.getUrl(cardPayActionAuthenticate)

	if err != nil {
		return err
	}

	client := tools.NewLoggedHttpClient(h.processor.log)
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
		cardPayRequestFieldTerminalCode: []string{h.processor.auth.Terminal},
		cardPayRequestFieldRefreshToken: []string{cardPayTokens[pmKey].RefreshToken},
	}

	qUrl, err := h.getUrl(cardPayActionRefresh)

	if err != nil {
		return err
	}

	client := tools.NewLoggedHttpClient(h.processor.log)
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
	u, err := url.ParseRequestURI(h.processor.cfg.CardPayOrderCreateUrl)

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
		PaymentData: &CardPayPaymentData{
			Currency: order.PaymentMethodOutcomeCurrency.CodeA3,
			Amount:   order.PaymentMethodOutcomeAmount,
		},
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
	year := requisites[paymentCreateFieldYear]

	if len(year) < 3 {
		year = strconv.Itoa(time.Now().UTC().Year())[:2] + year
	}

	expire := requisites[paymentCreateFieldMonth] + "/" + year

	cpo.CardAccount = &CardPayCardAccount{
		Card: &CardPayBankCardAccount{
			Pan:        requisites[paymentCreateFieldPan],
			HolderName: requisites[paymentCreateFieldHolder],
			Cvv:        requisites[paymentCreateFieldCvv],
			Expire:     expire,
		},
	}
}

func (h *cardPay) getEWalletCardPayOrder(cpo *CardPayOrder, requisites map[string]string) {
	cpo.EWalletAccount = &CardPayEWalletAccount{
		Id: requisites[paymentCreateFieldEWallet],
	}
}

func (h *cardPay) getCryptoCurrencyCardPayOrder(cpo *CardPayOrder, requisites map[string]string) {
	cpo.CryptoCurrencyAccount = &CardPayCryptoCurrencyAccount{
		RollbackAddress: requisites[paymentCreateFieldCrypto],
	}
}
