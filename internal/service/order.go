package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ProtocolONE/geoip-service/pkg/proto"
	"github.com/dgrijalva/jwt-go"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/google/uuid"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
	"github.com/paysuper/paysuper-recurring-repository/pkg/proto/entity"
	repo "github.com/paysuper/paysuper-recurring-repository/pkg/proto/repository"
	"github.com/paysuper/paysuper-recurring-repository/tools"
	"github.com/paysuper/paysuper-tax-service/proto"
	"github.com/streadway/amqp"
	"github.com/ttacon/libphonenumber"
	"go.uber.org/zap"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	orderErrorProjectNotFound                          = "project with specified identifier not found"
	orderErrorProjectInactive                          = "project with specified identifier is inactive"
	orderErrorPaymentMethodNotAllowed                  = "payment method not specified for project"
	orderErrorPaymentMethodNotFound                    = "payment method with specified not found"
	orderErrorPaymentMethodInactive                    = "payment method with specified is inactive"
	orderErrorPaymentMethodIncompatible                = "payment method setting for project incompatible with main settings"
	orderErrorPaymentMethodEmptySettings               = "payment method setting for project is empty"
	orderErrorPaymentSystemInactive                    = "payment system for specified payment method is inactive"
	orderErrorPayerRegionUnknown                       = "payer region can't be found"
	orderErrorFixedPackagesIsEmpty                     = "project's fixed packages list is empty"
	orderErrorFixedPackageForRegionNotFound            = "project not have fixed packages for payer region"
	orderErrorFixedPackageNotFound                     = "project not have fixed package with specified amount or currency"
	orderErrorFixedPackageUnknownCurrency              = "to fixed package of project set unknown currency"
	orderErrorProjectOrderIdIsDuplicate                = "request with specified project order identifier processed early"
	orderErrorDynamicNotifyUrlsNotAllowed              = "dynamic verify url or notify url not allowed for project"
	orderErrorDynamicRedirectUrlsNotAllowed            = "dynamic payer redirect urls not allowed for project"
	orderErrorCurrencyNotFound                         = "currency received from request not found"
	orderErrorAmountLowerThanMinAllowed                = "order amount is lower than min allowed payment amount for project"
	orderErrorAmountGreaterThanMaxAllowed              = "order amount is greater than max allowed payment amount for project"
	orderErrorAmountLowerThanMinAllowedPaymentMethod   = "order amount is lower than min allowed payment amount for payment method"
	orderErrorAmountGreaterThanMaxAllowedPaymentMethod = "order amount is greater than max allowed payment amount for payment method"
	orderErrorCanNotCreate                             = "order can't create. try request later"
	orderErrorSignatureInvalid                         = "order request signature is invalid"
	orderErrorNotFound                                 = "order with specified identifier not found"
	orderErrorOrderAlreadyComplete                     = "order with specified identifier payed early"
	orderErrorFormInputTimeExpired                     = "time to enter date on payment form expired"
	orderErrorCurrencyIsRequired                       = "parameter currency in create order request is required"
	orderErrorUnknown                                  = "unknown error. try request later"
	orderCurrencyConvertationError                     = "unknown error in process currency conversion. try request later"
	orderGetSavedCardError                             = "saved card data with specified identifier not found"
	paymentRequestIncorrect                            = "payment request has incorrect format"
	callbackRequestIncorrect                           = "callback request has incorrect format"
	callbackHandlerIncorrect                           = "unknown callback type"
	orderErrorCountryByPaymentAccountNotFound          = "information about user country can't be found"
	orderErrorPaymentAccountIncorrect                  = "account in  payment system is incorrect"

	orderErrorCreatePaymentRequiredFieldIdNotFound            = "required field with order identifier not found"
	orderErrorCreatePaymentRequiredFieldPaymentMethodNotFound = "required field with payment method identifier not found"
	orderErrorCreatePaymentRequiredFieldEmailNotFound         = "required field \"email\" not found"
	orderErrorCreatePaymentRequiredFieldUserCountryNotFound   = "user country is required"
	orderErrorCreatePaymentRequiredFieldUserCityNotFound      = "user city is required"
	orderErrorCreatePaymentRequiredFieldUserZipNotFound       = "user zip is required"

	paymentCreateBankCardFieldBrand         = "card_brand"
	paymentCreateBankCardFieldType          = "card_type"
	paymentCreateBankCardFieldCategory      = "card_category"
	paymentCreateBankCardFieldIssuerName    = "bank_issuer_name"
	paymentCreateBankCardFieldIssuerCountry = "bank_issuer_country"

	orderDefaultDescription      = "Payment by order # %s"
	orderInlineFormUrlMask       = "%s://%s/order/%s"
	orderInlineFormImagesUrlMask = "//%s%s"

	defaultExpireDateToFormInput = 30

	taxTypeVat      = "vat"
	taxTypeSalesTax = "sales_tax"
)

type orderCreateRequestProcessorChecked struct {
	project       *billing.Project
	currency      *billing.Currency
	payerData     *billing.PayerData
	fixedPackage  *billing.FixedPackage
	paymentMethod *billing.PaymentMethod
}

type OrderCreateRequestProcessor struct {
	*Service
	checked *orderCreateRequestProcessorChecked
	request *billing.OrderCreateRequest
}

type PaymentFormProcessor struct {
	service *Service
	order   *billing.Order
	request *grpc.PaymentFormJsonDataRequest
}

type PaymentCreateProcessor struct {
	service *Service
	data    map[string]string
	checked struct {
		order         *billing.Order
		project       *billing.Project
		paymentMethod *billing.PaymentMethod
	}
}

type BinData struct {
	Id                bson.ObjectId `bson:"_id"`
	CardBin           int32         `bson:"card_bin"`
	CardBrand         string        `bson:"card_brand"`
	CardType          string        `bson:"card_type"`
	CardCategory      string        `bson:"card_category"`
	BankName          string        `bson:"bank_name"`
	BankCountryName   string        `bson:"bank_country_name"`
	BankCountryCodeA2 string        `bson:"bank_country_code_a2"`
	BankCountryCodeA3 string        `bson:"bank_country_code_a3"`
	BankSite          string        `bson:"bank_site"`
	BankPhone         string        `bson:"bank_phone"`
}

