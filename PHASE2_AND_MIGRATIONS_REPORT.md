# Phase2 Terminology & Migration Robustness Report

> **Date:** 2026-08-20
> **Reviewer:** fx (automated)
> **Scope:** `backend/internal/database/phase2.go`, `backend/internal/handlers/phase2.go`, `backend/internal/cache/cached_database.go`, `backend/internal/config/config.go`, `backend/internal/database/database.go`, `backend/internal/database/postgres.go`, `backend/internal/database/migrations.go`, `backend/internal/server/server.go`, and related test files.
> **Branch:** `ds-review`

---

## PART 1 — What "Phase2" Actually Is

### 1.1 Origin

The term "Phase 2" comes from the project's implementation plan (`documentation/implementation_phase2.md`), which describes a feature wave built on top of the "Phase 1" Go backend migration. The documentation defines Phase 2 as:

> *"Phase 2: Campus-Grade Features"* — transforms the product from a generic network monitor into a purpose-built campus network monitoring platform, adding 30+ new tables and 8+ new React pages.

In practice, "Phase2" in the codebase is **not a phase at all** — it is a permanent, production-critical subsystem. It is the generic CRUD layer that powers **two-thirds of the API routes** in the application, covering locations, subnets, contacts, incidents, maintenance windows, status pages, escalation policies, SLA definitions, discovery jobs, ISP links, scheduled reports, roles, users, and more.

### 1.2 What the Code Actually Does

The `phase2` subsystem is a **dynamic resource registry** — a data-driven CRUD engine that maps string resource names to database tables, eliminating the need for a dedicated handler and database method per table.

**Architecture (3 layers):**

```
HTTP Route (server.go)
    │  phase2.List("contacts"), phase2.Create("subnets"), etc.
    ▼
Phase2Handler (handlers/phase2.go)
    │  Parse filters, cursor pagination, buildLocationTree, PublicStatusHTML
    │  Calls: h.phase2.ListPhase2(ctx, resource, filters)
    ▼
Phase2Store interface (database/database.go:63-71)
    │  ListPhase2, ListPhase2Cursor, GetPhase2, CreatePhase2, UpdatePhase2, DeletePhase2, Phase2Summary
    ▼
Postgres implementation (database/phase2.go)
    │  phase2Resources map → {Table, Select, Cols, OrderBy}
    │  Builds parameterized SQL dynamically from the registry
```

**The resource registry** (`phase2Resources` map in `database/phase2.go`) defines 26 resources:

| Category | Resources |
|----------|-----------|
| Topology | `locations`, `subnets` |
| Contacts | `contacts`, `device_contacts` |
| Escalation | `escalation_policies`, `escalation_steps`, `oncall_schedules` |
| Incidents | `incidents`, `incident_timeline`, `suppressed_alerts` |
| SLA | `sla_definitions` |
| Status Page | `status_page_services`, `status_page_incidents`, `status_page_incident_updates` |
| Maintenance | `maintenance_windows` |
| Discovery | `discovery_jobs`, `discovery_results` |
| Reports | `scheduled_reports`, `generated_reports`, `notification_log` |
| ISP | `isp_links`, `isp_metrics` |
| RBAC | `roles`, `users`, `user_scopes` |

### 1.3 Scale of Usage

The term "phase2" / "Phase2" appears in **197 lines across 9 Go source files** and is referenced in the CHANGELOG, review plan, and review report. The `Phase2Handler` methods (`List`, `Get`, `Create`, `Update`, `Delete`) are called from **~60 route registrations** in `server.go`, making it the single largest API surface in the application.

---

## PART 2 — Why "Phase2" Is Bad Terminology

### 2.1 Problems

