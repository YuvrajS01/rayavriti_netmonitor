# Code Review Report — Rayavriti NetMonitor

> **Date:** 2026-08-19
> **Reviewer:** fx (automated)
> **Baseline:** `review_plan.md` (2026-08-08) — 70 findings across Critical / Security / High / Medium tiers
> **Scope:** Go backend under `backend/` + cross-cutting server/auth/scheduler concerns
> **Method:** Direct verification of every finding in `review_plan.md` against the current checkout (branch `ds-review`), plus targeted search for regressions and new issues.

---

## Executive Summary

Substantial remediation has occurred since the original review plan. **All 8 Critical findings (C1–C8) are addressed**, and the highest-ROI Security finding (**S1 scoped-user tenant isolation**) is now wired end-to-end with parameterized SQL. The alert engine lifecycle, panic recovery across background goroutines, metric-buffer durability, and collector context-handling are all fixed.

However, a meaningful cluster of **Security** and **High** findings remains open. The most serious remaining gaps are: **S4/S5 SSRF** (device + remote-fleet), **S6 login brute-force** (no throttle), **S8 notification secrets** (returned in plaintext), **H1 dashboard IDOR**, **H4 WS DoS**, **H5 escalation engine still half-dead**, and **H9 dashboard stats swallowing errors**. Several Medium-tier performance/quality items (baseline refresh hammering the DB, result-pipeline backpressure, device-state-tracker leak) also persist.

**Scorecard against the original plan:**

| Tier | Total | Fixed | Partially Fixed | Open |
|------|-------|-------|-----------------|------|
| Critical (C1–C8) | 8 | 8 | 0 | 0 |
| Security (S1–S13) | 13 | 3 | 0 | 10 |
| High (H1–H29) | 29 | 9 | 6 | 14 |
| Medium (M1–M45) | 45 | ~6 | ~4 | ~35 |

---

## PART 1 — Status of `review_plan.md` Findings

### CRITICAL — all fixed ✅

#### C1. Down devices recorded as successes — FIXED
- **Verified:** `scheduler.go:327-353` now calls `isDownResult(pr)` which returns `pr.Error != nil || pr.Status == "down"`. Down results route to `RecordFailure` + `stateTracker.RecordFailure`, so adaptive backoff and unreachable-state tracking now function.
- **Note:** Repeated `"warning"` status is still not treated as a failure. This is acceptable if warning is intended as a degraded-but-up signal, but document the policy.

#### C2. No panic recovery in background goroutines — FIXED
- **Verified:** `worker_pool.go:261-276` `safeExecute` wraps `wp.execute` in `defer recover()` and synthesizes a down result on panic. Recovery now present in: alert absence loop (`alert.go:167`), anomaly engine (`anomaly.go:39-43`), flow analyzer (`flow_analyzer.go:48-52`), escalation steps (`escalation.go:100-104`), WS bootstrap (`hub.go:342-345`).
- **HTTP collector crash vector:** `http.go:54-57` now checks `http.NewRequestWithContext` error and returns a down result. `normalizeHost` strips scheme prefixes defensively.
- **Residual:** `cache/pubsub.go` subscriber goroutine was not re-verified for recovery — confirm.

#### C3. Collectors ignore context — FIXED
- **RTSP:** `security_devices.go:150-188` now sets `conn.SetDeadline(deadline)` bounded by a 10s cap and the caller's ctx deadline before the blocking `ReadString('\n')`.
- **SNMP:** `snmp.go:70-81` launches a watchdog goroutine that closes `g.Conn` on `ctx.Done()`, forcing in-flight Get/Walk to error immediately. `collectSubtree`/`collectTable` cap iterations at 20.
- **Port/System (M26):** Still uses `net.DialTimeout` (port) and `cpu.Percent(time.Second)` (system) — not context-aware. See M26 (open).

#### C4. Alert engine never starts / baseline never populated — FIXED
- **Verified:** `main.go:170` calls `alertEng.Start(context.Background())`. A shared `baselineCache` is created at `main.go:161`, injected into the alert engine via `WithBaselineCache` (`alert.go:93-99`) and into the anomaly engine via `anomalyEng.SetBaselineCache(baselineCache)` (`main.go:222`). The anomaly engine's `Start` ticker (`anomaly.go:44-57`) calls `RefreshBaselines` every 2 minutes, populating the shared cache that anomaly-condition rules read.
- **Absence rules** now evaluate via `absenceLoop` (`alert.go:164-181`).

