# Deployment Manual: lges-mem0ai-go

This document describes how to deploy the `lges-mem0ai-go` server as a standalone application using Podman on Ubuntu 22.04.

## Prerequisites

- Ubuntu 22.04 LTS
- Podman and Podman-Compose
- Internet access (for building images and downloading Qdrant)

## Installation

1. **Install Podman and Podman-Compose** (if not already installed):

   ```bash
   sudo apt update
   sudo apt install -y podman podman-compose
   ```

2. **Configure Environment Variables**:

   Copy `.env.example` to `.env` and fill in your API keys and configuration.

   ```bash
   cp .env.example .env
   # Edit .env with your preferred editor
   ```

3. **Deploy the Services**:

   Run the `start.sh` or `run_prod.sh` script to build and start the containers.

   ```bash
   chmod +x *.sh
   ./start.sh
   ```

   This will:
   - Build the Go application container using the provided `Dockerfile`.
   - Start the `server` and `qdrant` services in the background.

## Managing the Application

- **Check Status**: `podman-compose ps`
- **View Logs**: `podman-compose logs -f`
- **Stop Services**: `./stop.sh`
- **Development Run**: `./run_dev.sh` (runs locally without containers)

## Data Persistence

- **SQLite Database**: Stored in `./data/vector.db` (or as configured in `.env`).
- **Qdrant Storage**: Stored in `./qdrant_data/`.

## Verification

After starting the services, verify the deployment:

```bash
curl http://localhost:8080/health
```

Expected response: `{"status":"ok","version":"2.0.0"}`
