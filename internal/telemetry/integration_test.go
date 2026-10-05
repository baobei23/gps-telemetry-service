package telemetry_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gps-telemetry-service/internal/telemetry"
)

func setupIntegrationService(t *testing.T) (*telemetry.Service, *pgxpool.Pool) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}
	user := os.Getenv("POSTGRES_USERNAME")
	if user == "" {
		user = "postgres"
	}
	pass := os.Getenv("POSTGRES_PASSWORD")
	if pass == "" {
		pass = "postgres"
	}
	dbName := os.Getenv("POSTGRES_STORENAME")
	if dbName == "" {
		dbName = "postgres"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, dbName)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skipf("skipping integration test: failed to configure pool: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("skipping integration test: database unreachable at %s:%s (%v)", host, port, err)
	}

	t.Cleanup(func() {
		// Clean up test devices and cascade positions
		_, _ = pool.Exec(context.Background(), "DELETE FROM devices WHERE id LIKE 'itest-%'")
		pool.Close()
	})

	store := telemetry.NewPostgresStore(pool)
	return telemetry.NewService(store), pool
}

func TestIntegration_WriteAndReadLatest(t *testing.T) {
	svc, _ := setupIntegrationService(t)
	ctx := context.Background()

	devID := fmt.Sprintf("itest-%d", time.Now().UnixNano())
	now := time.Now().UTC().Truncate(time.Microsecond)
	speed := 22.5
	bearing := 180.0
	altitude := 35.0
	accuracy := 4.2
	hdop := 0.75
	batt := 88.0
	charge := true

	pos := &telemetry.Position{
		DeviceID:   devID,
		DeviceTime: now,
		ServerTime: now.Add(50 * time.Millisecond),
		Lat:        -6.175392,
		Lon:        106.827153,
		Valid:      1,
		Speed:      &speed,
		Bearing:    &bearing,
		Altitude:   &altitude,
		Accuracy:   &accuracy,
		HDOP:       &hdop,
		Batt:       &batt,
		Charge:     &charge,
	}

	// 1. Ingest position
	if err := svc.Ingest(ctx, pos); err != nil {
		t.Fatalf("failed to ingest position: %v", err)
	}

	// 2. Read latest position
	latest, err := svc.GetLatestPosition(ctx, devID)
	if err != nil {
		t.Fatalf("failed to get latest position: %v", err)
	}

	if latest.DeviceID != devID {
		t.Errorf("expected device ID %s, got %s", devID, latest.DeviceID)
	}
	if !latest.DeviceTime.Equal(now) {
		t.Errorf("expected device time %v, got %v", now, latest.DeviceTime)
	}
	if latest.Lat != pos.Lat || latest.Lon != pos.Lon {
		t.Errorf("expected coords (%f, %f), got (%f, %f)", pos.Lat, pos.Lon, latest.Lat, latest.Lon)
	}
	if latest.Speed == nil || *latest.Speed != speed {
		t.Errorf("expected speed %f, got %v", speed, latest.Speed)
	}
	if latest.Charge == nil || *latest.Charge != charge {
		t.Errorf("expected charge %v, got %v", charge, latest.Charge)
	}

	// 3. Verify in ListDevices
	devices, err := svc.ListDevices(ctx, time.Minute)
	if err != nil {
		t.Fatalf("failed to list devices: %v", err)
	}
	var found bool
	for _, d := range devices {
		if d.ID == devID {
			found = true
			if d.Status != "active" {
				t.Errorf("expected status 'active', got %s", d.Status)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected device %s to appear in ListDevices", devID)
	}
}

func TestIntegration_DeviceIsolation(t *testing.T) {
	svc, _ := setupIntegrationService(t)
	ctx := context.Background()

	id1 := fmt.Sprintf("itest-iso1-%d", time.Now().UnixNano())
	id2 := fmt.Sprintf("itest-iso2-%d", time.Now().UnixNano())
	now := time.Now().UTC()

	pos1 := &telemetry.Position{
		DeviceID:   id1,
		DeviceTime: now,
		ServerTime: now,
		Lat:        -6.1,
		Lon:        106.1,
		Valid:      1,
	}
	pos2 := &telemetry.Position{
		DeviceID:   id2,
		DeviceTime: now,
		ServerTime: now,
		Lat:        -6.9,
		Lon:        106.9,
		Valid:      1,
	}

	if err := svc.Ingest(ctx, pos1); err != nil {
		t.Fatalf("failed ingesting pos1: %v", err)
	}
	if err := svc.Ingest(ctx, pos2); err != nil {
		t.Fatalf("failed ingesting pos2: %v", err)
	}

	latest1, err := svc.GetLatestPosition(ctx, id1)
	if err != nil {
		t.Fatalf("failed getting latest1: %v", err)
	}
	latest2, err := svc.GetLatestPosition(ctx, id2)
	if err != nil {
		t.Fatalf("failed getting latest2: %v", err)
	}

	if latest1.DeviceID != id1 || latest1.Lat != -6.1 {
		t.Errorf("device 1 data corrupted by device 2: %+v", latest1)
	}
	if latest2.DeviceID != id2 || latest2.Lat != -6.9 {
		t.Errorf("device 2 data corrupted by device 1: %+v", latest2)
	}
}

func TestIntegration_Idempotency(t *testing.T) {
	svc, pool := setupIntegrationService(t)
	ctx := context.Background()

	devID := fmt.Sprintf("itest-idemp-%d", time.Now().UnixNano())
	ts := time.Now().UTC().Truncate(time.Second)

	pos1 := &telemetry.Position{
		DeviceID:   devID,
		DeviceTime: ts,
		ServerTime: ts,
		Lat:        -6.2,
		Lon:        106.8,
		Valid:      1,
	}

	// Ingest once
	if err := svc.Ingest(ctx, pos1); err != nil {
		t.Fatalf("first ingest failed: %v", err)
	}

	// Ingest exact duplicate (same device_id and device_time)
	posDuplicate := &telemetry.Position{
		DeviceID:   devID,
		DeviceTime: ts,
		ServerTime: ts.Add(time.Second),
		Lat:        -6.3, // Attempt to mutate coords
		Lon:        106.9,
		Valid:      1,
	}
	if err := svc.Ingest(ctx, posDuplicate); err != nil {
		t.Fatalf("duplicate ingest should not return error, got: %v", err)
	}

	// Verify only 1 row exists in positions table for this device
	var count int
	err := pool.QueryRow(ctx, "SELECT count(*) FROM positions WHERE device_id = $1", devID).Scan(&count)
	if err != nil {
		t.Fatalf("failed counting positions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row, got %d", count)
	}

	// Verify initial coordinates were preserved (idempotent ignored second write)
	latest, err := svc.GetLatestPosition(ctx, devID)
	if err != nil {
		t.Fatalf("failed reading latest: %v", err)
	}
	if latest.Lat != -6.2 {
		t.Errorf("expected original lat -6.2, got %f", latest.Lat)
	}
}

func TestIntegration_OutOfOrderTimestamps(t *testing.T) {
	svc, pool := setupIntegrationService(t)
	ctx := context.Background()

	devID := fmt.Sprintf("itest-ooo-%d", time.Now().UnixNano())
	base := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	t1 := base.Add(-time.Hour) // Earliest: 11:00
	t2 := base                 // Middle:   12:00
	t3 := base.Add(time.Hour)  // Latest:   13:00

	// Send in out-of-order sequence: T2 -> T1 -> T3
	for _, ts := range []time.Time{t2, t1, t3} {
		pos := &telemetry.Position{
			DeviceID:   devID,
			DeviceTime: ts,
			ServerTime: time.Now().UTC(),
			Lat:        -6.2,
			Lon:        106.8,
			Valid:      1,
		}
		if err := svc.Ingest(ctx, pos); err != nil {
			t.Fatalf("ingest failed for timestamp %v: %v", ts, err)
		}
	}

	// Verify devices table has correct first_seen_at (t1) and last_seen_at (t3)
	var firstSeen, lastSeen time.Time
	err := pool.QueryRow(ctx, "SELECT first_seen_at, last_seen_at FROM devices WHERE id = $1", devID).Scan(&firstSeen, &lastSeen)
	if err != nil {
		t.Fatalf("failed querying device: %v", err)
	}

	if !firstSeen.Equal(t1) {
		t.Errorf("expected first_seen_at %v, got %v", t1, firstSeen)
	}
	if !lastSeen.Equal(t3) {
		t.Errorf("expected last_seen_at %v, got %v", t3, lastSeen)
	}
}

func TestIntegration_CursorPagination(t *testing.T) {
	svc, _ := setupIntegrationService(t)
	ctx := context.Background()

	devID := fmt.Sprintf("itest-page-%d", time.Now().UnixNano())
	base := time.Now().UTC().Truncate(time.Second)
	totalPositions := 15

	// Ingest 15 sequential positions (s0 = base, s1 = base - 1s, ...)
	for i := 0; i < totalPositions; i++ {
		pos := &telemetry.Position{
			DeviceID:   devID,
			DeviceTime: base.Add(-time.Duration(i) * time.Second),
			ServerTime: time.Now().UTC(),
			Lat:        -6.2 + (float64(i) * 0.001),
			Lon:        106.8 + (float64(i) * 0.001),
			Valid:      1,
		}
		if err := svc.Ingest(ctx, pos); err != nil {
			t.Fatalf("failed ingesting position %d: %v", i, err)
		}
	}

	// Paginate with limit 5
	pageSize := 5
	var cursor *string
	collectedIDs := make([]int64, 0, totalPositions)
	var lastTime time.Time

	for page := 1; ; page++ {
		filter := telemetry.PositionsFilter{
			DeviceID: devID,
			Limit:    pageSize,
		}
		if cursor != nil {
			cTime, err := telemetry.DecodeCursor(*cursor)
			if err != nil {
				t.Fatalf("page %d: failed to decode cursor %q: %v", page, *cursor, err)
			}
			filter.CursorTime = &cTime
		}

		res, err := svc.ListPositions(ctx, filter)
		if err != nil {
			t.Fatalf("page %d: ListPositions failed: %v", page, err)
		}

		if len(res.Positions) == 0 {
			t.Fatalf("page %d: expected items, got 0", page)
		}

		for _, p := range res.Positions {
			// Verify descending order
			if !lastTime.IsZero() && !p.DeviceTime.Before(lastTime) {
				t.Errorf("pagination order broken: %v is not before %v", p.DeviceTime, lastTime)
			}
			lastTime = p.DeviceTime
			collectedIDs = append(collectedIDs, p.ID)
		}

		if res.NextCursor == nil {
			// End of pagination reached
			break
		}
		cursor = res.NextCursor
	}

	// Verify all 15 positions were retrieved without duplicates or gaps
	if len(collectedIDs) != totalPositions {
		t.Fatalf("expected total %d items retrieved, got %d", totalPositions, len(collectedIDs))
	}

	idMap := make(map[int64]bool)
	for _, id := range collectedIDs {
		if idMap[id] {
			t.Errorf("duplicate position ID %d found in cursor pagination", id)
		}
		idMap[id] = true
	}
}

func TestIntegration_UnknownDeviceNotFound(t *testing.T) {
	svc, _ := setupIntegrationService(t)
	ctx := context.Background()

	unknownID := "itest-non-existent-device"

	_, err := svc.GetLatestPosition(ctx, unknownID)
	if !errors.Is(err, telemetry.ErrDeviceNotFound) {
		t.Errorf("expected ErrDeviceNotFound from GetLatestPosition, got %v", err)
	}

	_, err = svc.ListPositions(ctx, telemetry.PositionsFilter{DeviceID: unknownID})
	if !errors.Is(err, telemetry.ErrDeviceNotFound) {
		t.Errorf("expected ErrDeviceNotFound from ListPositions, got %v", err)
	}
}
