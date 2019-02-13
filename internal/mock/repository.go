package mock

import (
	"context"
	"errors"
	"github.com/ProtocolONE/payone-repository/pkg/proto/billing"
	"github.com/ProtocolONE/payone-repository/pkg/proto/repository"
	"github.com/globalsign/mgo/bson"
	"github.com/micro/go-micro/client"
)

type RepositoryServiceOk struct {}
type RepositoryServiceEmpty struct {}
type RepositoryServiceError struct {}

func NewRepositoryServiceOk() repository.RepositoryService {
	return &RepositoryServiceOk{}
}

func NewRepositoryServiceEmpty() repository.RepositoryService {
	return &RepositoryServiceEmpty{}
}

func NewRepositoryServiceError() repository.RepositoryService {
	return &RepositoryServiceError{}
}

func (r *RepositoryServiceOk) FindBinData(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*billing.BinData, error) {
	return &billing.BinData{}, nil
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
		SavedCards: []*billing.SavedCard{
			{
				Id: bson.NewObjectId().Hex(),
				Account: "test@unit.unit",
				ProjectId: projectId,
				Pan: "555555******4444",
				Expire: &billing.CardExpire{Month: "12", Year: "2019"},
				IsActive: true,
			},
			{
				Id: bson.NewObjectId().Hex(),
				Account: "test@unit.unit",
				ProjectId: projectId,
				Pan: "400000******0002",
				Expire: &billing.CardExpire{Month: "12", Year: "2019"},
				IsActive: true,
			},
		},
	}, nil
}

func (r *RepositoryServiceOk) FindSavedCard(
	ctx context.Context, 
	in *repository.SavedCardRequest, 
	opts ...client.CallOption,
) (*billing.SavedCard, error) {
	return &billing.SavedCard{}, nil
}

func (r *RepositoryServiceOk) FindSavedCardById(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*billing.SavedCard, error) {
	return &billing.SavedCard{}, nil
}

func (r *RepositoryServiceEmpty) FindBinData(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*billing.BinData, error) {
	return &billing.BinData{}, nil
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

func (r *RepositoryServiceEmpty) FindSavedCard(
	ctx context.Context,
	in *repository.SavedCardRequest,
	opts ...client.CallOption,
) (*billing.SavedCard, error) {
	return &billing.SavedCard{}, nil
}

func (r *RepositoryServiceEmpty) FindSavedCardById(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*billing.SavedCard, error) {
	return &billing.SavedCard{}, nil
}

func (r *RepositoryServiceError) FindBinData(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*billing.BinData, error) {
	return &billing.BinData{}, nil
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

func (r *RepositoryServiceError) FindSavedCard(
	ctx context.Context,
	in *repository.SavedCardRequest,
	opts ...client.CallOption,
) (*billing.SavedCard, error) {
	return &billing.SavedCard{}, nil
}

func (r *RepositoryServiceError) FindSavedCardById(
	ctx context.Context,
	in *repository.FindByStringValue,
	opts ...client.CallOption,
) (*billing.SavedCard, error) {
	return &billing.SavedCard{}, nil
}