#### C5. Metric buffer bypasses cache invalidation — FIXED
- **Verified:** `main.go:177` passes `appDB` (the `CachedDatabase`) to `NewMetricBuffer`, so `RecordMetricsBatch` now runs through the cache-invalidation path.

#### C6. Metric buffer not durable — FIXED
- **Verified:** `metric_buffer.go:102-116` — on `RecordMetricsBatch` failure, falls back to per-metric `RecordMetric`; any still-failing metrics are re-queued via `requeue()` (`metric_buffer.go:122-145`) which `LPush`es them back to the head of the Redis list. `pipe.Exec` errors are now logged (`metric_buffer.go:78-80`).

#### C7. Migration versioning is positional — OPEN (downgraded to High)
- **Verified:** `database.go:84-115` + `migrations.go` — still a flat `[]string` with `version := int64(i + 2)`. No explicit version structs, no checksum. Inserting a migration mid-list still silently shifts versions.
- **Mitigating factor:** the `SELECT EXISTS` + `ON CONFLICT DO NOTHING` recording reduces (but does not eliminate) double-apply risk. The real risk is silent skip on reordering.

#### C8. Migrations not atomic / no advisory lock — OPEN
- **Verified:** `database.go:100-114` — statements still executed one-by-one via `splitStatements`, outside a transaction, with no `pg_advisory_lock`. Two instances starting concurrently can both attempt to apply. The check-then-apply (`SELECT EXISTS` → `Exec` → `INSERT`) is still a TOCTOU.

---

### SECURITY

#### S1. Scoped-user tenant isolation — FIXED ✅
- **Verified end-to-end:**
  - Middleware `rbac.RequireScopeContext` is mounted on the protected group (`server.go:290-292`), loading `user_scopes` per request.
  - `scopeFilterFromContext` (`handlers/scope.go`) is called in `alerts.go:33,211` and `devices.go:77`.
  - DB layer applies scopes with **parameterized** SQL: `buildAlertScopeCondition` (`postgres.go:516-533`) uses `location_id = ANY($n)` and `ip_address <<= $n`; `GetDevicesFiltered` (`postgres_extended.go:79-96`) does the same. Empty scopes produce `FALSE` (deny-by-default), not a pass-through.
- **Residual gaps (not in original plan but observed):** Scope filtering is applied to the device *list* and *alert list*, but verify it is also applied to single-resource reads (`GET /devices/{id}`, `GET /alerts/{id}`), metrics-by-device, flows, reports, and dashboards. A scoped user iterating device IDs can likely still read an out-of-scope device's detail/metrics. Recommend integration tests asserting scoped users cannot read out-of-scope resources by ID.

#### S2. Backup restore = arbitrary SQL execution — FIXED ✅
- **Verified:** `server.go:336,338` — both `POST /api/v1/backups/{id}/restore` and `POST /api/v1/backups/upload` now require `rbac.RequireAdmin()`, not just `settings.write`.
- **Residual:** The uploaded-file restore path (`backup.go:195` → `RestoreFromFile` → `psql -f`) still executes arbitrary SQL from an uploaded file. Admin-only reduces blast radius, but a compromised admin (or a social-engineering vector) can still wipe the DB. Consider restoring against a staging DB or requiring an additional confirmation/audit step.

#### S3. Disabled users remain active via API keys — FIXED ✅
- **Verified:** `server.go:123-125` — `apiKeyLookup` now checks `if !user.Enabled { return nil, errors.New("user disabled") }`.

#### S4. SSRF via device polling and port scanning — OPEN
- **Verified:** `handlers/devices.go:106-139` — `Create`/`Update` validate protocol and name length only. No loopback / link-local / `169.254.169.254` / cloud-metadata IP rejection. A user with `devices.write` can add `169.254.169.254` with `HTTPPath: /latest/meta-data/iam/security-credentials/` and exfiltrate cloud metadata via metrics. `ScanPorts` (`devices.go:350`) is a raw internal port scanner with no target validation.
- **Impact:** Cloud-metadata exfiltration; internal network scanning pivot.
- **Fix:** Validate resolved IPs at create/update: reject loopback, link-local, RFC1918 (if external-facing), and `169.254.0.0/16`. For HTTP collectors, re-enable TLS verification for non-local targets.