| Problem | Impact |
|---------|--------|
| **Misleading** | "Phase2" implies a temporary, staging, or incomplete feature set. In reality it is a permanent, production-critical subsystem carrying the majority of API routes. New developers may assume it can be skipped, disabled, or deleted. |
| **Meaningless to outsiders** | The term carries zero semantic information about what the code does. A reviewer, contributor, or security auditor scanning the codebase sees `Phase2Store`, `ListPhase2`, `phase2Resources` with no indication that these manage incidents, contacts, SLAs, or user accounts. |
| **Embedded in user-facing surfaces** | The handler returns `"phase 2 storage is not available"` as a 501 error message (`handlers/phase2.go:28`). This leaks internal project-management jargon to API consumers. |
| **Config coupling** | `Phase2Config` in `config.go` holds Telegram tokens, WhatsApp credentials, SMS gateways, report SMTP settings, ISP monitoring, and status page branding — none of which are "phase 2" concepts; they are notification/integration configuration. |
| **Documentation drift** | `documentation/implementation_phase2.md` still frames everything as future work ("to be implemented"), but the code is fully shipped and in production. The naming creates a false impression of immaturity. |
| **Inconsistent** | Some Phase 2 features have dedicated handlers (`campusH`, `incidentH`, `contactH`, `statusPageH`, `discH`) that bypass the Phase2 CRUD layer entirely. The boundary between "phase2 generic" and "dedicated handler" is arbitrary. |

### 2.2 Recommended Replacement Terminology

The subsystem has two distinct responsibilities that should be named separately:

#### A. The Generic CRUD Engine → `ResourceStore` / `DynamicResource`

The data-driven CRUD layer that maps resource names to tables should be called the **Dynamic Resource** layer. This accurately describes what it does: dynamically resolves a resource name to its table, columns, and sort order, then generates parameterized SQL.

| Current | Proposed |
|---------|----------|
| `Phase2Store` (interface) | `ResourceStore` |
| `Phase2Summary` (struct) | `ResourceSummary` |
| `Phase2Handler` (handler) | `ResourceHandler` |
| `NewPhase2Handler` | `NewResourceHandler` |
| `phase2Resources` (map) | `resourceRegistry` |
| `phase2Resource` (struct) | `resourceDefinition` |
| `getPhase2Resource` | `getResourceDefinition` |
| `ListPhase2` / `GetPhase2` / `CreatePhase2` / `UpdatePhase2` / `DeletePhase2` | `ListResources` / `GetResource` / `CreateResource` / `UpdateResource` / `DeleteResource` |
| `ListPhase2Cursor` | `ListResourcesCursor` |
| `Phase2Summary` (method) | `ResourceSummary` |
| `"phase 2 storage is not available"` | `"resource storage is not available"` |
| `phase2Inner` (cached_database.go) | `resourceStoreInner` |

#### B. The Config → `IntegrationsConfig` / `NotificationConfig`

`Phase2Config` should be split or renamed to reflect its actual contents:

| Current | Proposed |
|---------|----------|
| `Phase2Config` | `IntegrationsConfig` (or split into `NotificationConfig` + `ReportConfig` + `DiscoveryConfig`) |
| `cfg.Phase2.TelegramBotToken` | `cfg.Integrations.TelegramBotToken` |

### 2.3 Migration Strategy

A pure rename is mechanical but touches 9 files and ~197 lines. The recommended approach:

1. **Rename the interface, types, and methods** in `database/database.go` and `database/phase2.go`.
2. **Rename the handler** in `handlers/phase2.go`.
3. **Update the cache delegation** in `cached_database.go`.
4. **Rename the config struct** in `config/config.go` and `cmd/server/main.go`.
5. **Update all route registrations** in `server.go` (variable name `phase2` → `resources`).
6. **Update tests** that reference `Phase2` / `phase2`.
7. **Rename the files** `phase2.go` → `resources.go` (database) and `phase2.go` → `resource_handler.go` (handlers).
8. **Update the user-facing error message** from `"phase 2 storage is not available"` to `"resource storage is not available"`.

This is a low-risk refactor — no logic changes, just identifiers. The existing test suite (build + vet + test + lint) validates correctness after the rename.

---

## PART 3 — C7/C8: Migration System Robustness

### 3.1 Current State

The migration runner lives in `postgres.go:83-115` (`RunMigrations`) and `migrations.go` (44 migration entries as a flat `[]string`). The `splitStatements` helper lives at `postgres.go:117-153`.

### 3.2 C7 — Migration Versioning Is Positional

**Current behavior** (`postgres.go:88-89`):

```go
for i, sql := range migrations[1:] {
    version := int64(i + 2) // 1-based, but index 0 is already applied above
```

