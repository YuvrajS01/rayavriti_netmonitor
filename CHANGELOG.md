# Changelog

All notable changes to Rayavriti NetMonitor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [4.0.0] - 2026-07-23

Core polling engine rewrite — major architectural overhaul replacing the goroutine-per-device model with a priority-based worker pool, timing wheel dispatcher, adaptive backoff, dependency-aware scheduling, and batch database operations. Also introduces hierarchical campus visualizations, real topology edges, and critical security vulnerability fixes.

### Added — Backend

#### Polling Engine Overhaul (Phases 1–7)

- **Worker pool engine** (`a713ad8`) — Fixed-size goroutine pool with three priority queues (critical/normal/low) and priority-based worker selection, replacing the unbounded goroutine-per-device model.
- **Poll dispatcher** (`a713ad8`) — Timing wheel using a min-heap schedule with a single shared timer instead of N per-device tickers; supports adaptive backoff on failures.
- **Result pipeline** (`a713ad8`) — Fan-in pipeline with batch buffer (size + time flush triggers), DB writes, WebSocket broadcast, and alert evaluation.
- **Collector meta interface** (`a713ad8`) — New `CollectorMeta` interface with `DefaultTimeout` and `Weight` methods for standardized collector behavior.
- **Scheduler rewrite** (`a713ad8`) — Orchestrates WorkerPool + PollDispatcher + ResultPipeline, replaces the old goroutine-per-device model.
- **Device state tracker** (`8f5cdf6`) — Per-device health tracking with adaptive backoff (1x→2x→4x→8x interval escalation) and rolling average response time.
- **Dependency tree** (`8f5cdf6`) — PRTG-inspired parent/child dependency model with auto-pause/resume support; child devices are paused when parent is unreachable.
- **Async remote collector** (`cbeb786`) — Replaced per-request `http.Client` with shared pooled transport (connection reuse); fan-out polling with bounded semaphore (max 10 concurrent).
- **Batch metric insert** (`5ed8d28`) — New `RecordMetricsBatch` on the Database interface using `pgx.CopyFrom` for bulk COPY insert; `MetricBuffer.flush()` updated to use batch writes.
- **Poller configuration env vars** (`1573fec`) — `POLLER_WORKER_COUNT`, `POLLER_MAX_WORKER_COUNT`, `POLLER_*_QUEUE_SIZE`, `POLLER_RESULT_BATCH_SIZE`, `POLLER_RESULT_FLUSH_MS`, `REMOTE_MAX_CONCURRENT`.
- **Self-monitoring metrics** (`1573fec`) — Added `WorkerPool*` and `Poller*` optional stat providers; poller metrics fields in `SystemMetrics` struct; migration V41 for new monitoring columns.
- **Device priority column** (`8aa4b46`) — New `priority INT` field on Device model (default 1); migration V42 with index.

#### Frontend

- **FloorPlanView component** (`f867768`) — Room view (location cards with device status) and Rack view (42U vertical rack diagram with device bars at rackPosition); status pulse animations; graceful empty states.
- **Hierarchical CampusMap** (`d279ba9`) — Rewritten from a flat grid into a multi-level tree (campus → building → floor → room/rack); building cards with proportional status bars, device count badges, and click-to-select with highlight.
- **Real topology edges in NetworkTopology** (`d497113`) — Now fetches dependency tree from `GET /api/v1/topology` and flattens into parent→child edges; category-aware node shapes (router=cross, switch=diamond, firewall=hexagon, server=rounded rect); dependency port labels; scroll-wheel zoom; animated particles change red on down endpoints; real edge count vs inferred.
- **Typed API helpers** (`ed0e83c`) — `TopologyNode`/`LocationNode` interfaces; `getTopologyTree()`, `getLocationTree()`, `getLocationDevices()` API client functions.

### Changed

- **Scheduler architecture** (`a713ad8`, `de16af0`) — Complete rewrite from goroutine-per-device to WorkerPool + PollDispatcher + ResultPipeline orchestration.
- **Dashboard and Campus pages** (`f867768`, `d279ba9`) — Campus page now uses FloorPlanView; CampusMap is now hierarchical.
- **NetworkTopology data source** (`d497113`) — Switched from simulated edges to real `/api/v1/topology` dependency tree.
- **Env dev example** (`8df92ee`) — Updated with new poller configuration variables.

### Fixed

- **Pollers not firing on reconcile** (`d0a522f`) — `Upsert` no longer unconditionally resets `NextPollAt`, which was pushing every device's next poll indefinitely into the future; only reschedules when interval actually changes.
- **Poller memory leaks** (`de16af0`) — `reconcile()` now removes stale devices via `PollDispatcher.DeviceIDs()` instead of accumulating indefinitely; fixed `jobCount` tracking sync with dispatcher count.
- **Result pipeline payload loss** (`de16af0`) — `ResultPipeline` now retains complete `CollectResult` payload (PacketLoss, CPUUsage, etc.) and persists device status changes.
- **Worker pool deadlock** (`9b7445c`) — Fixed deadlock in `dispatchDue` by releasing lock before enqueue; fixed infinite loop on paused entries by scheduling them 24h ahead; fixed Resume not waking dispatcher.
- **Lint/typecheck/build issues** (`44c70f9`) — Fixed ESLint `react-hooks/refs` violation, TypeScript type error, gofmt formatting (14 files), staticcheck QF1003; all builds lints and tests pass.
- **npm vulnerabilities** (`714e355`) — Upgraded axios and brace-expansion to resolve high-severity vulnerabilities.
- **Go vulnerability** (`ad233f5`) — Upgraded `golang.org/x/text` v0.38.0 → v0.39.0 to fix GO-2026-5970.

### Removed

- Old implementation docs (`20fcaac`) — Removed `remote_implement.md` and `review_codex.md` as they represent stale design documents superseded by implementation.

