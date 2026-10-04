package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	"log/slog"
	"sync/atomic"

	"github.com/baobei23/goapp/cmd/server/grpc"
	xhttp "github.com/baobei23/goapp/cmd/server/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

func shutdown(
	shutdownGraceperiod time.Duration,
	probeInterval time.Duration,
	isReady *atomic.Bool,
	healthResp *http.Server,
	httpServer *xhttp.HTTP,
	grpcServer *grpc.GRPC,
	pqDriver *pgxpool.Pool,
) {
	// set the service as Not ready as soon as it's exiting main
	isReady.Store(false)

	// the time should be decided based on the grace period allowed for shutdown.
	// e.g. for Kubernetes terminationGracePeriodSeconds, https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/
	ctx, cancel := context.WithTimeout(context.Background(), shutdownGraceperiod)
	defer cancel()

	/*
		Note: It is important to keep healthcheck endpoint available as long as possible to provide
		probes as much context as possible. Esepcially during the graceful shutdown period.
		Hence it is recommended to setup an independent server for health checks alone.
	*/
	defer func() {
		if healthResp != nil {
			_ = healthResp.Shutdown(ctx)
		}
	}()

	/*
		When a server begins its shutdown process, it first signals Kubernetes (or any other prober)
		by changing its readiness state to "not ready". This ensures that the server stops receiving new traffic.

		However, there is typically a delay before Kubernetes detects this readiness change because
		it relies on periodic probing to check the status. During this delay, Kubernetes may still
		route new requests to the server, unaware that shutdown has initiated.

		To handle this, a deliberate pause is introduced between changing the readiness state to
		"not ready" and initiating the full shutdown. This pause should be longer than Kubernetes'
		readiness probe interval. This way, Kubernetes has enough time to notice the readiness change
		and stop sending new requests before the server begins rejecting them.
	*/
	// in this case, the Kuberenetes probe interval is assumed to be 2 seconds
	time.Sleep(probeInterval)
	slog.InfoContext(ctx, "initiating shutdown")
	shutdownDependenciesAndServices(ctx, httpServer, grpcServer, pqDriver)
}

func shutdownDependenciesAndServices(
	ctx context.Context,
	httpServer *xhttp.HTTP,
	grpcServer *grpc.GRPC,
	pqDriver *pgxpool.Pool,
) {
	wgroup := &sync.WaitGroup{}
	if httpServer != nil {
		wgroup.Add(1)
		go func() {
			defer wgroup.Done()
			_ = httpServer.Shutdown(ctx)
		}()
	}

	if grpcServer != nil {
		wgroup.Add(1)
		go func() {
			defer wgroup.Done()
			_ = grpcServer.Shutdown(ctx)
		}()
	}

	// after all the APIs of the application are shutdown (e.g. HTTP, gRPC, Pubsub listener etc.)
	// we should close connections to dependencies like database, cache etc.
	wgroup.Wait()
	if pqDriver != nil {
		pqDriver.Close()
	}
}
