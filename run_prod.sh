#!/bin/bash
# run_prod.sh - Production run on host (Direct Execution, No Podman/Docker)
set -e

# 1. Environment Check and Setup
if [ ! -f ".env" ]; then
    if [ -f ".env.example" ]; then
        cp .env.example .env
        echo "Created .env from .env.example."
    else
        echo "Error: .env file not found."
        exit 1
    fi
fi

# Ensure Vector DB provider is set to sqlite for host-based standalone execution
# if not already set or if it was set to qdrant (which requires a separate service)
sed -i 's/VECTORDB_PROVIDER=qdrant/VECTORDB_PROVIDER=sqlite/' .env || true

# 2. Build Go Server
echo "Building lges-mem0ai-go server..."
go build -o server ./cmd/server/

# 3. Re-combine Models if needed (Standalone robustness)
if [ ! -d "models" ] && [ -f "models.tar.gz.part_aa" ]; then
    echo "Extracting model parts..."
    cat models.tar.gz.part_* > models.tar.gz
    tar -xvzf models.tar.gz
    rm models.tar.gz
fi

# 4. Run Server in background
echo "Starting server in background..."
export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$(pwd)/lib
nohup ./server > server.log 2>&1 &
echo $! > server.pid

echo "======================================"
echo "Server started successfully!"
echo "PID: $(cat server.pid)"
echo "Logs: tail -f server.log"
echo "======================================"
