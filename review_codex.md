# Rayavriti NetMonitor Codebase Review

Review date: 2026-06-28  
Scope: backend, frontend, deployment, configuration, dependencies, security posture, performance, maintainability, and missing product capabilities for a small-organization network monitoring platform.

## Executive Summary

The project has a strong base for a campus/small-organization network monitor: Go backend, React/Vite frontend, TimescaleDB-oriented schema, Redis caching hooks, JWT/API-key auth, alerting, discovery, packet capture, reporting, status pages, RBAC scaffolding, tests, and Docker packaging. The current code is not yet "military grade". The largest gaps are not syntax or build health; they are authorization coverage, secret handling, deployment hardening, token/session design, network-capture isolation, audit completeness, and operational controls.

Verification was positive for basic quality gates:

- `go test ./...` passed when run with local socket permissions.
- `npm run lint -w client` passed.
- `npm run typecheck -w client` passed.
- `npm run test -w client` passed: 8 test files, 101 tests.
- `npm run build -w client` passed.
- `npm audit --workspaces --json` reported 0 known npm vulnerabilities.

Important caveat: passing tests do not prove security. Several serious risks are visible from code inspection.

## Critical Findings

### 1. Real secrets are present in local `.env` files and weak defaults remain in examples

Evidence:

- `.env` contains a production-looking `JWT_SECRET`, `ADMIN_PASSWORD=admin@123`, and `POSTGRES_PASSWORD=netmonitor`.
- `.env.dev` contains another `JWT_SECRET`, `ADMIN_PASSWORD=admin123`, and default database credentials.
- A second-pass Git check showed `.env` and `.env.dev` are ignored and not currently tracked, but they still exist in the workspace and are easy to accidentally copy, leak, or deploy.
- The tracked `.env.example`, `.env.dev.example`, and `.env.prod.example` files still normalize weak sample database passwords such as `netmonitor`.

Impact:

- Anyone with filesystem access to this workspace can mint valid JWTs if the deployed environment uses the local secret.
- The admin password and database password are weak and exposed.
- If these files were ever copied into deployment, support bundles, screenshots, or a remote repository, assume the secrets are compromised.

Recommendation:

- Keep `.env` and `.env.dev` untracked, and delete/rotate the current local values if they have ever been used outside local testing.
- Rotate `JWT_SECRET`, admin password, database password, Redis password if added later, API keys, and notification provider tokens.
- Keep only `.env.example`, `.env.dev.example`, and `.env.prod.example` with empty or obviously fake values.
- Add `.env`, `.env.*`, except `*.example`, to `.gitignore`.
- Add secret scanning in CI, for example `gitleaks` or `trufflehog`.

### 2. RBAC is incomplete on high-impact write routes

Evidence:

- In [backend/internal/server/server.go](backend/internal/server/server.go), most routes are inside `requireAuth`, but many writes are not wrapped with `rbac.RequirePermission`.
- Examples include legacy device writes at lines 215-218, alerts at 234 and 244-250, capture at 278-279, sensors at 290-292, dashboards at 296-299, alert rules at 303-308, notification channels at 312-316, discovery at 357 and 361-363, status page admin at 367-378, maintenance at 384-388, contacts at 390-397, escalation at 399-408, incidents at 415 and 419-424, roles/users at 430-440, scheduled reports at 463-467, and ISP links at 474-478.
- Only a narrow subset of v1 device routes later receives permission wrappers at lines 457-459.

Impact:

- Any authenticated user may be able to mutate operationally sensitive resources depending on handler/database checks.
- This undermines role separation for viewers, department admins, network admins, and super admins.
- In a college deployment, a low-privilege user could potentially trigger scans, capture packets, change alerting, modify users/roles, or alter the public status page.

Recommendation:

- Make authorization deny-by-default.
- Define route groups by capability: `devices.read`, `devices.write`, `devices.delete`, `alerts.acknowledge`, `alerts.resolve`, `alert_rules.write`, `capture.execute`, `discovery.execute`, `users.manage`, `status_page.manage`, `maintenance.write`, `contacts.write`, `reports.generate`, `settings.write`.
- Add tests proving a viewer receives `403` for every write route.
- Remove duplicate unprotected route registrations after protected equivalents are added.

