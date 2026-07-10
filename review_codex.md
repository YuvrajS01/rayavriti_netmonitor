# Rayavriti NetMonitor Codebase Review

Review date: 2026-07-10  
Prior review baseline: 2026-06-28  
Scope: backend, frontend, deployment, configuration, dependencies, security posture, performance, maintainability, and missing product capabilities for a small-organization network monitoring platform.

## Executive Summary

Rayavriti NetMonitor has a strong foundation for a campus or small-organization network monitoring platform: Go backend, React/Vite frontend, TimescaleDB-oriented schema, Redis caching/rate-limit hooks, JWT/API-key auth, RBAC permissions, alerting, discovery, packet capture, reporting, status pages, tests, CI, and Docker packaging.

The previous review identified several critical gaps. A number of those have since been addressed in code:

- Most v1 routes now have `rbac.RequirePermission` wrappers in `backend/internal/server/server.go`.
- WebSocket scope filtering now denies missing or malformed scoped events, loads user scopes on connect, and no longer accepts query-string tokens.
- Refresh now re-fetches the current user, checks `enabled`, reloads permissions, rotates the refresh token, and issues tokens with current permissions.
- API-key authentication now loads role permissions into claims.
- Security headers no longer include `unsafe-inline` CSP, HSTS is conditional, and CORS headers are explicit.
- Packet capture now has feature-flag and quota controls for duration, packet count, byte count, and packet result limits.
- `.gitignore` ignores coverage artifacts, and the CI workflow includes gitleaks and Trivy.

The codebase is still not production-hardened to a "military grade" standard. The highest remaining risks are local secret exposure, deployment privilege boundaries, broad generic CRUD for sensitive resources, packet-capture payload handling, incomplete audit guarantees, localStorage token exposure, SNMP credential handling, and operational hardening.

## Verification Status

Previously reported quality gates:

- `go test ./...` passed when run with local socket permissions.
- `npm run lint -w client` passed.
- `npm run typecheck -w client` passed.
- `npm run test -w client` passed: 8 test files, 101 tests.
- `npm run build -w client` passed.
- `npm audit --workspaces --json` reported 0 known npm vulnerabilities.

Current document refresh:

- This update reviewed the current repository files directly and corrected stale claims from the prior review.
- Full test, build, audit, and dependency-version checks were not rerun during this document refresh.

## Critical Findings

### 1. Previous local secrets were neutralized, but prior values should be treated as exposed

Evidence:

- `.env` and `.env.dev` no longer contain the previously observed production-looking JWT secrets or weak admin/database passwords.
- `.env.example`, `.env.dev.example`, and `.env.prod.example` use empty or clearly fake secret values.
- `docker-compose.yml` now requires `POSTGRES_PASSWORD` and `REDIS_PASSWORD` instead of falling back to weak defaults.

Impact:

- Any previously used local values should still be considered exposed because they existed in the workspace before this cleanup.

Recommendation:

- Rotate `JWT_SECRET`, admin password, database password, Redis password, API keys, and notification-provider tokens if any prior local values were ever used outside disposable local testing.
- Keep `.env`, `.env.*`, except `*.example`, ignored.
- Keep gitleaks enabled in CI and consider a pre-commit secret scan for local development.

### 2. Deployment still runs the main application with broad network privileges

Evidence:

- `docker-compose.yml` uses `network_mode: host` for the main `netmonitor` service.
- The app container receives `NET_RAW` and `NET_ADMIN`, although `cap_drop: [ALL]`, `read_only`, `tmpfs`, and `no-new-privileges` are now present.
- TimescaleDB is bound to `127.0.0.1:5433`.
- Redis is now bound to `127.0.0.1:6379` so the host-networked app can reach it, and Redis requires a configured password.
- Production `Dockerfile` now runs as the `netmonitor` user and no longer installs `tcpdump`, which resolves part of the earlier finding.

Impact:

- A compromise of the main API process still has an unusually high host/network blast radius.
- Host networking makes service isolation, firewalling, and per-component policy harder.

Recommendation:

- Split packet capture, NetFlow, and privileged probing into a separate collector/capture agent.
- Run the web/API process on a normal private Docker network without `NET_RAW`, `NET_ADMIN`, or host networking.
- Keep host networking only for the component that strictly needs it.
- Add CPU/memory limits, log rotation limits, and image digest pinning.

