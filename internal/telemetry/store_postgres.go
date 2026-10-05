package telemetry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

// ListDevices returns all devices ordered by last_seen_at DESC with active status.
func (s *PostgresStore) ListDevices(ctx context.Context, activeThreshold time.Duration) ([]DeviceSummary, error) {
	const query = `
		SELECT id, first_seen_at, last_seen_at
		FROM devices
		ORDER BY last_seen_at DESC;
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query devices: %w", err)
	}
	defer rows.Close()

	summaries := make([]DeviceSummary, 0)
	now := time.Now()

	for rows.Next() {
		var d DeviceSummary
		if err := rows.Scan(&d.ID, &d.FirstSeenAt, &d.LastSeenAt); err != nil {
			return nil, fmt.Errorf("failed to scan device row: %w", err)
		}

		if now.Sub(d.LastSeenAt) <= activeThreshold {
			d.Status = "active"
		} else {
			d.Status = "inactive"
		}

		summaries = append(summaries, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during devices iteration: %w", err)
	}

	return summaries, nil
}

// GetLatestPosition returns the latest position for a device or ErrDeviceNotFound if device does not exist.
func (s *PostgresStore) GetLatestPosition(ctx context.Context, deviceID string) (*Position, error) {
	const query = `
		SELECT id, device_id, device_time, server_time, lat, lon, valid,
		       speed, bearing, altitude, accuracy, hdop, batt, charge
		FROM positions
		WHERE device_id = $1
		ORDER BY device_time DESC
		LIMIT 1;
	`
	var p Position
	err := s.pool.QueryRow(ctx, query, deviceID).Scan(
		&p.ID,
		&p.DeviceID,
		&p.DeviceTime,
		&p.ServerTime,
		&p.Lat,
		&p.Lon,
		&p.Valid,
		&p.Speed,
		&p.Bearing,
		&p.Altitude,
		&p.Accuracy,
		&p.HDOP,
		&p.Batt,
		&p.Charge,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Check if device itself exists
			var exists bool
			_ = s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM devices WHERE id = $1)", deviceID).Scan(&exists)
			if !exists {
				return nil, ErrDeviceNotFound
			}
			return nil, ErrPositionNotFound
		}
		return nil, fmt.Errorf("failed to query latest position: %w", err)
	}

	return &p, nil
}

// ListPositions retrieves paginated position history within an optional time range using keyset cursor pagination.
func (s *PostgresStore) ListPositions(ctx context.Context, filter PositionsFilter) (*PaginatedPositions, error) {
	// First verify device exists
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM devices WHERE id = $1)", filter.DeviceID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check device existence: %w", err)
	}
	if !exists {
		return nil, ErrDeviceNotFound
	}

	const query = `
		SELECT id, device_id, device_time, server_time, lat, lon, valid,
		       speed, bearing, altitude, accuracy, hdop, batt, charge
		FROM positions
		WHERE device_id = $1
		  AND ($2::timestamptz IS NULL OR device_time >= $2)
		  AND ($3::timestamptz IS NULL OR device_time <= $3)
		  AND ($4::timestamptz IS NULL OR device_time < $4)
		ORDER BY device_time DESC
		LIMIT $5;
	`

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}

	// Fetch limit + 1 to check if there is a next page
	fetchLimit := limit + 1

	rows, err := s.pool.Query(ctx, query,
		filter.DeviceID,
		filter.From,
		filter.To,
		filter.CursorTime,
		fetchLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query positions: %w", err)
	}
	defer rows.Close()

	items := make([]Position, 0)
	for rows.Next() {
		var p Position
		if err := rows.Scan(
			&p.ID,
			&p.DeviceID,
			&p.DeviceTime,
			&p.ServerTime,
			&p.Lat,
			&p.Lon,
			&p.Valid,
			&p.Speed,
			&p.Bearing,
			&p.Altitude,
			&p.Accuracy,
			&p.HDOP,
			&p.Batt,
			&p.Charge,
		); err != nil {
			return nil, fmt.Errorf("failed to scan position row: %w", err)
		}
		items = append(items, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error during positions iteration: %w", err)
	}

	var nextCursor *string
	if len(items) > limit {
		// Truncate the extra peeked item
		items = items[:limit]
		// Bookmark is the last item included in the current page
		lastItem := items[len(items)-1]
		c := EncodeCursor(lastItem.DeviceTime)
		nextCursor = &c
	}

	return &PaginatedPositions{
		Positions:  items,
		NextCursor: nextCursor,
	}, nil
}

// Ping verifies the database connection pool is healthy.
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}