### 3. WebSocket scope filtering is effectively permissive

Evidence:

- The WebSocket hub allows all origins when `allowedOrigins` is empty in [backend/internal/websocket/hub.go](backend/internal/websocket/hub.go) lines 85-96.
- The scope filter in [backend/internal/server/server.go](backend/internal/server/server.go) lines 144-178 returns `true` if `len(info.Scopes) == 0`, if `msg.Data` is not `map[string]any`, if `device_id` cannot be parsed as `float64`, or if `locationID == 0`.
- `ClientInfo.Scopes` is never populated in [backend/internal/websocket/hub.go](backend/internal/websocket/hub.go) lines 229-245 when a client connects.

Impact:

- All authenticated WebSocket users can receive broad operational event streams unless every message happens to match the expected shape and scopes are loaded elsewhere.
- Query-string WebSocket tokens are supported at line 226, which risks token leakage through logs, browser history, reverse proxy logs, and support screenshots.

Recommendation:

- Load user scopes during WebSocket authentication and store them in `ClientInfo`.
- Default to deny on malformed scoped events, unknown device IDs, and missing scope data.
- Remove query-string token support for browser clients; prefer `Sec-WebSocket-Protocol` or a short-lived one-time WebSocket ticket.
- In production, reject WebSocket origins unless explicitly configured.

### 4. Refresh/session model is vulnerable to stale-claim and disabled-user use

Evidence:

- Login embeds permissions in the access token in [backend/internal/handlers/auth.go](backend/internal/handlers/auth.go) lines 44-57.
- Refresh validates the refresh JWT and DB token existence, then issues a new pair from claims in the old token at lines 122-147.
- Refresh does not re-fetch the user, check `enabled`, check current role/permissions, or include permissions in the new access token; it calls `GenerateTokenPair`, not `GenerateTokenPairWithPerms`.
- Logout routes are public in [backend/internal/server/server.go](backend/internal/server/server.go) lines 194 and 201 and only revoke a supplied refresh token.

Impact:

- Disabled users or users whose privileges were reduced may continue refreshing until refresh expiry if their token remains in the DB.
- New access tokens after refresh can have stale or missing permissions.
- There is no device/session binding, reuse detection response, or global invalidation on password/role change.

Recommendation:

- On refresh, fetch the user by ID, require `enabled=true`, reload current role and permissions, and issue tokens from current state.
- Store session metadata: user agent hash, IP/cidr, device label, created/last-used timestamps, revoked reason.
- On refresh-token reuse, revoke the full token family for that user/session.
- Revoke all sessions on password reset, role change, user disable, or JWT secret rotation.

### 5. Deployment runs with high network privileges and weak default services

Evidence:

- [docker-compose.yml](docker-compose.yml) uses `network_mode: host` at line 57.
- The app container gets `NET_RAW` and `NET_ADMIN` at lines 67-69.
- Redis and TimescaleDB are exposed to the host on ports 6379 and 5433 at lines 16-17 and 31-32.
- TimescaleDB defaults to `POSTGRES_PASSWORD=netmonitor` at line 36.
- [Dockerfile](Dockerfile) installs `tcpdump` in the production image at line 30 and runs without an explicit non-root user.

Impact:

- A compromise of the application process is a high-impact host/network compromise path.
- Redis has no password/TLS configuration in compose.
- Database credentials are predictable unless overridden.

Recommendation:

- Split packet capture into a separate, least-privileged capture agent with narrow capabilities and a signed control channel.
- Run the web/API process as a non-root user.
- Avoid host networking for the main API where possible; use host networking only for the capture/NetFlow component that requires it.
- Add Redis authentication or bind Redis to a private network only.
- Pin image versions by digest for reproducible production builds.
- Add container `read_only`, `tmpfs`, `security_opt: no-new-privileges:true`, dropped capabilities by default, CPU/memory limits, and log limits.

## High Findings

### 6. Generic Phase 2 CRUD is too broad for sensitive resources