### Tests

- **Async poller tests** (`9b7445c`) — ~45 new tests covering WorkerPool, PollDispatcher, DeviceStateTracker, DependencyTree, ResultPipeline, and Scheduler (deadlock, infinite loop, wakeup edge cases).
- **Existing test suite** (`44c70f9`) — All 108 frontend tests and all backend tests passing.

### New Environment Variables

- `POLLER_WORKER_COUNT` — Number of poller workers (default: 5)
- `POLLER_MAX_WORKER_COUNT` — Max concurrent poller workers (default: 20)
- `POLLER_CRITICAL_QUEUE_SIZE` — Critical priority queue capacity (default: 100)
- `POLLER_NORMAL_QUEUE_SIZE` — Normal priority queue capacity (default: 200)
- `POLLER_LOW_QUEUE_SIZE` — Low priority queue capacity (default: 300)
- `POLLER_RESULT_BATCH_SIZE` — Result batch size before flush (default: 50)
- `POLLER_RESULT_FLUSH_MS` — Result flush interval in ms (default: 1000)
- `REMOTE_MAX_CONCURRENT` — Max concurrent remote polls (default: 10)
- `COLLECTOR_INTERVAL_SEC` — Default poll frequency in seconds

### Database Migrations

- V41 — New columns in `monitoring_app_health` for poller self-monitoring metrics
- V42 — `priority INT` column on `devices` table with index

## [3.9.0] - 2026-07-19

Visual UI redesign release. Transforms the text-heavy, table-centric interface into an infographic-driven experience with interactive charts, force-directed topology maps, heatmaps, sparklines, Gantt timelines, and live data visualizations across all pages. Also fixes WebSocket realtime authentication, device modal AI Health score rendering, and campus multi-level location hierarchy.

### Added — Frontend

#### New Components
- **AnimatedCounter** (`65697fb`) — Rolling number component supporting number/percent/bytes/ms formats with configurable animation duration.
- **Sparkline** (`65697fb`) — Inline gradient area chart with trend coloring and optional trend arrow indicator.
- **RingGauge** (`65697fb`) — Animated SVG ring/arc gauge replacing inline AiHealthScore SVG, supports label, size, and stroke width props.
- **StatCard enhancements** (`65697fb`) — Added animated counters, inline sparklines, trend arrows, delta values, and glow effects to all stat cards across Discovery, ISP, Maintenance, RemoteMonitoring, and Sensors pages.
- **NetworkTopology page** (`65697fb`) — New `/devices/topology` route with interactive force-directed graph built on a dependency-free canvas simulation; includes status-colored nodes, hover tooltips, click-to-DeviceModal, zoom/pan/drag-pin, filters (status/protocol/location), live WebSocket updates, animated edge particles, and hub-and-spoke fallback when no parent dependency edges exist.
- **NetworkTopologyMini** (`65697fb`) — Compact force-graph preview on Dashboard linking to full topology page.
- **StatusHeatmap** (`65697fb`) — Device × time status grid on Dashboard for at-a-glance monitoring.
- **SankeyDiagram** (`65697fb`) — Flow visualization on FlowAnalysis page showing source→destination bandwidth.
- **Treemap** (`65697fb`) — Protocol bandwidth treemap on FlowAnalysis page.
- **RadarChart** (`65697fb`) — Multi-factor comparison on AvgResponseByStatus dashboard widget.
- **HeatmapChart** (`65697fb`) — Reusable heatmap visualization component.
- **GanttTimeline** (`65697fb`) — Timeline visualization for incident windows, maintenance schedule, and recent events.
- **AreaSparkline** (`65697fb`) — Reusable area sparkline chart component.
- **AnimatedDonut** (`65697fb`) — Animated donut chart used in StatusDistribution with hover glow/expand and animated center label.

#### Dashboard Overhaul
- **ResponseTimeChart** (`65697fb`) — AreaChart with gradient fills and staggered draw animation.
- **AiHealthScore** (`65697fb`) — RingGauge + health trend sparkline replacing static SVG.
- **ResourceLoadChart/ResourceBar** (`65697fb`) — Glow-threshold warnings when utilization exceeds 90%.
- **LatestMetricsTable** (`65697fb`) — Status dots with glow, response bars, staggered row animation.
- **ActiveAlertsList** (`65697fb`) — Severity border accents, pulsing dots, timeline bars.
- **DashboardSkeleton** (`65697fb`) — Shimmer-shaped chart/skeleton placeholders for loading states.

#### Page Enhancements
- **Devices** (`65697fb`) — Sparklines, status rings with glow on up/ok devices, hover elevation with shadow, stagger animation on card grid, topology link in page header.
- **Campus** (`65697fb`) — Tree/map/floor tabs with schematic SVG CampusMap and status tiles.
- **DeviceModal** (`65697fb`, `53d29cc`) — Response sparkline + animated health ring showing real persisted AI Health score; smaller font for modal context.
- **FlowAnalysis** (`65697fb`) — SankeyDiagram + Treemap + protocol comparison bars.
- **PacketCapture** (`65697fb`) — Packet-rate sparkline, size histogram, capture session ring gauge.
- **AIHealth** (`65697fb`) — RingGauge device cards + radar charts per device.
- **Alerts** (`65697fb`) — Animated stat cards with severity indicators.
- **ISP** (`65697fb`) — Circuit throughput comparison bars, sparklines on stat cards.
- **Incidents** (`65697fb`) — GanttTimeline for recent incident windows.
- **IncidentDetail** (`65697fb`) — Impacted topology grid with status glow dots, active step ring highlight on status flow.
- **Discovery** (`65697fb`) — Sparklines on stat cards, discovery sweep visualization grid.
- **Maintenance** (`65697fb`) — GanttTimeline for maintenance schedule, sparklines on stat cards.
- **Logs** (`65697fb`) — Log volume pulse bar chart by severity level.
- **RemoteMonitoring** (`65697fb`) — Sparklines and trend arrows on stat cards.
- **Sensors** (`65697fb`) — Migrated to StatCard component with animated values.
- **UserManagement** (`65697fb`) — Stat cards with animated counters.
- **Login** (`65697fb`) — Glass-card login form with atmosphere background.

