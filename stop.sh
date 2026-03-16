#!/bin/bash
# stop.sh - Stop host processes (No Podman/Docker)

# 1. Load DataDir from .env
DATA_DIR=$(grep "^DATA_DIR=" .env | cut -d'=' -f2)
DATA_DIR=${DATA_DIR:-"./data"}

# 2. Stop Go Server
PID_FILE=""
if [ -f "$DATA_DIR/server.pid" ]; then
    PID_FILE="$DATA_DIR/server.pid"
elif [ -f "/tmp/server.pid" ]; then
    PID_FILE="/tmp/server.pid"
elif [ -f "server.pid" ]; then
    PID_FILE="server.pid"
fi

if [ -n "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    echo "Stopping lges-mem0ai-go server (PID: $PID)..."
    kill $PID || true
    rm "$PID_FILE" 2>/dev/null || true
else
    echo "No server.pid found. Attempting pkill..."
    pkill server || true
fi

echo "All services stopped."
