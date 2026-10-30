# Rayavriti NetMonitor — v5 Review Report

Review of `develop` @ `67de49a` (post-v4.5.0), 2026-10-30. Read-only review: no source files changed.
Scope: backend security, backend correctness/performance/architecture, frontend engineering, visual design/UX, ops/CI, git workflow.

Every finding below was located in code. Items spot-checked a second time are marked ✔. Items that depend on deployment details are marked *(plausible)*.

**ID prefixes:**
- `S` security
- `B` backend correctness/perf
- `F` frontend engineering
- `D` design/UX
- `O` ops/CI/Docker
- `G` git/release

## Status

**Fixed in v4.5.1 (Phase 0):**
- **Security:** S1–S6, S12–S14.
- **Backend:** B3, B5, B6 (completed in 5.0.0), B7, B15.
- **Frontend and design:** F2, D1, D2.
- **Ops:** O1, O3, O6, O8–O10.

**Fixed in v5.0.0 (Phase 1):**
- **Security:** S7–S11, S15–S19, S21, S22, plus an Origin check on cookie-authenticated writes.
- **Backend:** B1, B2, B8, B9, B11, B12, B14, B17, B18, B20, B25.
- **Frontend:** F3–F11.
- **Design:** D3–D8, except the navigation restructure.
- **Ops:** O2.

**Still open:**
- **Security:** S20 (2FA).
- **Backend:** B4 (SNMP bulk walks and SNMPv3), B10 partially (discovery still forks ping and arp per host), B13 (NetFlow ingest), B16 (WebSocket protocol v2).
- **Design:** the navigation restructure.
- **Ops:** O4 (simulator service), O5, O7.
- **Roadmap:** everything in Phase 2.

---

## 0. Executive summary

v4.5.0 fixed a lot, but four themes stand out:

1. **Features that silently don't work.** The README advertises them and the code doesn't deliver.
   - SNMP devices never go down (B3 ✔).
   - Maintenance windows and parent-child suppression are never wired (B6 ✔).
   - NetFlow ingest is never started (B13 ✔).
   - Daily and weekly reports run hourly (B5 ✔), and the performance report always errors (B5 ✔).
   - Backup and restore can't work in the Docker image (O1).
   - Packet capture likely has no capabilities (O3).
   - Live device-status events never reach the Devices page or the topology view (F2 ✔), and scoped users get no realtime events at all (S12 ✔).
2. **The dashboard shows fabricated numbers** (D1 ✔).
   - "CPU" and "Memory" are formulas over ping times.
   - The 24h heatmap is a modulo pattern.
   - Alert bars are `15+(id*17)%82`.
   - For a monitoring product this is the #1 trust issue.
3. **Authorization is still thin past the device/alert list endpoints.**
   - A refresh token works as an access token (S2 ✔).
   - A remote-fleet SSRF leaks stored API keys (S1 ✔).
   - Self-escalation to super_admin is possible (S4 ✔).
   - The tcpdump filter allows argument injection (S6 ✔).
   - Viewers can delete alerts (S5 ✔).
   - Scope checks are missing on most endpoints (S7).
4. **Scale and data lifecycle.**
   - Roughly 500–1,000 devices at a 60s interval on default settings.
   - No TimescaleDB compression, continuous aggregates or retention policies.
   - Several tables grow forever, including `system_log_events`, which gets 2 rows per HTTP request.

Visually, the app is a dark-only, near-black theme with about 250 uses of 10px text and four different status palettes. The headline and body fonts never actually load (D2 ✔: a one-line fix).

---

## 1. Security (S)

### Critical
- **S1 ✔ Remote fleet SSRF and API-key theft.**
  - `remote/registry.go:128-145`: `Update` with an empty `apiKey` skips `Validate()` but still accepts a new URL.
  - `remote/collector.go` sends the decrypted stored key as `X-Api-Key` to that URL, and `Test`/`Devices` return the response body.
  - **Exploit:** a user with `remote.manage` (network_admin) points the URL at `http://169.254.169.254/...?x=` to read cloud credentials, or at their own server to receive the stored key.
  - Also: redirects are followed with the header attached, DNS is checked only at save time, and response bodies are unbounded.
  - **Fix:**
    - Always validate.
    - Require a new key when the URL changes.
    - Re-check the IP at connect time with a dialer `Control` hook.
    - Set `CheckRedirect` to deny.
    - Use `io.LimitReader`.
    - Don't echo raw remote bodies.

### High
- **S2 ✔ A refresh token is accepted as an access token.**
  - `auth/jwt.go:15-49`: identical claims, no `typ`/`aud`/`jti`. `RequireAuth` and the WebSocket accept any valid token.
  - A 7-day refresh token is therefore a 7-day bearer token. It survives logout (only the DB row is deleted).
  - The JWT path never re-checks `user.enabled`, and permissions baked into the token stay stale for up to 7 days.
  - **Fix:**
    - Add a `typ` claim and enforce it.
    - Add `jti`.
    - Add a per-user `token_version` (bump it on disable, role change or password change).
- **S3 Secrets are logged to the DB and readable by network_admin.**
  - `logging/http_logger.go:73-83`: the first 1KB of every non-GET body is logged at Info and persisted to `system_log_events`. Redaction applies only to `/auth/login` and only to the first match.
  - Leaks user passwords (`POST /users`), refresh tokens, Telegram bot tokens, webhook headers, remote API keys and SNMP communities.
  - `system.logs` (network_admin) can search them.
  - **Fix:**
    - Stop logging bodies.
    - Add recursive key redaction.
    - Purge the existing rows.