Evidence:

- [backend/internal/database/phase2.go](backend/internal/database/phase2.go) maps many resources, including `roles`, `users`, `user_scopes`, `scheduled_reports`, `contacts`, `maintenance_windows`, `status_page_incidents`, and `isp_links`, into a generic CRUD layer at lines 20-45.
- Table and column names are allowlisted, which is good for SQL injection prevention, but validation is shallow.
- `ListPhase2` has a hard-coded `LIMIT 500` with no offset/cursor at line 88.
- `CreatePhase2` and `UpdatePhase2` accept arbitrary allowed columns without domain validation at lines 121-163.

Impact:

- Business rules can be bypassed. For example, user/role/scope edits need invariants that a generic CRUD function cannot safely enforce.
- Large deployments will hit pagination and UX limits.

Recommendation:

- Replace generic write handlers for sensitive resources with typed services and typed request validation.
- Keep generic read-only helpers only for low-risk lookup tables.
- Add explicit validators for severity/status enums, cron syntax, IP/CIDR fields, time ranges, SLA values, notification recipient formats, and role permission sets.
- Implement cursor pagination and total counts for every list endpoint.

### 7. Production CORS and security headers are not strict enough

Evidence:

- Non-production CORS allows all origins in [backend/internal/server/server.go](backend/internal/server/server.go) lines 61-83.
- Production with empty `CORS_ORIGINS` blocks browser CORS but does not fail startup at lines 66-67.
- `AllowedHeaders` is `"*"` with credentials enabled at lines 81-83.
- CSP allows `'unsafe-inline'` for scripts and styles in [backend/internal/server/middleware.go](backend/internal/server/middleware.go) lines 52-53.
- HSTS is set unconditionally at line 52, even if the app is served over plain HTTP behind a misconfigured proxy.

Impact:

- Inline script allowance weakens XSS protection.
- Credentialed wildcard headers increase exposure if an origin rule is misconfigured.
- Environment mistakes silently degrade production behavior.

Recommendation:

- Fail startup in production unless `CORS_ORIGINS` is set to an explicit allowlist.
- Replace wildcard allowed headers with required headers only.
- Remove `'unsafe-inline'` by using nonces/hashes and static CSS bundles.
- Set HSTS only when the externally visible scheme is HTTPS.
- Add `Cross-Origin-Opener-Policy`, `Cross-Origin-Resource-Policy`, and `Cross-Origin-Embedder-Policy` where compatible.

### 8. Packet capture can leak sensitive payload data and overload storage/WebSockets

Evidence:

- Capture uses `tcpdump -x`, parses hex payload lines, stores payloads, and broadcasts packet payloads via WebSocket in [backend/internal/handlers/capture.go](backend/internal/handlers/capture.go) lines 271-359.
- `GetPackets` defaults to 200 but accepts any positive `limit` without a maximum at lines 233-240.
- Only one capture can run at a time, but there are no duration, packet count, byte count, or storage quotas in the handler.

Impact:

- Captures may store passwords, session cookies, personal data, and student/staff traffic payloads.
- Large captures can generate heavy DB write load and WebSocket traffic.

Recommendation:

- Default to metadata-only capture. Make payload capture an explicit, audited, time-limited privileged action.
- Enforce max duration, max packet count, max bytes, max result limit, and retention TTL.
- Add a legal/approval workflow for packet capture in institutional environments.
- Redact or hash payloads unless deep packet inspection is explicitly required.

### 9. Audit logging exists but is not consistently enforced for sensitive actions

Evidence:

- The repository contains audit logger code, but route handlers such as role/user changes, packet capture start/stop, discovery scan, alert rule mutation, notification-channel changes, API-key creation, and maintenance changes are not consistently shown as mandatory audit events in route wiring.

Impact:

- Incident response and compliance investigations will lack reliable trails.

Recommendation:

- Define an audit event matrix and make audit logging middleware/service calls mandatory for every sensitive route.
- Include actor ID, role, source IP, request ID, resource type/id, before/after diff for config changes, and result.
- Make audit logs append-only from the application perspective.

## Medium Findings

