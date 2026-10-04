package telemetry

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// IngestPosition inserts a position and updates the device's first/last seen times atomically.
// It executes in a single round-trip using CTE with idempotency ON CONFLICT DO NOTHING.
func (s *PostgresStore) IngestPosition(ctx context.Context, pos *Position) error {
	const query = `
		WITH ins_device AS (
			INSERT INTO devices (id, first_seen_at, last_seen_at)
			VALUES ($1, $2, $2)
			ON CONFLICT (id) DO UPDATE SET
				last_seen_at = GREATEST(devices.last_seen_at, EXCLUDED.last_seen_at),
				first_seen_at = LEAST(devices.first_seen_at, EXCLUDED.first_seen_at),
				updated_at = NOW()
		)
		INSERT INTO positions (
			device_id, device_time, server_time, lat, lon, valid,
			speed, bearing, altitude, accuracy, hdop, batt, charge
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13
		)
		ON CONFLICT (device_id, device_time) DO NOTHING;
	`
	_, err := s.pool.Exec(ctx, query,
		pos.DeviceID,
		pos.DeviceTime,
		pos.ServerTime,
		pos.Lat,
		pos.Lon,
		pos.Valid,
		pos.Speed,
		pos.Bearing,
		pos.Altitude,
		pos.Accuracy,
		pos.HDOP,
		pos.Batt,
		pos.Charge,
	)
	if err != nil {
		return fmt.Errorf("failed to ingest position: %w", err)
	}
	return nil
}
