package billing

import "github.com/paysuper/paysuper-billing-server/pkg"

func (m *Merchant) ChangesAllowed() bool {
	return m.Status == pkg.MerchantStatusDraft || m.Status == pkg.MerchantStatusRejected
}