### 3. Packet capture payload capture is now opt-in, but the privileged runtime remains

Evidence:

- Packet capture defaults to metadata-only because `CAPTURE_PAYLOAD_ENABLED=false` controls whether `tcpdump -x` is used and whether payloads are stored/broadcast.
- Capture has `CAPTURE_ENABLED`, max duration, max packets, max bytes, and packet-read limit controls.
- `GetPackets` is now capped at 500 results per request.

Impact:

- If payload capture is enabled, captures may include passwords, cookies, tokens, personal data, student/staff traffic, or regulated data.
- Packet capture still runs inside the main API runtime.

Recommendation:

- Make payload capture an explicit, audited, time-limited privileged action.
- Add approval workflow, reason capture, retention TTL, and redaction/hash options.
- Keep packet capture in a separate runtime boundary from the main API.

## High Findings

### 4. Generic Phase 2 CRUD remains too broad for sensitive resources

Evidence:

- `backend/internal/database/phase2.go` maps sensitive resources such as `roles`, `users`, `user_scopes`, `scheduled_reports`, `contacts`, `maintenance_windows`, `status_page_incidents`, and `isp_links` into a generic CRUD layer.
- Table and column names are allowlisted, which helps SQL injection risk, but validation and domain invariants are shallow.
- Cursor pagination exists via `ListPhase2Cursor`, but the default list path still uses a fixed `LIMIT 500`.

Impact:

- Business rules can be bypassed when sensitive resources are updated through generic column-level writes.
- Role, user, scope, maintenance, contact, and status-page edits need typed validation and audit semantics.

Recommendation:

- Replace generic writes for sensitive resources with typed services and typed request validation.
- Keep generic handlers for low-risk lookup/read-only resources only.
- Add validators for severity/status enums, cron syntax, IP/CIDR fields, time ranges, SLA values, notification recipients, and role permission sets.
- Use cursor pagination as the default list path and include total/count semantics where needed.

### 5. Audit logging is present but not proven mandatory for every sensitive action

Evidence:

- Audit logger code and request audit middleware exist.
- The route layer does not itself prove that role/user changes, packet capture start/stop, discovery scans, alert-rule mutations, notification-channel changes, API-key creation, maintenance changes, and restore operations all produce complete domain audit events with before/after context.

Impact:

- Incident response and compliance reviews may lack reliable evidence for who changed sensitive configuration and why.

Recommendation:

- Define an audit event matrix for every sensitive route.
- Require audit events from service methods, not only generic request middleware.
- Include actor ID, role, source IP, request ID, resource type/id, before/after diff for config changes, result, and failure reason.
- Store audit logs append-only from the application perspective.

### 6. Production origin policy now fails fast when unset

Evidence:

- `config.Load` now returns an error in production when `CORS_ORIGINS` is empty.
- HTTP CORS and WebSocket origin checks share the configured origin list.

Impact:

- The main remaining risk is operational: deployments must provide the correct origin allowlist for every public hostname.

Recommendation:

- Keep explicit origin allowlists in production deployment manifests.
- Keep non-browser WebSocket clients supported through explicit deployment profiles or mTLS/API-key flows if needed.

### 7. Browser token storage now avoids refresh-token persistence

Evidence:

- `client/src/api/http.ts` keeps access tokens in memory only.
- `client/src/api/auth.ts` no longer writes access or refresh tokens to `localStorage`.
- `client/src/store/authSlice.ts` persists the user profile only and removes legacy token keys on logout/session clear.
- Backend API and WebSocket auth now accept the HttpOnly access cookie.

Impact:

- XSS can no longer read the refresh token from `localStorage`.
- XSS risk remains for in-memory access tokens while the page is active and for any authenticated actions the browser can perform.

Recommendation:

- Add CSRF protection if cookie auth becomes the primary browser auth mechanism.
- Consider a BFF/session-cookie model that avoids exposing access tokens to JavaScript at all.

### 8. SNMP defaults and credential storage are weak for production

Evidence:

- SNMP templates and discovery still use community string `public` in several places.
- Discovery performs minimal SNMP probing with community `public`.
- Device SNMP community strings are stored in `devices.snmp_community` as plain text.

Impact:

- SNMPv2c community strings are shared secrets and should not be stored or logged as ordinary configuration.
- Default `public` probing can be noisy and may violate stricter campus network policy.

Recommendation:

