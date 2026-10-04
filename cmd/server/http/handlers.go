package http

import (
	"net/http"
)

// Handlers struct has all the dependencies required for HTTP handlers
type Handlers struct{}

func (h *Handlers) registerRoutes(mux *http.ServeMux) {
	// root
	mux.HandleFunc("GET /{$}", h.HelloWorld)
}

func (h *Handlers) HelloWorld(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, "hello world")
}
