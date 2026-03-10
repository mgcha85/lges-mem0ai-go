#!/bin/bash
# run_prod.sh - Production run on host (Direct Execution)
set -e

# 1. Environment Check
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example .env
        echo "Created .env from .env.example. Please configure it."
    else
        echo "Error: .env file not found."
        exit 1
    fi
fi

# 2. Start Qdrant Container (Sidecar)
echo "Starting Qdrant service..."
if ! podman ps | grep -q qdrant_server; then
    podman run -d \
        --name qdrant_server \
        -p 6333:6333 -p 6334:6334 \
        -v ./qdrant_data:/qdrant/storage:Z \
        docker.io/qdrant/qdrant:latest
fi

# 3. Build Go Server
echo "Building server..."
go build -o server ./cmd/server/

# 4. Run Server in background
echo "Starting lges-mem0ai-go server..."
export LD_LIBRARY_PATH=$(pwd)/lib
nohup ./server > server.log 2>&1 &
echo $! > server.pid

echo "Server started with PID $(cat server.pid). Logs: server.log"