func (s *Service) OrderCreateProcess(
	ctx context.Context,
	req *billing.OrderCreateRequest,
	rsp *billing.Order,
) error {
	processor := &OrderCreateRequestProcessor{Service: s, request: req, checked: &orderCreateRequestProcessorChecked{}}

	if err := processor.processProject(); err != nil {
		return err
	}

	if req.Signature != "" || processor.checked.project.SignatureRequired == true {
		if err := processor.processSignature(); err != nil {
			return err
		}
	}

	if err := processor.processPayerData(); err != nil {
		return err
	}

	if req.Currency != "" {
		if err := processor.processCurrency(); err != nil {
			return err
		}
	}

	if processor.checked.project.OnlyFixedAmounts == true {
		if err := processor.processFixedPackage(); err != nil {
			return err
		}
	}

	if processor.checked.currency == nil {
		return errors.New(orderErrorCurrencyIsRequired)
	}

	if req.OrderId != "" {
		if err := processor.processProjectOrderId(); err != nil {
			return err
		}
	}

	if req.PaymentMethod != "" {
		pm, err := s.GetPaymentMethodByGroupAndCurrency(req.PaymentMethod, processor.checked.currency.CodeInt)

		if err != nil {
			return errors.New(orderErrorPaymentMethodNotFound)
		}

		if err := processor.processPaymentMethod(pm); err != nil {
			return err
		}
	}

	if err := processor.processLimitAmounts(); err != nil {
		return err
	}

	order, err := processor.prepareOrder()

	if err != nil {
		return err
	}

	err = s.db.Collection(pkg.CollectionOrder).Insert(order)

	if err != nil {
		zap.S().Errorw(fmt.Sprintf(errorQueryMask, pkg.CollectionOrder), "err", err, "inserted_data", order)
		return errors.New(orderErrorCanNotCreate)
	}

	rsp.Id = order.Id
	rsp.Project = order.Project
	rsp.Description = order.Description
	rsp.ProjectOrderId = order.ProjectOrderId
	rsp.ProjectAccount = order.ProjectAccount
	rsp.ProjectIncomeAmount = order.ProjectIncomeAmount
	rsp.ProjectIncomeCurrency = order.ProjectIncomeCurrency
	rsp.ProjectOutcomeAmount = order.ProjectOutcomeAmount
	rsp.ProjectOutcomeCurrency = order.ProjectOutcomeCurrency
	rsp.ProjectParams = order.ProjectParams
	rsp.PayerData = order.PayerData
	rsp.Status = order.Status
	rsp.CreatedAt = order.CreatedAt
	rsp.IsJsonRequest = order.IsJsonRequest
	rsp.FixedPackage = order.FixedPackage
	rsp.AmountInMerchantAccountingCurrency = order.AmountInMerchantAccountingCurrency
	rsp.PaymentMethodOutcomeAmount = order.PaymentMethodOutcomeAmount
	rsp.PaymentMethodOutcomeCurrency = order.PaymentMethodOutcomeCurrency
	rsp.PaymentMethodIncomeAmount = order.PaymentMethodIncomeAmount
	rsp.PaymentMethodIncomeCurrency = order.PaymentMethodIncomeCurrency
	rsp.PaymentMethod = order.PaymentMethod
	rsp.ProjectFeeAmount = order.ProjectFeeAmount
	rsp.PspFeeAmount = order.PspFeeAmount
	rsp.PaymentSystemFeeAmount = order.PaymentSystemFeeAmount
	rsp.PaymentMethodOutcomeAmount = order.PaymentMethodOutcomeAmount
	rsp.Tax = order.Tax
	rsp.Uuid = order.Uuid
	rsp.ExpireDateToFormInput = order.ExpireDateToFormInput
	rsp.TotalPaymentAmount = order.TotalPaymentAmount

	return nil
}

func (s *Service) PaymentFormJsonDataProcess(
	ctx context.Context,
	req *grpc.PaymentFormJsonDataRequest,
	rsp *grpc.PaymentFormJsonDataResponse,
) error {
	order, err := s.getOrderById(req.OrderId)

	if err != nil {
		return err
	}

	p := &PaymentFormProcessor{service: s, order: order, request: req}
	p1 := &OrderCreateRequestProcessor{Service: s,
		request: &billing.OrderCreateRequest{
			PayerIp: req.Ip,
		},
		checked: &orderCreateRequestProcessorChecked{},
	}

	err = p1.processPayerData()

	if err != nil {
		return err
	}

	loc, ctr := s.getCountryFromAcceptLanguage(req.Locale)
	cToken := req.Token

	if cToken == "" && order.CustomerToken != "" {
		cToken = order.CustomerToken
	}

	var customer *billing.Customer

	if cToken != "" {
		customer, err = s.getCustomerBy(bson.M{"token": req.Token})

		if err != nil {
			return err
		}

		if customer.Ip != req.Ip || customer.Locale != loc {
			if customer.Ip != req.Ip {
				customer.Ip = req.Ip

				customer.Address = &billing.OrderBillingAddress{
					Country:    p1.checked.payerData.Country,
					City:       p1.checked.payerData.City.En,
					PostalCode: p1.checked.payerData.Zip,
					State:      p1.checked.payerData.State,
				}
			}

			if customer.Locale != loc {
				customer.Locale = loc
			}
		}
	} else {
		customer = &billing.Customer{
			ProjectId: order.Project.Id,
			Ip:        p1.checked.payerData.Ip,
			Locale:    loc,
			Address: &billing.OrderBillingAddress{
				Country:    p1.checked.payerData.Country,
				City:       p1.checked.payerData.City.En,
				PostalCode: p1.checked.payerData.Zip,
				State:      p1.checked.payerData.State,
			},
		}
	}

	customer, err = s.changeCustomer(customer)

	if err != nil {
		return err
	}

	order.User = customer

	if ctr != order.User.Address.Country {
		order.UserAddressDataRequired = true
		rsp.UserAddressDataRequired = order.UserAddressDataRequired
	}

	p1.processOrderVat(order)
	err = s.updateOrder(order)

	if err != nil {
		return err
	}

	pms, err := p.processRenderFormPaymentMethods()

	if err != nil {
		return err
	}

	expire := time.Now().Add(time.Minute * 30).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": order.Id, "exp": expire})

	rsp.Id = order.Uuid
	rsp.Account = order.ProjectAccount
	rsp.HasVat = order.Tax.Amount > 0
	rsp.Vat = order.Tax.Amount
	rsp.Currency = order.ProjectIncomeCurrency.CodeA3
	rsp.Project = &grpc.PaymentFormJsonDataProject{
		Name:       order.Project.Name,
		UrlSuccess: order.Project.UrlSuccess,
		UrlFail:    order.Project.UrlFail,
	}
	rsp.PaymentMethods = pms
	rsp.Token, _ = token.SignedString([]byte(s.cfg.CentrifugoSecret))
	rsp.InlineFormRedirectUrl = fmt.Sprintf(orderInlineFormUrlMask, req.Scheme, req.Host, rsp.Id)
	rsp.Amount = order.PaymentMethodOutcomeAmount
	rsp.TotalAmount = order.TotalPaymentAmount

	return nil
}

