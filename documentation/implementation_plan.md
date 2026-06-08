# Rayavriti NetMonitor — Backend Migration to Go + Alert Engine Redesign + Production Logging & Observability

Migrate the entire Node.js/TypeScript backend (`server/`) to a production-grade Go backend while preserving **100% HTTP REST API compatibility**, building a proper **rule-based alert engine**, and implementing a **comprehensive logging & monitoring system** that records every minute detail of application behavior — from HTTP requests and database queries to collector executions, alert decisions, and security events. The realtime layer changes from Socket.IO to a native WebSocket protocol unless a temporary Node.js Socket.IO bridge is explicitly chosen.

---

## User Review Required

> [!IMPORTANT]
> **Breaking change to project structure**: The `server/` directory will be replaced with a `backend/` directory containing Go code. The npm workspace for `server` will be removed. The monorepo structure changes from `client + server` (npm workspaces) to `client` (npm) + `backend` (Go modules). The root `package.json` will only manage the client workspace.

> [!WARNING]
> **Database migration**: The backend will move from SQLite to PostgreSQL with the TimescaleDB extension. Existing `.db` files are not runtime-compatible with the new backend and must be migrated through an explicit export/import migration command before cutover. New tables are added for the alert engine (`alert_rules`, `alert_rule_conditions`, `notification_channels`, `alert_history`). No data loss when the migration command is run successfully and verified before retiring the SQLite file.

> [!IMPORTANT]
> **Deployment change**: The Dockerfile changes from a Node.js multi-stage build to a Go multi-stage build. Docker Compose files will be updated. The production image embeds the React SPA. A no-packet-capture build can be static; the packet-capture-enabled build requires CGo and the runtime `libpcap` package.

> [!WARNING]
> **Frontend realtime change**: REST endpoints keep the existing response shapes, but the React realtime client must migrate from Socket.IO to the native WebSocket protocol described in Phase 6 unless the temporary Node.js bridge option is selected.

## Open Questions

> [!IMPORTANT]
> **1. Database deployment**: PostgreSQL + TimescaleDB becomes a required runtime dependency. Docker Compose will provide a `timescaledb` service for development and production-like deployments. Non-Docker installs must provision PostgreSQL with `CREATE EXTENSION IF NOT EXISTS timescaledb;` before starting the backend.

> [!IMPORTANT]
> **2. Alert notification channels**: The new alert engine design includes webhook, email (SMTP), and Slack notification channels. Should we implement all three in Phase 1, or start with webhooks only and add email/Slack later?

> [!IMPORTANT]
> **3. Realtime transport**: Go does not have a mature, maintained Socket.IO v4 server with full feature parity. The previously considered `googollee/go-socket.io` package is archived and only supports old Socket.IO clients, so it must not be used as the v4 compatibility path. Options:
> - **A) Native WebSocket (`gorilla/websocket` or `nhooyr.io/websocket`) + small typed event protocol** — Recommended. Requires replacing the React Socket.IO client, but gives a maintained Go implementation and explicit auth/RBAC handling.
> - **B) Server-sent events for dashboard updates + REST commands** — Simpler for one-way updates, but not enough for packet capture/control channels without additional APIs.
> - **C) Keep a small Node.js Socket.IO bridge temporarily** — Preserves the current client during migration, but keeps a second runtime and should be treated as a short-lived compatibility bridge.
> - **Recommendation**: Option A unless zero frontend changes is a hard requirement for the migration window.

> [!IMPORTANT]
> **4. Packet capture**: The Node.js backend uses the `cap` npm package (libpcap bindings). Go has `google/gopacket` with native libpcap support. This requires CGo for the pcap portion. Are you okay with CGo being required for the packet capture feature specifically?

---

## Proposed Changes

The migration is organized into **8 phases**, designed so that each phase produces a working, testable backend.

---

### Phase 1: Go Project Scaffolding & Core Infrastructure

Set up the Go module, project layout, configuration loading, structured logging, and the HTTP server skeleton.

#### [NEW] [go.mod](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/go.mod)
Go module definition: `github.com/rayavriti/netmonitor-backend`

#### [NEW] Project Layout
```
backend/
├── cmd/
│   └── server/
│       └── main.go              # Entry point, DI wiring, graceful shutdown
├── internal/
│   ├── config/
│   │   └── config.go            # Env-based config (maps .env to Go struct)
│   ├── server/
│   │   ├── server.go            # HTTP server, middleware chain, router setup
│   │   ├── middleware.go         # CORS, helmet-equivalent headers, rate limiter, request ID
│   │   └── response.go          # sendOk/sendError JSON envelope helpers
│   ├── auth/
│   │   ├── jwt.go               # JWT creation, validation, refresh flow (HS256)
│   │   ├── password.go          # scrypt hashing + SHA-256 legacy fallback
│   │   ├── session.go           # In-memory session store (token → user mapping)
│   │   ├── middleware.go        # requireAuth, authenticateV1, extractToken
│   │   └── apikey.go            # API key verification
│   ├── database/
│   │   ├── database.go          # Interface definitions (Go equivalent of types.ts)
│   │   ├── postgres.go          # PostgreSQL implementation (pgxpool, transactions, all queries)
│   │   ├── migrations.go        # Versioned schema migrations + Timescale hypertables
│   │   ├── sqlite_import.go     # One-time SQLite .db export/import migration helper
│   │   └── seed.go              # Default devices, users, API keys seeding
│   ├── models/
│   │   └── models.go            # Struct definitions: Device, Sensor, Metric, Alert, etc.
│   ├── handlers/
│   │   ├── auth.go              # POST /api/auth/login, /logout, /me, v1 auth endpoints
│   │   ├── devices.go           # CRUD /api/devices, /api/v1/devices
│   │   ├── sensors.go           # CRUD /api/v1/sensors
│   │   ├── metrics.go           # GET /api/metrics/*, /api/v1/metrics/query
│   │   ├── alerts.go            # CRUD /api/alerts, /api/v1/alerts + acknowledge/resolve
│   │   ├── reports.go           # GET /api/reports/summary, timeseries, devices, CSV
│   │   ├── insights.go          # GET /api/insights, /api/insights/history
│   │   ├── flows.go             # GET /api/v1/flows, top-talkers, protocols, timeseries
│   │   ├── capture.go           # Packet capture endpoints
│   │   ├── ports.go             # Port scan endpoints
│   │   ├── dashboards.go        # CRUD /api/v1/dashboards
│   │   ├── simulator.go         # POST /api/simulator/metrics, /api/simulator/flows
│   │   └── health.go            # GET /health
│   ├── collectors/
│   │   ├── collector.go         # Collector interface + registry
│   │   ├── ping.go              # ICMP ping (go-ping or raw socket)
│   │   ├── http.go              # HTTP/HTTPS endpoint check
│   │   ├── port.go              # TCP port connectivity check
│   │   ├── snmp.go              # SNMP v1/v2c/v3 polling (gosnmp)
│   │   ├── system.go            # Local system metrics (gopsutil)
│   │   ├── netflow.go           # NetFlow v5/v9 + sFlow UDP collector
│   │   └── packet_capture.go    # libpcap-based packet capture (gopacket)
│   ├── scanner/
│   │   └── port_scanner.go      # Concurrent TCP port scanner
│   ├── scheduler/
│   │   └── scheduler.go         # Cron-like device polling scheduler
│   ├── engine/
│   │   ├── anomaly.go           # AI health scoring (port of anomalyEngine.ts)
│   │   ├── alert_engine.go      # NEW: Rule-based alert engine (see Phase 7)
│   │   ├── alert_rules.go       # Rule evaluation logic
│   │   ├── alert_conditions.go  # Condition types and matchers
│   │   ├── notifier.go          # Notification dispatch (webhook, email, slack)
│   │   └── flow_analyzer.go     # Flow analysis + anomaly detection
│   ├── logging/
│   │   ├── logger.go            # Core structured logger (slog wrapper, context-aware)
│   │   ├── context.go           # Request-scoped log context (request ID, user, trace)
│   │   ├── http_logger.go       # HTTP request/response middleware logger
│   │   ├── db_logger.go         # Database query logger (query, args, duration, rows)
│   │   ├── collector_logger.go  # Collector execution logger (device, protocol, result)
│   │   ├── audit_logger.go      # Security & user action audit logger
│   │   ├── alert_logger.go      # Alert engine decision trail logger
│   │   ├── ws_logger.go         # WebSocket event logger
│   │   ├── rotation.go          # Log file rotation & management
│   │   └── sink.go              # Multi-sink output (stdout, file, DB)
│   ├── monitoring/
│   │   ├── recorder.go          # Operational metrics recorder (writes to monitoring DB tables)
│   │   ├── self_monitor.go      # Self-monitoring goroutine (memory, goroutines, GC, uptime)
│   │   └── handlers.go          # GET /api/v1/system/logs, /system/monitoring endpoints
│   ├── retention/
│   │   └── retention.go         # Periodic data pruning scheduler
│   └── websocket/
│       └── hub.go               # Native WebSocket hub for real-time events
├── Makefile                     # build, test, lint, run targets
└── Dockerfile                   # Multi-stage Go build
```

