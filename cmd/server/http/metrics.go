package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gps_http_requests_total",
			Help: "Total number of HTTP requests processed by method, path, and status code.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gps_http_request_duration_seconds",
			Help:    "Histogram of HTTP request latency in seconds.",
			Buckets: []float64{0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		},
		[]string{"method", "path"},
	)
)

// normalizePath converts dynamic path segments like IDs into templates
// to avoid high cardinality metrics explosion.
func normalizePath(path string) string {
	if path == "/" || path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/api/devices/") {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		// /api/devices/{id}/latest or /api/devices/{id}/positions
		if len(parts) == 4 {
			return "/api/devices/:id/" + parts[3]
		}
		if len(parts) == 3 {
			return "/api/devices/:id"
		}
	}
	return path
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		duration := time.Since(start).Seconds()
		normPath := normalizePath(r.URL.Path)
		statusStr := strconv.Itoa(rec.statusCode)

		httpRequestsTotal.WithLabelValues(r.Method, normPath, statusStr).Inc()
		httpRequestDuration.WithLabelValues(r.Method, normPath).Observe(duration)
	})
}
