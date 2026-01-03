# SparksAI Audit Service

Microservice for audit logging with buffered batch inserts.

## Features

- **Buffered Batch Inserts**: Accumulates audit logs and inserts them in batches for efficiency
- **Non-blocking API**: Endpoints return immediately while processing happens in background
- **Configurable Thresholds**: Time-based and count-based flush triggers
- **Auto Database Creation**: Creates database and tables automatically on startup

## API Endpoints

### POST `/api/audit-logs`

Create audit log entries (accepts array of logs). Returns immediately after queuing logs for processing.

**Request Body:**
```json
{
  "logs": [
    {
      "user_id": "uuid-string",
      "endpoint_path": "/api/v1/endpoint",
      "session_id": "550e8400-e29b-41d4-a716-446655440000",
      "action": "action-name",
      "action_date": "2024-01-01T12:00:00Z",
      "count": 42,
      "http_method": "GET",
      "status_code": 200,
      "response_time_seconds": 0.123,
      "ip_address": "192.168.1.1",
      "user_agent": "Mozilla/5.0...",
      "chat_history_id": 123,
      "insights_id": 456,
      "tokens_used": 1000,
      "query_raw": "param1=value1&param2=value2",
      "body_raw": "{\"key\": \"value\"}"
    }
  ]
}
```

**Required Fields:**
- `endpoint_path` (string) - Endpoint path
- `http_method` (string) - HTTP method (GET, POST, PUT, DELETE, etc.)
- `status_code` (integer) - HTTP status code
- `response_time_seconds` (float) - Response time in seconds

**Optional Fields:**
- `user_id` (string) - User ID
- `session_id` (string, UUID) - Session identifier
- `action` (string) - Action name
- `action_date` (string, RFC3339) - Action timestamp
- `count` (integer) - Count value
- `ip_address` (string) - IP address
- `user_agent` (string) - User agent string
- `chat_history_id` (integer) - Chat history ID
- `insights_id` (integer) - Insights ID
- `tokens_used` (integer) - Tokens used
- `query_raw` (string) - Raw query string (will be parsed and stored as JSONB)
- `body_raw` (string) - Raw request body (will be parsed and stored as JSONB)

**Response (202 Accepted):**
```json
{
  "status": "accepted",
  "message": "Audit logs queued for processing",
  "queued_count": 1
}
```

**Error Responses:**
- `400 Bad Request` - Invalid request body or missing required fields
- `500 Internal Server Error` - Server error

---

### GET `/api/audit-logs`

Retrieve audit logs with optional filters. Results are ordered by `created_at DESC` (latest first).

**Query Parameters (all optional):**
- `user_id` (string) - Filter by user ID
- `action` (string) - Filter by action
- `limit` (integer) - Maximum number of records to return (default: 500, max: 500)

**Examples:**
```
GET /api/audit-logs
GET /api/audit-logs?user_id=123&limit=100
GET /api/audit-logs?action=login&limit=50
GET /api/audit-logs?user_id=123&action=login&limit=200
```

**Response (200 OK):**
```json
[
  {
    "id": 1,
    "user_id": "123",
    "endpoint_path": "/api/v1/endpoint",
    "session_id": "550e8400-e29b-41d4-a716-446655440000",
    "action": "login",
    "action_date": "2024-01-01T12:00:00Z",
    "count": 42,
    "http_method": "GET",
    "status_code": 200,
    "response_time_seconds": 0.123,
    "created_at": "2024-01-01T12:00:00Z",
    "ip_address": "192.168.1.1",
    "user_agent": "Mozilla/5.0...",
    "chat_history_id": 123,
    "insights_id": 456,
    "tokens_used": 1000,
    "query_raw": "param1=value1&param2=value2",
    "body_raw": "{\"key\": \"value\"}"
  }
]
```

**Error Responses:**
- `400 Bad Request` - Invalid limit parameter
- `500 Internal Server Error` - Server error

---

### GET `/api/audit-logs/actions`

Returns a list of all distinct action values from audit logs.

**Response (200 OK):**
```json
[
  "action1",
  "action2",
  "action3"
]
```

**Error Responses:**
- `500 Internal Server Error` - Server error

---

### Health Check Endpoints

#### GET `/health`

Basic health check endpoint.

**Response (200 OK):**
```json
{
  "status": "healthy",
  "service": "sparksai-audit-service"
}
```

#### GET `/health/live`

Kubernetes liveness probe - checks if the application is alive.

**Response (200 OK):**
```json
{
  "status": "alive"
}
```

#### GET `/health/ready`

Kubernetes readiness probe - checks if the application is ready to serve traffic (includes database connectivity check).

**Response (200 OK):**
```json
{
  "status": "ready"
}
```

**Response (503 Service Unavailable):**
```json
{
  "status": "not_ready",
  "reason": "database connection unavailable"
}
```

---

## Configuration

Environment variables (in `configs/app.env`):

```env
AGILEAGENT_SERVER_HOMEDIR="/agileagent_serverapp"

# Database
POSTGRES_HOST=yamanote.proxy.rlwy.net
POSTGRES_PORT=23188
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=audit_db

# Server
SERVER_PORT=8083

# Buffering Configuration
AUDIT_BUFFER_MAX_SIZE=100          # Max entries before batch insert
AUDIT_BUFFER_FLUSH_INTERVAL=5      # Seconds before auto-flush
AUDIT_BUFFER_BATCH_SIZE=50         # Max entries per batch insert
```

### Configuration Details