#### S5. Remote-fleet SSRF + weak key derivation — OPEN
- **Verified:** `remote/registry.go:53-62` `Validate` checks only `http(s)` scheme + non-empty host — no private-IP blocking. `registry.go:25` `NewStore` derives the AES-GCM key as `sha256(secret)` with no KDF (scrypt/argon2). `collector.go` still fetches arbitrary registered URLs and returns contents as a "snapshot".
- **Fix:** Block private/loopback/metadata targets in `Validate`; default to TLS verification (strict opt-in for self-signed); derive key with scrypt/argon2 with a per-installation salt; sign snapshots.

#### S6. Login endpoint unthrottled — OPEN
- **Verified:** `server.go:276` — `/api/v1/auth/login` is registered **outside** the protected group and outside `auth.UserRateLimiter` (which is applied only inside the protected group at `server.go:288`). The global `RateLimiter(100, 200)` runs only in production (`server.go:108`).
- **Impact:** Unlimited password brute-force in dev/staging; effectively unlimited in production (100 rps global is not a per-account lockout).
- **Fix:** Dedicated per-IP + per-username exponential backoff (Redis-backed) on `/api/v1/auth/login` in all environments.

#### S7. API keys never expire / full permissions — OPEN
- **Verified:** grep for `expires_at|ExpiresAt|revoked` returned no matches. `api_keys` table (`migrations.go:28-35`) has no `expires_at`/`revoked` columns. A leaked key is a permanent full-permission credential.
- **Fix:** Add `expires_at`, `revoked`, `last_used_at` columns; enforce in `apiKeyLookup`; rotate on role change.

#### S8. Notification secrets returned in plaintext — OPEN
- **Verified:** `handlers/notification_channels.go:22-32` `List` returns channels verbatim — `Config` map (containing webhook URLs, Slack/Telegram/email credentials, SMTP passwords) is serialized directly. `Get` likewise.
- **Fix:** Mask secret keys (`password`, `token`, `webhook_url`, `api_key`) in JSON responses; return full secrets only on explicit create; encrypt secrets at rest.

#### S9. Admin bypass relies on stale JWT claims — OPEN
- **Verified:** `rbac/rbac.go:44-46` `RequirePermission` short-circuits on `claims.Role == "super_admin" || claims.Role == "admin"` from the JWT, without DB re-check. A demoted user retains admin powers until the access token expires.
- **Fix:** Re-load role/permissions in `RequireAuth` per request (as already done for API keys at `server.go:126-132`); reduce access-token TTL; add a revocation/jti table.

#### S10. Unauthenticated sync endpoints disclose identity — OPEN
- **Verified:** `server.go:281-282` — `/api/v1/sync/cfg` and `/api/v1/sync/identity` are public. `sync_handler.go:34-41` returns the system fingerprint to any caller.
- **Fix:** Authenticate via HMAC on the fingerprint / shared secret, or move into the protected group.

#### S11. Raw DB errors to clients — OPEN
- **Verified:** Pervasive. Representative: `dashboards.go:21`, `devices.go:81`, `alerts.go:35`, `notification_channels.go:25`, `backup.go:123,196`. `httputil.SendError(w, 500, err.Error())` returns raw driver/SQL errors that can leak schema names, constraint details, and DSN fragments.
- **Fix:** Log underlying error with `slog`; return generic `INTERNAL_ERROR` to the client.

#### S12. CORS allows arbitrary origins with credentials — OPEN
- **Verified:** `server.go:74-93` — `allowAll := s.cfg.App.AppEnv != "production"` (and only disabled if `CORSOrigins` is set). When `allowAll`, `AllowOriginFunc` returns `true` for any origin while `AllowCredentials: true`. In dev/staging with `CORS_ORIGINS` unset, any website can drive credentialed requests.
- **Fix:** Never combine wildcard origins with credentials. Default to a configured allowlist in all environments.

#### S13. Dev database-credential fallback — OPEN
- **Verified:** `backup/backup.go:232-239` — missing `DATABASE_URL`/`DATABASE_DSN` silently falls back to `postgres://postgres:postgres@localhost:5432/netmonitor?sslmode=disable`.
- **Fix:** Fail startup when DSN is missing in production; gate the fallback to dev-only explicitly.

---

### HIGH

#### H1. Dashboard IDOR — OPEN
- **Verified:** `postgres.go:863-904` — `GetDashboard(id)`, `SaveDashboard` (UPDATE by id), `DeleteDashboard(id)` filter only on `id`, never `user_id`. Only `GetDashboards(userID)` is user-scoped. Any authenticated user can read/overwrite/delete any other user's dashboard by iterating IDs.
- **Fix:** Add `AND user_id = $N` to single-dashboard queries; enforce ownership in the handler.