- Add SNMPv3 support as the production default.
- Store SNMP credentials encrypted with envelope encryption or integrate with an external vault.
- Make `public` probing opt-in per scan profile, with audit logging.
- Add credential rotation and per-location credential inheritance.

### 9. TimescaleDB retention relies on application DELETE jobs

Evidence:

- The retention scheduler calls application-level prune functions.
- Migrations create hypertables but do not define TimescaleDB native retention or compression policies.

Impact:

- Large metrics, flow, capture, and log tables can accumulate bloat and deletion overhead.
- Retention depends on the application scheduler running successfully.

Recommendation:

- Use TimescaleDB native `add_retention_policy` for hypertables.
- Add compression policies for older metrics, flows, monitoring tables, capture metadata, and notification logs.
- Track retention job success/failure in health metrics.
- Size chunks based on ingest volume.

## Medium Findings

### 10. RBAC coverage is improved, but authorization needs regression tests

Evidence:

- Current route wiring wraps most high-impact v1 routes with `rbac.RequirePermission`.
- The earlier finding that many writes had no RBAC wrapper is no longer accurate.
- Tests still need to prove role boundaries route by route.

Impact:

- RBAC can regress quietly when new routes are added.
- Permissions can be attached to the wrong capability even when a route is wrapped.

Recommendation:

- Add an authorization test table for every route, method, and role.
- Add viewer-negative tests for every write route.
- Add tests for disabled users, role changes, refresh-token reuse, WebSocket scoping, capture authorization, and discovery authorization.

### 11. Refresh/session model is improved but lacks full session management

Evidence:

- Refresh now re-fetches the current user, checks `enabled`, reloads permissions, rotates refresh tokens, and issues permission-bearing tokens.
- Remaining gaps include full token-family reuse detection, session metadata, session-management UI, and global invalidation on password/role/security changes.

Impact:

- A stolen refresh token may not trigger broad session-family revocation.
- Operators lack complete session visibility and control.

Recommendation:

- Store user agent hash, IP/CIDR, device label, created/last-used timestamps, and revoked reason.
- On refresh-token reuse, revoke the full token family for that user/session.
- Revoke all sessions on password reset, role change, user disable, or JWT secret rotation.
- Add a session-management UI.

### 12. Query limits and pagination remain inconsistent

Evidence:

- Some handlers use bounded query parsing.
- Capture packet reads are capped.
- Metric and report export limits are now capped through the shared time-range parser.
- Default Phase 2 list calls still use fixed `LIMIT 500`.

Impact:

- Fixed caps without default pagination create incomplete views as deployments grow.

Recommendation:

- Centralize query parameter parsing with min/default/max rules.
- Enforce endpoint-specific hard caps.
- Make cursor pagination the default for all list endpoints.
- Add response-size and query-time budget metrics.

### 13. TLS verification is intentionally disabled for device probing

Evidence:

- HTTP/HTTPS collectors and discovery use `InsecureSkipVerify` for unknown or self-signed network devices.
- `gosec` excludes G402 in linter configuration.

Impact:

- This may be practical for campus devices, but it weakens identity assurance and can hide man-in-the-middle issues during monitoring.

Recommendation:

- Keep insecure TLS as an explicit per-device or per-scan-profile setting.
- Support pinned certificates or a campus CA bundle.
- Surface insecure TLS usage in UI and reports.
- Audit when insecure TLS checks are created or used.

### 14. Package and runtime versions are very new

Evidence:

- `backend/go.mod` declares Go 1.26.
- `Dockerfile` uses `golang:1.26-alpine` and `node:24-alpine`.
- `package.json` allows Node `22.x || 24.x || 26.x`.
- The frontend uses very new major versions of Vite, TypeScript, ESLint, React, and React Router.

Impact:

- Very new toolchains may be unavailable in some enterprise mirrors, CI images, scanners, or host environments.
- Reliability-focused deployments often prefer a narrower, current-LTS support matrix.

Recommendation:

- Decide and document an explicit support matrix.
- Pin production image versions by digest.
- If staying on Go 1.26 and Node 24/26, verify scanner, CI, and deployment support before release.

## Performance Review

### Backend

Strengths:

- Go is a good fit for high-concurrency monitoring workloads.
- TimescaleDB hypertables are used for metrics and flows.
- Redis-backed cache and rate limiter hooks exist.
- Request timeouts and request-size limits are configured.

