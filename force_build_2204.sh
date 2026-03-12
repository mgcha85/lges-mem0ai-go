#!/bin/bash
# force_build_2204.sh - Final build script for Ubuntu 22.04 parity
set -e

echo "--- Cleaning up previous artifacts ---"
rm -rf vendor/ server .cache/

echo "--- Starting Ubuntu 22.04 container for complete build and sync ---"
USER_ID=$(id -u)
GROUP_ID=$(id -g)

podman run --rm \
    -v "$(pwd):/app:Z" \
    -w /app \
    docker.io/library/ubuntu:22.04 \
    /bin/bash -c "
        export DEBIAN_FRONTEND=noninteractive
        apt-get update && apt-get install -y wget gcc git libc6-dev ca-certificates
        
        echo '--- Installing Go 1.24 ---'
        wget -qO- https://go.dev/dl/go1.24.0.linux-amd64.tar.gz | tar -C /usr/local -xz
        export PATH=/usr/local/go/bin:\$PATH
        
        echo '--- Syncing dependencies and creating vendor/ ---'
        export GOCACHE=/app/.cache/go-build
        export GOMODCACHE=/app/.cache/go-mod
        export CGO_ENABLED=1
        
        go mod tidy
        go mod vendor
        
        echo '--- Building server binary with -mod=vendor ---'
        export LD_LIBRARY_PATH=/app/lib
        go build -mod=vendor -v -o server ./cmd/server/
        
        echo '--- Verifying binary linkages (GLIBC parity) ---'
        ldd ./server | grep libc.so
        
        echo '--- Adjusting file ownership ---'
        chown -R $USER_ID:$GROUP_ID vendor server go.mod go.sum
        
        echo '--- Build in container finished successfully ---'
    "

rm -rf .cache/
echo "--- Host: Verifying results ---"
if [ -f "server" ]; then
    file server
    ls -ld vendor/
    echo "SUCCESS: Binary and vendor folder are ready for Ubuntu 22.04."
else
    echo "ERROR: Build failed."
    exit 1
fi
