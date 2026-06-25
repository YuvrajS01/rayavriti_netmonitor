# Changelog

All notable changes to Rayavriti NetMonitor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