func (s *Service) PaymentCreateProcess(
	ctx context.Context,
	req *grpc.PaymentCreateRequest,
	rsp *grpc.PaymentCreateResponse,
) error {
	processor := &PaymentCreateProcessor{service: s, data: req.Data}
	err := processor.processPaymentFormData()

	if err != nil {
		rsp.Error = err.Error()
		rsp.Status = pkg.StatusErrorValidation

		return nil
	}

	order := processor.checked.order
	order.PaymentMethod = &billing.PaymentMethodOrder{
		Id:            processor.checked.paymentMethod.Id,
		Name:          processor.checked.paymentMethod.Name,
		Params:        processor.checked.paymentMethod.Params,
		PaymentSystem: processor.checked.paymentMethod.PaymentSystem,
		Group:         processor.checked.paymentMethod.Group,
	}

	commissionProcessor := &OrderCreateRequestProcessor{Service: s}
	err = commissionProcessor.processOrderCommissions(order)

	if err != nil {
		rsp.Error = err.Error()
		rsp.Status = pkg.StatusErrorValidation

		return nil
	}

	err = processor.processPaymentAmounts()

	if err != nil {
		rsp.Error = orderCurrencyConvertationError
		rsp.Status = pkg.StatusErrorSystem

		return nil
	}

	if _, ok := order.PaymentRequisites[pkg.PaymentCreateFieldRecurringId]; ok {
		req.Data[pkg.PaymentCreateFieldRecurringId] = order.PaymentRequisites[pkg.PaymentCreateFieldRecurringId]
		delete(order.PaymentRequisites, pkg.PaymentCreateFieldRecurringId)
	}

	err = s.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	if err != nil {
		s.logError("Update order data failed", []interface{}{"err", err.Error(), "order", order})

		rsp.Error = orderErrorUnknown
		rsp.Status = pkg.StatusErrorSystem

		return nil
	}

	if s.isProductionEnvironment() == true {
		order.PaymentMethod.Params.Terminal = processor.checked.project.PaymentMethods[order.PaymentMethod.Group].Terminal
		order.PaymentMethod.Params.Password = processor.checked.project.PaymentMethods[order.PaymentMethod.Group].Password
		order.PaymentMethod.Params.CallbackPassword = processor.checked.project.PaymentMethods[order.PaymentMethod.Group].CallbackPassword
	}

	h, err := s.NewPaymentSystem(s.cfg.PaymentSystemConfig, order)

	if err != nil {
		rsp.Error = err.Error()
		rsp.Status = pkg.StatusErrorSystem

		return nil
	}

	url, err := h.CreatePayment(req.Data)
	errDb := s.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	if errDb != nil {
		s.logError("Update order data failed", []interface{}{"err", err.Error(), "order", order})

		rsp.Error = orderErrorUnknown
		rsp.Status = pkg.StatusErrorSystem

		return nil
	}

	if err != nil {
		s.logError("Order create in payment system failed", []interface{}{"err", err.Error(), "order", order})

		rsp.Error = err.Error()
		rsp.Status = pkg.StatusErrorPaymentSystem

		return nil
	}

	rsp.Status = pkg.StatusOK
	rsp.RedirectUrl = url

	return nil
}

func (s *Service) PaymentCallbackProcess(
	ctx context.Context,
	req *grpc.PaymentNotifyRequest,
	rsp *grpc.PaymentNotifyResponse,
) error {
	order, err := s.getOrderById(req.OrderId)

	if err != nil {
		return errors.New(orderErrorNotFound)
	}

	var data protobuf.Message

	switch order.PaymentMethod.Params.Handler {
	case pkg.PaymentSystemHandlerCardPay:
		data = &billing.CardPayPaymentCallback{}
		err := json.Unmarshal(req.Request, data)

		if err != nil {
			return errors.New(paymentRequestIncorrect)
		}
		break
	default:
		return errors.New(orderErrorPaymentMethodNotFound)
	}

	h, err := s.NewPaymentSystem(s.cfg.PaymentSystemConfig, order)

	if err != nil {
		return err
	}

	pErr := h.ProcessPayment(data, string(req.Request), req.Signature)

	if pErr != nil {
		s.logError(
			"Callback processing failed",
			[]interface{}{
				"err", pErr.Error(),
				"order_id", req.OrderId,
				"request", string(req.Request),
				"signature", req.Signature,
			},
		)

		pErr, _ := pErr.(*Error)

		rsp.Error = pErr.Error()
		rsp.Status = pErr.Status()

		if pErr.Status() == pkg.StatusTemporary {
			return nil
		}
	}

	err = s.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	if err != nil {
		s.logError("Update order data failed", []interface{}{"err", err.Error(), "order", order})

		rsp.Error = orderErrorUnknown
		rsp.Status = pkg.StatusErrorSystem

		return nil
	}

	if pErr == nil {
		if h.IsRecurringCallback(data) {
			s.saveRecurringCard(order, h.GetRecurringId(data))
		}

		err = s.broker.Publish(constant.PayOneTopicNotifyPaymentName, order, amqp.Table{"x-retry-count": int32(0)})

		if err != nil {
			s.logError("Publish notify message to queue failed", []interface{}{"err", err.Error(), "order", order})
		}

		rsp.Status = pkg.StatusOK
	}

	return nil
}