Issues and improvements:

- Phase 2 default lists still use fixed `LIMIT 500`; make cursor pagination the default.
- Packet capture writes packets one-by-one inside `flushBatch`; use bulk insert/copy for high-rate captures.
- WebSocket broadcasts marshal and iterate all clients on every event; add topic subscriptions, per-tenant/location channels, and event coalescing.
- Scope filtering does a DB lookup per scoped event; enrich events with location/scope at source or cache device scope mappings.
- Verify `metrics(device_id, timestamp DESC)` and related indexes with `EXPLAIN ANALYZE` at realistic scale.
- Add Prometheus metrics for queue lengths, dropped WebSocket messages, collector duration, DB pool saturation, cache hit ratio, and scheduler lag.
- Add load tests for 1k, 5k, and 20k devices with realistic polling intervals.

### Frontend

Findings:

- Build output still includes a large Material Symbols font asset.
- Chart-heavy pages are split into chunks, but chart/data-table performance still needs scale testing.
- There is no evidence in this review of Playwright/Cypress end-to-end coverage or axe accessibility automation.

Recommendations:

- Replace the full Material Symbols font with tree-shaken SVG icons or a subset font.
- Lazy-load chart-heavy pages and individual chart components where practical.
- Consider uPlot, ECharts selective imports, or visx for large realtime datasets if Recharts becomes a bottleneck.
- Virtualize large tables and event lists.
- Keep CI bundle budgets and make warnings actionable.
- Add frontend performance tests for dashboard cold load, WebSocket update bursts, and low-power lab machines.

## Dependency and Supply Chain Review

### npm and Go

The previous review reported no npm audit vulnerabilities and identified patch/minor update opportunities. Those checks were not rerun in this document refresh.

Recommendations:

- Rerun `npm audit --workspaces --json`.
- Rerun selected `npm view` or `npm outdated` checks.
- Rerun `go list -m -u -json all` and `govulncheck ./...`.
- Use controlled patch/minor update branches with lint, typecheck, tests, build, and dashboard smoke testing.
- Add Dependabot or Renovate with grouped updates and lockfile maintenance.

### CI and Images

Current status:

- CI includes Go vet, `golangci-lint`, frontend lint/typecheck, Go race tests, Go coverage, gitleaks, `govulncheck`, `npm audit --audit-level=high`, client build, bundle budget checks, Docker builds, and Trivy image scanning.
- CI does not currently show dependency license policy checks, SBOM generation, image signing, or SLSA provenance.
- Production images are not digest pinned.

Recommendations:

- Generate SBOMs with Syft or Docker buildx SBOM support.
- Sign images with Cosign and publish provenance.
- Add license policy checks if the deployment environment requires them.
- Pin production base images by digest and rebuild on base-image CVE updates.

## Frontend UX/UI Review

Strengths:

- The app has many expected pages for network operations: dashboard, alerts, devices, discovery, campus, reports, packet capture, incidents, ISP, maintenance, user management, status page, sensors, service templates.
- Frontend lint, typecheck, tests, and build previously passed.

Issues:

- Token persistence in `localStorage` remains a security issue.
- The app likely needs stronger NOC-style information architecture for repeated use: global search, saved filters, keyboard-friendly tables, and consistent bulk actions.
- Build output indicates excess icon/font weight.
- Accessibility automation and critical end-to-end workflow coverage are not evident from this review.

Recommendations:

- Add e2e tests for login, dashboard load, device CRUD, alert acknowledge/resolve, discovery scan, report generation, and packet capture permissions.
- Add axe accessibility checks in CI.
- Add global search/command across devices, IPs, MACs, alerts, incidents, and locations.
- Add consistent route-level skeleton, error, and empty states.
- Add NOC wallboard and status display modes.

## Cleanup and Dead-Weight Review

Items to remove or reduce:

- Replace the full Material Symbols font with a subset or SVG icons.
- Remove duplicate legacy API routes after clients move to `/api/v1`, or keep them behind a compatibility flag with a deprecation date.
- Replace broad generic CRUD writes with typed services for sensitive resources.
- Review checked-in documentation PDFs if generated from Markdown sources; prefer text source of truth and generated PDFs in CI/releases.
- Do not reintroduce generated coverage artifacts into Git; `.gitignore` already covers them.

Items to keep but isolate:

