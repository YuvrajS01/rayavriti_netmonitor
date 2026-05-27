# Production Readiness Plan — Rayavriti NetMonitor

Harden the existing Rayavriti NetMonitor monorepo for production deployment on a single-server Docker Compose setup for local network monitoring.

## Decisions (Finalized)

| Decision | Answer |
|----------|--------|
| Scope | Hardening only, no new features |
| Deployment | Single-server Docker Compose, local network |
| HTTPS | Not needed — local network only |
| Fonts | Self-host League Spartan + Space Grotesk |
| Database | Stay on SQLite, add abstraction layer for future Postgres+TimescaleDB migration |
| Data retention | Metrics: 30 days, Flows: 7 days, Resolved alerts: 90 days |
| Users | Single admin, credentials from `.env` |
| Notifications | In-app only (defer external integrations) |

---

## Proposed Changes

8 phases, ordered by dependency and criticality.

---

### Phase 1: Environment & Configuration Hardening

#### [NEW] [.env.example](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/.env.example)
Documented template with all configurable values:
```env
# Server
NODE_ENV=production
PORT=3000

# Security
JWT_SECRET=             # REQUIRED — use: openssl rand -base64 32
ADMIN_USERNAME=admin
ADMIN_PASSWORD=         # REQUIRED — hashed on first boot

# API Keys
DEFAULT_API_KEY=        # Optional — pre-seed an API key

# Database
DB_PATH=./data/netmonitor.db

# Data Retention
METRICS_RETENTION_DAYS=30
FLOW_RETENTION_DAYS=7
ALERTS_RETENTION_DAYS=90

# Network Collectors
NETFLOW_PORT=2055
PORT_DISCOVERY_ENABLED=true
PORT_DISCOVERY_INTERVAL_MINUTES=60
PORT_SCAN_TIMEOUT_MS=900
PORT_SCAN_CONCURRENCY=16
CAPTURE_ENABLED=true
```

#### [MODIFY] [auth.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/auth.ts)
- Remove hardcoded `JWT_SECRET` fallback — fail fast if not set in production.
- Replace SHA-256 password hashing with Node's built-in `crypto.scrypt` (no extra dependency, bcrypt-equivalent security).

#### [MODIFY] [database.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/database.ts)
- Remove hardcoded `admin123` password hash. Read `ADMIN_USERNAME`/`ADMIN_PASSWORD` from env, hash with scrypt at boot.
- Remove hardcoded `sk_live_demo123` API key. Optionally seed from `DEFAULT_API_KEY` env var.
- Make `DB_PATH` configurable via env.
- Add `PRAGMA busy_timeout = 5000` and `PRAGMA synchronous = NORMAL` for resilience.

