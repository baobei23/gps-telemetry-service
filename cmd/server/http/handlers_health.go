package http

import (
	"context"
	"net/http"
	"time"
)

// Live handles GET /health/live.
//
//	@Summary		Liveness probe
//	@Description	Returns 200 OK if the service process is alive.
//	@Tags			Health
//	@Produce		plain
//	@Success		200	{string}	string	"OK"
//	@Router			/health/live [get]
func (h *Handlers) Live(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Ready handles GET /health/ready.
//
//	@Summary		Readiness probe
//	@Description	Returns 200 OK if the service is ready to accept traffic and dependent database is reachable. Returns 503 during shutdown or DB outage.
//	@Tags			Health
//	@Produce		plain
//	@Success		200	{string}	string	"OK"
//	@Failure		503	{string}	string	"Service Unavailable"
//	@Router			/health/ready [get]
func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
	if h.isReady != nil && !h.isReady.Load() {
		http.Error(w, "shutting down", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.telemetry.Ping(ctx); err != nil {
		http.Error(w, "db not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
