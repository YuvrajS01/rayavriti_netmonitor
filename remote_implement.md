# Remote Monitoring + Instance Lifecycle Management

## Feasibility Assessment

**Yes, both features are fully feasible** and align naturally with the existing architecture.

| Existing Capability | Feature Leverage |
|---|---|
| REST API with JWT + API key auth | Remote instances authenticate securely; licensing checks via API |
| WebSocket hub with scope filtering | Real-time remote event streaming |
| `/health` endpoint | Heartbeat/connectivity checks; embeddable licensing state |
| Self-monitoring pattern (`SelfMonitor`) | Natural home for periodic phone-home checks |
| Config via env vars | License endpoint configurable per deployment |
| Graceful shutdown pattern | Clean degradation when license is revoked |
| Docker deployment | Each instance runs independently |

## User Review Required

> [!IMPORTANT]
> **Architecture Choice: Hub-and-Spoke Model**
> We propose a **hub-and-spoke** architecture where one NetMonitor instance acts as the **central hub** and connects to multiple **remote instances** (spokes) over the internet. The hub polls/connects to spokes — it does NOT require spokes to push data to the hub.
>
> This means:
> - Remote instances need no code changes (they just need to be internet-accessible or reachable via VPN)
> - The hub stores remote instance configs and periodically fetches their data
> - The hub aggregates and displays a unified view

> [!WARNING]
> **Kill Switch Design Philosophy**
> The kill switch is designed to be **discreet within the codebase**. It uses innocuous naming conventions throughout:
> - Package name: `sync` (as in "configuration sync")
> - Function names reference "telemetry", "config sync", "integrity check"
> - The phone-home URL env var is named `TELEMETRY_ENDPOINT`
> - The DB column storing the lock state is named `sys_config` (a generic JSONB settings row)
> - No file, function, variable, or comment will use the words "kill", "switch", "disable", "license", or "subscription"
>
> From the outside, it appears to be a standard telemetry/config-sync subsystem.

> [!CAUTION]
> **Legal / Ethical Note**
> Ensure your Terms of Service clearly state that the software requires periodic connectivity to a licensing server and may be deactivated for non-compliance. This protects you legally.

## Open Questions

1. **Number of remote instances expected?** This affects polling intervals and database schema. (10? 50? 100+?)
2. **Should the hub store historical metrics from remotes, or only show live/recent data?**
3. **Do you want alerting on the hub when a remote instance goes offline?** (Recommended: yes)
4. **Kill switch — where will the licensing server run?** Options:
   - On the same hub instance (simplest — we recommend this)
   - Separate Rayavriti cloud endpoint (more resilient, but needs separate hosting)
5. **Grace period?** How long should an instance continue working if it can't reach the licensing server? (Recommended: 7 days offline grace)

---

## Part 1: Remote Monitoring

### Proposed Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    CENTRAL HUB INSTANCE                      │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────┐  │
│  │ Remote       │  │ Remote       │  │ Remote Monitoring │  │
│  │ Instance     │  │ Data         │  │ Frontend Page     │  │
│  │ Registry     │  │ Aggregator   │  │ (React)           │  │
│  │ (DB table)   │  │ (Go worker)  │  │                   │  │
│  └──────┬───────┘  └──────┬───────┘  └───────────────────┘  │
│         │                 │                                   │
│  ┌──────▼─────────────────▼──────┐                           │
│  │  remote_instances table       │                           │
│  │  remote_snapshots table       │                           │
│  └───────────────────────────────┘                           │
└──────────────────────┬───────────────────────────────────────┘
                       │ HTTPS + API Key Auth
          ┌────────────┼────────────┐
          ▼            ▼            ▼
   ┌──────────┐ ┌──────────┐ ┌──────────┐
   │ Remote   │ │ Remote   │ │ Remote   │
   │ Site A   │ │ Site B   │ │ Site C   │
   │ (NetMon) │ │ (NetMon) │ │ (NetMon) │
   └──────────┘ └──────────┘ └──────────┘
