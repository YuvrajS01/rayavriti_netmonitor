# Backend Review Plan — Rayavriti NetMonitor

> **Scope:** Go backend under `backend/` (~30k LOC, 25 internal packages).
> **Date:** 2026-08-08
> **Method:** Four parallel deep-dives (database/cache, scheduler/collectors, security/auth, engine/ws/handlers) plus direct verification of the highest-impact findings.
>
> Findings are prioritized into **Critical**, **Security**, **High**, and **Medium** tiers. Each finding lists the problem, impact, verification (file:line), and a proposed fix. The suggested order of work is the **Recommended first 10 fixes** section.

---

## Legend

- **[C]** Critical — crash, data loss, or major correctness bug that silently disables a core feature.
- **[S]** Security — vulnerability or broken authorization.
- **[H]** High — concurrency/safety issue or significant performance or correctness defect.
- **[M]** Medium — quality, maintainability, or performance improvement.

---

## CRITICAL — Correctness / Stability

### C1. Down devices are recorded as *successes* — adaptive backoff system is dead code

- **Files:** `internal/scheduler/scheduler.go:327-344`, `internal/scheduler/poll_dispatcher.go:177-209`, `internal/collectors/ping.go:18`, `internal/collectors/http.go:67`, `internal/collectors/port.go:26`, `internal/collectors/snmp.go:66`
- **Problem:** `handlePollResult` distinguishes success/failure purely by `pr.Error != nil`. Every built-in collector returns a down device as `&Result{Status: "down"}, nil` — an intentional pattern to convey *status* rather than an *operational error*. As a result, an unreachable device takes the success branch: `entry.Failures` is reset, `effectiveInterval()` never grows, `StateUnreachable` is never set, and `DeviceStateTracker` marks the device healthy.
- **Impact:** The 1x→2x→4x→8x adaptive backoff, unreachable-state tracking, paused counters, and degraded-service relief are inert. Dead SNMP devices are polled at full rate forever (expensive walks), and `UnreachableCount`/`PausedCount` are always zero.
- **Fix:** In `handlePollResult`, treat `pr.Status == "down"` (or repeated `"warning"`) as a failure for state/backoff purposes, while continuing to skip *operational* errors (lock contention, unknown protocol) via the existing `pr.Error != nil && pr.Status == ""` guard.

### C2. No panic recovery in any background goroutine — one collector bug kills the process

**Files:**
- `internal/scheduler/worker_pool.go:240-255` — `executeJob` calls `wp.execute` with no `defer recover()`.
- `internal/server/middleware.go:177` — the *only* `recover()` in the backend (HTTP middleware); it does not cover worker goroutines.
- Also: `internal/engine/alert.go:105` (absence loop), `internal/engine/anomaly.go:36-51`, `internal/engine/flow_analyzer.go:45`, `internal/engine/escalation.go:98`, `internal/handlers/capture.go:272`, `internal/websocket/hub.go:340,367`, `internal/cache/pubsub.go:42`.
- **Problem:** A Go panic in *any* goroutine terminates the entire process. There is no recovery around job execution, ticker loops, WS reader/writer/bootstrap, or the Redis subscriber.
- **Concrete crash vector already present:** `internal/collectors/http.go:54-55` ignores the error from `http.NewRequestWithContext`. If a device's `IPAddress` contains a port (e.g. `10.0.0.1:8080`), the URL becomes malformed, `req` is `nil`, and `req.Header.Set(...)` at line 55 panics.
- **Impact:** One bad device row / one future collector bug takes down monitoring for all devices; `wg.Wait()` at shutdown can then hang forever.
- **Fix:** Add `defer func(){ if r := recover(); r != nil { slog.Error(...); /* produce a down result */ } }()` around `wp.execute`. Wrap every long-lived background goroutine in a `recover` that logs the stack. Fix the HTTP collector to check the request-construction error and return a down result instead of proceeding.

### C3. Collectors ignore context — workers can block forever and shutdown hangs

- **RTSP probe (camera/NVR):** `internal/collectors/security_devices.go:150-177` — after `DialContext` respects `ctx`, the code does `bufio.NewReader(conn).ReadString('\n')` with **no read deadline and no ctx awareness**. A device that accepts TCP but never sends a response line blocks a worker goroutine forever. The scheduler's 30s timeout (`internal/scheduler/scheduler.go:284`) is never enforced.
- **SNMP walks:** `internal/collectors/snmp.go:55-93,199-239` — gosnmp v1.43.2 has no context-aware variants. Every poll issues 1x Get + subtree walks (hrProcessor, storage table, ifTable, ifXTable) via linear `GetNext` with `Timeout=5s, Retries=1` (~10s per unanswered OID). A device with thousands of interface OIDs can hold a worker for tens of minutes, far beyond the collect deadline.
- **Impact:** A permanently blocked worker is lost to the pool; `pool.Stop()` → `wp.wg.Wait()` (`worker_pool.go:114-120`) hangs indefinitely, so graceful shutdown never completes. ResponseMs/duration metrics are also wrong.
- **Fix:** Set `conn.SetReadDeadline(...)` before reading (or run the read in a goroutine and select on `ctx.Done()`). Enforce a hard per-collect deadline the collector cannot ignore — e.g. a watchdog that closes the gosnmp `Conn`, which will error in-flight requests. Cap walk size / use `BulkWalk`/GETBULK.

### C4. Alert engine never starts and its baseline cache is never populated