func (s *Service) PaymentFormLanguageChanged(
	ctx context.Context,
	req *grpc.PaymentFormUserChangeLangRequest,
	rsp *grpc.PaymentFormDataChangeResponse,
) error {
	order, err := s.getOrderByUuidToForm(req.OrderId)

	if err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = &grpc.PaymentFormDataChangeResponseItem{
		UserAddressDataRequired: false,
	}

	if order.User.Locale == req.Lang {
		return nil
	}

	order.User.Locale = req.Lang
	order.UserAddressDataRequired = true

	err = s.updateOrder(order)

	if err != nil {
		rsp.Status = pkg.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Item.UserAddressDataRequired = true
	rsp.Item.UserIpData = &grpc.UserIpData{
		Country: order.User.Address.Country,
		City:    order.User.Address.City,
		Zip:     order.User.Address.PostalCode,
	}

	return nil
}

func (s *Service) PaymentFormPaymentAccountChanged(
	ctx context.Context,
	req *grpc.PaymentFormUserChangePaymentAccountRequest,
	rsp *grpc.PaymentFormDataChangeResponse,
) error {
	order, err := s.getOrderByUuidToForm(req.OrderId)

	if err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = err.Error()

		return nil
	}

	pm, err := s.GetPaymentMethodById(req.MethodId)

	if err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = orderErrorPaymentMethodNotFound

		return nil
	}

	match, err := regexp.MatchString(pm.AccountRegexp, req.Account)

	if match == false || err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = orderErrorPaymentAccountIncorrect

		return nil
	}

	country := ""

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = &grpc.PaymentFormDataChangeResponseItem{}

	switch pm.Params.ExternalId {
	case constant.PaymentSystemGroupAliasBankCard:
		data := s.getBinData(req.Account)

		if data == nil {
			rsp.Status = pkg.ResponseStatusBadData
			rsp.Message = orderErrorCountryByPaymentAccountNotFound

			return nil
		}

		country = data.BankCountryCodeA2
		break
	case constant.PaymentSystemGroupAliasQiwi:
		req.Account = "+" + req.Account
		num, err := libphonenumber.Parse(req.Account, CountryCodeUSA)

		if err != nil || num.CountryCode == nil {
			rsp.Status = pkg.ResponseStatusBadData
			rsp.Message = orderErrorPaymentAccountIncorrect
		}

		ok := false
		country, ok = pkg.CountryPhoneCodes[*num.CountryCode]

		if !ok {
			rsp.Status = pkg.ResponseStatusBadData
			rsp.Message = orderErrorCountryByPaymentAccountNotFound

			return nil
		}

		break
	default:
		return nil
	}

	if order.PayerData.Country == country {
		return nil
	}

	order.User.Address.Country = country
	order.UserAddressDataRequired = true

	err = s.updateOrder(order)

	if err != nil {
		rsp.Status = pkg.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Item.UserAddressDataRequired = true
	rsp.Item.UserIpData = &grpc.UserIpData{
		Country: order.User.Address.Country,
		City:    order.User.Address.City,
		Zip:     order.User.Address.PostalCode,
	}

	return nil
}

func (s *Service) ProcessBillingAddress(
	ctx context.Context,
	req *grpc.ProcessBillingAddressRequest,
	rsp *grpc.ProcessBillingAddressResponse,
) error {
	order, err := s.getOrderByUuidToForm(req.OrderId)

	if err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = err.Error()

		return nil
	}

	order.BillingAddress = &billing.OrderBillingAddress{
		Country:    req.Country,
		City:       req.City,
		PostalCode: req.Zip,
	}

	processor := &OrderCreateRequestProcessor{Service: s}
	processor.processOrderVat(order)

	err = s.updateOrder(order)

	if err != nil {
		rsp.Status = pkg.ResponseStatusSystemError
		rsp.Message = err.Error()

		return nil
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = &grpc.ProcessBillingAddressResponseItem{
		HasVat:      order.Tax.Amount > 0,
		Vat:         order.Tax.Amount,
		Amount:      float32(tools.FormatAmount(order.PaymentMethodOutcomeAmount)),
		TotalAmount: float32(tools.FormatAmount(order.TotalPaymentAmount)),
	}

	return nil
}

func (s *Service) saveRecurringCard(order *billing.Order, recurringId string) {
	req := &repo.SavedCardRequest{
		Account:   order.ProjectAccount,
		ProjectId: order.Project.Id,
		MaskedPan: order.PaymentMethodTxnParams[pkg.PaymentCreateFieldPan],
		Expire: &entity.CardExpire{
			Month: order.PaymentRequisites[pkg.PaymentCreateFieldMonth],
			Year:  order.PaymentRequisites[pkg.PaymentCreateFieldYear],
		},
		RecurringId: recurringId,
	}

	_, err := s.rep.InsertSavedCard(context.TODO(), req)

	if err != nil {
		s.logError(
			"Call repository service to save recurring card failed",
			[]interface{}{
				"err", err.Error(),
				"request", req,
			},
		)
	}
}

func (s *Service) updateOrder(order *billing.Order) error {
	err := s.db.Collection(pkg.CollectionOrder).UpdateId(bson.ObjectIdHex(order.Id), order)

	if err != nil {
		s.logError("Update order data failed", []interface{}{"error", err.Error(), "order", order})

		return errors.New(orderErrorUnknown)
	}

	return nil
}

func (s *Service) getOrderById(id string) (order *billing.Order, err error) {
	err = s.db.Collection(pkg.CollectionOrder).FindId(bson.ObjectIdHex(id)).One(&order)

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Order not found in payment create process", []interface{}{"err", err.Error(), "order_id", id})
	}

	if order == nil {
		return order, errors.New(orderErrorNotFound)
	}

	return
}

func (s *Service) getOrderByUuid(uuid string) (order *billing.Order, err error) {
	err = s.db.Collection(pkg.CollectionOrder).Find(bson.M{"uuid": uuid}).One(&order)

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Order not found in payment create process", []interface{}{"err", err.Error(), "uuid", uuid})
	}

	if order == nil {
		return order, errors.New(orderErrorNotFound)
	}

	return
}

func (s *Service) getOrderByUuidToForm(uuid string) (*billing.Order, error) {
	order, err := s.getOrderByUuid(uuid)

	if err != nil {
		return nil, errors.New(orderErrorNotFound)
	}

	if order.HasEndedStatus() == true {
		return nil, errors.New(orderErrorOrderAlreadyComplete)
	}

	if order.FormInputTimeIsEnded() == true {
		return nil, errors.New(orderErrorFormInputTimeExpired)
	}

	return order, nil
}

func (s *Service) getBinData(pan string) (data *BinData) {
	if len(pan) < 6 {
		s.logError("Incorrect PAN to get BIN data", []interface{}{"pan", pan})
		return
	}

	i, err := strconv.ParseInt(pan[:6], 10, 32)

	if err != nil {
		s.logError("Parse PAN to int failed", []interface{}{"error", err.Error(), "pan", pan})
		return
	}

	err = s.db.Collection(pkg.CollectionBinData).Find(bson.M{"card_bin": int32(i)}).One(&data)

	if err != nil {
		s.logError("Query to get bank card BIN data failed", []interface{}{"error", err.Error(), "pan", pan})
		return
	}

	return
}

