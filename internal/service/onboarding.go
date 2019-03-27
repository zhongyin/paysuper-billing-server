package service

import (
	"context"
	"errors"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"time"
)

const (
	merchantErrorChangeNotAllowed            = "merchant data changing not allowed"
	merchantErrorCountryNotFound             = "merchant country not found"
	merchantErrorCurrencyNotFound            = "merchant bank accounting currency not found"
	merchantErrorStatusDraft                 = "merchant status can't be set to draft. draft status allowed only for new merchant"
	merchantErrorAgreementRequested          = "agreement for merchant can't be requested"
	merchantErrorOnReview                    = "merchant hasn't allowed status for review"
	merchantErrorReturnFromReview            = "this action is impossible by workflow"
	merchantErrorSigning                     = "signing unapproved merchant is impossible"
	merchantErrorSigned                      = "document can't be mark as signed"
	merchantErrorUnknown                     = "request processing failed. try request later"
	merchantErrorNotFound                    = "merchant with specified identifier not found"
	merchantErrorBadData                     = "request data is incorrect"
	merchantErrorAgreementTypeSelectNotAllow = "merchant status not allow select agreement type"
	notificationErrorMerchantIdIncorrect     = "merchant identifier incorrect, notification can't be saved"
	notificationErrorUserIdIncorrect         = "user identifier incorrect, notification can't be saved"
	notificationErrorMessageIsEmpty          = "notification message can't be empty"
	notificationErrorNotFound                = "notification not found"
)

var (
	ErrMerchantNotFound = errors.New(merchantErrorNotFound)

	NotificationStatusChangeTitles = map[int32]string{
		pkg.MerchantStatusDraft:              "New merchant created",
		pkg.MerchantStatusAgreementRequested: "Merchant asked for agreement",
		pkg.MerchantStatusOnReview:           "Merchant on KYC review",
		pkg.MerchantStatusApproved:           "Merchant approved",
		pkg.MerchantStatusRejected:           "Merchant rejected",
		pkg.MerchantStatusAgreementSigning:   "Agreement signing",
		pkg.MerchantStatusAgreementSigned:    "Agreement signed",
	}
)

