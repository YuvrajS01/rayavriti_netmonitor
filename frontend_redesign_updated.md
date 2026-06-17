# UI Overhaul: Clean Flat Design + Expanded Palette

Strip the UI to a clean, flat style with an expanded color system. No glows, no shadows, no gradients on containers. Let content, whitespace, and color do the work. Same fonts (League Spartan / Space Grotesk), zero functional changes.

---

## Expanded Color Palette

The current palette has gaps: warning uses raw Tailwind `amber-400`/`amber-500` (not in the theme), error has two conflicting reds (`#ff7351` vs `#ff4444`), and chart colors are hardcoded hex values with no relationship to the theme.

### New Semantic Colors

These are **warm-toned** to match the existing olive dark theme. Every color was picked to have enough contrast on the dark surfaces while feeling cohesive.

| Token | Hex | Role | Replaces |
|---|---|---|---|
| `--color-warning` | `#e5a910` | Warning states, degraded status | Raw `amber-400`/`amber-500` |
| `--color-on-warning` | `#3d2e00` | Text on warning backgrounds | — |
| `--color-warning-container` | `#e5a910` (at 10% opacity via class) | Warning tint backgrounds | `bg-amber-500/10` |
| `--color-success` | `#8cc63f` | Success states, healthy confirmations | Using `primary` for success |
| `--color-on-success` | `#1a2e00` | Text on success backgrounds | — |
| `--color-info` | `#6bb8c9` | Informational, bandwidth, latency data | Hardcoded `#6ee7f7` |
| `--color-on-info` | `#003540` | Text on info backgrounds | — |

### New Chart Accent Colors

These replace the random hardcoded hex colors scattered across `chartConfig.tsx`, `AIHealth.tsx`, `FlowAnalysis.tsx`, etc. They're designed to be distinguishable on dark backgrounds and harmonize with the olive theme.

| Token | Hex | Chart Purpose |
|---|---|---|
| `--color-chart-1` | `#d9fd3a` | Primary data (existing primary) |
| `--color-chart-2` | `#6bb8c9` | Secondary data (same as info) |
| `--color-chart-3` | `#c084fc` | Tertiary data (soft purple — kept) |
| `--color-chart-4` | `#e5a910` | Quaternary (same as warning) |
| `--color-chart-5` | `#8cc63f` | Quinary (same as success) |
| `--color-chart-6` | `#f0856a` | Senary (warm coral, softened from error) |
| `--color-chart-7` | `#f472b6` | Septenary (warm pink — kept) |
| `--color-chart-8` | `#fb923c` | Octonary (warm orange — kept) |

### Colors Being Removed / Unified

| Old | New | Reason |
|---|---|---|
| `#ff4444` (hardcoded critical red) | `--color-error` (`#ff7351`) | One red, not two |
| `amber-400` / `amber-500` (Tailwind) | `--color-warning` (`#e5a910`) | Theme-consistent warning |
| `#6ee7f7` (hardcoded cyan) | `--color-info` (`#6bb8c9`) | Slightly warmer, less electric |
| `#4ade80` (hardcoded green) | `--color-success` (`#8cc63f`) | Warmer green, distinct from primary |
| `#8a8a78` (hardcoded axis tick) | `--color-outline` (`#77766d`) | Use existing theme token |
| `#c8c5b0` (hardcoded legend text) | `--color-on-surface-variant` (`#adaba1`) | Use existing theme token |

---

## Flat Design System Rules

| Property | Rule |
|---|---|
| **Shadows** | None. Remove all `shadow-*`, `neon-glow`, `ambient-glow-*`, `glow-*` |
| **Gradients** | None on containers. Only inside SVG chart fills (Recharts area gradients) |
| **Borders** | `1px solid` only. `border-outline-variant/20` for cards. No `border-l-2/4/6` accents |
| **Border radius** | `rounded-lg` (8px) for cards/inputs. `rounded-md` (6px) for buttons/badges |
| **Backgrounds** | Solid flat colors. No `backdrop-blur`, no `glass-panel`, no opacity tricks |
| **Hover states** | Background tint only. No glow, no brightness, no border-color animation |
| **Typography** | 5 sizes: `text-2xl` (page title), `text-base` (section title), `text-sm` (body), `text-xs` (labels), `text-[11px]` (metadata) |
| **Font weight** | `font-bold` headings, `font-semibold` sections, `font-medium` labels. No `font-black` |
| **Uppercase** | Only on: tab labels, status badges, metadata. Never on page titles or body |
| **Letter-spacing** | `tracking-wide` max. Remove `tracking-widest`, `tracking-[0.2em]` |
| **Animations** | Keep `page-enter`, chart/gauge animations. Remove all `animate-pulse` dots |
| **Copy tone** | Professional and descriptive. No military/surveillance language |

---

## Proposed Changes

> [!IMPORTANT]
> All changes are **visual and copy only**. No logic, routing, API, WebSocket, or state management changes.

---

### Global Styles + Palette Expansion

#### [MODIFY] [index.css](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/index.css)

