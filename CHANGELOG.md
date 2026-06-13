# Changelog

All notable changes to Rayavriti NetMonitor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
