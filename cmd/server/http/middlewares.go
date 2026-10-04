package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type contextKey struct{}

var (
	userIDKey    = &contextKey{}
	userEmailKey = &contextKey{}
)

// AuthMiddleware validates the JWT token in Authorization header
func (h *Handlers) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, r, http.StatusUnauthorized, errors.New("authorization header is missing"))
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeError(w, r, http.StatusUnauthorized, errors.New("invalid authorization header format"))
			return
		}

		tokenStr := parts[1]
		claims, err := h.tm.Validate(tokenStr)
		if err != nil {
			writeError(w, r, http.StatusUnauthorized, errors.New("invalid token"))
			return
		}

		if claims.TokenType != "access" {
			writeError(w, r, http.StatusUnauthorized, errors.New("invalid token type"))
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, userEmailKey, claims.Email)
		next(w, r.WithContext(ctx))
	}
}

// GetUserID retrieves the userID from the context
func GetUserID(r *http.Request) string {
	if val, ok := r.Context().Value(userIDKey).(string); ok {
		return val
	}
	return ""
}

// GetUserEmail retrieves the userEmail from the context
func GetUserEmail(r *http.Request) string {
	if val, ok := r.Context().Value(userEmailKey).(string); ok {
		return val
	}
	return ""
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.ErrorContext(r.Context(), "panic recovered", "error", rec)
				writeError(w, r, http.StatusInternalServerError, errors.New("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *responseRecorder) WriteHeader(statusCode int) {
	rec.statusCode = statusCode
	rec.ResponseWriter.WriteHeader(statusCode)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.InfoContext(r.Context(), "http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.statusCode,
			"duration", time.Since(start).String(),
		)
	})
}
