package mock

import (
	"context"
	"errors"
	"github.com/globalsign/mgo/bson"
	"github.com/micro/go-micro/client"
	"github.com/paysuper/paysuper-recurring-repository/pkg/proto/entity"
	"github.com/paysuper/paysuper-recurring-repository/pkg/proto/repository"
)

type RepositoryServiceOk struct{}
type RepositoryServiceEmpty struct{}
type RepositoryServiceError struct{}

func NewRepositoryServiceOk() repository.RepositoryService {
	return &RepositoryServiceOk{}
}

func NewRepositoryServiceEmpty() repository.RepositoryService {
	return &RepositoryServiceEmpty{}
}

func NewRepositoryServiceError() repository.RepositoryService {
	return &RepositoryServiceError{}
}

func (r *RepositoryServiceOk) InsertSavedCard(
	ctx context.Context,
	in *repository.SavedCardRequest,
	opts ...client.CallOption,
) (*repository.Result, error) {
	return &repository.Result{}, nil
}

func (r *RepositoryServiceOk) DeleteSavedCard(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*repository.Result, error) {
	return &repository.Result{}, nil
}

func (r *RepositoryServiceOk) FindSavedCards(
	ctx context.Context,
	in *repository.SavedCardRequest,
	opts ...client.CallOption,
) (*repository.SavedCardList, error) {
	projectId := bson.NewObjectId().Hex()

	return &repository.SavedCardList{
		SavedCards: []*entity.SavedCard{
			{
				Id:        bson.NewObjectId().Hex(),
				Token:     bson.NewObjectId().Hex(),
				ProjectId: projectId,
				MaskedPan: "555555******4444",
				Expire:    &entity.CardExpire{Month: "12", Year: "2019"},
				IsActive:  true,
			},
			{
				Id:        bson.NewObjectId().Hex(),
				Token:     bson.NewObjectId().Hex(),
				ProjectId: projectId,
				MaskedPan: "400000******0002",
				Expire:    &entity.CardExpire{Month: "12", Year: "2019"},
				IsActive:  true,
			},
		},
	}, nil
}

func (r *RepositoryServiceOk) FindSavedCardById(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*entity.SavedCard, error) {
	return &entity.SavedCard{}, nil
}

func (r *RepositoryServiceEmpty) InsertSavedCard(
	ctx context.Context,
	in *repository.SavedCardRequest,
	opts ...client.CallOption,
) (*repository.Result, error) {
	return &repository.Result{}, nil
}

func (r *RepositoryServiceEmpty) DeleteSavedCard(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*repository.Result, error) {
	return &repository.Result{}, nil
}

func (r *RepositoryServiceEmpty) FindSavedCards(
	ctx context.Context,
	in *repository.SavedCardRequest,
	opts ...client.CallOption,
) (*repository.SavedCardList, error) {
	return &repository.SavedCardList{}, nil
}

func (r *RepositoryServiceEmpty) FindSavedCardById(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*entity.SavedCard, error) {
	return &entity.SavedCard{}, nil
}

func (r *RepositoryServiceError) InsertSavedCard(
	ctx context.Context,
	in *repository.SavedCardRequest,
	opts ...client.CallOption,
) (*repository.Result, error) {
	return &repository.Result{}, nil
}

func (r *RepositoryServiceError) DeleteSavedCard(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*repository.Result, error) {
	return &repository.Result{}, nil
}

func (r *RepositoryServiceError) FindSavedCards(
	ctx context.Context,
	in *repository.SavedCardRequest,
	opts ...client.CallOption,
) (*repository.SavedCardList, error) {
	return &repository.SavedCardList{}, errors.New("some error")
}

func (r *RepositoryServiceError) FindSavedCardById(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*entity.SavedCard, error) {
	return &entity.SavedCard{}, nil
}
