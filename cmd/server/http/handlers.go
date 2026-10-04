package http

import (
	"net/http"

	"gps-telemetry-service/internal/telemetry"
)

// Handlers struct has all the dependencies required for HTTP handlers
type Handlers struct {
	telemetry *telemetry.Service
}

func (h *Handlers) registerRoutes(mux *http.ServeMux) {
	// OsmAnd telemetry ingestion
	mux.HandleFunc("GET /{$}", h.IngestPosition)
}
