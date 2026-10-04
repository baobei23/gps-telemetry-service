package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type BaseResponse struct {
	Data any `json:"data,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"Something went wrong"`
}

// writeJSON sends a JSON response with the given data
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(BaseResponse{
		Data: data,
	})
}

// writeError sends a sanitized error response and logs internal server errors
func writeError(w http.ResponseWriter, r *http.Request, status int, err error) {
	var clientMsg string

	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "internal server error",
			"method", r.Method,
			"path", r.URL.Path,
			"error", err,
		)
		clientMsg = "Internal server error. Please try again later."
	} else if err != nil {
		clientMsg = err.Error()
	} else {
		clientMsg = "Bad request"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: clientMsg,
	})
}

// decodeJSON decodes the request body with a size ceiling
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	return json.NewDecoder(r.Body).Decode(dst)
}