#### CSS & Animations
- **index.css** (`65697fb`) — Shimmer, glow-pulse, particle-drift keyframes; glass-card, glow-success/glow-warning/glow-error utilities; card-stagger animation; status-dot-live pulse; login-atmosphere gradient.

### Added — Backend

- **AI Health scores API** (`53d29cc`) — New `GET /api/v1/health/scores` and `GET /api/v1/health/scores/{deviceId}` endpoints (devices.read permission) returning `DeviceHealthScoreRow` for device modal and topology widgets.

### Changed

- **Vite config** (`65697fb`) — Manual chunk splitting for chart libraries; vendor-charts chunk for production optimization.
- **App.tsx** (`65697fb`) — Lazy-loaded `/devices/topology` route.

### Fixed

- **WebSocket subprotocol auth** (`9cbabbd`) — Always send access token as `Sec-WebSocket-Protocol` subprotocol when an in-memory token is available, fixing realtime connection failure over plain HTTP and non-prod setups.
- **WebSocket token rehydration** (`3b440c0`) — Persist access token alongside user in localStorage and rehydrate at module load so SocketProvider.connect() authenticates immediately on page reload instead of staying stuck on "Realtime paused".
- **WebSocket dev origin** (`36297b1`) — `DevAwareAllowedOrigins` now appends common local dev origins (Vite `:5173` + API `:3000`) in non-production, fixing 403 "request origin not allowed" on WebSocket upgrade from the Vite dev server.
- **Device modal AI Health score** (`53d29cc`) — Replaced hardcoded 92/20 RingGauge values with the device's persisted AI Health score from `/api/v1/health/scores/{id}`; falls back to status-based value only if no score exists.
- **AI Health score display** (`964aa14`) — Round RingGauge value (decimals=0) and use `text-lg` instead of `text-3xl` so the score fits cleanly in the 54px modal gauge.
- **Campus location hierarchy** (`aba6f45`) — Store empty location code as NULL (not `''`) so the UNIQUE constraint permits multiple locations without a code; qualify all recursive CTE columns with table alias to fix ambiguous column references in `GetSubtree`, unblocking nested locations like Main Campus → CS Dept → Lab 1.

### Removed

- `react-force-graph-2d` dependency — Topology uses a dependency-free canvas simulation.
- `leaflet` and `react-leaflet` dependencies — Campus uses schematic SVG instead of interactive maps.

## [3.8.0] - 2026-07-19

Fleet operations release. Adds remote monitoring for centralized management of multiple NetMonitor instances, service mode coordination (active/readonly/maintenance), and telemetry-based config sync.

### Added — Backend

- **Remote monitoring registry** (`e5bc96a`) — New `remote` package with `Store` for CRUD on remote NetMonitor instances; encrypted API key storage; poll interval, TLS skip verify, location label, and tag support. Migrations V38–V40 create `remote_instances`, `remote_snapshots` (TimescaleDB hypertable), and `sys_config` tables.
- **Remote collector** (`e5bc96a`) — Background `Collector` polls registered instances on a configurable interval, fetches `/api/v1/remote/overview` and `/api/v1/remote/instances` snapshots via API key auth, stores device counts, alert counts, health score, and latency as TimescaleDB time-series rows. Automatic snapshot pruning by retention days.
- **Remote API handler** (`e5bc96a`) — `RemoteHandler` exposes 8 endpoints: List, Overview, Get, Create, Update, Delete, Test, and Mode under `/api/v1/remote/*`. Test triggers an immediate poll cycle; Mode switches service mode (active/readonly/maintenance) on a remote instance.
- **Config sync service** (`e5bc96a`) — `ConfigSyncService` periodically POSTs system fingerprint to a configurable telemetry endpoint and receives back the assigned service mode. Includes grace period before mode downgrade on missed check-ins.
- **Service mode middleware** (`e5bc96a`) — `ServiceModeMiddleware` gates mutating POST/PUT/DELETE requests based on the instance's current service mode; read-only and maintenance modes return 403 on writes.
- **System config store** (`e5bc96a`) — Key-value `sys_config` table with `SysConfigStore` for persisting system identity fingerprint and other configuration.
- **Remote handler for sync** (`e5bc96a`) — `SyncHandler` exposes `POST /api/v1/sync/config` (returns service mode for a fingerprint) and `GET /api/v1/sync/identity` (returns local system fingerprint).

### Added — Frontend

- **Remote Monitoring page** (`e5bc96a`) — New `/remote` route with full management UI: overview stat cards (instances, online, offline, degraded, alerts), instance registration form, instance detail sidebar with device counts, health score, latency, service mode selector, connection test, and removal.
- **Remote API client** (`e5bc96a`) — `remoteApi.ts` with typed functions for all remote endpoints.
- **Layout sidebar entry** (`e5bc96a`) — "Remote Monitoring" link in sidebar navigation with `lan` icon.

### Added — Tests

- **Remote monitoring tests** (`e5bc96a`) — Unit tests for remote instance list, overview, and error handling.

### Changed

- **Server wiring** (`e5bc96a`) — `Server` struct gains `remoteCollector` and `serviceMode` fields; `main.go` conditionally starts remote collector and config sync when `REMOTE_ENABLED=true`.
- **WebSocket hub** (`e5bc96a`) — Added `remote:status` and `remote:metrics` event types for realtime updates.
- **Dockerfile** (`e5bc96a`) — Updated base image.

### Fixed

