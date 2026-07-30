<div align="center">
  <h1>Rayavriti NetMonitor</h1>
  <p><strong>Production-grade, real-time network monitoring and traffic visibility platform.</strong></p>

  ![Version](https://img.shields.io/badge/Version-4.0.0-brightgreen?style=flat-square)
  ![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)
  ![React](https://img.shields.io/badge/React-19-blue?style=flat-square&logo=react)
  ![TypeScript](https://img.shields.io/badge/TypeScript-Strict-blue?style=flat-square&logo=typescript)
  ![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?style=flat-square&logo=postgresql)
  ![TimescaleDB](https://img.shields.io/badge/TimescaleDB-Hypertables-FBB040?style=flat-square)
  ![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker)
  ![License](https://img.shields.io/badge/License-Proprietary-red?style=flat-square)
</div>

---

Rayavriti NetMonitor is a full-stack network monitoring platform built for local infrastructure visibility. It provides real-time device monitoring, packet capture, NetFlow/sFlow analysis, AI-powered anomaly detection, campus topology management, remote fleet coordination, and a modern infographic-driven SPA dashboard—all deployable via Docker Compose or bare metal.

Designed for colleges, schools, offices, hostels, labs, and small campuses as a PRTG-inspired alternative with no per-device licensing.

---

## Features

- **Real-Time Dashboard** — Live metrics, alerts, device status, and AI health scores via WebSockets
- **Multi-Protocol Monitoring** — Ping (ICMP), HTTP/HTTPS, TCP port, SNMP (v1/v2c/v3), and system metrics (CPU/memory/disk)
- **NetFlow/sFlow Analysis** — NetFlow v5/v9 and sFlow collection with top-talker, protocol breakdowns, Sankey diagrams, and treemaps
- **Packet Capture** — Real-time packet sniffing with protocol analysis (requires `libpcap` + `CAP_NET_RAW`)
- **AI Health Scoring** — Anomaly detection engine with z-score analysis, health scoring (availability/latency/alerts/stability/ports), and baseline cache
- **Alert Engine** — Rule evaluation with severity-based alerts, acknowledge/resolve workflow, suppression rules, escalation policies, and multi-channel notifications
- **Campus Topology** — Hierarchical location tree (campus/building/floor/room/rack), force-directed topology graph with real dependency edges, floor plan and rack views
- **Auto-Discovery** — Subnet ICMP sweep, ARP lookup, OUI manufacturer identification, port scanning, SNMP probing, HTTP/SSH/TLS banner extraction
- **Remote Fleet Management** — Centralized registry for multiple NetMonitor instances with encrypted API key storage, service modes (active/readonly/maintenance), and config sync
- **RBAC** — 5 seeded roles (super_admin, network_admin, dept_admin, viewer, public) with 18 permission types and scope-based filtering
- **Incident Management** — Full incident lifecycle with timeline tracking and SLA monitoring
- **Reports & Export** — Time-series, device, alert, and ISP SLA reports in HTML/CSV with scheduled cron-based generation
- **Backup & Restore** — pg_dump/psql management with SHA-256 checksums and auto-pruning
- **Service Templates** — 13 pre-built college service templates (ERP, LMS, Email, DNS, CCTV, etc.)
- **Maintenance Windows** — One-time and recurring maintenance scheduling with auto-snooze

---

## Architecture

```
                           React 19 SPA (Vite + Tailwind v4)
              Redux Toolkit • Recharts • @visx • framer-motion • WebSocket
                                    |
                          REST + WebSocket (gorilla/ws)
                                    |
                        Go Backend (go-chi v5 + pgx v5)
     ┌───────────┐ ┌───────────┐ ┌──────────┐ ┌────────────┐ ┌──────────┐
     │  Polling  │ │Collectors │ │  Engine  │ │  Campus &  │ │  Remote  │
     │  Engine   │ │ping/http  │ │  Alert   │ │ Discovery  │ │  Fleet   │
     │ WorkerPool│ │snmp/port  │ │ Anomaly  │ │  Topology  │ │ Registry │
     │Dispatcher │ │netflow    │ │ Health   │ │   Import   │ │  Config  │
     │ResultPipe │ │capture    │ │Notifier  │ │            │ │   Sync   │
     └───────────┘ └───────────┘ └──────────┘ └────────────┘ └──────────┘
     ┌───────────┐ ┌───────────┐ ┌──────────┐ ┌────────────┐
     │  Auth &   │ │    RBAC   │ │ WebSocket│ │  Reports & │
     │   2FA     │ │   ACL     │ │   Hub    │ │   Backup   │
     └───────────┘ └───────────┘ └──────────┘ └────────────┘
                                    |
                    ┌───────────────┴───────────────┐
                    │       PostgreSQL 16 + TimescaleDB     │
                    │   Hypertables • Retention Policies    │
                    └───────────────────────────────────────┘
                                    |
                          Redis 7 (optional)
                    Cache • Pub/Sub • Rate Limiting • Locks
```

**Monorepo** using npm workspaces:

```
rayavriti-netmonitor/
├── backend/           # Go backend (24 internal packages)
│   ├── cmd/server/    # Application entry point
│   └── internal/      # auth, cache, collectors, campus, config, database,
│                      # discovery, engine, handlers, rbac, remote, reports,
│                      # scheduler, scanner, websocket, logging, backup, etc.
├── client/            # React SPA (30 pages, 40+ components)
│   └── src/
│       ├── api/       # 15 API client modules
│       ├── components/# Charts, dashboard widgets, UI library
│       ├── pages/     # 30 route-driven page components
│       └── store/     # Redux Toolkit state
├── simulator/         # Network device simulator for testing
├── documentation/     # API docs, deployment guide, specs
├── Dockerfile         # 5-stage multi-stage build
├── docker-compose.yml # Production orchestration
└── bootstrap.sh       # One-curl-bootstrap installer
```

---

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| **Go** | 1.26+ | Backend runtime |
| **Node.js** | 22.x / 24.x / 26.x | Frontend build |
| **npm** | 9+ | Comes with Node.js |
| **libpcap** | — | Required for packet capture (`apt install libpcap-dev`) |
| **Docker** (optional) | 24+ | For containerized deployment |

---

## Quick Start

### One-Line Install (any system)

```bash
curl -fsSL https://raw.githubusercontent.com/YuvrajS01/rayavriti_netmonitor/main/bootstrap.sh | bash
```

Interactive prompts guide you through prerequisites, dev/prod mode, and Docker/bare-metal selection.

### Docker Production

```bash
git clone <repository-url>
cd rayavriti-netmonitor
cp .env.example .env
# Edit .env — set JWT_SECRET and ADMIN_PASSWORD
docker compose up -d
# Open http://localhost:3000
```

### Docker Development

```bash
git clone <repository-url>
cd rayavriti-netmonitor
cp .env.dev.example .env.dev
docker compose -f docker-compose.dev.yml up --build
# Frontend: http://localhost:5173  Backend: http://localhost:3000
```

### Bare Metal

```bash
git clone <repository-url>
cd rayavriti-netmonitor
npm install --workspace client
cd backend && make build && cd ..
cp .env.example .env
# Edit .env — set JWT_SECRET and ADMIN_PASSWORD
./backend/bin/netmonitor
```

> **Note:** Packet capture and SNMP require root or `CAP_NET_RAW`:
> ```bash
> sudo setcap cap_net_raw+ep ./backend/bin/netmonitor
> ```

**Default dev credentials:** `admin` / `admin123` (only when `ADMIN_PASSWORD` is not set)

---

## Scripts

| Command | Description |
|---|---|
| `npm run setup` | One-command Docker dev setup |
| `npm run setup:prod` | One-command Docker prod setup |
| `npm run dev` | Start Go backend with hot reload (air) |
| `npm run dev:client` | Start Vite dev server |
| `npm run build` | Build Go backend + React client for production |
| `npm run start` | Start production server |
| `npm run simulate` | Run network device simulator |
| `cd backend && make test` | Run Go tests |
| `cd backend && make lint` | Run golangci-lint |
| `npm run lint -w client` | Run ESLint on client |
| `npm run typecheck -w client` | Run TypeScript type checking |

---

## Configuration

All configuration via environment variables. See [`.env.example`](.env.example) for the full list.

### Required (Production)

| Variable | Description |
|---|---|
| `JWT_SECRET` | JWT signing secret — minimum 32 chars (`openssl rand -base64 32`) |
| `ADMIN_PASSWORD` | Admin password — hashed with scrypt on first boot |

### Key Optional Variables

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | Set to `production` for production mode |
| `PORT` | `3000` | HTTP server port |
| `DATABASE_URL` | — | PostgreSQL connection string |
| `REDIS_URL` | — | Redis connection string (optional) |
| `NETFLOW_PORT` | `2055` | UDP port for NetFlow/sFlow collector |
| `POLLER_WORKER_COUNT` | `10` | Worker pool size for device polling |
| `POLLER_RESULT_BATCH_SIZE` | `50` | Batch flush size for metric writes |
| `METRICS_RETENTION_DAYS` | `30` | Auto-delete metrics older than N days |
| `FLOW_RETENTION_DAYS` | `7` | Auto-delete flow records older than N days |
| `ALERTS_RETENTION_DAYS` | `90` | Auto-delete resolved alerts older than N days |
| `CAPTURE_ENABLED` | `true` | Enable packet capture feature |

---

## Polling Engine (v4.0)

## Security devices and vendor identification

Rayavriti NetMonitor has first-class profiles for IP cameras/NVRs and biometric
attendance terminals. Choose **CCTV Camera / NVR** or **Biometric terminal**
when adding a device; the form records only non-secret endpoint settings.

- Cameras check the management UI and send an RTSP `OPTIONS` request. HTTP
  `401`/`403` and RTSP `401` responses count as healthy because the service is
  reachable and correctly requires authentication.
- Biometric terminals check their management UI and the attendance port,
  defaulting to the ZKTeco-compatible port `4370`. A single failed endpoint is
  reported as a warning; both unavailable is down.
- Discovery identifies vendors from SNMP enterprise OIDs and descriptions,
  management-page/TLS/SSH fingerprints, then MAC OUI as a fallback. Native
  device fingerprints take precedence over MAC ownership, which avoids common
  false positives from virtualized or rebranded hardware.

`monitorConfig` is returned with a device and supports `managementScheme`,
`managementPath`, `managementPort`, `rtspPort`, `rtspPath`, and
`attendancePort`. Do not place passwords or stream credentials in it.

The core polling engine was rewritten in v4.0.0 for production-scale reliability:

| Component | Purpose |
|---|---|
| **WorkerPool** | Fixed-size goroutine pool with 3 priority queues (critical/normal/low) — prevents runaway goroutine growth |
| **PollDispatcher** | Timing wheel with min-heap scheduling — accurate interval-based dispatch without busy-waiting |
| **ResultPipeline** | Fan-in batch buffer with size+time flush thresholds, uses `pgx.CopyFrom` for bulk COPY inserts |
| **DeviceStateTracker** | Per-device health tracking with adaptive backoff (1x → 2x → 4x → 8x interval escalation on failure) |
| **DependencyTree** | PRTG-inspired parent/child model — auto-pauses dependents when parent is unreachable |

---

## API

The server exposes REST APIs at `/api` (legacy) and `/api/v1` (current), plus WebSocket at `/ws`.

**Authentication:** `Authorization: Bearer <token>` or `X-Api-Key: <key>` header.

See `documentation/api_documentation.md` and `documentation/postman_guide.md` for full reference.

### Key Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/auth/login` | Authenticate and get JWT tokens |
| `GET` | `/api/devices` | List all monitored devices |
| `POST` | `/api/devices` | Add a new device |
| `GET` | `/api/metrics/latest` | Get latest metrics per device |
| `GET` | `/api/alerts` | List alerts |
| `GET` | `/api/stats` | Dashboard statistics |
| `GET` | `/api/v1/flows` | Query flow records (NetFlow/sFlow) |
| `POST` | `/api/v1/capture/start` | Start packet capture session |
| `GET` | `/api/v1/health/scores` | AI-powered health scores |
| `GET` | `/api/v1/topology` | Dependency tree topology |
| `GET` | `/api/v1/campus` | Campus hierarchy |
| `GET` | `/api/v1/remote` | Remote instance registry |
| `POST` | `/api/v1/backup` | Trigger database backup |
| `GET` | `/health` | Service health check |

---

## Database

**PostgreSQL 16 + TimescaleDB** with automated retention policies.

### Hypertables

| Table | Time Column | Purpose |
|---|---|---|
| `metrics` | `timestamp` | Device metrics (ping, HTTP, SNMP, etc.) |
| `flows` | `created_at` | NetFlow/sFlow records |
| `capture_packets` | `timestamp` | Captured packet data |
| `alert_history` | `created_at` | Alert lifecycle events |
| `remote_snapshots` | `created_at` | Remote instance snapshots |

### Retention (auto-prunes every 6h)

- **Metrics:** 30 days
- **Flow records:** 7 days
- **Resolved alerts:** 90 days

---

## Security

- **Password hashing:** scrypt with random 32-byte salt (backward-compatible with legacy SHA-256)
- **JWT authentication:** HS256 tokens (15-min access / 7-day refresh) with HttpOnly cookie support
- **API key auth:** Alternative bearer token for headless/script access
- **2FA support:** Two-factor authentication
- **RBAC:** 5 roles, 18 permissions, scope-based filtering on all data queries
- **Security headers:** CSP (no unsafe-inline in prod), HSTS, COOP, COOR, COEP
- **Rate limiting:** Per-IP with Redis-backed sliding window (falls back to in-memory)
- **Audit logging:** All auth and admin actions logged to `audit_log` table
- **Structured logging:** Go slog with DB, file rotation (lumberjack), and WebSocket sinks

---

## Docker Deployment

```bash
# Configure production environment
cp .env.prod.example .env
docker compose up -d
```

**Important notes:**
- `network_mode: host` — Required for ping, SNMP, and packet capture
- `cap_add: NET_RAW, NET_ADMIN` — Required for raw socket access
- PostgreSQL + TimescaleDB data persists in `postgres_data` volume
- Windows/macOS overrides available in `docker-compose.windows.yml`

---

## Branching Model

| Branch | Purpose |
|---|---|
| `main` | Production — always deployable |
| `develop` | Development integration |
| `feature/<name>` | New features, branched from `develop` |
| `fix/<name>` | Non-urgent fixes, branched from `develop` |
| `release/<version>` | Stabilization before merging to `main` |
| `hotfix/<name>` | Urgent production fixes, branched from `main` |

---

## Contributing

1. UI additions must follow the **sage-charcoal dark theme** design language
2. Backend services should integrate with the WebSocket event-driven architecture
3. TypeScript strict mode is enabled; Go code must pass golangci-lint
4. Create feature branches, never commit directly to `main`
5. Run `make test` + `make lint` in backend, and `npm run lint -w client` before submitting PRs

---

## License

**Proprietary** — All rights reserved.
