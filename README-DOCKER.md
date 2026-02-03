# Docker Setup for Job Queue System

This guide explains how to run the Job Queue System using Docker.

## Prerequisites

- Docker
- Docker Compose

## Quick Start

### 1. Build and Start All Services

```bash
docker-compose up -d
```

This will start:
- **API Server** on `http://localhost:8080`
- **Worker** (background job processor)
- **Web Dashboard** on `http://localhost:3000`

### 2. View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f api
docker-compose logs -f worker
docker-compose logs -f web
```

### 3. Stop All Services

```bash
docker-compose down
```

### 4. Stop and Remove Volumes (Clean Database)

```bash
docker-compose down -v
```

## Services

### API Server
- **Port**: 8080
- **Health Check**: `http://localhost:8080/metrics`
- **Database**: `/app/data/jobs.db` (shared volume)

### Worker
- **Database**: `/app/data/jobs.db` (shared volume)
- Processes jobs from the database

### Web Dashboard
- **Port**: 3000
- **URL**: `http://localhost:3000`
- Serves static files from `./web` directory

## Data Persistence

The database is stored in `./data/jobs.db` on your host machine, so data persists even when containers are stopped.

## Development

### Rebuild After Code Changes

```bash
docker-compose build
docker-compose up -d
```

### Run Individual Services

```bash
# Start only API
docker-compose up api

# Start API and Worker
docker-compose up api worker
```

## Troubleshooting

### Check Service Status

```bash
docker-compose ps
```

### View Service Logs

```bash
docker-compose logs api
docker-compose logs worker
docker-compose logs web
```

### Restart a Service

```bash
docker-compose restart api
docker-compose restart worker
docker-compose restart web
```

### Access Container Shell

```bash
# API container
docker-compose exec api sh

# Worker container
docker-compose exec worker sh
```

### Check Database

```bash
# Access API container and check database
docker-compose exec api sh
sqlite3 /app/data/jobs.db "SELECT COUNT(*) FROM jobs;"
```

## Production Considerations

For production use, consider:

1. **Environment Variables**: Use environment variables for configuration
2. **Secrets Management**: Don't hardcode sensitive data
3. **Resource Limits**: Add resource limits to docker-compose.yml
4. **Health Checks**: Already included for API service
5. **Logging**: Configure proper logging aggregation
6. **Database**: Consider using PostgreSQL instead of SQLite for production
7. **Reverse Proxy**: Add nginx/traefik for production deployment

## Example Production docker-compose.yml

```yaml
version: '3.8'

services:
  api:
    build: .
    command: /app/api -db /app/data/jobs.db -port 8080
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    restart: always
    deploy:
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M

  worker:
    build: .
    command: /app/worker -db /app/data/jobs.db
    volumes:
      - ./data:/app/data
    restart: always
    deploy:
      replicas: 2  # Run multiple workers
      resources:
        limits:
          cpus: '0.5'
          memory: 512M
```
