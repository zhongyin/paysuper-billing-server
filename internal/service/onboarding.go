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
	merchantErrorAgreementRequested          = "agreement for merchant can't be requested"
	merchantErrorOnReview                    = "merchant hasn't allowed status for review"
	merchantErrorSigning                     = "signing uncompleted merchant is impossible"
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

func (s *Service) ListMerchants(
	ctx context.Context,
	req *grpc.MerchantListingRequest,
	rsp *grpc.MerchantListingResponse,
) error {
	var merchants []*billing.Merchant
	query := make(bson.M)

	if req.QuickSearch != "" {
		query["$or"] = []bson.M{
			{"name": bson.RegEx{Pattern: ".*" + req.QuickSearch + ".*", Options: "i"}},
			{"user.email": bson.RegEx{Pattern: ".*" + req.QuickSearch + ".*", Options: "i"}},
		}
	} else {
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
	}

	if len(req.Statuses) > 0 {
		query["status"] = bson.M{"$in": req.Statuses}
	}

	count, err := s.db.Collection(pkg.CollectionMerchant).Find(query).Count()

	if err != nil {
		s.logError("Query to count merchants failed", []interface{}{"err", err.Error(), "query", query})
		return errors.New(merchantErrorUnknown)
	}

	err = s.db.Collection(pkg.CollectionMerchant).Find(query).Sort(req.Sort...).Limit(int(req.Limit)).
		Skip(int(req.Offset)).All(&merchants)

	if err != nil {
		s.logError("Query to find merchants failed", []interface{}{"err", err.Error(), "query", query})
		return errors.New(merchantErrorUnknown)
	}

	rsp.Count = int32(count)
	rsp.Items = []*billing.Merchant{}

	if len(merchants) > 0 {
		rsp.Items = merchants
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

	if isNew || merchant == nil {
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
	s.merchantCache[merchant.Id] = merchant

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

	if req.Status == pkg.MerchantStatusAgreementRequested && merchant.Status != pkg.MerchantStatusDraft {
		return errors.New(merchantErrorAgreementRequested)
	}

	if req.Status == pkg.MerchantStatusOnReview && merchant.Status != pkg.MerchantStatusAgreementRequested {
		return errors.New(merchantErrorOnReview)
	}

	if req.Status == pkg.MerchantStatusAgreementSigning && merchant.CanChangeStatusToSigning() == false {
		return errors.New(merchantErrorSigning)
	}

	if req.Status == pkg.MerchantStatusAgreementSigned && (merchant.Status != pkg.MerchantStatusAgreementSigning ||
		merchant.HasMerchantSignature != true || merchant.HasPspSignature != true) {
		return errors.New(merchantErrorSigned)
	}

	nStatuses := &billing.SystemNotificationStatuses{From: merchant.Status, To: req.Status}
	merchant.Status = req.Status

	if req.Status == pkg.MerchantStatusAgreementSigned {
		merchant.IsSigned = true
	}

	if req.Status == pkg.MerchantStatusDraft {
		merchant.AgreementType = 0
		merchant.HasPspSignature = false
		merchant.HasMerchantSignature = false
		merchant.IsSigned = false
	}

	if title, ok := NotificationStatusChangeTitles[req.Status]; ok {
		_, err = s.addNotification(title, req.Message, merchant.Id, "", nStatuses)

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
	s.merchantCache[merchant.Id] = merchant

	return nil
}

func (s *Service) ChangeMerchantData(
	ctx context.Context,
	req *grpc.ChangeMerchantDataRequest,
	rsp *grpc.ChangeMerchantDataResponse,
) error {
	merchant, err := s.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(req.MerchantId)})

	if err != nil {
		rsp.Status = pkg.ResponseStatusNotFound
		rsp.Message = merchantErrorNotFound

		return nil
	}

	if req.AgreementType > 0 && merchant.AgreementType != req.AgreementType {
		if merchant.ChangesAllowed() == false {
			rsp.Status = pkg.ResponseStatusBadData
			rsp.Message = merchantErrorAgreementTypeSelectNotAllow

			return nil
		}

		nStatuses := &billing.SystemNotificationStatuses{From: merchant.Status, To: pkg.MerchantStatusAgreementRequested}
		_, err = s.addNotification(NotificationStatusChangeTitles[merchant.Status], "", merchant.Id, "", nStatuses)

		if err != nil {
			s.logError("Add notification failed", []interface{}{"err", err.Error(), "data", merchant})
		}

		merchant.Status = pkg.MerchantStatusAgreementRequested
		merchant.AgreementType = req.AgreementType
	}

	merchant.HasPspSignature = req.HasPspSignature
	merchant.HasMerchantSignature = req.HasMerchantSignature
	merchant.AgreementSentViaMail = req.AgreementSentViaMail
	merchant.MailTrackingLink = req.MailTrackingLink
	merchant.IsSigned = merchant.HasPspSignature == true && merchant.HasMerchantSignature == true

	if merchant.NeedMarkESignAgreementAsSigned() == true {
		merchant.Status = pkg.MerchantStatusAgreementSigned
	}

	err = s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)

	if err != nil {
		s.logError("Query to change merchant data failed", []interface{}{"err", err.Error(), "data", merchant})
		return errors.New(merchantErrorUnknown)
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = merchant

	s.merchantCache[merchant.Id] = merchant

	return nil
}

func (s *Service) SetMerchantS3Agreement(
	ctx context.Context,
	req *grpc.SetMerchantS3AgreementRequest,
	rsp *grpc.ChangeMerchantDataResponse,
) error {
	merchant, err := s.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(req.MerchantId)})

	if err != nil {
		rsp.Status = pkg.ResponseStatusNotFound
		rsp.Message = merchantErrorNotFound

		return nil
	}

	merchant.S3AgreementName = req.S3AgreementName

	err = s.db.Collection(pkg.CollectionMerchant).UpdateId(bson.ObjectIdHex(merchant.Id), merchant)

	if err != nil {
		s.logError("Query to change merchant data failed", []interface{}{"err", err.Error(), "data", merchant})
		return errors.New(merchantErrorUnknown)
	}

	rsp.Status = pkg.ResponseStatusOk
	rsp.Item = merchant

	s.merchantCache[merchant.Id] = merchant

	return nil
}

