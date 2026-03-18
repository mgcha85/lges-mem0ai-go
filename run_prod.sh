#!/bin/bash
# run_prod.sh - Production run on host (Offline Standalone - Pre-built)
set -e

# 1. Environment Check
if [ ! -f ".env" ]; then
    echo "Error: .env file not found."
    exit 1
fi

# 2. Extract configuration for runtime (resilient to read-only)
DATA_DIR=$(grep "^DATA_DIR=" .env | cut -d'=' -f2)
DATA_DIR=${DATA_DIR:-"./data"}

# 2. Pre-built Verification
if [ ! -f "./server" ]; then
    echo "Error: server binary not found. It must be pre-built for read-only environments."
    exit 1
fi

# 4. Run Server in background
echo "Starting lges-mem0ai-go server..."
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
export LD_LIBRARY_PATH="$SCRIPT_DIR/lib:$LD_LIBRARY_PATH"

# Direct output to /dev/null to prevent nohup.out in read-only dir
# Logs should be captured from stdout by the supervisor/container engine
nohup ./server > /dev/null 2>&1 & 
SERVER_PID=$!

# Try to write PID to DATA_DIR (NAS), fallback to /tmp
if ! echo $SERVER_PID > "$DATA_DIR/server.pid" 2>/dev/null; then
    echo $SERVER_PID > "/tmp/server.pid"
    echo "PID stored in /tmp/server.pid (DATA_DIR was not writable)"
else
    echo "PID stored in $DATA_DIR/server.pid"
fi

echo "======================================"
echo "Server started successfully (Read-Only Mode)!"
echo "PID: $SERVER_PID"
echo "Logs: Outputting to stdout (Check system logs)"
echo "======================================"
