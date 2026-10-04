package http

import (
	"net/http"
	"time"

	"gps-telemetry-service/internal/telemetry"
)

// IngestPosition handles GET / requests containing OsmAnd query parameters.
func (h *Handlers) IngestPosition(w http.ResponseWriter, r *http.Request) {
	serverTime := time.Now()

	pos, err := telemetry.ParseOsmAnd(r.URL.Query(), serverTime)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}

	if err := h.telemetry.Ingest(r.Context(), pos); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