#### Key Go Dependencies
| Purpose | Package |
|---|---|
| HTTP Router | `net/http` (stdlib) + `chi` or `gorilla/mux` |
| PostgreSQL Driver | `github.com/jackc/pgx/v5` |
| TimescaleDB | PostgreSQL extension, provisioned by migrations |
| Structured Logging | `log/slog` (stdlib, Go 1.21+) |
| Log Rotation | `gopkg.in/natefinch/lumberjack.v2` |
| JWT | `github.com/golang-jwt/jwt/v5` |
| SNMP | `github.com/gosnmp/gosnmp` |
| Ping | `github.com/prometheus-community/pro-bing` |
| Packet Capture | `github.com/google/gopacket` |
| System Metrics | `github.com/shirou/gopsutil/v4` |
| Runtime Metrics | `runtime` (stdlib) — goroutines, memory, GC |
| Realtime | `github.com/gorilla/websocket` or `nhooyr.io/websocket` |
| Password (scrypt) | `golang.org/x/crypto/scrypt` |
| CORS | `github.com/rs/cors` |
| Rate Limiting | `golang.org/x/time/rate` |
| Cron Scheduling | `github.com/robfig/cron/v3` |
| Email (SMTP) | `net/smtp` (stdlib) |
| HTTP Client | `net/http` (stdlib) |

#### [MODIFY] [package.json](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/package.json)
- Remove `server` from workspaces
- Update scripts to reference the Go binary

---

### Phase 1B: Logging System & Observability Infrastructure 📋

Build a **zero-blind-spot logging system** that captures every operation, decision, and state change in the application. Every log entry is structured JSON with full context — making it trivially searchable, parseable, and ingestable by any log management tool.

#### Logging Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Application Code                                │
│  (Handlers, Collectors, Scheduler, Alert Engine, WebSocket, DB)     │
└─────────────────────┬───────────────────────────────────────────────┘
                      │  structured log calls with context
                      ▼
┌─────────────────────────────────────────────────────────────────────┐
│                   Core Logger (slog wrapper)                        │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ Request      │  │ Log Level    │  │ Context Enrichment       │  │
│  │ Context      │  │ Filtering    │  │ (request_id, user,       │  │
│  │ Propagation  │  │ (per-module) │  │  trace_id, component)    │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
└─────────────────────┬───────────────────────────────────────────────┘
                      │  fan-out to multiple sinks
          ┌───────────┼───────────────┐
          ▼           ▼               ▼
   ┌────────────┐ ┌────────────┐ ┌────────────┐
   │  stdout    │ │  Log File  │ │ PostgreSQL │
   │  (JSON /   │ │  (rotated) │ │ Monitoring │
   │  pretty)   │ │  lumberjack│ │  Tables    │
   └────────────┘ └────────────┘ └────────────┘
```

#### [NEW] [logger.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/logging/logger.go)

Core structured logger wrapping `log/slog` with multi-sink output:

```go
// Log levels — 6 levels for granular control
const (
    LevelTrace  slog.Level = -8  // Packet-level detail, raw payloads
    LevelDebug  slog.Level = -4  // Internal state, function entry/exit
    LevelInfo   slog.Level = 0   // Normal operations, successful actions
    LevelWarn   slog.Level = 4   // Degraded conditions, non-fatal issues
    LevelError  slog.Level = 8   // Failures that need attention
    LevelFatal  slog.Level = 12  // Unrecoverable errors, shutdown
)

// Every log entry includes these base fields
type LogEntry struct {
    Timestamp   string `json:"timestamp"`    // ISO 8601 with nanoseconds
    Level       string `json:"level"`        // TRACE/DEBUG/INFO/WARN/ERROR/FATAL
    Component   string `json:"component"`    // e.g., "http", "db", "collector.ping", "alert_engine"
    RequestID   string `json:"request_id"`   // Propagated from HTTP context (empty for bg jobs)
    TraceID     string `json:"trace_id"`     // Links related operations across components
    UserID      string `json:"user_id"`      // Authenticated user (empty for anonymous/system)
    Message     string `json:"message"`      // Human-readable description
    Data        any    `json:"data"`         // Structured payload (varies by log category)
    DurationMs  *float64 `json:"duration_ms"` // Operation duration (nil if not applicable)
    Error       *string  `json:"error"`       // Error message (nil if success)
    StackTrace  *string  `json:"stack_trace"` // Go stack trace (only for ERROR/FATAL)
    Hostname    string `json:"hostname"`     // Machine hostname
    PID         int    `json:"pid"`          // Process ID
    GoRoutineID int    `json:"goroutine_id"` // Goroutine ID (for concurrency debugging)
    Version     string `json:"version"`      // Application version
}
```

**Configuration** (via environment variables):

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `debug` (dev) / `info` (prod) | Minimum log level for stdout |
| `LOG_FORMAT` | `pretty` (dev) / `json` (prod) | Output format |
| `LOG_FILE_ENABLED` | `false` | Write logs to file |
| `LOG_FILE_PATH` | `./data/logs/netmonitor.log` | Log file location |
| `LOG_FILE_MAX_SIZE_MB` | `100` | Max size before rotation |
| `LOG_FILE_MAX_BACKUPS` | `10` | Number of rotated files to keep |
| `LOG_FILE_MAX_AGE_DAYS` | `30` | Max age of rotated files |
| `LOG_FILE_COMPRESS` | `true` | Gzip compress rotated files |
| `LOG_DB_ENABLED` | `true` | Write select logs to monitoring DB tables |
| `LOG_MODULE_LEVELS` | `""` | Per-module overrides, e.g., `"db=debug,collector=trace,http=info"` |
| `LOG_SLOW_QUERY_MS` | `100` | Threshold for slow query warnings |
| `LOG_SLOW_REQUEST_MS` | `1000` | Threshold for slow request warnings |
| `LOG_DB_SAMPLE_RATE` | `1.0` | Fraction of non-error DB query logs persisted to monitoring tables |
| `LOG_DB_QUEUE_SIZE` | `10000` | Async queue capacity for DB log writes |
| `LOG_DB_DROP_POLICY` | `drop_debug` | Backpressure policy when the DB log queue is full |

Database-backed logging must be asynchronous and recursion-safe:
- Application code writes log records to a bounded channel; request handlers and collectors do not block on monitoring inserts.
- Writes to monitoring/log tables are never logged back into `monitoring_db_queries`.
- Errors, audit events, and security events bypass sampling; high-volume DEBUG/TRACE records may be sampled or dropped under pressure.
- Sensitive values are redacted before enqueue, not only before stdout/file output.

---

#### Log Categories & What Gets Recorded

Every subsystem has a dedicated logger that captures category-specific structured fields. Below is the **exhaustive** list of what gets logged.

---

##### Category 1: HTTP Request/Response Logging

**File:** [http_logger.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/logging/http_logger.go)

Wraps every HTTP request in a middleware that logs both the request and response with full detail.

```json
// ── REQUEST START ──
{
  "timestamp": "2026-06-05T12:30:01.123456789Z",
  "level": "INFO",
  "component": "http",
  "request_id": "a1b2c3d4e5f6g7h8",
  "message": "→ POST /api/v1/devices",
  "data": {
    "event": "request_start",
    "method": "POST",
    "path": "/api/v1/devices",
    "full_url": "/api/v1/devices?page=1&pageSize=20",
    "query_params": {"page": "1", "pageSize": "20"},
    "remote_addr": "192.168.1.50:54321",
    "user_agent": "Mozilla/5.0 ...",
    "content_type": "application/json",
    "content_length": 245,
    "auth_type": "jwt",
    "user_id": "1",
    "username": "admin",
    "referer": "http://localhost:5173/devices",
    "x_forwarded_for": "",
    "request_body_preview": "{\"name\":\"Router-01\",\"ipAddress\":\"10.0.0.1\",\"protocol\":\"snmp\"}"
  }
}

// ── REQUEST END ──
{
  "timestamp": "2026-06-05T12:30:01.145678901Z",
  "level": "INFO",
  "component": "http",
  "request_id": "a1b2c3d4e5f6g7h8",
  "message": "← 201 POST /api/v1/devices",
  "duration_ms": 22.22,
  "data": {
    "event": "request_end",
    "status_code": 201,
    "status_text": "Created",
    "response_size_bytes": 384,
    "rate_limit_remaining": 997,
    "rate_limit_limit": 1000
  }
}
```

**What it captures:**
- Full request headers (sanitized — no `Authorization` value, only `Bearer ***`)
- Request body preview (first 1KB, redacted for `/auth/login` password fields)
- Response status code, size, and timing
- Rate limit state after processing
- Slow request warnings (logs at WARN if `duration > LOG_SLOW_REQUEST_MS`)
- 4xx/5xx errors logged at WARN/ERROR respectively with response body

---

##### Category 2: Database Query Logging

**File:** [db_logger.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/logging/db_logger.go)

Wraps every database call to log the query, parameters, duration, and result count.

```json
{
  "timestamp": "2026-06-05T12:30:01.130000000Z",
  "level": "DEBUG",
  "component": "db",
  "request_id": "a1b2c3d4e5f6g7h8",
  "trace_id": "tr-9f8e7d6c",
  "message": "SELECT completed",
  "duration_ms": 1.45,
  "data": {
    "event": "query",
    "operation": "SELECT",
    "table": "devices",
    "method": "GetDevices",
    "sql": "SELECT * FROM devices WHERE enabled = 1 ORDER BY id DESC",
    "params": [],
    "rows_returned": 12,
    "rows_affected": 0,
    "slow_query": false,
    "caller": "handlers/devices.go:47"
  }
}