### 10. Frontend stores tokens in `localStorage`

Evidence:

- Tokens are read/written in [client/src/api/http.ts](client/src/api/http.ts) lines 13-24 and 67-83.
- Redux auth state also persists the access token in [client/src/store/authSlice.ts](client/src/store/authSlice.ts) lines 9-26.

Impact:

- Any XSS can steal access and refresh tokens.

Recommendation:

- Prefer secure, HttpOnly, SameSite cookies for refresh tokens.
- Keep access tokens in memory only, or use a BFF/session-cookie model.
- Add CSRF protections if cookie auth is used.

### 11. API key claims do not include permissions

Evidence:

- API-key auth builds claims with user ID, username, and role only in [backend/internal/server/server.go](backend/internal/server/server.go) lines 98-108.

Impact:

- Permission checks that depend on `Claims.Permissions` may reject valid API-key users or behave inconsistently compared with JWT auth.

Recommendation:

- Load role permissions for API-key users just like login does.
- Add API-key scopes independent of user roles for least privilege.
- Add API-key expiry, rotation, last-used update, and per-key rate-limit identity.

### 12. Test coverage is good in places but does not prove RBAC boundaries

Evidence:

- Backend and frontend test suites pass, but route-level authorization gaps are still visible.
- CI already runs Go vet, `golangci-lint`, frontend lint/typecheck, Go race tests, Go coverage, `govulncheck`, `npm audit`, client build, and Docker builds in [.github/workflows/ci.yml](.github/workflows/ci.yml).

Recommendation:

- Add an authorization test table for every route and role.
- Add integration tests for disabled users, role changes, refresh-token reuse, WebSocket scoping, capture authorization, and discovery authorization.
- Add security regression tests for CORS, CSP, and request size limits.
- Add secret scanning and image vulnerability scanning to CI; those are not present in the current workflow.

### 13. Package and runtime versions are very new and may reduce deployability

Evidence:

- [backend/go.mod](backend/go.mod) declares `go 1.26` at line 3.
- [Dockerfile](Dockerfile) uses `golang:1.26-alpine` at lines 14 and 52.
- [package.json](package.json) allows Node `22.x || 24.x || 26.x`.
- [client/package.json](client/package.json) uses Vite 8, TypeScript 6, ESLint 10, React 19.2, and React Router 7.

Impact:

- Very new toolchains are fast-moving and may be unavailable in some enterprise package mirrors, CI images, scanners, or host environments.
- Military-grade reliability often favors current LTS/stable toolchains with predictable support windows.

Recommendation:

- Decide an explicit support matrix: for example current Go stable/LTS-equivalent, Node LTS, and pinned Docker image digests.
- If staying on Go 1.26 and Node 24/26, document that requirement and ensure CI/build images exist and are pinned.

## Performance Review

### Backend performance

Strengths:

- Go is a good fit for high-concurrency monitoring workloads.
- TimescaleDB hypertables are used for metrics/flows.
- Redis-backed cache/rate limiter hooks exist.
- Request timeouts are set on the HTTP server.

Issues and improvements:

- `ListPhase2` uses a fixed `LIMIT 500` and no cursor pagination. Add cursor pagination and indexes for every commonly filtered column.
- Packet capture writes packets one-by-one inside `flushBatch`; use bulk insert/copy for high-rate captures.
- WebSocket broadcasts marshal and iterate all clients on every event. Add topic subscriptions, per-tenant/location channels, and event coalescing.
- Scope filtering does a DB lookup per event for some message types. Enrich events with location/scope at source or cache device scope mappings.
- `GetLatestMetrics` uses `DISTINCT ON`; verify the `metrics(device_id, timestamp DESC)` index is used at scale with `EXPLAIN ANALYZE`.
- Add Prometheus metrics for queue lengths, dropped WebSocket messages, collector duration, DB pool saturation, cache hit ratio, and scheduler lag.
- Add load tests that simulate 1k, 5k, and 20k devices with realistic polling intervals.

### Frontend performance

Evidence from `npm run build -w client`:

