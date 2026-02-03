# Distributed Task Queue & Job Processor

A distributed job queue system built in Go with a web dashboard for monitoring and managing jobs.

## Features

- **Job Management**: Create, track, and monitor jobs through REST API
- **Worker Processing**: Background workers process jobs with lease-based execution
- **Retry Logic**: Automatic retry for failed jobs (max 3 retries)
- **Dead Letter Queue**: Failed jobs after max retries are moved to DLQ
- **Rate Limiting**: Per-tenant limits (5 concurrent jobs, 10 submissions/minute)
- **Idempotency**: Prevent duplicate jobs using idempotency keys
- **Web Dashboard**: Real-time dashboard to view metrics and job status
- **Observability**: Metrics endpoint and comprehensive logging

## Quick Start

### Using Docker (Recommended)

1. **Start all services:**
   ```bash
   docker-compose up -d
   ```

2. **Access the dashboard:**
   - Web Dashboard: http://localhost:3001
   - API Server: http://localhost:8081

3. **Stop services:**
   ```bash
   docker-compose down
   ```

### Using Go (Local Development)

1. **Install dependencies:**
   ```bash
   go mod download
   ```

2. **Start API server:**
   ```bash
   go run cmd/api/main.go -db jobs.db -port 8080
   ```

3. **Start worker (in separate terminal):**
   ```bash
   go run cmd/worker/main.go -db jobs.db
   ```

4. **Start web dashboard (in separate terminal):**
   ```bash
   go run cmd/web/main.go -port 3000
   ```

5. **Access the dashboard:**
   - Web Dashboard: http://localhost:3000
   - API Server: http://localhost:8080

## Project Structure

```
.
├── cmd/
│   ├── api/          # API server
│   ├── worker/       # Background worker
│   └── web/          # Web dashboard server
├── internal/
│   ├── handler/       # HTTP handlers
│   ├── service/      # Business logic
│   ├── repository/  # Database layer
│   ├── models/       # Data models
│   └── metrics/      # Metrics tracking
├── web/              # Frontend (HTML, CSS, JS)
├── migrations/       # Database schema
├── docker-compose.yml
└── Dockerfile
```

## API Endpoints

### Create Job
```bash
POST /jobs
Content-Type: application/json

{
  "tenant_id": "tenant-1",
  "payload": "job data",
  "idempotency_key": "optional-key",
  "max_retries": 3
}
```

### Get Job
```bash
GET /jobs/{job-id}
```

### List Jobs by Status
```bash
GET /jobs?status=PENDING
GET /jobs?status=RUNNING
GET /jobs?status=DONE
GET /jobs?status=FAILED
```

### Get Metrics
```bash
GET /metrics
```

### Get Dead Letter Queue
```bash
GET /dlq
```

## Job Lifecycle

1. **PENDING** → Job is created and waiting to be processed
2. **RUNNING** → Worker leases and processes the job
3. **DONE** → Job completed successfully
4. **FAILED** → Job failed (will retry if retries remaining)
5. **DLQ** → Job moved to Dead Letter Queue after max retries

## Configuration

### API Server
- `-db`: Database file path (default: `jobs.db`)
- `-port`: HTTP server port (default: `8080`)

### Worker
- `-db`: Database file path (default: `jobs.db`)

### Web Dashboard
- `-port`: HTTP server port (default: `3000`)

## Rate Limiting

- **Concurrent Jobs**: Max 5 RUNNING jobs per tenant
- **Submission Rate**: Max 10 job submissions per minute per tenant

## Testing

Use the provided test script:
```bash
./test-api.sh
```

Or use curl:
```bash
# Create a job
curl -X POST http://localhost:8081/jobs \
  -H "Content-Type: application/json" \
  -d '{"tenant_id": "test", "payload": "test job"}'

# Get metrics
curl http://localhost:8081/metrics
```

## Docker Ports

- **API**: 8081 (mapped from container port 8080)
- **Web Dashboard**: 3001 (mapped from container port 3000)

## License

This is an assignment project.
