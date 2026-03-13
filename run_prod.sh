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
sed -i 's/VECTORDB_PROVIDER=qdrant/VECTORDB_PROVIDER=sqlite/' .env || true

# 2. Pre-built Verification
if [ ! -f "./server" ]; then
    echo "Pre-built binary not found. Attempting to build from vendor..."
    export LD_LIBRARY_PATH=$(pwd)/lib
    go build -mod=vendor -o server ./cmd/server/
fi
chmod +x ./server

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
chmod +x ./server
nohup ./server > server.log 2>&1 &
echo $! > server.pid

echo "======================================"
echo "Server started successfully (Offline Mode)!"
echo "PID: $(cat server.pid)"
echo "Logs: tail -f server.log"
echo "======================================"