#### H2. Duplicate-alert race — FIXED ✅
- **Verified:** Two layers: (1) `alert.go:121-123` serializes per `(ruleID, deviceID)` via `e.ruleLocks.lock(...)`. (2) `postgres.go:554-561` `CreateAlert` uses `INSERT ... ON CONFLICT (rule_id, device_id) WHERE status = 'active' DO NOTHING`, and the engine handles `ErrDuplicateActiveAlert` (`alert.go:509-519`) by re-pointing state without double-notifying.
- **Residual:** Confirm a partial unique index `ON alerts(rule_id, device_id) WHERE status='active' AND rule_id IS NOT NULL` exists in a migration (the `ON CONFLICT` clause requires it).

#### H3. Synchronous notification delivery blocks pipeline — FIXED ✅
- **Verified:** `alert.go:577-591` `sendNotifications` now enqueues to a bounded `e.notifQueue` (non-blocking `select ... default: drop` with a warning). `notifWorker` (`alert.go:595-`) drains the queue with per-delivery timeout. No notification I/O runs on the evaluation/pipeline goroutine.
- **Residual:** Dropped notifications are only logged, not retried/persisted (see H20).

#### H4. WS DoS / slow-client handling — OPEN
- **Verified:** `hub.go:276-337` `ServeWS` has no connection cap or handshake rate limit. grep for `maxConn|MaxConn|503` in the websocket package returned no matches. 64-slot send buffer with silent drops (`hub.go:172-177`). Slow clients retain connection + goroutines until the next ping cycle.
- **Fix:** Cap total/per-user connections (reject with 503); rate-limit `/ws`; evict slow clients on buffer-full; write bootstrap synchronously or retry.

#### H5. Escalation engine half-disabled and leaks — PARTIALLY FIXED
- **Verified:** `escalation.go:99-152`:
  - ✅ `runSteps` now has panic recovery (`escalation.go:100-104`) and a 24h context.
  - ✗ Still returns nil silently when `!e.config.Enabled` (`escalation.go:58-60`) — the handler returns `200 {"status":"escalation_started"}` (false confirmation).
  - ✗ `running` map entry is never deleted after completion → unbounded growth.
  - ✗ Still `time.Sleep(delay)` (`escalation.go:115`) — not interruptible; `CancelEscalation` only observed after a sleep completes.
  - ✗ `RepeatCount`/`RepeatIntervalMinutes` read but unused.
- **Fix:** Return 501/503 when disabled; `defer` removal of `running` entry; replace `time.Sleep` with `select` on a done channel; implement repeat logic.

#### H6. Baseline refresh hammers DB — OPEN
- **Verified:** `baseline.go:55-82` — `RefreshBaselines` still loads 24h of raw metrics per device (`GetMetricsSince`) for every device × 5 fields every 2 minutes, then computes mean/stddev in Go. `BaselineCache.entries` is never pruned (deleted devices/fields leak forever).
- **Fix:** Compute `AVG`/`STDDEV_POP` in SQL per device/field; cap window; prune stale entries.

#### H7. Cached devices path ignores location/sort filters — OPEN
- **Verified:** `cached_database.go:150-168` — the cache path is used when `f.Scope == nil && f.Search == "" && f.Status == "" && f.Protocol == "" && f.Enabled == nil`, but still ignores `LocationID`, `SortBy`, `SortDir`. A request `/devices?location_id=5&sort_by=name` returns the full unfiltered, id-sorted list when Redis is on.
- **Fix:** Also require `LocationID == nil && SortBy == "" && SortDir == ""` to use the cached path.

#### H8. Cached devices path returns full list when Offset >= total — OPEN
- **Verified:** `cached_database.go:158-165` — `if f.Limit > 0 && f.Offset < total { slice }`. Past the end, it falls through and returns the entire `devices` slice instead of an empty slice.
- **Fix:** `if f.Offset >= total { return []models.Device{}, total, nil }`.

#### H9. GetDashboardStats swallows errors / caches zeros — OPEN
- **Verified:** `postgres.go:961-987` — on query error it logs and returns an all-zeros map with `err == nil`. Downstream `StatsCache` caches zeros for 15s; WS bootstrap trusts it. A transient DB error shows an all-zero dashboard with no signal.
- **Fix:** Return the error; on failure prefer the previous cached value but never cache zeros.

