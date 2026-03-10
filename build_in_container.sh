#!/bin/bash
# build_in_container.sh - Build Go binary inside Ubuntu 22.04 container for GLIBC parity
set -e

echo "Starting Ubuntu 22.04 container build..."

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
        export LD_LIBRARY_PATH=/workspace/lib && \
        go build -mod=vendor -v -o server ./cmd/server/ && \
        echo 'Build successful. Checking GLIBC version of binary...' && \
        ldd ./server | grep libc.so
    "

echo "Build process finished. Binary 'server' is now compatible with Ubuntu 22.04."
