package service

import (
	"context"
	"errors"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-billing-server/pkg"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
)

func (s *Service) ChangeProject(
	ctx context.Context,
	req *billing.Project,
	rsp *grpc.ChangeProjectResponse,
) error {
	prj := &billing.Project{}

	if req.Id != "" {

	}
}

func (s *Service) GetProject() {

}

func (s *Service) ListProjects() {

}

func (s *Service) DeleteProject() {

}

func (s *Service) getProjectBy(query bson.M) (project *billing.Project, err error) {
	err = s.db.Collection(pkg.CollectionProject).Find(query).One(&project)

	if err != nil && err != mgo.ErrNotFound {
		s.logError("Query to find project by failed", []interface{}{"err", err.Error(), "query", query})
		return project, errors.New(merchantErrorUnknown)
	}

	if merchant == nil {
		return merchant, ErrMerchantNotFound
	}

	return
}