- `material-symbols-outlined` font asset is about 3.93 MB.
- `charts` chunk is about 434.19 KB raw / 121.96 KB gzip.
- `vendor` chunk is about 178.68 KB raw / 56.48 KB gzip.

Recommendations:

- Replace full Material Symbols font with tree-shaken SVG icons such as `lucide-react`, or subset the icon font to used glyphs.
- Lazy-load chart-heavy pages and individual chart components.
- Consider replacing Recharts with a lighter charting library for large realtime datasets, such as uPlot, ECharts with selective imports, or visx depending on interaction needs.
- Virtualize large tables and event lists.
- Add bundle budget checks in CI.
- Add frontend performance tests for dashboard cold load, WebSocket update bursts, and low-power college lab machines.

## Dependency Review

### npm

`npm audit --workspaces --json` reported no known vulnerabilities.

Selected latest-version checks from npm on 2026-06-28:

- `react`: manifest `^19.2.5`, latest observed `19.2.7`.
- `react-dom`: manifest `^19.2.5`, latest observed `19.2.7`.
- `vite`: manifest `^8.0.10`, latest observed `8.1.0`.
- `typescript`: manifest `~6.0.2`, latest observed `6.0.3`.
- `axios`: manifest `^1.15.2`, latest observed `1.18.1`.
- `@reduxjs/toolkit`: manifest `^2.11.2`, latest observed `2.12.0`.
- `react-router-dom`: manifest `^7.17.0`, latest observed `7.18.0`.
- `recharts`: manifest `^3.8.1`, latest observed `3.9.0`.
- `eslint`: manifest `^10.2.1`, latest observed `10.6.0`.
- `vitest`: manifest `^4.1.9`, latest observed `4.1.9`.

Recommendation:

- Update patch/minor versions in a controlled branch and run lint, typecheck, tests, build, and a dashboard smoke test.
- Add Dependabot/Renovate with grouped updates and lockfile maintenance.

### Go

`go list -m -u -json all` showed patch/minor updates for several direct and indirect modules. Notable direct updates:

- `golang.org/x/crypto`: `v0.52.0` to `v0.53.0`.
- `modernc.org/sqlite`: `v1.52.0` to `v1.53.0`.

Notable indirect updates:

- `golang.org/x/net`: `v0.54.0` to `v0.56.0`.
- `golang.org/x/sys`: `v0.45.0` to `v0.46.0`.
- `golang.org/x/text`: `v0.37.0` to `v0.38.0`.
- `golang.org/x/sync`: `v0.20.0` to `v0.21.0`.
- OpenTelemetry modules from `v1.41.0`/`v0.60.0` to newer versions.

Recommendation:

- Run `go get -u=patch ./...` first, then targeted minor updates.
- Run `go mod tidy`, `go test ./...`, and integration tests with Postgres/TimescaleDB.
- Add `govulncheck ./...` to CI.

### Docker/base images

Findings:

- `timescale/timescaledb:latest-pg16` is mutable.
- `redis:7-alpine`, `node:24-alpine`, `golang:1.26-alpine`, and `alpine:3.21` are not digest pinned.

Recommendation:

- Pin production images to immutable digests.
- Use vulnerability scanning in CI, for example Trivy or Grype.
- Rebuild on base image CVE updates.

## Frontend UX/UI Review

Strengths:

- The app has many expected pages for a network operations tool: dashboard, alerts, devices, discovery, campus, reports, packet capture, incidents, ISP, maintenance, user management, status page, sensors, service templates.
- Typecheck, lint, tests, and build all pass.

Issues:

- Token storage in `localStorage` is a security issue.
- The app has many pages and likely needs stronger information architecture for NOC-style repeated use: global search, saved filters, keyboard-friendly tables, and consistent bulk actions.
- Build output indicates too much icon/font weight.
- There is no evidence from this review of accessibility automation such as axe, keyboard navigation tests, or contrast checks.
- There is no evidence of Playwright/Cypress end-to-end coverage for critical workflows.

Recommendations:

