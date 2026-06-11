# Rayavriti NetMonitor — Comprehensive Testing & Code Review Plan

> **Scope**: 60 Go source files across 15 internal packages, 0 existing test files
> **Target Coverage**: ≥ 80% line coverage (critical packages ≥ 90%)
> **Go Version**: 1.24 · **Database**: PostgreSQL 16 + TimescaleDB
> **Branch**: `major/backend-go`

---

## Table of Contents

1. [Testing Philosophy & Strategy](#1-testing-philosophy--strategy)
2. [Tooling & Infrastructure](#2-tooling--infrastructure)
3. [Unit Tests — Per Package](#3-unit-tests--per-package)
4. [Integration Tests](#4-integration-tests)
5. [System / End-to-End Tests](#5-system--end-to-end-tests)
6. [Security Testing](#6-security-testing)
7. [Performance & Load Testing](#7-performance--load-testing)
8. [Concurrency & Race Condition Testing](#8-concurrency--race-condition-testing)
9. [Code Review Plan](#9-code-review-plan)
10. [Static Analysis & Linting](#10-static-analysis--linting)
11. [CI/CD Pipeline](#11-cicd-pipeline)
12. [Test Data & Fixtures](#12-test-data--fixtures)
13. [Makefile Targets](#13-makefile-targets)
14. [Appendix: Full Test Matrix](#appendix-full-test-matrix)

---

## 1. Testing Philosophy & Strategy

### Testing Pyramid

```
            ┌──────────────┐
            │   System /   │  ← 5–10 end-to-end scenarios
            │     E2E      │     (Docker Compose, real DB)
            ├──────────────┤
            │ Integration  │  ← 30–50 tests
            │   Tests      │     (testcontainers-go, real Postgres)
            ├──────────────┤
            │  Unit Tests  │  ← 400+ tests
            │  (isolated)  │     (mocks, no I/O, fast)
            └──────────────┘
```

### Guiding Principles

1. **Every exported function gets at least one test** — happy path + primary error path.
2. **Every interface gets a mock** — generated via `mockgen` or hand-written.
3. **Table-driven tests** — use `[]struct{ name string; ... }` pattern everywhere.
4. **Parallel by default** — all unit tests call `t.Parallel()`.
5. **Golden files** for complex JSON responses — stored in `testdata/`.
6. **No flaky tests** — timeouts and retries are explicit, not implicit.
7. **Test naming**: `Test<Type>_<Method>_<Scenario>` (e.g., `TestAlertCondition_Match_StatusDown`).

### Coverage Targets

| Package | Target | Rationale |
|---------|--------|-----------|
| `auth/*` | **95%** | Security-critical |
| `engine/*` | **90%** | Alert logic correctness |
| `handlers/*` | **85%** | HTTP contract validation |
| `collectors/*` | **75%** | External I/O heavy |
| `logging/*` | **80%** | Observability correctness |
| `monitoring/*` | **80%** | Operational visibility |
| `database/*` | **80%** | Data integrity |
| `server/*` | **80%** | Middleware chain |
| `config/*` | **90%** | Startup correctness |
| All other | **80%** | General quality |

---

## 2. Tooling & Infrastructure

### Go Testing Tools

| Tool | Purpose | Install |
|------|---------|---------|
| `go test` (stdlib) | Test runner, coverage, benchmarks | Built-in |
| `go test -race` | Race condition detection | Built-in flag |
| `go test -fuzz` | Fuzz testing (Go 1.18+) | Built-in |
| [`testify`](https://github.com/stretchr/testify) | Assertions, require, suite, mock | `go get github.com/stretchr/testify` |
| [`mockgen`](https://github.com/uber-go/mock) | Interface mock generation | `go install go.uber.org/mock/mockgen@latest` |
| [`testcontainers-go`](https://github.com/testcontainers/testcontainers-go) | Dockerized Postgres for integration tests | `go get github.com/testcontainers/testcontainers-go` |
| [`httpexpect`](https://github.com/gavv/httpexpect) | HTTP API integration testing | `go get github.com/gavv/httpexpect/v2` |

### Static Analysis & Code Review Tools

| Tool | Purpose | Install |
|------|---------|---------|
| [`golangci-lint`](https://golangci-lint.run/) | Meta-linter (50+ linters) | `go install github.com/golangci-lint/golangci-lint/v2/cmd/golangci-lint@latest` |
| [`gosec`](https://github.com/securego/gosec) | Security vulnerability scanner | `go install github.com/securego/gosec/v2/cmd/gosec@latest` |
| [`govulncheck`](https://go.dev/doc/security/vuln) | Known vulnerability detection | `go install golang.org/x/vuln/cmd/govulncheck@latest` |
| [`goreportcard`](https://goreportcard.com/) | Overall code quality grade | Web tool + CLI |
| [`go-critic`](https://github.com/go-critic/go-critic) | Opinionated code style checks | Via golangci-lint |
| [`errcheck`](https://github.com/kisielk/errcheck) | Unchecked error returns | Via golangci-lint |
| [`staticcheck`](https://staticcheck.dev/) | Advanced static analysis | Via golangci-lint |
| [`deadcode`](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) | Dead code detection | `go install golang.org/x/tools/cmd/deadcode@latest` |

### Coverage & Reporting

| Tool | Purpose |
|------|---------|
| `go test -coverprofile=coverage.out` | Raw coverage data |
| `go tool cover -html=coverage.out` | Browser coverage report |
| [`gocover-cobertura`](https://github.com/boumenot/gocover-cobertura) | Cobertura XML for CI |
| [`sonarqube`](https://www.sonarqube.org/) (optional) | Continuous quality dashboard |

### Performance Testing

| Tool | Purpose |
|------|---------|
| `go test -bench` | Micro-benchmarks |
| [`k6`](https://k6.io/) | HTTP load testing |
| [`vegeta`](https://github.com/tsenart/vegeta) | HTTP load testing (CLI) |
| [`pprof`](https://pkg.go.dev/net/http/pprof) | CPU/memory profiling |

---

## 3. Unit Tests — Per Package

### 3.1 `internal/auth` — Authentication & Authorization

**File**: `internal/auth/jwt_test.go`

```
TestGenerateTokenPair_HappyPath
    → generates valid access + refresh tokens
    → access token expires at AccessTokenExpiry
    → refresh token expires at RefreshTokenExpiry
    → both tokens contain correct UserID, Username, Role
    → both tokens use HS256 signing method

TestGenerateTokenPair_EmptySecret
    → returns error with empty secret string

TestValidateToken_ValidToken
    → parses and returns correct Claims
    → Subject matches Username

TestValidateToken_ExpiredToken
    → returns error for expired token (use time override or short expiry)

TestValidateToken_WrongSecret
    → returns error when validated with different secret

TestValidateToken_MalformedToken
    → "not.a.jwt" → error
    → "" → error
    → "eyJhbGciOiJSUzI1NiJ9..." (RS256 token) → error (wrong signing method)

TestValidateToken_TamperedPayload
    → modify payload bytes, keep signature → error

TestValidateToken_NoneAlgorithm
    → token with alg:none → error (critical security test)

TestClaims_MissingFields
    → token with UserID=0 → still valid (struct zero values)
    → verify Role="" is returned correctly
```

**File**: `internal/auth/password_test.go`

```
TestHashPassword_Produces_ScryptFormat
    → output matches "scrypt:<hex>:<hex>"
    → salt is 64 hex chars (32 bytes)
    → hash is 64 hex chars (32 bytes)

TestHashPassword_Unique_Salts
    → hashing same password twice produces different outputs

TestCheckPassword_Scrypt_Correct
    → hash("password123") then check("password123") → true

TestCheckPassword_Scrypt_Wrong
    → hash("password123") then check("wrong") → false

TestCheckPassword_SHA256_Legacy_Correct
    → "sha256:<sha256hex of 'admin'>" matches "admin"

TestCheckPassword_SHA256_Legacy_Wrong
    → "sha256:<sha256hex of 'admin'>" does NOT match "wrong"

TestCheckPassword_InvalidFormat
    → "plaintext" → false
    → "" → false
    → "scrypt:too:few:parts" → false (4 parts)
    → "scrypt:notHex:notHex" → false

TestCheckPassword_TimingConstancy
    → (benchmark) verify constant-time via subtle.ConstantTimeCompare
    → wrong password takes ~same time as correct password ±5%
```

**File**: `internal/auth/session_test.go`

```
TestSessionStore_CreateAndGet
    → Create stores session; Get returns it

TestSessionStore_Get_Expired
    → Create with 1ms TTL, sleep 5ms, Get → nil, false
    → verify expired entry is deleted from internal map

TestSessionStore_Get_NonExistent
    → Get("unknown") → nil, false

TestSessionStore_Delete
    → Create, Delete, Get → nil, false

TestSessionStore_ConcurrentAccess
    → 100 goroutines Create/Get/Delete simultaneously → no panic, no data corruption

TestSessionStore_MemoryCleanup
    → Create 1000 sessions with 1ms TTL, sleep, Get each → all cleaned up
```

**File**: `internal/auth/apikey_test.go`

```
TestGenerateAPIKey_Format
    → key is non-empty base64url string
    → hash is 64-char hex string
    → hash == HashAPIKey(key)

TestGenerateAPIKey_Uniqueness
    → two calls produce different keys and hashes

TestHashAPIKey_Deterministic
    → same input → same hash (twice)

TestHashAPIKey_KnownVector
    → verify against a known SHA-256 test vector
```

**File**: `internal/auth/middleware_test.go`

```
TestRequireAuth_ValidJWT
    → request with "Bearer <valid_token>" → next handler called, claims in context

TestRequireAuth_ExpiredJWT
    → "Bearer <expired>" → 401 JSON response, next NOT called

TestRequireAuth_MissingHeader
    → no Authorization header, no X-Api-Key → 401

TestRequireAuth_InvalidPrefix
    → "Basic <token>" → 401

TestRequireAuth_ValidAPIKey
    → X-Api-Key header with valid key → next handler called via apiKeyLookup

TestRequireAuth_InvalidAPIKey
    → X-Api-Key with unknown key → falls through to JWT → 401

TestRequireAuth_APIKeyLookupError
    → apiKeyLookup returns error → falls through to JWT → 401

TestRequireRole_Admin_AccessingAdmin
    → claims.Role="admin" with RequireRole("admin") → 200

TestRequireRole_Viewer_AccessingAdmin
    → claims.Role="viewer" with RequireRole("admin") → 403

TestRequireRole_NoClaims
    → no claims in context → 401

TestRequireRole_HierarchyCheck
    → super_admin ≥ admin ≥ network_admin ≥ dept_admin ≥ viewer
    → each level can access its own and lower levels
    → each level is blocked from higher levels
```

---

### 3.2 `internal/engine` — Alert Engine & Analysis

**File**: `internal/engine/alert_conditions_test.go`

```
TestAlertCondition_Match_StatusEq
    → {Field:"status", Op:"eq", Value:"down"} + metric.Status="down" → true
    → {Field:"status", Op:"eq", Value:"down"} + metric.Status="up" → false

TestAlertCondition_Match_StatusNe
    → {Field:"status", Op:"ne", Value:"up"} + metric.Status="down" → true

TestAlertCondition_Match_ResponseTime_GT
    → threshold=1000.0, actual=1500.0 → true
    → threshold=1000.0, actual=500.0 → false

TestAlertCondition_Match_ResponseTime_LT
    → threshold=100.0, actual=50.0 → true

TestAlertCondition_Match_ResponseTime_GTE_Boundary
    → threshold=100.0, actual=100.0 → true (gte)
    → threshold=100.0, actual=99.99 → false (gte)

TestAlertCondition_Match_ResponseTime_LTE_Boundary
    → threshold=100.0, actual=100.0 → true (lte)

TestAlertCondition_Match_AllNumericFields
    → packet_loss, cpu_usage, memory_usage, bandwidth all tested

TestAlertCondition_Match_NilField
    → metric.ResponseTime=nil → false (regardless of operator)

TestAlertCondition_Match_UnknownField
    → Field="unknown_field" → false

TestAlertCondition_Match_InvalidThresholdType
    → Value="not_a_number" for numeric field → false

TestAlertCondition_Match_AlternateOperatorSyntax
    → Op=">" same as "gt", "<" same as "lt", etc.

TestToFloat64_TypeConversions
    → float64(42.5) → 42.5
    → float32(42.5) → 42.5
    → int(42) → 42.0
    → int64(42) → 42.0
    → string("42") → error
    → nil → error
```

**File**: `internal/engine/alert_rules_test.go`

```
TestAlertRule_Evaluate_Enabled
    → enabled=true, conditions match → true

TestAlertRule_Evaluate_Disabled
    → enabled=false → false (regardless of conditions)

TestAlertRule_Evaluate_DeviceFilter_Matching
    → rule.DeviceID=ptr(5), metric.DeviceID=5 → conditions evaluated

TestAlertRule_Evaluate_DeviceFilter_NonMatching
    → rule.DeviceID=ptr(5), metric.DeviceID=10 → false

TestAlertRule_Evaluate_DeviceFilter_Nil
    → rule.DeviceID=nil → applies to all devices

TestAlertRule_Evaluate_MultipleConditions_AllMatch
    → [status=="down", response_time>1000] both match → true

TestAlertRule_Evaluate_MultipleConditions_OneFailsShortCircuit
    → first condition fails → false, second not evaluated

TestAlertRule_Evaluate_EmptyConditions
    → no conditions → true (vacuous truth)

TestDefaultRules_Count
    → len(DefaultRules()) == 5

TestDefaultRules_Names
    → ["Device Down", "High Packet Loss", "Slow Response", "High CPU", "High Memory"]

TestDefaultRules_AllEnabled
    → every rule.Enabled == true
```

**File**: `internal/engine/alert_test.go`

```
TestAlertEngine_EvaluateMetric_DeviceDown
    → metric.Status="down" + no existing active alert → creates alert

TestAlertEngine_EvaluateMetric_DeviceDown_AlreadyAlerted
    → metric.Status="down" + existing active alert → no duplicate created

TestAlertEngine_EvaluateMetric_DeviceUp
    → metric.Status="up" → no alert created

TestAlertEngine_EvaluateMetric_DBError_GetDevice
    → GetDevice returns error → EvaluateMetric returns error

TestAlertEngine_EvaluateMetric_DBError_CreateAlert
    → CreateAlert returns error → EvaluateMetric returns error
```

**File**: `internal/engine/notifier_test.go`

```
TestNotifier_Send_WebhookSuccess
    → mock HTTP server receives correct JSON payload
    → verify Content-Type is application/json
    → verify all fields (alert_id, device_name, severity, message, timestamp)

TestNotifier_Send_WebhookServerError
    → mock returns 500 → logs warning, doesn't panic

TestNotifier_Send_WebhookTimeout
    → mock server delays 15s with 10s client timeout → error logged

TestNotifier_Send_WebhookNoURL
    → channel.Config["url"]="" → error returned

TestNotifier_Send_DisabledChannel
    → channel.Enabled=false → not called

TestNotifier_Send_UnsupportedChannelType
    → channel.Type="sms" → logs warning

TestNotifier_Send_MultipleChannels
    → 3 channels (2 enabled, 1 disabled) → 2 webhook calls made

TestNotifier_Send_ContextCancellation
    → cancelled context → webhook request fails gracefully
```

**File**: `internal/engine/flow_analyzer_test.go`

```
TestFlowAnalyzer_Ingest_BatchFlush
    → send 500+ flows → auto-flushed to DB

TestFlowAnalyzer_Ingest_TickerFlush
    → send 10 flows, wait 30s ticker → flushed to DB

TestFlowAnalyzer_Stop_FlushesPending
    → send 50 flows, call Stop() → all 50 persisted to DB

TestFlowAnalyzer_DBError
    → RecordFlows returns error → logged, pending cleared, no crash

TestFlowAnalyzer_ChannelFull
    → fill IngestChannel to capacity → new sends don't block
```

**File**: `internal/engine/anomaly_test.go`

```
TestAnomalyEngine_StartStop
    → Start + Stop → no goroutine leak (verify with runtime.NumGoroutine)

TestAnomalyEngine_Stop_BeforeStart
    → Stop without Start → no panic
```

---

### 3.3 `internal/collectors` — Protocol Collectors

**File**: `internal/collectors/collector_test.go`

```
TestRegistry_Register_And_Get
    → Register("ping", PingCollector{}) → Get("ping") returns it

TestRegistry_Get_UnknownProtocol
    → Get("unknown") → nil, false

TestRegistry_Register_Overwrite
    → Register same name twice → latest wins
```

**File**: `internal/collectors/http_test.go`

```
TestHTTPCollector_Name
    → == "http"

TestHTTPCollector_Collect_Success
    → mock HTTP server returns 200 → Result{Status:"up", ResponseTime:>0}

TestHTTPCollector_Collect_ServerDown
    → unreachable host → Result{Status:"down"}

TestHTTPCollector_Collect_CustomPath
    → device.HTTPPath="/health" → request goes to http://host/health

TestHTTPCollector_Collect_ExpectedStatusMismatch
    → device.HTTPExpectedStatus=200, server returns 503 → Status:"down"

TestHTTPCollector_Collect_ExpectedStatusMatch
    → device.HTTPExpectedStatus=200, server returns 200 → Status:"up"

TestHTTPCollector_Collect_Timeout
    → mock server delays 15s → Status:"down" (10s client timeout)

TestHTTPCollector_Collect_ContextCancelled
    → cancelled context → returns quickly with Status:"down"
```

**File**: `internal/collectors/ping_test.go`

> Note: Ping requires privileged access or mocking. Test with interface mock.

```
TestPingCollector_Name
    → == "ping"

TestPingCollector_Collect_InvalidHost
    → device.IPAddress="this.host.does.not.exist.invalid" → Status:"down"

(Integration test in CI with --privileged):
TestPingCollector_Collect_Localhost
    → device.IPAddress="127.0.0.1" → Status:"up", PacketLoss:0
```

**File**: `internal/collectors/port_test.go`

```
TestPortCollector_Name
    → == "port"

TestPortCollector_Collect_OpenPort
    → start local TCP listener → Status:"up", ResponseTime>0

TestPortCollector_Collect_ClosedPort
    → unused high port → Status:"down"

TestPortCollector_Collect_DefaultPort
    → device.SNMPPort=0 → uses port 80
```

**File**: `internal/collectors/snmp_test.go`

> Note: Requires mock SNMP agent or integration test.

```
TestSNMPCollector_Name
    → == "snmp"

TestSNMPCollector_Collect_ConnectionRefused
    → unreachable host → Status:"down"

TestSNMPCollector_Collect_DefaultCommunity
    → device.SNMPCommunity="" → uses "public"

TestSNMPCollector_Collect_CustomPort
    → device.SNMPPort=1161 → connects to port 1161

TestSNMPCollector_Collect_Version1
    → device.SNMPVersion="1" → uses SNMPv1
```

**File**: `internal/collectors/netflow_test.go`

```
TestParseNetFlowV5_ValidPacket
    → construct valid v5 header + 3 records → returns 3 flows
    → verify SrcIP, DstIP, SrcPort, DstPort, Protocol, Bytes, Packets

TestParseNetFlowV5_TooShort
    → data < 24 bytes → nil

TestParseNetFlowV5_WrongVersion
    → version=9 → nil

TestParseNetFlowV5_TruncatedRecord
    → header says 5 records but only 2 fit → returns 2

TestParseNetFlowV5_ZeroCount
    → count=0 → empty slice

TestProtoName
    → 6 → "TCP", 17 → "UDP", 1 → "ICMP", 99 → "99"

TestNetFlowCollector_Listen_Integration
    → start listener, send UDP packet, verify flows arrive on channel

TestNetFlowCollector_Listen_ContextCancel
    → cancel context → listener stops, no goroutine leak
```

**File**: `internal/collectors/system_test.go`

```
TestSystemCollector_Name
    → == "system"

TestSystemCollector_Collect
    → always returns Status:"up" with Details["note"]
```

---

### 3.4 `internal/handlers` — HTTP Handlers

> **Pattern**: Use `httptest.NewServer` or `httptest.NewRecorder` with a mock DB.
> Generate mock: `mockgen -source=internal/database/database.go -destination=internal/database/mock_db.go`

**File**: `internal/handlers/auth_test.go`

```
TestLogin_HappyPath
    → POST /api/auth/login {username, password} → 200 {accessToken, refreshToken, user}

TestLogin_WrongPassword
    → invalid password → 401 "invalid credentials"

TestLogin_UnknownUser
    → DB returns error → 401 "invalid credentials"

TestLogin_DisabledUser
    → user.Enabled=false → 403 "account disabled"

TestLogin_EmptyBody
    → empty/malformed JSON → 400

TestLogin_MissingUsername
    → {password:"x"} → 400 or 401

TestLogout
    → POST /api/auth/logout → 200 {message:"logged out"}

TestMe_Authenticated
    → valid token in context → 200 {user data}

TestMe_Unauthenticated
    → no claims in context → 401

TestRefresh_ValidRefreshToken
    → POST {refreshToken} → 200 {accessToken, refreshToken}

TestRefresh_InvalidRefreshToken
    → POST {refreshToken:"invalid"} → 401

TestRefresh_EmptyBody
    → POST {} → 400

TestListAPIKeys
    → GET → 200 [array of keys]

TestCreateAPIKey
    → POST {description} → 201 {id, key, description, createdAt}

TestDeleteAPIKey_Valid
    → DELETE /apikeys/1 → 200

TestDeleteAPIKey_InvalidID
    → DELETE /apikeys/abc → 400

TestParseID
    → "42" → 42, nil
    → "abc" → 0, error
    → "-1" → -1, nil
    → "" → 0, error
    → "9999999999999999999" → overflow behavior
```

**File**: `internal/handlers/devices_test.go`

```
TestDeviceList
    → GET /api/devices → 200 [devices]
    → DB error → 500

TestDeviceGet_Valid
    → GET /api/devices/1 → 200 {device}

TestDeviceGet_NotFound
    → GET /api/devices/999 → 404

TestDeviceGet_InvalidID
    → GET /api/devices/abc → 400

TestDeviceCreate_HappyPath
    → POST {name, ipAddress} → 201 {device}
    → default protocol="ping", interval=60, enabled=true, status="unknown"

TestDeviceCreate_MissingName
    → POST {ipAddress:"1.2.3.4"} → 400 "name and ipAddress are required"

TestDeviceCreate_MissingIP
    → POST {name:"Server"} → 400

TestDeviceCreate_InvalidJSON
    → POST "not json" → 400

TestDeviceCreate_CustomProtocol
    → POST {name, ipAddress, protocol:"http"} → protocol preserved

TestDeviceUpdate
    → PUT /api/devices/1 {name:"new"} → 200

TestDeviceUpdate_InvalidID
    → PUT /api/devices/abc → 400

TestDeviceDelete
    → DELETE /api/devices/1 → 200 {message:"deleted"}

TestDeviceDelete_DBError
    → DB returns error → 500
```

**File**: `internal/handlers/alerts_test.go`

```
TestAlertList
    → GET /api/alerts → 200 {alerts, total}
    → GET /api/alerts?status=active&limit=10&offset=0 → filtered

TestAlertGet_Valid
    → GET /api/alerts/1 → 200

TestAlertGet_NotFound
    → GET /api/alerts/999 → 404

TestAlertCreate
    → POST {deviceId, severity, message} → 201, status forced to "active"

TestAlertAcknowledge
    → POST /api/alerts/1/acknowledge → 200, by=claims.Username

TestAlertAcknowledge_NoClaims
    → POST without auth claims → by=""

TestAlertResolve
    → POST /api/alerts/1/resolve → 200

TestAlertDelete
    → DELETE /api/alerts/1 → 200
```

**File**: `internal/handlers/metrics_test.go`

```
TestMetricLatest
    → GET /api/metrics/latest → 200 [metrics]

TestMetricForDevice
    → GET /api/metrics/5 → 200 [metrics for device 5]

TestMetricForDevice_InvalidID
    → GET /api/metrics/abc → 400

TestMetricQuery_WithDeviceID
    → GET /api/v1/metrics/query?deviceId=5 → device metrics

TestMetricQuery_Summary
    → GET /api/v1/metrics/query → summary (no deviceId)

TestParseTimeRange
    → no params → from=now-24h, to=now, limit=0
    → from=2024-01-01T00:00:00Z → parsed correctly
    → invalid from → default used
    → limit=50 → parsed correctly
```

**File**: `internal/handlers/flows_test.go`

```
TestFlowList
    → GET /api/v1/flows → 200 {flows, total}
    → with from, to, limit, offset params

TestFlowTopTalkers
    → GET /api/v1/flows/top-talkers?n=10 → 200 [talkers]

TestFlowProtocols
    → GET /api/v1/flows/protocols → 200 {protocol stats}
```

**File**: `internal/handlers/dashboards_test.go`

```
TestDashboardList
    → GET → 200 [dashboards for user]

TestDashboardGet_Valid
    → GET /1 → 200

TestDashboardGet_NotFound
    → GET /999 → 404

TestDashboardSave_New
    → POST → 200 {saved dashboard}
    → UserID set from claims

TestDashboardSave_Update
    → PUT /1 → 200

TestDashboardDelete
    → DELETE /1 → 200
```

**File**: `internal/handlers/reports_test.go`

```
TestReportSummary
    → GET → 200 {merged summary + stats}

TestReportTimeseries
    → GET → 200 [metrics]

TestReportDevices
    → GET → 200 {stats}

TestReportExport_CSV
    → GET → Content-Type: text/csv
    → Content-Disposition: attachment; filename=metrics.csv
    → verify CSV header row
    → verify data rows with nil ResponseTime → empty string
```

**File**: `internal/handlers/capture_test.go`

```
TestCaptureStart
    → POST → 200 {status:"started"}

TestCaptureStart_AlreadyRunning
    → POST twice → 409 "capture already running"

TestCaptureStop
    → POST → 200 {status:"stopped"}

TestCaptureStats
    → GET → 200 {running, totalPackets, totalBytes, protocols}

TestCaptureStats_AfterStartStop
    → Start, Stats → running:true
    → Stop, Stats → running:false
```

**File**: `internal/handlers/insights_test.go`

```
TestInsightCurrent
    → returns device scores based on status and response time
    → device down → score 0
    → device up, RT>1000 → score 50
    → device up, RT<1000 → score 100

TestInsightHistory
    → returns metrics history
```

**File**: `internal/handlers/simulator_test.go`

```
TestSimulatorMetrics
    → POST {metric} → 200, recorded to DB
    → zero timestamp → defaults to now

TestSimulatorFlows
    → POST [{flow}, ...] → 200 {recorded: N}
    → zero timestamps → filled

TestSimulatorAlert
    → POST {alert} → 201, status forced to "active"
```

---

### 3.5 `internal/config` — Configuration

**File**: `internal/config/config_test.go`

```
TestLoad_RequiresJWTSecret
    → JWT_SECRET unset → error

TestLoad_DefaultValues
    → set only JWT_SECRET → verify all defaults:
      Port=3000, NodeEnv="development", Version="1.1.0"
      MaxConns=20, MinConns=2, MaxConnLifetime=1h, HealthCheckPeriod=30s
      LogLevel="info", LogFormat="pretty", FileEnabled=false
      FilePath="./data/logs/netmonitor.log", FileCompress=true
      DBEnabled=true, DBSampleRate=1.0, DBQueueSize=10000, DBDropPolicy="drop_debug"
      SlowQueryMs=100, SlowRequestMs=1000
      CollectorIntervalSec=60, MetricsRetentionDays=30
      AccessTokenExpiry=15m, RefreshTokenExpiry=7d

TestLoad_OverrideValues
    → set PORT=8080, NODE_ENV=production, LOG_LEVEL=debug → values override defaults

TestLoad_ModuleLevels
    → LOG_MODULE_LEVELS="http=debug,db=warn" → parsed to map

TestLoad_InvalidInt
    → PORT=abc → uses default 3000

TestLoad_InvalidBool
    → LOG_FILE_ENABLED=maybe → uses default false

TestLoad_InvalidDuration
    → ACCESS_TOKEN_EXPIRY=xyz → uses default 15m

TestLoad_InvalidFloat
    → LOG_DB_SAMPLE_RATE=abc → uses default 1.0

TestEnvStr_EmptyReturnsDefault
    → envStr("NONEXISTENT", "fallback") → "fallback"

TestEnvInt_EmptyReturnsDefault
    → envInt("NONEXISTENT", 42) → 42

TestEnvBool_EmptyReturnsDefault
    → envBool("NONEXISTENT", true) → true

TestEnvDuration_EmptyReturnsDefault
    → envDuration("NONEXISTENT", 5*time.Second) → 5s

TestEnvFloat64_EmptyReturnsDefault
    → envFloat64("NONEXISTENT", 0.5) → 0.5
```

---

### 3.6 `internal/logging` — Structured Logging System

**File**: `internal/logging/logger_test.go`

```
TestNew_JSONFormat
    → cfg.Logging.Format="json" → handler is JSONHandler

TestNew_TextFormat
    → cfg.Logging.Format="pretty" → handler is TextHandler

TestNew_CapturesHostnameAndPID
    → logger.Hostname() non-empty, logger.PID() > 0

TestLogger_With_CreatesChildLogger
    → logger.With("http") → child.component == "http"
    → child preserves hostname, PID, version

TestLogger_LogLevels
    → Trace, Debug, Info, Warn, Error each produce output at appropriate level

TestLogger_Fatal_ExitsProcess
    → (test with exec.Command + os.Exit check)

TestLogger_ContextEnrichment
    → ctx with request_id, trace_id, user_id → all appear in log output

TestParseLevel
    → "trace" → LevelTrace(-8)
    → "debug" → slog.LevelDebug
    → "info" → slog.LevelInfo
    → "warn" → slog.LevelWarn
    → "error" → slog.LevelError
    → "unknown" → slog.LevelInfo (default)
```

**File**: `internal/logging/context_test.go`

```
TestWithRequestID_GetRequestID
    → roundtrip: set and get

TestWithUserID_GetUserID
    → roundtrip

TestWithTraceID_GetTraceID
    → roundtrip

TestGet_EmptyContext
    → GetRequestID(context.Background()) → ""
    → GetUserID(context.Background()) → ""
    → GetTraceID(context.Background()) → ""

TestGenerateRequestID
    → length == 16, is hex

TestGenerateTraceID
    → with prefix "collect" → starts with "collect-"
    → without prefix → starts with "tr-"
    → always contains random hex suffix
```

**File**: `internal/logging/http_logger_test.go`

```
TestRequestLogger_BasicRequest
    → GET /health → logs request_start + request_end, status=200

TestRequestLogger_SlowRequest
    → handler sleeps 2s, threshold=1000ms → WARN level with slow_request=true

TestRequestLogger_4xxWarning
    → handler returns 404 → logged at WARN level

TestRequestLogger_5xxError
    → handler returns 500 → logged at ERROR level

TestRequestLogger_RequestID
    → X-Request-ID header set on response

TestRequestLogger_PasswordRedaction
    → POST /auth/login {password:"secret"} → body preview shows "***"

TestRedactPasswords
    → `{"password":"hunter2"}` → `{"password":"***"}`
    → `{"username":"admin"}` → unchanged (no password field)

TestResponseWriter_Flush
    → underlying writer supports Flusher → no panic
    → underlying writer doesn't support Flusher → no panic
```

**File**: `internal/logging/db_logger_test.go`

```
TestDBLogger_LogQuery_Success
    → logs at DEBUG level with sql, params, duration, rows

TestDBLogger_LogQuery_Error
    → logs at ERROR level with event="query_error"

TestDBLogger_LogQuery_Slow
    → duration > slowQueryMs → WARN level, event="slow_query"

TestDBLogger_LogTransaction
    → action="BEGIN" → logged at DEBUG
    → action="COMMIT" with error → logged at ERROR
```

**File**: `internal/logging/sink_test.go`

```
TestMultiSink_Write_AllWriters
    → 3 writers → all receive same data

TestMultiSink_Write_OneErrors
    → writer 2 of 3 errors → returns error

TestDBSink_Enqueue
    → Write enqueues data, consumer calls writer func

TestDBSink_QueueFull_DropDebug
    → fill queue, Write → silently dropped

TestDBSink_QueueFull_DropOldest
    → policy="drop_oldest", fill queue → oldest dequeued

TestDBSink_RecursionSafety
    → writer func calls Write back → doesn't re-enqueue (prevents infinite loop)

TestDBSink_Stop_Drains
    → enqueue 100 items, Stop() → all 100 processed

TestDBSink_ConcurrentWrites
    → 100 goroutines writing simultaneously → no panic, no data loss
```

**File**: `internal/logging/audit_logger_test.go`

```
TestAuditLogger_LogEvent
    → logs with correct event type, severity, actor, details

TestAuditLogger_LogLogin_Success
    → success=true → event="auth.login_success", severity="info"

TestAuditLogger_LogLogin_Failure
    → success=false → event="auth.login_failure", severity="warn"

TestAuditLogger_LogConfigChange_Delete
    → action="deleted" → severity="warn"

TestAuditLogger_LogConfigChange_Create
    → action="created" → severity="info"
```

**File**: `internal/logging/alert_logger_test.go`

```
TestAlertLogger_LogEvaluation
    → logs condition results array

TestAlertLogger_LogTriggered
    → logs at WARN level with condition_values

TestAlertLogger_LogNotificationSent
    → logs at INFO with channel details

TestAlertLogger_LogNotificationFailed
    → logs at ERROR with error and retry info
```

---

### 3.7 `internal/server` — HTTP Server & Middleware

**File**: `internal/server/middleware_test.go`

```
TestSecurityHeaders
    → response has X-Frame-Options, X-Content-Type-Options, HSTS, CSP, etc.

TestRateLimiter_AllowsBurst
    → burst=5 → first 5 requests succeed

TestRateLimiter_BlocksExcess
    → burst=2, send 10 requests → ≥8 get 429

TestRateLimiter_DifferentIPs
    → IP-A and IP-B have independent limits

TestRateLimiter_CleanupStaleEntries
    → (integration) verify stale entries cleaned after 3min

TestRecovery_PanicHandler
    → handler panics → 500 response, no crash

TestRecovery_NoPanic
    → handler doesn't panic → normal response

TestRequestSize_UnderLimit
    → 100 byte body with 1MB limit → passes through

TestRequestSize_OverLimit
    → 2MB body with 1MB limit → request body truncated
```

---

### 3.8 `internal/httputil` — Response Helpers

**File**: `internal/httputil/response_test.go`

```
TestSendOK
    → status 200, body has {"success":true, "data": ...}

TestSendError
    → status 400, body has {"success":false, "error": "msg"}

TestSendCreated
    → status 201, body has {"success":true, "data": ...}

TestParseJSON_ValidBody
    → valid JSON → parsed correctly

TestParseJSON_InvalidJSON
    → "not json" → error

TestParseJSON_EmptyBody
    → ContentLength=0 → returns nil (no error)
```

---

### 3.9 `internal/scanner` — Port Scanner

**File**: `internal/scanner/port_scanner_test.go`

```
TestScanPorts_OpenPort
    → start local listener on port X → PortResult{Open:true}

TestScanPorts_ClosedPort
    → high unused port → PortResult{Open:false}

TestScanPorts_MixedPorts
    → 1 open + 1 closed → correct results in order

TestScanPorts_DefaultOptions
    → Concurrency=0 → uses DefaultOptions.Concurrency (100)
    → Timeout=0 → uses DefaultOptions.Timeout (2s)

TestScanPorts_ContextCancelled
    → cancel context during scan → returns quickly with all Open=false

TestScanPorts_EmptyPortList
    → ports=[] → returns []

TestScanPorts_ConcurrencyLimit
    → scan 200 ports with Concurrency=10 → at most 10 concurrent connections

TestCommonPorts
    → len(CommonPorts) == 22
    → contains 22, 80, 443, 3306, 5432
```

---

### 3.10 `internal/scheduler` — Collection Scheduler

**File**: `internal/scheduler/scheduler_test.go`

```
TestScheduler_StartStop
    → Start + Stop → no goroutine leak

TestScheduler_RunOnce_SkipsDisabledDevices
    → 3 devices, 1 disabled → only 2 collected

TestScheduler_RunOnce_UnknownProtocol
    → device.Protocol="unknown" → skipped, no error

TestScheduler_CollectOne_RecordsMetric
    → collector returns Result → db.RecordMetric called

TestScheduler_CollectOne_CollectorError
    → collector returns error → no metric recorded, no crash

TestScheduler_CollectOne_BroadcastsToHub
    → successful collection → hub.Broadcast called with EventMetricUpdate

TestScheduler_CollectOne_UpdatesDeviceStatus
    → if db implements UpdateDeviceStatus → called
```

---

### 3.11 `internal/retention` — Data Retention

**File**: `internal/retention/retention_test.go`

```
TestRetention_Prune_CallsAllThree
    → prune() → PruneMetrics, PruneFlows, PruneAlerts all called
    → thresholds match now - retentionDays

TestRetention_StartStop
    → Start + Stop → no goroutine leak

TestRetention_Stop_BeforeStart
    → no panic
```

---

### 3.12 `internal/websocket` — WebSocket Hub

**File**: `internal/websocket/hub_test.go`

```
TestHub_Run_Broadcast
    → register client, broadcast message → client receives it

TestHub_Broadcast_NoClients
    → broadcast with no clients → no panic

TestHub_ConnectionCount
    → 0 initially, add 2 clients → 2, remove 1 → 1

TestHub_Stop
    → Stop() → Run goroutine exits

TestHub_ServeWS_NoToken
    → missing ?token= → 401

TestHub_ServeWS_InvalidToken
    → invalid JWT → 401

TestHub_ServeWS_ValidToken
    → valid JWT → WebSocket upgrade succeeds, client registered
```

---

### 3.13 `internal/monitoring` — Self-Monitoring

**File**: `internal/monitoring/self_monitor_test.go`

```
TestSelfMonitor_Collect_RuntimeMetrics
    → GoroutineCount > 0, HeapAllocBytes > 0, NumCPU > 0

TestSelfMonitor_Collect_OptionalProviders
    → WSConnectionCount set → ActiveWSConnections populated
    → WSConnectionCount nil → ActiveWSConnections=0

TestSelfMonitor_StartStop
    → Start + Stop → no goroutine leak

TestSelfMonitor_DefaultInterval
    → interval=0 → uses 60s
```

**File**: `internal/monitoring/recorder_test.go`

```
TestRecorder_RecordHTTP
    → delegates to db.RecordHTTPRequest

TestRecorder_RecordDB
    → delegates to db.RecordDBQuery

TestRecorder_RecordAudit
    → delegates to db.RecordAuditEvent

TestRecorder_RecordAlert
    → delegates to db.RecordAlertActivity
```

**File**: `internal/monitoring/handlers_test.go`

```
TestSystemLogs_AllComponents
    → GET /system/logs → returns all categories

TestSystemLogs_HTTPComponent
    → GET /system/logs?component=http → only HTTP requests

TestSystemMonitoring
    → GET /system/monitoring → latest health snapshot

TestSystemMonitoringHistory
    → GET /system/monitoring/history?hours=24 → historical data

TestSystemMonitoringRequests_Filter
    → ?path=/api/devices&status_code=500 → filtered results

TestSystemMonitoringQueries_SlowOnly
    → ?slow_only=true → only slow queries

TestSystemAuditLog_FilterByEvent
    → ?event_type=auth.login_failure → filtered

TestSystemCollectorsStats
    → aggregates per-device success/failure rates
```

---

### 3.14 `internal/models` — Data Models

**File**: `internal/models/models_test.go`

```
TestDevice_JSONSerialization
    → marshal → unmarshal roundtrip preserves all fields
    → JSON keys use camelCase (json tags)

TestDevice_Tags_NilSlice
    → Tags=nil → serializes as null, not error

TestMetric_NilOptionalFields
    → ResponseTime=nil → omitted in JSON

TestAlertRule_NilDeviceID
    → DeviceID=nil → applies to all devices, serialized as null

TestSensor_ConfigMap
    → Config=map[string]any{"threshold":5} → serializes correctly
```

---

## 4. Integration Tests

### 4.1 Database Integration Tests

> Use `testcontainers-go` to spin up PostgreSQL + TimescaleDB.

**File**: `internal/database/postgres_integration_test.go`

```go
// +build integration

func TestPostgres_Connect_And_Migrate(t *testing.T)
    → container starts, Connect succeeds, RunMigrations creates all tables

func TestPostgres_DeviceCRUD(t *testing.T)
    → Create → Get → Update → Delete roundtrip

func TestPostgres_CreateDevice_Validation(t *testing.T)
    → duplicate name → error
    → empty IP → error

func TestPostgres_MetricRecordAndQuery(t *testing.T)
    → RecordMetric → GetDeviceMetrics → data returned

func TestPostgres_AlertCRUD(t *testing.T)
    → CreateAlert → GetAlert → UpdateAlertStatus → DeleteAlert

func TestPostgres_FlowRecordAndQuery(t *testing.T)
    → RecordFlows → GetFlows → GetTopTalkers → GetProtocolStats

func TestPostgres_DashboardCRUD(t *testing.T)
    → SaveDashboard (create) → GetDashboard → SaveDashboard (update) → DeleteDashboard

func TestPostgres_UserOperations(t *testing.T)
    → GetUserByUsername → GetUserByID
    → Seed creates admin user

func TestPostgres_APIKeyCRUD(t *testing.T)
    → CreateAPIKey → GetAPIKey by hash → GetAPIKeysByUser → DeleteAPIKey

func TestPostgres_Retention(t *testing.T)
    → Insert old metrics/flows/alerts → Prune → verify deleted

func TestPostgres_Ping(t *testing.T)
    → Ping → nil

func TestPostgres_ConcurrentWrites(t *testing.T)
    → 50 goroutines recording metrics simultaneously → no errors, all recorded
```

### 4.2 HTTP API Integration Tests

> Full server with real DB via testcontainers.

**File**: `internal/server/api_integration_test.go`

```go
// +build integration

func TestAPI_FullAuthFlow(t *testing.T)
    → Login → use accessToken → Me → Refresh → use new token → Logout

func TestAPI_DeviceLifecycle(t *testing.T)
    → Create device → List (has it) → Update → Get (updated) → Delete → Get (404)

func TestAPI_AlertWorkflow(t *testing.T)
    → Create alert → Acknowledge → Resolve → verify status transitions

func TestAPI_MetricsAfterCollection(t *testing.T)
    → Create device → Simulate metric → Query → data returned

func TestAPI_DashboardPerUser(t *testing.T)
    → User A creates dashboard → User B can't see it

func TestAPI_CSVExport(t *testing.T)
    → Export → parse CSV → verify headers and data

func TestAPI_UnauthorizedAccess(t *testing.T)
    → GET /api/devices without token → 401
    → GET /api/devices with expired token → 401

func TestAPI_CORS(t *testing.T)
    → OPTIONS preflight → Access-Control-Allow-Credentials: true
    → verify no Access-Control-Allow-Origin: * (must be specific origin)

func TestAPI_RateLimiting(t *testing.T)
    → send burst+1 requests in production mode → last one gets 429
```

---

## 5. System / End-to-End Tests

### 5.1 Docker Compose E2E

**File**: `e2e/e2e_test.go`

```
Prerequisite: docker-compose.e2e.yml with netmonitor + postgres + timescaledb

TestE2E_ServerStartsAndServesHealth
    → GET /health → {status:"ok", database:"ok"}

TestE2E_FullMonitoringWorkflow
    1. Login as admin
    2. Create 3 devices (ping, http, port)
    3. Wait for 2 scheduler cycles
    4. Verify metrics exist for each device
    5. Verify alerts if any device is down
    6. Check WebSocket receives metric:update events
    7. Export CSV report
    8. Delete devices
    9. Logout

TestE2E_GracefulShutdown
    → send SIGTERM → server shuts down within 30s
    → no in-flight requests dropped

TestE2E_DatabaseConnectionLoss
    → stop postgres container → health endpoint shows database error
    → restart postgres → health recovers
```

---

## 6. Security Testing

### 6.1 Authentication & Authorization

```
TestSecurity_JWTNoneAlgorithm
    → token with "alg":"none" → rejected

TestSecurity_JWTAlgorithmConfusion
    → RS256 token against HS256 secret → rejected

TestSecurity_BruteForceProtection
    → 100 rapid login failures → rate limited (429)

TestSecurity_SQLInjection
    → device name="'; DROP TABLE devices;--" → properly escaped

TestSecurity_XSSInDeviceName
    → device name="<script>alert(1)</script>" → stored as-is (JSON API, no HTML rendering)

TestSecurity_PathTraversal
    → GET /api/devices/../../etc/passwd → 400 invalid id

TestSecurity_OversizedPayload
    → POST 5MB body → rejected by RequestSize middleware

TestSecurity_PasswordNotInResponse
    → Login response does NOT contain password_hash

TestSecurity_APIKeyNotReversible
    → given hash, cannot recover original key

TestSecurity_RefreshTokenNotUsableAsAccess
    → (current gap) use refresh token in Authorization header → should be rejected
    → NOTE: currently tokens are identical — this test documents the vulnerability
```

### 6.2 Gosec Security Scan

```bash
gosec -fmt=json -out=gosec-report.json ./...
```

Verify no HIGH or CRITICAL findings for:
- Hardcoded credentials
- SQL injection
- Path traversal
- Weak crypto
- Unvalidated redirects
- Race conditions on shared data

### 6.3 Dependency Vulnerability Scan

```bash
govulncheck ./...
```

---

## 7. Performance & Load Testing

### 7.1 Go Benchmarks

**File**: `internal/auth/password_bench_test.go`

```go
BenchmarkHashPassword(b *testing.B)
    → measure scrypt hashing throughput

BenchmarkCheckPassword_Scrypt(b *testing.B)
    → measure verification throughput

BenchmarkCheckPassword_SHA256(b *testing.B)
    → measure legacy verification throughput
```

**File**: `internal/engine/alert_conditions_bench_test.go`

```go
BenchmarkAlertCondition_Match(b *testing.B)
    → measure condition evaluation throughput

BenchmarkAlertRule_Evaluate_5Conditions(b *testing.B)
    → measure full rule evaluation
```

**File**: `internal/collectors/netflow_bench_test.go`

```go
BenchmarkParseNetFlowV5_10Records(b *testing.B)
    → measure parsing throughput for 10-record packets

BenchmarkParseNetFlowV5_30Records(b *testing.B)
    → maximum records per v5 packet
```

**File**: `internal/scanner/port_scanner_bench_test.go`

```go
BenchmarkScanPorts_100Ports(b *testing.B)
    → measure scan throughput against localhost
```

### 7.2 k6 Load Tests

**File**: `loadtest/api.js`

```javascript
// Scenarios:
// 1. Steady state: 50 VUs, 5 minutes
//    → GET /health, GET /api/devices, GET /api/metrics/latest
//    → Target: p95 < 50ms, error rate < 0.1%
//
// 2. Spike test: ramp to 200 VUs over 30s
//    → Mix of reads (80%) and writes (20%)
//    → Target: p95 < 200ms, error rate < 1%
//
// 3. Soak test: 20 VUs, 30 minutes
//    → Memory should not grow unboundedly
//    → Target: no OOM, no goroutine leak
//
// 4. Stress test: ramp to 500 VUs
//    → Find breaking point
//    → Server should gracefully degrade (429s), not crash
```

### 7.3 Memory & CPU Profiling

```bash
# CPU profile during load test
go tool pprof http://localhost:3000/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:3000/debug/pprof/heap

# Goroutine profile (check for leaks)
go tool pprof http://localhost:3000/debug/pprof/goroutine
```

---

## 8. Concurrency & Race Condition Testing

### Run all tests with race detector:

```bash
go test -race -count=3 ./...
```

### Specific concurrency tests:

```
TestSessionStore_ConcurrentCreateGetDelete
    → 100 goroutines operating on shared SessionStore

TestRateLimiter_ConcurrentRequests
    → 50 goroutines hitting rate limiter with same IP

TestHub_ConcurrentBroadcast
    → 20 goroutines broadcasting while clients connect/disconnect

TestDBSink_ConcurrentWrites
    → 100 goroutines writing to DBSink simultaneously

TestFlowAnalyzer_ConcurrentIngest
    → 10 goroutines sending flow batches concurrently

TestCaptureHandler_ConcurrentStartStop
    → multiple goroutines calling Start/Stop simultaneously

TestScheduler_ConcurrentCollect
    → verify multiple collectOne goroutines don't corrupt shared state
```

---

## 9. Code Review Plan

### 9.1 Review Checklist — Per File

For every source file, verify:

- [ ] **Error handling**: Every error returned by a function is checked. No `_ = fn()` for non-trivial functions.
- [ ] **Context propagation**: All functions that do I/O accept `context.Context` as first parameter.
- [ ] **Input validation**: All user inputs are validated before use (IDs, query params, JSON bodies).
- [ ] **Resource cleanup**: All `io.Closer`, `*sql.Rows`, `net.Conn`, `http.Response.Body` are closed.
- [ ] **Nil pointer safety**: All pointer dereferences are guarded by nil checks.
- [ ] **Goroutine lifecycle**: Every goroutine has a clear termination path (via context or channel).
- [ ] **Logging**: Errors are logged with sufficient context (request_id, operation, params).
- [ ] **No hardcoded secrets**: No passwords, tokens, or keys in source code.
- [ ] **Documentation**: All exported types and functions have doc comments.

### 9.2 Architecture Review

| Area | Review Focus |
|------|--------------|
| **Dependency Direction** | Handlers → Database → Models (no reverse imports) |
| **Interface Segregation** | `database.Database` has 30+ methods — consider splitting into domain-specific interfaces |
| **Error Types** | Currently all errors are raw `error` — consider custom error types for 404/409/500 differentiation |
| **Configuration** | Config is loaded once at startup — verify no env reads elsewhere |
| **Graceful Shutdown** | All background goroutines (scheduler, anomaly, retention, flow analyzer, self-monitor, hub) are stopped |

### 9.3 Package-Specific Code Review

#### `auth/` — Security Review

- [ ] JWT tokens don't distinguish access vs refresh (Critical: refresh token usable as access token)
- [ ] Password comparison uses `subtle.ConstantTimeCompare` ✓ (Fixed)
- [ ] Session cleanup happens on `Get()` ✓ (Fixed) — but no periodic sweep
- [ ] Role hierarchy is correct (viewer < dept_admin < network_admin < admin = super_admin)
- [ ] API key hashing uses SHA-256 (acceptable for API keys, not passwords)

#### `handlers/` — Input Validation Review

- [ ] `parseID` handles negative numbers, overflow, empty strings
- [ ] All JSON parsing errors return 400, not 500
- [ ] `parseTimeRange` returns safe defaults for invalid inputs
- [ ] Alert List: `limit=0` and `offset=0` — does DB handle this correctly?
- [ ] Device Create: no validation on IP address format
- [ ] Device Create: no validation on protocol (could be "foo")
- [ ] Dashboard Save: no authorization check (User A can overwrite User B's dashboard by ID)

#### `collectors/` — Reliability Review

- [ ] Ping collector `SetPrivileged(true)` requires root — documented?
- [ ] HTTP collector creates new `http.Client` per call (connection pool waste)
- [ ] SNMP collector uses `ConnectIPv4` — IPv6 not supported
- [ ] NetFlow parser doesn't validate record boundaries beyond length check
- [ ] System collector is a stub — does it need to exist?

#### `engine/` — Correctness Review

- [ ] `AlertEngine.EvaluateMetric` fetches ALL active alerts to check duplicates (O(n) per metric)
- [ ] `AlertCondition.Match` compares `float64 == threshold` — floating point equality issues
- [ ] `Notifier.Send` only supports `webhook` — other types silently warn
- [ ] `FlowAnalyzer` silently drops errors from `RecordFlows`
- [ ] `AnomalyEngine.Start` is a no-op (TODO comment)

#### `server/` — Middleware Review

- [ ] CORS uses `AllowOriginFunc: func(origin string) bool { return true }` — effectively same as `*`; consider making configurable
- [ ] Rate limiter cleanup goroutine runs indefinitely — should it be cancellable?
- [ ] Request logger creates new body reader for every request — allocation overhead

#### `logging/` — Correctness Review

- [ ] `rotation.go` and `logger.go` both create lumberjack loggers — `rotation.go`'s `NewRotatingWriter` is dead code
- [ ] `sink.go` `MultiSink.Write` returns `len(p)` even if a writer wrote fewer bytes
- [ ] `http_logger.go` body capture doesn't handle `Transfer-Encoding: chunked` (ContentLength=-1)
- [ ] `audit_logger.go` `formatInt64` uses `slog.Int64Value.String()` — unusual approach

---

## 10. Static Analysis & Linting

### `.golangci.yml` Configuration

```yaml
run:
  timeout: 5m
  tests: true

linters:
  enable:
    # Bug detection
    - govet
    - staticcheck
    - errcheck
    - ineffassign
    - typecheck
    
    # Security
    - gosec
    
    # Style & consistency
    - gofmt
    - goimports
    - misspell
    - unconvert
    - unparam
    - unused
    
    # Code quality
    - gocritic
    - revive
    - prealloc
    - bodyclose
    - noctx
    - sqlclosecheck
    - rowserrcheck
    - exportloopref
    
    # Complexity
    - gocyclo
    - funlen
    - lll
    - nestif

linters-settings:
  gocyclo:
    min-complexity: 15
  funlen:
    lines: 100
    statements: 60
  lll:
    line-length: 140
  gocritic:
    enabled-tags:
      - diagnostic
      - style
      - performance
  revive:
    rules:
      - name: exported
        severity: warning
      - name: unexported-return
        severity: warning
  gosec:
    excludes:
      - G104  # Unhandled errors (too noisy, errcheck handles)

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - funlen
        - lll
```

### Specific Lint Checks

```bash
# Full lint suite
golangci-lint run ./...

# Security scan
gosec ./...

# Known vulnerabilities
govulncheck ./...

# Dead code
deadcode ./...

# Missing error checks specifically
errcheck ./...
```

---

## 11. CI/CD Pipeline

### GitHub Actions Workflow

```yaml
# .github/workflows/test.yml
name: Test & Lint
on: [push, pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - uses: golangci/golangci-lint-action@v6
        with: { working-directory: backend }

  unit-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: cd backend && go test -race -coverprofile=coverage.out -covermode=atomic ./...
      - run: cd backend && go tool cover -func=coverage.out | tail -1  # total coverage
      - uses: codecov/codecov-action@v4
        with: { files: backend/coverage.out }

  integration-test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: timescale/timescaledb:latest-pg16
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: netmonitor_test
        ports: ['5432:5432']
        options: --health-cmd pg_isready --health-interval 5s --health-timeout 5s --health-retries 5
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: cd backend && go test -tags integration -race -v ./...
        env:
          DATABASE_DSN: postgres://postgres:postgres@localhost:5432/netmonitor_test?sslmode=disable
          JWT_SECRET: test-secret-for-ci

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: go install github.com/securego/gosec/v2/cmd/gosec@latest
      - run: cd backend && gosec ./...
      - run: go install golang.org/x/vuln/cmd/govulncheck@latest
      - run: cd backend && govulncheck ./...
```

---

## 12. Test Data & Fixtures

### Mock Database

Generate with:
```bash
mockgen -source=internal/database/database.go \
        -destination=internal/database/mock_database.go \
        -package=database
```

### Monitoring Mock

```bash
mockgen -source=internal/monitoring/recorder.go \
        -destination=internal/monitoring/mock_monitoring.go \
        -package=monitoring \
        -mock_names=MonitoringDB=MockMonitoringDB
```

### Test Fixtures Directory Structure

```
backend/
├── testdata/
│   ├── fixtures/
│   │   ├── devices.json         # Sample devices for handler tests
│   │   ├── metrics.json         # Sample metrics
│   │   ├── alerts.json          # Sample alerts
│   │   ├── flows.json           # Sample flow records
│   │   └── netflow_v5.bin       # Binary NetFlow v5 packet
│   ├── golden/
│   │   ├── health_response.json # Expected /health response
│   │   ├── device_list.json     # Expected device list response
│   │   └── csv_export.csv       # Expected CSV export output
│   └── certs/
│       └── test_jwt_secret.txt  # Fixed JWT secret for deterministic tests
```

### Factory Functions

```go
// internal/testutil/factories.go
package testutil

func NewTestDevice(overrides ...func(*models.Device)) *models.Device
func NewTestMetric(deviceID int64, overrides ...func(*models.Metric)) *models.Metric
func NewTestAlert(deviceID int64, overrides ...func(*models.Alert)) *models.Alert
func NewTestUser(overrides ...func(*models.User)) *models.User
func NewTestClaims(userID int64, role string) *auth.Claims
func NewTestConfig() *config.Config  // all defaults + JWT_SECRET="test-secret"
```

---

## 13. Makefile Targets

Add to existing `backend/Makefile`:

```makefile
# ─── Testing ───────────────────────────────────────────────
.PHONY: test test-race test-cover test-integration test-e2e test-security bench

test:                    ## Run unit tests
	go test -short -count=1 ./...

test-race:               ## Run tests with race detector
	go test -race -count=3 ./...

test-cover:              ## Run tests with coverage report
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"
	@go tool cover -func=coverage.out | tail -1

test-integration:        ## Run integration tests (requires Docker)
	go test -tags integration -race -v -count=1 ./...

test-e2e:                ## Run end-to-end tests (requires Docker Compose)
	docker compose -f docker-compose.e2e.yml up -d
	go test -tags e2e -race -v -count=1 ./e2e/...
	docker compose -f docker-compose.e2e.yml down

test-security:           ## Run security scans
	gosec -fmt=json -out=gosec-report.json ./...
	govulncheck ./...

bench:                   ## Run benchmarks
	go test -bench=. -benchmem -run=^$$ ./...

# ─── Code Quality ──────────────────────────────────────────
.PHONY: lint lint-fix fmt vet mock

lint:                    ## Run linters
	golangci-lint run ./...

lint-fix:                ## Run linters with auto-fix
	golangci-lint run --fix ./...

fmt:                     ## Format code
	gofmt -s -w .
	goimports -w .

vet:                     ## Run go vet
	go vet ./...

mock:                    ## Generate mocks
	mockgen -source=internal/database/database.go \
	        -destination=internal/database/mock_database.go \
	        -package=database
	mockgen -source=internal/monitoring/recorder.go \
	        -destination=internal/monitoring/mock_monitoring.go \
	        -package=monitoring

# ─── Combined ──────────────────────────────────────────────
.PHONY: ci

ci: lint vet test-race test-cover test-security  ## Full CI pipeline locally
	@echo "All checks passed ✓"
```

---

## Appendix: Full Test Matrix

| Package | File | Unit Tests | Integration | Fuzz | Benchmark |
|---------|------|:----------:|:-----------:|:----:|:---------:|
| `auth` | jwt.go | 9 | — | ✓ (token parsing) | — |
| `auth` | password.go | 8 | — | ✓ (hash inputs) | ✓ |
| `auth` | session.go | 5 | — | — | — |
| `auth` | apikey.go | 4 | — | — | — |
| `auth` | middleware.go | 11 | ✓ | — | — |
| `config` | config.go | 14 | — | — | — |
| `collectors` | collector.go | 3 | — | — | — |
| `collectors` | http.go | 8 | ✓ | — | — |
| `collectors` | ping.go | 2 | ✓ (privileged) | — | — |
| `collectors` | port.go | 4 | ✓ | — | — |
| `collectors` | snmp.go | 5 | ✓ (mock agent) | — | — |
| `collectors` | netflow.go | 7 | ✓ (UDP) | ✓ (packet parsing) | ✓ |
| `collectors` | system.go | 2 | — | — | — |
| `engine` | alert_conditions.go | 14 | — | — | ✓ |
| `engine` | alert_rules.go | 10 | — | — | ✓ |
| `engine` | alert.go | 5 | ✓ | — | — |
| `engine` | notifier.go | 8 | ✓ (mock HTTP) | — | — |
| `engine` | flow_analyzer.go | 5 | ✓ | — | — |
| `engine` | anomaly.go | 2 | — | — | — |
| `handlers` | auth.go | 16 | ✓ | — | — |
| `handlers` | devices.go | 12 | ✓ | — | — |
| `handlers` | alerts.go | 8 | ✓ | — | — |
| `handlers` | metrics.go | 6 | ✓ | — | — |
| `handlers` | flows.go | 3 | ✓ | — | — |
| `handlers` | dashboards.go | 5 | ✓ | — | — |
| `handlers` | reports.go | 4 | ✓ | — | — |
| `handlers` | capture.go | 5 | — | — | — |
| `handlers` | insights.go | 2 | ✓ | — | — |
| `handlers` | simulator.go | 3 | ✓ | — | — |
| `handlers` | sensors.go | 2 | ✓ | — | — |
| `handlers` | ports.go | 1 | — | — | — |
| `handlers` | health.go | 2 | ✓ | — | — |
| `httputil` | response.go | 5 | — | — | — |
| `logging` | logger.go | 9 | — | — | — |
| `logging` | context.go | 6 | — | — | — |
| `logging` | http_logger.go | 8 | — | — | — |
| `logging` | db_logger.go | 4 | — | — | — |
| `logging` | sink.go | 8 | — | — | — |
| `logging` | audit_logger.go | 5 | — | — | — |
| `logging` | alert_logger.go | 4 | — | — | — |
| `monitoring` | self_monitor.go | 4 | — | — | — |
| `monitoring` | recorder.go | 4 | ✓ | — | — |
| `monitoring` | handlers.go | 8 | ✓ | — | — |
| `server` | middleware.go | 8 | — | — | — |
| `server` | server.go | — | ✓ | — | — |
| `scanner` | port_scanner.go | 7 | ✓ | — | ✓ |
| `scheduler` | scheduler.go | 7 | ✓ | — | — |
| `retention` | retention.go | 3 | ✓ | — | — |
| `websocket` | hub.go | 7 | ✓ (ws client) | — | — |
| `database` | postgres.go | — | ✓ (12 tests) | — | — |
| `models` | models.go | 5 | — | — | — |
| **Security** | — | 10 | — | — | — |
| **E2E** | — | — | — | — | — |
| | | | | | |
| **TOTALS** | **48 files** | **~320** | **~50** | **4** | **6** |

---

> **Estimated effort**: 4–6 developer-weeks for full implementation of this plan.
> **Priority order**: `auth` → `engine` → `handlers` → `database` (integration) → `collectors` → rest.
> **Quick win**: Start with `engine/alert_conditions_test.go` (pure logic, no mocks needed).
