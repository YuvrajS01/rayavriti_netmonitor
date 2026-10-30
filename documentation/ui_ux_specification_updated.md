# UI/UX Specification — Rayavriti NetMonitor v5 design system

This document describes the design system that ships in `client/`. It replaces the
earlier lime / League Spartan / neon-glow specification, which no longer matches the
product. When the code and this document disagree, the code is right and this
document needs fixing.

**Audience.** Campus NOC operators. They work at a desk and on wall displays seen
from across a room, often on cheap projectors. They need to spot what is broken
quickly. Every rule below follows from that.

**Sources of truth**

| Concern | File |
|---|---|
| Tokens (colour, type, radius, elevation, z-index, motion, density) | `client/src/index.css` |
| Theme preference and density at runtime | `client/src/theme/theme.ts`, `client/public/theme-init.js` |
| Status, severity, lifecycle, health bands | `client/src/utils/status.ts` |
| Chart theme | `client/src/utils/chartConfig.tsx`, `client/src/utils/tokens.ts` |
| Primitives | `client/src/components/ui/*` |
| Contrast guarantees | `client/src/__tests__/contrast.test.ts` |

---

## 1. Principles

1. **Status first.** Status colours are the only saturated colours on screen. Everything else is neutral, so red always means trouble.
2. **Never colour alone.** Every status has a shape, an icon and a label. The UI must still work for colour-blind users, in greyscale print, and on a washed-out projector.
3. **No data means no widget.** We never draw placeholder, invented or "illustrative" numbers.
4. **Motion explains a change, then stops.** There are no infinite loops.
5. **Readable from three metres.** Text is never smaller than 12px. Borders are solid. Surface steps are far enough apart to survive a projector.

---

## 2. Themes

There are two complete themes. **Dark** is the default and suits a NOC. **Light** is for daytime offices and for printing. The preference has three values:

| Preference | Behaviour |
|---|---|
| System | Follows `prefers-color-scheme` and updates live when the OS setting changes |
| Dark | Always dark |
| Light | Always light |

- **Where it is stored.** The preference is saved in `localStorage` under `nm-theme`. Density is saved under `nm-density` (`comfortable` or `compact`).
- **How it is applied.** The resolved theme is written as `data-theme="dark|light"` on `<html>`. Compact density adds `data-density="compact"`.
- **Before first paint.** `public/theme-init.js` is a classic, render-blocking script in `<head>`. It is not inline because the CSP forbids inline scripts. It applies both attributes so the page never flashes the wrong theme.
- **After boot.** `theme.ts` takes over. It follows OS changes while System is selected, syncs across tabs, and exposes the `useThemePreference`, `useResolvedTheme` and `useDensity` hooks.
- **Controls.** *Settings › Appearance* has Theme and Density. The header **account menu** has Theme.
- **Print** always uses the light tokens.

---

## 3. Colour tokens

Tokens are Tailwind v4 `@theme` variables. The dark values sit in `@theme`. `:root[data-theme="light"]` overrides the same names. Use them as utilities, for example `bg-surface-1`, `text-fg-muted`, `border-line` or `text-status-down`. In SVG or inline styles, use `var(--color-…)`. Canvas code cannot read `var()`, so it uses `readToken('status-down')` at draw time and re-reads on `THEME_CHANGE_EVENT`.

**Never** write hex or rgb literals, or default Tailwind palette classes (`green-400` and so on), in TSX.

### 3.1 Surfaces and borders

| Token | Dark | Light | Use |
|---|---|---|---|
| `canvas` | `#0B0F0D` | `#F5F7F6` | Page background, header, sidebar |
| `surface-1` | `#121815` | `#FFFFFF` | Cards, panels, dialogs |
| `surface-2` | `#19211D` | `#EEF2F0` | Hover, nested panels, tooltips, selected rows |
| `surface-3` | `#212A25` | `#E2E8E5` | Pressed or selected controls, chips, gauge tracks |
| `inset` | `#0B0F0D` | `#F5F7F6` | Inputs and recessed wells inside cards |
| `line` | `#2E3832` | `#D3DBD7` | Structural borders and dividers. Always solid, never alpha |
| `line-strong` | `#46524B` | `#A3AEA8` | Hovered borders, input borders on emphasis, axes |
| `scrim` | black at 62% | `#141A17` at 45% | Modal and drawer backdrop (`dialog::backdrop`, mobile nav scrim) |

### 3.2 Text and accent

