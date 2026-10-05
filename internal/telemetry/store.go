package telemetry

import (
	"context"
	"errors"
	"time"
)

var (
	ErrDeviceNotFound   = errors.New("device not found")
	ErrPositionNotFound = errors.New("position not found")
)

// Store defines persistence operations for devices and telemetry positions.
type Store interface {
	IngestPosition(ctx context.Context, pos *Position) error
	ListDevices(ctx context.Context, activeThreshold time.Duration) ([]DeviceSummary, error)
	GetLatestPosition(ctx context.Context, deviceID string) (*Position, error)
	ListPositions(ctx context.Context, filter PositionsFilter) (*PaginatedPositions, error)
}
