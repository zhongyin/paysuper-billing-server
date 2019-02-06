package service

import (
	"fmt"
	"github.com/ProtocolONE/payone-billing-service/pkg/proto/billing"
	"github.com/globalsign/mgo/bson"
)

type Project Currency

func newProjectHandler(svc *Service) Cacher {
	c := &Project{svc: svc}

	return c
}

func (h *Project) setCache(recs []interface{}) {
	h.svc.projectCache = make(map[string]*billing.Project)

	for _, r := range recs {
		project := r.(*billing.Project)

		h.svc.mx.Lock()
		h.svc.projectCache[project.Id] = project
		h.svc.mx.Unlock()
	}
}

func (h *Project) getAll() (recs []interface{}, err error) {
	var data []*billing.Project

	err = h.svc.db.Collection(collectionProject).Find(bson.M{}).All(&data)

	if data != nil {
		for _, v := range data {
			recs = append(recs, v)
		}
	}

	return
}

func (s *Service) GetProjectById(id string) (*billing.Project, error) {
	rec, ok := s.projectCache[id]

	if !ok {
		return nil, fmt.Errorf(errorNotFound, collectionProject)
	}

	return rec, nil
}

