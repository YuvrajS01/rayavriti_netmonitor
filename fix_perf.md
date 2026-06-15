# Fix Slow & Failed Page Loads — Dashboard, AI Health, Alerts

Pages take a long time to load and sometimes don't load at all without a manual refresh. Root cause analysis identified **7 issues** across the frontend and backend.

## Root Causes Identified

| # | Issue | Impact | Location |
|---|-------|--------|----------|
| 1 | **`ProtectedRoute` calls `/auth/me` on every navigation** — blocks rendering with "Verifying session..." | Every page transition is delayed by an API round-trip | [App.tsx:34-62](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/App.tsx#L34-L62) |
| 2 | **Dashboard's `getSystemInfo()` is sequential in `finally` block** — and the backend handler sleeps 1 second (`cpu.Percent(time.Second)`) | Dashboard load takes at least 1s longer than necessary | [Dashboard.tsx:185-191](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Dashboard.tsx#L185-L191) + [system.go:24](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/backend/internal/handlers/system.go#L24) |
| 3 | **`getInsights()` makes 3 sub-requests**, duplicating `/metrics/latest` and `/alerts` that Dashboard already fetches | 6-7 total API calls on Dashboard instead of 4 | [insights.ts:5-17](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/insights.ts#L5-L17) |
| 4 | **Token refresh `axios.post()` has no timeout** | Could hang all authenticated requests indefinitely | [http.ts:70](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/http.ts#L70) |
| 5 | **Alerts page missing `setLoading(true)`** on tab switches | No loading indicator on tab change; stale data displayed | [Alerts.tsx:24](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Alerts.tsx#L24) |
| 6 | **Alerts ack/resolve have no error handling** | Unhandled rejections, no user feedback on failure | [Alerts.tsx:43-51](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Alerts.tsx#L43-L51) |
| 7 | **AIHealth has no error UI** — silently shows "No devices" on API failure | Misleading empty state, no retry affordance | [AIHealth.tsx:324-325](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/AIHealth.tsx#L324-L325) |

---

## Proposed Changes

### Auth & Routing

#### [MODIFY] [App.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/App.tsx)

The `ProtectedRoute` component calls `api.get('/auth/me')` on **every mount** (every page navigation), blocking rendering with "Verifying session..." until the API responds. This is the biggest contributor to the "slow load" feeling across **all** pages.

**Fix:** Cache the session check result so it only runs once per app session, not on every route change. The subsequent navigations will use the cached result and render immediately. A failed check (or 401 interceptor) will still log the user out.

---

### Dashboard

#### [MODIFY] [Dashboard.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Dashboard.tsx)

1. **Make `getSystemInfo()` non-blocking:** Move it out of the `finally` block so it doesn't delay `setLoading(false)`. Fire it as a parallel call alongside the initial `Promise.all`, not sequentially after it. The 1-second `cpu.Percent()` sleep on the backend means this call always takes 1+ seconds — it should never block the page render.

2. **Deduplicate `getInsights()` sub-calls:** The Dashboard already fetches `/metrics/latest` and `/alerts?status=active` via its `Promise.all`. But `getInsights()` (called in the same `Promise.all`) internally fetches the same two endpoints again. Pass the already-fetched data into the insights enrichment logic instead of re-fetching.

---

### Alerts Page

#### [MODIFY] [Alerts.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Alerts.tsx)

1. **Add `setLoading(true)` at the start of `load()`** so tab switches show a loading indicator.
2. **Wrap `handleAck` and `handleResolve` in try/catch** with user-visible error feedback.

---

### AI Health Page

#### [MODIFY] [AIHealth.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/AIHealth.tsx)

1. **Add an error state** — currently the catch block is empty and the page silently shows "No devices match this view" when the API fails. Add an `error` state with a visible error banner and retry button.

---

### HTTP Client

#### [MODIFY] [http.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/http.ts)

Add a `timeout` to the bare `axios.post()` call used for token refresh (line 70). Currently it has no timeout, so if the auth server is slow/unreachable, **all** authenticated API calls queue behind it and hang until the browser's default TCP timeout (30-120s).

---

### Insights API Client  

#### [MODIFY] [insights.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/api/insights.ts)

Refactor `getInsights()` to accept optional pre-fetched metrics and alerts data. When called from Dashboard (which already has this data), avoid the redundant API calls. When called standalone (from AIHealth page), still fetch internally.

---

## Verification Plan

### Automated Tests
```bash
cd client && npx tsc --noEmit
```

### Manual Verification
- Navigate between Dashboard → Alerts → AI Health → Dashboard — confirm no "Verifying session..." delay after initial login
- Dashboard loads without 1s+ delay from system info
- Alerts tab switches show loading state
- AI Health shows error banner on API failure (can test by stopping backend)
