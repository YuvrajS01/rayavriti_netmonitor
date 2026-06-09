<div align="center">
  <h1>🌐 Rayavriti NetMonitor</h1>
  <p><strong>Production-grade, real-time network monitoring and traffic visibility platform.</strong></p>

  ![Version](https://img.shields.io/badge/Version-1.0.0-brightgreen?style=flat-square)
  ![Node.js](https://img.shields.io/badge/Node.js-v22+-green?style=flat-square&logo=node.js)
  ![React](https://img.shields.io/badge/React-v19-blue?style=flat-square&logo=react)
  ![TypeScript](https://img.shields.io/badge/TypeScript-Strict-blue?style=flat-square&logo=typescript)
  ![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat-square&logo=docker)
  ![License](https://img.shields.io/badge/License-Proprietary-red?style=flat-square)
</div>

---

Rayavriti NetMonitor is a full-stack network monitoring platform built for local infrastructure visibility. It provides real-time device monitoring, packet capture, NetFlow/sFlow analysis, AI-powered anomaly detection, and a modern SPA dashboard — all deployable as a single Docker container.

## ✨ Features

- ⚡ **Real-Time Dashboard** — Live metrics, alerts, and device status via WebSockets
- 🔍 **Packet Capture** — Real-time packet sniffing with protocol analysis (requires `libpcap`)
- 📊 **Flow Analysis** — NetFlow v5/v9 and sFlow collection with top-talker and protocol breakdowns
- 🤖 **AI Health Scoring** — Anomaly detection engine with historical trend analysis
- 🏥 **Multi-Protocol Monitoring** — Ping (ICMP), HTTP, TCP port, SNMP, and system metrics
- 🔔 **Alert Management** — Severity-based alerts with acknowledge/resolve workflow
- 📈 **Reports & Export** — Time-series reports, device breakdowns, and CSV export
- 🔒 **Authentication** — JWT-based auth with scrypt password hashing and API key support
- 🐳 **Docker-Ready** — Single-container deployment with Docker Compose

---

## 🏗️ Architecture

```text
┌─────────────────────────────────────────────────────────┐
│                    React 19 SPA                         │
│  Redux Toolkit • Recharts • Socket.IO Client • Vite    │
└────────────────────────┬────────────────────────────────┘
                         │ WebSocket + REST API
┌────────────────────────▼────────────────────────────────┐
│                  Go Backend (go-chi)                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌───────────┐  │
│  │ Scheduler│ │Collectors│ │ Anomaly  │ │ Retention │  │
│  │  (cron)  │ │ping/http │ │  Engine  │ │ Scheduler │  │
│  │          │ │snmp/port │ │  (AI)    │ │ (pruning) │  │
│  └──────────┘ └──────────┘ └──────────┘ └───────────┘  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                │
│  │ NetFlow  │ │  Packet  │ │   Flow   │                │
│  │Collector │ │ Capture  │ │ Analyzer │                │
│  └──────────┘ └──────────┘ └──────────┘                │
└────────────────────────┬────────────────────────────────┘
                         │
            ┌────────────▼────────────┐
            │  PostgreSQL + TimescaleDB │
            │  Hypertables + Retention │
            └─────────────────────────┘
```

**Monorepo** using npm workspaces:
```
rayavriti-netmonitor/
├── client/           # React frontend (Vite + Tailwind CSS v4)
├── backend/          # Go backend (go-chi + pgx + TimescaleDB)
│   ├── cmd/server/   # Application entry point
│   └── internal/     # Handlers, database, models, collectors
├── simulator/        # Network device simulator for testing
├── documentation/    # Product specs, API docs, deployment guide
├── Dockerfile        # Multi-stage production build
├── docker-compose.yml
└── .env.example      # Configuration template
```

---

## ⚙️ Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| **Go** | 1.26+ | Backend runtime |
| **Node.js** | 22.x | Frontend build (defined in `engines`) |
| **npm** | 9+ | Comes with Node.js |
| **libpcap** | — | Required for packet capture (`apt install libpcap-dev`) |
| **Docker** (optional) | 24+ | For containerized deployment |

---

## 🚀 Quick Start

### Option 1: Docker (Recommended for Production)

```bash
# 1. Clone and configure
git clone <repository-url>
cd rayavriti-netmonitor
cp .env.example .env

# 2. Set required environment variables
#    Edit .env and set at minimum:
#    - JWT_SECRET (run: openssl rand -base64 32)
#    - ADMIN_PASSWORD (your admin login password)

# 3. Build and run
docker compose up -d

# 4. Check health
curl http://localhost:3000/health

# 5. Open dashboard
open http://localhost:3000
```

### Option 2: Docker Development

```bash
# 1. Configure local development
cp .env.dev.example .env.dev

# 2. Start backend and frontend with hot reload
docker compose -f docker-compose.dev.yml up --build

# 3. Open dashboard
open http://localhost:5173
```

The development compose file starts two services:

| Service | Purpose | URL |
|---|---|---|
| `server` | Express API, Socket.IO, collectors, hot reload | `http://localhost:3000` |
| `client` | Vite React dev server | `http://localhost:5173` |

### Option 3: Bare Metal

```bash
# 1. Clone and install
git clone <repository-url>
cd rayavriti-netmonitor
npm install --workspace client

# 2. Build Go backend
cd backend && make build && cd ..

# 3. Build client
npm run build -w client

# 4. Configure
cp .env.example .env
# Edit .env — set JWT_SECRET and ADMIN_PASSWORD

# 5. Start
./backend/bin/netmonitor
```

> **Note:** Packet capture and SNMP require root or `NET_RAW` capability:
> ```bash
> sudo setcap cap_net_raw+ep $(which node)
> # or run with sudo
> ```

---

## 💻 Development

```bash
# Terminal 1 — Backend (with hot reload)
npm run dev
# May need sudo for packet capture

# Terminal 2 — Frontend (Vite dev server)
npm run dev:client
# Opens at http://localhost:5173 (proxies API to :3000)
```

**Default dev credentials:** `admin` / `admin123` (only when `ADMIN_PASSWORD` is not set)

---

## 🔧 Configuration

All configuration is via environment variables. See [`.env.example`](.env.example) for the full list.

### Required (Production)

| Variable | Description |
|---|---|
| `JWT_SECRET` | JWT signing secret — minimum 32 chars. Generate: `openssl rand -base64 32` |
| `ADMIN_PASSWORD` | Admin user password — hashed with scrypt on first boot |

### Optional

| Variable | Default | Description |
|---|---|---|
| `NODE_ENV` | `development` | Set to `production` for production mode |
| `PORT` | `3000` | HTTP server port |
| `ADMIN_USERNAME` | `admin` | Admin username |
| `DB_PATH` | `./data/netmonitor.db` | SQLite database file path |
| `NETFLOW_PORT` | `2055` | UDP port for NetFlow/sFlow collector |
| `METRICS_RETENTION_DAYS` | `30` | Auto-delete metrics older than N days |
| `FLOW_RETENTION_DAYS` | `7` | Auto-delete flow records older than N days |
| `ALERTS_RETENTION_DAYS` | `90` | Auto-delete resolved alerts older than N days |
| `PORT_DISCOVERY_ENABLED` | `true` | Enable automatic port scanning |
| `CAPTURE_ENABLED` | `true` | Enable packet capture feature |

---

## 🛠️ Scripts

| Command | Description |
|---|---|
| `npm run dev` | Start Go backend with hot reload (air) |
| `npm run dev:client` | Start Vite dev server |
| `npm run build` | Build Go backend and React client for production |
| `npm run build:client` | Build client React bundle only |
| `npm run start` | Start production server |

---

## 🐳 Docker Deployment

The production Docker setup uses **host networking** to allow the container to directly monitor local network devices.

```bash
# Configure production environment
cp .env.prod.example .env

# Build and start
docker compose up -d

# View logs
docker compose logs -f netmonitor

# Stop
docker compose down

# Rebuild after code changes
docker compose up -d --build
```

**Important Docker notes:**
- `network_mode: host` — Required for ping, SNMP, and packet capture to reach local devices
- `cap_add: NET_RAW, NET_ADMIN` — Required for raw socket access
- PostgreSQL + TimescaleDB data persists in the `postgres_data` Docker volume
- Health check runs every 30s against `/health`

---

## 🌿 Branching Model

This repository uses a two-branch deployment flow:

| Branch | Purpose |
|---|---|
| `main` | Production branch. Keep this always deployable. |
| `develop` | Development integration branch. Merge completed feature work here first. |
| `feature/<name>` | New product work, branched from `develop`. |
| `fix/<name>` | Non-urgent fixes, branched from `develop`. |
| `release/<version>` | Optional stabilization branch before merging to `main`. |
| `hotfix/<name>` | Urgent production fixes, branched from `main`, then merged back to `develop`. |

Recommended flow:

```bash
# Start feature work
git checkout develop
git pull origin develop
git checkout -b feature/example

# Open PR: feature/example -> develop

# Release to production
git checkout develop
git checkout -b release/v1.1.0
# Open PR: release/v1.1.0 -> main

# After release, tag main
git checkout main
git pull origin main
git tag v1.1.0
git push origin v1.1.0
```

Protect both long-lived branches:

| Branch | Required checks | Merge rule |
|---|---|---|
| `main` | CI build, typecheck, Docker production image | PR approval, no direct pushes |
| `develop` | CI build, typecheck, Docker dev/prod image builds | PR approval recommended |

---

## 📡 API

The server exposes a REST API at `/api` (legacy) and `/api/v1` (current).

**Authentication:** Include `Authorization: Bearer <token>` header, or `X-Api-Key: <key>` for API key auth.

See [`documentation/api_documentation.md`](documentation/api_documentation.md) and [`documentation/postman_guide.md`](documentation/postman_guide.md) for full API reference.

### Key Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/auth/login` | Authenticate and get JWT tokens |
| `GET` | `/api/devices` | List all monitored devices |
| `POST` | `/api/devices` | Add a new device |
| `GET` | `/api/metrics/latest` | Get latest metrics per device |
| `GET` | `/api/alerts` | List alerts |
| `GET` | `/api/stats` | Dashboard statistics |
| `GET` | `/api/v1/flows` | Query flow records |
| `POST` | `/api/v1/capture/start` | Start packet capture |
| `GET` | `/api/insights` | AI health scores |
| `GET` | `/health` | Service health check |

---

## 🗄️ Database

Rayavriti NetMonitor uses **PostgreSQL + TimescaleDB** for time-series data storage and high-performance queries.

### Hypertables

The following tables are partitioned as TimescaleDB hypertables for efficient time-series operations:

| Table | Time Column | Purpose |
|---|---|---|
| `metrics` | `timestamp` | Device metrics (ping, HTTP, SNMP, etc.) |
| `flows` | `created_at` | NetFlow/sFlow records |
| `capture_packets` | `timestamp` | Captured packet data |
| `alert_history` | `created_at` | Alert lifecycle events |

### Data Retention

Automated pruning runs every 6 hours via the Go retention scheduler:
- **Metrics:** 30 days (configurable via `METRICS_RETENTION_DAYS`)
- **Flow records:** 7 days (configurable via `FLOW_RETENTION_DAYS`)
- **Resolved alerts:** 90 days (configurable via `ALERTS_RETENTION_DAYS`)

---

## 🔒 Security

- **Password hashing:** scrypt with random 32-byte salt (backward-compatible with legacy SHA-256)
- **JWT authentication:** HS256 signed tokens with 15-minute access / 7-day refresh
- **Security headers:** Go middleware (CORS, X-Frame-Options, rate limiting)
- **CORS:** Restricted in production mode
- **Request limits:** 1MB body size limit
- **Rate limiting:** Per-IP request throttling
- **Error handling:** Stack traces hidden in production, global error recovery middleware
- **Structured logging:** Go slog with structured fields (machine-parsable in production)

---

## 🤝 Contributing

1. **Design:** UI additions must follow the **neon-minimalist** design language
2. **Architecture:** Backend services should integrate with the WebSocket event-driven architecture
3. **Code Quality:** TypeScript strict mode is enabled — maintain type safety
4. **Branching:** Create feature branches, never commit directly to `main`

---

## 📄 License

**Proprietary** — All rights reserved.
