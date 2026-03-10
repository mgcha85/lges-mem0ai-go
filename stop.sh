#!/bin/bash
# stop.sh - Stop host processes and Qdrant container

# 1. Stop Go Server
if [ -f "server.pid" ]; then
    PID=$(cat server.pid)
    echo "Stopping server (PID: $PID)..."
    kill $PID || true
    rm server.pid
else
    echo "No server.pid found. Trying pkill..."
    pkill server || true
fi

# 2. Stop Qdrant Container
echo "Stopping Qdrant container..."
podman stop qdrant_server || true
podman rm qdrant_server || true

echo "All services stopped."