- Add e2e tests for login, dashboard load, device CRUD, alert acknowledge/resolve, discovery scan, report generation, and packet capture permissions.
- Add axe accessibility checks in CI.
- Add route-level skeleton/error/empty state consistency checks.
- Add global command/search for devices, alerts, locations, and incidents.
- Add NOC wallboard and status display modes.

## Cleanup and Dead-Weight Review

Items to remove or reduce:

- Do not commit generated coverage artifacts such as `backend/coverage.out` and `backend/coverage.html` unless there is a specific reporting workflow that requires them in source control.
- Avoid shipping `tcpdump` in the main production API image. Keep capture tooling in a separate capture-agent image.
- Replace the full Material Symbols font with a subset or SVG icons; the current build emits a roughly 3.93 MB font asset.
- Remove duplicate legacy API routes after clients move to `/api/v1`, or keep them behind a compatibility flag with a deprecation date.
- Remove public logout routes or make them authenticated session endpoints; they currently add surface area without strong value.
- Replace broad generic CRUD writes with typed services for sensitive resources instead of continuing to grow the generic Phase 2 handler.
- Review checked-in documentation PDFs if they are generated from Markdown sources; keep source-of-truth docs in text and generate PDFs in CI/releases.

Items to keep but isolate:

- The simulator is useful, but keep it out of production profiles and production images.
- Packet capture is useful, but it should live behind explicit privileges, audit, quotas, and a separate runtime boundary.
- The SQLite import path is useful for migration, but it should be treated as an admin-only offline/import tool with tests and limits.

## Missing Features for a One-Stop College Network Monitor

Security and governance:

- SSO/SAML/OIDC with campus identity providers.
- MFA/2FA is currently explicitly not implemented.
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

1. Remove committed secrets and rotate all exposed credentials.
2. Add deny-by-default RBAC wrappers to every write route.
3. Fix refresh-token flow to revalidate user state and current permissions.
4. Lock down WebSocket origins, scope loading, and token transport.
5. Disable or isolate packet capture by default.
6. Replace default DB/Redis exposure with private networks and strong credentials.
7. Add CI gates: backend tests, frontend lint/typecheck/test/build, `npm audit`, `govulncheck`, secret scanning, Docker scanning.

### Next hardening wave

1. Replace generic CRUD writes for users, roles, scopes, status page, maintenance, reports, and notification config with typed services.
2. Add audit events for every sensitive action.
3. Add typed request validation and domain invariants.
4. Add pagination/cursors and query indexes for all list endpoints.
5. Add e2e and RBAC matrix tests.
6. Add bundle budgets and remove the full Material Symbols font.

### Scale and reliability wave

1. Build a separate collector/capture agent.
2. Add HA collector scheduling and leader election.
3. Load-test device polling, flow ingestion, WebSockets, and report generation.
4. Add metrics dashboards for the monitor itself.
5. Add backup/restore and disaster-recovery automation.

## Second-Pass Additional Findings

### A. Generated coverage files are tracked

Evidence:

- `git ls-files` shows `backend/coverage.out` and `backend/coverage.html` are tracked.
- `.gitignore` does not ignore `coverage.out`, `coverage.html`, or general coverage report directories.

Impact:

- Generated reports add repository noise and can become stale quickly.
- HTML coverage files may contain copied source snippets and are not useful as source-of-truth artifacts.

Recommendation:

- Remove tracked coverage artifacts from Git and generate them in CI.
- Add `coverage.out`, `coverage.html`, `coverage/`, and equivalent frontend coverage outputs to `.gitignore`.
- Upload coverage as a CI artifact or to a coverage service instead.

### B. CI security is present but incomplete

Evidence:

- CI includes `govulncheck`, `npm audit --audit-level=high`, `golangci-lint` with `gosec`, race tests, and Docker image builds.
- CI does not show secret scanning, dependency license policy checks, SBOM generation, container image vulnerability scanning, or provenance/signing.
- Local scanner binaries `govulncheck`, `gosec`, `trivy`, and `gitleaks` were not installed in this workspace during review, so local reproducibility depends on installing them.

Impact:

- The pipeline catches some code and dependency risks, but it can still publish images with leaked secrets, vulnerable OS packages, unknown licenses, or unsigned provenance.

