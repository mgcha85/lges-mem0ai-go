#!/bin/bash
# build_in_container.sh - Build Go binary and vendor dependencies inside Ubuntu 22.04 container for GLIBC parity
set -e

echo "Starting Ubuntu 22.04 container build & vendoring..."

# Use Ubuntu 22.04 image to ensure GLIBC 2.35 compatibility
podman run --rm \
    -v $(pwd):/workspace:Z \
    -w /workspace \
    docker.io/library/ubuntu:22.04 \
    /bin/bash -c "
        apt-get update && \
        apt-get install -y wget gcc git && \
        wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz && \
        tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz && \
        export PATH=\$PATH:/usr/local/go/bin && \
        go version && \
        echo 'Running go mod tidy and vendor inside container...' && \
        go env -w GOCACHE=/workspace/.cache/go-build && \
        go env -w GOMODCACHE=/workspace/.cache/go-mod && \
        go mod tidy && \
        go mod vendor && \
        export LD_LIBRARY_PATH=/workspace/lib && \
        echo 'Building server binary...' && \
        go build -mod=vendor -v -o server ./cmd/server/ && \
        echo 'Build successful. Checking GLIBC version of binary...' && \
        ldd ./server | grep libc.so && \
        rm -rf /workspace/.cache
    "

echo "Build process and vendoring finished. Binary 'server' and 'vendor/' are now perfectly compatible with Ubuntu 22.04."