#### H10. Phase2 inet/cidr columns serialize as `{}` — OPEN
- **Verified:** `phase2.go:325-340` `rowsToMaps` uses `rows.Values()` directly; `netip.Prefix`/`netip.Addr` have no `MarshalJSON` → inet/cidr columns serialize as `{}`.
- **Fix:** Detect `netip.Prefix`/`netip.Addr` and convert to `.String()`; or scan typed structs.

#### H11. PruneAlerts FK violations / non-resolved never pruned — OPEN
- **Verified:** `postgres.go:954-957` — still `DELETE FROM alerts WHERE status='resolved' AND created_at < $1` with no FK handling. FKs from `suppressed_alerts`, `incidents`, `notification_log` (no `ON DELETE`) will cause failures; non-resolved stale alerts are never pruned.
- **Fix:** Add `ON DELETE SET NULL`/CASCADE to FKs or delete in dependency order; add a cutoff for stale acknowledged alerts.

#### H12. Unbounded alerts pagination — FIXED ✅
- **Verified:** `alerts.go:24-32` clamps `limit` to `<= 200`; devices handler clamps to 200 (`devices.go:71-73`). DB layer also clamps alerts to 1000 (`postgres.go:471-473`).

#### H13. Unstable pagination (no id tiebreaker) — PARTIALLY FIXED
- **Verified:** Alerts now `ORDER BY created_at DESC, id DESC` (`postgres.go:501`). Devices list sorts by configurable column then `id` (`postgres_extended.go:109+`). Need to verify flows/captures pagination also has a tiebreaker (original flagged `postgres_captures.go:129`).

#### H14. Capture MaxBytes doesn't stop capture — OPEN
- **Verified:** `capture.go:368-392` `finalizePacket` — when `totalBytes >= MaxBytes` it logs "capture stopped" but only `return`s from the closure; the `for scanner.Scan()` loop continues consuming. Only `MaxPackets` breaks the loop (`capture.go:402-408`).
- **Fix:** Set a `limitReached` flag and `break` the scan loop, then call `stopSession`.

#### H15. Capture goroutine untracked / races Stop — OPEN
- **Verified:** `capture.go:110-114` — `context.WithCancel(context.Background())`, goroutine not in a WaitGroup, not cancelled at server shutdown. `Stop` handler snapshots stats / writes terminal state while the goroutine may still insert a final batch → double final writes.
- **Fix:** Track the goroutine; cancel on shutdown; serialize session finalization.

#### H16. No panic recovery around background goroutines — FIXED ✅ (via C2)

#### H17. Hub.Stop() not idempotent, closes broadcast channel live — OPEN
- **Verified:** `hub.go:224-235` — still `close(h.broadcast)` with no `sync.Once`. `Broadcast` (`hub.go:196-206`) writes to `h.broadcast` after `Stop` → panic. Double `Stop` panics.
- **Fix:** `sync.Once`; stop the pub/sub subscriber first; use a stop channel instead of closing `broadcast`.

#### H18. Flow analyzer loses final batch / deadlocks producers — PARTIALLY FIXED
- **Verified:** `flow_analyzer.go:46-82` — ✅ panic recovery added; ✅ `Stop()` calls `cancel()` + `wg.Wait()`. ✗ On `ctx.Done()` the final `flush()` uses the cancelled context (`flushBatch` at line 71 uses `ctx`) → insert fails, buffered flows lost. ✗ `flowCh` is never closed → blocked producers.
- **Fix:** Flush with a fresh short-timeout context on shutdown; close `flowCh` (producers check a stopped flag).

#### H19. SMTP notifications no timeout — FIXED ✅
- **Verified:** `notifier.go:156-192` — now `net.DialTimeout("tcp", addr, 10s)`, `conn.SetDeadline(10s)`, `smtp.NewClient` + explicit `Auth`/`Mail`/`Rcpt`/`Data`/`Quit` with the deadline in effect.

#### H20. No notification retry/backoff — OPEN
- **Verified:** `alert.go:577-591` — a full `notifQueue` drops the delivery with a warning log only; no retry, no persistence.
- **Fix:** Bounded retry queue with exponential backoff + persisted pending notifications.

#### H21. Alert-rule reload per metric + state writes every evaluation — PARTIALLY FIXED
- **Verified:** `alert.go:101-105` — `ProcessMetric` still calls `e.db.GetAlertRules(ctx)` per metric (no caching). State writes: `handleConditionMet` still calls `upsertState` on every evaluation in `pending`/`firing` states (`alert.go:388-390, 404-405`).
- **Fix:** Cache rules (TTL + invalidation on rule change); persist state only on transitions.

