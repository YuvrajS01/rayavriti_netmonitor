# Rayavriti NetMonitor+ — Frontend Review & Improvement Plan

## Current State Summary

The frontend is a **React 19 + TypeScript + TailwindCSS 4 + Recharts** SPA with 9 pages, a sidebar layout, WebSocket real-time updates, and Redux auth state. The dark theme uses a custom neon-green/olive Material Design 3 palette. The app is functional with real-time monitoring, flow analysis, packet capture, AI health scoring, and reporting.

---

## Critical Issues (P0)

### 1. **Massive Code Duplication**
- `statusColor()`, `iconForProtocol()`, `TOOLTIP_STYLE`, `formatBytes()` are copy-pasted across 5+ files (Dashboard, Devices, Sensors, FlowAnalysis, PacketCapture)
- **Fix:** Extract a shared `src/utils/` module: `colors.ts`, `icons.ts`, `formatters.ts`, `chartConfig.ts`

### 2. **Dynamic Tailwind Classes Won't Work**
- `Devices.tsx:248`: `hover:${sc.border}` — Tailwind can't detect dynamically constructed class names at build time
- **Fix:** Use full static class names or inline `style` attributes for dynamic values

### 3. **No Reusable Card/Panel Components**
- Every page repeats the same `<div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">` pattern dozens of times
- **Fix:** Create `<Card>`, `<StatCard>`, `<SectionHeader>`, `<EmptyState>` components

### 4. **Accessibility Gaps**
- Sidebar has no `<nav aria-label>` (`Layout.tsx:112`)
- No skip-to-content link for keyboard users
- Modals don't prevent background scroll
- Packet capture table has no accessible row headers
- Color-only status indicators (no text labels for screen readers)
- **Fix:** Add ARIA labels, skip links, scroll lock in modals, and `aria-live` for real-time data

---

## High Priority Issues (P1)

### 5. **No Responsive Design on Multiple Pages**
- Dashboard: 4-column grid collapses poorly (`md:grid-cols-4` with tiny mobile stat cards)
- AI Health: `xl:grid-cols-[auto_1fr_280px]` breaks on tablet
- Flow analysis: tables scroll horizontally with no mobile alternative
- **Fix:** Add responsive breakpoints, collapsible columns, mobile-friendly stacked layouts

### 6. **Inconsistent Typography & Spacing**
- Page headers use inconsistent sizes: `text-4xl` (Alerts, Sensors), `text-5xl` (Dashboard, Flow, Packet), `text-4xl font-black` (Reports)
- Label styles mix `text-[10px]`, `text-xs`, `text-[9px]` inconsistently
- Section spacing varies: `mb-6`, `mb-8`, `mb-10`, `mb-12`
- **Fix:** Standardize to shared header component and spacing scale

### 7. **No Shared Loading / Error / Empty-State System**
- Each page manually implements its own loading spinner, error banner, and empty state
- **Fix:** Create `<LoadingState>`, `<ErrorState>`, `<EmptyState>` components

### 8. **Bug: `SlaGauge` Text Positioned Absolutely but Parent Isn't Relative**
- `SlaTab.tsx:22`: `<div className="absolute">` inside a parent that lacks `relative`
- **Fix:** Add `relative` to the parent container

### 9. **Sidebar Layout Flash on Mobile**
- Sidebar defaults `open` on mount, then animates shut — causes layout flash
- Mobile bottom nav conflicts with sidebar `ml-64`
- **Fix:** Detect viewport on mount, default sidebar to closed on mobile, use CSS `@media` for initial sidebar state

### 10. **No Skeleton Loading**
- All loading states show a pulsing `hourglass_top` icon — no meaningful preview of what's loading
- **Fix:** Add skeleton placeholder components matching the layout

---

## Medium Priority Issues (P2)

### 11. **No Toast / Notification System**
- Alert acknowledgments, device deletions, capture start/stop give no visual feedback
- **Fix:** Add a toast notification system (e.g., sonner or a custom implementation)