- **Remote monitoring response handling** (`6b72b5d`) — Hardened JSON parsing and body close patterns across remote collector and registry to prevent nil dereference on malformed responses.
- **Disabled remote monitoring reporting** (`77bb6fc`) — Frontend now returns a clear 503 message when `REMOTE_ENABLED=false` instead of showing an empty state with no explanation.
- **Packet capture execution** (`351cc74`) — Restored broken packet capture functionality.
- **golangci-lint issues** (`2aa468d`) — Fixed unchecked `body.Close()`, XSS taint via `json.NewEncoder` for proxy responses, and if-else to switch conversion per QF1003.

### New Environment Variables

- `REMOTE_ENABLED` — Enable remote monitoring (default: `false`)
- `REMOTE_POLL_INTERVAL` — Seconds between remote polls (default: `60`)
- `REMOTE_HEALTH_INTERVAL` — Seconds between health checks (default: `30`)
- `REMOTE_SNAPSHOT_RETENTION_DAYS` — Days to retain snapshots (default: `30`)
- `REMOTE_HTTP_TIMEOUT` — HTTP client timeout in seconds (default: `10`)
- `TELEMETRY_ENDPOINT` — Config sync telemetry URL (default: empty)
- `TELEMETRY_SYNC_INTERVAL` — Seconds between telemetry syncs (default: `21600`)
- `TELEMETRY_GRACE_DAYS` — Days before mode downgrade on missed sync (default: `7`)

## [3.7.5] - 2026-07-16

Cross-platform Docker release. Adds Windows/macOS compatibility via bridge networking overrides, fixes CORS boot crash when `CORS_ORIGINS` is unset, and makes Vite dev proxy target configurable.

### Added

- **Docker Windows/macOS overrides** (`9e3bfa1`, `2f6f69c`) — New `docker-compose.windows.yml` and `docker-compose.dev.windows.yml` override files that switch from host networking (Linux-only) to a bridge network with service-name DNS, enabling the stack to run on Windows and macOS. Linux host-networking remains the default.
- **Configurable Vite proxy target** (`2f6f69c`) — `client/vite.config.ts` now accepts `VITE_PROXY_TARGET` env var (defaults to `http://localhost:3000`) so the dev client can reach the backend by service name on a bridge network.

### Fixed

- **CORS boot crash** (`0ff825c`) — `CORS_ORIGINS` now defaults to `http://localhost:3000` when unset, preventing the server from crashing on startup with "CORS_ORIGINS is required in production". Config tests updated to assert default and explicit override behavior.

### Changed

- **Docker compose restructured** (`9e3bfa1`) — Restored default compose files to host networking (Linux default); Windows overrides provided as separate files instead of changing the baseline.

## [3.7.0] - 2026-07-15

Backup and restore release. Adds full database backup/restore management with pg_dump/psql, app shell redesign with faster navigation, and security hardening fixes.

### Added

- **Backup & Restore system** (`a2024cf`) — Complete backup management with pg_dump/psql restore, SHA-256 checksum verification, and auto-pruning of old backups. Adds `backups` table migration (V37) tracking status, type, size, and checksum. Eight API endpoints for listing, creating, downloading, restoring, deleting, uploading backups, and config. Frontend backup page with one-click backup, file upload restore, and full management UI. New env vars: `BACKUP_DIR`, `BACKUP_MAX_BACKUPS`, `BACKUP_RETENTION_DAYS`.
- **docker-compose.dev.yml** — Switched to host networking for all services (`a2024cf`)

### Changed

- **App shell navigation redesign** (`1fdd6bb`) — Redesigned sidebar and top navigation for improved UX
- **App shell performance** (`6ba0359`) — Optimized interactions for faster responsiveness throughout app shell

### Fixed

- **Realtime connection indicator** (`b6fa7b0`) — Fixed connection status display in realtime monitoring views
- **Backup golangci lint issues** (`c629858`) — Resolved linter errors in backup implementation

### Security

- **Go toolchain update** (`883a95c`) — Upgraded to Go 1.26.5 to fix GO-2026-5856 vulnerability
- **Review hardening findings** (`2c78906`) — Implemented recommendations from security review

## [3.6.0] - 2026-07-06

Bug fix and minor feature release. Adds ISP link reports tab, fixes cached database 501 errors, alert badge refresh, ISP modal positioning, location toggle, and fallback roles in user management.

### Added

- **ISP links tab on Reports page** (`4bd3e78`) — New `GET /api/v1/reports/isp-links` endpoint returning per-link SLA metrics (uptime, latency, jitter, packet loss, throughput) for the selected period; new `IspTab` component with sortable table integrated as the fifth tab on the Reports page

### Fixed

- **Alert badge not refreshing on resolve** (`22c2766`, `ceecf89`) — Sidebar badge now listens to `alert:resolved` WebSocket events; count stays accurate when alerts auto-resolve instead of remaining stale
- **CachedDatabase missing ListPhase2Cursor** (`ceecf89`) — Delegates `ListPhase2Cursor` through the caching layer, fixing 501 errors on roles, users, and other Phase2 handler endpoints; also fixes `display_name` JSON tag mismatch in `CreateUser` handler
- **ISP modal positioning and clipping** (`da0cc5b`, `0fe21da`, `9a7ab69`) — Switches to `items-start` with scroll to prevent top clipping, increases modal height to fill available viewport space, updates default app version to 3.5.0
- **Empty role dropdown in user create/edit** (`d0ae7ba`) — Adds fallback role options when the roles API returns an empty list, preventing broken select inputs
- **Location toggle defaulting to disabled** (`2cb7c82`) — Fixes toggle button initial state and slider position

### Changed

- **gofmt formatting in reports.go** (`0c5f9c2`) — Code formatting cleanup in ISP handler

## [3.1.0] - 2026-06-30