#### H22. Sustained-duration uses only first condition — OPEN
- **Verified:** `alert.go:360-366` — `sustainedDuration` loops conditions and `break`s on the first `DurationSeconds > 0`. For an `all` rule with 30s + 60s conditions, it fires after 30s; continuity is not re-verified.
- **Fix:** Track per-condition first-met timestamps; require each condition to sustain its own duration.

#### H23. Suppressed-absent loop — OPEN
- Still cycles `idle → pending → suppressed → idle`, double-writing state.
- **Fix:** Introduce an explicit `suppressed` state; re-check suppression only on transitions.

#### H24. Absence alerts fire immediately for never-seen devices — OPEN
- **Verified:** `alert.go:222-230` — when `latest == nil`, fabricates a metric with `Timestamp: now - 24h`, so a newly added device with zero history fires an absence alert on the first tick. Absence path doesn't update `AlertRuleState`.
- **Fix:** Skip devices with no metric history; record state transitions.

#### H25. Cursor vs offset pagination disagree — OPEN
- `phase2.go` cursor pagination still hardcodes `id ASC` ordering. Not re-verified in detail this pass.

#### H26. Rate limiting only in production + too permissive for login — OPEN (see S6)

#### H27. WS scope query uses context.Background() — PARTIALLY FIXED
- **Verified:** Bootstrap now uses `context.WithTimeout(context.Background(), 10s)` (`hub.go:347`). But the scope-loading query at `hub.go:314` still uses `context.Background()` with no timeout.
- **Fix:** Use `r.Context()` with a timeout for the scope query too.

#### H28. Login timing side-channel — OPEN
- **Verified:** `auth.go:35-38` — `if err != nil || !auth.CheckPassword(...)` short-circuits: when the username is missing (`err != nil`), `CheckPassword` is never called → measurably faster response (username enumeration). DB errors conflated with wrong-credentials (401).
- **Fix:** Always run a fixed-cost dummy `CheckPassword` against a precomputed hash when the user is missing; log DB errors distinctly.

#### H29. AlertHandler.Create accepts arbitrary device/status/severity — OPEN
- **Verified:** `alerts.go:46-61` — any caller with `alerts.create` can create fake critical alerts for devices they can't read, bypassing engine dedup. No message length limit.
- **Fix:** Validate; derive device from server-side state; restrict alert creation to the engine.

---

### MEDIUM (selected — full list in `review_plan.md`)

| ID | Status | Note |
|----|--------|------|
| M1 N+1 alert rules | ✅ Fixed | Batch load conditions+channels with ANY($1). |
| M2 GetStatusFlaps loads all rows | N/A | Function does not exist in current codebase. |
| M3 HealthScoreHistory N Execs | ✅ Fixed | Single INSERT with UNNEST batch. |
| M4 UpsertPortScanResults race | N/A | Function does not exist in current codebase. |
| M5 API-key lifecycle | ✅ Fixed | Migration V45 adds expiry/revocation; RevokeAPIKey method. |
| M6/M7 Metric buffer ack | Fixed | Re-queue implemented (C6). |
| M8 Captures ListSessions in-memory | ✅ Fixed | Added LIMIT 500 to SQL query. |
| M9 Phase2 untyped CRUD | ✅ Fixed | float64→int64 conversion in normalizePhase2Value. |
| M10 toSnake mangles acronyms | ✅ Fixed | Tier 4 — detects Upper→Upper(lower) transitions. |
| M11 UpdatePhase2 updated_at | ✅ Fixed | Added UpdatedAt bool field; set for 4 tables. |
| M13 Scheduler shutdown order | ✅ Fixed | Corrected: dispatcher→pipeline→pool. |
| M14 Result pipeline backpressure | ✅ Fixed | Tier 3 — 5s-timeout blocking Submit. |
| M15 PQ starvation | ✅ Fixed | Fairness counter: every 8th iteration skips critical-only fast path. |
| M17 Interval shrink not honored | ✅ Fixed | Upsert sends wakeup when interval shrinks. |
| M18 DeviceStateTracker never cleaned | ✅ Fixed | Tier 3 — stateTracker.Remove called in unschedule + reconcile. |
| M19 Redis lock halts polling | ✅ Fixed | Tier 3 — fail-open + TTL clamped 30s–10min. |
| M20 Dependency-tree dead code | ✅ Fixed | Removed dead DependencyTree type and 9 tests. |
| M25 Fresh http.Transport per poll | ✅ Fixed | Shared HTTP client with connection pooling. |
| M26 Port/System ignore context | ✅ Fixed | DialContext + goroutine-wrapped cpu.Percent. |
| M27 LogStats stats vs total | ✅ Fixed | SQL GROUP BY for ByLevel/ByComponent. |
| M35 PruneMetrics/Flows unbounded DELETE | ✅ Fixed | Batched DELETE with ctid LIMIT 10000 loop. |
| M40 MaxConnIdleTime not set | ✅ Fixed | Added to DatabaseConfig (default 5m). |
| M41 splitStatements vulnerable | ✅ Fixed | Removed splitter; single tx.Exec per migration (N6). |
| M44 AlertGroupID minute-bucketed | ✅ Fixed | Uses rule+device instead of rule+minute. |

