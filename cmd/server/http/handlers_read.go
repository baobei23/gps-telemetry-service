package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gps-telemetry-service/internal/telemetry"
)

// ListDevices handles GET /api/devices.
//
//	@Summary		List tracked devices
//	@Description	Retrieves all tracked GPS devices with first/last seen timestamps and activity status.
//	@Tags			Devices
//	@Produce		json
//	@Success		200	{object}	BaseResponse{data=[]telemetry.DeviceSummary}
//	@Failure		500	{object}	ErrorResponse	"Internal server error"
//	@Router			/api/devices [get]
func (h *Handlers) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.telemetry.ListDevices(r.Context(), time.Minute)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, devices)
}

// GetLatestPosition handles GET /api/devices/{id}/latest.
//
//	@Summary		Get latest device position
//	@Description	Retrieves the most recent GPS fix reported by the specified device ID.
//	@Tags			Devices
//	@Produce		json
//	@Param			id	path		string	true	"Device identifier"
//	@Success		200	{object}	BaseResponse{data=telemetry.Position}
//	@Failure		400	{object}	ErrorResponse	"Device ID is required"
//	@Failure		404	{object}	ErrorResponse	"Device not found"
//	@Failure		500	{object}	ErrorResponse	"Internal server error"
//	@Router			/api/devices/{id}/latest [get]
func (h *Handlers) GetLatestPosition(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("device id is required"))
		return
	}

	pos, err := h.telemetry.GetLatestPosition(r.Context(), id)
	if err != nil {
		if errors.Is(err, telemetry.ErrDeviceNotFound) || errors.Is(err, telemetry.ErrPositionNotFound) {
			writeError(w, r, http.StatusNotFound, err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, pos)
}

// ListPositions handles GET /api/devices/{id}/positions.
//
//	@Summary		List position history
//	@Description	Retrieves paginated GPS position history for a device within an optional time range using keyset cursor pagination.
//	@Tags			Devices
//	@Produce		json
//	@Param			id		path		string	true	"Device identifier"
//	@Param			from	query		string	false	"Start timestamp in RFC3339 format"
//	@Param			to		query		string	false	"End timestamp in RFC3339 format"
//	@Param			limit	query		integer	false	"Page size limit (1-100, default 50)"
//	@Param			cursor	query		string	false	"Opaque cursor token for next page"
//	@Success		200		{object}	PaginatedResponse{data=[]telemetry.Position}
//	@Failure		400		{object}	ErrorResponse	"Invalid query parameter format or range"
//	@Failure		404		{object}	ErrorResponse	"Device not found"
//	@Failure		500		{object}	ErrorResponse	"Internal server error"
//	@Router			/api/devices/{id}/positions [get]
func (h *Handlers) ListPositions(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, r, http.StatusBadRequest, errors.New("device id is required"))
		return
	}

	filter := telemetry.PositionsFilter{
		DeviceID: id,
		Limit:    50,
	}

	q := r.URL.Query()

	if fromStr := strings.TrimSpace(q.Get("from")); fromStr != "" {
		fromTime, err := parseRFC3339(fromStr)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, errors.New("invalid parameter from: must be RFC3339 format"))
			return
		}
		filter.From = &fromTime
	}

	if toStr := strings.TrimSpace(q.Get("to")); toStr != "" {
		toTime, err := parseRFC3339(toStr)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, errors.New("invalid parameter to: must be RFC3339 format"))
			return
		}
		filter.To = &toTime
	}

	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		writeError(w, r, http.StatusBadRequest, errors.New("parameter 'from' cannot be after 'to'"))
		return
	}

	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l <= 0 {
			writeError(w, r, http.StatusBadRequest, errors.New("invalid parameter limit: must be a positive integer"))
			return
		}
		if l > 100 {
			l = 100
		}
		filter.Limit = l
	}

	if cursorStr := strings.TrimSpace(q.Get("cursor")); cursorStr != "" {
		cTime, err := telemetry.DecodeCursor(cursorStr)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, err)
			return
		}
		filter.CursorTime = &cTime
	}

	res, err := h.telemetry.ListPositions(r.Context(), filter)
	if err != nil {
		if errors.Is(err, telemetry.ErrDeviceNotFound) {
			writeError(w, r, http.StatusNotFound, err)
			return
		}
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	writePaginatedJSON(w, http.StatusOK, res.Positions, filter.Limit, res.NextCursor)
}

func parseRFC3339(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}
