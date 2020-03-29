package base

import (
	"context"
)

type Service interface {
	Check(ctx context.Context) (bool, error)
}

type baseService struct {
	service Service
}

func NewService() Service {
	return baseService{}
}

func (s baseService) Check(ctx context.Context) (bool, error) {
	return true, nil
}
