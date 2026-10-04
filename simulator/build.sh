#!/usr/bin/env sh
# Cross-compile the simulator. Binaries are build artifacts: do not commit them.
# Output names carry the OS and CPU architecture they were built for.
set -eu
cd "$(dirname "$0")"

build() { # $1=GOOS $2=GOARCH $3=output
	CGO_ENABLED=0 GOOS="$1" GOARCH="$2" go build -trimpath -ldflags "-s -w" -o "$3" .
	echo "  $3  ($1/$2)"
}

echo "building simulator:"
# First build fetches golang.org/x/term (raw keyboard input). Run
# "go mod vendor" once if builds must work offline.
build windows amd64 simulator-windows-amd64.exe
build linux amd64 simulator-linux-amd64

# macOS binaries are only useful when built on macOS, so build for the host CPU.
if [ "$(go env GOOS)" = darwin ]; then
	build darwin "$(go env GOARCH)" "simulator-darwin-$(go env GOARCH)"
fi
