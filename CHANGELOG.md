# Changelog

All notable changes to Rayavriti NetMonitor will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