Security hardening release implementing comprehensive review_codex.md recommendations. Adds request validation, typed service handlers, HttpOnly cookie auth, audit logging, RBAC boundary tests, CI security scanning, and frontend performance improvements.

### Security

- **Default password scrubbing** — Replaced weak defaults in `.env.example`, `.env.dev.example`, `.env.prod.example` with `CHANGEME_*` placeholders
- **WebSocket scope isolation** — Non-admin users denied by default when connecting with empty scopes; scope resolution requires `pgxpool.Pool`; unresolvable `device_id` returns error; query-string token support removed for browsers
- **Refresh token re-validation** — `Refresh()` re-fetches user from DB, checks `enabled` status, reloads role/permissions via `GetRolePermissions`, issues new token pair with fresh claims
- **Docker production hardening** — `tcpdump` removed from prod image; non-root `netmonitor` user; Redis requires password; TimescaleDB bound to `127.0.0.1`; container gets `cap_drop: ALL`, `no-new-privileges`, `read_only: true`, `tmpfs`
- **Capture disabled by default** — `CAPTURE_ENABLED=false`; quotas: max duration 300s, max packets 10k, max bytes 10MB; returns 403 when disabled
- **Strict CSP** — Removed `'unsafe-inline'` from `script-src` and `style-src`
- **Conditional HSTS** — Only set when `r.TLS != nil` or `X-Forwarded-Proto: https`
- **Security headers** — Added COOP/COOR/COEP headers
- **CORS hardening** — Wildcard `AllowedHeaders` replaced with explicit list
- **Audit logging** — `AuditLog` middleware logs every POST/PUT/DELETE on protected routes with actor, IP, resource type, path
- **Request validation** — `CreateUser` validates username (3-64 chars), password (min 8), role whitelist, email format, display name length; Device CRUD validates protocol whitelist, port range (0-65535), interval non-negative, name ≤255
- **API key permissions** — API key auth loads role permissions via `GetRolePermissions()` and includes them in JWT Claims
- **HttpOnly refresh cookie** — Login sets `HttpOnly + Secure + SameSite=Strict` cookie; frontend `v1` axios instance uses `withCredentials: true`; refresh interceptor sends empty body (cookie handles token); logout clears both cookies

### Added — Backend

- **Cursor-based pagination** — `ListPhase2Cursor` on `Phase2Store` interface; handler accepts `cursor` and `limit` params, returns `{data, next_cursor, has_more}` envelope
- **Typed Role handler** — `RoleHandler` with List/Get/Create/Update/Delete; validates permission names against allowlist of 23 valid permissions; rejects duplicate role names (409); prevents modification/deletion of system roles (`is_system`); prevents deletion of roles assigned to users
- **Typed UserScope handler** — `UserScopeHandler` with List/Create/Delete; validates `scope_type` against enum (`location`/`department`/`device`); rejects duplicate scope assignments (409)
- **Validation helpers** — `IsValidEmail` (regex), `RequiredString`, `InRangeInt`, `ValidationError` type in `httputil/response.go`

### Added — CI/CD

- **gitleaks secret scanning** — `gitleaks/gitleaks-action@v2` with `fetch-depth: 0`
- **Trivy container scanning** — `aquasecurity/trivy-action@master` after Docker build, fails on CRITICAL/HIGH
- **Bundle budget check** — Warns on individual chunks >500KB JS / 100KB CSS; fails on total JS >1200KB
- **golangci-lint v2** — Config migrated to v2 format; CI installs v2

### Added — Tests

- **RBAC boundary tests** — `TestRequirePermission_HTTP` (6 cases) and `TestRequireAnyPermission_HTTP` (5 cases)

### Performance

- **Material Symbols font replacement** — Switched from `material-symbols` (3.93MB) to `@fontsource-variable/material-symbols-outlined` (727KB, 81% reduction); Latin subset variable font

### Changed

- **API standardization** — All frontend API calls migrated from `/api/*` to `/api/v1/*`; legacy `/api/*` route aliases removed from backend; frontend `api` axios instance removed entirely
- **golangci-lint config** — Migrated to v2 format; `gosimple` removed (merged into `staticcheck` in v2)
- **CI lint install** — Updated to install golangci-lint v2

### Fixed

- **Double-protocol prefix** — HTTP device URLs no longer produce `http://http://...`
- **Login redirect loop** — Prevents redirect on missing `/auth/permissions` endpoint
- **Port state change detection** — Implemented with trend delta formatting to 2 decimal places
- **Sidebar navigation** — Simplified with collapsible sections
- **Scrollbar visibility** — Low-priority Safari fallback fixes
- **Accessibility** — Critical, high, and medium priority accessibility, color token, and responsive fixes
- **CSS var tokens** — Updated tests for CSS custom properties and WCAG AA color compliance
- **Lint error** — Fixed App.tsx lint error

### Dependencies

- `golang.org/x/net` v0.54.0 → v0.56.0 (fixes 5 HIGH CVEs: CVE-2026-25680, CVE-2026-25681, CVE-2026-27136, CVE-2026-39821, CVE-2026-42502)

### Removed

- Redundant review spec (`documentation/review_codex.md`)
- Legacy `api` axios instance from frontend
- `tcpdump` from production Docker image
- `coverage.out` and `coverage.html` from git tracking

## [3.5.0] - 2026-07-02

Frontend redesign with new sage-charcoal dark theme, operational logs system with verbose sessions, and security dependency fix.

### Added — Frontend

#### Complete UI Redesign
- **Sage-charcoal dark theme** — New color palette with sage greens and charcoal backgrounds replacing previous dark theme
- **Redesigned components** — Updated Button, Card, Modal, Toast, EmptyState, ErrorState, LoadingState, SectionHeader, and StatCard styling
- **New brand assets** — Added SVG logo lockups and icons (color and invert variants)
- **Fontsource variable fonts** — Replaced Material Symbols font (3.93MB) with fontsource-variable (727KB) for reduced bundle size

