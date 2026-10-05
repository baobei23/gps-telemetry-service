package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"gps-telemetry-service/internal/telemetry"
)

type mockTelemetryStore struct {
	ingested []*telemetry.Position
	devices  []telemetry.DeviceSummary
	latest   map[string]*telemetry.Position
	history  map[string][]telemetry.Position
	pingErr  error
}

func (m *mockTelemetryStore) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockTelemetryStore) IngestPosition(ctx context.Context, pos *telemetry.Position) error {
	m.ingested = append(m.ingested, pos)
	return nil
}

func (m *mockTelemetryStore) ListDevices(ctx context.Context, activeThreshold time.Duration) ([]telemetry.DeviceSummary, error) {
	return m.devices, nil
}

func (m *mockTelemetryStore) GetLatestPosition(ctx context.Context, deviceID string) (*telemetry.Position, error) {
	if m.latest != nil {
		if p, ok := m.latest[deviceID]; ok {
			return p, nil
		}
	}
	return nil, telemetry.ErrDeviceNotFound
}

func (m *mockTelemetryStore) ListPositions(ctx context.Context, filter telemetry.PositionsFilter) (*telemetry.PaginatedPositions, error) {
	if m.history == nil {
		return nil, telemetry.ErrDeviceNotFound
	}
	items, ok := m.history[filter.DeviceID]
	if !ok {
		return nil, telemetry.ErrDeviceNotFound
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	var nextCursor *string
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		c := telemetry.EncodeCursor(last.DeviceTime)
		nextCursor = &c
	}

	return &telemetry.PaginatedPositions{
		Positions:  items,
		NextCursor: nextCursor,
	}, nil
}

func setupTestMux(mock *mockTelemetryStore) *http.ServeMux {
	var isReady atomic.Bool
	isReady.Store(true)
	return setupTestMuxWithReady(mock, &isReady)
}

func setupTestMuxWithReady(mock *mockTelemetryStore, isReady *atomic.Bool) *http.ServeMux {
	svc := telemetry.NewService(mock)
	handlers := &Handlers{
		telemetry: svc,
		isReady:   isReady,
	}
	mux := http.NewServeMux()
	handlers.registerRoutes(mux)
	return mux
}

func TestIngestPosition_Success(t *testing.T) {
	mock := &mockTelemetryStore{}
	mux := setupTestMux(mock)

	req := httptest.NewRequest(http.MethodGet, "/?id=357565514527184&valid=1&timestamp=1700000000&lat=-6.200000&lon=106.816666&speed=16.19", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(mock.ingested) != 1 {
		t.Fatalf("expected 1 ingested position, got %d", len(mock.ingested))
	}
	if mock.ingested[0].DeviceID != "357565514527184" {
		t.Fatalf("expected device ID 357565514527184, got %s", mock.ingested[0].DeviceID)
	}
}

func TestIngestPosition_ValidationErrors(t *testing.T) {
	mock := &mockTelemetryStore{}
	mux := setupTestMux(mock)

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "missing id",
			query:      "/?timestamp=1700000000&lat=-6.2&lon=106.8",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing timestamp",
			query:      "/?id=123&lat=-6.2&lon=106.8",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid lat",
			query:      "/?id=123&timestamp=1700000000&lat=95.0&lon=106.8",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid lon",
			query:      "/?id=123&timestamp=1700000000&lat=-6.2&lon=200.0",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.query, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d body=%s", tc.wantStatus, rec.Code, rec.Body.String())
			}

			var res ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
				t.Fatalf("failed decoding json: %v", err)
			}
			if res.Error == "" {
				t.Fatal("expected non-empty error message")
			}
		})
	}
}

func TestListDevices(t *testing.T) {
	mock := &mockTelemetryStore{
		devices: []telemetry.DeviceSummary{
			{
				ID:          "device-1",
				FirstSeenAt: time.Now().Add(-time.Hour),
				LastSeenAt:  time.Now(),
				Status:      "active",
			},
		},
	}
	mux := setupTestMux(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var res BaseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}

	raw, _ := json.Marshal(res.Data)
	var devs []telemetry.DeviceSummary
	if err := json.Unmarshal(raw, &devs); err != nil {
		t.Fatalf("failed decoding devices data: %v", err)
	}
	if len(devs) != 1 || devs[0].ID != "device-1" {
		t.Fatalf("unexpected devices: %+v", devs)
	}
}