The version number is derived from the slice index — `migrations[1]` is version 2, `migrations[2]` is version 3, etc. There are no explicit version structs, no checksums, and no human-readable descriptions.

**Risk:** Inserting a migration in the middle of the list silently shifts all subsequent version numbers. For example, inserting a new migration at index 10 makes what was version 11 become version 12. A database that already recorded version 11 as applied will skip the new migration entirely (the `SELECT EXISTS` check returns true for version 11), leaving the schema in an inconsistent state.

**Mitigating factor:** The `SELECT EXISTS` + `ON CONFLICT DO NOTHING` pattern means migrations are idempotent on re-apply. But this does not protect against the reordering-skip scenario above.

**Recommended fix:**

```go
type Migration struct {
    Version    int64
    Name       string  // human-readable description
    SQL        string
    Checksum   string // sha256 of SQL, stored and verified on re-apply
}

var migrations = []Migration{
    {Version: 1, Name: "schema_migrations table", SQL: `CREATE TABLE IF NOT EXISTS ...`},
    {Version: 2, Name: "devices table", SQL: `CREATE TABLE ...`},
    // ...
}
```

The `schema_migrations` table should add a `checksum TEXT` and `name TEXT` column. On apply, the runner verifies that any existing record's checksum matches the migration's checksum — a mismatch means the migration was modified after being applied (a serious integrity error that should halt startup).

### 3.3 C8 — Migrations Not Atomic / No Advisory Lock

**Current behavior** (`postgres.go:100-108`):

```go
for _, stmt := range splitStatements(sql) {
    if _, err := p.pool.Exec(ctx, stmt); err != nil {
        // ...
        return fmt.Errorf("migration %d: %w\nSQL: %s", version, err, stmt)
    }
}
if _, err := p.pool.Exec(ctx,
    `INSERT INTO schema_migrations(version) VALUES($1) ON CONFLICT DO NOTHING`, version); err != nil {
    // ...
}
```

Two problems:

1. **No transaction:** Each statement in a migration is executed individually. If statement 3 of 5 fails, statements 1-2 are already committed, but the version is never recorded (the `INSERT` at the end never runs). The next startup re-attempts all 5 statements — statements 1-2 may fail with "already exists" errors (non-fatal for `CREATE TABLE IF NOT EXISTS`, but fatal for `CREATE INDEX` without `IF NOT EXISTS` or `ALTER TABLE ADD COLUMN` without `IF NOT EXISTS`).

2. **No advisory lock:** Two server instances starting concurrently can both pass the `SELECT EXISTS` check (TOCTOU race) and both attempt to apply the same migration simultaneously.

**Recommended fix:**

```go
func (p *Postgres) RunMigrations(ctx context.Context) error {
    // Acquire a session-level advisory lock so only one instance runs migrations.
    if _, err := p.pool.Exec(ctx, `SELECT pg_advisory_lock(727280)`); err != nil {
        return fmt.Errorf("acquire migration lock: %w", err)
    }
    defer p.pool.Exec(ctx, `SELECT pg_advisory_unlock(727280)`)

    for _, m := range migrations {
        // Check if already applied + verify checksum
        var existingChecksum *string
        err := p.pool.QueryRow(ctx,
            `SELECT checksum FROM schema_migrations WHERE version=$1`, m.Version).
            Scan(&existingChecksum)
        if err == nil && existingChecksum != nil {
            if *existingChecksum != m.Checksum {
                return fmt.Errorf("migration %d (%s) checksum mismatch: stored %s, expected %s",
                    m.Version, m.Name, *existingChecksum, m.Checksum)
            }
            continue // already applied, checksum OK
        }

        // Apply in a single transaction
        tx, err := p.pool.Begin(ctx)
        if err != nil {
            return fmt.Errorf("begin tx for migration %d: %w", m.Version, err)
        }
        for _, stmt := range splitStatements(m.SQL) {
            if _, err := tx.Exec(ctx, stmt); err != nil {
                tx.Rollback(ctx)
                return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
            }
        }
        if _, err := tx.Exec(ctx,
            `INSERT INTO schema_migrations(version, name, checksum) VALUES($1,$2,$3)`,
            m.Version, m.Name, m.Checksum); err != nil {
            tx.Rollback(ctx)
            return fmt.Errorf("record migration %d: %w", m.Version, err)
        }
        if err := tx.Commit(ctx); err != nil {
            return fmt.Errorf("commit migration %d: %w", m.Version, err)
        }
    }
    return nil
}
```

