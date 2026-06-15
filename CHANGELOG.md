# Changelog

All notable changes to Rayavriti NetMonitor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
