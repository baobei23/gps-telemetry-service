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

	"github.com/baobei23/goapp/internal/configs"
)

//	@title			GoApp API
//	@description	API for GoApp
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	https://github.com/baobei23
//	@contact.email	reginaldsaja98@gmail.com

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@BasePath					/
//
//	@securityDefinitions.apikey	ApiKeyAuth
//	@in							header
//	@name						Authorization
//	@description				Bearer token

var exitErr error

// recoverer is used for panic recovery of the application (note: this is not for the HTTP/gRPC servers).
// So that even if the main function panics we can produce required logs for troubleshooting

func recoverer() {
	exitCode := 0
	var exitInfo any
	rec := recover()
	err, _ := rec.(error)
	if err != nil {
		exitCode = 1
		exitInfo = err
	} else if rec != nil {
		exitCode = 2
		exitInfo = rec
	} else if exitErr != nil {
		exitCode = 3
		exitInfo = exitErr
	}

	// exiting without error is a clean exit
	if exitErr == nil {
		exitCode = 0
	}

	ctx := context.Background()
	// logging this because we have info logs saying "listening to" various port numbers
	// based on the server type (gRPC, HTTP etc.). But it's unclear *from the logs*
	// if the server is up and running, if it exits for any reason
	if exitCode == 0 {
		slog.InfoContext(ctx, fmt.Sprintf("shutdown complete: %+v", exitInfo))
	} else {
		slog.ErrorContext(ctx, fmt.Sprintf("shutdown complete (exit: %d): %+v", exitCode, exitInfo))
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