func (v *OrderCreateRequestProcessor) prepareOrder() (*billing.Order, error) {
	id := bson.NewObjectId().Hex()
	amount := tools.FormatAmount(v.request.Amount)
	merAccAmount := amount
	merchantPayoutCurrency := v.checked.project.Merchant.GetPayoutCurrency()

	if (v.request.UrlVerify != "" || v.request.UrlNotify != "") && v.checked.project.AllowDynamicNotifyUrls == false {
		return nil, errors.New(orderErrorDynamicNotifyUrlsNotAllowed)
	}

	if (v.request.UrlSuccess != "" || v.request.UrlFail != "") && v.checked.project.AllowDynamicRedirectUrls == false {
		return nil, errors.New(orderErrorDynamicRedirectUrlsNotAllowed)
	}

	if merchantPayoutCurrency != nil && v.checked.currency.CodeInt != merchantPayoutCurrency.CodeInt {
		amount, err := v.Service.Convert(v.checked.currency.CodeInt, merchantPayoutCurrency.CodeInt, v.request.Amount)

		if err != nil {
			return nil, err
		}

		merAccAmount = amount
	}

	order := &billing.Order{
		Id: id,
		Project: &billing.ProjectOrder{
			Id:                v.checked.project.Id,
			Name:              v.checked.project.Name,
			UrlSuccess:        v.checked.project.UrlRedirectSuccess,
			UrlFail:           v.checked.project.UrlRedirectFail,
			SendNotifyEmail:   v.checked.project.SendNotifyEmail,
			NotifyEmails:      v.checked.project.NotifyEmails,
			SecretKey:         v.checked.project.SecretKey,
			UrlCheckAccount:   v.checked.project.UrlCheckAccount,
			UrlProcessPayment: v.checked.project.UrlProcessPayment,
			CallbackProtocol:  v.checked.project.CallbackProtocol,
			Merchant:          v.checked.project.Merchant,
		},
		Description:                        fmt.Sprintf(orderDefaultDescription, id),
		ProjectOrderId:                     v.request.OrderId,
		ProjectAccount:                     v.request.Account,
		ProjectIncomeAmount:                amount,
		ProjectIncomeCurrency:              v.checked.currency,
		ProjectOutcomeAmount:               amount,
		ProjectOutcomeCurrency:             v.checked.project.CallbackCurrency,
		ProjectParams:                      v.request.Other,
		PayerData:                          v.checked.payerData,
		Status:                             constant.OrderStatusNew,
		CreatedAt:                          ptypes.TimestampNow(),
		IsJsonRequest:                      v.request.IsJson,
		FixedPackage:                       v.checked.fixedPackage,
		AmountInMerchantAccountingCurrency: merAccAmount,
		PaymentMethodOutcomeAmount:         amount,
		PaymentMethodOutcomeCurrency:       v.checked.currency,
		PaymentMethodIncomeAmount:          amount,
		PaymentMethodIncomeCurrency:        v.checked.currency,

		Uuid: uuid.New().String(),
		User: nil,
	}

	if order.User != nil {
		v.processOrderVat(order)
	}

	if v.request.Description != "" {
		order.Description = v.request.Description
	}

	if v.request.UrlSuccess != "" {
		order.Project.UrlSuccess = v.request.UrlSuccess
	}

	if v.request.UrlFail != "" {
		order.Project.UrlFail = v.request.UrlFail
	}

	if v.checked.paymentMethod != nil {
		order.PaymentMethod = &billing.PaymentMethodOrder{
			Id:            v.checked.paymentMethod.Id,
			Name:          v.checked.paymentMethod.Name,
			Params:        v.checked.paymentMethod.Params,
			PaymentSystem: v.checked.paymentMethod.PaymentSystem,
			Group:         v.checked.paymentMethod.Group,
		}

		if err := v.processOrderCommissions(order); err != nil {
			return nil, err
		}
	}

	order.ExpireDateToFormInput, _ = ptypes.TimestampProto(time.Now().Add(time.Minute * defaultExpireDateToFormInput))

	return order, nil
}

func (v *OrderCreateRequestProcessor) processProject() error {
	project, err := v.GetProjectById(v.request.ProjectId)

	if err != nil {
		zap.S().Errorw("[PAYONE_BILLING] Order create get project error", "err", err, "request", v.request)
		return errors.New(orderErrorProjectNotFound)
	}

	if project.IsActive == false {
		return errors.New(orderErrorProjectInactive)
	}

	v.checked.project = project

	return nil
}

func (v *OrderCreateRequestProcessor) processCurrency() error {
	currency, err := v.GetCurrencyByCodeA3(v.request.Currency)

	if err != nil {
		zap.S().Errorw("[PAYONE_BILLING] Order create get currency error", "err", err, "request", v.request)
		return errors.New(orderErrorCurrencyNotFound)
	}

	v.checked.currency = currency

	return nil
}

func (v *OrderCreateRequestProcessor) processPayerData() error {
	rsp, err := v.geo.GetIpData(context.TODO(), &proto.GeoIpDataRequest{IP: v.request.PayerIp})

	if err != nil {
		zap.S().Errorw("[PAYONE_BILLING] Order create get payer data error", "err", err, "ip", v.request.PayerIp)
		return errors.New(orderErrorPayerRegionUnknown)
	}

	data := &billing.PayerData{
		Ip:          v.request.PayerIp,
		Country:     rsp.Country.IsoCode,
		CountryName: &billing.Name{En: rsp.Country.Names["en"], Ru: rsp.Country.Names["ru"]},
		City:        &billing.Name{En: rsp.City.Names["en"], Ru: rsp.City.Names["ru"]},
		Timezone:    rsp.Location.TimeZone,
		Email:       v.request.PayerEmail,
		Phone:       v.request.PayerPhone,
		Language:    v.request.Language,
	}

	if len(rsp.Subdivisions) > 0 {
		data.State = rsp.Subdivisions[0].IsoCode
	}

	if rsp.Postal != nil {
		data.Zip = rsp.Postal.Code
	}

	v.checked.payerData = data

	return nil
}

func (v *OrderCreateRequestProcessor) processFixedPackage() error {
	fps := v.checked.project.GetFixedPackage()

	if len(fps) <= 0 {
		return errors.New(orderErrorFixedPackagesIsEmpty)
	}

	region := v.checked.payerData.Country

	if v.request.Region != "" {
		region = v.request.Region
	}

	if region == "" {
		return errors.New(orderErrorPayerRegionUnknown)
	}

	regionFps, ok := v.checked.project.FixedPackage[region]

	if !ok || len(regionFps.FixedPackage) <= 0 {
		return errors.New(orderErrorFixedPackageForRegionNotFound)
	}

	var fp *billing.FixedPackage

	for _, val := range regionFps.FixedPackage {
		if val.Price != v.request.Amount || val.IsActive == false ||
			(v.checked.currency != nil && val.Currency.CodeA3 != v.checked.currency.CodeA3) {
			continue
		}

		fp = val
	}

	if fp == nil {
		return errors.New(orderErrorFixedPackageNotFound)
	}

	if v.checked.currency == nil {
		currency, err := v.GetCurrencyByCodeA3(fp.Currency.CodeA3)

		if err != nil {
			return errors.New(orderErrorFixedPackageUnknownCurrency)
		}

		v.checked.currency = currency
	}

	v.checked.fixedPackage = fp

	return nil
}

