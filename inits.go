package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/baobei23/goapp/cmd/server/grpc"
	xhttp "github.com/baobei23/goapp/cmd/server/http"
	"github.com/baobei23/goapp/internal/configs"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/baobei23/goapp/internal/pkg/jwt"
	"github.com/baobei23/goapp/internal/pkg/postgres"
	"github.com/baobei23/goapp/internal/usernotes"
	"github.com/baobei23/goapp/internal/users"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func setupTelemetry(cfgs *configs.Configs) *sdktrace.TracerProvider {
	if !cfgs.EnableTracing && !cfgs.EnableMetrics {
		return nil
	}

	if cfgs.EnableMetrics {
		go func() {
			port := 9090
			mux := http.NewServeMux()
			mux.Handle("/-/metrics", promhttp.Handler())
			slog.Info(fmt.Sprintf("[otel/http] starting prometheus metrics on :%d/-/metrics", port))
			_ = http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
		}()
	}

	var tp *sdktrace.TracerProvider
	if cfgs.EnableTracing {
		res, _ := resource.Merge(
			resource.Default(),
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceName(cfgs.AppName),
				semconv.ServiceVersion(cfgs.AppVersion),
				semconv.DeploymentEnvironment(cfgs.Environment.String()),
			),
		)

		exporter, _ := otlptracegrpc.New(context.Background())

		tp = sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.5)),
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(exporter),
		)
		otel.SetTracerProvider(tp)
	}

	return tp
}

func startServers(userSvc *users.Users, noteSvc *usernotes.UserNotes, cfgs *configs.Configs, tm *jwt.TokenManager, fatalErr chan<- error) (*xhttp.HTTP, *grpc.GRPC) {
	hcfg, _ := cfgs.HTTP()
	hserver, err := xhttp.NewService(hcfg, userSvc, noteSvc, tm)
	if err != nil {
		fatalErr <- fmt.Errorf("failed to initialize HTTP server: %w", err)
	}

	go func() {
		defer func() {
			rec := recover()
			if rec != nil {
				fatalErr <- fmt.Errorf("%+v", rec)
			}
		}()
		err = hserver.Start()
		if err != nil {
			fatalErr <- fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}()

	return hserver, nil
}

func startHealthServer(ctx context.Context, db *pgxpool.Pool, isReady *atomic.Bool, fatalErr chan<- error) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/-/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		if !isReady.Load() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	srv := &http.Server{Addr: ":2000", Handler: mux}
	go func() {
		slog.InfoContext(ctx, "[http/health] listening on :2000")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatalErr <- err
		}
	}()
	return srv
}

func start(
	ctx context.Context,
	isReady *atomic.Bool,
	cfgs *configs.Configs,
	fatalErr chan<- error,
) (hserver *xhttp.HTTP, gserver *grpc.GRPC, healthServer *http.Server, pqdriver *pgxpool.Pool) {
	pqdriver, err := postgres.NewPool(cfgs.Postgres())
	if err != nil {
		panic(fmt.Errorf("failed to connect to postgres: %w", err))
	}

	healthServer = startHealthServer(ctx, pqdriver, isReady, fatalErr)

	userPGstore := users.NewPostgresStore(pqdriver)
	userSvc := users.NewService(userPGstore)

	notePGstore := usernotes.NewPostgresStore(pqdriver)
	noteSvc := usernotes.NewService(notePGstore)

	tm := cfgs.JWT()
	hserver, gserver = startServers(userSvc, noteSvc, cfgs, tm, fatalErr)
	return
}
