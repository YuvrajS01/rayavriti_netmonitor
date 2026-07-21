# Visual UI Redesign — Rayavriti NetMonitor

Transform the existing text-heavy, table-centric UI into a rich, infographic-driven visual experience with interactive charts, animated data visualizations, topology maps, heatmaps, sparklines, and dynamic maps — while preserving all existing functionality.

## Current State Assessment

The app currently has **29 pages** and **~20 components** built with React 19, Vite, Tailwind CSS v4, Redux Toolkit, Recharts, and WebSocket real-time updates. The design follows a "sage charcoal neon-minimalist" theme with OKLCH colors.

**What exists today:**
- Dashboard: 4 stat cards, 1 SVG gauge, 1 multi-line chart, 1 donut chart, 2 bar charts, 1 data table, 1 alert list
- FlowAnalysis: 1 area chart, 2 horizontal bar charts, 1 donut, 1 flow table, 1 live feed
- AIHealth: Device health cards with factor bars
- Other pages: Predominantly text tables with stat cards — no charts, maps, or visual data representations

**What's missing (will be added):**
- Sparklines on stat cards & device cards
- Interactive network topology maps
- Interactive geographic/campus maps
- Heatmaps (device status, time-of-day activity, alert patterns)
- Radar/spider charts for multi-factor health scoring
- Animated counters (rolling numbers)
- Stacked area charts & multi-line comparisons across more pages
- Treemaps for protocol/bandwidth distribution
- Gantt-style timelines for incidents/maintenance
- Sankey diagrams for flow source→destination visualization
- Animated SVG particle effects for live data flows
- Card micro-animations, shimmer loading states
- Ring progress indicators, segmented gauges

---

## User Review Required