func (v *OrderCreateRequestProcessor) processProjectOrderId() error {
	var order *billing.Order

	filter := bson.M{
		"project._id":      bson.ObjectIdHex(v.checked.project.Id),
		"project_order_id": v.request.OrderId,
	}

	err := v.db.Collection(pkg.CollectionOrder).Find(filter).One(&order)

	if err != nil && err != mgo.ErrNotFound {
		zap.S().Errorw("[PAYONE_BILLING] Order create check project order id unique", "err", err, "filter", filter)
		return errors.New(orderErrorCanNotCreate)
	}

	if order != nil {
		return errors.New(orderErrorProjectOrderIdIsDuplicate)
	}

	return nil
}

func (v *OrderCreateRequestProcessor) processPaymentMethod(pm *billing.PaymentMethod) error {
	if pm.IsActive == false {
		return errors.New(orderErrorPaymentMethodInactive)
	}

	if pm.PaymentSystem == nil || pm.PaymentSystem.IsActive == false {
		return errors.New(orderErrorPaymentSystemInactive)
	}

	if v.isProductionEnvironment() == true {
		if len(v.checked.project.PaymentMethods) <= 0 {
			return errors.New(orderErrorPaymentMethodNotAllowed)
		}

		ppm, ok := v.checked.project.PaymentMethods[pm.Group]

		if !ok {
			return errors.New(orderErrorPaymentMethodNotAllowed)
		}

		if ppm.Id != pm.Id {
			return errors.New(orderErrorPaymentMethodIncompatible)
		}

		if ppm.Terminal == "" || ppm.Password == "" {
			return errors.New(orderErrorPaymentMethodEmptySettings)
		}
	}

	v.checked.paymentMethod = pm

	return nil
}

func (v *OrderCreateRequestProcessor) processLimitAmounts() (err error) {
	amount := v.request.Amount

	if v.checked.project.LimitsCurrency.CodeInt != v.checked.currency.CodeInt {
		amount, err = v.Convert(v.checked.currency.CodeInt, v.checked.project.LimitsCurrency.CodeInt, amount)

		if err != nil {
			return
		}
	}

	if amount < v.checked.project.MinPaymentAmount {
		return errors.New(orderErrorAmountLowerThanMinAllowed)
	}

	if v.checked.project.MaxPaymentAmount > 0 && amount > v.checked.project.MaxPaymentAmount {
		return errors.New(orderErrorAmountGreaterThanMaxAllowed)
	}

	if v.checked.paymentMethod != nil {
		if v.request.Amount < v.checked.paymentMethod.MinPaymentAmount {
			return errors.New(orderErrorAmountLowerThanMinAllowedPaymentMethod)
		}

		if v.checked.paymentMethod.MaxPaymentAmount > 0 && v.request.Amount > v.checked.paymentMethod.MaxPaymentAmount {
			return errors.New(orderErrorAmountGreaterThanMaxAllowedPaymentMethod)
		}
	}

	return
}

