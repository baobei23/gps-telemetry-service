package grpc

import (
	"context"
)

type GRPC struct{}

func (gr *GRPC) Shutdown(ctx context.Context) error {
	_ = ctx
	return nil
}

func New() *GRPC {
	return &GRPC{}
}
