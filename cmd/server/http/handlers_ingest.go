package http

import (
	"net/http"
	"time"

	"gps-telemetry-service/internal/telemetry"
)

// IngestPosition handles GET / requests containing OsmAnd query parameters.
//
//	@Summary		Ingest a GPS position
//	@Description	Accepts a GPS position as OsmAnd query parameters. A successful request is acknowledged with an empty 200 response after the position has been persisted. Repeated device ID and timestamp pairs are ignored.
//	@Tags			Telemetry
//	@Param			id			query	string	true	"Device identifier (up to 32 characters)"
//	@Param			timestamp	query	int64	true	"Device timestamp as Unix epoch seconds"
//	@Param			lat			query	number	true	"Latitude in decimal degrees (-90 to 90)"
//	@Param			lon			query	number	true	"Longitude in decimal degrees (-180 to 180)"
//	@Param			valid		query	integer	false	"Position validity (0 or 1; defaults to 1)"
//	@Param			speed		query	number	false	"Speed in knots (non-negative)"
//	@Param			bearing		query	number	false	"Bearing in degrees (0 to 360)"
//	@Param			altitude	query	number	false	"Altitude in metres"
//	@Param			accuracy	query	number	false	"Horizontal accuracy in metres (non-negative)"
//	@Param			hdop		query	number	false	"Horizontal dilution of precision (non-negative)"
//	@Param			batt		query	number	false	"Battery level (0 to 100)"
//	@Param			charge		query	boolean	false	"Whether the device is charging"
//	@Success		200			"Position persisted or duplicate ignored; empty response body"
//	@Failure		400			{object}	ErrorResponse	"Invalid or missing query parameter"
//	@Failure		500			{object}	ErrorResponse	"Unexpected persistence error"
//	@Router			/																																																				[get]
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