func (v *OrderCreateRequestProcessor) processSignature() error {
	var hashString string

	if v.request.IsJson == false {
		var keys []string
		var elements []string

		for k := range v.request.RawParams {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, k := range keys {
			value := k + "=" + v.request.RawParams[k]
			elements = append(elements, value)
		}

		hashString = strings.Join(elements, "") + v.checked.project.SecretKey
	} else {
		hashString = v.request.RawBody + v.checked.project.SecretKey
	}

	h := sha512.New()
	h.Write([]byte(hashString))

	if hex.EncodeToString(h.Sum(nil)) != v.request.Signature {
		return errors.New(orderErrorSignatureInvalid)
	}

	return nil
}

// Calculate VAT for order
func (v *OrderCreateRequestProcessor) processOrderVat(order *billing.Order) {
	order.TotalPaymentAmount = order.PaymentMethodOutcomeAmount

	order.Tax = &billing.OrderTax{
		Type:     taxTypeVat,
		Currency: order.PaymentMethodOutcomeCurrency.CodeA3,
	}
	req := &tax_service.GetRateRequest{
		IpData: &tax_service.GeoIdentity{
			Country: order.User.Address.Country,
			City:    order.User.Address.City,
		},
		UserData: &tax_service.GeoIdentity{},
	}

	if order.BillingAddress != nil {
		req.UserData.Country = order.BillingAddress.Country
		req.UserData.City = order.BillingAddress.City
	}

	if order.PayerData.Country == CountryCodeUSA {
		order.Tax.Type = taxTypeSalesTax

		req.IpData.Zip = order.User.Address.PostalCode
		req.IpData.State = order.User.Address.State

		if order.BillingAddress != nil {
			req.UserData.Zip = order.BillingAddress.PostalCode
		}
	}

	rsp, err := v.tax.GetRate(context.TODO(), req)

	if err != nil {
		v.logError("Tax service return error", []interface{}{"error", err.Error(), "request", req})
		return
	}

	if order.BillingAddress != nil {
		req.UserData.State = rsp.Rate.State
	}

	order.Tax.Rate = rsp.Rate.Rate
	order.Tax.Amount = float32(tools.FormatAmount(order.PaymentMethodOutcomeAmount * float64(rsp.Rate.Rate)))
	order.TotalPaymentAmount = tools.FormatAmount(order.TotalPaymentAmount + float64(order.Tax.Amount))

	return
}

// Calculate all possible commissions for order, i.e. payment system fee amount, PSP (P1) fee amount,
// commission shifted from project to user and VAT
func (v *OrderCreateRequestProcessor) processOrderCommissions(o *billing.Order) error {
	mAccCur := o.Project.Merchant.GetPayoutCurrency()
	pmOutCur := o.PaymentMethodOutcomeCurrency.CodeInt
	amount := float64(0)

	// calculate commissions to selected payment method
	commission, err := v.Service.CalculatePmCommission(o.Project.Id, o.PaymentMethod.Id, o.PaymentMethodOutcomeAmount)

	if err != nil {
		return err
	}

	// save information about payment system commission
	o.PaymentSystemFeeAmount = &billing.OrderFeePaymentSystem{
		AmountPaymentMethodCurrency: tools.FormatAmount(commission),
	}

	// convert payment system amount of fee to accounting currency of payment system
	amount, err = v.Service.Convert(pmOutCur, o.PaymentMethod.PaymentSystem.AccountingCurrency.CodeInt, commission)

	if err != nil {
		return err
	}

	o.PaymentSystemFeeAmount.AmountPaymentSystemCurrency = amount

	if mAccCur != nil {
		// convert payment system amount of fee to accounting currency of merchant
		amount, _ = v.Service.Convert(pmOutCur, mAccCur.CodeInt, commission)
		o.PaymentSystemFeeAmount.AmountMerchantCurrency = amount
	}

	return nil
}

// Get payment methods of project for rendering in payment form
func (v *PaymentFormProcessor) processRenderFormPaymentMethods() ([]*billing.PaymentFormPaymentMethod, error) {
	var projectPms []*billing.PaymentFormPaymentMethod

	project, ok := v.service.projectCache[v.order.Project.Id]

	if !ok {
		return projectPms, errors.New(orderErrorProjectNotFound)
	}

	if projectPms, ok := v.service.projectPaymentMethodCache[project.Id]; ok {
		return projectPms, nil
	}

	for k, val := range v.service.paymentMethodCache {
		pm, ok := val[v.order.PaymentMethodOutcomeCurrency.CodeInt]

		if !ok || pm.IsActive == false ||
			pm.PaymentSystem.IsActive == false {
			continue
		}

		if v.order.ProjectIncomeAmount < pm.MinPaymentAmount ||
			(pm.MaxPaymentAmount > 0 && v.order.ProjectIncomeAmount > pm.MaxPaymentAmount) {
			continue
		}

		if v.service.isProductionEnvironment() == true {
			if len(project.PaymentMethods) <= 0 {
				return projectPms, errors.New(orderErrorPaymentMethodNotAllowed)
			}

			ppm, ok := project.PaymentMethods[k]

			if !ok || ppm.Id != pm.Id ||
				ppm.Terminal == "" || ppm.Password == "" {
				continue
			}
		}

		formPm := &billing.PaymentFormPaymentMethod{
			Id:            pm.Id,
			Name:          pm.Name,
			Icon:          fmt.Sprintf(orderInlineFormImagesUrlMask, v.request.Host, pm.Icon),
			Type:          pm.Type,
			Group:         pm.Group,
			AccountRegexp: pm.AccountRegexp,
		}

		err := v.processPaymentMethodsData(formPm)

		if err != nil {
			zap.S().Errorw(
				"[PAYONE_BILLING] Process payment method data failed",
				"error", err,
				"order_id", v.order.Id,
			)
			continue
		}

		projectPms = append(projectPms, formPm)
	}

	if len(projectPms) <= 0 {
		return projectPms, errors.New(orderErrorPaymentMethodNotAllowed)
	}

	v.service.mx.Lock()
	v.service.projectPaymentMethodCache[v.order.Project.Id] = projectPms
	v.service.mx.Unlock()

	return projectPms, nil
}

func (v *PaymentFormProcessor) processPaymentMethodsData(pm *billing.PaymentFormPaymentMethod) error {
	pm.HasSavedCards = false

	if pm.IsBankCard() == true {
		req := &repo.SavedCardRequest{Account: v.order.ProjectAccount, ProjectId: v.order.Project.Id}
		rsp, err := v.service.rep.FindSavedCards(context.TODO(), req)

		if err != nil {
			zap.S().Errorw(
				"[PAYONE_BILLING] Get saved cards from repository failed",
				"error", err,
				"account", v.order.ProjectAccount,
				"project_id", v.order.Project.Id,
				"order_id", v.order.Id,
			)
		} else {
			pm.HasSavedCards = len(rsp.SavedCards) > 0
			pm.SavedCards = []*billing.SavedCard{}

			for _, v := range rsp.SavedCards {
				d := &billing.SavedCard{
					Id:     v.Id,
					Pan:    v.MaskedPan,
					Expire: &billing.CardExpire{Month: v.Expire.Month, Year: v.Expire.Year},
				}

				pm.SavedCards = append(pm.SavedCards, d)
			}

		}
	}

	return nil
}

// Validate data received from payment form and write validated data to order
func (v *PaymentCreateProcessor) processPaymentFormData() error {
	if _, ok := v.data[pkg.PaymentCreateFieldOrderId]; !ok ||
		v.data[pkg.PaymentCreateFieldOrderId] == "" {
		return errors.New(orderErrorCreatePaymentRequiredFieldIdNotFound)
	}

	if _, ok := v.data[pkg.PaymentCreateFieldPaymentMethodId]; !ok ||
		v.data[pkg.PaymentCreateFieldPaymentMethodId] == "" {
		return errors.New(orderErrorCreatePaymentRequiredFieldPaymentMethodNotFound)
	}

	if _, ok := v.data[pkg.PaymentCreateFieldEmail]; !ok ||
		v.data[pkg.PaymentCreateFieldEmail] == "" {
		return errors.New(orderErrorCreatePaymentRequiredFieldEmailNotFound)
	}

	order, err := v.service.getOrderByUuidToForm(v.data[pkg.PaymentCreateFieldOrderId])

	if err != nil {
		return err
	}

	if order.UserAddressDataRequired == true {
		if _, ok := v.data[pkg.PaymentCreateFieldUserCountry]; !ok ||
			v.data[pkg.PaymentCreateFieldUserCountry] == "" {
			return errors.New(orderErrorCreatePaymentRequiredFieldUserCountryNotFound)
		}

		if _, ok := v.data[pkg.PaymentCreateFieldUserCity]; !ok ||
			v.data[pkg.PaymentCreateFieldUserCity] == "" {
			return errors.New(orderErrorCreatePaymentRequiredFieldUserCityNotFound)
		}

		if _, ok := v.data[pkg.PaymentCreateFieldUserZip]; !ok ||
			v.data[pkg.PaymentCreateFieldUserZip] == "" {
			return errors.New(orderErrorCreatePaymentRequiredFieldUserZipNotFound)
		}
	}

	processor := &OrderCreateRequestProcessor{
		Service: v.service,
		request: &billing.OrderCreateRequest{
			ProjectId: order.Project.Id,
			Amount:    order.ProjectIncomeAmount,
		},
		checked: &orderCreateRequestProcessorChecked{
			currency: order.ProjectIncomeCurrency,
		},
	}

	if err := processor.processProject(); err != nil {
		return err
	}

	pm, err := v.service.GetPaymentMethodById(v.data[pkg.PaymentCreateFieldPaymentMethodId])

	if err != nil {
		return errors.New(orderErrorPaymentMethodNotFound)
	}

	if err = processor.processPaymentMethod(pm); err != nil {
		return err
	}

	if err := processor.processLimitAmounts(); err != nil {
		return err
	}

	order.PayerData.Email = v.data[pkg.PaymentCreateFieldEmail]
	order.PaymentRequisites = make(map[string]string)

	if order.UserAddressDataRequired == true {
		if order.BillingAddress == nil {
			order.BillingAddress = &billing.OrderBillingAddress{}
		}

		if order.BillingAddress.Country != v.data[pkg.PaymentCreateFieldUserCountry] {
			order.BillingAddress.Country = v.data[pkg.PaymentCreateFieldUserCountry]
		}

		if order.BillingAddress.City != v.data[pkg.PaymentCreateFieldUserCity] {
			order.BillingAddress.City = v.data[pkg.PaymentCreateFieldUserCity]
		}

		if order.BillingAddress.PostalCode != v.data[pkg.PaymentCreateFieldUserZip] {
			order.BillingAddress.PostalCode = v.data[pkg.PaymentCreateFieldUserZip]
		}

		processor.processOrderVat(order)
	}

	delete(v.data, pkg.PaymentCreateFieldOrderId)
	delete(v.data, pkg.PaymentCreateFieldPaymentMethodId)
	delete(v.data, pkg.PaymentCreateFieldEmail)

	if processor.checked.paymentMethod.IsBankCard() == true {
		if id, ok := v.data[pkg.PaymentCreateFieldStoredCardId]; ok {
			storedCard, err := v.service.rep.FindSavedCardById(context.TODO(), &repo.FindByStringValue{Value: id})

			if err != nil {
				v.service.logError("Get data about stored card failed", []interface{}{"err", err.Error(), "id", id})
			}

			if err != nil || storedCard == nil {
				v.service.logError("Get data about stored card failed", []interface{}{"err", err.Error(), "id", id})
				return errors.New(orderGetSavedCardError)
			}

			order.PaymentRequisites[pkg.PaymentCreateFieldPan] = storedCard.MaskedPan
			order.PaymentRequisites[pkg.PaymentCreateFieldMonth] = storedCard.Expire.Month
			order.PaymentRequisites[pkg.PaymentCreateFieldYear] = storedCard.Expire.Year
			order.PaymentRequisites[pkg.PaymentCreateFieldRecurringId] = storedCard.RecurringId
		} else {
			validator := &bankCardValidator{
				Pan:    v.data[pkg.PaymentCreateFieldPan],
				Cvv:    v.data[pkg.PaymentCreateFieldCvv],
				Month:  v.data[pkg.PaymentCreateFieldMonth],
				Year:   v.data[pkg.PaymentCreateFieldYear],
				Holder: v.data[pkg.PaymentCreateFieldHolder],
			}

			if err := validator.Validate(); err != nil {
				return err
			}

			order.PaymentRequisites[pkg.PaymentCreateFieldPan] = tools.MaskBankCardNumber(v.data[pkg.PaymentCreateFieldPan])
			order.PaymentRequisites[pkg.PaymentCreateFieldMonth] = v.data[pkg.PaymentCreateFieldMonth]

			if len(v.data[pkg.PaymentCreateFieldYear]) < 3 {
				v.data[pkg.PaymentCreateFieldYear] = strconv.Itoa(time.Now().UTC().Year())[:2] + v.data[pkg.PaymentCreateFieldYear]
			}

			order.PaymentRequisites[pkg.PaymentCreateFieldYear] = v.data[pkg.PaymentCreateFieldYear]
		}

		bin := v.service.getBinData(order.PaymentRequisites[pkg.PaymentCreateFieldPan])

		if bin != nil {
			order.PaymentRequisites[paymentCreateBankCardFieldBrand] = bin.CardBrand
			order.PaymentRequisites[paymentCreateBankCardFieldType] = bin.CardType
			order.PaymentRequisites[paymentCreateBankCardFieldCategory] = bin.CardCategory
			order.PaymentRequisites[paymentCreateBankCardFieldIssuerName] = bin.BankName
			order.PaymentRequisites[paymentCreateBankCardFieldIssuerCountry] = bin.BankCountryName
		}
	} else {
		account := ""

		if acc, ok := v.data[pkg.PaymentCreateFieldEWallet]; ok {
			account = acc
		}

		if acc, ok := v.data[pkg.PaymentCreateFieldCrypto]; ok {
			account = acc
		}

		if account == "" {
			return errors.New(paymentSystemErrorEWalletIdentifierIsInvalid)
		}

		order.PaymentRequisites = v.data
	}

	v.checked.project = processor.checked.project
	v.checked.paymentMethod = processor.checked.paymentMethod
	v.checked.order = order

	if order.ProjectAccount == "" {
		order.ProjectAccount = order.PayerData.Email
	}

	return nil
}

func (v *PaymentCreateProcessor) processPaymentAmounts() (err error) {
	order := v.checked.order

	order.ProjectOutcomeAmount, err = v.service.Convert(
		order.PaymentMethodIncomeCurrency.CodeInt,
		order.ProjectOutcomeCurrency.CodeInt,
		order.PaymentMethodOutcomeAmount,
	)

	if err != nil {
		v.service.logError(
			"Convert to project outcome currency failed",
			[]interface{}{
				"error", err.Error(),
				"from", order.PaymentMethodIncomeCurrency.CodeInt,
				"to", order.ProjectOutcomeCurrency.CodeInt,
				"order_id", order.Id,
			},
		)

		return
	}

	order.AmountInPspAccountingCurrency, err = v.service.Convert(
		order.PaymentMethodIncomeCurrency.CodeInt,
		v.service.accountingCurrency.CodeInt,
		order.PaymentMethodOutcomeAmount,
	)

	if err != nil {
		v.service.logError(
			"Convert to PSP accounting currency failed",
			[]interface{}{
				"error", err.Error(),
				"from", order.PaymentMethodIncomeCurrency.CodeInt,
				"to", v.service.accountingCurrency.CodeInt,
				"order_id", order.Id,
			},
		)

		return
	}

	merchantPayoutCurrency := order.Project.Merchant.GetPayoutCurrency()

	if merchantPayoutCurrency != nil {
		order.AmountOutMerchantAccountingCurrency, err = v.service.Convert(
			order.PaymentMethodIncomeCurrency.CodeInt,
			merchantPayoutCurrency.CodeInt,
			order.PaymentMethodOutcomeAmount,
		)

		if err != nil {
			v.service.logError(
				"Convert to merchant accounting currency failed",
				[]interface{}{
					"error", err.Error(),
					"from", order.PaymentMethodIncomeCurrency.CodeInt,
					"to", merchantPayoutCurrency.CodeInt,
					"order_id", order.Id,
				},
			)

			return
		}
	}

	order.AmountInPaymentSystemAccountingCurrency, err = v.service.Convert(
		order.PaymentMethodIncomeCurrency.CodeInt,
		order.PaymentMethod.GetAccountingCurrency().CodeInt,
		order.PaymentMethodOutcomeAmount,
	)

	if err != nil {
		v.service.logError(
			"Convert to payment system accounting currency failed",
			[]interface{}{
				"error", err.Error(),
				"from", order.PaymentMethodIncomeCurrency.CodeInt,
				"to", order.PaymentMethod.GetAccountingCurrency().CodeInt,
				"order_id", order.Id,
			},
		)
	}

	return
}