- The simulator is useful, but keep it out of production profiles and production images.
- Packet capture is useful, but it should live behind explicit privileges, audit, quotas, and a separate runtime boundary.
- The SQLite import path is useful for migration, but should be treated as an admin-only offline/import tool with tests and limits.

## Missing Features for a One-Stop College Network Monitor

Security and governance:

- SSO/SAML/OIDC with campus identity providers.
- MFA/2FA is explicitly not implemented.
- Per-department tenancy with strict data boundaries.
- Password policy, account lockout, breached-password checks, and admin password rotation workflows.
- Full session management UI.
- API-key scopes, expiry, rotation, and usage analytics.
- Immutable audit log export.
- Approval workflow for packet capture and intrusive scans.

Monitoring:

- SNMPv3 credential management and encrypted secret storage.
- Device credential vault integration.
- Syslog ingestion.
- Full NetFlow/sFlow/IPFIX normalization and retention controls.
- Config backup and change detection for switches/routers.
- LLDP/CDP topology discovery.
- Wireless controller/AP monitoring.
- UPS, CCTV, firewall, DHCP, DNS, RADIUS, and ISP-specific templates.
- Certificate expiry monitoring.
- SLA/SLO burn-rate alerting.
- Maintenance-aware alert suppression with clear audit trails.

Operations:

- Multi-site/campus federation.
- HA deployment mode with multiple collectors.
- Collector agents for remote subnets.
- Backup/restore tooling for DB and configuration.
- Disaster recovery runbooks.
- Upgrade/migration runbooks.
- License-free offline deployment option for isolated campuses.
- Prometheus/OpenTelemetry export.
- Alert noise reduction/deduplication and incident correlation.

User experience:

- Global search across devices, IPs, MACs, alerts, incidents, and locations.
- Bulk device edit, tagging, ownership, and lifecycle states.
- Import validation preview with rollback.
- Mobile-friendly incident acknowledgement.
- Public status page customization and stakeholder subscriptions.
- Report scheduling with PDF/email delivery verification.

## Suggested Hardening Roadmap

### Immediate, before production

1. Rotate/remove local secrets and replace weak compose defaults.
2. Split privileged capture/probing from the main API runtime.
3. Default packet capture to metadata-only and add audit/approval for payload capture.
4. Fail production startup when HTTP or WebSocket origin allowlists are missing.
5. Add route-by-route RBAC regression tests.
6. Remove browser refresh-token persistence from `localStorage`.
7. Add typed services for sensitive Phase 2 resources.

### Next hardening wave

1. Add mandatory domain audit events for every sensitive action.
2. Add full session metadata, token-family reuse detection, and session-management UI.
3. Add typed request validation and domain invariants.
4. Make cursor pagination and hard query caps consistent across endpoints.
5. Add SNMPv3 and encrypted credential storage.
6. Add e2e, accessibility, and frontend performance tests.
7. Remove full icon-font weight and keep bundle budgets enforced.

### Scale and reliability wave

1. Build a separate collector/capture agent.
2. Add HA collector scheduling and leader election.
3. Use TimescaleDB native retention/compression policies.
4. Load-test device polling, flow ingestion, WebSockets, packet capture, and report generation.
5. Add metrics dashboards for the monitor itself.
6. Add backup/restore and disaster-recovery automation.
7. Add SBOM generation, image signing, provenance, and digest-pinned production images.

## Resolved Since Prior Review

These earlier findings should no longer be tracked as active without fresh evidence:

- Broad missing RBAC wrappers on high-impact v1 write routes.
- WebSocket scope filter allowing scoped events on missing scope data or malformed message data.
- WebSocket query-string token support.
- Refresh flow issuing new tokens from stale claims without checking current user state.
- API-key claims missing role permissions.
- CSP using `'unsafe-inline'` and HSTS being set unconditionally.
- Capture packet reads accepting unlimited positive `limit` values.
- Coverage files being tracked and not ignored.
- CI lacking secret scanning and container image vulnerability scanning.
- Production Docker image running as root.
- Production Docker image installing `tcpdump`.

## Bottom Line

The codebase is functional and meaningfully improved since the earlier review, but it is not production-hardened for a high-security institutional deployment yet. The next priorities are secret hygiene, runtime privilege separation, packet-capture privacy controls, typed sensitive-resource services, mandatory audit events, token storage cleanup, RBAC regression tests, and operational supply-chain hardening.
