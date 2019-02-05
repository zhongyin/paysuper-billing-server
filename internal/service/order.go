package service

import (
	"context"
	proto "github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
)

func (s *Service) OrderCreateProcess(ctx context.Context, req *proto.OrderCreateRequest, rsp *proto.Order) error {
	return nil
}
