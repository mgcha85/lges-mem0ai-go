#!/bin/bash
# run_prod.sh - Production run on host (Offline Standalone - Pre-built)
set -e

# 1. Environment Check
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example .env
        echo "Created .env from .env.example."
    else
        echo "Error: .env file not found."
        exit 1
    fi
fi

# Ensure SQLite is used as the default for offline standalone (no external services)
sed -i 's/VECTORDB_PROVIDER=qdrant/VECTORDB_PROVIDER=sqlite/' .env 2>/dev/null || true

# 2. Extract configuration for runtime (resilient to read-only)
DATA_DIR=$(grep "^DATA_DIR=" .env | cut -d'=' -f2)
DATA_DIR=${DATA_DIR:-"./data"}
mkdir -p "$DATA_DIR" 2>/dev/null || true

# 2. Pre-built Verification
if [ ! -f "./server" ]; then
    echo "Pre-built binary not found. Attempting to build from vendor..."
    export LD_LIBRARY_PATH=$(pwd)/lib
    go build -mod=vendor -o server ./cmd/server/
fi
chmod +x ./server 2>/dev/null || true

# 3. Handle models if split/missing (Robustness)
if [ ! -d "models" ] && [ -f "models.tar.gz.part_aa" ]; then
    echo "Extracting model parts..."
    cat models.tar.gz.part_* > models.tar.gz
    tar -xvzf models.tar.gz
    rm models.tar.gz
fi

# 4. Run Server in background
echo "Starting lges-mem0ai-go server (Pre-built binary for Ubuntu 22.04)..."
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
export LD_LIBRARY_PATH="$SCRIPT_DIR/lib:$LD_LIBRARY_PATH"
chmod +x ./server 2>/dev/null || true

# Direct output to stdout (logs managed by system)
# PID stored in DATA_DIR (NAS)
nohup ./server & 
SERVER_PID=$!
echo $SERVER_PID > "$DATA_DIR/server.pid" 2>/dev/null || echo $SERVER_PID > /tmp/server.pid

echo "======================================"
echo "Server started successfully (Offline Mode)!"
echo "PID: $SERVER_PID (Stored in DATA_DIR/server.pid or /tmp/server.pid)"
echo "Logs: Outputting to stdout"
echo "======================================"