- **S4 ✔ Self-escalation to super_admin.**
  - `database/resources.go:42`: `role`, `role_id` and `enabled` are writable columns on the generic `users` resource.
  - `PUT /users/{id}/role {"role":"super_admin"}` works for anyone with `users.manage`. `CreateUser` also accepts `super_admin` (`handlers/auth.go:364`).
  - **Fix:**
    - Use a typed user-update handler with a role hierarchy: you can only grant roles at or below your own.
    - Only super_admin can create or modify admins.
- **S5 ✔ Alert mutation gated on read permission.**
  - `server/server.go:369-370`: `PUT` and `DELETE /alerts/{id}` require only `alerts.read`. Viewers can delete alerts or set `status=resolved`, bypassing `alerts.resolve`.
- **S6 ✔ tcpdump argument injection.**
  - `handlers/capture.go:26,299-311`: the filter regex allows a leading `-`, and the filter is appended without `--`. getopt permutation means `-w/path` writes arbitrary files as the tcpdump user (possible RCE via cron.d).
  - **Fix:**
    - Add `--` before the filter.
    - Reject a leading `-`.
    - Pass `-Z`.
- **S7 Scope (tenant) checks exist only on device/alert List and Get.**
  - Missing on:
    - device Update/Delete/ScanPorts
    - `/metrics/*`, `/health/scores*`, `/flows/*`, `/insights/*`, `/capture/*`
    - locations/topology
    - every generic resource
    - reports, alert counts, ack/resolve/history
  - Device Update copies `LocationID` unconditionally, so a scoped user can move devices across scopes. Omitting the field also clears `location_id` and `parent_id`.
  - The WS bootstrap (`cmd/server/main.go:110-132`) sends all devices, metrics and alerts to every connection.
  - **Fix:** one central `authorizeDevice(ctx,id)` / `authorizeLocation` helper, applied to every by-id and list handler, plus a scoped bootstrap.
- **S8 SSRF policy covers only device create/update.** Bypasses:
  - **HTTP collector:** builds the URL with `Sprintf` and an unvalidated `HTTPPath`. `@169.254.169.254/...` changes the host. `collectors/http.go:56`.
  - **Bulk import execute:** doesn't re-validate rows (`importer/csv_importer.go:348`).
  - **Discovery:** approve inserts any scanned IP.
  - **Service templates:** `req.Host` is never validated.
  - **Notification channels:** webhook/Slack URLs and the SMTP host are unvalidated, redirects are followed, and `Test` echoes `err.Error()`, which acts as a banner oracle. With `network_mode: host`, loopback means the host's own Postgres and Redis.
  - **netutil gaps:**
    - No connect-time check, so DNS rebinding works.
    - Missing ranges: 100.64/10 (includes Alibaba metadata at 100.100.100.200), 64:ff9b::/96, 2002::/16, 0/8.
    - `fd00:ec2::254` is allowed under the Default policy.
  - **Fix:**
    - One shared "safe dialer" used by every outbound client.
    - Build URLs with `url.URL{}`.
    - `HTTPPath` must start with `/`.
- **S9 Backup restore runs psql on uploaded SQL.**
  - `backup/backup.go:393`: `\! cmd` executes shell commands, and `COPY ... TO PROGRAM` does the same with a superuser DSN. This turns any admin compromise (S2, S3, S4) into OS command execution.
  - The DB password is also passed in argv, so it is visible in `ps`.
  - **Fix:**
    - Use `pg_dump -Fc` + `pg_restore`.
    - Use a non-superuser DSN.
    - Pass the password via `PGPASSWORD`.

### Medium
- **S10 Secret exposure.**
  - `Device.SNMPCommunity` is returned to viewers (list, get, WS bootstrap).
  - `secret_mask.go` misses `bot_token`, `chat_id` and webhook `headers`.
  - Channel Create/Update return the unmasked config.
  - A PUT that echoes the masked value back overwrites the real secret.
- **S11 HTML injection on the public `/status` page.** `resource_handler.go:425`: `severity` is not escaped.
- **S12 ✔ WS scope filter is broken** (`server/server.go:244`).
  - It asserts `data["device_id"].(float64)`, but payloads use the key `deviceId` with an int64 value (`metric:update` is a struct). Scoped users get no live device events.
  - It also runs a synchronous DB query per client per message inside `Hub.Run` while holding `RLock`.
  - `SetScopeFilter` is called after `go hub.Run()`, which is a data race.
- **S13 Login throttle.**
  - `clientIP` trusts the leftmost `X-Forwarded-For`, so the IP limit is trivially bypassed.
  - The per-username limit counts all attempts, so anyone can lock out `admin` indefinitely.
  - User-not-found skips scrypt, so timing reveals which usernames exist.
- **S14 Refresh rotation isn't atomic** (`handlers/auth.go:166-217`).
  - Concurrent refreshes fork the token family, which evades reuse detection.
  - Two tabs refreshing at once trigger spurious family revocation.
  - **Fix:** `DELETE ... RETURNING` as the claim.
- **S15 DB error leakage fix is incomplete.**
  - Generic resource Create/Update/Delete, roles, discovery, campus, servicetmpl and remote handlers still return `err.Error()`.
  - Public `/health` leaks the pgx connection error.
- **S16 Backups.**
  - Create/Download/Delete need only `settings.write`.
  - Dumps contain password hashes and plaintext channel secrets.
  - Files are created world-readable before umask.
- **S17 Routes act on the wrong ID.**
  - `DELETE /users/{id}/scopes/{sid}` deletes by the *user* id (`roles.go:326`).
  - `DELETE /devices/{id}/contacts/{cid}` has the same bug (`server.go:521`).
  - `GET /contacts/{id}/devices`, `/escalation-policies/{id}/steps` and `POST /status-page/incidents/{id}/update` ignore `{id}`.