| Token | Dark | Light | Use |
|---|---|---|---|
| `fg` | `#E6ECE8` | `#141A17` | Primary text and values |
| `fg-muted` | `#A3AFA8` | `#4B5751` | Secondary text and labels |
| `fg-subtle` | `#7F8B85` | `#616D67` | Captions, placeholders, chart axis ticks |
| `primary` | `#EADDC1` (ivory) | `#5C4A1E` | Accent: primary buttons, selected tab, active nav bar, links |
| `on-primary` | `#1A1710` | `#FFFFFF` | Text on `primary` |
| `focus` | same as primary | same as primary | 2px focus outline |

### 3.3 Status

| Status | Dark | Light | Shape | Icon | Label |
|---|---|---|---|---|---|
| up | `#2DD4BF` | `#0F7A45` | ● circle | `check_circle` | Up |
| warning | `#F5B83D` | `#945600` | ▲ triangle | `warning` | Warning |
| down | `#F4626A` | `#C02A26` | ■ square | `dangerous` | Down |
| unknown | `#9AA4B2` | `#5B6472` | ? (ring) | `help` | Unknown |
| maintenance | `#A78BFA` | `#6D4FD8` | ‖ pause | `pause_circle` | Maintenance |
| info | `#60A5FA` | `#1D63C9` | – dash | `info` | Info |

- **Text on a solid status fill.** `on-status` is `#0B0F0D` in dark and `#FFFFFF` in light.
- **Feedback aliases.** `success` maps to up, `error` to down, `warning` and `info` to themselves. These are for form errors, toasts and destructive buttons.
- **Destructive hover.** `error-hover` is the hover colour for destructive buttons.

### 3.4 Categorical chart palette

There are eight hues, and they deliberately avoid the status hues. **A data series is never drawn in alarm red, amber or teal.** Use them in fixed order and never cycle past eight. A ninth series goes into "Other".

| Slot | Dark | Light |
|---|---|---|
| chart-1 (blue) | `#4E86E8` | `#2F6BD0` |
| chart-2 (orange) | `#D9692A` | `#C2561A` |
| chart-3 (orchid) | `#C55BC0` | `#A43FA0` |
| chart-4 (green) | `#5FA83A` | `#3F8A26` |
| chart-5 (cerulean) | `#2E9DC2` | `#1C7FA6` |
| chart-6 (rose) | `#D45E8A` | `#C04274` |
| chart-7 (olive) | `#A38F2A` | `#8A7414` |
| chart-8 (periwinkle) | `#7F8CE0` | `#5C66C9` |

This order passes a colour-vision-deficiency (CVD) validator for adjacent pairs in both modes:
- worst adjacent CVD ΔE is about 9.6;
- the normal-vision ΔE floor is at least 18.8;
- every slot is at least 3:1 against `surface-1`.

A series coloured by status (for example up and down counts) uses `statusColorVar(status)`, not a chart slot.

### 3.5 Contrast guarantees

`src/__tests__/contrast.test.ts` parses `index.css` and runs as part of `npm test`. It fails if any of these pairs drops below its minimum in **either** theme:

| Pair | Minimum |
|---|---|
| `fg`, `fg-muted`, `fg-subtle` on `canvas`, `surface-1` and `surface-2` | 4.5:1 |
| Every status colour as text on those surfaces | 4.5:1 |
| `primary` on surfaces | 4.5:1 |
| `on-primary` on `primary`, and `on-status` on every status fill (solid danger button, solid pills) | 4.5:1 |
| Status text on its own 12% chip wash | 4.5:1 |
| `focus` ring and chart slots on `surface-1` | 3:1 |

Current values:

| Pair | Dark | Light |
|---|---|---|
| `fg` on `surface-1` | 15.02 | 17.65 |
| `fg-muted` on `surface-1` | 7.93 | 7.55 |
| `fg-subtle` on `surface-1` | 5.08 | 5.40 |
| `fg-subtle` on `surface-2` | 4.65 | 4.78 |
| `primary` on `surface-1` | 13.37 | 8.56 |
| `on-status` on `status-down` (danger button) | 6.25 | 5.83 |
| `status-down` on `surface-1` | 5.83 | 5.83 |
| `status-up` on its 12% chip | 7.67 | 4.56 |
| `status-down` on its 12% chip | 5.04 | 4.80 |
| chart slots on `surface-1` (minimum) | 4.82 | 4.31 |

For comparison, v4 had axis and caption text at 2.73:1, error chips at about 3.5:1 and the danger button at 3.48:1.

If you change a token and the test fails, tune the value. Do not loosen the test.

---

## 4. Typography

