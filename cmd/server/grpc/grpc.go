package grpc

import (
	"context"

	"github.com/baobei23/goapp/internal/users"
)

type GRPC struct {
	users *users.Users
}

func (gr *GRPC) Shutdown(ctx context.Context) error {
	_ = ctx
	return nil
}

func New(userSvc *users.Users) *GRPC {
	return &GRPC{
		users: userSvc,
	}
}
