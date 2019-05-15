package service

import (
	"context"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
)

func (s *Service) ProcessSettlementReport(
	ctx context.Context,
	req *grpc.ProcessSettlementReportRequest,
	rsp *grpc.CheckProjectRequestSignatureResponse,
) error {
	return nil
}

func (s *Service) SetSettlementReportLoadToPause(
	ctx context.Context,
	req *grpc.SettlementRequest,
	rsp *grpc.CheckProjectRequestSignatureResponse,
) error {
	return nil
}

func (s *Service) IsSettlementReportLoadOnPause(
	ctx context.Context,
	req *grpc.SettlementRequest,
	rsp *grpc.IsSettlementReportLoadOnPauseResponse,
) error {
	return nil
}
