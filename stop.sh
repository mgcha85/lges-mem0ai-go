#!/bin/bash
# stop.sh - Stop host processes (No Podman/Docker)

# 1. Stop Go Server
if [ -f "server.pid" ]; then
    PID=$(cat server.pid)
    echo "Stopping lges-mem0ai-go server (PID: $PID)..."
    kill $PID || true
    rm server.pid
else
    echo "No server.pid found. Attempting pkill..."
    pkill server || true
fi

echo "All services stopped."
