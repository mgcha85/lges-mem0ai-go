# Deployment Manual: lges-mem0ai-go

This document describes how to deploy the `lges-mem0ai-go` server as a standalone application using Podman on Ubuntu 22.04.

## Prerequisites

- Ubuntu 22.04 LTS
- Go 1.24+ (for building on host)
- Internet access (for building/API keys)

## Installation

1. **Configure Environment Variables**:

   Copy `.env.example` to `.env` and fill in your API keys.

   ```bash
   cp .env.example .env
   ```

2. **Deploy the Services**:

   Run the `run_prod.sh` script to build and start the services on host.

   ```bash
   chmod +x *.sh
   ./run_prod.sh
   ```

   This will:
   - Build the Go application binary on the host.
   - Reconstitute models if they were split.
   - Run the `lges-mem0ai-go` server in the background using SQLite for both data and vector storage.

## Managing the Application

- **Check Status**: `ps aux | grep server`
- **View Logs**: `tail -f server.log`
- **Stop Services**: `./stop.sh`
- **Development Run**: `./run_dev.sh` (runs locally in foreground)

## Data Persistence

- **SQLite Database**: Managed by the Go application in `./data/`.
- **Qdrant Storage**: Persistence handled via `./qdrant_data/`.

## Verification

After starting the services, verify the deployment:

```bash
curl http://localhost:8080/health
```

Expected response: `{"status":"ok","version":"2.0.0"}`