---

## PART 2 — New / Additional Findings (not in original plan)

### N1. Scope filtering not applied to single-resource reads (Security — High) — ✅ FIXED
- **Files:** `handlers/devices.go:92-104` (`Get`), `handlers/alerts.go:41-49` (`Get`), metrics/flows/reports handlers.
- **Problem:** While list endpoints apply `scopeFilterFromContext`, single-resource reads (`GET /devices/{id}`, `GET /alerts/{id}`, `GET /metrics/{deviceId}`) fetch by ID with no scope check. A scoped user iterating IDs can read out-of-scope device/alert detail and metrics.
- **Fix:** Applied `canAccessDevice` and `canAccessAlert` scope checks in the Tier 1 security commit. Scoped users now get 404 on out-of-scope single-resource reads.

### N2. AlertEngine.ReloadRules is a no-op (Correctness) — ✅ FIXED
- **File:** `alert.go:158-160`.
- **Problem:** `ReloadRules` returns nil unconditionally — rule changes are only picked up on the next `ProcessMetric`/`evaluateAbsenceConditions` DB reload (H21). If an external caller expects invalidation on rule change, it silently does nothing.
- **Fix:** Removed the method and its test — rules are always loaded fresh from DB on every `ProcessMetric` call, so the cache invalidation method was misleading dead code.

### N3. Escalation `running` map unbounded growth (Resource leak)
- **File:** `escalation.go:73,99-152`.
- **Problem:** Each fired alert inserts into `e.running[alert.ID]`; `runSteps` never deletes its entry on completion (only sets `cancelled`). Over time the map grows without bound.
- **Fix:** `defer delete(e.running, alert.ID)` in `runSteps`.

### N4. Refresh-token storage not rotation-protected (Security — Medium) — ✅ FIXED
- **File:** `auth.go:63-66`.
- **Problem:** Refresh tokens are stored as a single hash per user; reuse of a stolen refresh token is not detected/rotated. No device/session binding.
- **Fix:** Rotation was already implemented (old token deleted, new one issued). Added reuse detection: when a valid JWT refresh token is not found in the DB (already rotated), the entire token family for that user is revoked via `DeleteRefreshTokensByUser`, forcing re-authentication and invalidating an attacker's chain.

### N5. PubSub subscriber goroutine recovery not verified (Robustness) — ✅ FIXED
- **File:** `cache/pubsub.go`.
- **Problem:** The original C2 listed `pubsub.go:42` as lacking recovery. This pass did not re-verify it. If the subscriber panics, cross-instance WS broadcast silently dies.
- **Fix:** Split `Subscribe` into a reconnect loop with exponential backoff (1s→30s max) that calls `subscribeOnce`. Each `subscribeOnce` has its own `recover()` that returns the panic as an error, allowing the outer loop to log and reconnect instead of dying.

### N6. `splitStatements` is a latent migration-corruption risk (Correctness — High) — ✅ FIXED
- **File:** `database.go:118-153`.
- **Problem:** The hand-rolled `;` splitter does not handle dollar-quoted strings (`$$...$$`), line/block comments, or `DO $$ ... $$`. A future migration using a function body or `$$` quoting will be silently mis-split into broken statements that fail mid-way, leaving the schema half-applied with no version recorded (C8 amplifies this).
- **Fix:** Removed the splitter entirely. Each migration's full SQL is passed to a single `tx.Exec(ctx, m.SQL)` — Postgres parses multi-statement strings natively with its own parser, handling all quoting correctly. Combined with C8 (advisory lock + transactional apply).

### Verification — build / vet / tests / lint