- **S18 Secrets and defaults.**
  - JWT_SECRET has no minimum length, and it doubles as the AES-GCM key for fleet credentials (plain SHA-256, no AAD).
  - `APP_ENV` defaults to `development`, which gives admin/admin123, non-Secure cookies and no rate limiter.
  - **Fix:**
    - Require ≥32 random bytes.
    - Use a separate `ENCRYPTION_KEY` with HKDF and AAD set to the row id.
    - Default to production.
- **S19 API keys.**
  - `ExpiresAt` is never set on creation, and any session can mint keys. A stolen token becomes a permanent key.
  - **Fix:**
    - Default expiry.
    - Require re-auth to mint keys.
- **S20 2FA is not implemented.** `Verify2FA` returns 501, yet the README advertises "Auth & 2FA".
- **S21 Discovery accepts any CIDR.** `0.0.0.0/0` causes an OOM. Concurrent scans are uncapped (also B10).
- **S22 Capture packets (with payload hex) are readable with `devices.read`.** `Stop` accepts any id.

### Low / hardening
- **Email:** subject CRLF injection via `alert.Message`.
- **Telegram:** MarkdownV2 is not escaped, so sends fail.
- **RTSP path:** CRLF injection.
- **Unauthenticated endpoints:**
  - `/api/v1/sync/identity` returns the instance fingerprint.
  - `/api/v1/public/incidents` is `SELECT *`, including `created_by` and resolved incidents.
- **NetFlow:** accepts flows from any source (once wired, add an exporter allowlist).
- **CSRF:** relies on SameSite=Strict alone. Add an Origin check and require `Content-Type: application/json`.
- **Passwords:**
  - Legacy unsalted `sha256:` hashes are still accepted and never rehashed.
  - There is no password-change endpoint.
  - No length rule on the seeded `ADMIN_PASSWORD`.
- **Dead code:** `rbac.FilterDeviceQuery` / `FilterAlertQuery` concatenate SQL. Unused today; delete them.
- **Frontend token storage:** the access token sits in `localStorage` (`client/src/api/auth.ts:30`), which defeats the HttpOnly cookie (see F5).

### Verified good
- **JWT:** alg pinned to HMAC.
- **Passwords:** scrypt N=32768 with a 32-byte salt and constant-time compare.
- **Cookies:** HttpOnly, SameSite=Strict, Secure in production.
- **Refresh:** reuse detection, with user/role/enabled re-read on refresh.
- **API keys:** 32 random bytes, stored as SHA-256 hashes.
- **SQL:** parameterized throughout, with column and ORDER BY allowlists.
- **exec:** argv-only, never a shell.
- **Backups:** path validation and server-generated names.
- **HTML reports:** escaped.
- **WebSocket:** origin allowlist and 4KB read limit.
- **CORS:** exact match.
- **Headers:** CSP, XFO, nosniff, COOP, HSTS.
- **Docker:** non-root, `cap_drop ALL`, no-new-privileges, read-only rootfs, DB and Redis bound to 127.0.0.1.

---

## 2. Backend correctness & performance (B)

**Test status:**
- `go vet` is clean.
- `go test ./...` passes.
- `-race` passes on scheduler, websocket, engine and cache.
- 0% coverage: `cmd/server`, `backup`, `database`, `servicetmpl`.
- The DB integration test is behind `//go:build integration` and never runs. That is how B5 shipped.

**Capacity:** workers = `NumCPU*4`, and a ping poll takes about 2–2.6s. On 8 cores that is roughly 750–960 ping devices at 60s. `MaxWorkerCount` is parsed but unused. The next ceilings are B2, then B1.

### High
- **B1 Hot-path DB round trips.** The single result-pipeline goroutine does, for every poll:
  - `UPDATE devices SET status, updated_at` (even when nothing changed; non-HOT, so it bloats about 8 indexes).
  - `GetAlertRules` (3 queries, uncached).
  - A rule-state read per rule.
  - An `upsertState` on every firing poll.
  - **Fix:**
    - Keep compiled rules in memory, invalidated on CRUD.
    - One `unnest` UPDATE, only on change.
    - Keep rule state in memory and write only on transitions.
    - Shard the pipeline by `device_id`.
- **B2 MetricBuffer drains at a fixed 50 metrics/s.**
  - `cache/metric_buffer.go:76`: one `LPOP 100` per 2s.
  - Above about 3k devices at 60s the backlog grows forever.
  - A poison row (FK violation from a deleted device) is re-queued forever.
  - **Fix:**
    - Drain in a loop.
    - Dead-letter rows that fail with SQLSTATE class 23.
    - Use `LMOVE` for at-least-once delivery.
- **B3 ✔ SNMP never reports down.** `collectors/snmp.go:164`: `status := "up"` unconditionally, and errors are ignored. There is no `CollectorMeta`, so it falls back to a 30s timeout and holds a worker for the full 30s.
- **B4 SNMP is slow and incomplete.**
  - `Walk` (GETNEXT) over 16 subtrees, so a 200-port switch needs about 1,800 round trips. `maxRepetitions` is unused.
  - No SNMPv3, despite the README.
  - No interface rate calculation (`Bandwidth` is never set).
  - Logs a "public community" warning on every poll.
  - **Fix:**
    - `BulkWalkAll` / multi-OID `GetBulk`.
    - Add USM (v3).
    - Compute counter deltas with wrap handling.