#### [MODIFY] [index.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/index.ts)
- Load `.env` at startup (Node 22's `--env-file` flag or `dotenv`).
- Add startup validation: fail fast if `JWT_SECRET` or `ADMIN_PASSWORD` are missing.

---

### Phase 2: Database Abstraction & Data Retention

This phase introduces a clean separation between SQL queries and business logic, making a future migration to Postgres+TimescaleDB a driver-level swap rather than a rewrite.

#### [NEW] [server/src/services/db/index.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/db/index.ts)
Create a repository-pattern abstraction layer:
```
db/
├── index.ts          # Re-exports the active adapter
├── types.ts          # Interfaces: IDeviceRepo, IMetricRepo, IAlertRepo, etc.
├── sqlite.ts         # Current SQLite implementation (refactored from database.ts)
└── (postgres.ts)     # Future: Postgres+TimescaleDB adapter
```

The key interfaces:
```ts
interface IMetricRepo {
  record(metric: MetricInput): void;
  getLatest(): Metric[];
  getByDevice(deviceId: number, limit: number): Metric[];
  query(params: MetricQuery): Metric[];
  prune(olderThanDays: number): number;  // returns rows deleted
}
```

Each module in the server imports from `db/index.ts` (which re-exports the SQLite adapter). To switch to Postgres later, you only change the re-export.

> [!NOTE]
> This is a **refactor** of the existing `database.ts` into the new structure. All existing SQL queries remain the same — they just move behind typed interfaces. We keep SQL queries ANSI-compatible where possible (avoid SQLite-specific syntax).

#### [MODIFY] [database.ts → db/sqlite.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/database.ts)
- Refactor into the new `db/sqlite.ts` module implementing the typed interfaces.
- Normalize SQLite-specific functions: use `strftime` carefully, document any SQLite-only syntax.

#### [NEW] [server/src/services/retentionScheduler.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/retentionScheduler.ts)
Automated pruning that runs on a daily interval:
```ts
// Deletes:
//   metrics older than METRICS_RETENTION_DAYS (default 30)
//   flow_records older than FLOW_RETENTION_DAYS (default 7)
//   resolved alerts older than ALERTS_RETENTION_DAYS (default 90)
```
- Runs every 6 hours via `setInterval`.
- Logs how many rows were pruned.
- Uses DELETE with LIMIT batches to avoid locking the DB for too long.

---

### Phase 3: Security Hardening

#### [MODIFY] [index.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/index.ts)
- **Add `helmet` middleware** for security headers (X-Frame-Options, CSP, etc.).
- **Add `cors` middleware** — restrict to `localhost` origins in production.
- **Add body size limit** — `express.json({ limit: '1mb' })`.
- **Add global error handler** — catch unhandled errors, return safe error responses (no stack traces).
- **Add request logging middleware** for audit trail.

#### [MODIFY] [auth.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/auth.ts)
- Add login attempt rate limiting (e.g., max 5 failed attempts per minute per IP).

#### [MODIFY] [client.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/client.ts)
- Add automatic token refresh using `/api/v1/auth/refresh` before expiry.
- Add request timeout (30s default).

---

### Phase 4: Build & Deployment Pipeline

#### [MODIFY] [vite.config.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/vite.config.ts)
- Set `build.outDir` to `../server/dist/public` so Express can serve the SPA.
- Add chunk splitting and minification options.

#### [MODIFY] [index.ts (server)](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/index.ts)
Add production static file serving:
```ts
if (process.env.NODE_ENV === 'production') {
  app.use(express.static(path.join(__dirname, 'public')));
  // SPA fallback for client-side routing
  app.get('*', (req, res, next) => {
    if (req.path.startsWith('/api') || req.path.startsWith('/socket.io')) return next();
    res.sendFile(path.join(__dirname, 'public', 'index.html'));
  });
}
```

#### [MODIFY] [package.json (root)](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/package.json)
Update production scripts:
```json
"start:prod": "NODE_ENV=production node --env-file=.env server/dist/index.js"
```

#### [NEW] [Dockerfile](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/Dockerfile)
Multi-stage build:
- Stage 1: Install deps + build server + client
- Stage 2: Production image with only `dist/`, production `node_modules`, `libpcap-dev`
- Non-root user, health check on `/health`

#### [NEW] [docker-compose.yml](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/docker-compose.yml)
```yaml
services:
  netmonitor:
    build: .
    ports:
      - "3000:3000"
      - "2055:2055/udp"
    volumes:
      - ./data:/app/data
    env_file: .env
    restart: unless-stopped
    cap_add:
      - NET_RAW
    network_mode: host    # Needed for packet capture + local device monitoring
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:3000/health"]
      interval: 30s
      retries: 3
```

#### [NEW] [.dockerignore](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/.dockerignore)
Exclude `node_modules`, `.git`, `data/`, `*.db`, etc. from build context.

---

### Phase 5: Graceful Shutdown

#### [MODIFY] [index.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/index.ts)
- Handle both `SIGTERM` (Docker) and `SIGINT` (Ctrl+C).
- Gracefully close HTTP server (stop accepting new connections).
- Drain WebSocket connections.
- Close SQLite database connection.
- Add shutdown guard to prevent double-cleanup.

---

### Phase 6: Logging & Observability

#### [NEW] [server/src/services/logger.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/services/logger.ts)
Structured logger using `pino`:
- JSON format in production (machine-parsable for log aggregation).
- Pretty format in development.
- Includes `requestId` for traceability.

#### [MODIFY] [index.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/src/index.ts)
- Replace all `console.log`/`console.error` with structured logger.
- Add `pino-http` request logging middleware.
- Enhance `/health` endpoint:
```json
{
  "ok": true,
  "service": "rayavriti-netmonitor",
  "version": "0.1.0",
  "uptime": 3600,
  "database": "ok",
  "timestamp": "..."
}
```

---

### Phase 7: TypeScript & Code Quality

#### [MODIFY] [tsconfig.json (server)](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/server/tsconfig.json)
Enable strict mode:
```json
{
  "strict": true,
  "noImplicitAny": true,
  "useUnknownInCatchVariables": true
}
```

#### [MODIFY] All server `.ts` files
- Convert `require()` / `module.exports` → ES module `import` / `export`.
- Remove trailing `export {};` dead code.
- Add type annotations to untyped function parameters.

---

### Phase 8: Frontend Production Hardening

#### [MODIFY] [index.html](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/index.html)
- Remove Google Fonts CDN link.
- Add `<noscript>` fallback message.
- Add favicon link.

#### [NEW] Self-hosted font files
- Download League Spartan + Space Grotesk WOFF2 files.
- Place in `client/public/fonts/`.
- Create `@font-face` declarations in CSS.

#### [MODIFY] [App.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/App.tsx)
- Add a global `ErrorBoundary` component to catch React rendering crashes.

#### [MODIFY] [client.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/client.ts)
- Make API base URL configurable via `import.meta.env.VITE_API_URL`.
- Add request timeout (30s).

#### [NEW] Favicon
- Generate a simple favicon for the app.

---

## New Dependencies

| Package | Purpose | Where |
|---------|---------|-------|
| `helmet` | Security headers | server |
| `cors` | CORS configuration | server |
| `pino` + `pino-http` | Structured logging | server |
| `pino-pretty` | Dev-only log formatting | server (devDep) |

> [!TIP]
> Node 22 supports `--env-file=.env` natively, so no `dotenv` package is needed.

---

## Execution Order & Estimates

| Phase | Description | Effort |
|-------|-------------|--------|
| 1 | Environment & Config | Medium |
| 2 | Database Abstraction & Retention | Large (biggest phase — refactoring database.ts) |
| 3 | Security Hardening | Medium |
| 4 | Build & Deployment | Medium |
| 5 | Graceful Shutdown | Small |
| 6 | Logging & Observability | Medium |
| 7 | TypeScript & Code Quality | Medium-Large (strict mode will surface issues) |
| 8 | Frontend Hardening | Small-Medium |

---

## Verification Plan

### Automated
```bash
# TypeScript compilation
npm run typecheck

# Full production build
npm run build

# Docker build & run
docker build -t rayavriti-netmonitor .
docker compose up -d
curl http://localhost:3000/health

# Login with env credentials
curl -X POST http://localhost:3000/api/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<from-env>"}'

# Security headers present
curl -I http://localhost:3000/ | grep -E 'X-Frame|Content-Security|X-Content-Type'
```

### Manual / Browser
- Verify SPA loads from Express static serving (not Vite dev server)
- Login flow works with env-configured credentials
- WebSocket reconnects on network disruption
- No console errors in browser DevTools
- Docker container auto-restarts after crash
- Data retention: insert old test data, verify it gets pruned

### Future Postgres Migration Path
When ready to migrate:
1. Add `pg` + `@types/pg` dependency
2. Create `db/postgres.ts` implementing the same interfaces
3. Change the re-export in `db/index.ts`
4. Run the same SQL schema (minor syntax adjustments)
5. For TimescaleDB: convert `metrics` and `flow_records` to hypertables
