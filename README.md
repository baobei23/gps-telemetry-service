# GPS Simulator

Simulates a GPS tracker moving around Jakarta and sends location updates to an OsmAnd-compatible HTTP endpoint.

## Run

```sh
go run .
```

Default endpoint: `http://localhost:5055`. Run `go run . -selftest` for offline logic checks.

## Docker

```sh
docker compose up --build
```

The simulator defaults to `localhost:5055`; from a container, set the endpoint to a hostname reachable from the container (for example, `host.docker.internal:5055`).

## Build binaries

```sh
./build.sh
```

Builds Windows amd64 and Linux amd64 binaries, plus a macOS binary for the host architecture.