- **Families.** Instrument Sans (`font-sans`) is used for all UI text and headings. JetBrains Mono (`font-mono`) is for data: IPs, MACs, ports, timestamps, IDs and log lines. There is no third family.
- **Scale.** 12 / 13 / 14 / 16 / 20 / 24 / 32 / 48, which maps to `text-xs`, `text-ui`, `text-sm`, `text-base`, `text-xl`, `text-2xl`, `text-3xl` and `text-5xl`. **12px is the floor.** Do not use `text-[10px]`, `[9px]` or `[8px]`. The same floor applies in SVG, recharts and canvas.
- **Case.** Use sentence case for headings, captions, labels, buttons and tabs ("Total devices", not "TOTAL DEVICES" or "Total Devices"). Use `uppercase` only when the content *is* a code, such as protocols (TCP, HTTP) or log levels. Never transform hostnames or device names.
- **Numbers.** `body` sets `tabular-nums`, so columns, tiles and counters do not jitter.

---

## 5. Scales

| Scale | Values |
|---|---|
| Spacing | 4px base (Tailwind default). Card padding `p-card` is 20px, or 12px in compact density |
| Radius | `rounded-sm` 4px (chips), `rounded-md` 8px (controls and insets), `rounded-lg` 12px (cards and dialogs), `rounded-full` (pills) |
| Elevation | `shadow-e1` (selected control), `shadow-e2` (menus, tooltips, toasts), `shadow-e3` (dialogs, drawers). Cards are flat, with a border and no shadow |
| z-index | `z-sticky` 10 (mobile bottom nav, sticky headers), `z-header` 30, `z-drawer` 40 (sidebar, account menu, scrim), `z-modal` 50, `z-toast` 60, `z-tooltip` 70. Modal `<dialog>`s and toasts use the browser top layer, so they never fight the header |
| Motion | `--duration-fast` 120ms (hover, state), `--duration-normal` 200ms (entrances, accordions) |
| Density | `[data-density="compact"]` sets a 15px root size and reduces `--space-card` (`p-card`) and `--control-h` (`h-control`, `min-h-control`) |

---

## 6. Status system (`utils/status.ts`)

This file is the single source for anything that conveys state.

| Function | Use |
|---|---|
| `normalizeStatus(raw)` | Maps backend strings (`ok`, `degraded`, `offline`, `paused`…) onto the six statuses |
| `statusMeta(raw)` | Returns the `{ token, label, icon, shape, text, bg, border, chip }` classes. They are literal strings, so Tailwind can see them |
| `severityMeta(sev)` | critical → down treatment labelled "Critical"; warning; info |
| `lifecycleMeta(state)` | Alert and incident states: active/open → down, acknowledged/investigating → warning, monitoring → info, resolved/closed → up |
| `healthBand(score)` / `healthMeta(score)` / `healthBandStatusMeta(band)` | **The one health banding**, mirroring the backend's `scoreToLabel()`. See the band table below. Each band has its own colour and shape, so Critical and Risk no longer collide |
| `statusColorVar(raw)` | `var(--color-status-…)` for SVG and recharts |

Health bands:

| Score | Band | Presented as |
|---|---|---|
| < 40 | Critical | down |
| < 60 | Risk | warning |
| < 80 | Watch | info |
| otherwise | Healthy | up |

Components (`components/ui/Status.tsx`):

- `<StatusPill status>` is a tinted chip with shape and label. Pass a raw string, or a meta from the helpers above. Children override the text, as in `<StatusPill status="down">SLA breach</StatusPill>`.
- `<StatusDot status>` is the shape alone in the status colour. It announces its label unless `label=""` is passed, which you should do when a visible label sits next to it.
- `<StatusShapeIcon shape>` is the raw SVG shape, for legends.

The network topology canvas draws the same shapes as a badge on every node, in token colours that it reads at draw time.

---

## 7. Primitives (`components/ui`)

| Component | Contract |
|---|---|
| `Button` | Variants: `primary`, `secondary`, `ghost`, `danger` (solid, ≥4.5:1), `danger-outline`, `primary-outline`. Sizes `sm` and `md` (`md` follows density). Leading icon via `icon` |
| `IconButton` | Icon-only. **`label` is required** and is rendered as `aria-label` and the tooltip. Variants `ghost`, `secondary`, `danger`. Supports `pressed` for toggles |
| `Dialog` | Built on native `<dialog>.showModal()`. Gets the top layer, an inert background and focus containment from the platform. Labelled by its title and described by `description`. Esc, the close button and a backdrop click close it. Initial focus goes to the first field or to `initialFocusRef`. Focus returns to the opener. Has a `footer` slot for actions and `placement="drawer"` for side panels. **All modals use it.** Do not hand-roll overlays, focus traps or Esc handlers |
| `ConfirmDialog` | An `alertdialog` on `Dialog`. **Focus starts on Cancel**, and the confirm button is solid (danger or primary) |
| `Tabs` / `TabPanel` | WAI-ARIA tabs: `tablist`/`tab`/`tabpanel`, `aria-selected`/`aria-controls`, roving tabindex, ←/→/Home/End with automatic activation. Pair with `useTabParam(ids)` so the tab lives in `?tab=`. Back/Forward, reload and links work; changes replace the history entry |
| `Card` | `bg-surface-1 border-line rounded-lg`. Use `hover` **only** when the whole card is clickable |
| `StatCard` | A static KPI tile: sentence-case label and a tabular value. It has no hover lift because it is not interactive; wrap it in a link if it should navigate |
| `StatusPill`, `StatusDot` | See §6 |
| `EmptyState` | Icon, title and description, plus an optional action. Uses `role="status"` |
| `RingGauge` | A 0–100 ring. `tone="health"` uses `healthBand` and shows the band. `tone="neutral"` is for other percentages |
| `ThemeSwitcher` / `DensitySwitcher` | Native radio groups styled as segmented controls |
| Toasts (`useToast`) | Rendered in a manual popover, so they stay above an open modal. They use the status shapes |