- **POSTGRES_DB**: Database name (will be created if doesn't exist)
- **AUDIT_BUFFER_MAX_SIZE**: Maximum number of logs in buffer before triggering flush
- **AUDIT_BUFFER_FLUSH_INTERVAL**: Time in seconds before auto-flushing buffer
- **AUDIT_BUFFER_BATCH_SIZE**: Maximum number of logs inserted per batch

---

## Database Schema

The service automatically creates the `audit_logs` table with the following structure:

| Column | Type | Description |
|--------|------|-------------|
| `id` | SERIAL PRIMARY KEY | Auto-incrementing ID |
| `user_id` | VARCHAR(255) | User ID (nullable) |
| `endpoint_path` | VARCHAR(500) NOT NULL | Endpoint path |
| `session_id` | UUID | Session identifier (nullable) |
| `action` | VARCHAR(255) | Action name (nullable) |
| `action_date` | TIMESTAMP WITH TIME ZONE | Action timestamp (nullable) |
| `count` | INTEGER | Count value (nullable) |
| `http_method` | VARCHAR(10) NOT NULL | HTTP method |
| `status_code` | INTEGER NOT NULL | HTTP status code |
| `response_time_seconds` | NUMERIC(10, 3) NOT NULL | Response time |
| `created_at` | TIMESTAMP WITH TIME ZONE | Creation timestamp (auto) |
| `ip_address` | INET | IP address (nullable) |
| `user_agent` | TEXT | User agent (nullable) |
| `chat_history_id` | INTEGER | Chat history ID (nullable) |
| `insights_id` | INTEGER | Insights ID (nullable) |
| `tokens_used` | INTEGER | Tokens used (nullable) |
| `query_raw` | JSONB | Raw query parameters (parsed and stored as JSONB) |
| `body_raw` | JSONB | Raw request body (parsed and stored as JSONB) |

**Indexes:**
- `idx_audit_logs_user_id` on `user_id`
- `idx_audit_logs_action` on `action`
- `idx_audit_logs_query_raw` on `query_raw` (GIN index for JSONB)
- `idx_audit_logs_body_raw` on `body_raw` (GIN index for JSONB)

---

## Building and Running

### Prerequisites

- Go 1.25 or higher
- PostgreSQL database (accessible via connection string in config)

### Local Development

```bash
# Navigate to service directory
cd audit-service

# Install dependencies
go mod download

# Run the service
go run cmd/main.go
```

The service will:
- Create `audit_db` database if it doesn't exist
- Create `audit_logs` table if it doesn't exist
- Start listening on port 8083 (or port specified in config)

### Docker

```bash
# Build image
docker build -t sparksai-audit-service .

# Run container
docker run -p 8083:8083 sparksai-audit-service
```

---

## Architecture

### Components

1. **Buffer** (`internal/buffer/buffer.go`)
   - Thread-safe buffered channel that accumulates audit logs
   - Non-blocking: drops logs if channel is full (with warning)
   - Background worker continuously processes logs

2. **Repository** (`internal/auditlog/service/repository.go`)
   - Handles database operations
   - Batch insert with transaction support

3. **Service** (`internal/auditlog/service/service.go`)
   - HTTP handlers for creating and reading audit logs
   - Validates input and queues logs to buffer
   - Supports filtering and pagination for read operations

### How It Works

1. **Write Flow:**
   - Client sends POST request with audit logs
   - Service validates and adds logs to buffer (non-blocking)
   - Returns 202 Accepted immediately
   - Background worker batches logs and inserts to database

2. **Read Flow:**
   - Client sends GET request with optional filters
   - Service queries database directly
   - Returns filtered results ordered by latest first

3. **Buffering:**
   - Logs accumulate in buffered channel
   - Worker flushes when:
     - Batch size reached (`AUDIT_BUFFER_BATCH_SIZE`)
     - Time interval elapsed (`AUDIT_BUFFER_FLUSH_INTERVAL`)

---

## Testing

### Test Create Audit Logs

```bash
curl -X POST http://localhost:8083/api/audit-logs \
  -H "Content-Type: application/json" \
  -d '{
    "logs": [{
      "endpoint_path": "/api/v1/test",
      "http_method": "GET",
      "status_code": 200,
      "response_time_seconds": 0.123
    }]
  }'
```

### Test Get Audit Logs

```bash
# Get all logs (up to 500)
curl http://localhost:8083/api/audit-logs

# Get logs with filters
curl "http://localhost:8083/api/audit-logs?user_id=123&limit=100"

# Get distinct actions
curl http://localhost:8083/api/audit-logs/actions
```

### Test Health Endpoints

```bash
curl http://localhost:8083/health
curl http://localhost:8083/health/live
curl http://localhost:8083/health/ready
```

---

## Project Structure

```
audit-service/
├── cmd/
│   └── main.go                    # Service entry point
├── configs/
│   └── app.env                    # Configuration
├── internal/
│   ├── db/
│   │   └── db.go                  # Database connection & table creation
│   ├── auditlog/
│   │   ├── auditlog.go            # Struct & interface
│   │   └── service/
│   │       ├── repository.go      # Database operations
│   │       ├── write_service.go    # Write handlers
│   │       └── read_service.go     # Read handlers
│   ├── buffer/
│   │   └── buffer.go               # Buffering logic
│   └── routes/
│       └── routes.go               # Route setup
├── go.mod
├── Dockerfile
├── railway.json
└── README.md
```

---

## Notes

- The service follows the same architectural pattern as `SparksAI-user-service`
- Database and tables are created automatically on startup
- Buffering ensures high performance by batching database writes
- All endpoints are non-blocking for write operations
- Read operations query database directly