func (s *Service) GetMerchantBy(
	ctx context.Context,
	req *grpc.GetMerchantByRequest,
	rsp *grpc.MerchantGetMerchantResponse,
) error {
	if req.MerchantId == "" && req.UserId == "" {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = merchantErrorBadData

		return nil
	}

	query := make(bson.M)

	if req.MerchantId != "" {
		query["_id"] = bson.ObjectIdHex(req.MerchantId)
	}

	if req.UserId != "" {
		query["user.id"] = req.UserId
	}

	merchant, err := s.getMerchantBy(query)

	if err != nil {
		rsp.Status = pkg.ResponseStatusNotFound
		rsp.Message = err.Error()

		if err != ErrMerchantNotFound {
			rsp.Status = pkg.ResponseStatusBadData
		}

		return nil
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = merchant

	return nil
}

func (s *Service) ListMerchants(ctx context.Context, req *grpc.MerchantListingRequest, rsp *grpc.Merchants) error {
	var merchants []*billing.Merchant

	query := make(bson.M)

	if req.Name != "" {
		query["name"] = bson.RegEx{Pattern: ".*" + req.Name + ".*", Options: "i"}
	}

	if req.LastPayoutDateFrom > 0 || req.LastPayoutDateTo > 0 {
		payoutDates := make(bson.M)

		if req.LastPayoutDateFrom > 0 {
			payoutDates["$gte"] = time.Unix(req.LastPayoutDateFrom, 0)
		}

		if req.LastPayoutDateTo > 0 {
			payoutDates["$lte"] = time.Unix(req.LastPayoutDateTo, 0)
		}

		query["last_payout.date"] = payoutDates
	}

	if req.IsSigned > 0 {
		if req.IsSigned == 1 {
			query["is_signed"] = false
		} else {
			query["is_signed"] = true
		}
	}

	if req.LastPayoutAmount > 0 {
		query["last_payout.amount"] = req.LastPayoutAmount
	}

	err := s.db.Collection(pkg.CollectionMerchant).Find(query).Sort(req.Sort...).Limit(int(req.Limit)).
		Skip(int(req.Offset)).All(&merchants)

	if err != nil {
		s.logError("Query to find merchants failed", []interface{}{"err", err.Error(), "query", query})
		return errors.New(merchantErrorUnknown)
	}

	if len(merchants) > 0 {
		rsp.Merchants = merchants
	}

	return nil
}

func (s *Service) ChangeMerchant(
	ctx context.Context,
	req *grpc.OnboardingRequest,
	rsp *billing.Merchant,
) (err error) {
	var merchant *billing.Merchant
	var isNew bool

	if req.Id == "" && (req.User == nil || req.User.Id == "") {
		isNew = true
	} else {
		query := make(bson.M)

		if req.Id != "" && req.User.Id != "" {
			query["$or"] = []bson.M{{"_id": bson.ObjectIdHex(req.Id)}, {"user.id": req.User.Id}}
		} else {
			if req.Id != "" {
				query["_id"] = bson.ObjectIdHex(req.Id)
			}

			if req.User.Id != "" {
				query["user.id"] = req.User.Id
			}
		}

		merchant, err = s.getMerchantBy(query)

		if err != nil {
			if err != ErrMerchantNotFound {
				return err
			}

			isNew = true
		}
	}

	if isNew {
		merchant = &billing.Merchant{
			Id:        bson.NewObjectId().Hex(),
			User:      req.User,
			Status:    pkg.MerchantStatusDraft,
			CreatedAt: ptypes.TimestampNow(),
		}
	}

	if merchant.ChangesAllowed() == false {
		return errors.New(merchantErrorChangeNotAllowed)
	}

	if req.Country != "" {
		country, err := s.GetCountryByCodeA2(req.Country)

		if err != nil {
			s.logError("Get country for merchant failed", []interface{}{"err", err.Error(), "request", req})
			return errors.New(merchantErrorCountryNotFound)
		}

		merchant.Country = country
	}

	merchant.Banking = &billing.MerchantBanking{}

	if req.Banking != nil && req.Banking.Currency != "" {
		currency, err := s.GetCurrencyByCodeA3(req.Banking.Currency)

		if err != nil {
			s.logError("Get currency for merchant failed", []interface{}{"err", err.Error(), "request", req})
			return errors.New(merchantErrorCurrencyNotFound)
		}

		merchant.Banking.Currency = currency
	}

	merchant.Name = req.Name
	merchant.AlternativeName = req.AlternativeName
	merchant.Website = req.Website
	merchant.State = req.State
	merchant.Zip = req.Zip
	merchant.City = req.City
	merchant.Address = req.Address
	merchant.AddressAdditional = req.AddressAdditional
	merchant.RegistrationNumber = req.RegistrationNumber
	merchant.TaxId = req.TaxId
	merchant.Contacts = req.Contacts
	merchant.Banking.Name = req.Banking.Name
	merchant.Banking.Address = req.Banking.Address
	merchant.Banking.AccountNumber = req.Banking.AccountNumber
	merchant.Banking.Swift = req.Banking.Swift
	merchant.Banking.Details = req.Banking.Details
	merchant.UpdatedAt = ptypes.TimestampNow()

	if isNew {
		err = s.db.Collection(pkg.CollectionMerchant).Insert(merchant)
	} else {
		err = s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)
	}

	if err != nil {
		s.logError("Query to change merchant data failed", []interface{}{"err", err.Error(), "data", merchant})
		return errors.New(merchantErrorUnknown)
	}

	s.mapMerchantData(rsp, merchant)

	return
}

func (s *Service) ChangeMerchantStatus(
	ctx context.Context,
	req *grpc.MerchantChangeStatusRequest,
	rsp *billing.Merchant,
) error {
	merchant, err := s.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(req.MerchantId)})

	if err != nil {
		return err
	}

	if req.Status == pkg.MerchantStatusDraft {
		return errors.New(merchantErrorStatusDraft)
	}

	if req.Status == pkg.MerchantStatusAgreementRequested && merchant.Status != pkg.MerchantStatusDraft &&
		merchant.Status != pkg.MerchantStatusRejected {
		return errors.New(merchantErrorAgreementRequested)
	}

	if req.Status == pkg.MerchantStatusOnReview && merchant.Status != pkg.MerchantStatusAgreementRequested {
		return errors.New(merchantErrorOnReview)
	}

	if (req.Status == pkg.MerchantStatusApproved || req.Status == pkg.MerchantStatusRejected) &&
		merchant.Status != pkg.MerchantStatusOnReview {
		return errors.New(merchantErrorReturnFromReview)
	}

	if req.Status == pkg.MerchantStatusAgreementSigning && merchant.Status != pkg.MerchantStatusApproved {
		return errors.New(merchantErrorSigning)
	}

	if req.Status == pkg.MerchantStatusAgreementSigned && (merchant.Status != pkg.MerchantStatusAgreementSigning ||
		merchant.HasMerchantSignature != true || merchant.HasPspSignature != true) {
		return errors.New(merchantErrorSigned)
	}

	merchant.Status = req.Status

	if req.Status == pkg.MerchantStatusAgreementSigned {
		merchant.IsSigned = true
	}

	if title, ok := NotificationStatusChangeTitles[req.Status]; ok {
		_, err = s.addNotification(title, req.Message, merchant.Id, "")

		if err != nil {
			return err
		}
	}

	err = s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)

	if err != nil {
		s.logError("Query to change merchant data failed", []interface{}{"err", err.Error(), "data", rsp})
		return errors.New(merchantErrorUnknown)
	}

	s.mapMerchantData(rsp, merchant)

	return nil
}

