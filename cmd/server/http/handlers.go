package http

import (
	"net/http"

	"github.com/baobei23/goapp/internal/pkg/jwt"
	"github.com/baobei23/goapp/internal/usernotes"
	"github.com/baobei23/goapp/internal/users"
)

// Handlers struct has all the dependencies required for HTTP handlers
type Handlers struct {
	users *users.Users
	notes *usernotes.UserNotes
	tm    *jwt.TokenManager
}

func (h *Handlers) registerRoutes(mux *http.ServeMux) {
	// root
	mux.HandleFunc("GET /{$}", h.HelloWorld)

	// auth
	mux.HandleFunc("POST /register", h.Register)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("POST /auth/refresh", h.RefreshToken)
	mux.HandleFunc("POST /auth/logout", h.Logout)

	// users
	mux.HandleFunc("GET /users", h.AuthMiddleware(h.ReadUserByID))
	mux.HandleFunc("PUT /users/password", h.AuthMiddleware(h.ChangePassword))

	// usernotes
	mux.HandleFunc("POST /usernotes", h.AuthMiddleware(h.RegisterNote))
	mux.HandleFunc("GET /usernotes/{noteID}", h.AuthMiddleware(h.ReadUserNote))
}

func (h *Handlers) HelloWorld(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, "hello world")
}