**Files:**
- `backend/cmd/server/main.go:164,255` — `alertEng` is constructed and handed to the server, but `alertEng.Start(...)` / `alertEng.Stop()` are **never called**.
- `internal/engine/alert.go:84-97,105-117` — the absence-check loop lives inside `Start`, which means all `absence`-type rules silently never evaluate.
- `internal/engine/baseline.go:55` and `internal/engine/anomaly.go:45` — `RefreshBaselines()` is only ever called by the separate `AnomalyEngine` on its own cache; `AlertEngine` creates its own `BaselineCache` at `alert.go:36` and reads it at `alert.go:242`, but nothing ever refreshes it, so `baselineCache.Get(...)` always returns nil and every `anomaly` condition returns `"insufficient baseline data"` forever.
- **Impact:** Two rule families (`absence`, `anomaly`) are feature-complete but permanently dead in production.
- **Fix:** Call `alertEng.Start(...)` after construction and `alertEng.Stop()` during graceful shutdown. Share a single `BaselineCache` between the engines (or refresh the alert engine's cache on a ticker).

### C5. Metric buffer bypasses cache invalidation — stale latest metrics

- **Files:** `cmd/server/main.go:170` (`cache.NewMetricBuffer(rdb, db, 100, 2*time.Second)` passes the **raw** `*Postgres`, not `appDB`), `internal/cache/metric_buffer.go:96`, `internal/cache/cached_database.go:115-124`.
- **Problem:** `flush()` calls `b.db.RecordMetricsBatch(...)` on the raw `*Postgres`. The `CachedDatabase.RecordMetricsBatch` path that invalidates `nm:metrics:latest` / per-device keys is never invoked. Under the default production config (Redis enabled), the entire metric-write path leaves the dashboard/WS "latest metrics" stale for the full TTL (10s), rolling forever.
- **Fix:** Pass `appDB` into `NewMetricBuffer`, or invalidate the metrics cache keys explicitly after a successful batch write.

### C6. Metric buffer is not durable — metrics lost on DB outage

- **File:** `internal/cache/metric_buffer.go:72-104`.
- **Problem:** `LPopCount` pops items from the Redis list, then `RecordMetricsBatch`; on failure it falls back to per-metric `RecordMetric`. If both fail (DB down / shutdown), the items are already gone from Redis and are **lost permanently**. The Redis list only shifts the outage window; there is no ack or re-queue. The `pipe.Exec` error at line 76 is also silently swallowed (no log), and unmarshal failures are dropped.
- **Fix:** On insert failure, re-`LPush` the batch back to the head of the list (only items not inserted). Log `pipe.Exec` errors. Consider a design where items are removed only after a successful insert (ack list or dead-letter).

### C7. Migration versioning is positional — silent permanent schema drift

- **Files:** `internal/database/database.go:88-113`, `internal/database/migrations.go` (whole file).
- **Problem:** Migrations are a flat `[]string`; version = array index (`version := int64(i + 2)`). `schema_migrations` stores only that integer; SQL content is never hashed.
- **Impact:** Insert a migration anywhere except the end of the list on an existing deployment → every subsequent version shifts by +1 → the new migration's version collides with a recorded version and the `SELECT EXISTS(...)` guard silently skips it forever — missing tables/columns in production with no log.
- **Fix:** Use explicit structs `[]struct{ Version int; SQL string }`, enforce strictly-increasing versions, and store a **checksum** of the SQL; reject startup on mismatch (detect drift).

### C8. Migrations are not atomic and have no advisory lock

- **File:** `internal/database/database.go:83-115`.
- **Problem:** Statements are executed one-by-one outside a transaction, and the check-then-apply (`SELECT EXISTS` → execute → `INSERT INTO schema_migrations`) is a TOCTOU pattern. No `pg_advisory_lock`.
- **Impact:** Two API instances starting together (multi-instance design via Redis pub/sub) can both apply the same migration. Most statements are `IF NOT EXISTS`/`ON CONFLICT`, which masks the race, but a partially-failed multi-statement migration (e.g. V34 which creates ~20 tables + `create_hypertable`) leaves the schema half-applied with no version recorded, so it re-runs and may fail.
- **Fix:** Wrap each migration in a transaction (Postgres DDL is transactional) and take `SELECT pg_advisory_lock(...)` around the whole run so only one instance migrates.

---

## SECURITY — High

### S1. Scoped-user tenant isolation is dead code (broken authorization)

- **Files:** `internal/rbac/scope.go:86,124` (definitions), `internal/rbac/scope_test.go` (only callers are tests), `internal/server/server.go:287` (middleware loads scopes into context).
- **Verified:** No handler ever calls `FilterDeviceQuery`, `FilterAlertQuery`, or `GetScopeContext`. `rg` confirms the only production call site is the middleware itself.
- **Exploit:** Assign a user a `location` scope (e.g. location 2). Login. `GET /api/v1/devices`, `GET /api/v1/alerts`, `GET /api/v1/metrics/{deviceId}`, flows, reports, dashboards — all return data from **every** location. The scoped-user feature is false security.
- **Severity:** High (broken multi-tenant authorization).
- **Fix:** Thread `*ScopeContext` through every data-fetching handler and apply `FilterDeviceQuery` / `FilterAlertQuery` to the base SQL (`WHERE location_id IN (...)` / `INET << subnet`). Add integration tests proving scoped users only see their own data.

### S2. Backup restore = arbitrary SQL execution

- **Files:** `internal/handlers/backup.go:129-199` (Upload), `internal/backup/backup.go:393-398` (`psql -f <uploaded file>`), `restore` at `backup.go:352`.
- **Problem:** Upload validates only the file **extension** (`.sql`/`.gz`/`.backup`), writes it to disk, then executes the full contents via `psql`.
- **Exploit:** Any authenticated user with `settings.write` (a grantable, non-admin permission) uploads `UPDATE users SET role='admin' WHERE username='attacker';`. Full database read/write.
- **Fix:** Require a dedicated admin-only `backup.restore` permission; never restore arbitrary user-uploaded files into the primary DB; run restores against a staging DB with a least-privilege role lacking DDL/DML on core tables.

### S3. Disabled user accounts remain active via API keys

- **Files:** `internal/server/server.go:113-129` (API-key lookup), `internal/handlers/auth.go:40,156` (login/refresh *do* check `user.Enabled`).
- **Verified:** The `apiKeyLookup` callback loads key → user → claims but **never checks `user.Enabled`**.
- **Exploit:** Admin disables a terminated/compromised account; the stored API key keeps authenticating with all prior permissions, indefinitely.
- **Fix:** In the `apiKeyLookup` callback: `if !user.Enabled { return nil, errors.New("user disabled") }`. Add key-level revocation/expiry (see M5).

### S4. SSRF via device polling and port scanning

- **Files:** `internal/handlers/devices.go` (Create/Update accept arbitrary host strings; only protocol/port validated), `internal/handlers/devices.go:229` (`ScanPorts` opens raw TCP to `IP:port`), `internal/collectors/http.go:54` (HTTP HEAD to any host, `InsecureSkipVerify: true`).
- **Exploit:** User with `devices.write` adds device with host `169.254.169.254` + `HTTPPath: /latest/meta-data/iam/security-credentials/`, or host `127.0.0.1` + internal paths; reads responses via metrics. `POST /api/v1/devices/{id}/scan-ports` is a raw internal port scanner.
- **Fix:** Reject loopback, link-local, `169.254.0.0/16`, 6to4/Teredo, and cloud-metadata IPs at device creation; require DNS to resolve to a public IP for HTTP polling; re-enable TLS verification for non-local targets.

### S5. Remote-fleet SSRF, TLS skip-verify, and weak key derivation

- **Files:** `internal/remote/collector.go:103-109`, `internal/remote/registry.go:53-62` (Validate only checks scheme http/https), `internal/remote/registry.go:25-30` (AES-GCM key = `sha256(secret)` — no KDF).
- **Exploit:** Register a "remote instance" at `http://127.0.0.1:8080/admin` → collector fetches and returns contents as a "snapshot" (internal HTTP read primitive). A DB leak lets an attacker derive encryption keys offline.
- **Fix:** Block private/loopback/metadata targets in `registry.Validate`; default to TLS verification (or make it a strict opt-in); derive the key with scrypt/argon2; sign snapshots.

### S6. Login endpoint is unthrottled (brute force)

- **Files:** `internal/server/server.go:107-109` (global limiter only in production, 100 rps — useless vs. brute force), `internal/server/server.go:272` (`/api/v1/auth/login` is outside the `auth.UserRateLimiter` group), `internal/handlers/auth.go:26-44`.
- **Exploit:** Unlimited password guessing from a single IP in dev; effectively unlimited in production.
- **Fix:** Per-IP and per-username exponential backoff/lockout (Redis-backed) on the login route, in all environments.

### S7. API keys never expire and carry full permissions

- **Files:** `internal/models/models.go:142-149`, `internal/database/postgres.go:675-703`.
- **Exploit:** A leaked API key is a permanent full-permission credential. Combined with S3, an off-boarded user's key works forever.
- **Fix:** Add `expires_at`, `revoked`, `last_used_at`; rotate keys on role change; enforce expiry + enabled in lookup.

### S8. Notification secrets returned in plaintext

- **Files:** `internal/models/models.go` (Config `map[string]any`), `internal/handlers/notification_channels.go`.
- **Problem:** webhook URLs, Slack/Telegram/email credentials in `Config` are returned verbatim by List/Get.
- **Fix:** Mask secret keys in JSON responses; return secrets only on create; encrypt secrets at rest.

### S9. Admin bypass relies on stale JWT claims

- **Files:** `internal/rbac/rbac.go:44-46` (admin short-circuit using claims), `internal/auth/jwt.go:15` (Role/Permissions embedded in token).
- **Problem:** Token claims are not re-checked against the DB per request; a demoted user keeps admin powers until the token expires (15 min access / 7-day refresh).
- **Fix:** Re-load role/permissions in `RequireAuth` for every request (as is already done for API keys), reduce access-token TTL, and add a revocation table.

### S10. Unauthenticated sync endpoints disclose system identity

- **Files:** `internal/server/server.go:276-279`, `internal/handlers/sync_handler.go:19-41`.
- **Exploit:** The system fingerprint (used to authenticate remote collectors) is exposed to unauthenticated callers, leaking service modes and enabling targeting of the master/collector handshake.
- **Fix:** Authenticate these endpoints (HMAC on the fingerprint / shared secret) or move them into the protected group.

### S11. Error responses leak internal details across handlers

- **Files (representative):** `internal/handlers/devices.go:79,172,190,297`, `internal/handlers/alerts.go:26,57,102,126,143,157,166,181`, `internal/handlers/dashboards.go:21,55,68`, `internal/handlers/incident.go`, `internal/handlers/status_page.go`, `internal/handlers/isp.go`, `internal/handlers/notification_channels.go`.
- **Problem:** Most `SendError(w, 500, err.Error())` calls return raw driver/SQL errors that can leak schema names, constraint details, and DSN fragments.
- **Fix:** Log the underlying error with `slog` and return a generic `INTERNAL_ERROR` message.

### S12. CORS allows arbitrary origins with credentials outside production

- **File:** `internal/server/server.go:73-93`.
- **Problem:** In dev/staging with `CORS_ORIGINS` unset, `AllowOriginFunc` returns true for any origin while `AllowCredentials: true`.
- **Exploit:** Any website can drive the victim's browser to make credentialed API calls to a reachable instance and read responses (CSRF/data exfiltration).
- **Fix:** Always default to a configured allowlist; never combine wildcard origins with credentials.

### S13. Dev database-credential fallback

- **File:** `internal/backup/backup.go:232-239`.
- **Problem:** Missing `DATABASE_URL`/`DATABASE_DSN` silently falls back to `postgres://postgres:postgres@localhost:5432/netmonitor?sslmode=disable`.
- **Fix:** Fail startup when the DSN is missing in production; never fall back to default credentials.

---

## HIGH — Concurrency, Safety, Performance

### H1. Dashboard IDOR — get/save/delete don't verify ownership

- **Files:** `internal/handlers/dashboards.go:27-72`, `internal/database/postgres.go:808-849`.
- **Problem:** `GetDashboard(id)`, `SaveDashboard` (UPDATE by id only), `DeleteDashboard(id)` filter only on `id`, not `user_id`. `GetDashboards` (line 797) is the only user-filtered query.
- **Impact:** Any authenticated user can read, overwrite, or delete any other user's dashboard by iterating IDs.
- **Fix:** Add `AND user_id = $N` to single-dashboard queries, or enforce ownership in the handler from `auth.GetClaims`.

### H2. Duplicate-alert race

- **Files:** `internal/engine/alert.go:258-279,403-445`, callers `scheduler/result_pipeline.go:143` and `handlers/devices.go:409`.
- **Problem:** `GetAlertRuleState` → evaluate → `UpsertAlertRuleState` is a non-atomic read-modify-write, and `findActiveAlertForRule` → `CreateAlert` is TOCTOU without a transaction or unique constraint. Two concurrent evaluations (pipeline goroutine + port-scan handler) can both fire the same rule / create duplicate active alerts, histories, and notifications.
- **Fix:** Serialize per `(ruleID, deviceID)` with a keyed mutex; add a partial unique index on `(rule_id, device_id, status='active')` and use `INSERT ... ON CONFLICT`.

### H3. Synchronous notification delivery blocks the entire monitoring pipeline

- **Files:** `internal/engine/alert.go:495-547` → `internal/scheduler/result_pipeline.go:114-145`, `internal/engine/notifier.go:117-151`.
- **Problem:** `fireAlert` → `sendNotifications` runs synchronously inside `ProcessMetric`, which runs in the single result-pipeline goroutine. A slow webhook (10s timeout) or `smtp.SendMail` with **no timeout at all** stalls metric persistence and alert evaluation for all devices; `resultCh` fills and `Submit` starts dropping results.
- **Fix:** Send notifications on a bounded worker pool with per-channel timeout/context (dial SMTP with deadlines), never on the pipeline goroutine.

### H4. WS DoS / slow-client handling

- **Files:** `internal/websocket/hub.go:275-429`, `internal/server/server.go:265-266`.
- **Problems:**
  - No global/per-user connection cap or handshake rate limit → FD/goroutine/memory exhaustion.
  - 64-slot send buffer with silent drops; bootstrap message is best-effort (can be lost forever if full at connect).
  - Writer goroutine learns of disconnect only on the next loop iteration / failed ping, retaining the connection + goroutines up to ~54s.
- **Fix:** Cap connections (reject with 503) and rate-limit `/ws`; evict slow clients (standard gorilla pattern: on buffer-full, close); write bootstrap synchronously or retry; signal the writer immediately on read error via a done channel.

### H5. Escalation engine is disabled by default and leaks

- **Files:** `internal/engine/escalation.go:42-59,98-146`, `internal/handlers/contact.go:34,98-103`.
- **Problems:**
  - `NewEscalationEngine(pool, resolver, notifier, nil)` becomes `&EscalationConfig{}` with `Enabled == false` → `StartEscalation` returns nil immediately, while the handler returns `200 {"status":"escalation_started"}` — a false confirmation.
  - `runSteps` never deletes its per-alert `running` entry → unbounded map growth.
  - `time.Sleep(delay)` ignores context; `CancelEscalation` only observed after a sleep completes.
  - `RepeatCount` / `RepeatIntervalMinutes` are read but never used.
- **Fix:** Populate config from env/settings (or return 501 when disabled); `defer` removal of the `running` entry; replace `time.Sleep` with `select` on a per-run done channel.

### H6. Anomaly baseline refresh hammers the DB

- **Files:** `internal/engine/baseline.go:55-82`, `internal/database/postgres_health.go:139-153`.
- **Problem:** `RefreshBaselines` loads 24h of raw metrics per device (no LIMIT) for every device × 5 fields every 2 minutes. Plus `BaselineCache.entries` is never evicted (deleted devices/fields leak in the map forever).
- **Fix:** Compute mean/stddev with SQL aggregates (`AVG`, `STDDEV_POP`) per device/field, cap the window, and prune stale entries.

### H7. `GetDevicesFiltered` cached path ignores location/sort filters

- **File:** `internal/cache/cached_database.go:150-168`.
- **Problem:** The read-through cache is used whenever `Search/Status/Protocol/Enabled` are empty; it ignores `LocationID`, `SortBy`, `SortDir`. A request like `/devices?location_id=5&sort_by=name` returns the full unfiltered, id-sorted list when Redis is on.
- **Fix:** Only use the cached path when `LocationID == nil && SortBy == "" && SortDir == ""`; also fix the offset bug below.

### H8. `GetDevicesFiltered` cached path returns full list when `Offset >= total`

- **File:** `internal/cache/cached_database.go:157-165`.
- **Problem:** `if f.Limit > 0 && f.Offset < total { ... slice ... }` — past the end, it returns the entire list instead of an empty slice.
- **Fix:** If `f.Offset >= total`, return an empty slice with `total` unchanged.

### H9. `GetDashboardStats` swallows errors and caches zeros

- **File:** `internal/database/postgres.go:906-932`.
- **Problem:** On error it logs and returns an all-zeros map with `err == nil`; `StatsCache.GetDashboardStats` then caches zeros for 15s, and the WS bootstrap trusts it. A transient DB error shows an all-zero dashboard with no signal.
- **Fix:** Return the error; on failure prefer the previous cached value (fail-open) but never cache the zero-filled result.

### H10. Phase2 inet/cidr columns serialize as `{}`

- **File:** `internal/database/phase2.go:325-340`.
- **Problem:** `rows.Values()` decodes `INET`/`CIDR` to `netip.Prefix`, which has no `MarshalJSON` → any inet/cidr column (e.g. `subnets.cidr`, `subnets.gateway`) serializes as `{}`.
- **Fix:** Detect `netip.Prefix`/`netip.Addr` in `rowsToMaps` and convert to `.String()`; better, scan typed structs per resource.

### H11. `PruneAlerts` hits FK violations; non-resolved alerts never pruned

- **File:** `internal/database/postgres.go:865-897` (+ migrations V23/V34 for the FKs).
- **Problem:** `DELETE FROM alerts WHERE status='resolved' AND created_at < $1` violates foreign keys from `suppressed_alerts.root_cause_alert_id`, `incidents.source_alert_id`, and `notification_log.alert_id` (no `ON DELETE`). Retention fails silently and old resolved alerts accumulate. Alerts stuck in `active`/`acknowledged` are never pruned.
- **Fix:** Add `ON DELETE SET NULL`/CASCADE to those FKs, or delete in dependency order; add a cutoff that eventually prunes stale acknowledged alerts.

### H12. Unbounded alerts pagination

- **Files:** `internal/handlers/alerts.go:19-30,189-241`, `internal/database/postgres.go:466-494`.
- **Problem:** `limit`/`offset` pass straight through; DB clamps only `limit <= 0 → 50` with **no upper bound**. `?limit=1000000000` returns an enormous set plus a full `COUNT(*)` per request. `Grouped` is likewise unbound. (Devices handler clamps to 200 — do the same here.)
- **Fix:** Clamp `limit` to a max (e.g. 200/1000) and `offset >= 0` in handlers.

### H13. Unstable pagination — no `id` tiebreaker

- **Files:** `internal/database/postgres.go:483` (`created_at DESC`), `postgres.go:734`, `postgres_captures.go:129`.
- **Problem:** Offset pagination over a column that can tie (bulk inserts / buffer flush share `created_at`) yields duplicate and skipped rows across pages.
- **Fix:** Append `, id DESC` (or a unique column) to each ordered pagination.

### H14. Capture `MaxBytes` doesn't actually stop capture

- **File:** `internal/handlers/capture.go:370-385`.
- **Problem:** When `totalBytes >= MaxBytes` it logs "capture stopped" but only returns from the closure; the scan loop keeps consuming tcpdump output and each packet is silently discarded (still counted). Only stops at MaxPackets/MaxDuration/tcpdump exit.
- **Fix:** Set a `limitReached` flag and break the scan loop, then call `stopSession`.

### H15. Capture goroutine untracked and races the Stop handler

- **Files:** `internal/handlers/capture.go:107-113,123-157,456-478`.
- **Problem:** `runCapture` uses `context.WithCancel(context.Background())`, the goroutine isn't in a WaitGroup, and nothing cancels it at server shutdown (tcpdump keeps running). `Stop` handler snapshots stats, writes `StopCaptureSession`, sets `running=0`, while the goroutine may still insert its final batch and later call `stopSession` → double final writes and inconsistent persisted stats.
- **Fix:** Track the goroutine; cancel on shutdown; serialize session finalization so only one path writes the terminal DB state.

### H16. No panic recovery around background goroutines (see C2) — broad blast radius

### H17. `Stop()` not idempotent, closes the broadcast channel live

- **Files:** `internal/websocket/hub.go:223-234`, `internal/server/server.go:181` (pub/sub subscriber never stopped).
- **Problem:** `Stop()` does `close(h.broadcast)`; subsystems may still `Broadcast` afterwards; calling `Stop()` twice panics; a broadcast between `close` and `Run()` exit is dropped.
- **Fix:** `sync.Once` guard; stop the pub/sub subscriber first; use a stop channel instead of closing `broadcast`.

### H18. Flow analyzer loses final batch and can deadlock producers

- **File:** `internal/engine/flow_analyzer.go:45-76`.
- **Problem:** On `ctx.Done()` the final `flush()` uses the cancelled context → insert fails, all buffered flows lost. After `Stop()`, `flowCh` is never closed → blocked producers.
- **Fix:** Flush with a fresh short-timeout context on shutdown; close the channel (producers check a stopped flag).

### H19. SMTP notifications have no timeout and ignore context

- **File:** `internal/engine/notifier.go:117-151` — `smtp.SendMail` directly, no dial/write deadline, no `ctx`.
- **Fix:** Wrap in `smtp.Client` with dial deadline and read/write deadlines; honor `ctx`.

### H20. No notification retry/backoff

- **File:** `internal/engine/alert.go:521-532`.
- **Problem:** A transient webhook 5xx / SMTP hiccup / Telegram rate-limit is logged and dropped permanently. Alerts can go to nobody.
- **Fix:** Bounded retry queue with exponential backoff (or ≥1-2 immediate retries) + persisted pending notifications.

### H21. Alert-rule reload per metric + state writes on every evaluation

- **Files:** `internal/engine/alert.go:62-82,319-323`.
- **Problem:** `ProcessMetric` reloads all alert rules from DB for every metric (O(batch × rules) rows per flush); in `pending`/`firing` states `upsertState` writes on every evaluation.
- **Fix:** Cache rules (TTL + invalidation on rule change); persist state only on transitions.

### H22. Sustained-duration logic uses only the first condition

- **File:** `internal/engine/alert.go:293-323`.
- **Problem:** For an `all` rule with 30s + 60s conditions, it fires after 30s; the snapshot is stored but never verified continuous → wall-clock since first trigger regardless of oscillation.
- **Fix:** Track per-condition first-met timestamps; require each condition to sustain its own duration.

### H23. Supressed-absent loop

- negativeDuration skip → suppressed fires reset state to idle; every poll re-cycles `idle → pending → suppressed → idle`, double-writing state and re-checking suppression.
- **Fix:** Introduce an explicit `suppressed` state; re-check suppression only on transitions.

### H24. Absence alerts fire immediately for never-seen devices

- **File:** `internal/engine/alert.go:152-215`.
- **Problem:** When no metric exists, it fabricates one with `Timestamp: now - 24h`, so a newly added device with zero history fires an absence alert on the first tick. Also the absence path never updates `AlertRuleState`.
- **Fix:** Skip devices with no metric history (start the absence clock at first-seen); record state transitions.

### H25. Cursor vs offset pagination disagree; cursor hardcodes `id` sort

- **File:** `internal/database/phase2.go:144`.
- **Problem:** `ListPhase2Cursor` always orders by `id ASC` regardless of the resource's declared `OrderBy` (most timestamps `DESC`), so the two pagination paths return different orders and paging is inconsistent with natural ordering.
- **Fix:** Encode the sort key into the cursor, or restrict cursor pagination to `id`-ordered resources.

### H26. Rate limiting only in production + too permissive for login

- **File:** `internal/server/server.go:107-109` — see S6. Apply a dedicated low per-IP/per-account limiter on auth routes in **all** environments.

### H27. WebSocket scope query uses `context.Background()` with no timeout

- **File:** `internal/websocket/hub.go:312-324` — each connection runs an unbounded `h.db.Query` in the upgrade handler.
- **Fix:** Use `r.Context()` with a timeout; load scopes asynchronously.

### H28. Login timing side-channel for username enumeration

- **File:** `internal/handlers/auth.go:26-39`.
- **Problem:** When the username is missing, `CheckPassword` is never called → measurably faster response. DB errors are conflated with wrong-credentials (401).
- **Fix:** Always run a fixed-cost dummy `CheckPassword` against a precomputed hash when the user is missing; log DB errors distinctly.

### H29. `AlertHandler.Create` accepts arbitrary device/status/severity

- **File:** `internal/handlers/alerts.go:46-61`.
- Any authenticated caller can create fake critical alerts for devices they can't read, bypassing engine dedup. No message length limit.
- **Fix:** Validate, derive the device from server-side state, or restrict alert creation to the engine.

---

## MEDIUM — Quality, Maintainability, Performance

### M1. N+1 in `GetAlertRules` / `GetAlertRule`

- **File:** `internal/database/postgres_alertrules.go:30-34,332-387`.
- Each rule runs `loadAlertRuleRelations` (2 queries each). Batch with `WHERE rule_id = ANY($1)` or JSON aggregation.
- **Fix:** Fetch conditions / channel links for all rules in one pass.

### M2. `GetStatusFlaps` loads every metric row into memory

- **File:** `internal/database/postgres_health.go:155-178`.
- Unbounded `SELECT` of device history; health engine runs it per device. 30 days ≈ 86k rows/device.
- **Fix:** Compute transitions in SQL (`COUNT(*) FILTER (WHERE status <> LAG(status) OVER (ORDER BY timestamp))`).

### M3. `InsertHealthScoreHistory` is N individual Execs, no transaction

- **File:** `internal/database/postgres_health.go:113-137`. Partial inserts on mid-loop failure; `factors` marshaling error swallowed.
- **Fix:** Use `CopyFrom` / `pgx.Batch` in a transaction; respect `e.ComputedAt`.

### M4. `UpsertPortScanResults` read-modify-write race + swallowed read error

- **File:** `internal/database/postgres_portscans.go:18-30`.
- Pre-read is outside the upsert transaction; on transient failure it reports every port as "changed". Propagate the read error; compute change count in the upsert.

### M5. API-key lifecycle gaps

- **File:** `internal/models/models.go:142-149`, `internal/database/postgres.go:675-703`. Add expiry/revocation; see S7.

### M6. Metric buffer: re-pop before insert for durability + log pipe errors

- **File:** `internal/cache/metric_buffer.go:72-104` — see C6.

### M7. `MetricBuffer.flush` unmarshal drops gone; Redis write drops during a Redis blip

- When the batch *and* individual path fail, items already popped are lost (C5). Additionally the buffer has no ack.

### M8. Captures: `ListSessions` loads the whole table then slices in memory

- **File:** `internal/handlers/capture.go:258-269`. Add `LIMIT`/`OFFSET` in SQL.

### M9. Phase2 generic CRUD is untyped — type-safety liabilities

- **Files:** `internal/database/phase2.go:203-246,293-323`.
  - Numbers arrive as `float64` → inserting into INT column is a runtime error; `null` passes as `nil` → NOT NULL violations.
  - `users` cannot be created at all via generic create (`username`/`password_hash` not whitelisted).
  - Responses return snake_case column names while the rest of the API uses camelCase.
- **Fix:** Generate typed per-resource structs (`pgx.RowToStructByPosition`/`RowToMap`) with proper JSON tags. Keep the column whitelist.

### M10. `toSnake` mangles acronyms

- **File:** `internal/database/phase2.go:342-355` — `ISPLink` → `i_s_p_link`; camelCase filter/create/update keys with acronyms get silently dropped.
- **Fix:** A proper case converter (e.g. `strcase.ToSnake`).

### M11. `UpdatePhase2` never maintains `updated_at`

- **File:** `internal/database/phase2.go:239-241` — the `updated_at` branch is dead code; resources with the column never bump it.
- **Fix:** Add `updated_at` to whitelisted columns for tables that have it.

### M12. `Phase2Summary` issues 9 sequential counts

- **File:** `internal/database/phase2.go:257-291`. Consolidate with scalar subselects (as `GetDashboardStats` does).

### M13. Stale setup of `workers` vs dispatchers during shutdown

- **File:** `internal/scheduler/scheduler.go:169-180`.
- `pool.Stop()` runs while the dispatcher is still enqueuing, and if a worker is stuck (C3) it blocks forever. Final batch flush uses a cancelled ctx.
- **Fix:** Order: stop dispatcher → drain queues → stop pool → stop pipeline → flush with a live context (see H13 / result_pipeline.go:98-112).
### M14. Result pipeline has no backpressure

- **File:** `internal/scheduler/result_pipeline.go:75-81,177-206`.
- `Submit` uses `select ... default: drop` — under DB slowness, poll results silently dropped; no signal to the dispatcher to slow down.
- **Fix:** Bounded-blocking submit (with timeout) or a "lagging" signal that extends poll intervals.

### M15. Priority-queue starvation

- **File:** `internal/scheduler/worker_pool.go:183-237`.
- The worker drains the critical queue non-blocking first; a sustained burst of critical jobs starves normal/low. Since most devices are priority 0, this can silently stop polling other devices.
- **Fix:** Randomized weighted selection or a single fair queue with weighted round-robin.

### M16. Enqueue drops jobs with no feedback

- **File:** `internal/scheduler/worker_pool.go:122-149`.
- Drops are only logged at warn; the dispatcher keeps re-scheduling. Return a counter/bool and let `dispatchDue` extend the next poll of dropped devices.

### M17. Interval shrink not honored until old timer fires

- **File:** `internal/scheduler/poll_dispatcher.go:89-131,251-272`.
- `Upsert` only wakes for new devices; an interval change 60s→5s does not re-arm the timer. Reconcile every 30s makes staleness constant.
- **Fix:** Send a wake-up whenever `NextPollAt` moves earlier than the current timer head.

### M18. `DeviceStateTracker` never cleaned up

- **File:** `internal/scheduler/device_state_tracker.go:163-167`; `scheduler.go:204-243`.
- `stateTracker.Remove` exists but is never called from reconcile/unschedule; every device ID ever seen accumulates forever (with `responseHistory` slices).
- **Fix:** Call `Remove` on device removal/unschedule.

### M19. Redis lock failure halts all polling

- **File:** `internal/scheduler/scheduler.go:254-266`.
- If Redis is configured but briefly down, `TryLock` errors and every poll is skipped. Also the lock TTL equals `device.Interval` (0 → Redis key set with no expiry → crash leaves a permanent lock).
- **Fix:** On lock error, collect anyway (log warn); clamp TTL to a sane minimum/maximum.

### M20. Dependency-tree / pause machinery is dead code

- **Files:** `internal/scheduler/dependency_tree.go` (never instantiated in production), pause/resume in `poll_dispatcher.go:145-175`, `device_state_tracker.go:145-161`.
- `Device.ParentDeviceID` is never read by the scheduler. The "auto-pause dependents when a parent is unreachable" feature does not exist at runtime.
- **Fix:** Wire it (on parent failure, pause children; on recovery, resume) or remove the dead code.

### M21. `ActiveWorkers` misnamed; `Accumulator` config idle

- **File:** `internal/scheduler/worker_pool.go:21-30,151-166` — `ActiveWorkers` counts live worker goroutines, not busy; `maxWorkers` stored but never used; `PollJob.Attempt` set but never read.

### M22. `Scheduler.Start` not idempotent

- **File:** `internal/scheduler/scheduler.go:144-167` — second `Start` overwrites cancel and starts a second reconcile loop.

### M23. NetFlow collector is dead code with real robustness issues

- **Files:** `internal/collectors/netflow.go` — `Listen` never called anywhere.
- 2048-byte buffer truncates datagrams (max ~64KB, ~1364 records); `select default` drops flows silently; on persistent socket error it busy-loops.
- **Fix:** Wire into main (or delete), enlarge buffer, add error backoff + consumer.

### M24. `packet_capture.go` races + goroutine leak

- **Files:** `internal/collectors/packet_capture.go:26-61` — `stats`/`startTime` written in Start, read by Stats without sync; `go func(){ <-ctx.Done(); ... }()` leaks if ctx never cancelled. It's also never registered/started.

### M25. Fresh `http.Transport` per HTTP poll

- **Files:** `internal/collectors/http.go:58-63` — new client/transport per poll; idle sockets + cleanup goroutines accumulate with no reuse. (Also see S4 `InsecureSkipVerify`.)
- **Fix:** Shared client with one Transport (`MaxIdleConns`, `IdleConnTimeout`).

### M26. Port/System collectors ignore context

- **Files:** `internal/collectors/port.go:23` (`net.DialTimeout`, 5s), `internal/collectors/system.go:22-25` (`cpu.Percent(time.Second,false)` — synchronous 1s sleep).
- **Fix:** `DialContext`; replace `cpu.Percent` sleep with `cpu.Times` delta.

### M27. `LogStats` stats vs total inconsistency

- **File:** `internal/monitoring/log_store.go:136-160` — computes breakdowns over newest 1000 rows while `Total` is the full `COUNT(*)`; sums of `ByLevel` ≠ Total.
- **Fix:** `GROUP BY` in SQL.

### M28. N+1 in public/status and ISP endpoints

- **Files:** `internal/handlers/status_page.go:28-93`, `internal/handlers/isp.go:28-67`.
- **Fix:** batch with `WHERE id = ANY($1)` / joins.

### M29. Legacy monitoring handlers filter in memory after fetching 1-2k rows

- **Files:** `internal/monitoring/legacy_handlers.go:154-263`. Push filters into SQL.

### M30. `percentile` is O(n²) insertion sort

- **File:** `internal/engine/health_scorer.go:234-255`. Use `sort.Float64s`.

### M31. Raw DB errors to clients (see S11).

### M32. `ClearRefreshCookie` forces Secure in all envs (cosmetic)

- **File:** `internal/auth/cookies.go:53`.

### M33. Access/refresh tokens also in JSON body at login

- **File:** `internal/handlers/auth.go:69-78` — tokens delivered in body (non-HttpOnly storage). Acceptable for SPA, flagged for XSS residual exposure.
- **Fix:** Consider returning tokens only via HttpOnly cookies (or `sessionStorage` best-effort).

### M34. WebSocket scope filter defaults to allow

- **File:** `internal/server/server.go:230-257` — `default: return true`; scoped users receive all non-device events.
- **Fix:** Default to deny; explicitly allow only in-scope events.

### M35. `PruneMetrics`/`PruneFlows` unbounded DELETE fallback

- **File:** `internal/database/postgres.go:865-897` — one giant statement for the non-TimescaleDB path, run every 6h. Batch with `ctid`/`LIMIT` looping.

### M36. `hasTimescaleDB` detection error swallowed

- **File:** `internal/database/postgres.go:60-62` — if the query fails, `hasTS = false` and all pruning falls back to full-table DELETEs with no signal. Log it.

### M37. Batch timestamps shared across a flush

- **File:** `internal/scheduler/result_pipeline.go:116,158` — all metrics in a batch get `now` at processing time; store `pr.FinishedAt` (the real poll-completion time) as the metric timestamp.

### M38. `AlertStateCache` writes-through on upsert (not invalidate)

- **File:** `internal/cache/alert_state_cache.go:22-45` — caches 5-min TTL; stale re-read possible.
- **Fix:** invalidate on upsert, or keep TTL but document the guarantee.

### M39. Refresh tokens not tied to a session/device

- **File:** `internal/auth/session.go` — consider rotation on reuse (refresh-token reuse detection).

### M40. `MaxConnIdleTime` not set on pgxpool

- **File:** `internal/database/postgres.go:48-51` — set `MaxConnIdleTime` (e.g. 5m).

### M41. `splitStatements` vulnerable to `$$` / comments

- **File:** `internal/database/database.go:117-153` — the hand-rolled `;` splitter will mis-split dollar-quoted functions or comments; fine today only because migrations avoid them.
- **Fix:** one statement per migration entry; drop the splitter.

### M42. Set `updated_at` / audit columns on generic writes

- See M11.

### M43. `ReportConfig`/`SMTP` connection retry not present

- Notifications/emails lack reconnect/backoff; see H20.

### M44. `AlertGroupID` minute-bucketed and instance-local

- **File:** `internal/engine/alert.go:428` — `groupID := fmt.Sprintf("%d-%d", rule.ID, now.Unix()/60)` collides across rules in the same minute and differs across instances/restarts.
- **Fix:** monotonic per-alert identifier or persisted group key.

### M45. Docker / deployment: no graceful drain for WS

- See H4/H17.

---

## Positive Observations

Verified safe / done well (do not mistake for grind places):

- **JWT algorithm pinned to HMAC** (`internal/auth/jwt.go`), scrypt + constant-time compare (`internal/auth/password.go`), API-key hashes, parameterized SQL everywhere (`devices.go:127-146`).
- **Phase-2 column names are whitelisted** in `internal/database/phase2.go` — low injection risk (residual risk is type coercion, not SQL injection).
- **NetFlow parsing is bounds-checked** (`netflow.go:78`); CSV importer fully parameterized.
- **Backup paths validated against a root** (`internal/backup/backup.go:410-427`) — path traversal guarded (authz/restore permission still per S2).
- **CI present** (`ci.yml`): lint + vet + typecheck on PRs; 90 test files incl. integration coverage.

---

## Recommended Fix Order (highest ROI, mostly small & contained)

1. **C2 — executed recover & error check** — `http.go` request-construction error check; wrap every background goroutine in `recover()`.
2. **C1 — `handlePollResult` treat `down` as failure** → unlocks backoff/pause/unreachable reporting.
3. **C3 — RTSP/SNMP deadlines + interruptible** — unblock shutdown & workers.
4. **C4 — pass `appDB` to `NewMetricBuffer`** (fixes stale caches).
5. **C5 — re-queue metrics on DB failure** (durability).
6. **C6 — wire `alertEng.Start/Stop` + shared baseline** — anomaly/absence rules actually work.
7. **S1 — implement scope filtering in handlers** (the silent data-isolation gap).
8. **S2/S3 — backup restore permission + disabled-user check in API-key auth.**
9. **H12/H13 — pagination clamps + `id` tiebreaker.**
10. **H2 — seek an alert-dedup mutex/unique index; H3 — deferred notifications with timeouts.**

Then the remaining H/M items in dependency order (capture lifecycle, WS caps, escalation wiring, migrations robustness, phase-2 typing).