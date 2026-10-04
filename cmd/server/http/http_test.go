package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gps-telemetry-service/internal/telemetry"
)

type mockTelemetryStore struct {
	ingested []*telemetry.Position
}

func (m *mockTelemetryStore) IngestPosition(ctx context.Context, pos *telemetry.Position) error {
	m.ingested = append(m.ingested, pos)
	return nil
}

func setupTestMux(mock *mockTelemetryStore) *http.ServeMux {
	svc := telemetry.NewService(mock)
	handlers := &Handlers{
		telemetry: svc,
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
