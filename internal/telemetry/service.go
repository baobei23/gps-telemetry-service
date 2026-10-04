package telemetry

import "context"

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