Recommendation:

- Add `gitleaks` or `trufflehog`.
- Add Trivy/Grype scans for filesystem and built image.
- Generate SBOMs with Syft or Docker buildx SBOM support.
- Sign images with Cosign and publish SLSA provenance.
- Make CI fail on critical/high container CVEs unless explicitly waived.

### C. SNMP defaults and credential storage are weak for production

Evidence:

- SNMP collector falls back to community string `public` and logs a warning when it is used.
- Discovery performs minimal SNMP probing with community `public`.
- Device SNMP community strings are stored in the `devices.snmp_community` column as plain text.

Impact:

- SNMPv2c community strings are shared secrets and should not be stored or logged as ordinary configuration.
- Default `public` probing can produce noisy scans and may violate network policy in stricter campuses.

Recommendation:

- Add SNMPv3 support as the production default.
- Store SNMP credentials in a secrets table encrypted with envelope encryption or integrate with an external vault.
- Make `public` probing opt-in per scan profile, with audit logging.
- Add credential rotation workflows and per-location credential inheritance.

### D. TimescaleDB retention is implemented as DELETE, not native retention/compression policy

Evidence:

- The retention scheduler calls application-level prune functions every 6 hours.
- Migrations create hypertables but do not define TimescaleDB compression or native retention policies.

Impact:

- Large metrics/flow/capture tables can accumulate bloat and deletion overhead.
- Retention behavior depends on the app scheduler running successfully.

Recommendation:

- Use TimescaleDB native `add_retention_policy` for hypertables.
- Add compression policies for older metrics, flows, monitoring tables, capture metadata, and notification logs.
- Track retention job success/failure in app health metrics.
- Use chunk interval sizing based on ingest volume.

### E. Query limits are inconsistent and some are unbounded

Evidence:

- Some handlers cap limits, for example generated reports cap at 200.
- Other handlers accept user-provided limits without a hard maximum, including capture packet reads and metric/report queries.
- Several Phase 2 lists use a fixed `LIMIT 500` with no client-controlled cursor.

Impact:

- Users can request expensive queries or large packet payload responses.
- Fixed caps without pagination create incomplete views once an organization grows.

Recommendation:

- Centralize query parameter parsing with min/default/max rules.
- Enforce endpoint-specific hard caps.
- Add cursor pagination for all list endpoints.
- Add response-size and query-time budget metrics.

### F. TLS verification is intentionally disabled for device probing

Evidence:

- HTTP/HTTPS collectors and discovery use `InsecureSkipVerify` for self-signed devices.
- `gosec` excludes G402 in the linter configuration.

Impact:

- This is understandable for campus network devices, but it weakens identity assurance and can hide man-in-the-middle issues during monitoring.

Recommendation:

- Keep insecure TLS as an explicit per-device/per-profile setting rather than a default.
- Support pinned certificates or a campus CA bundle.
- Surface insecure TLS usage in UI and reports.
- Audit when insecure TLS checks are created or used.

## Verification Log

Commands run:

- `go test ./...` from `backend`: passed with required local socket permissions.
- `npm run lint -w client`: passed.
- `npm run typecheck -w client`: passed.
- `npm run test -w client`: passed, 101 tests.
- `npm run build -w client`: passed.
- `npm audit --workspaces --json`: passed, 0 vulnerabilities.
- `go list -m -u -json all`: completed and identified update opportunities.
- `npm view ... version`: completed for selected frontend packages.

Commands with limitations:

- Initial `go test ./...`, `go list -m -u -json all`, `npm audit`, and `npm outdated` were blocked by sandbox/network restrictions. They were rerun with the required permissions where needed.
- `npm outdated --workspaces --all --json` was unreliable/slow in this environment, so selected `npm view` checks were used for the main frontend packages.

## Bottom Line

The codebase is functional and testable, but it should not be treated as military-grade yet. The top priority is security architecture, not feature volume: remove secrets, enforce authorization everywhere, harden sessions/WebSockets, isolate packet capture, and make audit logging mandatory. After that, focus on pagination, load testing, bundle reduction, typed validation, and operational runbooks.