**Add new color tokens to `@theme`:**
```css
/* Semantic status colors */
--color-warning: #e5a910;
--color-on-warning: #3d2e00;
--color-success: #8cc63f;
--color-on-success: #1a2e00;
--color-info: #6bb8c9;
--color-on-info: #003540;

/* Chart accent palette */
--color-chart-1: #d9fd3a;
--color-chart-2: #6bb8c9;
--color-chart-3: #c084fc;
--color-chart-4: #e5a910;
--color-chart-5: #8cc63f;
--color-chart-6: #f0856a;
--color-chart-7: #f472b6;
--color-chart-8: #fb923c;
```

**Remove these utility classes entirely:**
- `.neon-glow`, `.glass-panel`, `.glass-panel-light`
- `.ambient-glow-primary`
- `.glow-healthy`, `.glow-watch`, `.glow-risk`, `.glow-critical`
- `.particle-bg`
- `.geometric-input`

**Keep unchanged:** `.gauge-ring`, `.trend-pulse`, `.factor-bar-fill`, `.animate-slide-down`, `.animate-slide-up`, `.page-enter`, `.content-visibility-auto`, all print styles, scrollbar styles, focus styles

---

### Color Utils + Chart Config (Use New Tokens)

#### [MODIFY] [colors.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/utils/colors.ts)
- Replace all `amber-400`/`amber-500` references with `warning` token
- Replace `#ff4444` with `#ff7351` (use the theme's error color)
- Replace `#f59e0b` with `#e5a910` (new warning)
- Update `statusTextColor`: `text-amber-400` → `text-warning`
- Update `statusBgColor`: `bg-amber-500` → `bg-warning`
- Update `statusBorderColor`: `border-amber-500` → `border-warning`
- Update `severityTextColor`: `text-amber-500` → `text-warning`
- Update `severityBgColor`: `bg-amber-500/10` → `bg-warning/10`
- Update `severityBorderColor`: `border-amber-500` → `border-warning`

#### [MODIFY] [chartConfig.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/utils/chartConfig.tsx)
- Replace `DEVICE_COLORS` hex array with CSS variable references using `getComputedStyle` or keep hex values synced to new chart tokens
- Replace `CHART_COLORS` hex array similarly
- Replace `PROTOCOL_COLORS` hardcoded hex with new token hex values
- Replace `AXIS_TICK_STYLE` fill `#8a8a78` with theme outline color
- Replace `legendFormatter` hardcoded `#c8c5b0` with theme on-surface-variant color

---

### Layout (Header + Sidebar + Nav)

#### [MODIFY] [Layout.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/Layout.tsx)
- Header: Remove neon shadow `shadow-[0_0_15px...]`
- Sidebar user: Remove pulsing dot, "{username} Node" → "{username}", remove "Network Ops Center"
- Sidebar active link: Remove gradient + inset shadow → flat `bg-primary/8 text-primary border-l-2 border-primary`
- Sign Out: `rounded-none` → `rounded-md`
- Mobile nav: Remove heavy shadow, `text-[9px]` → `text-[11px]`
- Replace any `text-amber-*` with `text-warning` if present

---

### Shared UI Components

#### [MODIFY] [SectionHeader.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/SectionHeader.tsx)
- Title: `text-5xl font-black uppercase tracking-tight` → `text-2xl font-bold`
- `mb-12` → `mb-8`

#### [MODIFY] [StatCard.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/StatCard.tsx)
- Remove `border-l-2` accent → uniform `border border-outline-variant/20`
- `rounded-xl` → `rounded-lg`
- Label: `text-[10px] tracking-[0.2em]` → `text-xs tracking-wide`
- Value: `text-3xl font-bold` → `text-2xl font-semibold`

#### [MODIFY] [Button.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/Button.tsx)
- Remove primary shadow, `rounded-lg` → `rounded-md`, `tracking-widest` → `tracking-wider`, remove `active:scale-95`

#### [MODIFY] [Card.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/Card.tsx)
- `rounded-xl` → `rounded-lg`, remove hover glow shadow

#### [MODIFY] [EmptyState.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/EmptyState.tsx)
- Remove `opacity-50`

#### [MODIFY] [LoadingState.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/LoadingState.tsx)
- Remove `animate-pulse` from icon

---

### Page: Login

#### [MODIFY] [Login.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Login.tsx)
- Remove blur orbs, particle-bg, neon-glow, glass-panel, geometric-input
- Flat card: `bg-surface-container-high rounded-lg border border-outline-variant/20`
- "Account Login" → "Sign in", "Enter identification string..." → "Username"
- "Network Surveillance Interface" → "Network Monitoring"
- Remove footer security theater text, "System Online" dot, "TLS Encrypted"
- Remove button shadow and arrow animation

---

### Page: Alerts (Major Redesign)

#### [MODIFY] [Alerts.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Alerts.tsx)
- "SYSTEM ALERTS" → "Alerts", professional subtitle
- Replace manual stat cards with `StatCard` component, 3-column grid (equal width)
- Remove `padStart(2, '0')`, dramatic subtitles
- Simplify section dividers to left-aligned text labels
- AlertItem: Remove `border-l-[6px]` → uniform `border border-outline-variant/20`
- Replace `amber-500` severity references with `warning` token
- Buttons: `rounded-md`, sentence case ("Acknowledge" not "ACKNOWLEDGE")

---

### Page: Dashboard

#### [MODIFY] [Dashboard.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Dashboard.tsx)
- Professional subtitle, remove pulsing dot, `mb-12` → `mb-8`

---

### Page: Devices

#### [MODIFY] [Devices.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Devices.tsx)
- Professional subtitle and labels
- `rounded-xl` → `rounded-lg`, remove dynamic hover borders, remove `animate-pulse` dots
- Replace `amber-500` references with `warning`, `#ff4444` with error color
- "New Node" → "Add Device"

---

### Page: Settings

#### [MODIFY] [Settings.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Settings.tsx)
- Use `SectionHeader`, "Settings" title, professional labels
- Remove "SECURED NODE" badge, `shadow-sm`, footer version text

---

### Dashboard Sub-components

#### [MODIFY] [AiHealthScore.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/AiHealthScore.tsx)
- Remove glow classes and drop-shadow filter
- Replace `#ff4444` with `--color-error`, `#f59e0b` with `--color-warning`
- `rounded-xl` → `rounded-lg`

#### [MODIFY] [SmartInsights.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/SmartInsights.tsx)
- `tracking-widest` → `tracking-wide`, `rounded-xl` → `rounded-lg`
- Replace `amber-400` with `warning` token

#### [MODIFY] [ResponseTimeChart.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/ResponseTimeChart.tsx)
- Remove hover glow, `rounded-xl` → `rounded-lg`

#### [MODIFY] [LatestMetricsTable.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/LatestMetricsTable.tsx)
- Remove `shadow-lg`, `rounded-xl` → `rounded-lg`

#### [MODIFY] [ActiveAlertsList.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/ActiveAlertsList.tsx)
- Remove `shadow-lg` and `animate-pulse`, `rounded-xl` → `rounded-lg`

#### [MODIFY] Other dashboard components ([StatusDistribution](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/StatusDistribution.tsx), [ResourceLoadChart](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/ResourceLoadChart.tsx), [AvgResponseByStatus](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/AvgResponseByStatus.tsx))
- `rounded-xl` → `rounded-lg` on all

---

### Other Pages (Flat + Color Token Updates)

#### [MODIFY] [Sensors.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Sensors.tsx)
- Professional subtitle, remove `ambient-glow-primary`
- `tracking-widest` → `tracking-wide`, `rounded-xl` → `rounded-lg`
- Replace `#ff4444` with error color in chart fills
- Replace `amber-*` with `warning` token

#### [MODIFY] [FlowAnalysis.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/FlowAnalysis.tsx)
- Professional subtitle, remove pulsing dots
- Replace all `#6ee7f7` with `--color-info` hex (`#6bb8c9`)
- Replace `#c084fc` with `--color-chart-3`
- `tracking-widest` → `tracking-wide`, `rounded-xl` → `rounded-lg`

#### [MODIFY] [PacketCapture.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/PacketCapture.tsx)
- Professional subtitle
- `tracking-widest` → `tracking-wide`, `rounded-xl` → `rounded-lg`

#### [MODIFY] [AIHealth.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/AIHealth.tsx)
- Professional subtitle, remove glow classes and drop-shadow filters
- Replace `#ff4444` with error, `#6ee7f7` with info, `#4ade80` with success, `#c084fc` with chart-3
- `tracking-widest` → `tracking-wide`, `rounded-xl` → `rounded-lg`

#### [MODIFY] [Reports.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Reports.tsx)
- Use `SectionHeader` if not already
- `tracking-widest` → `tracking-wide`, `rounded-xl` → `rounded-lg`

---

### Modals & Dialogs

#### [MODIFY] All modals: [ConfirmDialog](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ConfirmDialog.tsx), [DeviceModal](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/DeviceModal.tsx), [DeviceAddModal](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/DeviceAddModal.tsx), [ExpandedChartsModal](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ExpandedChartsModal.tsx), [ResourceLoadModal](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ResourceLoadModal.tsx)
- `rounded-xl` → `rounded-lg`
- Standardize tracking and label sizes
- Replace any `amber-*` with `warning`

---

### Report Sub-components

#### [MODIFY] Components in [reports/](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/reports)
- `rounded-xl` → `rounded-lg`, `tracking-widest` → `tracking-wide`
- Replace `#ff4444` with error hex, `#6ee7f7` with info hex, `#f59e0b` with warning hex
- Remove any glow effects from SlaTab gauge

---

## Verification Plan

### Automated Tests
```bash
cd "/home/yuvraj/Projects/Rayavriti NetMonitor/client" && npx tsc --noEmit
```
```bash
cd "/home/yuvraj/Projects/Rayavriti NetMonitor/client" && npm run build
```

### Manual Verification
- Every page renders flat (no visible shadows/glows)
- All colors come from theme tokens (no raw amber/hardcoded hex outside chart fills)
- Warning, success, info colors appear correctly in status indicators, charts, alerts
- All interactive elements function: buttons, modals, forms, tabs, filters
- Responsive behavior unchanged