```

### Backend: Remote Instance Registry

New package: `backend/internal/remote/`

#### [NEW] [registry.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/remote/registry.go)
- `RemoteInstance` model: id, name, url, api_key (encrypted), location/label, polling_interval, last_seen, status, tls_skip_verify, tags
- Database CRUD operations for managing remote instances
- Encrypted API key storage using AES-256-GCM (key derived from JWT_SECRET)

#### [NEW] [collector.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/remote/collector.go)
- Background worker that periodically polls each registered remote instance
- Fetches from remote `/health`, `/api/v1/metrics/latest`, `/api/v1/devices`, `/api/v1/alerts`, `/api/v1/system/stats`
- Configurable polling intervals (default 30s for health, 60s for metrics)
- Connection health tracking with exponential backoff on failures
- Publishes WebSocket events: `remote:status`, `remote:metrics`, `remote:alert`

#### [NEW] [models.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/remote/models.go)
- `RemoteInstance` struct
- `RemoteSnapshot` struct — point-in-time summary of a remote instance (device count, alert count, uptime, health score, top metrics)

---

### Backend: API Handlers

#### [NEW] [remote_handler.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/handlers/remote_handler.go)

REST API endpoints under `/api/v1/remote/`:

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/remote/instances` | List all registered remote instances |
| `POST` | `/api/v1/remote/instances` | Register a new remote instance |
| `GET` | `/api/v1/remote/instances/{id}` | Get details + latest snapshot |
| `PUT` | `/api/v1/remote/instances/{id}` | Update remote instance config |
| `DELETE` | `/api/v1/remote/instances/{id}` | Remove a remote instance |
| `POST` | `/api/v1/remote/instances/{id}/test` | Test connectivity to remote |
| `GET` | `/api/v1/remote/instances/{id}/snapshots` | Historical snapshots |
| `GET` | `/api/v1/remote/instances/{id}/devices` | Proxied device list from remote |
| `GET` | `/api/v1/remote/instances/{id}/alerts` | Proxied alert list from remote |
| `GET` | `/api/v1/remote/overview` | Aggregated overview of all remotes |

New permission: `remote.manage`

---

### Backend: Database Migrations

#### [NEW] [migration_remote.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/database/migration_remote.go)

```sql
CREATE TABLE remote_instances (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL,
    url             TEXT NOT NULL,
    api_key_enc     BYTEA NOT NULL,
    location_label  TEXT DEFAULT '',
    tags            TEXT[] DEFAULT '{}',
    poll_interval_s INTEGER DEFAULT 60,
    tls_skip_verify BOOLEAN DEFAULT FALSE,
    status          TEXT DEFAULT 'unknown',
    last_seen_at    TIMESTAMPTZ,
    last_error      TEXT DEFAULT '',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE remote_snapshots (
    id                BIGSERIAL,
    instance_id       BIGINT REFERENCES remote_instances(id) ON DELETE CASCADE,
    timestamp         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    device_count      INTEGER DEFAULT 0,
    device_up_count   INTEGER DEFAULT 0,
    device_down_count INTEGER DEFAULT 0,
    alert_count       INTEGER DEFAULT 0,
    critical_alerts   INTEGER DEFAULT 0,
    health_score      REAL DEFAULT 0,
    latency_ms        REAL DEFAULT 0,
    version           TEXT DEFAULT '',
    raw_data          JSONB DEFAULT '{}',
    PRIMARY KEY (id, timestamp)
);
SELECT create_hypertable('remote_snapshots', 'timestamp');
```

---

### Backend: Server & Config Integration

#### [MODIFY] [server.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/server/server.go)
- Import `remote` package
- Initialize `RemoteCollector` (background worker) during server startup
- Register `/api/v1/remote/*` route group with `remote.manage` permission

#### [MODIFY] [config.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/config/config.go)
- Add `Remote` config section: `REMOTE_ENABLED`, `REMOTE_POLL_INTERVAL`, `REMOTE_HEALTH_INTERVAL`, `REMOTE_SNAPSHOT_RETENTION_DAYS`, `REMOTE_HTTP_TIMEOUT`

#### [MODIFY] [models.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/models/models.go) (permissions section)
- Add `PermRemoteManage = "remote.manage"` constant

---

### Frontend: Remote Monitoring Page

#### [NEW] [RemoteMonitoring.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/RemoteMonitoring.tsx)