### 12. **Device Form Inline UI is Janky**
- `Devices.tsx:173-237`: The add-device form appears/disappears inline with no animation, takes up significant vertical space, and has no validation feedback
- **Fix:** Move to a modal or drawer with form validation and smooth transitions

### 13. **No Keyboard Navigation for Charts**
- Recharts charts have no keyboard-accessible data
- **Fix:** Add visible data tables as accessible alternatives (can be screen-reader-only)

### 14. **Settings Page is Minimal**
- Only shows username, role, sign-out — no functional settings
- No profile editing, notification preferences, theme toggle, or admin features
- **Fix:** Build out with real settings (notification preferences, theme, user management for admin)

### 15. **No Light Theme**
- Hardcoded dark theme only; `index.html` has `<html class="dark">` but no light mode tokens or toggle
- **Fix:** Add light theme tokens and a theme switcher

### 16. **Performance Concerns**
- `Dashboard.tsx`: `historyMetrics` array capped at 500, but `buildMultiLineData` does O(n) grouping + sorting every render
- `Sensors.tsx`: 120 metric items rendered as individual DOM nodes with no virtualization
- **Fix:** Use `react-window` or similar for long lists; memoize heavier computations

### 17. **No Real-Time Alert Badges in Sidebar**
- Sidebar/alerts icon doesn't show the count of active alerts
- **Fix:** Add badge count to sidebar Notifications link

---

## Low Priority / Polish (P3)

### 18. **Inconsistent Button Styles**
- Rounded vs `rounded-none`, filled vs outlined vs ghost — no consistent pattern
- **Fix:** Define `<Button variant="primary|secondary|danger|ghost">` component

### 19. **No Focus Ring on Interactive Cards**
- Device cards, alert items, and sensor items are clickable but have no `:focus-visible` style
- **Fix:** Add focus-visible ring styles to all clickable non-button elements

### 20. **Color Palette Usability**
- Primary green `#d9fd3a` on dark background has poor readability for body text (used in labels)
- **Fix:** Reserve primary for accents/indicators only; use `--color-on-surface` for text

### 21. **Login Page Doesn't Handle Token Expiry Gracefully**
- If a token expires mid-session, the ProtectedRoute does a `/auth/me` check on every navigation — potential double-requests
- **Fix:** Debounce or cache the auth check

### 22. **No 404 Page**
- Catch-all route redirects to `/` — no "page not found" feedback
- **Fix:** Add a proper 404 page

### 23. **Missing Page Transitions**
- Navigation between pages is instant with no transition — feels jarring
- **Fix:** Add subtle fade/slide transitions using CSS or framer-motion

### 24. **Print Styles Could Be Richer**
- Existing print styles are basic — chart SVGs may not render well in print
- **Fix:** Replace chart areas with tabular data in print media queries

---

## Proposed Implementation Order

| Phase | Items | Effort |
|-------|-------|--------|
| **Phase 1: Foundation** | Extract shared utils (#1), Fix dynamic Tailwind (#2), Create reusable components (#3, #7, #18) | 3-4 days |
| **Phase 2: Accessibility & Bugs** | ARIA additions (#4), Fix SlaGauge (#8), Sidebar mobile (#9), Focus rings (#19), Toast system (#11) | 2-3 days |
| **Phase 3: Responsive & Consistency** | Responsive breakpoints (#5), Typography/spacing standardization (#6), Page headers component | 2-3 days |
| **Phase 4: Features & Polish** | Light theme (#15), Settings page (#14), Alert badges (#17), 404 page (#22), Transitions (#23), Skeleton loading (#10) | 3-4 days |
| **Phase 5: Performance** | List virtualization (#16), Auth check debounce (#21), Print improvements (#24) | 1-2 days |

**Total estimated: ~12-16 days of focused work.**
