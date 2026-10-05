package telemetry

import (
	"context"
	"time"
)

// Service handles telemetry ingestion and retrieval business logic.
type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

// Ingest stores a validated position and ensures the device record is updated.
func (s *Service) Ingest(ctx context.Context, pos *Position) error {
	return s.store.IngestPosition(ctx, pos)
}

// ListDevices returns all tracked devices with their status and latest coordinates.
func (s *Service) ListDevices(ctx context.Context, activeThreshold time.Duration) ([]DeviceSummary, error) {
	if activeThreshold <= 0 {
		activeThreshold = time.Minute // default 60 seconds
	}
	return s.store.ListDevices(ctx, activeThreshold)
}

// GetLatestPosition returns the most recent position fix for a device.
func (s *Service) GetLatestPosition(ctx context.Context, deviceID string) (*Position, error) {
	return s.store.GetLatestPosition(ctx, deviceID)
}

// ListPositions retrieves paginated position history within an optional time range.
func (s *Service) ListPositions(ctx context.Context, filter PositionsFilter) (*PaginatedPositions, error) {
	return s.store.ListPositions(ctx, filter)
}

// Ping checks whether the underlying telemetry store is healthy.
func (s *Service) Ping(ctx context.Context) error {
	return s.store.Ping(ctx)
}
