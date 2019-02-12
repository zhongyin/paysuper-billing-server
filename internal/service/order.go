package service

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ProtocolONE/geoip-service/pkg/proto"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/ProtocolONE/payone-repository/pkg/constant"
	"github.com/ProtocolONE/payone-repository/tools"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"sort"
	"strings"
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
	orderErrorOrderPaymentMethodIncomeCurrencyNotFound = "unknown currency received from payment system"
	orderErrorOrderPSPAccountingCurrencyNotFound       = "unknown PSP accounting currency"
	orderErrorOrderDeclined                            = "payment system decline order with specified identifier early"
	orderErrorOrderCanceled                            = "payment system cancel order with specified identifier early"
	orderErrorCurrencyIsRequired                       = "parameter currency in create order request is required"

	orderErrorCreatePaymentRequiredFieldIdNotFound            = "required field with order identifier not found"
	orderErrorCreatePaymentRequiredFieldPaymentMethodNotFound = "required field with payment method identifier not found"
	orderErrorCreatePaymentRequiredFieldEmailNotFound         = "required field \"email\" not found"

	orderSignatureElementsGlue = "|"

	orderDefaultDescription = "Payment by order # %s"
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

func (s *Service) OrderCreateProcess(ctx context.Context, req *billing.OrderCreateRequest, rsp *billing.Order) error {
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
		if err := processor.processPaymentMethod(); err != nil {
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

	err = s.db.Collection(collectionOrder).Insert(order)

	if err != nil {
		s.log.Errorw(fmt.Sprintf(errorQueryMask, collectionOrder), "err", err, "inserted_data", order)
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

	return nil
}

func (v *OrderCreateRequestProcessor) prepareOrder() (*billing.Order, error) {
	id := bson.NewObjectId().Hex()
	amount := tools.FormatAmount(v.request.Amount)
	merAccAmount := amount

	if v.checked.currency.CodeInt != v.checked.project.Merchant.Currency.CodeInt {
		amount, err := v.Service.Convert(v.checked.currency.CodeInt, v.checked.project.Merchant.Currency.CodeInt, v.request.Amount)

		if err != nil {
			return nil, err
		}

		merAccAmount = tools.FormatAmount(amount)
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
		AmountInMerchantAccountingCurrency: tools.FormatAmount(merAccAmount),
		PaymentMethodOutcomeAmount:         amount,
		PaymentMethodOutcomeCurrency:       v.checked.currency,
		PaymentMethodIncomeAmount:          amount,
		PaymentMethodIncomeCurrency:        v.checked.currency,
	}

	if v.request.Description != "" {
		order.Description = v.request.Description
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

	return order, nil
}

func (v *OrderCreateRequestProcessor) processProject() error {
	project, err := v.GetProjectById(v.request.ProjectId)

	if err != nil {
		v.log.Errorw("[PAYONE_BILLING] Order create get project error", "err", err, "request", v.request)
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
		v.log.Errorw("[PAYONE_BILLING] Order create get currency error", "err", err, "request", v.request)
		return errors.New(orderErrorCurrencyNotFound)
	}

	v.checked.currency = currency

	return nil
}

func (v *OrderCreateRequestProcessor) processPayerData() error {
	rsp, err := v.geo.GetIpData(context.TODO(), &proto.GeoIpDataRequest{IP: v.request.PayerIp})

	if err != nil {
		v.log.Errorw("[PAYONE_BILLING] Order create get payer data error", "err", err, "ip", v.request.PayerIp)
		return errors.New(orderErrorPayerRegionUnknown)
	}

	data := &billing.PayerData{
		Ip:            v.request.PayerIp,
		CountryCodeA2: rsp.Country.IsoCode,
		CountryName:   &billing.Name{En: rsp.Country.Names["en"], Ru: rsp.Country.Names["ru"]},
		City:          &billing.Name{En: rsp.City.Names["en"], Ru: rsp.City.Names["ru"]},
		Timezone:      rsp.Location.TimeZone,
		Email:         v.request.PayerEmail,
		Phone:         v.request.PayerPhone,
	}

	if len(rsp.Subdivisions) > 0 {
		data.Subdivision = rsp.Subdivisions[0].IsoCode
	}

	v.checked.payerData = data

	return nil
}

func (v *OrderCreateRequestProcessor) processFixedPackage() error {
	fps := v.checked.project.GetFixedPackage()

	if len(fps) <= 0 {
		return errors.New(orderErrorFixedPackagesIsEmpty)
	}

	region := v.checked.payerData.CountryCodeA2

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
			(v.checked.currency != nil && val.CurrencyA3 != v.checked.currency.CodeA3) {
			continue
		}

		fp = val
	}

	if fp == nil {
		return errors.New(orderErrorFixedPackageNotFound)
	}

	if v.checked.currency == nil {
		currency, err := v.GetCurrencyByCodeA3(fp.CurrencyA3)

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

	err := v.db.Collection(collectionOrder).Find(filter).One(&order)

	if err != nil && err != mgo.ErrNotFound {
		v.log.Errorw("[PAYONE_BILLING] Order create check project order id unique", "err", err, "filter", filter)
		return errors.New(orderErrorCanNotCreate)
	}

	if order != nil {
		return errors.New(orderErrorProjectOrderIdIsDuplicate)
	}

	return nil
}

func (v *OrderCreateRequestProcessor) processPaymentMethod() error {
	pm, err := v.GetPaymentMethodByGroupAndCurrency(v.request.PaymentMethod, v.checked.currency.CodeInt)

	if err != nil {
		return errors.New(orderErrorPaymentMethodNotFound)
	}

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

// Calculate all possible commissions for order, i.e. payment system fee amount, PSP (P1) fee amount,
// commission shifted from project to user and VAT
func (v *OrderCreateRequestProcessor) processOrderCommissions(o *billing.Order) error {
	pmOutAmount := o.PaymentMethodOutcomeAmount

	// if merchant enable VAT calculation then we're need to calculate VAT for payer
	if o.Project.Merchant.IsVatEnabled == true {
		vat, err := v.Service.CalculateVat(o.PaymentMethodOutcomeAmount, o.PayerData.CountryCodeA2, o.PayerData.Subdivision)

		if err != nil {
			return err
		}

		o.VatAmount = tools.FormatAmount(vat)

		// add VAT amount to payment amount
		pmOutAmount += o.VatAmount
	}

	// calculate commissions to selected payment method
	commission, err := v.Service.CalculateCommission(o.Project.Id, o.PaymentMethod.Id, o.PaymentMethodOutcomeAmount)

	if err != nil {
		return err
	}

	mAccCur := o.Project.Merchant.Currency.CodeInt
	pmOutCur := o.PaymentMethodOutcomeCurrency.CodeInt

	totalCommission := commission.PMCommission + commission.PspCommission

	// if merchant enable to shift commissions form project to payer then we're need to calculate commissions shifting
	if o.Project.Merchant.IsCommissionToUserEnabled == true {
		// subtract commission to user from project's commission
		totalCommission -= commission.ToUserCommission

		// add commission to user to payment amount
		pmOutAmount += commission.ToUserCommission
		o.ToPayerFeeAmount = &billing.OrderFee{AmountPaymentMethodCurrency: tools.FormatAmount(commission.ToUserCommission)}

		// convert amount of fee shifted to user to accounting currency of merchant
		amount, err := v.Service.Convert(pmOutCur, mAccCur, commission.ToUserCommission)

		if err != nil {
			return err
		}

		o.ToPayerFeeAmount.AmountMerchantCurrency = tools.FormatAmount(amount)
	}

	o.ProjectFeeAmount = &billing.OrderFee{AmountPaymentMethodCurrency: tools.FormatAmount(totalCommission)}

	// convert amount of fee to project to accounting currency of merchant
	amount, err := v.Service.Convert(pmOutCur, mAccCur, commission.ToUserCommission)

	if err != nil {
		return err
	}

	o.ProjectFeeAmount.AmountMerchantCurrency = tools.FormatAmount(amount)
	o.PspFeeAmount = &billing.OrderFeePsp{AmountPaymentMethodCurrency: commission.PspCommission}

	// convert PSP amount of fee to accounting currency of merchant
	amount, _ = v.Service.Convert(pmOutCur, mAccCur, commission.PspCommission)

	o.PspFeeAmount.AmountMerchantCurrency = tools.FormatAmount(amount)

	// convert PSP amount of fee to accounting currency of PSP
	amount, err = v.Service.Convert(pmOutCur, v.Service.accountingCurrency.CodeInt, commission.PspCommission)

	if err != nil {
		return err
	}

	o.PspFeeAmount.AmountPspCurrency = tools.FormatAmount(amount)

	// save information about payment system commission
	o.PaymentSystemFeeAmount = &billing.OrderFeePaymentSystem{
		AmountPaymentMethodCurrency: tools.FormatAmount(commission.PMCommission),
	}

	// convert payment system amount of fee to accounting currency of payment system
	amount, err = v.Service.Convert(pmOutCur, o.PaymentMethod.PaymentSystem.AccountingCurrency.CodeInt, commission.PMCommission)

	if err != nil {
		return err
	}

	o.PaymentSystemFeeAmount.AmountPaymentSystemCurrency = tools.FormatAmount(amount)

	// convert payment system amount of fee to accounting currency of merchant
	amount, _ = v.Service.Convert(pmOutCur, mAccCur, commission.PMCommission)

	o.PaymentSystemFeeAmount.AmountMerchantCurrency = tools.FormatAmount(amount)
	o.PaymentMethodOutcomeAmount = tools.FormatAmount(pmOutAmount)

	return nil
}