func (s *Service) ChangeMerchantAgreementType(
	ctx context.Context,
	req *grpc.ChangeMerchantAgreementTypeRequest,
	rsp *grpc.ChangeMerchantAgreementTypeResponse,
) error {
	merchant, err := s.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(req.MerchantId)})

	if err != nil {
		rsp.Status = pkg.ResponseStatusNotFound
		rsp.Message = merchantErrorNotFound

		return nil
	}

	if merchant.SelectAgreementTypeAllow() == false {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = merchantErrorAgreementTypeSelectNotAllow

		return nil
	}

	merchant.Status = pkg.MerchantStatusAgreementSigning
	merchant.AgreementType = req.AgreementType

	err = s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)

	if err != nil {
		s.logError("Query to change merchant data failed", []interface{}{"err", err.Error(), "data", merchant})
		return errors.New(merchantErrorUnknown)
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = merchant

	return nil
}

func (s *Service) CreateNotification(
	ctx context.Context,
	req *grpc.NotificationRequest,
	rsp *billing.Notification,
) error {
	if req.UserId == "" || bson.IsObjectIdHex(req.UserId) == false {
		return errors.New(notificationErrorUserIdIncorrect)
	}

	if req.Message == "" {
		return errors.New(notificationErrorMessageIsEmpty)
	}

	n, err := s.addNotification(req.Title, req.Message, req.MerchantId, req.UserId)

	if err != nil {
		return err
	}

	rsp.Id = n.Id
	rsp.MerchantId = n.MerchantId
	rsp.UserId = n.UserId
	rsp.Title = n.Title
	rsp.Message = n.Message
	rsp.IsRead = n.IsRead
	rsp.IsSystem = n.IsSystem

	return nil
}

func (s *Service) GetNotification(
	ctx context.Context,
	req *grpc.GetNotificationRequest,
	rsp *billing.Notification,
) error {
	notification, err := s.getNotificationById(req.MerchantId, req.NotificationId)

	if err != nil {
		return err
	}

	s.mapNotificationData(rsp, notification)

	return nil
}

func (s *Service) ListNotifications(
	ctx context.Context,
	req *grpc.ListingNotificationRequest,
	rsp *grpc.Notifications,
) error {
	var notifications []*billing.Notification

	query := make(bson.M)

	if req.MerchantId != "" && bson.IsObjectIdHex(req.MerchantId) == true {
		query["merchant_id"] = bson.ObjectIdHex(req.MerchantId)
	}

	if req.UserId != "" && bson.IsObjectIdHex(req.UserId) == true {
		query["user_id"] = bson.ObjectIdHex(req.UserId)
	}

	err := s.db.Collection(pkg.CollectionNotification).Find(query).
		Limit(int(req.Limit)).Skip(int(req.Offset)).All(&notifications)

	if err != nil {
		if err != mgo.ErrNotFound {
			s.logError("Query to find notifications failed", []interface{}{"err", err.Error(), "query", query})
		}

		return nil
	}

	if len(notifications) > 0 {
		rsp.Notifications = notifications
	}

	return nil
}

func (s *Service) MarkNotificationAsRead(
	ctx context.Context,
	req *grpc.GetNotificationRequest,
	rsp *billing.Notification,
) error {
	notification, err := s.getNotificationById(req.MerchantId, req.NotificationId)

	if err != nil {
		return err
	}

	notification.IsRead = true

	err = s.db.Collection(pkg.CollectionNotification).UpdateId(bson.ObjectIdHex(notification.Id), notification)

	if err != nil {
		s.logError("Update notification failed", []interface{}{"err", err.Error(), "query", notification})
		return errors.New(merchantErrorUnknown)
	}

	s.mapNotificationData(rsp, notification)

	return nil
}

