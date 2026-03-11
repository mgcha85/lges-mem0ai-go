#!/bin/bash
set -e
echo "Starting build in ubuntu:22.04 container..."
USER_ID=$(id -u)
GROUP_ID=$(id -g)

podman run --rm -v "$(pwd):/app:Z" -w /app ubuntu:22.04 /bin/bash -c "
    apt-get update && apt-get install -y wget gcc git libc6-dev
    wget -qO- https://go.dev/dl/go1.24.0.linux-amd64.tar.gz | tar -C /usr/local -xz
    export PATH=/usr/local/go/bin:\$PATH
    export GOCACHE=/app/.cache/go-build
    export GOMODCACHE=/app/.cache/go-mod
    export CGO_ENABLED=1
    
    go mod tidy
    go mod vendor
    
    export LD_LIBRARY_PATH=/app/lib
    go build -mod=vendor -v -o server ./cmd/server/
    
    chown -R $USER_ID:$GROUP_ID vendor server .cache go.mod go.sum
    rm -rf .cache
"
echo "Build complete. Verifying binary:"
file server
ldd server | grep libc.so