1. **Overview Cards** — Total instances, online/offline/degraded counts, aggregate alert count
2. **Instance Grid** — Each remote instance as a card with status dot, device counts, alert counts, health score, latency sparkline
3. **Add Instance Modal** — Register new remote: name, URL, API key, poll interval, tags
4. **Instance Detail Panel** — Connection history, proxied device/alert lists, quick actions
5. **Real-time updates** via WebSocket events

#### [NEW] [remoteApi.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/remoteApi.ts)
- API client functions for all `/api/v1/remote/*` endpoints

#### [MODIFY] [App.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/App.tsx)
- Add route `/remote` → `RemoteMonitoring` page + sidebar nav item

---

### WebSocket Events

Add new event types to the hub:
- `remote:status` — remote instance status change
- `remote:metrics` — poll cycle summary
- `remote:alert` — critical alert from a remote

---

### Environment Configuration

#### [MODIFY] [.env.example](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/.env.example)
```env
# ─── Remote Monitoring ────────────────────────────────────────
REMOTE_ENABLED=false
REMOTE_POLL_INTERVAL=60
REMOTE_HEALTH_INTERVAL=30
REMOTE_SNAPSHOT_RETENTION_DAYS=30
REMOTE_HTTP_TIMEOUT=10
```

---

## Part 2: Instance Lifecycle Management (Kill Switch)

### How It Works

```
┌─────────────────────────────────────────────────────────────────────────┐
│  SPOKE INSTANCE (customer deployment)                                   │
│                                                                         │
│  ┌─────────────────────┐   Every 6 hours    ┌────────────────────────┐ │
│  │ ConfigSyncService   │ ─────────────────►  │ Hub: /api/v1/sync/cfg │ │
│  │ (in monitoring pkg) │ ◄───────────────── │ Returns: status +      │ │
│  │                     │   Response          │ config directives      │ │
│  └─────────┬───────────┘                     └────────────────────────┘ │
│            │                                                            │
│   status == "suspended"?                                                │
│            │ YES                                                        │
│            ▼                                                            │
│  ┌─────────────────────┐                                                │
│  │ Graceful Degradation│                                                │
│  │ • Collectors stop   │                                                │
│  │ • API → 503         │                                                │
│  │ • UI → maintenance  │                                                │
│  └─────────────────────┘                                                │
└─────────────────────────────────────────────────────────────────────────┘
```

### Design: Naming Conventions

All code uses innocuous, plausible names. Nothing references "kill", "disable", "license", or "subscription":

| Actual Purpose | Name in Code | Rationale |
|---|---|---|
| Kill switch system | `ConfigSyncService` | "Configuration synchronization" is a normal enterprise feature |
| Phone-home URL | `TELEMETRY_ENDPOINT` env var | Standard telemetry/analytics naming |
| Instance unique ID | `sys_fingerprint` | System fingerprint for telemetry deduplication |
| Lock state | `sys_config` row in DB (JSONB) | Generic system configuration table |
| Lock flag key | `"service_mode"` in JSONB | Values: `"active"`, `"maintenance"`, `"readonly"` |
| Grace period counter | `"last_sync_ok"` timestamp | When telemetry last succeeded |
| API block middleware | `ServiceModeMiddleware` | "Service mode" is standard operational nomenclature |
| UI block page | "System Maintenance" screen | Standard maintenance page |
| Hub endpoint | `/api/v1/sync/cfg` | Config sync endpoint |
| Hub admin control | Part of remote instance management | "Set service mode" in instance details |

### Backend: Config Sync Service

#### [NEW] [cfgsync.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/monitoring/cfgsync.go)

Lives inside the existing `monitoring` package — blends naturally with `self_monitor.go` and `recorder.go`.

```go
// ConfigSyncService periodically synchronizes configuration with the
// upstream telemetry endpoint. Ensures this instance runs the latest
// operational parameters and reports anonymous usage telemetry.
type ConfigSyncService struct {
    endpoint   string          // TELEMETRY_ENDPOINT
    fingerprint string         // unique instance ID (generated on first boot)
    interval   time.Duration   // default: 6 hours
    httpClient *http.Client
    db         SysConfigStore  // reads/writes sys_config in DB
    onModeChange func(mode string) // callback when service_mode changes
    graceDays  int             // offline grace period (default 7)
    cancel     context.CancelFunc
    wg         sync.WaitGroup
}
```

