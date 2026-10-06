package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"sync/atomic"

	"gps-telemetry-service/internal/configs"
)

//	@title			GPS Telemetry Ingestion API
//	@version		1.0.0
//	@description	HTTP API for ingesting GPS positions using the Traccar OsmAnd query parameter format.
// //	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	https://github.com/baobei23
//	@contact.email	reginaldsaja98@gmail.com
//
//	@BasePath		/

var exitErr error

// recoverer is used for panic recovery of the application (note: this is not for the HTTP/gRPC servers).
// So that even if the main function panics we can produce required logs for troubleshooting

func recoverer() {
	exitCode := 0
	var exitInfo any
	rec := recover()
	if rec != nil {
		exitCode = 2
		exitInfo = rec
	} else if exitErr != nil {
		exitCode = 1
		exitInfo = exitErr
	}

	ctx := context.Background()
	if exitCode == 0 {
		slog.InfoContext(ctx, "shutdown complete")
	} else {
		slog.ErrorContext(ctx, "shutdown complete with failure", "exitCode", exitCode, "error", exitInfo)
	}

	os.Exit(exitCode)
}

func main() {
	defer recoverer()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		fatalErr            = make(chan error, 1)
		shutdownGraceperiod = time.Minute
		probeInterval       = time.Second * 3
		isReady             atomic.Bool
	)
	isReady.Store(true)

	cfgs, err := configs.New()
	if err != nil {
		panic(fmt.Errorf("failed to load configurations: %w", err))
	}

	if err := cfgs.Validate(); err != nil {
		panic(err)
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(jsonHandler).With(
		"app", cfgs.AppName,
		"appVersion", cfgs.AppVersion,
		"env", cfgs.Environment.String(),
	))

	// This needs to remain after log initialisation and before server initialisation.
	tp := setupTelemetry(cfgs)
	if tp != nil {
		defer tp.Shutdown(ctx)
	}

	hserver, gserver, healthServer, db := start(ctx, &isReady, cfgs, fatalErr)

	defer shutdown(
		shutdownGraceperiod,
		probeInterval,
		&isReady,
		healthServer,
		hserver,
		gserver,
		db,
	)
	select {
	case exitErr = <-fatalErr:
	case <-ctx.Done():
		exitErr = nil
	}
}
