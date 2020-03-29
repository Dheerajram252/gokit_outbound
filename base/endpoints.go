package base

import (
	"context"
	"errors"
	"github.com/go-kit/kit/endpoint"
)

var ErrBadRequest = errors.New("bad request")

type Endpoints struct {
	Check endpoint.Endpoint
}

func NewServerEndPoints(s Service) Endpoints {
	return Endpoints{
		Check: MakeCheck(s),
	}
}

func MakeCheck(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		return s.Check(ctx)
	}
}