// ── SLOW QUERY WARNING ──
{
  "level": "WARN",
  "component": "db",
    "message": "Slow query detected (245.3ms)",
  "duration_ms": 245.3,
  "data": {
    "event": "slow_query",
    "operation": "SELECT",
    "table": "metrics",
    "method": "GetMetricsForReport",
    "sql": "SELECT m.id, m.device_id ... WHERE m.timestamp BETWEEN $1 AND $2 LIMIT 5000",
    "params": ["2026-06-04 00:00:00", "2026-06-05 00:00:00"],
    "rows_returned": 5000,
    "threshold_ms": 100
  }
}

// ── DB ERROR ──
{
  "level": "ERROR",
  "component": "db",
  "message": "INSERT failed",
  "duration_ms": 5002.1,
  "error": "ERROR: duplicate key value violates unique constraint \"metrics_pkey\" (SQLSTATE 23505)",
  "data": {
    "event": "query_error",
    "operation": "INSERT",
    "table": "metrics",
    "method": "RecordMetric",
    "sql": "INSERT INTO metrics ...",
    "params": [42, "up", 23.5, null, "OK"],
    "retry_count": 3
  }
}
```

**What it captures:**
- Every SQL query executed (operation type, table, full SQL in DEBUG, parameterized in INFO)
- Query duration with automatic slow-query detection
- Row counts (returned for SELECT, affected for INSERT/UPDATE/DELETE)
- Database errors with retry information
- Transaction boundaries (BEGIN/COMMIT/ROLLBACK)
- Connection pool stats (periodically at INFO level)
- Versioned schema migrations and connection-pool stats at startup

---

##### Category 3: Collector Execution Logging

**File:** [collector_logger.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/logging/collector_logger.go)

Logs every single collector run — success or failure — with full context.

```json
// ── COLLECTOR START ──
{
  "level": "DEBUG",
  "component": "collector.ping",
  "trace_id": "tr-collect-42-1717587001",
  "message": "Starting collection: Google DNS (8.8.8.8)",
  "data": {
    "event": "collector_start",
    "device_id": 2,
    "device_name": "Google DNS",
    "host": "8.8.8.8",
    "protocol": "ping",
    "sensor_id": 2,
    "interval_seconds": 30,
    "attempt": 1
  }
}

// ── COLLECTOR RESULT ──
{
  "level": "INFO",
  "component": "collector.ping",
  "trace_id": "tr-collect-42-1717587001",
  "message": "✓ Google DNS (8.8.8.8) → up (12ms)",
  "duration_ms": 12.4,
  "data": {
    "event": "collector_result",
    "device_id": 2,
    "device_name": "Google DNS",
    "host": "8.8.8.8",
    "protocol": "ping",
    "status": "up",
    "previous_status": "up",
    "status_changed": false,
    "response_time_ms": 12.4,
    "value": 1.0,
    "message": "Reply from 8.8.8.8: time=12ms",
    "metric_id": 98432,
    "packet_loss_percent": 0.0
  }
}

// ── COLLECTOR FAILURE ──
{
  "level": "ERROR",
  "component": "collector.snmp",
  "trace_id": "tr-collect-5-1717587060",
  "message": "✗ Core Switch (10.0.0.1) → down",
  "duration_ms": 5012.0,
  "error": "Request timeout (no response received before the timeout period)",
  "data": {
    "event": "collector_error",
    "device_id": 5,
    "device_name": "Core Switch",
    "host": "10.0.0.1",
    "protocol": "snmp",
    "snmp_version": "2c",
    "snmp_community": "***",
    "status": "down",
    "previous_status": "up",
    "status_changed": true,
    "consecutive_failures": 3,
    "last_success_at": "2026-06-05T12:25:01Z"
  }
}

// ── SNMP DETAIL (TRACE level) ──
{
  "level": "TRACE",
  "component": "collector.snmp",
  "message": "SNMP GET response",
  "data": {
    "event": "snmp_response",
    "device_id": 5,
    "oids_requested": ["1.3.6.1.2.1.1.3.0", "1.3.6.1.2.1.1.5.0"],
    "varbinds": [
      {"oid": "1.3.6.1.2.1.1.3.0", "type": "TimeTicks", "value": 123456789},
      {"oid": "1.3.6.1.2.1.1.5.0", "type": "OctetString", "value": "core-switch-01"}
    ],
    "response_time_ms": 34.2
  }
}
```

**What it captures for each protocol:**

| Protocol | Extra Fields Logged |
|---|---|
| `ping` | packet_loss_percent, ttl, resolved_ip |
| `http` | http_status_code, tls_version, tls_expiry_days, redirect_count, final_url |
| `port` | target_port, connection_state, service_guess |
| `snmp` | snmp_version, oids_queried, varbind_count, community (masked), cpu/mem/disk values |
| `system` | cpu_percent, memory_percent, disk_percent, load_avg, goroutine_count |
| `netflow` | exporter_ip, template_id, flow_count, total_bytes, buffer_size |
| `pcap` | interface, bpf_filter, packets_captured, bytes_captured, session_id |

---

##### Category 4: Security & Audit Logging

**File:** [audit_logger.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/logging/audit_logger.go)

Records every security-relevant action for compliance and forensics. **Always written to DB regardless of log level.**

```json
// ── LOGIN SUCCESS ──
{
  "level": "INFO",
  "component": "audit",
  "message": "User login successful",
  "data": {
    "event": "auth.login_success",
    "username": "admin",
    "user_id": 1,
    "remote_addr": "192.168.1.50",
    "user_agent": "Mozilla/5.0 ...",
    "auth_method": "password",
    "token_type": "access+refresh",
    "token_expires_at": "2026-06-05T12:45:01Z"
  }
}

// ── LOGIN FAILURE ──
{
  "level": "WARN",
  "component": "audit",
  "message": "User login failed",
  "data": {
    "event": "auth.login_failure",
    "username": "admin",
    "remote_addr": "10.0.0.99",
    "reason": "invalid_password",
    "consecutive_failures": 3
  }
}

// ── API KEY USAGE ──
{
  "level": "INFO",
  "component": "audit",
  "message": "API key authenticated",
  "data": {
    "event": "auth.apikey_used",
    "key_name": "Default Integration Key",
    "key_id": 1,
    "remote_addr": "10.0.0.200",
    "endpoint": "GET /api/v1/devices"
  }
}

// ── RATE LIMIT HIT ──
{
  "level": "WARN",
  "component": "audit",
  "message": "Rate limit exceeded",
  "data": {
    "event": "security.rate_limit",
    "user_id": "1",
    "auth_type": "jwt",
    "limit": 1000,
    "window_seconds": 3600,
    "remote_addr": "192.168.1.50"
  }
}
```

**Full audit event catalog:**

| Event | Level | Trigger |
|---|---|---|
| `auth.login_success` | INFO | Successful login |
| `auth.login_failure` | WARN | Failed login attempt |
| `auth.logout` | INFO | User logout |
| `auth.token_refresh` | DEBUG | Token refreshed |
| `auth.token_revoked` | INFO | Token manually revoked |
| `auth.token_expired` | DEBUG | Token rejected as expired |
| `auth.apikey_used` | INFO | API key authenticated |
| `auth.apikey_invalid` | WARN | Invalid API key attempted |
| `security.rate_limit` | WARN | Rate limit exceeded |
| `security.invalid_token` | WARN | Malformed/invalid JWT |
| `security.unauthorized` | WARN | Unauthenticated access attempt |
| `config.device_created` | INFO | New device added |
| `config.device_updated` | INFO | Device configuration changed |
| `config.device_deleted` | WARN | Device removed |
| `config.sensor_created` | INFO | New sensor added |
| `config.sensor_deleted` | WARN | Sensor removed |
| `config.rule_created` | INFO | Alert rule created |
| `config.rule_updated` | INFO | Alert rule modified |
| `config.rule_deleted` | WARN | Alert rule deleted |
| `config.channel_created` | INFO | Notification channel added |
| `config.dashboard_modified` | INFO | Dashboard layout changed |
| `capture.started` | INFO | Packet capture started |
| `capture.stopped` | INFO | Packet capture stopped |
| `data.retention_purge` | INFO | Data retention pruning executed |

---

##### Category 5: Alert Engine Decision Logging

**File:** [alert_logger.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/logging/alert_logger.go)

Logs the **entire decision trail** of the alert engine — why a rule fired, why it didn't, escalation decisions, and notification delivery.

```json
// ── RULE EVALUATION ──
{
  "level": "DEBUG",
  "component": "alert_engine",
  "trace_id": "tr-alert-eval-42",
  "message": "Evaluating rule 'High Latency' for device 2",
  "data": {
    "event": "rule_evaluation",
    "rule_id": 2,
    "rule_name": "High Latency",
    "device_id": 2,
    "device_name": "Google DNS",
    "conditions_checked": 1,
    "conditions_met": 0,
    "condition_results": [
      {
        "condition_id": 3,
        "type": "threshold",
        "field": "response_time",
        "operator": "gt",
        "threshold": 500,
        "actual_value": 12.4,
        "result": false,
        "sustained_seconds": 0,
        "required_duration_seconds": 300
      }
    ],
    "verdict": "no_fire",
    "rule_state": "idle"
  }
}

// ── ALERT FIRED ──
{
  "level": "WARN",
  "component": "alert_engine",
  "message": "🔔 Alert FIRED: Device Down — Core Switch",
  "data": {
    "event": "alert_fired",
    "alert_id": 891,
    "rule_id": 1,
    "rule_name": "Device Down",
    "severity": "critical",
    "device_id": 5,
    "device_name": "Core Switch",
    "trigger_reason": "status == 'down' sustained for 65 seconds (threshold: 60s)",
    "condition_values": {"status": "down", "sustained_seconds": 65},
    "previous_fire_at": null,
    "cooldown_remaining_seconds": 0
  }
}