func TestGetLatestPosition_Success(t *testing.T) {
	now := time.Now().UTC()
	speed := 15.5
	mock := &mockTelemetryStore{
		latest: map[string]*telemetry.Position{
			"device-1": {
				ID:         1,
				DeviceID:   "device-1",
				DeviceTime: now,
				Lat:        -6.2,
				Lon:        106.8,
				Valid:      1,
				Speed:      &speed,
			},
		},
	}
	mux := setupTestMux(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/devices/device-1/latest", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var res BaseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	raw, _ := json.Marshal(res.Data)
	var pos telemetry.Position
	if err := json.Unmarshal(raw, &pos); err != nil {
		t.Fatalf("failed decoding position: %v", err)
	}
	if pos.DeviceID != "device-1" || pos.Lat != -6.2 {
		t.Fatalf("unexpected position data: %+v", pos)
	}
}

func TestGetLatestPosition_NotFound(t *testing.T) {
	mock := &mockTelemetryStore{}
	mux := setupTestMux(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/devices/unknown-device/latest", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListPositions_Success(t *testing.T) {
	now := time.Now().UTC()
	mock := &mockTelemetryStore{
		history: map[string][]telemetry.Position{
			"device-1": {
				{ID: 2, DeviceID: "device-1", DeviceTime: now, Lat: -6.2, Lon: 106.8},
				{ID: 1, DeviceID: "device-1", DeviceTime: now.Add(-time.Second), Lat: -6.2001, Lon: 106.8001},
			},
		},
	}
	mux := setupTestMux(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/devices/device-1/positions?limit=10", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var res PaginatedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding paginated response: %v", err)
	}
	if res.Pagination.Limit != 10 {
		t.Errorf("expected limit 10, got %d", res.Pagination.Limit)
	}
}

func TestListPositions_NotFound(t *testing.T) {
	mock := &mockTelemetryStore{}
	mux := setupTestMux(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/devices/unknown-device/positions", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestListPositions_InvalidParams(t *testing.T) {
	mock := &mockTelemetryStore{}
	mux := setupTestMux(mock)

	tests := []struct {
		name  string
		query string
	}{
		{"invalid from", "/api/devices/d1/positions?from=not-rfc3339"},
		{"invalid to", "/api/devices/d1/positions?to=invalid-date"},
		{"from after to", "/api/devices/d1/positions?from=2026-10-05T12:00:00Z&to=2026-10-05T10:00:00Z"},
		{"invalid limit", "/api/devices/d1/positions?limit=-10"},
		{"invalid cursor", "/api/devices/d1/positions?cursor=bad!cursor"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.query, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	logged := loggingMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/?id=357565514527184", nil)
	rec := httptest.NewRecorder()

	logged.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}
}

func TestRequestIDMiddleware_Generated(t *testing.T) {
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := requestIDMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	respID := rec.Header().Get("X-Request-ID")
	if respID == "" {
		t.Fatal("expected non-empty X-Request-ID response header")
	}
	if capturedID != respID {
		t.Fatalf("expected context request ID %q to match response header %q", capturedID, respID)
	}
}

func TestRequestIDMiddleware_Preserved(t *testing.T) {
	const customID = "custom-uuid-12345"
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := requestIDMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", customID)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	respID := rec.Header().Get("X-Request-ID")
	if respID != customID {
		t.Fatalf("expected preserved X-Request-ID %q, got %q", customID, respID)
	}
	if capturedID != customID {
		t.Fatalf("expected context request ID %q, got %q", customID, capturedID)
	}
}

func TestPanicRecovery(t *testing.T) {
	panickingHandler := recoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("unexpected boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()

	panickingHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
	var res ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed parsing response json: %v", err)
	}
	if res.Error != "Internal server error. Please try again later." {
		t.Fatalf("unexpected error response: %q", res.Error)
	}
}

func TestHealthLive(t *testing.T) {
	mux := setupTestMux(&mockTelemetryStore{})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Fatalf("expected 'OK', got %q", rec.Body.String())
	}
}

func TestHealthReady_Success(t *testing.T) {
	mux := setupTestMux(&mockTelemetryStore{})
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "OK" {
		t.Fatalf("expected 'OK', got %q", rec.Body.String())
	}
}

func TestHealthReady_ShuttingDown(t *testing.T) {
	var isReady atomic.Bool
	isReady.Store(false)
	mux := setupTestMuxWithReady(&mockTelemetryStore{}, &isReady)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHealthReady_DatabaseError(t *testing.T) {
	mock := &mockTelemetryStore{
		pingErr: errors.New("connection refused"),
	}
	mux := setupTestMux(mock)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/", "/"},
		{"/health/live", "/health/live"},
		{"/api/devices", "/api/devices"},
		{"/api/devices/357565514000001", "/api/devices/:id"},
		{"/api/devices/357565514000001/latest", "/api/devices/:id/latest"},
		{"/api/devices/357565514000001/positions", "/api/devices/:id/positions"},
	}

	for _, tc := range tests {
		got := normalizePath(tc.input)
		if got != tc.want {
			t.Errorf("normalizePath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMetricsMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := metricsMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/api/devices/357565514000001/latest", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