### 3.4 N6 — splitStatements Is a Latent Corruption Risk

**Current behavior** (`postgres.go:117-153`):

The hand-rolled `;` splitter handles single-quote and double-quote escaping, but has **no awareness of dollar-quoted strings** (`$$...$$`), line comments (`-- ...`), or block comments (`/* ... */`).

**Risk:** A future migration that uses a PL/pgSQL function body (`DO $$ BEGIN ... END $$;`) or a `CREATE FUNCTION ... LANGUAGE plpgsql AS $$ ... $$` will be silently mis-split at the first `;` inside the function body. The resulting fragments are invalid SQL — they will fail mid-application, leaving the schema half-applied with no version recorded (amplified by C8's lack of transactions).

**Current migrations:** The existing 44 migrations do not use dollar-quoting, so the bug is latent. But it is an accident waiting to happen — the first migration that needs a trigger function, a computed column default, or a `DO` block will silently corrupt the schema.

**Recommended fix:**

The simplest safe approach is to **require one statement per migration entry** — eliminate the splitter entirely. Multi-statement migrations are split into separate entries with consecutive version numbers. This is verbose but eliminates the parsing risk entirely.

Alternatively, use a proper SQL parser. The `pgconn` library's `ExecParams` can handle multi-statement strings natively (Postgres parses them server-side), so the splitter could be removed and the entire migration SQL passed to a single `tx.Exec(ctx, m.SQL)` call. This is the cleanest fix — it leverages Postgres's own parser and handles all quoting correctly.

### 3.5 Implementation Plan

Both C7/C8 and the Phase2 rename are mechanical refactors with no logic changes. The recommended order:

1. **C8 first (advisory lock + transaction):** Add `pg_advisory_lock` + per-migration `BEGIN/COMMIT`. This is the highest-value change — it prevents concurrent-startup corruption. No schema changes needed (existing `schema_migrations` table works).

2. **N6 next (drop splitStatements):** Replace the `splitStatements(sql)` loop with a single `tx.Exec(ctx, m.SQL)` per migration. Postgres parses multi-statement strings natively. This eliminates the dollar-quote/comment parsing bug.

3. **C7 (explicit version structs + checksum):** Add `Migration` struct with `Version`, `Name`, `SQL`, `Checksum`. Add `checksum` and `name` columns to `schema_migrations` (a new migration that `ALTER TABLE ADD COLUMN IF NOT EXISTS`). Verify checksum on re-apply.

4. **Phase2 rename (last):** Mechanical identifier rename across 9 files. No logic changes. Validated by the full test suite.

### 3.6 Verification

All changes should be verified with:
- `go build ./...`
- `go vet ./...`
- `go test ./... -count=1 -timeout 180s`
- `golangci-lint run ./...`
- For C8: a concurrent-startup test that launches two `RunMigrations` calls in parallel goroutines and verifies only one applies.
- For N6: a migration that uses `DO $$ ... $$` to verify it applies correctly.
- For the rename: the existing test suite is sufficient (no new tests needed for a pure rename).

---

## Summary

| Item | Type | Risk | Effort | Priority |
|------|------|------|--------|----------|
| Phase2 → ResourceStore rename | Terminology | Low | Medium (9 files, ~197 lines) | Medium |
| C8 Advisory lock + transaction | Correctness | High (silent corruption) | Small (1 function) | High |
| N6 Drop splitStatements | Correctness | High (latent schema corruption) | Small (remove 1 function, 1 line change) | High |
| C7 Explicit version structs + checksum | Correctness | Medium (silent skip on reorder) | Medium (migrations.go + schema_migrations ALTER) | Medium |

The C8 + N6 fixes are the highest priority — they prevent silent schema corruption and can be implemented in a single focused PR. The Phase2 rename and C7 version structs can follow as separate PRs.