// ── NOTIFICATION SENT ──
{
  "level": "INFO",
  "component": "alert_engine.notifier",
  "message": "Notification delivered: webhook → ops-alerts",
  "duration_ms": 234.5,
  "data": {
    "event": "notification_sent",
    "alert_id": 891,
    "channel_id": 1,
    "channel_type": "webhook",
    "channel_name": "ops-alerts",
    "delivery_status": "success",
    "http_status": 200,
    "retry_count": 0
  }
}

// ── NOTIFICATION FAILURE ──
{
  "level": "ERROR",
  "component": "alert_engine.notifier",
  "message": "Notification delivery FAILED: email → admin@rayavriti.com",
  "duration_ms": 5002.1,
  "error": "dial tcp: connection refused",
  "data": {
    "event": "notification_failed",
    "alert_id": 891,
    "channel_id": 2,
    "channel_type": "email",
    "channel_name": "admin-email",
    "delivery_status": "failed",
    "retry_count": 3,
    "max_retries": 3,
    "will_retry": false
  }
}

// ── AUTO-RESOLVE ──
{
  "level": "INFO",
  "component": "alert_engine",
  "message": "✓ Alert auto-resolved: Device Down — Core Switch",
  "data": {
    "event": "alert_auto_resolved",
    "alert_id": 891,
    "rule_id": 1,
    "device_id": 5,
    "device_name": "Core Switch",
    "duration_active_seconds": 340,
    "resolved_reason": "all_conditions_cleared"
  }
}
```

---

##### Category 6: WebSocket Event Logging

**File:** [ws_logger.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/logging/ws_logger.go)

```json
// ── CLIENT CONNECT ──
{
  "level": "INFO",
  "component": "websocket",
  "message": "Client connected",
  "data": {
    "event": "ws.connect",
    "socket_id": "ws-a1b2c3",
    "user_id": 1,
    "username": "admin",
    "remote_addr": "192.168.1.50:54321",
    "transport": "websocket",
    "total_connections": 3
  }
}

// ── EVENT BROADCAST ──
{
  "level": "DEBUG",
  "component": "websocket",
  "message": "Broadcasting metric:update to 3 clients",
  "data": {
    "event": "ws.broadcast",
    "event_name": "metric:update",
    "payload_bytes": 412,
    "recipient_count": 3,
    "device_id": 2
  }
}
```

---

##### Category 7: Scheduler & Lifecycle Logging

Logged directly by the scheduler, retention, and application lifecycle components:

```json
// ── SCHEDULER ──
{"component": "scheduler", "message": "Scheduler started: 12 devices, 12 jobs registered"}
{"component": "scheduler", "message": "Job registered: device=2 (Google DNS), interval=30s, next_run=2026-06-05T12:30:30Z"}
{"component": "scheduler", "message": "Scheduler reloaded: 2 jobs added, 1 job removed"}

// ── RETENTION ──
{"component": "retention", "message": "Retention sweep completed", "data": {"metrics_deleted": 4521, "flows_deleted": 12003, "alerts_deleted": 34, "captures_deleted": 2, "duration_ms": 890.2}}

