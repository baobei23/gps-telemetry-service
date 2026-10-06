package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const requestIDKey contextKey = "requestID"

type errorCarrier struct {
	err any
}

type errorCarrierKey struct{}

func recordError(ctx context.Context, err any) bool {
	if c, ok := ctx.Value(errorCarrierKey{}).(*errorCarrier); ok && c != nil {
		c.err = err
		return true
	}
	return false
}

// GetRequestID extracts the request ID from the request context.
func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if reqID == "" {
			reqID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				recordError(r.Context(), rec)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(ErrorResponse{
					Error: "Internal server error. Please try again later.",
				})
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
		carrier := &errorCarrier{}
		ctx := context.WithValue(r.Context(), errorCarrierKey{}, carrier)

		next.ServeHTTP(rec, r.WithContext(ctx))

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.statusCode,
			"duration", time.Since(start).String(),
		}
		if reqID := GetRequestID(r.Context()); reqID != "" {
			attrs = append(attrs, "requestId", reqID)
		}
		deviceID := r.URL.Query().Get("id")
		if deviceID == "" {
			deviceID = r.PathValue("id")
		}
		if deviceID != "" {
			attrs = append(attrs, "deviceId", deviceID)
		}

		if rec.statusCode >= http.StatusInternalServerError {
			if carrier.err != nil {
				attrs = append(attrs, "error", carrier.err)
			}
			slog.ErrorContext(r.Context(), "http request failed", attrs...)
		} else {
			slog.InfoContext(r.Context(), "http request", attrs...)
		}
	})
}