### Charts

Charts use `chartConfig.tsx`:
- `AXIS_TICK_STYLE`: 12px ticks in `fg-subtle`.
- `GRID_COLOR`.
- `TOOLTIP_STYLE`, `TOOLTIP_LABEL_STYLE` and `TOOLTIP_ITEM_STYLE`.
- `LEGEND_STYLE` with `legendFormatter`.
- `SERIES_COLORS`, `DEVICE_COLORS` and `PROTOCOL_COLORS`, all drawn from the chart slots.

All of these are `var()` references, so recharts follows the theme without re-rendering. Spread `useChartAnimation()` onto every series so animation is off under reduced motion.

---

## 8. Layout and navigation

- **Header.** The fixed header (`z-header`) holds:
  - the navigation toggle, an `IconButton` with `aria-expanded` and `aria-controls`;
  - the page title;
  - search, a `role="search"` form with a labelled input;
  - a live/paused indicator using `StatusDot`;
  - the alerts link, whose accessible name includes the active count;
  - the account menu, which holds the theme switch, Settings and Sign out. It closes on Esc (returning focus to its button) and on an outside press.
- **Sidebar.** Its group toggles expose `aria-expanded` and `aria-controls`. Collapsed groups are `inert`. The active link has `aria-current="page"`, set by `NavLink`, plus an accent bar.
- **Mobile.** On narrow screens the sidebar becomes a drawer with a scrim. Esc or a tap on the scrim closes it. The bottom nav sits on `z-sticky`, below drawers and dialogs.
- **Main content.** `<main id="main-content" tabIndex={-1}>` receives focus on every route change after the first, so screen readers announce the new page. A skip link targets it.

---

## 9. Motion

| Allowed | Not allowed |
|---|---|
| Entrance `rise-in` (120–200ms) | Infinite `glow-pulse`, live-dot pulses, rotating particle rings |
| Bars growing from the left (200ms) | Drifting background particles |
| Gauge fill on first paint (600ms) | Hover lifts on tiles that aren't interactive |
| Skeleton shimmer, only while loading | Decorative motion that repeats |
| `.alert-new-pulse`: three 1s pulses for a critical alert that **arrived after page load** | Pulsing every critical row forever |

`@media (prefers-reduced-motion: reduce)` collapses CSS animation and transitions. JavaScript-driven motion checks `useReducedMotion()`:
- `AnimatedCounter` jumps straight to the value.
- Recharts series receive `isAnimationActive={false}` via `useChartAnimation()`.
- Topology particles stop.

---

## 10. Accessibility checklist (per PR)

- [ ] No hex or rgb literals and no default palette classes in TSX. Colours come from tokens.
- [ ] Status is shown with `StatusPill` or `StatusDot` (shape plus colour, with text where it fits).
- [ ] Text is 12px or larger, captions are sentence case, and data uses `font-mono` or tabular numbers.
- [ ] Every icon-only control is an `IconButton` with a specific `label` (for example "Delete sw-a", not "Delete"). Decorative icons carry `aria-hidden`.
- [ ] Every input has a visible label or an `aria-label`. Errors use `aria-invalid` and text, not colour alone.
- [ ] Modals use `Dialog` and confirmations use `ConfirmDialog`.
- [ ] Tabs use `Tabs` with `useTabParam`.
- [ ] Charts use the `chartConfig` tokens and `useChartAnimation()`, and offer a `ChartDataTable` for screen readers or print where it helps.
- [ ] You checked the page in **both** themes.

---

## 11. Phase 2 (not in this release)

These items come from the design review and are deliberately deferred:
- the navigation restructure to about 12 items in 4 groups;
- NOC/wall mode with a 1.5× root size;
- a ⌘K command palette;
- a shared DataTable with sort, filter, virtualisation and bulk actions;
- `/devices/:id` detail routes;
- customisable dashboards.