func (s *Service) GetMerchantPaymentMethod(
	ctx context.Context,
	req *grpc.GetMerchantPaymentMethodRequest,
	rsp *billing.MerchantPaymentMethod,
) error {
	pms, ok := s.merchantPaymentMethods[req.MerchantId]

	if ok {
		pm, ok := pms[req.PaymentMethodId]

		if ok {
			rsp.PaymentMethod = pm.PaymentMethod
			rsp.Commission = pm.Commission
			rsp.Integration = pm.Integration
			rsp.IsActive = pm.IsActive

			return nil
		}
	}

	pm, err := s.GetPaymentMethodById(req.PaymentMethodId)

	if err != nil {
		s.logError(
			"Payment method with specified id not found in cache",
			[]interface{}{
				"error", err.Error(),
				"id", req.PaymentMethodId,
			},
		)
		return errors.New(orderErrorPaymentMethodNotFound)
	}

	rsp.PaymentMethod = &billing.MerchantPaymentMethodIdentification{
		Id:   pm.Id,
		Name: pm.Name,
	}
	rsp.Commission = &billing.MerchantPaymentMethodCommissions{
		PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{},
	}
	rsp.Integration = &billing.MerchantPaymentMethodIntegration{}
	rsp.IsActive = true

	return nil
}

func (s *Service) ListMerchantPaymentMethods(
	ctx context.Context,
	req *grpc.ListMerchantPaymentMethodsRequest,
	rsp *grpc.ListingMerchantPaymentMethod,
) error {
	var pms []*billing.PaymentMethod

	query := bson.M{"is_active": true}

	if req.PaymentMethodName != "" {
		query["name"] = bson.RegEx{Pattern: ".*" + req.PaymentMethodName + ".*", Options: "i"}
	}

	err := s.db.Collection(pkg.CollectionPaymentMethod).Find(query).All(&pms)

	if err != nil {
		s.logError("Query to find payment methods failed", []interface{}{"error", err.Error(), "query", query})
		return nil
	}

	if len(pms) <= 0 {
		return nil
	}

	mPms, ok := s.merchantPaymentMethods[req.MerchantId]

	for _, pm := range pms {
		mPm, ok1 := mPms[pm.Id]

		paymentMethod := &billing.MerchantPaymentMethod{
			PaymentMethod: &billing.MerchantPaymentMethodIdentification{
				Id:   pm.Id,
				Name: pm.Name,
			},
			Commission: &billing.MerchantPaymentMethodCommissions{
				PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{},
			},
			Integration: &billing.MerchantPaymentMethodIntegration{},
			IsActive:    true,
		}

		if ok && ok1 {
			paymentMethod.Commission = mPm.Commission
			paymentMethod.Integration = mPm.Integration
			paymentMethod.IsActive = mPm.IsActive
		}

		rsp.PaymentMethods = append(rsp.PaymentMethods, paymentMethod)
	}

	return nil
}

func (s *Service) ChangeMerchantPaymentMethod(
	ctx context.Context,
	req *grpc.MerchantPaymentMethodRequest,
	rsp *grpc.MerchantPaymentMethodResponse,
) (err error) {
	merchant, err := s.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(req.MerchantId)})

	if err != nil {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = err.Error()

		return
	}

	pm, ok := s.paymentMethodIdCache[req.PaymentMethod.Id]

	if !ok {
		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = orderErrorPaymentMethodNotFound

		return
	}

	req.Integration.Integrated = req.HasIntegration()

	if req.HasPerTransactionCurrency() {
		if _, ok := s.currencyCache[req.GetPerTransactionCurrency()]; !ok {
			rsp.Status = pkg.ResponseStatusBadData
			rsp.Message = orderErrorCurrencyNotFound

			return
		}
	}

	if len(merchant.PaymentMethods) <= 0 {
		merchant.PaymentMethods = make(map[string]*billing.MerchantPaymentMethod)
	}

	merchant.PaymentMethods[pm.Id] = &billing.MerchantPaymentMethod{
		PaymentMethod: req.PaymentMethod,
		Commission:    req.Commission,
		Integration:   req.Integration,
		IsActive:      req.IsActive,
	}

	err = s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)

	if err != nil {
		s.logError("Query to update merchant payment methods failed", []interface{}{"error", err.Error(), "query", merchant})

		rsp.Status = pkg.ResponseStatusBadData
		rsp.Message = orderErrorUnknown

		return
	}

	s.mx.Lock()
	defer s.mx.Unlock()

	if _, ok := s.merchantPaymentMethods[merchant.Id]; !ok {
		s.merchantPaymentMethods[merchant.Id] = make(map[string]*billing.MerchantPaymentMethod)
	}

	s.merchantPaymentMethods[merchant.Id][pm.Id] = merchant.PaymentMethods[pm.Id]

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = merchant.PaymentMethods[pm.Id]

	return
}

