package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/baobei23/goapp/internal/pkg/jwt"
	"github.com/baobei23/goapp/internal/usernotes"
	"github.com/baobei23/goapp/internal/users"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Config holds all the configuration required to start the HTTP server
type Config struct {
	Host string
	Port uint16

	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	EnableAccessLog bool
	EnableTracing   bool
}

type HTTP struct {
	server *http.Server
}

// Start starts the HTTP server
func (h *HTTP) Start() error {
	return h.server.ListenAndServe()
}

func (h *HTTP) Shutdown(ctx context.Context) error {
	return h.server.Shutdown(ctx)
}

// NewService returns an instance of HTTP with all its dependencies set
func NewService(cfg *Config, userSvc *users.Users, noteSvc *usernotes.UserNotes, tm *jwt.TokenManager) (*HTTP, error) {
	handlers := &Handlers{
		users: userSvc,
		notes: noteSvc,
		tm:    tm,
	}

	mux := http.NewServeMux()
	handlers.registerRoutes(mux)

	var handler http.Handler = mux

	if cfg.EnableTracing {
		handler = otelhttp.NewHandler(handler, "goapp")
	}

	if cfg.EnableAccessLog {
		handler = loggingMiddleware(handler)
	}

	handler = recoveryMiddleware(handler)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	return &HTTP{
		server: srv,
	}, nil
}