- **B5 ✔ Reports.**
  - The performance report queries non-existent columns (`response_time_ms`, …; `reports/generator.go:386`), so it always fails, and scan errors are swallowed.
  - The cron matcher is substring-based with `"0 *"` checked first (`reports/scheduler.go:123`), so daily and weekly reports run hourly and time of day is ignored.
  - `GetMetricsForReport` silently truncates at `LIMIT 5000`.
  - A zero `PeriodFrom` sets `PeriodTo` instead (`generator.go:108`).
  - **Fix:**
    - Correct the columns.
    - `robfig/cron/v3` with a stored `next_run_at`.
    - Aggregate in SQL / continuous aggregates.
- **B6 ✔ Maintenance windows and parent-child suppression are never wired.** `WithMaintenanceChecker`, `WithSuppressionChecker` and `WithSuppressedAlertRecorder` exist (`engine/alert.go:78-88`) and have implementations, but `main.go` never passes them. A parent outage therefore becomes an alert storm.
- **B7 ✔ `slog.SetDefault` is never called.**
  - 186 `slog.*` call sites ignore `LOG_LEVEL`, JSON format, the file sink, DB persistence and verbose sessions.
  - `LOG_DB_ENABLED`, `LOG_DB_SAMPLE_RATE` and `LOG_DB_DROP_POLICY` are parsed but never read.
- **B8 Two `system_log_events` rows per HTTP request.** Single-row INSERTs into a table with 6 indexes (including GIN), and the table is never pruned.
- **B9 No TimescaleDB lifecycle.**
  - No compression, continuous aggregates or retention policies.
  - Never pruned: `system_log_events`, `alert_history`, `notification_log`, `capture_packets`, `isp_metrics`, `incident_timeline`, `suppressed_alerts`, `health_score_history`, `monitoring_*`.
  - Capture "prune" deletes nothing.
  - `metrics.details` JSONB is 1–2KB per SNMP row, uncompressed.
- **B10 Discovery OOM.** `expandCIDR` materializes every address and spawns one goroutine per IP before the semaphore. It also forks `ping`/`arp` per host. **Fix:** cap at /16, iterate lazily, use native ICMP.
- **B11 Shutdown can panic.** `main.go:286-298` shuts HTTP down last, after `notifQueue` and `hub.broadcast` are closed, so in-flight handlers send on a closed channel. `Hub.Broadcast` also has a check-then-send race.

### Medium
- **B12 Dispatcher.**
  - No in-flight dedupe, so overload feeds itself with duplicate jobs.
  - No startup splay: every device is due at t=0 and the queues overflow.
  - The "re-arm" comment at `poll_dispatcher.go:322` describes code that doesn't exist.
- **B13 ✔ NetFlow / sFlow ingest is dead code.**
  - `NetFlowCollector.Listen` and `FlowAnalyzer` are never instantiated outside tests; flows only arrive via `/api/simulator`.
  - No v9, IPFIX or sFlow support, despite the README.
  - Flows are stamped with receive time and sampling rate is ignored.
- **B14 Shutdown loses metrics.** The pipeline flushes using the already-cancelled ctx, and workers stop after the pipeline. Cancelled polls are recorded as "down".
- **B15 `device:status` almost never fires.** `scheduler.go:344` overwrites `Device.Status` before the comparison at `result_pipeline.go:231`. The alert engine's `previousStatus` is wrong for the same reason.
- **B16 WebSocket fan-out.**
  - One frame per metric.
  - Synchronous Redis `PUBLISH` on the pipeline goroutine, with no local fallback if Redis is down.
  - Each message is JSON-encoded three times.
- **B17 `GetLatestMetrics` scans every chunk** (`DISTINCT ON` with no time bound). It runs on every WS connect, and its cache is invalidated on every flush. `GetDashboardStats` aggregates 24h of raw rows.
- **B18 Baseline and health refresh is heavy.** Every 2 minutes it pulls 24h of full rows per device (about 1.4M rows plus 4k queries at 1k devices). The cap also keeps the *oldest* 5000 rows.
- **B19 Absence checks are N+1** (once per rule per device per minute).
- **B20 Migration advisory lock/unlock run via `pool.Exec`, possibly on different connections.**
  - The lock can leak, and a second instance then blocks at startup.
  - The "already hypertable" branch records a rolled-back migration as applied.
  - The plain-Postgres fallback is unreachable.
- **B21 One 15s deadline covers all notification channels and retries.** One slow channel cancels the rest. Overflow drops are only logged.
- **B22 Connection leaks.** The camera/biometric collector creates a new `http.Transport` on every poll. The remote collector clones its transport per request.
- **B23 Packet capture.** One synchronous INSERT per packet inside the read loop, so tcpdump stalls and drops packets. The flush only triggers when a new line arrives.
- **B24 Ping.** RTT is truncated to whole ms (LAN shows 0ms). Each poll tries privileged then unprivileged mode.
- **B25 No observability.**
  - No Prometheus endpoint, no pprof.
  - `/health` returns 200 when the DB is down and hard-codes version `"1.1.0"`.
  - `monitoring.Recorder` and `SelfMonitor` are never constructed, so the `monitoring_*` tables are dead schema.

### Low
- **Unbounded growth:** `keyedMutex` never evicts entries.
- **Alert state:** resolved state re-fires right after the cooldown, skipping the condition duration.
- **ISP collector:** holds a DB cursor open while pinging serially.
- **HTTP collector:** treats 5xx as up.
- **Availability report:** counts `warning` as down.
- **PubSub:** backoff never resets.
- **HTTP server:** `WriteTimeout` 30s cuts off large exports.
- **`metrics` schema:** `id` column and PK index are unused.
- **Structure:**
  - `database.Database` is a 110-method god interface.
  - incident/status_page/isp handlers run SQL directly.
  - Biggest files: `discovery/scanner.go` 1496 lines, `migrations.go` 1133, `postgres.go` 1072.