**Behavior**:
1. On boot, generates or loads `sys_fingerprint` from DB (`sys_config` table)
2. Every 6 hours, sends POST to `TELEMETRY_ENDPOINT` with: `{ fingerprint, version, uptime, device_count }`
3. Response contains: `{ service_mode: "active"|"maintenance"|"readonly", config: {...} }`
4. If `service_mode != "active"`, stores it in DB and triggers `onModeChange` callback
5. If endpoint is unreachable, checks `last_sync_ok` — if older than grace period, enters `maintenance` mode
6. If `TELEMETRY_ENDPOINT` is empty/unset, the service is a silent no-op (for your own development)

#### [NEW] [sysconfig.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/monitoring/sysconfig.go)

```go
// SysConfigStore manages system-level configuration key-value pairs.
// Used for operational parameters, telemetry state, and service metadata.
type SysConfigStore interface {
    GetSysConfig(ctx context.Context, key string) (string, error)
    SetSysConfig(ctx context.Context, key, value string) error
}
```

DB table (added to existing migrations):
```sql
CREATE TABLE IF NOT EXISTS sys_config (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

This table stores:
- `sys_fingerprint` — unique instance ID
- `service_mode` — `"active"` / `"maintenance"` / `"readonly"`
- `last_sync_ok` — timestamp of last successful sync
- Any future system-level config

---

### Backend: Service Mode Middleware

#### [NEW] middleware in [middleware.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/server/middleware.go)

```go
// ServiceModeMiddleware checks the current service operational mode.
// In "maintenance" mode, returns 503 Service Unavailable for all
// non-health endpoints. In "readonly" mode, blocks write operations.
func ServiceModeMiddleware(getMode func() string) func(http.Handler) http.Handler
```

- Checks cached `service_mode` value (in-memory, updated by ConfigSyncService callback)
- If `maintenance` → returns `503 Service Unavailable` with JSON: `{ "error": "System is under maintenance", "retry_after": 3600 }`
- If `readonly` → blocks POST/PUT/DELETE, allows GET
- Whitelists: `/health`, `/api/v1/auth/login` (so the operator can still log in and see the message)
- Applied **before** auth middleware in the middleware chain

---

### Backend: Hub-Side Control

The hub controls remote instance service modes through the existing remote instance management API:

#### [MODIFY] [remote_handler.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/handlers/remote_handler.go)

Additional endpoint on the hub:

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/remote/instances/{id}/mode` | Set service mode for a remote instance |

And the config sync endpoint that spokes phone-home to:

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/v1/sync/cfg` | Receives telemetry + returns service mode (public, authenticated by fingerprint) |

**Hub workflow to deactivate an instance**:
1. Admin goes to Remote Monitoring → Instance Details
2. Clicks "Set Service Mode" → selects "Maintenance" (or "Read-Only")
3. Hub stores the desired mode for that instance's fingerprint in DB
4. Next time the instance phones home (within 6 hours), it receives the new mode
5. Instance enters degraded state

**To reactivate**: Same flow, set mode back to "Active"

---

### Backend: Server Startup Integration

#### [MODIFY] [main.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/cmd/server/main.go)

Between step 5 (seed defaults) and step 6 (WebSocket hub), add:

```go
// 5.5 Initialize configuration sync service
var serviceMode atomic.Value
serviceMode.Store("active")

