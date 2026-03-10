# Build stage
FROM docker.io/library/golang:1.24 AS builder

WORKDIR /app

# Copy the main project and the local dependency
COPY . .

# Build the application
RUN go build -o server ./cmd/server/

# Runtime stage
FROM docker.io/library/ubuntu:22.04

WORKDIR /app

# Install necessary runtime dependencies (e.g., for SQLite and networking)
RUN apt-get update && apt-get install -y \
    ca-certificates \
    libsqlite3-0 \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary and required assets
COPY --from=builder /app/server .
COPY --from=builder /app/lib ./lib
COPY --from=builder /app/models ./models
COPY --from=builder /app/.env.example ./.env

# Create data directory
RUN mkdir -p /app/data

# Environment variables for dynamic linking
ENV LD_LIBRARY_PATH=/app/lib

# Expose the server port
EXPOSE 8080

# Start command
CMD ["./server"]