> [!IMPORTANT]
> **New Library Additions** — This plan introduces **4 new npm dependencies**:
> 1. `@visx/visx` — Low-level visualization primitives for heatmaps, sparklines, treemaps, Sankey diagrams (Airbnb's D3-React bridge; tree-shakeable, ~50KB gzipped for modules used)
> 2. `react-force-graph-2d` — Canvas-based force-directed graph for network topology (~35KB gzipped)
> 3. `leaflet` + `react-leaflet` — Interactive maps for campus/location geographic views (~40KB gzipped)
> 4. `framer-motion` — Production animation library for counters, transitions, layout animations (~30KB gzipped)
>
> Recharts remains the primary charting library. visx fills gaps (heatmaps, sparklines, treemaps, sankey). Total bundle impact: ~155KB gzipped (with tree-shaking, actual impact will be lower since pages are lazy-loaded).


---

## Open Questions

> [!IMPORTANT]
> 1. **Geographic Map Data** — The `locations` API has `type`, `name`, `code`, and `description` fields but no `latitude`/`longitude` coordinates. Should I:
>    - (A) Add `latitude`/`longitude` fields to the location model on the backend + a migration, OR
>    - (B) Use a campus-style schematic map (SVG floor plan layout) instead of a real geographic map?
> use B 
> 2. **Network Topology Data** — Devices have `parentDeviceId` and `dependencyPort` fields (for parent-child relationships). Should the topology map:
>    - (A) Auto-discover topology from `parentDeviceId` relationships only, OR
>    - (B) Also create a new backend endpoint that attempts to build topology from flow data (src→dst patterns)?
> use B 
> 3. **Performance Budget** — Are you comfortable with lazy-loading the heavier visualizations (maps, topology, Sankey) so they only load when the user navigates to those pages? This keeps initial bundle size minimal.
Yes 
---

## Proposed Changes

The implementation is organized into **5 phases**, each independently deployable:

---

### Phase 1 — Foundation: Animation System, Sparklines, Animated Counters

Establish the visual foundation that every page will build upon.

---

#### [NEW] [AnimatedCounter.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/AnimatedCounter.tsx)

A reusable animated number component using `framer-motion`'s `useSpring` + `useMotionValue`. Numbers will smoothly roll from previous value to new value on update. Supports formatting (decimals, percentages, bytes). Will replace static `{value}` in all StatCard instances.

```tsx
// AnimatedCounter — rolls numbers smoothly on value change
// Props: value, format ('number' | 'percent' | 'bytes' | 'ms'), duration, className
// Uses framer-motion useSpring for 60fps animation
```

---

#### [NEW] [Sparkline.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/Sparkline.tsx)

Tiny inline SVG sparkline chart (using `@visx/shape` + `@visx/curve`). Renders a ~80×24px area chart with gradient fill. Supports trend coloring (green for upward, red for downward). Will be embedded in StatCards and device cards.

```tsx
// Sparkline — minimal inline area chart
// Props: data (number[]), width, height, color, showTrend, animate
// Uses @visx/shape AreaClosed with curveBasis
```

---

#### [NEW] [RingGauge.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/RingGauge.tsx)

Reusable animated ring/arc gauge component with gradient strokes, animated fill using CSS `stroke-dashoffset` transitions, and configurable thresholds for color changes. Replaces the inline SVG in `AiHealthScore`.

---

#### [MODIFY] [StatCard.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/ui/StatCard.tsx)

- Add optional `sparklineData` prop (number[]) to render an inline Sparkline below the value
- Replace static value with AnimatedCounter
- Add optional `trend` prop (up/down/flat) with animated trend indicator arrow
- Add subtle glow effect on the accent color
- Add `delta` prop showing +/- change from previous period

Before:
```
┌─────────────────┐
│ TOTAL DEVICES    │
│ 42               │
└─────────────────┘
```

After:
```
┌─────────────────────┐
│ TOTAL DEVICES   ▲5  │
│ 42  ▁▃▅▇▅▃▂▄▆▅▃    │
│ ~~~~~~~~sparkline~~ │
└─────────────────────┘
```

---

#### [MODIFY] [index.css](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/index.css)

Add new animation keyframes and utility classes:
- `@keyframes count-up` — for number rolling effect
- `@keyframes shimmer` — loading shimmer effect
- `@keyframes glow-pulse` — subtle glow on status indicators
- `@keyframes particle-drift` — for background particle effects
- `.glass-card` — glassmorphism card variant with backdrop-blur
- `.glow-success`, `.glow-error`, `.glow-warning` — colored glow utilities
- Enhanced card hover animations with scale + shadow transitions

---

#### [MODIFY] [DashboardSkeleton.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/DashboardSkeleton.tsx)

Replace basic pulse skeleton with shimmer animation skeleton that mimics the shapes of actual charts and sparklines (chart-shaped skeletons, gauge-ring skeletons).

---

### Phase 2 — Dashboard Visual Overhaul

Transform the dashboard from "adequate monitoring view" to "impressive command center."

---

#### [MODIFY] [Dashboard.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Dashboard.tsx)

- Pass sparkline data to StatCards (derive from `historyMetrics` — last 20 values per device aggregated)
- Add new grid section: **Status Heatmap** (devices × time grid)
- Add new grid section: **Network Topology Mini-view** (force-graph preview, click to expand)
- Add stagger animation to all card sections using `card-stagger` with increasing `--i` values
- Add a "live pulse" animated indicator next to the section header when WebSocket is connected

New layout:
```
┌─────────────┬────────────┬────────────┬────────────┐
│ Total ▁▃▅▇  │ Online ▁▅▇ │ Uptime ▇▅▃ │ Alerts ▅▇█ │
│ 42  ▲5      │ 38  ▲2     │ 94.2%      │ 3  ▼1      │
└─────────────┴────────────┴────────────┴────────────┘
┌──────────────────┬──────────────────┬───────────────┐
│  AI Health Ring  │  Smart Insights  │ Status Heatmap│
│  (animated SVG)  │  (card list)     │ (device×time) │
└──────────────────┴──────────────────┴───────────────┘
┌───────────────────────────────┬─────────────────────┐
│  Response Time (multi-line)   │  Status Distribution│
│  (with area gradient fill)   │  (animated donut)   │
└───────────────────────────────┴─────────────────────┘
┌───────────────────────────────┬─────────────────────┐
│  Resource Load (stacked bars) │  Avg Response Radar │
│                               │  (spider chart)     │
└───────────────────────────────┴─────────────────────┘
┌───────────────────────────────┬─────────────────────┐
│  Latest Metrics Table         │  Active Alerts List  │
│  (with inline sparklines)    │  (severity timeline) │
└───────────────────────────────┴─────────────────────┘
```

---

#### [NEW] [StatusHeatmap.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/StatusHeatmap.tsx)

A device-vs-time heatmap grid using `@visx/heatmap`. Rows = devices, columns = time buckets (last 24h in 30-min slots). Colors: green (up), yellow (warning), red (down), gray (no data). Hover shows tooltip with device name, time, status, response time. Click drills into device detail.

```
Device A  ■■■■■□□■■■■■■■■□□□■■■■■■
Device B  ■■■□□□□□□■■■■■■■■■■■■■■■
Device C  ■■■■■■■■■■■■□□□□□■■■■■■■
          └── 24h ago ─────── now ──┘
```

---

#### [NEW] [NetworkTopologyMini.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/NetworkTopologyMini.tsx)

A compact, non-interactive force-directed graph preview showing the top 20 devices with status-colored nodes. Renders on a small canvas area on the dashboard. Click expands to full topology page. Uses `react-force-graph-2d` with custom node rendering.

---

#### [MODIFY] [ResponseTimeChart.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/ResponseTimeChart.tsx)

- Switch from `LineChart` to `AreaChart` with gradient fills under each device line
- Add animated entry — lines draw from left to right using Recharts `animationBegin` stagger
- Add cursor crosshair with synchronized tooltip across all lines
- Add hover glow effect on active line

---

#### [MODIFY] [StatusDistribution.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/StatusDistribution.tsx)

- Add entry animation — slices grow from 0° to final angle
- Add hover: slice expands outward + glow effect
- Add animated center label that transitions when hovering different slices
- Add outer ring showing trend (was this better or worse than yesterday?)

---

#### [MODIFY] [AiHealthScore.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/AiHealthScore.tsx)

- Replace inline SVG with the new `RingGauge` component
- Add animated color gradient on the ring (shifts from red→yellow→green as score changes)
- Add particle effect inside the ring when score > 90 (celebration effect)
- Add small sparkline showing health trend below the gauge

---

#### [MODIFY] [ResourceLoadChart.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/ResourceLoadChart.tsx)

- Replace static horizontal bars with animated stacked bar chart
- Add CPU/Memory/Disk breakdown as a stacked area chart over time
- Add animated fill with spring physics
- Glow threshold warnings (e.g., red glow when CPU > 90%)

---

#### [MODIFY] [AvgResponseByStatus.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/AvgResponseByStatus.tsx)

- Replace bar chart with a **radar/spider chart** showing multi-factor comparison (availability, latency, alert rate, stability, port security) per status group
- Uses `@visx/shape` Polygon + custom radar grid

---

#### [MODIFY] [LatestMetricsTable.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/LatestMetricsTable.tsx)

- Add inline sparkline column showing response time trend for each device (last 10 readings)
- Add animated status dot with glow pulse
- Add bar indicator for response time (visual bar proportional to value)
- Row entrance animation (stagger fade-in from top)

---

#### [MODIFY] [ActiveAlertsList.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/dashboard/ActiveAlertsList.tsx)

- Add severity-colored left border accent
- Add pulsing dot for critical alerts
- Add mini timeline showing when alert was triggered (relative time bar)
- Add entrance animation (slide in from right)

---

### Phase 3 — Devices, Campus & Network Topology

Add visual richness to inventory and spatial pages.

---

#### [MODIFY] [Devices.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Devices.tsx)

- Add Sparkline to each device card showing response time trend
- Add animated status ring around the protocol icon (pulsing ring for "up", static for "down")
- Add "View as Topology" toggle button that switches to network topology view
- Device card hover: subtle scale(1.02) + shadow elevation + border glow
- Add animated entry for device cards (stagger grid animation)

---

#### [NEW] [NetworkTopology.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/NetworkTopology.tsx)

Full-page interactive network topology view using `react-force-graph-2d`:

- **Nodes**: Each device is a node, colored by status (green/yellow/red/gray)
- **Edges**: Derived from `parentDeviceId` relationships + optional flow data
- **Node rendering**: Custom canvas drawing — icon based on device category, status ring, name label
- **Interactions**: Click node to open DeviceModal, hover for tooltip with key stats
- **Layout**: Force-directed with manual pin/unpin, zoom/pan
- **Filters**: Filter by status, protocol, location
- **Live updates**: Nodes pulse/change color in real-time via WebSocket metric updates
- **Edge animation**: Animated particles flowing along edges for active connections

Route: `/devices/topology` (alternate view from Devices page)

---

#### [MODIFY] [Campus.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Campus.tsx)

Add a **visual campus map** alongside the existing tree view:

- **Tab 1 (Tree View)**: Existing location tree (keep as-is, enhance with status indicators)
- **Tab 2 (Map View)**: Interactive map using Leaflet showing locations as markers
  - Marker color by aggregate status (green if all devices up, red if any down)
  - Marker size proportional to device count
  - Click marker → shows popup with device summary, link to detail
  - Cluster markers when zoomed out
- **Tab 3 (Floor Plan)**: SVG-based schematic showing building layout with device positions (future, placeholder for now)

Add a **location health heatmap** at the top showing all locations as colored tiles.

---

#### [NEW] [CampusMap.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/CampusMap.tsx)

Leaflet-based interactive map component:
- Renders `react-leaflet` Map with tile layer (OpenStreetMap)
- Locations rendered as CircleMarkers with status-based colors
- Custom popup with device count, status breakdown, sparkline of uptime
- Animated marker transitions when status changes
- Supports dark theme tile layer to match app aesthetic

---

#### [MODIFY] [DeviceModal.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/DeviceModal.tsx)

- Add a **response time sparkline** in the modal header
- Add a **mini heatmap** showing device uptime over last 7 days
- Add animated health ring gauge for the device's AI health score
- Add port scan results as a **visual port grid** (colored squares for open/closed ports)
- Animate modal entrance with scale + fade

---

### Phase 4 — Flow Analysis, Packet Capture & AI Health Visual Upgrade

Rich data visualizations for the analysis-heavy pages.

---

#### [MODIFY] [FlowAnalysis.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/FlowAnalysis.tsx)

- **Sankey Diagram**: Add a new section showing flow from top sources → top destinations with proportional band widths. Uses `@visx/sankey`. Color-coded by protocol.
- **Treemap**: Replace protocol distribution donut with a treemap for bandwidth distribution by protocol (more space-efficient, shows hierarchy).
- **Live Flow Animation**: Add animated particle trail effect on the traffic chart — particles flow along the area chart curve to indicate live data.
- **Geo-Flow Map** (if lat/lng available): Map showing flows between geolocated IPs with animated arcs.
- StatCards get animated counters + sparklines.

New sections layout:
```
┌────────────────────────────────────────────────┐
│ [Stats Row with Sparklines & Animated Numbers] │
├───────────────────────────────────────────────-┤
│ Traffic Volume Over Time (enhanced area chart) │
├───────────────────┬───────────────┬────────────┤
│ Source→Dest Sankey │ Protocol      │ Bandwidth  │
│ (flow diagram)    │ Treemap       │ Comparison │
├───────────────────┴───────────────┴────────────┤
│ Flow Records Table (with inline sparklines)    │
│ Live Flow Feed (with animated entry)           │
└────────────────────────────────────────────────┘
```

---

#### [NEW] [SankeyDiagram.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/charts/SankeyDiagram.tsx)

Reusable Sankey/alluvial diagram component using `@visx/sankey`:
- Left nodes = source IPs, right nodes = destination IPs
- Band width proportional to bytes transferred
- Color-coded by protocol
- Hover highlights connected paths
- Animated band drawing on initial render

---

#### [NEW] [Treemap.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/charts/Treemap.tsx)

Reusable treemap component using `@visx/treemap`:
- Nested rectangles proportional to value
- Color by category/protocol
- Hover shows tooltip with details
- Animated transitions when data changes
- Supports zooming into subcategories

---

#### [MODIFY] [PacketCapture.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/PacketCapture.tsx)

- Add **protocol distribution donut** for captured packets
- Add **packet size histogram** (bar chart, buckets: 0-64B, 64-128B, 128-256B, etc.)
- Add **real-time packet rate sparkline** in the capture controls area
- Add animated packet flow visualization — a horizontal stream showing packets as dots flowing left→right, colored by protocol
- Enhance capture session list with progress ring showing capture duration

---

#### [MODIFY] [AIHealth.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/AIHealth.tsx)

- Replace text-based health distribution with **animated donut chart**
- Add **radar/spider chart** for each device showing the 5 health factors (availability, latency, alerts, stability, ports) as axes
- Add **health trend line chart** (24h history with gradient fill)
- Add **health score heatmap** across all devices over time
- Add animated transitions between health states
- Add visual comparison mode: select 2 devices, see overlay radar charts

New layout:
```
┌──────────────────┬──────────────────┬───────────────┐
│ Network Score    │ Health           │ Risk Score    │
│ (animated ring)  │ Distribution     │ Ranking       │
│                  │ (animated donut) │ (sorted bars) │
├──────────────────┴──────────────────┴───────────────┤
│ Health Trend Over Time (24h sparkline area chart)   │
├─────────────────────────────────────────────────────┤
│ Device Health Cards (with radar chart per device)   │
│ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│ │ Radar   │ │ Radar   │ │ Radar   │ │ Radar   │   │
│ │ Chart   │ │ Chart   │ │ Chart   │ │ Chart   │   │
│ │ Score:88│ │ Score:72│ │ Score:95│ │ Score:45│   │
│ └─────────┘ └─────────┘ └─────────┘ └─────────┘   │
└─────────────────────────────────────────────────────┘
```

---

#### [NEW] [RadarChart.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/charts/RadarChart.tsx)

Reusable radar/spider chart using `@visx/shape`:
- Configurable axes (labels, max values)
- Animated polygon fill on data change
- Multiple overlaid datasets for comparison
- Custom themed grid lines (matching sage charcoal palette)
- Hover tooltips per axis

---

#### [NEW] [HeatmapChart.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/charts/HeatmapChart.tsx)

Reusable heatmap component using `@visx/heatmap`:
- Configurable rows/columns/color scale
- Hover tooltips
- Animated cell rendering (fade-in stagger)
- Supports continuous and categorical color scales
- Click handler for cell drill-down

---

### Phase 5 — Secondary Pages Visual Enhancement

Apply the visual system to remaining pages.

---

#### [MODIFY] [Alerts.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Alerts.tsx)

- Add **alert timeline chart** — area chart showing alert frequency over time (24h), colored by severity
- Add **severity distribution donut** in the header area
- Add animated alert count badges
- Add severity-colored left border on alert rows
- Add sparkline showing alert frequency trend

---

#### [MODIFY] [Reports.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Reports.tsx)

- Add visual chart previews in report cards (mini sparklines showing report data)
- Add animated stat counters for report summaries

---

#### [MODIFY] [ReportBuilder.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/ReportBuilder.tsx)

- Enhance chart visualizations with gradient fills and animations
- Add multi-line comparison view for device breakdowns
- Add visual indicators for availability thresholds

---

#### [MODIFY] [Incidents.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Incidents.tsx)

- Add **incident timeline** — Gantt-style horizontal bar chart showing incident duration over time
- Add severity breakdown donut
- Add animated counters for open/resolved counts

---

#### [MODIFY] [IncidentDetail.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/IncidentDetail.tsx)

- Add visual timeline showing incident lifecycle (created → acknowledged → resolved)
- Add affected device mini-map showing impacted topology nodes
- Add related metrics sparkline during the incident window

---

#### [MODIFY] [ISP.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/ISP.tsx)

- Add **ISP bandwidth comparison chart** — grouped bar chart comparing upload/download across ISP links
- Add **uptime history heatmap** per ISP link
- Add animated progress rings for utilization percentages
- Add sparklines on ISP link cards showing latency trend

---

#### [MODIFY] [Sensors.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Sensors.tsx)

- Add sparklines on sensor cards showing recent readings
- Add animated status indicators
- Add visual grouping by parent device with connection lines

---

#### [MODIFY] [RemoteMonitoring.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/RemoteMonitoring.tsx)

- Add **remote instance map** — geographic map showing remote instances as markers
- Add health trend sparklines per instance
- Add animated connection status indicators
- Add latency comparison bar chart across instances

---

#### [MODIFY] [Logs.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Logs.tsx)

- Add **log volume timeline** — area chart at the top showing log entries over time
- Add log level distribution mini-donut
- Add animated log entry appearance (slide-in from bottom)

---

#### [MODIFY] [Discovery.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Discovery.tsx)

- Add **discovery progress visualization** — animated radar sweep effect during active scans
- Add discovered device map/grid showing subnet layout
- Add animated counters for discovered devices

---

#### [MODIFY] [Maintenance.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Maintenance.tsx)

- Add **maintenance calendar heatmap** — showing scheduled maintenance density over time
- Add Gantt-style timeline for active/upcoming maintenance windows
- Add animated countdown timers for upcoming windows

---

#### [MODIFY] [Login.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/Login.tsx)

- Add subtle animated background (particle network effect or gradient animation)
- Add glassmorphism card effect for the login form
- Add smooth form field focus animations

---

#### [MODIFY] [UserManagement.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/pages/UserManagement.tsx)

- Add role distribution donut chart
- Add animated user count stats
- Add visual permission matrix

---

### Cross-Cutting: Shared Chart Components

#### [NEW] [components/charts/](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/components/charts/) directory

Create a dedicated charts directory with all reusable visualization components:

| Component | Library | Used In |
|-----------|---------|---------|
| `Sparkline.tsx` | @visx/shape | StatCard, DeviceCard, Tables |
| `RingGauge.tsx` | SVG + framer-motion | Dashboard, AIHealth, ISP |
| `RadarChart.tsx` | @visx/shape | AIHealth, DeviceModal |
| `HeatmapChart.tsx` | @visx/heatmap | Dashboard, AIHealth, Maintenance |
| `SankeyDiagram.tsx` | @visx/sankey | FlowAnalysis |
| `Treemap.tsx` | @visx/treemap | FlowAnalysis, Reports |
| `GanttTimeline.tsx` | @visx/shape + scale | Incidents, Maintenance |
| `AnimatedDonut.tsx` | Recharts + framer-motion | Dashboard, Alerts, AIHealth |
| `AreaSparkline.tsx` | @visx/shape | ISP, Sensors, Remote |

---

### App-Level Changes

#### [MODIFY] [App.tsx](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/src/App.tsx)

- Add lazy route for new `/devices/topology` page
- Import and setup Leaflet CSS globally (loaded conditionally)

#### [MODIFY] [vite.config.ts](file:///home/yuvraj/Projects/Rayavriti%20NetMonitor/client/vite.config.ts)

- Add manual chunk splitting for chart libraries to optimize bundle:
  - `vendor-charts` chunk: recharts + @visx modules
  - `vendor-maps` chunk: leaflet + react-leaflet
  - `vendor-topology` chunk: react-force-graph-2d

---

## Verification Plan

### Automated Tests

```bash
# Type-check all new components
cd client && npx tsc -b --noEmit

# Run existing tests to ensure no regressions
cd client && npm test

# Lint check
cd client && npm run lint

# Build verification (ensures no import/bundle errors)
cd client && npm run build
```

### Manual Verification

After each phase:
1. **Visual Review**: Open each modified page in the browser and verify:
   - Charts render correctly with real data and empty states
   - Animations are smooth (60fps, no jank)
   - Dark theme consistency (no white flashes, correct OKLCH colors)
   - Responsive layouts work on mobile/tablet/desktop
   - Sparklines show meaningful data trends
   - Interactive elements (hover, click, zoom) work correctly

2. **Performance Check**:
   - Verify lazy-loading works (chart libraries only load when needed)
   - Check bundle size with `npm run build` output
   - Test with `React.StrictMode` for double-render safety
   - Verify WebSocket real-time updates don't cause excessive re-renders

3. **Accessibility**:
   - All charts have `aria-label` descriptions
   - Screen-reader-only data tables exist for all visual charts (existing pattern preserved)
   - Color is never the sole indicator (shapes/icons used alongside)
   - Animations respect `prefers-reduced-motion`

---

## Implementation Order Summary

| Phase | Scope | Files | Estimated Effort |
|-------|-------|-------|-----------------|
| **Phase 1** | Foundation (animations, sparklines, counters) | 6 new/modified | Core infrastructure |
| **Phase 2** | Dashboard overhaul | 10 new/modified | Highest visual impact |
| **Phase 3** | Devices, Campus, Topology | 5 new/modified | Maps & topology |
| **Phase 4** | Flows, Capture, AI Health | 7 new/modified | Data-heavy viz |
| **Phase 5** | All remaining pages | 12+ modified | Broad polish |