### v4.5.0 fixes re-verified
- **Correct:**
  - Alert-rule N+1 batching.
  - Queue fairness.
  - Migration checksums.
  - Hub `stopOnce`.
  - `CopyFrom` direct path.
- **Not correct:**
  - Scheduler shutdown order (B14).
  - Advisory lock (B20).
  - Hub stop race (B11).
  - Metric requeue: correct but poisonable (B2).

---

## 3. Frontend engineering (F)

**Status:**
- Build passes, with all 30 routes lazy-loaded.
- Lint is clean (but no type-aware rules, no jsx-a11y).
- Typecheck is clean, with 0 explicit `any`.
- Vitest: 108 tests in 10 files pass. **CI never runs them** (O8).

**First-load JS:** about 113KB gzipped. `vendor-charts` adds 108KB gzipped on chart pages. **The single largest asset is the Material Symbols font at 744KB woff2**, for roughly 80–170 icons actually used.

### High
- **F1 ✔ = D1** Fabricated dashboard metrics (see D1).
- **F2 ✔ Live device status never updates.** `Devices.tsx:66` and `NetworkTopology.tsx:119` read `{device_id, new_status}`, but the backend sends `{deviceId, status}`.
- **F3 Hand-rolled data fetching in about 25 files.**
  - 68 raw `v1.get/post` calls bypass `src/api`.
  - No `AbortSignal` anywhere.
  - `/devices` is fetched in 9 places.
  - No debounce: Logs fires 3 requests per keystroke.
  - FlowAnalysis polls 6 endpoints every 10s even in a hidden tab, and one failure blanks the whole page.
  - Devices `load()` has no catch, so the skeleton stays forever on error.
  - Reports has unguarded races.
  - **Fix:** TanStack Query.
- **F4 Untyped WebSocket contract.** Handlers receive `Record<string,unknown>` and cast it, and the `EventName` union is stale. This is how F2 happened. **Fix:** a discriminated `WsMessage` union generated from the Go `EventType` constants.

### Medium
- **F5 Access token in `localStorage`** despite the HttpOnly cookie.
  - A refreshed token is never written back, so after a reload the stale token always triggers a 401 and a refresh, and the WS handshake fails.
  - `clearCredentials` doesn't remove `netmonitor_token`.
  - There are two different functions named `clearCredentials`.
- **F6 Refresh interceptor.**
  - Retries use bare `axios` (no interceptors).
  - Hard `window.location` redirect to login.
  - No cross-tab logout sync.
- **F7 SocketProvider.**
  - A stale socket's `onclose` nulls out the live socket (StrictMode), which leaks it.
  - The 4001–4003 close codes are dead because the backend rejects at the HTTP level, so there is no token refresh and it retries forever.
  - No jitter.
  - The context value isn't memoized.
  - `emit` is unused.
- **F8 Dashboard re-renders all 13 widgets on every `metric:update`.**
  - Side effects run inside a state updater.
  - The history buffer ordering is inconsistent between bootstrap and live updates.
  - **Fix:** batch per animation frame or per second, and use a ring buffer.
- **F9 One ErrorBoundary above the router.** A page error replaces the whole app, and there is no handling for stale lazy-loaded chunks after a deploy.
- **F10 Dead dependencies and bundle size.**
  - `@visx/visx` and `framer-motion` have **0 imports**.
  - The icon font should be subset or replaced with SVG icons.
  - The Devices page pulls recharts through a statically imported `DeviceModal`.
  - Redux (37KB) holds only `authSlice`.
- **F11 Modal accessibility.**
  - 15 inline page modals have no dialog role, no Escape handling and no focus trap.
  - 6 component modals copy-paste a focus trap.
  - **Fix:** one `<Dialog>` primitive.

### Low
- **Accessibility:**
  - `<main>` lacks `tabIndex={-1}`, so route-change focus does nothing.
  - 36 clickable divs.
  - 83 inputs vs 22 `htmlFor`.
  - Tables have no `scope`.
- **Toast:** the context value isn't memoized.
- **Large components:** UserManagement (610 lines, 16 `useState`), AIHealth, NetworkTopology, PacketCapture.
- **Error swallowing:** 14 `.catch(()=>{})` with comments saying "handled by interceptor"; the interceptor doesn't handle them.
- **Backup page:** hardcodes `/api/v1` and calls `window.open` without `noopener`.
- **Devices list:** unpaginated and unvirtualized.

---

## 4. Visual design & UX (D)

### High
- **D1 ✔ Fabricated data presented as telemetry.**
  - `StatusHeatmap.tsx:13` is a modulo pattern labelled "24h" that actually draws 16 cells.
  - `Dashboard.tsx:72-82`: CPU = `avgResp/6+warn*4`, Memory = `avgResp/7+down*8+28`.
  - Sparklines are `v+i*2`.
  - "Uptime" is actually the online count divided by the metric count.
  - `ActiveAlertsList.tsx:41` bar width is `15+(id*17)%82`.
  - The radar chart's Availability/Stability/Coverage axes are invented.
  - DeviceModal health falls back to 20/60/92.
  - The multi-line chart aligns devices by array index.
  - About 40 `StatCard` sparklines plot non-time-series data, and their trend colour follows arrow direction, not meaning ("Down ↑" renders green).
  - **Rule for v5:** no data means no widget.