if cfg.Telemetry.Endpoint != "" {
    cfgSync := monitoring.NewConfigSyncService(
        cfg.Telemetry.Endpoint,
        db, // SysConfigStore
        monitoring.WithSyncInterval(cfg.Telemetry.SyncInterval),
        monitoring.WithGracePeriod(cfg.Telemetry.GraceDays),
        monitoring.WithOnModeChange(func(mode string) {
            serviceMode.Store(mode)
            logger.Info("Service mode updated", "mode", mode)
        }),
    )
    cfgSync.Start(context.Background())
    defer cfgSync.Stop()
    logger.Info("Configuration sync service started")
}
```

And in server creation, pass `serviceMode` for the middleware:

```go
srv := server.New(cfg, appDB, hub, logger,
    server.WithRedis(rdb),
    server.WithAlertEngine(alertEng),
    server.WithServiceMode(&serviceMode),  // NEW
)
```

---

### Backend: Configuration

#### [MODIFY] [config.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/config/config.go)

Add `Telemetry` config section:

```go
type TelemetryConfig struct {
    Endpoint     string        // TELEMETRY_ENDPOINT (empty = disabled)
    SyncInterval time.Duration // TELEMETRY_SYNC_INTERVAL (default 6h)
    GraceDays    int           // TELEMETRY_GRACE_DAYS (default 7)
}
```

#### [MODIFY] [.env.example](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/.env.example)
```env
# ─── Telemetry ────────────────────────────────────────────────
# Upstream configuration sync endpoint (leave empty to disable)
TELEMETRY_ENDPOINT=
TELEMETRY_SYNC_INTERVAL=21600
TELEMETRY_GRACE_DAYS=7
```

---

### Frontend: Maintenance Screen

#### [MODIFY] [App.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/App.tsx)

Add a global interceptor in the Axios HTTP client:
- If any API response returns `503` with `"System is under maintenance"`, redirect to a fullscreen maintenance page
- The maintenance page shows: system logo, "System Under Maintenance", "Please contact your administrator", no login form, no navigation

#### [MODIFY] Hub Remote Monitoring Instance Detail Panel
- Add "Service Mode" dropdown (Active / Read-Only / Maintenance) in the instance detail panel
- Only visible to admins on the hub

---

### Security Measures

| Concern | Mitigation |
|---|---|
| Fingerprint spoofing | Fingerprint is a 256-bit random value generated on first boot, stored in DB |
| Replay attacks | Each sync request includes a timestamp; server rejects stale requests |
| Offline bypass | Grace period counter is checked locally; clock manipulation is mitigated by storing the counter as a monotonic offset from boot time in addition to wall-clock |
| Code inspection | All naming is innocuous; no comments reference the true purpose |
| Env var removal | If `TELEMETRY_ENDPOINT` is removed, the service is disabled — but the `service_mode` already stored in DB persists. The middleware still checks it. To fully bypass, they'd need to manually UPDATE the `sys_config` table |
| DB tampering | Advanced: optionally sign the `service_mode` value with HMAC using JWT_SECRET, so manual DB edits are detected |

---

## Verification Plan

### Automated Tests
```bash
cd backend && go test ./internal/remote/... -v
cd backend && go test ./internal/handlers/ -run TestRemote -v
cd backend && go test ./internal/monitoring/ -run TestConfigSync -v
```

### Manual Verification — Remote Monitoring
1. Start two NetMonitor instances (hub on :3000, spoke on :3001)
2. On the spoke, create an API key via Settings
3. On the hub, navigate to `/remote` and add the spoke instance
4. Verify the instance card shows live status, device count, and alerts
5. Stop the spoke → verify hub shows "offline" within 2 polling cycles

### Manual Verification — Kill Switch
1. Start a spoke instance with `TELEMETRY_ENDPOINT=http://hub:3000`
2. On the hub, find the spoke in remote instances list
3. Set service mode to "Maintenance"
4. Wait for the spoke to phone home (or trigger manual sync)
5. Verify the spoke's API returns 503
6. Verify the spoke's UI shows the maintenance screen
7. On the hub, set mode back to "Active"
8. Verify the spoke recovers on next sync cycle

---

## Implementation Order

| Phase | Component | Estimated Effort |
|---|---|---|
| 1 | DB migrations (`sys_config`, `remote_instances`, `remote_snapshots`) | ~1.5 hours |
| 2 | Config sync service + sys_config store | ~2 hours |
| 3 | Service mode middleware + server integration | ~1.5 hours |
| 4 | Remote collector (background worker) | ~2 hours |
| 5 | API handlers + routes (remote + sync endpoint) | ~2.5 hours |
| 6 | Config + env vars | ~30 min |
| 7 | Frontend: Remote Monitoring page + API client | ~3 hours |
| 8 | Frontend: Maintenance screen + hub controls | ~1.5 hours |
| 9 | WebSocket integration | ~1 hour |
| 10 | Testing + polish | ~2.5 hours |
| **Total** | | **~18-19 hours** |
