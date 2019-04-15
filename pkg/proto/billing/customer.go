package billing

import (
	"github.com/golang/protobuf/ptypes"
	"time"
)

func (m *Customer) IsEmptyRequest() bool {
	return m.ExternalId == "" && m.Email == "" && m.Phone == "" && m.Token == ""
}

func (m *Customer) IsTokenExpired() bool {
	if m.ExpireAt == nil {
		return true
	}

	t, err := ptypes.Timestamp(m.ExpireAt)

	if err != nil {
		return true
	}

	return t.Before(time.Now())
}
