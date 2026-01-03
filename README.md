# SparksAI Audit Service

A microservice for audit logging with buffered batch inserts. Collects and stores API audit logs efficiently using non-blocking batch processing.

## What It Does

- **Collects audit logs** from API requests (endpoint, method, status, response time, user info, etc.)
- **Buffers logs** in memory and inserts them in batches to PostgreSQL for high performance
- **Non-blocking API** - returns immediately while processing happens in background
- **Auto-creates** database and tables on startup
- **Provides query endpoints** to retrieve and filter audit logs

## API Endpoints

### POST `/api/audit-logs`
Create audit log entries. Accepts array of logs, returns immediately.

**Required Parameters:**
- `endpoint_path` (string) - API endpoint path
- `http_method` (string) - HTTP method (GET, POST, PUT, DELETE, etc.)
- `status_code` (integer) - HTTP status code
- `response_time_seconds` (float) - Response time in seconds

**Optional Parameters:**
- `user_id`, `session_id`, `action`, `action_date`, `count`, `ip_address`, `user_agent`, `chat_history_id`, `insights_id`, `tokens_used`, `query_raw`, `body_raw`

### GET `/api/audit-logs`
Retrieve audit logs with optional filters.

**Query Parameters:**
- `user_id` (string) - Filter by user ID
- `action` (string) - Filter by action
- `limit` (integer) - Max records (default: 500, max: 500)

### GET `/api/audit-logs/actions`
Returns list of all distinct action values.

### Health Endpoints
- `GET /health` - Basic health check
- `GET /health/live` - Kubernetes liveness probe
- `GET /health/ready` - Kubernetes readiness probe (checks DB connection)

## Configuration Parameters

Environment variables in `configs/app.env`:

**Database:**
- `POSTGRES_HOST` - PostgreSQL host
- `POSTGRES_PORT` - PostgreSQL port
- `POSTGRES_USER` - Database user
- `POSTGRES_PASSWORD` - Database password
- `POSTGRES_DB` - Database name (auto-created if missing)

**Server:**
- `SERVER_PORT` - Server port (default: 8083)

**Buffering:**
- `AUDIT_BUFFER_MAX_SIZE` - Max entries in buffer before flush (default: 100)
- `AUDIT_BUFFER_FLUSH_INTERVAL` - Auto-flush interval in seconds (default: 30)
- `AUDIT_BUFFER_BATCH_SIZE` - Max entries per batch insert (default: 100)

## Quick Start

```bash
# Install dependencies
go mod download

# Run the service
go run cmd/main.go
```

The service will automatically create the database and tables on startup.

## Docker

```bash
docker build -t sparksai-audit-service .
docker run -p 8083:8083 sparksai-audit-service
```

## Prerequisites

- Go 1.25+
- PostgreSQL database