func (s *Service) CreateNotification(
	ctx context.Context,
	req *grpc.NotificationRequest,
	rsp *billing.Notification,
) error {
	_, err := s.getMerchantBy(bson.M{"_id": bson.ObjectIdHex(req.MerchantId)})

	if err != nil {
		return err
	}

	if req.UserId == "" || bson.IsObjectIdHex(req.UserId) == false {
		return errors.New(notificationErrorUserIdIncorrect)
	}

	if req.Message == "" {
		return errors.New(notificationErrorMessageIsEmpty)
	}

	n, err := s.addNotification(req.Title, req.Message, req.MerchantId, req.UserId, nil)

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

	if req.IsSystem > 0 {
		if req.IsSystem == 1 {
			query["is_system"] = false
		} else {
			query["is_system"] = true
		}
	}

	count, err := s.db.Collection(pkg.CollectionNotification).Find(query).Count()

	if err != nil {
		s.logError("Query to count merchant notifications failed", []interface{}{"err", err.Error(), "query", query})
		return errors.New(orderErrorUnknown)
	}

	err = s.db.Collection(pkg.CollectionNotification).Find(query).Sort(req.Sort...).
		Limit(int(req.Limit)).Skip(int(req.Offset)).All(&notifications)

	if err != nil {
		if err != mgo.ErrNotFound {
			s.logError("Query to find notifications failed", []interface{}{"err", err.Error(), "query", query})
			return errors.New(orderErrorUnknown)
		}

		return nil
	}

	rsp.Count = int32(count)
	rsp.Items = []*billing.Notification{}

	if len(notifications) > 0 {
		rsp.Items = notifications
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
		Fee: DefaultPaymentMethodFee,
		PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
			Fee:      DefaultPaymentMethodPerTransactionFee,
			Currency: DefaultPaymentMethodCurrency,
		},
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

	err := s.db.Collection(pkg.CollectionPaymentMethod).Find(query).Sort(req.Sort...).All(&pms)

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
				Fee: DefaultPaymentMethodFee,
				PerTransaction: &billing.MerchantPaymentMethodPerTransactionCommission{
					Fee:      DefaultPaymentMethodPerTransactionFee,
					Currency: DefaultPaymentMethodCurrency,
				},
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

	s.merchantCache[merchant.Id] = merchant

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
	rsp.S3AgreementName = merchant.S3AgreementName
	rsp.AgreementType = merchant.AgreementType
	rsp.AgreementSentViaMail = merchant.AgreementSentViaMail
	rsp.MailTrackingLink = merchant.MailTrackingLink
}

func (s *Service) addNotification(
	title, msg, merchantId, userId string,
	nStatuses *billing.SystemNotificationStatuses,
) (*billing.Notification, error) {
	if merchantId == "" || bson.IsObjectIdHex(merchantId) == false {
		return nil, errors.New(notificationErrorMerchantIdIncorrect)
	}

	notification := &billing.Notification{
		Id:         bson.NewObjectId().Hex(),
		Title:      title,
		Message:    msg,
		MerchantId: merchantId,
		IsRead:     false,
		Statuses:   nStatuses,
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
