package telemetry

import (
	"context"
	"errors"
)

var (
	ErrDeviceNotFound   = errors.New("device not found")
	ErrPositionNotFound = errors.New("position not found")
)

// Store defines persistence operations for devices and telemetry positions.
type Store interface {
	IngestPosition(ctx context.Context, pos *Position) error
}