#### Logs Page (`/logs`)
- **Queryable log viewer** — Search operational events with filters for time range, level, component, request ID, device ID, and free-text search
- **Log statistics dashboard** — Event counts, error counts, slow API/DB request metrics, and active verbose session counts
- **Verbose session management** — Enable scoped debug/trace logging per component, device, and user with configurable duration
- **Live auto-refresh** — Toggle between manual and 10-second auto-refresh polling
- **Log export** — Download filtered log data as CSV
- **Detail panel** — Slide-out panel showing full event context, attributes, and error traces

### Added — Backend

#### Operational Logs System
- **Structured log storage** — `LogStore` with PostgreSQL-backed queryable log events (level, component, message, context, attributes)
- **Async log sink** — `AsyncLogSink` for non-blocking log ingestion with request context propagation
- **Verbose session API** — CRUD endpoints for creating, listing, and stopping verbose logging sessions with expiry
- **Log query API** — `GET /api/v1/logs` with filters for level, component, time range, request ID, device ID, and text search
- **Log statistics API** — Aggregated stats by level, component, and error counts
- **Database migration** — `operational_logs` and `verbose_log_sessions` tables

### Changed
- **Frontend color tokens** — Complete palette overhaul: surface, primary, tertiary, error, outline, and chart colors updated across all 25+ pages
- **Login page redesign** — Updated to match new sage-charcoal theme
- **Dashboard widgets** — Active alerts, AI health score, response time charts, and resource load cards restyled

### Fixed
- **AsyncLogSink context** — Uses request context instead of `context.Background()` for proper trace propagation
- **golangci-lint v2 path** — Resolved linter configuration path issue

### Security
- **golang.org/x/net upgrade** — Updated v0.54.0 → v0.56.0 to fix 5 HIGH CVEs

### Removed
- Redundant design spec document (`documentation/frontend_spec.md`)
- Previous Material Symbols font dependency

## [3.0.0] - 2026-06-25

Major release transforming Rayavriti NetMonitor from a generic network monitor into a purpose-built campus network monitoring platform. Adds 12 new backend features, 12 new frontend pages, and comprehensive testing.

### Added — Backend

#### Location Hierarchy & Campus Topology
- **Location tree** — Recursive campus → building → floor → room → rack hierarchy with JSONB metadata
- **Location CRUD** — Create, update, delete locations with parent-child relationships and device counts
- **Device-location assignment** — Assign devices to locations; view assigned devices per location
- **Campus topology overview** — Aggregate device status across all locations

#### Dependency Tree & Alert Suppression
- **Device dependency tree** — Map parent-child device relationships (e.g., switch → AP)
- **Alert suppression** — When a parent device goes down, child alerts are automatically suppressed
- **Root-cause analysis** — Single root-cause alert fires instead of hundreds of individual alerts
- **Suppression audit trail** — All suppressed alerts logged for post-incident review

#### Auto-Discovery Scanner
- **Subnet scanner** — ICMP sweep, ARP lookup, OUI manufacturer identification, TCP port scanning
- **Device role heuristic** — Auto-detects device type (router/switch/workstation/printer/CCTV) based on open ports
- **SNMP probe** — Detects SNMP-reachable devices and extracts sysDescr/sysName
- **Device identification enrichment** — HTTP title, SSH banner, TLS certificate CN extraction
- **Discovery results** — Scan results stored for admin approval before adding to monitoring

#### Public Status Page
- **Standalone status page** — Server-rendered HTML at `/status` with auto-refresh
- **Service groups** — Organize services into collapsible groups
- **Incident announcements** — Publish active incidents with status updates
- **Admin UI** — Configure services, groups, and incidents from the dashboard

#### Maintenance Windows
- **Recurring maintenance** — Daily, weekly, or custom recurring schedules
- **One-time maintenance** — Scheduled downtime windows
- **Alert suppression during maintenance** — Alerts silenced for devices in active maintenance windows
- **Scope-based** — Apply to device, location, subnet, or global

#### Contacts & Escalation
- **Contact directory** — Name, designation, department, email, phone, notification preferences
- **Device/location contacts** — Assign contacts to specific devices or locations
- **Escalation policies** — Multi-step escalation with configurable delays
- **On-call rotation** — Schedule-based on-call assignments
- **Quiet hours** — Suppress notifications during configured hours
- **Telegram bot integration** — Interactive acknowledge/resolve via Telegram messages

#### Incident Management
- **Incident lifecycle** — Open → Investigating → Identified → Fixing → Monitoring → Resolved → Closed
- **Timeline entries** — Every status change, note, and action logged with timestamps
- **SLA breach detection** — Configurable thresholds (Critical: 15min/2hr, Major: 30min/8hr, Minor: 1hr/24hr)
- **Device association** — Link incidents to affected devices

#### RBAC (Role-Based Access Control)
- **18 permission types** — CRUD for devices, alerts, metrics, flows, capture, dashboards, users, reports
- **5 seeded roles** — super_admin, network_admin, dept_admin, viewer, public
- **Scope-based filtering** — dept_admin only sees their department's devices
- **WebSocket event filtering** — RBAC enforced on realtime traffic events
- **Permission-guarded API routes** — Middleware enforces permissions per endpoint
- **Permission editor UI** — Role CRUD with grouped permission checkboxes

#### ISP Link Monitoring
- **ISP link CRUD** — Add ISP links with provider, bandwidth, circuit ID, monthly cost
- **SLA compliance tracking** — Monthly uptime, latency, jitter, packet loss metrics
- **Metrics time series** — Historical latency/jitter/packet loss/throughput charts
- **ISP comparison view** — Compare performance across multiple ISP links
- **Background collector** — Ping-based latency/jitter/packet loss + HTTP throughput measurement via Cloudflare speed test endpoints