- **D2 ✔ Brand fonts never apply.** `index.css:5-7` asks for `"Instrument Sans"` / `"Plus Jakarta Sans"`, but fontsource registers `"… Variable"`. The browser downloads both fonts and renders system-ui.
- **D3 Four different status palettes** (tokens, `colors.ts`, topology canvas, leftover lime in reports and AIHealth).
  - Health thresholds disagree between files (55/75 vs 40/70 vs four bands).
  - AIHealth Critical and Risk are the same colour.
  - `statusIcon('unknown')` returns `check_circle`.
  - Under deuteranopia, up, warn and down all become olive.
  - Status is colour-only in the heatmap, donut and topology.
- **D4 No light theme, no NOC/wall mode, no `2xl` layouts.**
  - 247 uses of `text-[10px]`, 417 `uppercase`.
  - Near-black surfaces are only 1–2% apart, and card borders at 20% alpha vanish on projectors.
  - The PRD §2.2 full-screen NOC mode is not implemented.
- **D5 Accessibility contrast failures (WCAG AA).**
  - `outline` text is 2.73:1. It is used for every chart axis and for unknown status.
  - Error chips are about 3.5:1.
  - The danger button is 3.48:1.
- **D6 Alerts page (the core NOC workflow).**
  - No live socket subscription despite the "Live" label.
  - Full-page loading flash on every ack.
  - No filter, bulk ack, assignee or ack note.
  - Absolute timestamps only; fetches 300 with no pagination.
- **D7 Devices page doesn't scale.**
  - Card grid only: no table, sort, group-by-location, bulk actions or pagination.
  - A red Delete button on every card.
  - Device detail is a 469-line modal with no URL (the spec wants `/devices/:id` with tabs).
- **D8 Spec is stale.** `ui_ux_specification_updated.md` still specifies lime `#d9fd3a`, League Spartan and neon glows.

### Medium
- **Navigation:**
  - 26 sidebar items.
  - Network Topology is not in the nav.
  - `ResourcePage.tsx` is not routed.
  - "Settings › General" has nothing configurable and shows version `'3.5.0'`.
  - Labels are inconsistent.
- **Search:** only matches nav labels. No ⌘K, and it is hidden on mobile.
- **Layout and controls:**
  - The mobile sidebar has no scrim or Escape.
  - The account menu doesn't close on outside click.
  - No `aria-current` or `aria-expanded`.
  - Icon-only buttons have no labels; only 37 `aria-label`s app-wide.
  - Tabs have no ARIA and no URL state.
- **Data freshness:** no data-age or stale indicator, and no "N events missed" after a reconnect.
- **Topology:** the canvas is not keyboard accessible, and the legend is missing the router glyph.
- **Motion:** reduced-motion doesn't cover rAF counters or chart animation. Infinite `glow-pulse` runs on every critical alert.
- **Forms:** 1 `aria-invalid` in the whole app and no inline errors.
- **Design system drift:**
  - 118 hard-coded hex colours.
  - Undefined `bg-scrim` token, so modal backdrops are transparent.
  - Six card padding values, five radius values, eight ad-hoc shadows.
  - Modals and the mobile bottom nav both use z-50, so the nav paints over modal backdrops.