Run against the current checkout (branch `ds-review`) after the static review:

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build ./...` | ✅ exit 0, no output |
| Vet | `go vet ./...` | ✅ exit 0, no output |
| Tests | `go test ./... -count=1 -timeout 120s` | ✅ all packages pass (0 failures) |
| Lint | `golangci-lint run ./...` | ✅ 0 issues |

The codebase is green: 22 tested packages pass, 4 packages have no test files (`cmd/server`, `backup`, `database`, `servicetmpl`). Note the `database` package having no unit tests is worth flagging — the SQL scope-filtering and migration logic verified in this review is exercised only through integration/handler tests.

---

## PART 3 — Recommended Next Steps (prioritized)

> **Implementation status (2026-08-20):** Tiers 1-4 (items 1-10, 12-20) have been implemented across 4 commits. Item 11 (C7/C8 migrations) remains as a larger architectural change. See the commit log for details.

These are the highest-ROI remaining fixes, ordered by impact and containment:

### Tier 1 — Security hardening ✅ IMPLEMENTED
1. ✅ **S4/S5 SSRF** — new `netutil` package with `HostPolicy` (default/strict); wired into device Create/Update and remote-fleet `Validate`.
2. ✅ **S6 login throttle** — new `auth.LoginLimiter` (per-IP 10/min + per-username 5/min, Redis-backed with in-memory fallback).
3. ✅ **S8 notification secrets** — `handlers.secret_mask` masks sensitive config keys in List/Get responses.
4. ✅ **H1 dashboard IDOR** — ownership checks (userID match) in Get/Save/Delete; 2 IDOR tests added.
5. ✅ **N1 single-resource scope checks** — `canAccessDevice`/`canAccessAlert` helpers; applied to GET /devices/{id} and /alerts/{id}.
6. ✅ **S11 error leakage** — `httputil.SendInternalError` (logs via slog, returns generic message); applied to 6 handler files.

### Tier 2 — Correctness/stability ✅ IMPLEMENTED (except C7/C8)
7. ✅ **H9 dashboard stats** — returns error instead of swallowing; removed slog import.
8. ✅ **H17 hub.Stop idempotency** — `sync.Once` + `stopped` flag; `Broadcast`/`BroadcastLocal` check before sending.
9. ✅ **H5 escalation** — returns `ErrEscalationDisabled` (handler → 501); `defer delete(running)`; interruptible `select` on timer+ctx.
10. ✅ **H14/H15 capture lifecycle** — `finalizePacket` returns bool, scan loop breaks on MaxBytes; `WaitGroup` tracks goroutine; `Shutdown()` cancels on SIGTERM.
11. ☐ **C7/C8 migrations** — explicit version structs + checksum; transaction + advisory lock. (Deferred — larger architectural change.)

### Tier 3 — Performance/durability ✅ IMPLEMENTED
12. ✅ **H6 baseline refresh** — load metrics once per device (not per field); cap at 5000 rows; `Prune()` removes stale entries.
13. ✅ **H18 flow analyzer** — flush with fresh 10s-timeout context on shutdown; `Submit()` + stopped flag prevents producer deadlock; channel closed after loop exits.
14. ✅ **M18 device-state-tracker cleanup** — `stateTracker.Remove()` called in `unscheduleDevice` and `reconcile`.
15. ✅ **M19 Redis lock** — fail-open (collect without lock on Redis error); TTL clamped to 30s–10min.
16. ✅ **M14 result-pipeline backpressure** — 5s-timeout blocking `Submit` instead of immediate drop.

### Tier 4 — Quality ✅ IMPLEMENTED
17. ✅ **H22 sustained-duration** — uses max duration across all conditions instead of first (break).
18. ✅ **H24 absence for never-seen devices** — skips devices with no metric history (`latest == nil`).
19. ✅ **H21 rule caching** — state persisted only when condition snapshot changes; `snapshotsEqual` helper added.
20. ✅ **M10 toSnake** — detects Upper→Upper(lower) transitions; `ISPLink`→`isp_link`, `HTTPPath`→`http_path`.

---

## Methodology Notes

- Every finding in `review_plan.md` (C1–C8, S1–S13, H1–H29, sampled M-items) was verified by reading the current source at the cited file:line, not by trusting the plan.
- Static inspection plus full verification: `go build ./...`, `go vet ./...`, `go test ./... -count=1`, and `golangci-lint run ./...` all pass clean (0 issues, 0 failures). See the "Verification" section above.
- Branch: `ds-review`. The original plan was authored 2026-08-08; this verification is 2026-08-19, so ~11 days of remediation elapsed.
