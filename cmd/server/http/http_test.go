package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestMux() *http.ServeMux {
	handlers := &Handlers{}
	mux := http.NewServeMux()
	handlers.registerRoutes(mux)
	return mux
}

func TestHelloWorld(t *testing.T) {
	mux := setupTestMux()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res BaseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}
	if res.Data != "hello world" {
		t.Fatalf("expected 'hello world', got %v", res.Data)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	logged := loggingMiddleware(inner)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
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