- **Component reuse:**
  - `Button` has 59 uses vs 117 raw `<button>`.
  - Card markup is retyped 199 times.
  - No Table, Modal, Tabs, Badge or Field primitives.
  - Charts are split across recharts, 6 hand-rolled SVG charts (named "Treemap" and "Sankey" but they aren't) and 2 canvas graphs.
  - One series colour is the alarm red.

### Proposed v5 design direction
- **Theme tokens:** dark-default NOC theme plus a full light theme via `[data-theme]`, and `[data-density]` comfortable/compact.
  - Dark surfaces: `#0B0F0D` / `#121815` / `#19211D` / `#212A25`, with solid borders `#2E3832` / `#46524B`.
  - Dark text: `#E6ECE8` / `#A3AFA8` / `#7F8B85` (all ≥ 4.5:1).
- **Status set (dark):**

  | Status | Hex | Shape |
  |---|---|---|
  | up | `#2DD4BF` | ● |
  | warning | `#F5B83D` | ▲ |
  | down | `#F2545B` | ■ |
  | unknown | `#9AA4B2` | ? |
  | maintenance | `#A78BFA` | ‖ |
  | info | `#60A5FA` | – |

  - Shape is always paired with colour.
  - Light-theme status set: `#13804A` / `#9A5B00` / `#C9302C`.
- **Typography:** two families (one sans + JetBrains Mono). Scale 12/13/14/16/20/24/32/48, 12px minimum, sentence-case captions, `tabular-nums`. Wall mode uses a 1.5× root size.
- **Scales:**
  - Spacing on a 4px base.
  - Radius 4/8/12/full.
  - Three elevation levels.
  - Z-index scale: sticky 10, header 30, drawer 40, modal 50, toast 60, tooltip 70.
  - Motion 120/200ms; no infinite pulses.
- **Components** (on Radix/Ark primitives):
  - Button/IconButton (label required).
  - Field/Input/Select/Combobox/DateRange.
  - Tabs (URL-synced), Dialog, Drawer, Menu, Tooltip.
  - **DataTable:** sort, filter, virtualization, bulk bar, saved views.
  - StatusPill/StatusDot, RelativeTime/DataAge.
  - EmptyState (first-run vs filtered), Skeleton presets.
  - PageHeader with breadcrumb.
  - One chart library reading a token-driven `chartTheme`.
- **Navigation: 26 items → about 12 in 4 groups.**
  - **Monitor:** Overview; Alerts & Incidents (tabs, including maintenance); Topology (network / campus / floor plans).
  - **Inventory:** Devices with saved views (cameras and biometrics become views, not pages) and `/devices/:id`; ISP; Remote sites.
  - **Analyze:** Flows & Capture; Logs; Reports (library / builder / scheduled).
  - **Configure:** `/settings/*` sub-layout.
- **Dashboard layout:**
  - Sticky health strip: "3 down · 7 warn · 412 up · 2 paused" with clickable filters, data age and socket state.
  - Alerts with inline ack next to a building/location status grid.
  - A real 24h status-history heatmap and ISP link panels.
  - Slowest/flapping devices and a state-change timeline.
  - 12-column grid with saved per-user layouts.

---

## 5. Ops, Docker, CI (O)

- **O1 Backup/restore cannot work in the production image.**
  - No `postgresql-client` (no `pg_dump`/`psql`).
  - `read_only: true`, and `./data/backups` / `./data/reports` are neither tmpfs nor volumes, so writes fail. Even if they worked, backups would be lost when the container is recreated.
  - The global `RequestSize(1MB)` middleware caps restore uploads despite the inner 500MB limit.
  - **Fix:**
    - `apk add postgresql16-client`.
    - Named volume `netmonitor_data:/app/data`.
    - Per-route body limit.
- **O2 Version-matched tools.** Once O1 is fixed, `pg_dump` must match server major 16. Pin `timescale/timescaledb:2.x-pg16` instead of `latest-pg16`.
- **O3 Capabilities under a non-root USER are not effective** *(plausible, high confidence)*. Docker doesn't grant ambient capabilities to non-root users, so `cap_add: NET_RAW/NET_ADMIN` doesn't reach tcpdump or raw ICMP. **Fix:** `setcap cap_net_raw,cap_net_admin+eip` on `/usr/local/bin/netmonitor` and `/usr/bin/tcpdump` in the Dockerfile.
- **O4 Simulator compose service is broken.** It uses the `development` (Go) image, which has no node, to run `npm run simulate`. That script uses `npx ts-node`, which isn't installed. **Fix:** use `tsx` and the `client-development` image.
- **O5 Unsafe defaults.**
  - `APP_ENV` defaults to development (S18).
  - `.env.example` sets production with `CORS_ORIGINS=http://localhost:3000`. Secure cookies over plain http break login on a LAN IP without TLS.
  - Document a TLS reverse proxy (Caddy) or add an `INSECURE_COOKIES` escape hatch.
- **O6 `/health` returns 200 when the DB is down.** Docker therefore never restarts or alerts. Split it into `/livez` and `/readyz`.
- **O7 bootstrap.sh.**
  - Prints JWT_SECRET to the terminal.
  - `sed "s|...|$SECRET|"` corrupts on user-supplied `&` or `|`.
  - `curl | bash` from `main` with no pin or checksum.
- **O8 CI gaps.**
  - **Vitest never runs.** There is no `npm test` step.
  - Go tests run twice (`-race`, then again for coverage); merge them into one `-race -coverprofile` run.
  - The integration suite is build-tagged off. Run it with a TimescaleDB service container.
  - Unpinned tools: `golangci-lint@latest`, `govulncheck@latest`, `air@latest`, `trivy-action@master`. Pin versions, and pin actions by SHA.
  - The bundle budget only warns per chunk.
  - Docker builds run without layer cache (`cache-from: type=gha`).
- **O9 Release job.** Runs only on push to `main`, so the `type=semver` tag rule never fires and images are tagged only `main` and the sha. Add `on: push: tags: ['v*']`.
- **O10 Version is scattered.**

  | Location | Version |
  |---|---|
  | git tag | v4.5.0 |
  | README badge | 4.0.0 |
  | root `package.json` | 2.0.0 |
  | client `package.json` | 0.0.0 |
  | Settings fallback | 3.5.0 |
  | `/health` | 1.1.0 |

  **Fix:** one source of truth. Inject `-ldflags "-X main.version=$(git describe)"` and `VITE_APP_VERSION` at build time.

---

## 6. Git & branching (G)

**Current model:** GitFlow-lite.
- `feat/*`, `fix/*`, `major/*`, `new/*` branches → PR → `develop`.
- `develop` → PR → `main`.
- Release tags (`v1.0.0`…`v4.5.0`) sit on the `main` merge commits.
- 360 commits, 52 merges, effectively one author (2 Copilot commits).

**Observations:**
- **G1 Stale branches.** 37 remote branches, and every one except `develop` is fully merged into `main` (oldest from May). They add noise and confuse reviewers. Delete them, and enable "Automatically delete head branches".
- **G2 `develop` and `main` have diverged.** `develop` has 2 direct commits ("Remove old plans") not on `main`, and `main` has merge commits not on `develop`. That's harmless, but because `main` is never merged back, `git log main..develop` gets noisy. Either fast-forward `main` from `develop` at release time, or back-merge `main` → `develop` after each release.
- **G3 `release/*` branches are inconsistent.** Used for v2 and v3, then dropped. Pick one approach: tag directly from `main` (simplest for a solo maintainer), or always cut `release/x.y`.
- **G4 Commit style.**
  - Recent work uses conventional commits (`fix:`, `perf:`, `security:`), plus review IDs — good.
  - "Remove old plans" ×2 breaks the pattern; `chore(docs):` would match.
  - Consider release-please or git-cliff to generate CHANGELOG.md from these prefixes. The 56KB CHANGELOG is hand-written today.
- **G5 Protect `main`** *(not verifiable locally)*. Require CI green and a PR for both `main` and `develop`, and disallow direct pushes.
- **G6 Repo hygiene.**
  - `opencode.json` is tracked.
  - `.agents/` and `.codex/` are ignored.
  - `new_features.md` sits at the repo root.
  - `documentation/` contains PDFs duplicating their `.md` sources.
  - A local `.env` and `.env.dev` exist with real-looking values. They are correctly gitignored; make sure they are never committed.

**Suggested flow for v5:**
- `develop` is the integration branch.
- `feat/v5-*` branches per epic.
- Release candidates tagged `v5.0.0-rc.N` from `develop`.
- `main` fast-forwarded at GA.
- A tag push triggers the image release.

---

## 7. v5 roadmap

### Phase 0: v4.5.1 hotfix (about 1 week, all small)
1. **Security:**
   - S1 (remote validate + redirect), S2 (`typ` claim), S4 (user role hierarchy), S5 (alert route permissions).
   - S6 (`--` for tcpdump), S3 (stop logging bodies, purge rows).
   - S12 (WS filter key and type), S13 (XFF).
2. **Silently broken features:**
   - B3 (SNMP down), B6 (wire maintenance/suppression), B5 (report columns + cron).
   - B7 (`slog.SetDefault`), B15 (previous status), F2 (WS field names).
3. **Ops:** O1 (pg client, volume, upload limit), O3 (setcap), O6 (readiness), O8 (run vitest in CI).
4. **Design:** D2 (font family names: one line).
5. **Data honesty:** D1 (delete fabricated widgets, wire real `getSystemInfo`).

### Phase 1: v5.0 foundations
- **Data layer:**
  - Continuous aggregates `metrics_1m`, `metrics_1h`, `metrics_1d` and `flows_5m`.
  - A compression policy (segment by `device_id`, compress after 2 days).
  - Retention policies on every growing table.
  - A narrow `interface_metrics` hypertable.
  - Point dashboard, reports, baselines and health at the aggregates (fixes B9, B17, B18, B5).
- **Ingestion:**
  - Sharded pipeline with `CopyFrom`.
  - One delta `unnest` UPDATE.
  - In-memory compiled rules and rule state.
  - A `device_latest` table.
  - Drain-until-empty buffer with a dead-letter list, or Redis Streams (B1, B2).
- **Scheduler:**
  - In-flight dedupe and hashed splay.
  - Block instead of drop.
  - Ordered supervisor shutdown (B11, B12, B14).
- **AuthZ:**
  - Central scope authorizer on every handler (S7).
  - Typed user/role management.
  - Shared safe dialer for all egress (S8).
  - `pg_restore` (S9).
  - Separate encryption key (S18).
- **Observability:**
  - Prometheus `/metrics` (queue depth, drops, poll latency by protocol, pool stats, WS clients).
  - `/livez`, `/readyz`, admin-only pprof.
  - Real build version.
- **Frontend platform:**
  - TanStack Query, with `AbortSignal` throughout.
  - Generated API and WS types (OpenAPI → openapi-typescript, or tygo from Go structs).
  - Cookie-only auth.
  - Hardened socket client.
  - Route error boundaries.
  - Drop visx, framer-motion and Redux; subset the icon font.
- **Design system:**
  - Token set (dark + light + density).
  - Unified status system with shapes.
  - Radix-based primitives: Dialog, Tabs, Menu, DataTable, Field.
  - One chart theme.
  - Rewrite the UI spec from the new tokens.

### Phase 2: v5.0 features (ordered by impact/effort)
1. **NOC/Wall mode** `/wall/:dashboard`:
   - Fullscreen, no chrome, 1.5× type, auto-rotate.
   - Large data-age clock.
   - Audible or border flash on a new critical.
   - Reconnect banner.
2. **Alerts workbench:**
   - Live updates, optimistic ack.
   - Ack note and assignee.
   - Bulk ack, filter bar, relative times.
   - j/k/a/r keyboard triage.
3. **Device detail page** `/devices/:id` with tabs Overview / Interfaces / Metrics / Alerts / Ports / Logs, plus a **Devices DataTable** with group-by-building, saved views and bulk actions.
4. **Command palette (⌘K):** devices by name/IP/MAC/room, alerts, pages, and actions (pause, ack).
5. **Real interface monitoring:** SNMP BulkWalk, SNMPv3, per-interface bps/utilization/errors with graphs. The top ask for any network monitor, and missing today.
6. **Working flow analytics:** NetFlow v5/v9/IPFIX + sFlow listener with an exporter allowlist and COPY ingest. Top talkers come from `flows_5m`.
7. **TOTP 2FA + WebAuthn:** the README already promises it and the endpoint returns 501. Add a password-change flow and session management ("log out other devices").
8. **Dependency-aware alerting:** wire B6, then auto-build the parent graph from LLDP/CDP neighbour tables over SNMP to generate topology edges *(new collector)*.
9. **Customizable dashboards:** drag/resize grid, multiple dashboards, share links, per-role defaults.
10. **Light theme + density toggle.**
11. **Syslog and SNMP trap receiver:** feeds Logs and Alerts. A natural complement to polling for campus switches.
12. **PWA + push notifications** for on-call staff, and a mobile-first alert view.

### Phase 3: v5.x scale
- Multi-instance polling with consistent-hash ownership (Postgres leases).
- `LISTEN/NOTIFY` for config changes.
- Separate DB pools for ingest and API.
- WS protocol v2: topic subscriptions, coalesced per-tick deltas, permessage-deflate.
- Durable notification queue via `notification_log`.
- Split `database.Database` into per-domain repositories with sqlc.
- Integration tests and a Playwright + axe smoke run over all routes in CI.

**Target:** 10k devices at 60s on one node (shared ICMP engine, async SNMP, per-protocol concurrency budgets).
