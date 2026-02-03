# Build stage
FROM golang:1.21-bullseye AS builder

# Install build dependencies for SQLite (CGO)
RUN apt-get update && apt-get install -y \
    gcc \
    libc6-dev \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all binaries with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/api ./cmd/api
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=1 GOOS=linux go build -o /bin/web ./cmd/web

# Copy web directory to a known location for runtime stage
RUN mkdir -p /web && cp -r /build/web /web/

# Runtime stage
FROM debian:bullseye-slim

# Install SQLite, CA certificates, and wget for healthcheck
RUN apt-get update && apt-get install -y \
    ca-certificates \
    sqlite3 \
    wget \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binaries from builder (rename web binary to avoid conflict)
COPY --from=builder /bin/api /app/api
COPY --from=builder /bin/worker /app/worker
COPY --from=builder /bin/web /app/web-server

# Copy web static files from builder
COPY --from=builder /web/ /app/web/

# Create data directory for database
RUN mkdir -p /app/data

# Expose ports
EXPOSE 8080 3000

# Default command (can be overridden in docker-compose)
CMD ["/app/api"]