func (s *Service) getMerchantBy(query bson.M) (merchant *billing.Merchant, err error) {
	err = s.db.Collection(pkg.CollectionMerchant).Find(query).One(&merchant)

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Query to find merchant by id failed", []interface{}{"err", err.Error(), "query", query})

		return merchant, errors.New(merchantErrorUnknown)
	}

	if merchant == nil {
		return merchant, ErrMerchantNotFound
	}

	return
}

func (s *Service) mapMerchantData(rsp *billing.Merchant, merchant *billing.Merchant) {
	rsp.Id = merchant.Id
	rsp.User = merchant.User
	rsp.Status = merchant.Status
	rsp.CreatedAt = merchant.CreatedAt
	rsp.Name = merchant.Name
	rsp.AlternativeName = merchant.AlternativeName
	rsp.Website = merchant.Website
	rsp.Country = merchant.Country
	rsp.State = merchant.State
	rsp.Zip = merchant.Zip
	rsp.City = merchant.City
	rsp.Address = merchant.Address
	rsp.AddressAdditional = merchant.AddressAdditional
	rsp.RegistrationNumber = merchant.RegistrationNumber
	rsp.TaxId = merchant.TaxId
	rsp.Contacts = merchant.Contacts
	rsp.Banking = merchant.Banking
	rsp.HasMerchantSignature = merchant.HasMerchantSignature
	rsp.HasPspSignature = merchant.HasPspSignature
	rsp.LastPayout = merchant.LastPayout
	rsp.IsSigned = merchant.IsSigned
	rsp.CreatedAt = merchant.CreatedAt
	rsp.UpdatedAt = merchant.UpdatedAt
}

func (s *Service) addNotification(title, msg, merchantId, userId string) (*billing.Notification, error) {
	if merchantId == "" || bson.IsObjectIdHex(merchantId) == false {
		return nil, errors.New(notificationErrorMerchantIdIncorrect)
	}

	notification := &billing.Notification{
		Id:         bson.NewObjectId().Hex(),
		Title:      title,
		Message:    msg,
		MerchantId: merchantId,
		IsRead:     false,
	}

	if userId == "" || bson.IsObjectIdHex(userId) == false {
		notification.UserId = pkg.SystemUserId
		notification.IsSystem = true
	} else {
		notification.UserId = userId
	}

	err := s.db.Collection(pkg.CollectionNotification).Insert(notification)

	if err != nil {
		s.logError("Query to insert notification failed", []interface{}{"err", err.Error(), "query", notification})
		return nil, errors.New(merchantErrorUnknown)
	}

	return notification, nil
}

func (s *Service) getNotificationById(
	merchantId, notificationId string,
) (notification *billing.Notification, err error) {
	query := bson.M{
		"merchant_id": bson.ObjectIdHex(merchantId),
		"_id":         bson.ObjectIdHex(notificationId),
	}
	err = s.db.Collection(pkg.CollectionNotification).Find(query).One(&notification)

	if err != nil {
		if err != mgo.ErrNotFound {
			s.logError("Query to find notification by id failed", []interface{}{"err", err.Error(), "query", query})
		}

		return notification, errors.New(notificationErrorNotFound)
	}

	if notification == nil {
		return notification, errors.New(notificationErrorNotFound)
	}

	return
}

func (s *Service) mapNotificationData(rsp *billing.Notification, notification *billing.Notification) {
	rsp.Id = notification.Id
	rsp.UserId = notification.UserId
	rsp.MerchantId = notification.MerchantId
	rsp.Message = notification.Message
	rsp.Title = notification.Title
	rsp.IsSystem = notification.IsSystem
	rsp.IsRead = notification.IsRead
	rsp.CreatedAt = notification.CreatedAt
	rsp.UpdatedAt = notification.UpdatedAt
}