#### Reporting Engine
- **HTML/CSV report generation** — Per-building uptime, incident breakdown, SLA compliance
- **Scheduled reports** — Cron-based delivery via email or file output
- **On-demand reports** — Generate reports from the UI
- **Report templates** — Uptime, incident, performance, and ISP reports

#### College Service Templates
- **13 pre-built templates** — College ERP, Moodle LMS, Email Server, DNS, LDAP/RADIUS, Proxy, CCTV, Biometric, UPS, Printer, WiFi Controller, File Server, Database Server
- **One-click application** — Templates create devices, sensors, and alert rules automatically

#### Bulk Device Import
- **CSV import** — Parse CSV with validation (IP format, required fields, duplicate detection)
- **Location code resolution** — Auto-resolve location codes to IDs
- **Dry-run preview** — Review import results before confirming

#### Additional Backend Features
- **Cached database wrapper** — `PoolProvider` interface with transparent caching layer
- **TimescaleDB hypertables** — metrics, flows, suppressed_alerts, notification_log, incident_timeline, isp_metrics
- **Retention policies** — Automatic `drop_chunks` with TimescaleDB 2.x compatibility
- **Telegram long-polling mode** — Recommended for college networks behind NAT
- **Device update partial merge** — Prevents blanking fields on partial updates
- **golangci-lint clean** — 0 issues across all 20 backend packages

### Added — Frontend

#### New Pages (12)
| Page | Route | Description |
|---|---|---|
| Campus Overview | `/campus` | Location tree with device status drill-down |
| Location Manager | `/settings/locations` | CRUD for location hierarchy with assigned devices |
| Incidents | `/incidents` | Active incidents list, create workflow, filters |
| Incident Detail | `/incidents/:id` | Timeline view, status transitions, resolve workflow |
| Contacts | `/settings/contacts` | Contact CRUD, notification preferences, device assignment |
| Status Page Admin | `/settings/status-page` | Configure services, groups, public incidents |
| Maintenance | `/maintenance` | Maintenance window CRUD with schedule/recurring options |
| User Management | `/settings/users` | User CRUD, role assignment, RBAC Permission Editor |
| Report Builder | `/reports/builder` | Scheduled report configuration, on-demand generation |
| Discovery | `/discovery` | Launch subnet scans, review results, approve/reject devices |
| ISP Dashboard | `/isp` | ISP link comparison, SLA metrics, detail modal with charts |
| Bulk Import | `/import` | CSV upload, validation preview, confirm import |

#### New Components
- **ISPLinkModal** — ISP link detail with SLA compliance, latency/jitter/packet loss/throughput charts
- **LocationTree** — Recursive tree rendering with device counts and status indicators
- **ConfirmDialog** — Accessible alert dialog with focus trapping
- **DeviceModal** — Device detail with location assignment, Phase 2 metadata fields
- **ResourceLoadModal** / **ExpandedChartsModal** — System analytics and chart expansion

#### Design System
- Dark theme (`#0e0e09` background, `#d9fd3a` lime accent)
- League Spartan headings, Space Grotesk body text
- Material Symbols Outlined icons
- Permission-guarded sidebar navigation
- Modal flex layout with proper centering and overflow handling

#### Frontend Quality
- **101 tests** across 8 test suites (vitest)
- **ESLint clean** — 0 errors
- **TypeScript strict** — Clean `tsc -b` compilation
- **golangci-lint v1 compatible** — Config works with both v1 and v2

### Changed
- **Device struct expanded** — +11 columns (location_id, parent_device_id, device_category, manufacturer, model, serial_number, mac_address, asset_tag, rack_position, dependency_port, notes)
- **Location struct snake_case** — JSON tags changed to `parent_id`, `floor_number`, `device_count`
- **Metric fields always present** — `Protocol` and `DeviceName` without `omitempty`
- **Device update uses fetch-then-merge** — Partial PUT requests no longer zero out all fields
- **Sidebar z-index lowered** — Header at z-40 so modals at z-50 render above
- **Modal centering** — All 17 modals use `pt-20` + flex-col layout for proper viewport centering
- **CSS animation fix** — Removed `transform` from `page-enter` to prevent containing block for `position: fixed`
- **ISP collector uses pure Go** — HTTP-based throughput measurement (Alpine container has no curl)
- **Database versioned migrations** — Layered on Phase 1 schema, additive/non-destructive only

### Fixed
- Location NULL string scanning — `code`, `description`, `address` scan as `*string`
- Location metadata NULL handling — `json.RawMessage` with local `[]byte` scan
- Discovery job NULL scan — `InitiatedBy` and `ErrorMessage` as `*string`
- Discovery scan goroutine context cancellation — Uses `context.Background()`
- Discovery page camelCase — Frontend matches Go JSON tags
- Phase2Store 501 — `PoolProvider` interface pattern replaces `*database.Postgres` type assertions
- TimescaleDB `drop_chunks` — Fixed signature from 3-arg to 2-arg form; added `hasTimescaleDB` flag
- ISP timeseries 500 — Scan `timestamptz` into `time.Time`, format as RFC3339
- ISP collector Alpine ping — Regex handles `round-trip min/avg/max` (3 fields, no mdev)
- Modal headers cut off — All modals use `shrink-0` header + `flex-1 min-h-0` content
- Modal z-index conflict — Header lowered to z-40, modals stay at z-50
- CSS transform fixed-position bug — Removed `transform` from `@keyframes page-enter`
- Modal centering — `pt-20` on overlay compensates for 64px fixed header
- ESLint `set-state-in-effect` — 12 page files refactored to use inline async IIFE pattern
- ESLint `no-explicit-any` — Campus.tsx and ServiceTemplates.tsx typed properly

### Removed
- Redundant frontend spec (`documentation/frontend_spec.md`)
- Compiled binary from git tracking (`backend/bin/netmonitor`)

