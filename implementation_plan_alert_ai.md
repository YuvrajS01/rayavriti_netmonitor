# Redesign: Alert Engine & AI Health Score

Comprehensive overhaul of both the Alert Engine and AI Health Score to transform them from basic/half-implemented features into production-grade, genuinely useful systems.

---

## Current State Analysis

### Alert Engine — What's Wrong

| Problem | Details |
|---------|---------|
| **Anomaly detection is a stub** | [evaluateAnomaly](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/alert_conditions.go#L104-L144) always returns `false` — it never fires. The comment says "a future enhancement". |
| **Absence detection is a stub** | [EvaluateCondition](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/alert_conditions.go#L25-L39) returns `false` for `absence` type — the "background loop" mentioned never runs. |
| **CooldownSec is overloaded** | The field doubles as both "sustained duration before firing" and "cooldown between re-fires". The per-condition `DurationSeconds` field exists in the model but is **ignored** by the engine — only `CooldownSec` on the rule is checked. |
| **Generic alert messages** | Alert messages are always `"Alert rule 'X' triggered for Y"` — no context about *what* threshold was breached or *what value* was observed. |
| **No escalation** | If an alert stays active for hours/days, nothing escalates it to a different severity or re-notifies. |
| **No alert correlation / grouping** | 100 devices going down simultaneously creates 100 independent alerts with no grouping or root-cause hint. |
| **Port state change condition is dead** | The seeded rule checks `MetricField: "port_state"` but `evaluateStateChange` only handles `status` — port state changes never trigger alerts. |

### AI Health Score — What's Wrong

| Problem | Details |
|---------|---------|
| **Backend score is discarded** | [anomaly.go computeHealthScores](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/anomaly.go#L62-L174) computes scores but **never returns or stores them** — the results are logged then thrown away. |
| **Frontend re-invents everything** | [insights.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/insights.ts#L4-L104) fetches the bare-bones `/insights` endpoint, throws the server's data away, then builds the entire health model client-side with hardcoded thresholds. |
| **Backend `/insights/current` is trivial** | [insights.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/handlers/insights.go) returns a score of either `100`, `50`, or `0` based on two checks — it's almost useless. |
| **No historical health scores** | The `/insights/history` endpoint returns raw metrics, not historical health scores. The frontend's "12-Hour Timeline" panel gets no real data. |
| **Trends are always "stable"** | The frontend hardcodes `trend: 'stable', trendDelta: 0` — trends never compute. |
| **Port score is hardcoded** | `portScore = 80` always — it never queries actual port scan data. |
| **Stability factor is fake** | `stabilityScore = isUp ? 95 : isDown ? 20 : 50` — no actual uptime/flap analysis. |
| **No server-side persistence** | Health scores are computed in-memory every 5 minutes and discarded. No DB table, no history query. |

---

## User Review Required

> [!IMPORTANT]
> **Database migration required** — This plan adds two new tables (`health_scores`, `health_score_history`) and modifies the alert computation pipeline. Existing alert rules and alerts are preserved.

> [!WARNING]
> **API contract changes** — The `/insights/current` response shape will change from a flat score array to the rich `InsightsResponse` format. The frontend already expects the rich format, so this is actually a **fix**, not a break. The `/insights/history` endpoint will return actual health history instead of raw metrics.

---

## Open Questions

> [!IMPORTANT]
> **Escalation policy** — Should alerts auto-escalate from `warning → critical` after a configurable duration, or should we just re-notify on the same severity? I propose re-notification with escalation as an opt-in per-rule setting.

> [!IMPORTANT]  
> **Alert grouping scope** — When multiple devices fail simultaneously, should we group by: (a) rule name, (b) time window only, or (c) both? I propose grouping by rule + 60-second time window.

---

## Proposed Changes

### Component 1 — Alert Engine Backend

Fixes stubs, adds real anomaly detection, absence monitoring, per-condition duration, richer messages, and alert grouping.

---

#### [MODIFY] [alert.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/alert.go)

- **Rich alert messages**: Replace the generic `"Alert rule 'X' triggered for Y"` with contextual messages that include the actual metric value and threshold. Example: `"High Latency on Gateway: response time 1523ms exceeds 500ms threshold"`.
- **Per-condition duration**: Use `condition.DurationSeconds` instead of `rule.CooldownSec` for the "sustained duration" check. `CooldownSec` becomes exclusively the post-resolution cooldown.
- **Absence background loop**: Implement `Start()` with a ticker that checks for missing metrics per-device (last metric timestamp vs `3× device.Interval`). Fire `absence` alerts when staleness exceeds the configured threshold.
- **Alert grouping**: When creating an alert, check if another alert from the same rule was created within the last 60 seconds. If so, attach a `group_id` to link them. The group leader gets an updated count message.

---

#### [MODIFY] [alert_conditions.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/alert_conditions.go)

- **Implement `evaluateAnomaly`**: Use a rolling z-score approach. Maintain a 24-hour sliding window of metric values per `(device, field)`. Compute mean and stddev. If the current value exceeds `mean ± sensitivity * stddev`, flag as anomaly. The window data comes from `db.GetDeviceMetrics` with a 24h lookback.
- **Implement `evaluateAbsence`**: Accept a `lastMetricTime` parameter. Return `true` if `time.Since(lastMetricTime) > condition.DurationSeconds`.
- **Add port state evaluation**: Handle `MetricField: "port_state"` in `evaluateStateChange` by comparing current open ports against the last scan.
- **Better return context**: Add `Description` field to `ConditionResult` with a human-readable explanation of what was evaluated (e.g., `"response_time 1523ms > 500ms"`).

---

#### [MODIFY] [anomaly.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/anomaly.go)

This file is renamed/refocused — it currently mixes anomaly detection with health score computation. The health score logic moves to a new dedicated file (see below). What remains:
- The `AnomalyEngine` struct stays but becomes a proper anomaly baseline calculator.
- Add `GetBaseline(ctx, deviceID, field, window) (mean, stddev, sampleCount)` method that queries the DB and caches results.
- Cache baselines in-memory with a 15-minute TTL to avoid repeated DB queries.

---

#### [NEW] [health_scorer.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/health_scorer.go)

New dedicated health scoring engine with real, server-side computation:

**Scoring formula** (weighted composite, 0–100):

| Factor | Weight | Source | Computation |
|--------|--------|--------|-------------|
| Availability | 30% | Metrics table | `(up_samples / total_samples) × 100` over the last 1 hour |
| Latency | 25% | Metrics table | Inversely proportional to p95 response time (100 at ≤50ms, 0 at ≥5000ms, linear scale) |
| Alert Load | 20% | Alerts table | `100 - (active_alerts × 25)`, floor 0. Critical alerts count double. |
| Stability | 15% | Metrics table | Based on flap count (status changes) in the last 1 hour. 0 flaps = 100, ≥10 flaps = 0. |
| Port Security | 10% | Port scan results | Based on unexpected port changes in last 24h. 0 changes = 100, ≥5 changes = 0. |

**Trend computation**: Compare current score vs score from 1 hour ago. `delta ≥ +5` = improving, `delta ≤ -5` = degrading, otherwise stable.

**Persistence**: Write each computation to a `health_scores` table (latest snapshot) and `health_score_history` table (time series for timeline charts).

**Scheduling**: Runs every 2 minutes via the existing `AnomalyEngine.Start()` ticker (reduced from 5 min).

---

### Component 2 — Database Layer

---

#### [NEW] Database migration for health score tables

Add migration to create:

```sql
CREATE TABLE IF NOT EXISTS health_scores (
    device_id      BIGINT PRIMARY KEY REFERENCES devices(id) ON DELETE CASCADE,
    score          REAL NOT NULL,
    label          TEXT NOT NULL,  -- healthy, watch, risk, critical
    trend          TEXT NOT NULL DEFAULT 'stable',
    trend_delta    REAL NOT NULL DEFAULT 0,
    factors        JSONB NOT NULL DEFAULT '{}',
    issues         JSONB NOT NULL DEFAULT '[]',
    computed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS health_score_history (
    id          BIGSERIAL PRIMARY KEY,
    device_id   BIGINT REFERENCES devices(id) ON DELETE CASCADE,
    score       REAL NOT NULL,
    label       TEXT NOT NULL,
    factors     JSONB,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_hsh_device_time ON health_score_history(device_id, computed_at DESC);
```

Also add a `group_id` column to the `alerts` table:

```sql
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS group_id TEXT;
```

---

#### [MODIFY] [database interface](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/database)

Add new methods to the `Database` interface:
- `UpsertHealthScore(ctx, deviceID, score *DeviceHealthScore) error`
- `GetHealthScores(ctx) ([]DeviceHealthScore, error)`
- `GetHealthScoreHistory(ctx, deviceID, hours int) ([]HealthHistoryPoint, error)`
- `GetNetworkHealthHistory(ctx, hours int) ([]HealthHistoryPoint, error)` — aggregated average across all devices
- `InsertHealthScoreHistory(ctx, entries []HealthHistoryEntry) error`
- `GetMetricsSince(ctx, deviceID int64, since time.Time) ([]Metric, error)` — for anomaly baselines
- `GetStatusFlaps(ctx, deviceID int64, since time.Time) (int, error)` — count status transitions
- `GetPortChanges(ctx, deviceID int64, since time.Time) (int, error)` — count port state changes

---

### Component 3 — API Handlers

---

#### [MODIFY] [insights.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/handlers/insights.go)

Complete rewrite of the `Current()` handler to return the full `InsightsResponse` structure:

```go
type InsightsResponse struct {
    GeneratedAt        string               `json:"generatedAt"`
    NetworkScore       int                  `json:"networkScore"`
    HealthDistribution HealthDistribution   `json:"healthDistribution"`
    TopRisks           []TopRiskDevice      `json:"topRisks"`
    Health             []DeviceHealthDetail `json:"health"`
    Insights           []InsightItem        `json:"insights"`
}
```

- Read from the `health_scores` table instead of computing on-the-fly.
- `NetworkScore` = weighted average of all device scores.
- `HealthDistribution` = count of devices in each label bucket.
- `TopRisks` = bottom 5 devices sorted by score ascending.
- `Insights` = auto-generated recommendations based on issues (e.g., "Consider increasing polling interval for Device X to reduce flap alerts").

Rewrite `History()` to query `health_score_history` for the network-wide average score timeline:
```go
// GET /insights/history?hours=12
// Returns { points: [{timestamp, score, label}] }
```

---

#### [MODIFY] [alerts.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/handlers/alerts.go)

- Add `group_id` to the alert list response.
- Add a new endpoint `GET /alerts/grouped` that returns alerts grouped by `group_id`, so the frontend can display "5 devices triggered 'Device Down' rule" as a single card that expands.

---

### Component 4 — Frontend: AI Health Score Page

---

#### [MODIFY] [insights.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/insights.ts)

**Delete the client-side score fabrication entirely.** The `getInsights` function currently fetches 3 endpoints, throws away the server response, and rebuilds everything client-side. Replace with:

```ts
export const getInsights = () =>
  api.get('/insights/current').then(r => ({
    data: unwrapGoResponse(r.data) as InsightsResponse,
    success: true,
  }));

export const getInsightsHistory = (hours = 12) =>
  api.get(`/insights/history?hours=${hours}`).then(r => ({
    data: unwrapGoResponse(r.data) as HealthHistoryResponse,
    success: true,
  }));
```

This is a massive simplification — ~100 lines of client code replaced by ~10 lines.

---

#### [MODIFY] [AIHealth.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/AIHealth.tsx)

- No structural changes needed — the component already renders the `InsightsResponse` shape correctly.
- Minor fixes: ensure the trend arrows and deltas now animate correctly since they'll receive real data instead of hardcoded zeros.
- Add a "Last computed" timestamp showing `data.generatedAt` in the header.

---

### Component 5 — Frontend: Alerts Page

---

#### [MODIFY] [Alerts.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Alerts.tsx)

- Add alert grouping display: when multiple alerts share a `group_id`, render them as a collapsible group with a header like "🔴 Device Down — 5 devices affected" and expandable list of individual alerts.
- Show richer alert messages (the backend will now return contextual messages with actual values).
- Add a mini timeline showing when the alert was created, acknowledged, and resolved.

---

#### [MODIFY] [alerts.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/alerts.ts)

- Add `getGroupedAlerts()` API call for the new grouped endpoint.
- Update the `Alert` type to include `groupId?: string`.

---

#### [MODIFY] [types.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/types.ts)

- Add `groupId?: string` to the `Alert` interface.

---

## Verification Plan

### Automated Tests

```bash
# Backend unit tests
cd backend && go test ./internal/engine/... -v -count=1
cd backend && go test ./internal/handlers/... -v -count=1
cd backend && go test ./internal/database/... -v -count=1

# Frontend type checking
cd client && npx tsc --noEmit
```

- Update existing tests in [alert_test.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/alert_test.go) and [alert_conditions_test.go](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/engine/alert_conditions_test.go) for the new behavior.
- Add tests for anomaly z-score computation with known datasets.
- Add tests for health score computation with mocked device/metric data.
- Add handler tests for the new `/insights/current` response shape.

### Manual Verification

- Start the dev server and verify the AI Health page shows real trend data and per-factor breakdowns.
- Verify alerts page shows grouped alerts when multiple devices trigger the same rule.
- Verify that bringing a device down triggers an alert with a rich contextual message.
- Check that the 12-Hour Timeline on the AI Health page populates with real data points.
