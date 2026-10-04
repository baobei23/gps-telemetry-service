package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/baobei23/goapp/internal/pkg/jwt"
	"github.com/baobei23/goapp/internal/usernotes"
	"github.com/baobei23/goapp/internal/users"
)

type mockUserStore struct{}

func (m *mockUserStore) GetUserByEmail(ctx context.Context, email string) (*users.User, error) {
	if email == "wrong@example.com" {
		return nil, users.ErrUserNotFound
	}
	u := &users.User{ID: "u-123", Email: email, Password: []byte("password123")}
	_ = u.HashPassword()
	return u, nil
}

func (m *mockUserStore) GetUserByID(ctx context.Context, id string) (*users.User, error) {
	u := &users.User{ID: id, FullName: "Test User", Email: "test@example.com", Password: []byte("password123")}
	_ = u.HashPassword()
	return u, nil
}

func (m *mockUserStore) SaveUser(ctx context.Context, user *users.User) (string, error) {
	if user.Email == "duplicate@example.com" {
		return "", users.ErrUserEmailAlreadyExists
	}
	return "u-123", nil
}

func (m *mockUserStore) BulkSaveUser(ctx context.Context, users []users.User) error {
	return nil
}

func (m *mockUserStore) UpdatePassword(ctx context.Context, id string, newPassword []byte) error {
	return nil
}

func (m *mockUserStore) SaveRefreshToken(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	return nil
}

func (m *mockUserStore) CheckRefreshToken(ctx context.Context, jti string) (bool, error) {
	return true, nil
}

func (m *mockUserStore) RevokeRefreshToken(ctx context.Context, jti string) error {
	return nil
}

type mockNoteStore struct{}

func (m *mockNoteStore) GetNoteByID(ctx context.Context, userID string, noteID string) (*usernotes.Note, error) {
	return &usernotes.Note{ID: noteID, UserID: userID, Title: "Note 1", Content: "Content 1"}, nil
}

func (m *mockNoteStore) SaveNote(ctx context.Context, note *usernotes.Note) (string, error) {
	return "n-123", nil
}

func setupTestMux() (*http.ServeMux, *jwt.TokenManager) {
	tm := &jwt.TokenManager{
		SecretKey:     "01234567890123456789012345678901",
		AccessExpiry:  15 * time.Minute,
		RefreshExpiry: 24 * time.Hour,
	}
	userSvc := users.NewService(&mockUserStore{})
	noteSvc := usernotes.NewService(&mockNoteStore{})
	handlers := &Handlers{
		users: userSvc,
		notes: noteSvc,
		tm:    tm,
	}
	mux := http.NewServeMux()
	handlers.registerRoutes(mux)
	return mux, tm
}

func TestHelloWorld(t *testing.T) {
	mux, _ := setupTestMux()

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

func TestAuthMiddleware(t *testing.T) {
	mux, tm := setupTestMux()

	// 1. Unauthorized when missing header
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth header, got %d", rec.Code)
	}

	// 2. Authorized with valid token
	accessToken, _, _, err := tm.GeneratePair("u-123", "test@example.com")
	if err != nil {
		t.Fatalf("failed generating token: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRegisterValidation(t *testing.T) {
	mux, _ := setupTestMux()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid email",
			body:       `{"fullName":"John Doe","email":"not-an-email","password":"password123"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "short password",
			body:       `{"fullName":"John Doe","email":"john@example.com","password":"123"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "duplicate email returns 409",
			body:       `{"fullName":"John Doe","email":"duplicate@example.com","password":"password123"}`,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "valid register",
			body:       `{"fullName":"John Doe","email":"john@example.com","password":"password123"}`,
			wantStatus: http.StatusCreated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d body=%s", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestUserNotesPathValue(t *testing.T) {
	mux, tm := setupTestMux()
	accessToken, _, _, _ := tm.GeneratePair("u-123", "test@example.com")

	req := httptest.NewRequest(http.MethodGet, "/usernotes/note-999", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var res BaseResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	data, ok := res.Data.(map[string]any)
	if !ok || data["ID"] != "note-999" {
		t.Fatalf("expected note-999, got %v", res.Data)
	}
}