## [2.5.0] - 2026-06-18

### Added
- **Expanded color palette** — 14 new tokens: warning, on-warning, success, on-success, info, on-info, chart-1–8
- **SectionHeader component** — Consistent page headers with title, subtitle, and optional action
- **StatCard component** — Reusable metric cards with icon, label, value, and trend
- **Flat design system** — No shadows, glows, gradients, or glass effects on containers
- **Professional copy** — Removed military/surveillance language across all pages

### Changed
- **Typography scale** — 5 standardized sizes: text-2xl (title), text-base (section), text-sm (body), text-xs (labels), text-[11px] (metadata)
- **Border radius** — rounded-lg (8px) for cards/inputs, rounded-md (6px) for buttons/badges
- **Button sizing** — Larger text (text-sm), proper padding (px-5 py-2.5), flat hover states
- **Sidebar navigation** — Bigger text (text-sm), improved readability and spacing
- **Score display** — All health scores standardized to 2 decimal places
- **Chart colors** — All hex values synced to new theme tokens (info, success, warning, chart palette)
- **Status colors** — Raw amber-400/amber-500 replaced with theme warning token
- **Error color** — Unified to single red (#ff7351), removed conflicting #ff4444

### Fixed
- AlertTab info state now uses theme info token (was raw sky-400)
- All hover states use background-color transitions (removed brightness filters)
- Removed animate-pulse from loading text indicators
- Removed active:scale-95 from all buttons
- Score gauges no longer use drop-shadow filters

### Removed
- Utility classes: .neon-glow, .glass-panel, .glass-panel-light, .ambient-glow-primary
- Utility classes: .glow-healthy, .glow-watch, .glow-risk, .glow-critical
- Utility classes: .particle-bg, .geometric-input
- Font-black weight (replaced with font-bold)
- Tracking-widest and tracking-[0.2em] (max tracking-wide)
- Shadow effects from all containers and modals
- Backdrop-blur from modals and toasts

## [2.2.0] - 2026-06-15

### Added
- **Alert grouping** — Collapsible alert groups with `group_id` (rule + 60s window) and `/api/alerts/grouped` endpoint
- **Grouped alerts view** — Alerts page toggle between grouped/list views with `AlertGroupCard` component
- **AI Health Score persistence** — Weighted composite scores (Availability 30%, Latency 25%, Alerts 20%, Stability 15%, Ports 10%) saved to `health_scores` and `health_score_history` tables
- **Health score history** — `/api/insights/history` returns network-wide score timeline for trend graphs
- **Absence monitoring** — Alert rule condition that fires when a device stops reporting for a configurable duration
- **Baseline cache** — 15-minute TTL cache for anomaly detection baselines, avoids repeated DB queries
- **Database V33 migration** — `health_scores`, `health_score_history` tables and `alerts.group_id` column

### Changed
- **Alert engine rewritten** — Real anomaly detection using z-score with configurable standard deviation threshold, real absence detection, and port state evaluation
- **Alert messages are contextual** — Rich descriptions like "Latency spike: 245ms (baseline 82ms, +199%)" instead of generic "Anomaly detected"
- **Health scorer runs every 2 minutes** — Down from 5 minutes; scores persisted to DB instead of discarded
- **Anomaly engine decoupled** — Uses `HealthScorer` and `BaselineCache` instead of computing inline
- **Frontend insights API simplified** — Single `/api/insights/current` call replaces client-side score fabrication
- **Resource load card uses real telemetry** — Dashboard card now shows actual server CPU/memory from `/v1/system/info` instead of synthetic heuristics

### Fixed
- Frontend crash when `issues` or `factors` are null from backend JSONB
- Trend delta showing floating point noise (e.g. `-19.17999954223633` → `-19.18`)
- All scores on AI Health page now display with 2 decimal places
- Dashboard resource load widget showing wrong data (was using synthetic heuristics instead of real server metrics)

### Removed
- Client-side health score fabrication in `insights.ts` (~100 lines replaced with ~10 lines)

## [2.0.0] - 2026-06-13

### Added
- **Go backend** — Complete rewrite from Node.js to Go 1.26 with go-chi router
- **PostgreSQL + TimescaleDB** — Replaced SQLite with TimescaleDB hypertables for time-series data
- **Redis caching layer** — Optional Redis integration with metric buffering, distributed locks, pub/sub, and rate limiting
- **Anomaly engine health scores** — Device health scoring based on metrics, alerts, and staleness
- **Monitoring log volume stats** — Aggregated stats by component, status, and hour
- **CI/CD pipeline** — GitHub Actions with lint, test, security scan, build, and Docker image publish
- **golangci-lint** — Comprehensive Go linter configuration
- **Frontend typecheck** — Dedicated `npm run typecheck` script
- **Docker image publishing** — Automatic GHCR image push on main branch
- **CHANGELOG** — This file

### Changed
- Bumped version to 2.0.0
- Production sourcemaps disabled by default
- CI now runs Go tests, frontend lint, and security scans before build
- Health scores now compute real device health based on response time, packet loss, and metric staleness

### Fixed
- Health score computation now runs on a 5-minute interval (was a no-op TODO)
- Log volume stats endpoint returns real aggregated data (was a stub)

### Removed
- Stale `implementation_plan_redis.md` (already implemented)
- Compiled binary from git tracking (`backend/bin/netmonitor`)

## [1.0.0] - 2026-01-01

### Added
- Initial release with Node.js backend
- React 19 SPA with Redux Toolkit and Recharts
- Real-time WebSocket monitoring
- Multi-protocol device monitoring (Ping, HTTP, HTTPS, TCP, SNMP, System)
- Packet capture with protocol analysis
- NetFlow v5/v9 and sFlow collection
- Alert management with severity-based workflow
- JWT authentication with scrypt password hashing
- Docker Compose deployment
- Network device simulator
