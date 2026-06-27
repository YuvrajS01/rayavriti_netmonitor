# Rayavriti NetMonitor — Frontend Audit Report

**Date:** 2026-06-27  
**Scope:** client/src (React/TS/Vite/Tailwind4)  
**Rating:** 6.5/10

---

## Executive Summary

| Severity | Count |
|----------|-------|
| Critical | 5 |
| High | 12 |
| Medium | 18 |
| Low | 11 |

The design does NOT look generically AI-generated. It uses a distinctive dark olive/chartreuse palette, a real MD3 token system, and consistent component patterns. The only AI-slop tells are slight metric-card-grid tendency on Dashboard (mitigated by varied chart types) and minor glassmorphism-adjacent `bg-primary/10` for severity badges — but very restrained.

---

## WCAG Contrast Ratio Summary

| Color Pair | Foreground | Background | Ratio | AA Normal | AA Large |
|-----------|-----------|------------|-------|-----------|----------|
| Primary on bg | `#d9fd3a` | `#0e0e09` | 16.64:1 | PASS | PASS |
| On-surface on bg | `#f4f1e6` | `#0e0e09` | 17.11:1 | PASS | PASS |
| On-surface-variant on bg | `#adaba1` | `#0e0e09` | 8.40:1 | PASS | PASS |
| Outline on bg | `#77766d` | `#0e0e09` | 4.23:1 | **FAIL** | PASS |
| Outline on surface-container | `#77766d` | `#1a1a13` | 3.83:1 | **FAIL** | PASS |
| Error on bg | `#ff7351` | `#0e0e09` | 7.21:1 | PASS | PASS |
| Warning on bg | `#e5a910` | `#0e0e09` | 9.23:1 | PASS | PASS |
| Success on bg | `#8cc63f` | `#0e0e09` | 9.46:1 | PASS | PASS |
| Info on bg | `#6bb8c9` | `#0e0e09` | 8.59:1 | PASS | PASS |
| Unknown (#6b7280) on bg | `#6b7280` | `#0e0e09` | 4.00:1 | **FAIL** | PASS |
| On-surface-variant on surface-container | `#adaba1` | `#1a1a13` | 7.59:1 | PASS | PASS |
| Axis tick (#77766d) on container-highest | `#77766d` | `#26261d` | 3.34:1 | **FAIL** | PASS |

**Three contrast failures:** `--color-outline` (`#77766d`) at 4.23:1 on bg, `#77766d` at 3.34:1 on container, and the non-token `#6b7280` at 4.00:1. All pass for large text (>=18pt / 14pt bold). Fix: lighten outline to `#878770` (~4.6:1) and replace `#6b7280` with the outline token or a lighter shade.

---

## 1. Accessibility (WCAG 2.1 AA)

### Critical

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| A1 | `components/ui/Button.tsx` | No `focus-visible` ring or disabled styling. All buttons across the app are invisible to keyboard users. | Add `focus-visible:ring-2 ring-primary ring-offset-2 ring-offset-surface` and `disabled:opacity-50 disabled:pointer-events-none` |
| A2 | `components/DeviceAddModal.tsx` | `role="dialog" aria-modal="true"` but no focus trap — screen-reader users tab out into background content. | Implement focus trap (use ref-focus pattern from ConfirmDialog) or use `@headlessui/react` dialog. |
| A3 | `utils/colors.ts:7` | `#6b7280` (unknown status) contrast on `#0e0e09` = 4.00:1 — fails WCAG AA for normal text (needs >=4.5:1). | Lighten to `#8b8b81` (~5.1:1) or use `text-outline` token. |

### High

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| A4 | `components/Layout.tsx` | No `<main>` landmark, no skip-to-content link. Screen-reader users must tab through entire sidebar. | Wrap content in `<main id="main-content">`, add `<a href="#main-content" class="sr-only focus:not-sr-only">Skip to content</a>`. |
| A5 | `components/ui/LoadingState.tsx` | Missing `role="status"` — screen readers don't announce loading state. | Add `role="status" aria-live="polite"`. |
| A6 | `components/ui/EmptyState.tsx` | Missing `role="region" aria-label="..."`. | Add `role="region"` with descriptive label. |
| A7 | `components/ui/ErrorState.tsx` | Missing `role="alert"`. | Add `role="alert"` so screen readers announce errors. |
| A8 | `components/dashboard/AiHealthScore.tsx` | SVG radial gauge has no ARIA — screen readers see meaningless SVG. | Add `role="img" aria-label="Health score: {score}%"` to the SVG element. |
| A9 | `components/dashboard/SmartInsights.tsx` | No ARIA on insight cards list. | Add `role="list"` on container, `role="listitem"` on each card. |
| A10 | `components/LocationTree.tsx` | Hierarchical list without `role="tree"`, `role="treeitem"`, `aria-expanded`. | Add tree ARIA roles and `aria-expanded` on expandable nodes. |
| A11 | `components/ConfirmDialog.tsx`, `ExpandedChartsModal.tsx`, `ResourceLoadModal.tsx` | Manual focus trap but no `role="dialog" aria-modal="true"` on outer container divs. | Add `role="dialog" aria-modal="true" aria-labelledby="..."`. |
| A12 | `components/DeviceAddModal.tsx` | Form inputs lack explicit `<label htmlFor>` association — rely only on wrapping labels. | Add `<label htmlFor={id}>` for each input. |
| A13 | `components/dashboard/AvgResponseByStatus.tsx` | Colored status pills have no text alternative for color-blind users. | Add status icon or text label alongside color indicator. |
| A14 | `App.tsx` lazy route transitions | No focus management after page transitions. | Add focus management after route change (focus `#main-content`). |

### Medium

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| A15 | `index.css:101-108` | Custom scrollbar hides native scrollbar affordances — no keyboard-accessible scroll indicators. | Ensure scrollable regions have `tabindex="0"` and visible scroll indicators. |
| A16 | `components/dashboard/StatusDistribution.tsx` | SVG text uses hardcoded `#f4f1e6` / `#77766d` instead of `currentColor` — won't inherit theme. | Use `fill="currentColor"` on SVG text elements. |
| A17 | `components/ui/Card.tsx` | No ARIA role for card landmark. | Add `role="region" aria-label={title}` when card is a distinct section. |

---

## 2. Bugs & Logic Issues

### Critical

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| B1 | `hooks/useSocketEvents.ts` | **Stale closure bug**: `useEffect` captures initial `handlers` ref from first render. If any handler regenerates (e.g. due to state dependency), socket still calls old handlers. Real-time data silently stops updating. | Use a `useRef` to always read latest handlers: `const handlersRef = useRef(handlers); handlersRef.current = handlers;` then call `handlersRef.current[type](payload)` inside the socket callback. |
| B2 | `App.tsx:module-level` | `let sessionChecked = false` at module scope — if module is hot-reloaded or two `<App/>` instances mount, the second mount skips auth check entirely (thinks session already checked). | Move `sessionChecked` into a `useRef` inside the component, or use `useState`. |

### High

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| B3 | `components/ISPLinkModal.tsx` | `useEffect` for body-scroll-lock passes `onClose` in dependency array, but `onClose` is a new function each render -> effect re-runs constantly, toggling `overflow: hidden` on/off rapidly. | Use `onCloseRef` pattern or remove `onClose` from deps and use ref for onClose. |
| B4 | `components/DeviceModal.tsx` | Unsafe double cast: `as unknown as MetricMessagePayload` — bypasses TypeScript's type safety; runtime crash if payload shape changes. | Use a runtime type guard function or Zod schema validation. |
| B5 | `pages/Campus.tsx:35` | `catch {}` (empty catch block) — fetch error silently swallowed, user sees no feedback. | Show ErrorState or toast on fetch failure. |
| B6 | `pages/Settings.tsx:16` | `catch {}` on logout — if logout API call fails, user thinks they're logged out but session token persists. | Show toast on logout failure. |

### Medium

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| B7 | `hooks/SocketProvider.tsx` | Exponential backoff resets cleanly but doesn't clear on `BeforeUnload` — could orphan WebSocket on rapid page-hide/show (mobile). | Call `ws.close()` on `visibilitychange` if hidden, reconnect on visible. |
| B8 | `components/ui/Toast.tsx:module` | `let nextId = 0` at module scope — prevents `Toast` from being used in test isolation; IDs are never recycled. | Use `useRef` + `useState` for ID generation, or `crypto.randomUUID()`. |
| B9 | `components/ResourceLoadModal.tsx` | Uses `let active = true` in `useEffect` for cleanup, but React strict-mode double-fires effects — the first invocation's `active = false` can race with the second. | Use `AbortController` signal instead of mutable `active` flag. |
| B10 | `pages/Incidents.tsx`, `pages/ISP.tsx`, etc. | Same `let active = true` pattern repeated across 5+ pages — DRY violation & same double-fire risk. | Extract `useAsyncEffect` custom hook with `AbortController`. |

---

## 3. Color / Design System

### High

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| C1 | `utils/colors.ts:7` | `#6b7280` (unknown status) is not a theme token — breaks design system consistency. | Replace with `var(--color-outline)` or `var(--color-on-surface-variant)`. |
| C2 | `utils/chartConfig.tsx` | Hardcoded hex for axis tick (`#77766d`), legend (`#adaba1`), `DEVICE_COLORS`, `CHART_COLORS` — won't update if theme changes. | Use `var(--color-outline)` and `var(--color-on-surface-variant)` refs, use `--color-chart-N` tokens for chart colors. |
| C3 | `components/dashboard/AiHealthScore.tsx` | Hardcoded `#26261d`, `#ff7351`, `#e5a910`, `#d9fd3a` in SVG attributes — not theme-aware. | Use `var(--color-surface-container-highest)`, `var(--color-error)`, `var(--color-warning)`, `var(--color-primary)`. |
| C4 | `components/dashboard/ResourceLoadChart.tsx` | Hardcoded `#d9fd3a`, `#cbee29`, `#ff7351` for line colors. | Use `var(--color-primary)`, `var(--color-primary-dim)`, `var(--color-error)`. |
| C5 | `pages/AIHealth.tsx` | `scoreBg()` returns hardcoded hex `#ff7351`, `#e5a910`, `#d9fd3a`. | Replace with CSS var lookups or Tailwind classes. |
| C6 | `pages/Campus.tsx` | Local `statusColors` object duplicates `colors.ts` but with potentially divergent values. | Import from `utils/colors.ts`. |

### Medium

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| C7 | `index.css` | Chart accent palette defined (`--color-chart-1` through `--color-chart-8`) but not used in `chartConfig.tsx` — `CHART_COLORS` duplicates them as raw hex. | Replace `CHART_COLORS` array with references to `var(--color-chart-N)` tokens. |
| C8 | `pages/PacketCapture.tsx` | Good pattern: uses `var(--color-*)` for `PROTO_COLORS` — replicate this across other files. | — |
| C9 | `index.css:77-79` | `::selection` uses `color-mix()` — not supported in Safari < 16.2 (released Dec 2022). | Add fallback solid-color background for older Safari. |

### Low

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| C10 | `index.css` | `--color-surface-variant` and `--color-surface-container-highest` both set to `#26261d` — redundant token. | Verify this is intentional; if not, differentiate values. |
| C11 | `index.css` | Print styles hardcode `#fff`, `#111`, `#444`, `#ddd` — reasonable for print but not theme-aware. | Acceptable as-is; print media has different constraints. |

---

## 4. Responsive / UX

### High

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| R1 | `components/Layout.tsx` | Mobile sidebar overlay doesn't trap focus — user can Tab into hidden sidebar items. | When mobile nav is open, set `aria-hidden="true" inert` on main content, or move focus into sidebar. |
| R2 | `components/ui/Button.tsx` | No explicit `min-height` / `min-width` — touch targets likely < 44x44px on some button variants. | Add `min-h-11 min-w-11` (44px) to all interactive buttons. |
| R3 | `components/dashboard/ExpandedChartsModal.tsx` | Full-screen chart modal may clip on short viewports without scroll. | Add `overflow-y-auto` to modal content area. |

### Medium

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| R4 | `pages/Dashboard.tsx` | Dashboard grid may overflow on viewports between 640-768px (2-column charts squeeze). | Add responsive breakpoint: `sm:grid-cols-1 md:grid-cols-2` instead of jumping directly to 2-col. |
| R5 | `pages/Devices.tsx` | Device table likely overflows horizontally on mobile — no horizontal scroll or card-view fallback. | Add `overflow-x-auto` wrapper or responsive card layout. |
| R6 | `pages/PacketCapture.tsx` | Long packet hex dump lines may overflow container without wrapping. | Add `overflow-x-auto` or `word-break: break-all`. |
| R7 | `components/DeviceModal.tsx` | Tab-panel content (metrics, interfaces) has no max-height with scroll — very long device data pushes modal off-screen. | Add `max-h-[70vh] overflow-y-auto` to content area. |
| R8 | `components/ResourceLoadModal.tsx` | Same as R7 — no max-height on modal body. | — |
| R9 | `components/ui/EmptyState.tsx` | Empty state illustration may be too large on very small screens. | Add responsive sizing: `w-24 sm:w-32`. |

### Low

| # | File & Line | Issue | Fix |
|---|------------|-------|-----|
| R10 | `index.css` | Print styles assume `main { margin-left: 0 }` — sidebar width hard-coded assumption. | Use CSS `@media print { aside { display: none; } main { margin: 0; } }` as already done, so OK. |
| R11 | `pages/ReportBuilder.tsx` | Report preview may not adapt well to mobile-width. | Test and add mobile-specific padding/layout. |

---

## Systemic Patterns

1. **Hard-coded hex colors** appear in 6+ files (AiHealthScore, StatusDistribution, ResourceLoadChart, chartConfig, AIHealth, Campus). The design system defined `--color-chart-N` tokens but they're unused in chart code. Every hardcoded hex breaks theme-switching and is a maintenance liability.
2. **Manual focus trap** duplicated across 4 modals (ConfirmDialog, ExpandedChartsModal, ResourceLoadModal, ISPLinkModal) — inconsistent implementations, one missing entirely (DeviceAddModal). Should be a shared `useFocusTrap` hook or use Headless UI.
3. **`let active = true` in useEffect** pattern repeated in 5+ pages — DRY violation, React 18 strict-mode risk.
4. **Empty catch blocks** in 2 pages — silently swallowed errors give users no feedback.
5. **Missing ARIA** is the most systemic issue: 10+ components lack proper roles, labels, or live regions.

---

## Positive Findings

- **Strong design system**: MD3 token architecture in `index.css` is well-structured with semantic status colors and chart palette.
- **PacketCapture.tsx** is the gold standard for color token usage — uses `var(--color-*)` exclusively.
- **ResponseTimeChart.tsx** has exemplary accessibility: `role="button"`, `tabIndex`, `onKeyDown`, `ChartDataTable` for screen readers.
- **ChartDataTable** component provides screen-reader-accessible data for charts — great pattern.
- **Toast** has `aria-live` region — works correctly for screen readers.
- **Print stylesheet** is well thought out: hides charts, shows data tables (`.sr-only` becomes visible), removes nav.
- **Content-visibility** utility class for off-screen rendering optimization.
- **GPU acceleration** and `will-change` used appropriately for animations.
- **Exponential backoff** in SocketProvider is correctly implemented.

---

## Recommendations by Priority

### Immediate (Critical — fix this sprint)

1. Fix `useSocketEvents` stale closure (B1) — **real-time data stops updating silently**
2. Fix `sessionChecked` race condition (B2) — move to `useRef`
3. Add focus ring + disabled state to Button (A1) — blocks keyboard use app-wide
4. Add focus trap to DeviceAddModal (A2)
5. Replace `#6b7280` with a token or lighten to pass AA (A3, C1)

### Short-term (High — next sprint)

6. Add `<main>` landmark + skip-to-content link (A4)
7. ARIA roles on LoadingState, EmptyState, ErrorState (A5-A7)
8. Add `role="dialog" aria-modal` to modals missing it (A11)
9. Fix ISPLinkModal stale useEffect (B3)
10. Add form labels in DeviceAddModal (A12)
11. Replace all hardcoded hex with CSS vars (C2-C6)
12. Mobile sidebar focus trap (R1)

### Medium-term (Quality — following sprint)

13. Extract `useFocusTrap` hook from 4 modals
14. Extract `useAsyncEffect` hook with AbortController (B9, B10)
15. Use `--color-chart-N` tokens in chartConfig.tsx (C7)
16. Add tree ARIA to LocationTree (A10)
17. SVG gauge ARIA (A8)
18. Touch target sizing (R2)
19. Fix empty catch blocks (B5, B6)

### Long-term (Nice-to-haves)

20. Lighten `--color-outline` to pass AA normal text
21. Responsive dashboard breakpoint refinement (R4)
22. Mobile card-view fallback for tables (R5)