// ── LIFECYCLE ──
{"component": "app", "message": "Rayavriti NetMonitor v1.0.0 starting", "data": {"go_version": "go1.24.2", "os": "linux", "arch": "amd64", "pid": 1234, "environment": "production", "database_host": "timescaledb", "database_name": "netmonitor", "port": 3000}}
{"component": "app", "message": "Shutdown signal received (SIGTERM), draining connections...", "data": {"active_connections": 3, "pending_requests": 1, "active_captures": 0}}
{"component": "app", "message": "Graceful shutdown complete", "data": {"uptime_seconds": 86423, "total_requests_served": 45231}}
```

---

#### Monitoring Database Tables

These tables store **operational telemetry** about the application itself — not the monitored network. They power the `/api/v1/system/monitoring` endpoints and self-diagnostics.

```sql
-- ── 1. HTTP Request Log ──────────────────────────────────────────
-- Every API request with timing, status, and user context
CREATE TABLE IF NOT EXISTS monitoring_http_requests (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    trace_id TEXT,
    method TEXT NOT NULL,               -- GET, POST, PUT, DELETE
    path TEXT NOT NULL,                  -- /api/v1/devices
    query_string TEXT,                   -- page=1&pageSize=20
    status_code INTEGER NOT NULL,        -- 200, 404, 500, etc.
    duration_ms REAL NOT NULL,           -- Response time in ms
    request_size_bytes INTEGER,          -- Request body size
    response_size_bytes INTEGER,         -- Response body size
    remote_addr TEXT,                    -- Client IP
    user_id BIGINT,                      -- Authenticated user (null = anonymous)
    auth_type TEXT,                      -- 'jwt', 'api_key', null
    user_agent TEXT,                     -- Browser/client identifier
    error_code TEXT,                     -- API error code if 4xx/5xx
    error_message TEXT,                  -- Error message if 4xx/5xx
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_mon_http_time ON monitoring_http_requests(created_at);
CREATE INDEX IF NOT EXISTS idx_mon_http_path ON monitoring_http_requests(path, method);
CREATE INDEX IF NOT EXISTS idx_mon_http_status ON monitoring_http_requests(status_code);
CREATE INDEX IF NOT EXISTS idx_mon_http_user ON monitoring_http_requests(user_id);

-- ── 2. Database Query Performance Log ───────────────────────────
-- Every query (or sampled in high-traffic mode) with execution time
CREATE TABLE IF NOT EXISTS monitoring_db_queries (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT,                     -- Links to HTTP request (null for bg jobs)
    trace_id TEXT,
    operation TEXT NOT NULL,             -- SELECT, INSERT, UPDATE, DELETE
    table_name TEXT NOT NULL,            -- Primary table involved
    method_name TEXT NOT NULL,           -- Go method name (e.g., 'GetDevices')
    duration_ms REAL NOT NULL,
    rows_returned INTEGER DEFAULT 0,
    rows_affected INTEGER DEFAULT 0,
    is_slow BOOLEAN DEFAULT false,
    is_error BOOLEAN DEFAULT false,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_mon_dbq_time ON monitoring_db_queries(created_at);
CREATE INDEX IF NOT EXISTS idx_mon_dbq_slow ON monitoring_db_queries(is_slow, created_at);
CREATE INDEX IF NOT EXISTS idx_mon_dbq_method ON monitoring_db_queries(method_name);

-- ── 3. Collector Execution Log ──────────────────────────────────
-- Every collector run with result, timing, and status transitions
CREATE TABLE IF NOT EXISTS monitoring_collector_runs (
    id BIGSERIAL PRIMARY KEY,
    trace_id TEXT,
    device_id BIGINT NOT NULL,
    device_name TEXT NOT NULL,
    host TEXT NOT NULL,
    protocol TEXT NOT NULL,              -- ping, http, port, snmp, system
    sensor_id BIGINT,
    status TEXT NOT NULL,                -- up, down, degraded, warning, error
    previous_status TEXT,                -- Status before this run
    status_changed BOOLEAN DEFAULT false,
    response_time_ms REAL,
    value REAL,
    message TEXT,                        -- Collector-specific message
    error_message TEXT,                  -- Error if collection failed
    duration_ms REAL NOT NULL,           -- Total collection time
    metric_id BIGINT,                    -- Resulting metric row ID
    alert_id BIGINT,                     -- Alert created (if any)
    consecutive_failures INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (device_id) REFERENCES devices(id)
);
CREATE INDEX IF NOT EXISTS idx_mon_coll_time ON monitoring_collector_runs(created_at);
CREATE INDEX IF NOT EXISTS idx_mon_coll_device ON monitoring_collector_runs(device_id, created_at);
CREATE INDEX IF NOT EXISTS idx_mon_coll_status ON monitoring_collector_runs(status_changed, created_at);

-- ── 4. Security Audit Log ───────────────────────────────────────
-- Every security-relevant event (auth, config changes, etc.)
CREATE TABLE IF NOT EXISTS monitoring_audit_log (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT,
    event_type TEXT NOT NULL,            -- e.g., 'auth.login_success', 'config.device_created'
    severity TEXT NOT NULL DEFAULT 'info', -- info, warn, error
    actor TEXT,                          -- 'user:admin', 'system', 'api_key:Default Integration Key'
    actor_ip TEXT,                       -- Remote IP address
    resource_type TEXT,                  -- 'device', 'alert_rule', 'user', 'session'
    resource_id TEXT,                    -- ID of affected resource
    description TEXT NOT NULL,           -- Human-readable description
    details JSONB,                   -- Full event context as JSON
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_mon_audit_time ON monitoring_audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_mon_audit_event ON monitoring_audit_log(event_type);
CREATE INDEX IF NOT EXISTS idx_mon_audit_actor ON monitoring_audit_log(actor);
CREATE INDEX IF NOT EXISTS idx_mon_audit_resource ON monitoring_audit_log(resource_type, resource_id);

-- ── 5. Application Health Snapshots ─────────────────────────────
-- Periodic self-monitoring snapshots (every 60 seconds)
CREATE TABLE IF NOT EXISTS monitoring_app_health (
    id BIGSERIAL PRIMARY KEY,
    uptime_seconds INTEGER NOT NULL,
    goroutine_count INTEGER NOT NULL,
    heap_alloc_bytes INTEGER NOT NULL,    -- Current heap memory in use
    heap_sys_bytes INTEGER NOT NULL,      -- Total heap memory from OS
    stack_in_use_bytes INTEGER NOT NULL,  -- Stack memory in use
    gc_pause_total_ns INTEGER NOT NULL,   -- Total GC pause time
    gc_runs INTEGER NOT NULL,             -- Total GC cycles
    gc_last_pause_ns INTEGER,             -- Last GC pause duration
    num_cpu INTEGER NOT NULL,
    active_ws_connections INTEGER NOT NULL,
    active_capture_sessions INTEGER NOT NULL,
    scheduler_jobs_active INTEGER NOT NULL,
    db_open_connections INTEGER,
    db_idle_connections INTEGER,
    db_wait_count INTEGER,
    db_wait_duration_ms REAL,
    requests_total INTEGER NOT NULL,      -- Lifetime HTTP requests served
    requests_active INTEGER NOT NULL,     -- Currently in-flight requests
    errors_total INTEGER NOT NULL,        -- Lifetime 5xx errors
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_mon_health_time ON monitoring_app_health(created_at);

-- ── 6. Alert Engine Activity Log ────────────────────────────────
-- Every rule evaluation, fire, notification attempt, and resolution
CREATE TABLE IF NOT EXISTS monitoring_alert_activity (
    id BIGSERIAL PRIMARY KEY,
    trace_id TEXT,
    rule_id BIGINT,
    rule_name TEXT,
    device_id BIGINT,
    device_name TEXT,
    action TEXT NOT NULL,                -- 'evaluated', 'condition_met', 'condition_cleared',
                                         -- 'pending', 'fired', 'notified', 'notification_failed',
                                         -- 'acknowledged', 'resolved', 'auto_resolved', 'cooldown_skip'
    severity TEXT,
    alert_id BIGINT,                     -- Resulting alert ID (if fired)
    channel_id BIGINT,                   -- Notification channel (if notified)
    channel_type TEXT,                   -- 'webhook', 'email', 'slack', 'in_app'
    details JSONB,                   -- Full evaluation context as JSON
    duration_ms REAL,                    -- Evaluation/notification time
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_mon_alert_act_time ON monitoring_alert_activity(created_at);
CREATE INDEX IF NOT EXISTS idx_mon_alert_act_rule ON monitoring_alert_activity(rule_id);
CREATE INDEX IF NOT EXISTS idx_mon_alert_act_device ON monitoring_alert_activity(device_id);
CREATE INDEX IF NOT EXISTS idx_mon_alert_act_action ON monitoring_alert_activity(action);
```

#### Monitoring Data Retention

Monitoring tables have their own retention policy, separate from network monitoring data:

| Table | Default Retention | Env Variable |
|---|---|---|
| `monitoring_http_requests` | 7 days | `MON_HTTP_RETENTION_DAYS` |
| `monitoring_db_queries` | 3 days | `MON_DB_RETENTION_DAYS` |
| `monitoring_collector_runs` | 14 days | `MON_COLLECTOR_RETENTION_DAYS` |
| `monitoring_audit_log` | 365 days | `MON_AUDIT_RETENTION_DAYS` |
| `monitoring_app_health` | 30 days | `MON_HEALTH_RETENTION_DAYS` |
| `monitoring_alert_activity` | 30 days | `MON_ALERT_RETENTION_DAYS` |

> [!TIP]
> Audit logs are kept for 1 year by default — they serve as the compliance and forensics trail.

#### [NEW] [self_monitor.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/monitoring/self_monitor.go)

Background goroutine that snapshots application health every 60 seconds:

```go
func (m *SelfMonitor) collectSnapshot() AppHealthSnapshot {
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)

    return AppHealthSnapshot{
        UptimeSeconds:          int(time.Since(m.startTime).Seconds()),
        GoroutineCount:         runtime.NumGoroutine(),
        HeapAllocBytes:         int64(memStats.HeapAlloc),
        HeapSysBytes:           int64(memStats.HeapSys),
        StackInUseBytes:        int64(memStats.StackInuse),
        GCPauseTotalNs:         int64(memStats.PauseTotalNs),
        GCRuns:                 int(memStats.NumGC),
        GCLastPauseNs:          int64(memStats.PauseNs[(memStats.NumGC+255)%256]),
        NumCPU:                 runtime.NumCPU(),
        ActiveWSConnections:    m.hub.ConnectionCount(),
        ActiveCaptureSessions:  m.captureManager.ActiveCount(),
        SchedulerJobsActive:    m.scheduler.JobCount(),
        DBOpenConnections:      m.dbpool.Stat().TotalConns(),
        DBIdleConnections:      m.dbpool.Stat().IdleConns(),
        DBWaitCount:            m.dbpool.Stat().EmptyAcquireCount(),
        DBWaitDurationMs:       float64(m.dbpool.Stat().AcquireDuration().Milliseconds()),
        RequestsTotal:          atomic.LoadInt64(&m.requestsTotal),
        RequestsActive:         atomic.LoadInt64(&m.requestsActive),
        ErrorsTotal:            atomic.LoadInt64(&m.errorsTotal),
    }
}
```

#### New System Monitoring API Endpoints

```
# System Logs & Monitoring
GET  /api/v1/system/logs                 — Query structured logs from DB tables
     ?component=http|db|collector|audit|alert_engine|websocket|scheduler
     ?level=trace|debug|info|warn|error
     ?from=ISO8601&to=ISO8601
     ?device_id=5
     ?request_id=a1b2c3d4
     ?limit=100&offset=0

GET  /api/v1/system/logs/stats           — Log volume statistics (counts by level, component, hour)

GET  /api/v1/system/monitoring           — Current application health snapshot
                                            (goroutines, memory, GC, connections, DB size)

GET  /api/v1/system/monitoring/history   — Historical health snapshots
     ?hours=24

GET  /api/v1/system/monitoring/requests  — HTTP request performance analytics
     ?from=ISO8601&to=ISO8601
     ?path=/api/v1/devices
     ?min_duration_ms=100
     ?status_code=500

GET  /api/v1/system/monitoring/queries   — Slow query analysis
     ?slow_only=true
     ?method=GetMetricsForReport

GET  /api/v1/system/audit-log            — Security audit trail
     ?event_type=auth.login_failure
     ?actor=user:admin
     ?from=ISO8601&to=ISO8601

GET  /api/v1/system/collectors/stats     — Collector success/failure rates per device
     ?device_id=5
     ?hours=24
```

#### Log File Rotation

**File:** [rotation.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/logging/rotation.go)

Uses `lumberjack.v2` for automatic log file rotation:

```go
&lumberjack.Logger{
    Filename:   cfg.LogFilePath,       // ./data/logs/netmonitor.log
    MaxSize:    cfg.LogFileMaxSizeMB,   // 100 MB per file
    MaxBackups: cfg.LogFileMaxBackups,  // Keep 10 rotated files
    MaxAge:     cfg.LogFileMaxAgeDays,  // Delete after 30 days
    Compress:   cfg.LogFileCompress,    // Gzip old files
    LocalTime:  true,
}
```

Rotated files: `netmonitor.log`, `netmonitor-2026-06-05T12-00-00.log.gz`, `netmonitor-2026-06-04T12-00-00.log.gz`, ...

---

### Phase 2: Database Layer (PostgreSQL + TimescaleDB)

Replace the SQLite runtime database with PostgreSQL and TimescaleDB while preserving the existing API-facing data model, seed data, and query behavior. Time-series-heavy tables move to Timescale hypertables for retention, compression, and efficient bucketed queries.

#### [NEW] [database.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/database/database.go)
Go interface definitions mirroring [types.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/db/types.ts):

```go
type Database interface {
    DeviceRepo
    SensorRepo
    MetricRepo
    AlertRepo
    FlowRepo
    PortScanRepo
    CaptureRepo
    DashboardRepo
    Raw(sql string, params ...any) error
    GetStats() Stats
    GetUserByUsername(username string) *User
    VerifyAPIKey(rawKey string) *APIKey
    Close() error
}
```

#### [NEW] [postgres.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/database/postgres.go)
PostgreSQL implementation using `pgx/v5` and `pgxpool`:
- DSN-based connection configuration via `DATABASE_URL`
- Pool tuning via `DB_MAX_CONNS`, `DB_MIN_CONNS`, `DB_MAX_CONN_LIFETIME`, `DB_HEALTH_CHECK_PERIOD`
- Context-aware query execution with request deadlines and cancellation
- Explicit transactions for multi-write operations and seed/migration steps
- Query methods ported from [sqlite.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/db/sqlite.ts) with PostgreSQL syntax (`$1` placeholders, `RETURNING`, `ON CONFLICT`, typed timestamps)
- JSON/JSONB columns where structured payloads are currently stringified
- `Raw(ctx, sql string, params ...any)` retained for compatibility, but guarded for admin/internal use only
- Database logging wrapper records query text, sanitized args, duration, row count, and errors

#### [NEW] [migrations.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/database/migrations.go)
Versioned PostgreSQL migrations replacing ad hoc `ensureColumn` schema changes:
- Create extension: `CREATE EXTENSION IF NOT EXISTS timescaledb;`
- Create schema version table: `schema_migrations(version bigint primary key, applied_at timestamptz not null default now())`
- Create tables: users, devices, sensors, metrics, alerts, dashboards, api_keys, flow_records, capture_sessions, port_scan_results
- Add alert engine tables: `alert_rules`, `alert_rule_conditions`, `notification_channels`, `alert_history`
- Add monitoring/logging tables used by Phase 1B
- Preserve current API field names while using PostgreSQL-native types (`bigserial`, `boolean`, `timestamptz`, `inet`, `jsonb`)
- Use foreign keys and cascading deletes where the existing model implies ownership
- Add indexes for device, sensor, timestamp, alert status, flow dimensions, and dashboard ownership

#### TimescaleDB Hypertables
Convert high-volume time-series tables after creation:

```sql
SELECT create_hypertable('metrics', 'timestamp', if_not_exists => TRUE);
SELECT create_hypertable('flow_records', 'timestamp', if_not_exists => TRUE);
SELECT create_hypertable('alert_history', 'created_at', if_not_exists => TRUE);
SELECT create_hypertable('monitoring_http_requests', 'created_at', if_not_exists => TRUE);
SELECT create_hypertable('monitoring_db_queries', 'created_at', if_not_exists => TRUE);
SELECT create_hypertable('monitoring_collector_runs', 'created_at', if_not_exists => TRUE);
SELECT create_hypertable('monitoring_alert_activity', 'created_at', if_not_exists => TRUE);
```

Timescale policies:
- Retention policy configurable per table (`METRICS_RETENTION_DAYS`, `FLOWS_RETENTION_DAYS`, `LOG_RETENTION_DAYS`)
- Compression enabled for older metrics/flows/log chunks
- Continuous aggregates for report queries that currently bucket by minute/hour/day
- Backfill-safe migration order: create tables, load historical data, then enable compression policies
- Hypertable unique indexes must include the time partition column. For high-volume time-series tables, prefer `(id, timestamp)`/`(id, created_at)` composite primary keys or avoid global uniqueness on surrogate IDs.

#### [NEW] [sqlite_import.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/database/sqlite_import.go)
One-time migration helper for existing installations:
- Opens the legacy SQLite `.db` file read-only
- Streams rows into PostgreSQL using `pgx.CopyFrom` where possible
- Preserves primary keys to keep API references stable
- Converts SQLite integer/string timestamps to `timestamptz`
- Converts serialized JSON strings into `jsonb`
- Rebuilds sequences after import with `setval(...)`
- Runs validation counts and checksum-style sanity checks per table
- Leaves the source `.db` untouched

CLI command:

```bash
netmonitor migrate-sqlite \
  --sqlite ./data/netmonitor.db \
  --database-url postgres://netmonitor:netmonitor@localhost:5432/netmonitor
```

#### Schema Compatibility Requirements
- Keep response JSON shapes identical to the current frontend contract
- Keep existing table concepts and seed records, but do not preserve SQLite-specific storage details
- Maintain legacy numeric IDs unless a later phase explicitly introduces UUIDs
- Store timestamps in UTC `timestamptz`; serialize API responses in the same ISO format currently expected by the frontend
- Implement pagination with stable ordering to avoid duplicate/missing rows under concurrent inserts
- Use PostgreSQL full-text or trigram indexes only where existing search/filter behavior requires it

#### [NEW] [models.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/models/models.go)
Go struct definitions for all entities with JSON tags matching the current API response shapes:
- `Device`, `Sensor`, `Metric`, `Alert`, `Dashboard`
- `FlowRecord`, `CaptureSession`, `PortScanResult`
- `User`, `APIKey`, `Stats`
- `InsightsResponse`, `DeviceHealth`, `HealthFactor`

---

### Phase 3: Authentication & Middleware

Port the JWT auth system, session management, password hashing, and all HTTP middleware.

#### [NEW] [jwt.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/auth/jwt.go)
Port of [auth.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/auth.ts):
- HS256 JWT signing with configurable `JWT_SECRET`
- 15-minute access tokens, 7-day refresh tokens
- `login()`, `logout()`, `refresh()`, `getSession()`, `extractToken()`
- In-memory session store with token → user mapping

#### [NEW] [password.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/auth/password.go)
Port of [password.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/password.ts):
- scrypt with 32-byte random salt (N=16384, r=8, p=1, keyLen=64)
- Format: `scrypt:base64(salt):base64(hash)`
- Backward-compatible SHA-256 verification for legacy hashes

#### [NEW] [middleware.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/server/middleware.go)
- **Security headers**: CSP, X-Frame-Options, HSTS, X-Content-Type-Options (equivalent of Helmet)
- **CORS**: Restricted in production, `*` in development
- **Request body limit**: 1MB
- **Request ID**: Random 16-hex-char per request
- **Rate limiting**: Per-user/API-key sliding window (1000 req/hr for JWT, 5000 for API keys)
- **Structured logging**: `slog` JSON format in production, text in development

---

### Phase 4: REST API Handlers

Port all 60+ REST endpoints preserving exact request/response shapes.

#### Legacy API (`/api/*`) — 18 endpoints
Exact port of [index.ts L382-L681](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/index.ts#L382-L681):
- Auth: `POST /api/auth/login`, `GET /api/auth/me`, `POST /api/auth/logout`
- Devices: `GET/POST/PUT/DELETE /api/devices[/:id]`
- Metrics: `GET /api/metrics/latest`, `GET /api/metrics/device/:id`
- Ports: `GET /api/devices/:id/ports`, `POST /api/devices/:id/scan-ports`
- Alerts: `GET /api/alerts`, `POST /api/alerts/:id/acknowledge`, `POST /api/alerts/:id/resolve`, `GET /api/alerts/counts`
- Stats: `GET /api/stats`
- Insights: `GET /api/insights`, `GET /api/insights/history`
- Reports: `GET /api/reports/summary`, `GET /api/reports/timeseries`, `GET /api/reports/devices`, `GET /api/reports/alerts`, `GET /api/reports/metrics.csv`
- Simulator: `POST /api/simulator/metrics`, `POST /api/simulator/flows`

#### V1 API (`/api/v1/*`) — 40+ endpoints
Exact port of [index.ts L683-L1337](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/index.ts#L683-L1337):
- Auth: `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`, `POST /api/v1/auth/2fa/verify`, `POST /api/v1/auth/logout`
- Devices: Full CRUD with pagination, sorting, filtering
- Sensors: Full CRUD
- Metrics: `GET /api/v1/metrics/query` with aggregation + bucketing
- Alerts: Full CRUD + acknowledge/resolve with pagination
- Dashboards: Full CRUD
- Reports: `GET /api/v1/reports`
- Flows: `GET /api/v1/flows`, `/flows/top-talkers`, `/flows/protocols`, `/flows/timeseries`, `/flows/stats`
- Capture: `GET /api/v1/capture/interfaces`, `POST /api/v1/capture/start`, `POST /api/v1/capture/:id/stop`, `GET /api/v1/capture/:id`, `GET /api/v1/capture/:id/packets`, `GET /api/v1/capture/sessions`

#### [NEW] [response.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/server/response.go)
Standard JSON envelope matching the current API shape:
```go
type APIResponse struct {
    Success bool        `json:"success"`
    Data    any         `json:"data"`
    Error   *APIError   `json:"error"`
    Meta    ResponseMeta `json:"meta"`
}
```

---

### Phase 5: Collectors & Schedulers

Port all 8 collector types and the scheduling infrastructure.

#### [NEW] [ping.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/collectors/ping.go)
Port of [ping.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/collectors/ping.ts):
- ICMP ping using `pro-bing` library
- Returns status (up/down), response time, packet loss message

#### [NEW] [http.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/collectors/http.go)
Port of [http.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/collectors/http.ts):
- HTTP/HTTPS GET with configurable timeout
- Returns status code, response time, TLS info

#### [NEW] [port.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/collectors/port.go)
Port of [port.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/collectors/port.ts):
- TCP connection check with timeout
- Returns open/closed status and response time

#### [NEW] [snmp.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/collectors/snmp.go)
Port of [snmp.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/collectors/snmp.ts):
- SNMP v1/v2c/v3 polling using `gosnmp`
- System info OIDs (sysDescr, sysUpTime, sysName)
- Interface traffic counters (ifInOctets, ifOutOctets)
- CPU, memory, disk via HOST-RESOURCES-MIB
- Returns structured metrics with interface data in message JSON

#### [NEW] [system.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/collectors/system.go)
Port of [system.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/collectors/system.ts):
- Local system CPU, memory, disk, uptime using `gopsutil`
- Returns structured system info in message JSON

#### [NEW] [netflow.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/collectors/netflow.go)
Port of [netflow.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/collectors/netflow.ts):
- UDP listener on configurable port (default 2055)
- NetFlow v5/v9 and sFlow packet parsing
- Protocol number → name mapping
- Batch insertion into `flow_records` table
- Real-time `flow:update` WebSocket event

#### [NEW] [packet_capture.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/collectors/packet_capture.go)
Port of [packetCapture.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/collectors/packetCapture.ts):
- `google/gopacket` + `pcap` layer
- `listInterfaces()`, `startCapture()`, `stopCapture()`
- Per-session packet ring buffer (500 packets max in memory)
- BPF filter support
- Real-time `capture:packet` WebSocket events
- Session stats tracking

#### [NEW] [port_scanner.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/scanner/port_scanner.go)
Port of [portScanner.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/collectors/portScanner.ts):
- Concurrent TCP port scanning with configurable concurrency (default 16)
- Well-known service name guessing (port → service map)
- Change detection (new open/closed ports generate alerts)
- Go's goroutine pool pattern replaces Node.js's p-limit-like concurrency

#### [NEW] [scheduler.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/scheduler/scheduler.go)
Port of [scheduler.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/scheduler.ts):
- Per-device polling timers based on `interval_seconds`
- On each tick: run collector → record metric → evaluate alerts → emit WebSocket event
- Graceful stop on shutdown
- Re-schedule on device CRUD changes

#### [NEW] [retention.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/retention/retention.go)
Port of [retentionScheduler.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/retentionScheduler.ts):
- Runs every 6 hours
- Prunes metrics older than `METRICS_RETENTION_DAYS` (default 30)
- Prunes flow records older than `FLOW_RETENTION_DAYS` (default 7)
- Prunes resolved alerts older than `ALERTS_RETENTION_DAYS` (default 90)

---

### Phase 6: WebSocket & Real-Time Events

#### [NEW] [hub.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/websocket/hub.go)
Replace the Socket.IO server logic from [index.ts L1339-L1361](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/index.ts#L1339-L1361) with a native WebSocket protocol:
- HTTP upgrade endpoint: `GET /api/v1/ws`
- Auth middleware: validate JWT from `Authorization: Bearer ...`, `Sec-WebSocket-Protocol`, or a short-lived `ws_token` query parameter issued by the REST API
- Initial `bootstrap` message on connect (stats, latestMetrics, activeAlerts, user)
- Typed JSON envelope for all events:
  ```json
  { "type": "metric:update", "request_id": "optional", "data": {} }
  ```
- Events emitted by backend:
  - `metric:update` — new metric recorded
  - `alert:triggered` — new alert created
  - `flow:update` — new flow records ingested
  - `capture:packet` — real-time packet capture data
  - `ports:scanned` — port scan completed
  - `device:status` — device status changed
- Frontend migration required: replace the Socket.IO client with the native browser `WebSocket` API or a small local wrapper that handles reconnect, heartbeats, and typed event dispatch.

---

### Phase 7: Alert Engine — Complete Redesign 🔔

The current alert engine ([alertEngine.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/alertEngine.ts)) is minimal — it only checks for `down`, `degraded`, or high latency (>500ms) and creates alerts with basic de-duplication. The new Go alert engine is a **rule-based, configurable system** with proper lifecycle management and notification channels.

#### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Alert Engine                          │
│                                                         │
│  ┌──────────┐    ┌──────────┐    ┌──────────────────┐  │
│  │  Rules    │    │Evaluator │    │  State Machine   │  │
│  │  Store    │───▶│ Pipeline │───▶│  (per alert)     │  │
│  │(DB-backed)│    │          │    │  idle → firing   │  │
│  └──────────┘    └──────────┘    │  → notified      │  │
│                                   │  → acknowledged  │  │
│                                   │  → resolved      │  │
│                                   └──────────────────┘  │
│                       │                    │             │
│                       ▼                    ▼             │
│              ┌──────────────┐    ┌──────────────────┐  │
│              │  Condition   │    │   Notification    │  │
│              │  Matchers    │    │   Dispatcher      │  │
│              │              │    │                   │  │
│              │ • threshold  │    │ • webhook (POST)  │  │
│              │ • absence    │    │ • email (SMTP)    │  │
│              │ • anomaly    │    │ • slack           │  │
│              │ • state      │    │ • in-app (WS)     │  │
│              │ • compound   │    │                   │  │
│              └──────────────┘    └──────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

#### New Database Tables

```sql
-- Rule definitions (user-configurable)
CREATE TABLE IF NOT EXISTS alert_rules (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT true,
    severity TEXT NOT NULL DEFAULT 'warning',   -- 'info', 'warning', 'critical'
    scope_type TEXT NOT NULL DEFAULT 'global',  -- 'global', 'device', 'device_group'
    scope_value TEXT,                            -- device ID or group name
    condition_logic TEXT DEFAULT 'all',          -- 'all' (AND) or 'any' (OR)
    cooldown_seconds INTEGER DEFAULT 300,        -- Min time between re-fires
    auto_resolve BOOLEAN DEFAULT true,              -- Auto-resolve when conditions clear
    created_by BIGINT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Individual conditions within a rule
CREATE TABLE IF NOT EXISTS alert_rule_conditions (
    id BIGSERIAL PRIMARY KEY,
    rule_id BIGINT NOT NULL,
    type TEXT NOT NULL,       -- 'threshold', 'absence', 'state_change', 'anomaly', 'compound'
    metric_field TEXT,         -- 'response_time', 'status', 'value', 'packet_loss'
    operator TEXT,             -- 'gt', 'lt', 'gte', 'lte', 'eq', 'neq', 'contains'
    value TEXT,                -- Threshold value (string for flexibility)
    duration_seconds INTEGER,  -- Sustained duration before firing
    config JSONB,          -- Extra config (e.g., anomaly sensitivity, absence window)
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

-- Notification channel configuration
CREATE TABLE IF NOT EXISTS notification_channels (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,        -- 'webhook', 'email', 'slack', 'in_app'
    config JSONB NOT NULL,  -- {"url": "...", "headers": {...}} or {"smtp_host": "..."}
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Many-to-many: which channels a rule notifies
CREATE TABLE IF NOT EXISTS alert_rule_channels (
    rule_id BIGINT NOT NULL,
    channel_id BIGINT NOT NULL,
    PRIMARY KEY (rule_id, channel_id),
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (channel_id) REFERENCES notification_channels(id) ON DELETE CASCADE
);

-- Audit log for alert state transitions
CREATE TABLE IF NOT EXISTS alert_history (
    id BIGSERIAL PRIMARY KEY,
    alert_id BIGINT NOT NULL,
    rule_id BIGINT,
    action TEXT NOT NULL,       -- 'fired', 'notified', 'acknowledged', 'resolved', 'auto_resolved', 'escalated'
    actor TEXT,                 -- 'system', 'user:admin', 'rule:5'
    details JSONB,              -- JSON with context
    created_at TIMESTAMPTZ DEFAULT now(),
    FOREIGN KEY (alert_id) REFERENCES alerts(id)
);

-- Durable per-rule/per-device state. This prevents restarts from losing
-- sustained-duration timers, cooldowns, and current firing state.
CREATE TABLE IF NOT EXISTS alert_rule_state (
    rule_id BIGINT NOT NULL,
    device_id BIGINT NOT NULL,
    state TEXT NOT NULL DEFAULT 'idle',    -- idle, pending, firing, notified, acknowledged, resolved
    first_met_at TIMESTAMPTZ,
    last_evaluated_at TIMESTAMPTZ,
    last_fired_at TIMESTAMPTZ,
    last_resolved_at TIMESTAMPTZ,
    active_alert_id BIGINT,
    condition_snapshot JSONB,
    PRIMARY KEY (rule_id, device_id),
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
    FOREIGN KEY (active_alert_id) REFERENCES alerts(id) ON DELETE SET NULL
);
```

#### Alert Rule Condition Types

| Type | Description | Example |
|---|---|---|
| `threshold` | Fires when a metric value crosses a numeric threshold | `response_time > 500ms for 5 minutes` |
| `state_change` | Fires on device status transitions | `status changed to 'down'` |
| `absence` | Fires when no data is received within a window | `No metrics for 10 minutes` |
| `anomaly` | Fires when metric deviates from baseline (std dev) | `response_time > 2.5σ from 24h baseline` |
| `compound` | Combines multiple conditions with AND/OR logic | `CPU > 90% AND memory > 85%` |

#### Alert Lifecycle State Machine

```mermaid
stateDiagram-v2
    [*] --> Idle: Rule created
    Idle --> Pending: Condition met
    Pending --> Idle: Condition clears before duration
    Pending --> Firing: Condition sustained for duration_seconds
    Firing --> Notified: Notification dispatched
    Notified --> Acknowledged: User acknowledges
    Notified --> Resolved: Auto-resolve (condition clears)
    Notified --> Resolved: User resolves
    Acknowledged --> Resolved: User resolves
    Acknowledged --> Resolved: Auto-resolve (condition clears)
    Resolved --> [*]
    Resolved --> Firing: Re-fire after cooldown
```

#### [NEW] [alert_engine.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/alert_engine.go)
Core alert engine:
```go
type AlertEngine struct {
    db          database.Database
    hub         *websocket.Hub
    rules       []*AlertRule       // Cached from DB
    ruleStates  map[RuleDeviceKey]*RuleState // Write-through cache backed by alert_rule_state
    notifier    *Notifier
    mu          sync.RWMutex
    evaluateCh  chan EvaluationEvent
    stopCh      chan struct{}
}

// ProcessMetric is called by the scheduler after each collector run
func (e *AlertEngine) ProcessMetric(device *models.Device, metric *models.Metric)

// EvaluateRules runs all rules against current state for a given device
func (e *AlertEngine) EvaluateRules(deviceID int)

// ReloadRules refreshes the rule cache from the database
func (e *AlertEngine) ReloadRules()

// Start begins the evaluation loop
func (e *AlertEngine) Start(ctx context.Context)

// Stop gracefully shuts down
func (e *AlertEngine) Stop()
```

#### [NEW] [alert_rules.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/alert_rules.go)
Rule evaluation logic:
- `evaluateCondition(condition, metrics) → bool`
- `evaluateThreshold(condition, value) → bool`
- `evaluateAbsence(condition, lastMetricTime) → bool`
- `evaluateStateChange(condition, currentStatus, previousStatus) → bool`
- `evaluateAnomaly(condition, recentValues, baselineValues) → bool`
- Sustained duration tracking per rule per device
- Rule state is persisted in `alert_rule_state` on every transition (`idle` → `pending` → `firing` → `resolved`) so backend restarts do not reset pending timers or cooldowns.
- Alert creation uses an idempotency key/unique constraint such as `(rule_id, device_id, status)` for active alerts to prevent duplicate active alerts under concurrent collector runs.

#### [NEW] [notifier.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/notifier.go)
Notification dispatcher:
```go
type Notifier struct {
    channels map[int]NotificationChannel
    db       database.Database
}

type NotificationChannel interface {
    Type() string
    Send(ctx context.Context, alert *models.Alert, rule *AlertRule) error
}

// Implementations:
type WebhookChannel struct { URL string; Headers map[string]string }
type EmailChannel struct { SMTPHost string; From string; To []string }
type SlackChannel struct { WebhookURL string; Channel string }
type InAppChannel struct { Hub *websocket.Hub }
```

#### New API Endpoints for Alert Engine

```
# Alert Rules CRUD
GET    /api/v1/alert-rules                    — List all rules (with conditions)
GET    /api/v1/alert-rules/:id                — Get rule details
POST   /api/v1/alert-rules                    — Create a new rule
PUT    /api/v1/alert-rules/:id                — Update a rule
DELETE /api/v1/alert-rules/:id                — Delete a rule
POST   /api/v1/alert-rules/:id/toggle         — Enable/disable a rule
POST   /api/v1/alert-rules/:id/test           — Dry-run the rule against current data

# Notification Channels CRUD
GET    /api/v1/notification-channels           — List channels
POST   /api/v1/notification-channels           — Create channel
PUT    /api/v1/notification-channels/:id       — Update channel
DELETE /api/v1/notification-channels/:id       — Delete channel
POST   /api/v1/notification-channels/:id/test  — Send test notification

# Alert History
GET    /api/v1/alerts/:id/history              — State transition audit log
GET    /api/v1/alert-stats                     — Alert engine statistics
```

#### Default Built-in Rules (seeded on first boot)

| Rule Name | Severity | Conditions |
|---|---|---|
| Device Down | `critical` | `status == 'down'` sustained for 60s |
| High Latency | `warning` | `response_time > 500ms` sustained for 5min |
| Critical Latency | `critical` | `response_time > 2000ms` sustained for 2min |
| Device Degraded | `warning` | `status == 'degraded' OR status == 'warning'` |
| No Data Received | `warning` | No metrics for `3 × interval_seconds` |
| Latency Anomaly | `warning` | `response_time > 2.5σ` from 24h baseline |
| Port State Change | `info` | Any port open/close change detected |

---

### Phase 8: Build, Docker & Deployment

#### [NEW] [Makefile](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/Makefile)
```makefile
.PHONY: build run test lint clean

build:
    CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/netmonitor ./cmd/server

build-pcap:  # With packet capture support (requires libpcap-dev)
    CGO_ENABLED=1 go build -ldflags="-s -w" -tags pcap -o bin/netmonitor ./cmd/server

run:
    go run ./cmd/server

test:
    go test ./... -v -count=1

lint:
    golangci-lint run ./...

clean:
    rm -rf bin/
```

#### [MODIFY] [Dockerfile](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/Dockerfile)
New multi-stage build:
```dockerfile
# Stage 1: Build React client
FROM node:22-alpine AS client-builder
WORKDIR /app
COPY client/package.json client/package-lock.json ./client/
RUN cd client && npm ci
COPY client/ ./client/
RUN cd client && npm run build

# Stage 2: Build Go backend
FROM golang:1.24-alpine AS go-builder
RUN apk add --no-cache gcc musl-dev libpcap-dev
WORKDIR /app
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend/ ./backend/
RUN cd backend && CGO_ENABLED=1 go build -tags pcap -ldflags="-s -w" -o /netmonitor ./cmd/server

# Stage 3: Production image
FROM alpine:3.21
RUN apk add --no-cache ca-certificates libpcap tzdata
COPY --from=go-builder /netmonitor /usr/local/bin/netmonitor
COPY --from=client-builder /app/client/dist /app/public
WORKDIR /app
EXPOSE 3000
ENTRYPOINT ["netmonitor"]
```

#### [MODIFY] [docker-compose.yml](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/docker-compose.yml)
Update to build from the new Dockerfile and add a required TimescaleDB service:
- `timescaledb` image: `timescale/timescaledb:latest-pg16`
- Persistent volume: `postgres_data:/var/lib/postgresql/data`
- Environment: `POSTGRES_DB=netmonitor`, `POSTGRES_USER=netmonitor`, `POSTGRES_PASSWORD` from `.env`
- Health check: `pg_isready -U netmonitor -d netmonitor`
- Backend `DATABASE_URL=postgres://netmonitor:${POSTGRES_PASSWORD}@timescaledb:5432/netmonitor?sslmode=disable`
- Backend depends on TimescaleDB health before startup
- Preserve host networking/capabilities only where packet capture and raw socket collectors require them

#### [MODIFY] [docker-compose.dev.yml](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/docker-compose.dev.yml)
Dev mode runs `go run ./cmd/server` with [air](https://github.com/air-verse/air) for hot reload and uses the same TimescaleDB service with a local development volume.

#### [MODIFY] [.env.example](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/.env.example)
Add PostgreSQL/TimescaleDB settings:
```env
DATABASE_URL=postgres://netmonitor:netmonitor@localhost:5432/netmonitor?sslmode=disable
POSTGRES_DB=netmonitor
POSTGRES_USER=netmonitor
POSTGRES_PASSWORD=netmonitor
DB_MAX_CONNS=20
DB_MIN_CONNS=2
DB_MAX_CONN_LIFETIME=1h
DB_HEALTH_CHECK_PERIOD=30s
METRICS_RETENTION_DAYS=90
FLOWS_RETENTION_DAYS=30
LOG_RETENTION_DAYS=14
```

#### [DELETE] [server/](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server)
Remove the Node.js server directory after migration is verified and tested.

---

## Verification Plan

### Automated Tests

#### Unit Tests
```bash
cd backend && go test ./internal/... -v -count=1
```
- Database layer: All 50+ query methods tested against PostgreSQL + TimescaleDB test containers
- Auth: JWT creation/validation, password hashing/verification, session management
- Alert engine: Rule evaluation for all 5 condition types
- Collectors: Mock-based tests for each collector type
- Handlers: HTTP handler tests using `httptest.NewServer`

#### Integration Tests
```bash
cd backend && go test ./integration/... -v -tags integration
```
- Full API endpoint tests (all 60+ endpoints)
- WebSocket connection + event verification
- Alert engine end-to-end: rule → evaluation → notification
- Data retention pruning verification

### Manual Verification

1. **REST API Compatibility**: Run the existing React frontend HTTP flows against the Go backend with unchanged REST request/response shapes — all pages (Dashboard, Devices, Alerts, Reports, Flows, Capture, Insights) must work after the realtime client wrapper is migrated
2. **WebSocket Events**: Verify real-time updates flow to the dashboard (metric updates, alert triggers)
3. **Docker**: `docker compose up -d` and verify health endpoint, full functionality
4. **Alert Engine**: Create custom rules via API, trigger conditions, verify notifications
5. **Performance**: Compare response times and memory usage between Node.js and Go backends
6. **Simulator**: Run `npm run simulate` (still Node.js) and verify the Go backend processes simulated data correctly

#### Logging & Monitoring Tests
```bash
cd backend && go test ./internal/logging/... ./internal/monitoring/... -v -count=1
```
- HTTP logger: verify all request/response fields are captured, body redaction works for auth endpoints
- DB logger: verify slow query detection, query timing accuracy, error capture
- Collector logger: verify all protocol-specific fields are logged for each collector type
- Audit logger: verify all 24 audit event types are recorded to DB
- Alert logger: verify rule evaluation decision trails are complete
- Self-monitor: verify health snapshots are written every 60s
- Monitoring DB: verify all 6 monitoring tables are created, indexed, and retention pruning works
- Log rotation: verify lumberjack rotation triggers at configured size threshold
- Log level filtering: verify per-module log level overrides work (`LOG_MODULE_LEVELS`)
- Context propagation: verify `request_id` and `trace_id` propagate across HTTP→DB→collector→alert chains

### Regression Checklist
- [ ] Login/logout/refresh flow works
- [ ] Device CRUD + auto-sensor creation
- [ ] Metric collection for all 6 protocols (ping, http, port, snmp, system, netflow)
- [ ] Real-time WebSocket updates
- [ ] Alert create/acknowledge/resolve lifecycle
- [ ] Report generation + CSV export
- [ ] Packet capture start/stop/list
- [ ] Port scanning
- [ ] AI health insights
- [ ] Data retention pruning
- [ ] Graceful shutdown
- [ ] Every HTTP request produces a structured log entry with request_id
- [ ] Every DB query is logged with duration and row count
- [ ] Every collector run is logged with device, status, and timing
- [ ] Failed logins are audit-logged with remote IP
- [ ] Slow queries (>100ms) are flagged at WARN level
- [ ] Slow requests (>1000ms) are flagged at WARN level
- [ ] Application health snapshots written every 60s to monitoring_app_health
- [ ] Monitoring data retention pruning runs alongside network data retention
- [ ] Log files rotate at configured size and old files are gzip compressed
- [ ] `GET /api/v1/system/logs` returns queryable structured log entries
- [ ] `GET /api/v1/system/monitoring` returns current app health snapshot
- [ ] `GET /api/v1/system/audit-log` returns security audit trail
