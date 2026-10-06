package http

import (
	"encoding/json"
	"net/http"
)

type BaseResponse struct {
	Data any `json:"data,omitempty"`
}

type PaginatedResponse struct {
	Data       any            `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

type PaginationMeta struct {
	Limit      int     `json:"limit"`
	NextCursor *string `json:"nextCursor"`
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

// writePaginatedJSON sends a JSON response with items and pagination metadata
func writePaginatedJSON(w http.ResponseWriter, status int, data any, limit int, nextCursor *string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(PaginatedResponse{
		Data: data,
		Pagination: PaginationMeta{
			Limit:      limit,
			NextCursor: nextCursor,
		},
	})
}

// writeError sends a sanitized error response and records server-side errors
func writeError(w http.ResponseWriter, r *http.Request, status int, err error) {
	if status >= http.StatusInternalServerError && err != nil {
		recordError(r.Context(), err)
	}

	clientMsg := "Bad request"
	if status >= http.StatusInternalServerError {
		clientMsg = "Internal server error. Please try again later."
	} else if err != nil {
		clientMsg = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error: clientMsg,
	})
}
