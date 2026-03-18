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

# 4. Run Server (Foreground for Container longevity)
echo "Starting lges-mem0ai-go server..."
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
export LD_LIBRARY_PATH="$SCRIPT_DIR/lib:$LD_LIBRARY_PATH"

# The PID of this shell will be the PID of the server after 'exec'
echo $$ > "$DATA_DIR/server.pid" 2>/dev/null || echo $$ > "/tmp/server.pid"

echo "======================================"
echo "Server starting (Read-Only Mode)..."
echo "Logs: Streaming to stdout/stderr"
echo "======================================"

# exec replaces the shell with the server process (PID 1 in containers)
exec ./server
