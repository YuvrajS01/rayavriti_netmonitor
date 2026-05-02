# UI/UX Specification Document
## Simplified Network Monitoring System (Rayavriti Theme)

**Version:** 2.0  
**Date:** February 28, 2026

---

## Design System

### Color Palette

#### Brand Colors (Primary)
```css
--primary: #d9fd3a;         /* Main brand color (Neon Yellowish-Green) */
--primary-hover: #C3E800;   /* Hover state */
--primary-active: #B2D700;  /* Active state */
```

#### Background Colors
```css
--bg-default: #11110B;      /* Main application background */
--bg-surface: #161612;      /* Cards and secondary sections */
--bg-elevated: #1C1C16;     /* Modals, dropdowns, popovers */
```

#### Text Colors
```css
--text-primary: #d9fd3a;    /* High-contrast headings and active elements */
--text-secondary: #B8C94D;  /* Secondary emphasis */
--text-muted: #7A7A66;      /* Disabled or less important text */
--text-body: #C5C5B0;       /* Standard body text format */
--text-inverse: #11110B;    /* Text on primary-colored background elements */
```

#### Border Colors
```css
--border-default: #2A2A22;  /* Standard dividers */
--border-active: #d9fd3a;   /* Focused elements */
--border-subtle: #1F1F18;   /* Light dividers between nested items */
```

#### Status Colors
```css
--success: #d9fd3a;         /* Operational/OK (Using brand primary) */
--warning: #F59E0B;         /* Warning state */
--error: #EF4444;           /* Error/Critical */
--info: #B8C94D;            /* Informational */
```

### Typography

**Font Families:**
- **Heading**: 'League Spartan', system-ui, sans-serif
- **Body**: 'Space Grotesk', system-ui, sans-serif

**Font Sizes:**
```css
--text-xs: 0.75rem;     /* 12px */
--text-sm: 0.875rem;    /* 14px */
--text-base: 1rem;      /* 16px */
--text-lg: 1.125rem;    /* 18px */
--text-xl: 1.25rem;     /* 20px */
--text-2xl: 1.5rem;     /* 24px */
--text-3xl: 1.75rem;    /* 28px */
--text-4xl: 2rem;       /* 32px (h4) */
--text-5xl: 2.25rem;    /* 36px (h3) */
--text-6xl: 3rem;       /* 48px (h2) */
--text-7xl: 4rem;       /* 64px (h1) */
```

### Spacing System
```css
--space-xs: 4px;
--space-sm: 8px;
--space-md: 16px;
--space-lg: 24px;
--space-xl: 32px;
--space-2xl: 48px;
--space-3xl: 64px;
```

### Layout
```css
--max-width: 1280px;
--gutter: 32px;
```

### Border Radius
```css
--radius-sm: 6px;
--radius-md: 10px;
--radius-lg: 16px;
--radius-xl: 24px;
--radius-full: 9999px;
```

### Shadows & Glows

Rayavriti emphasizes a dark, futuristic aesthetic. Soft layering with high-contrast neon glows creates a distinct technological feel.
```css
--shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.3);
--shadow-soft: 0 4px 16px rgba(0, 0, 0, 0.25), 0 8px 32px rgba(0, 0, 0, 0.15);
--shadow-hard: 0 8px 24px rgba(0, 0, 0, 0.35), 0 16px 48px rgba(0, 0, 0, 0.25);
--shadow-glow: 0 0 20px rgba(217, 253, 58, 0.25), 0 0 40px rgba(217, 253, 58, 0.1);
```

### Motion & Animation
```css
--duration-fast: 120ms;
--duration-normal: 220ms;
--duration-slow: 400ms;
--easing-standard: cubic-bezier(0.4, 0, 0.2, 1);
--easing-emphasized: cubic-bezier(0.2, 0, 0, 1);
```

---

## Key Screens

### 1. Login Screen

**Layout:**
- Centered, minimal interface on a dark `--bg-default` background
- Glow effect (`--shadow-glow`) surrounding the main login card (`--bg-surface`)
- Monochromatic text and neon-accented borders
- Primary action button filled with `--primary` and text colored `--text-inverse`

**Visual Elements:**
- Futuristic branding with 'League Spartan' uppercase typography
- Floating particles or subtle animated background graphics utilizing `--primary` low-opacity accents
- Smooth transitions on input focus

---

### 2. Main Dashboard

**Layout:**
```
┌─────────────────────────────────────────────────┐
│ [Logo] Dashboard        [Search] [Alerts] [User]│
├─────────────────────────────────────────────────┤
│ Sidebar │ Content Area                          │
│         │                                       │
│ • Overview│ ┌─────┐ ┌─────┐ ┌─────┐           │
│ • Devices │ │Widget│ │Widget│ │Widget│          │
│ • Sensors │ └─────┘ └─────┘ └─────┘           │
│ • Alerts  │                                     │
│ • Reports │ ┌───────────────────────┐          │
│ • Settings│ │   Chart Widget        │          │
│           │ └───────────────────────┘          │
└───────────┴─────────────────────────────────────┘
```

**Components:**
- **Navigation**: Uses `--border-default` for subtle partitioning. Sidebar features `--bg-surface`. Active items receive `--primary` text and a left-aligned `--border-active`.
- **Widgets**: Housed in `--bg-elevated` panels with `--radius-lg` and slight `--shadow-soft`. Hovering over widgets introduces `--shadow-glow` and slight negative Y translation (`transform: translateY(-4px)`).
- **Typography Format**: Headers and data-driven values use `League Spartan` while tabular text logic utilizes `Space Grotesk`.

---

### 3. Device List View

**Layout:**
```
┌─────────────────────────────────────────────────┐
│ Devices                         [+ Add Device]  │
├─────────────────────────────────────────────────┤
│ [Search] [Filter▾] [Group▾] [Sort▾]   [⚙]  [⊞] │
├─────────────────────────────────────────────────┤
│ ● Server-01      192.168.1.100    12 sensors ► │
│ ● Router-01      192.168.1.1       8 sensors ► │
│ ● Switch-01      192.168.1.2       6 sensors ► │
│ ◐ Server-02      192.168.1.101     PAUSED    ► │
│ ○ Server-03      192.168.1.102     DOWN      ► │
└─────────────────────────────────────────────────┘
```

**Features:**
- Neon indicators (Green `#d9fd3a` glow = up, Red `#EF4444` = down, Yellow `#F59E0B` = warning)
- Search inputs have a focal state that pulses softly.
- Active tables have a subtle `--border-subtle` line beneath rows.

---

### 4. Device Detail View

**Layout:**
```
┌─────────────────────────────────────────────────┐
│ ← Back to Devices                               │
├─────────────────────────────────────────────────┤
│ ● Server-01                          [Edit] [⋮] │
│ 192.168.1.100 • Ubuntu 22.04 • Datacenter-A    │
├─────────────────────────────────────────────────┤
│ [Overview] [Sensors] [Metrics] [Alerts] [Logs] │
├─────────────────────────────────────────────────┤
│                                                 │
│ Quick Metrics                                   │
│ ┌────────────┬────────────┬────────────┐       │
│ │ CPU: 45%   │ Memory: 62%│ Disk: 38% │       │
│ └────────────┴────────────┴────────────┘       │
│                                                 │
│ Active Sensors                                  │
│ ┌─────────────────────────────────────┐        │
│ │ ✓ Ping         15ms      Last: 1m   │        │
│ │ ✓ CPU Usage    45%       Last: 30s  │        │
│ │ ⚠ Memory       85%       Last: 30s  │        │
│ └─────────────────────────────────────┘        │
│                                                 │
│ Performance Chart (24h)                         │
│ [Line chart showing neon data paths]            │
│                                                 │
└─────────────────────────────────────────────────┘
```

---

### 5. Alert Management & Status Feedback

**Features:**
- Action buttons have an expanding underline effect (`--primary` trace logic) on hover.
- Badges strictly distinguish priority. Critical alerts utilize harsh, un-rounded boxes to create tension, while operational badges are softly rounded and glow.
- Empty states showcase wireframe SVGs of networked nodes using `--text-muted`.

---

## Components Library

### 1. Interactive Buttons

**Primary Button:**
- Background: `--primary`
- Text: `--text-inverse` (`League Spartan`, Uppercase)
- Hover State: Pulsating `--shadow-glow`, transform to floating `translateY(-2px)`. Inner ripple effect or radiating geometric paths.
- Animation profile:
```css
.btn-primary:hover {
  transform: translateY(-2px);
  animation: glowPulse 2s ease-in-out infinite;
}
```

**Secondary Button:**
- Background: Transparent
- Text: `--text-primary`
- Hover State: `::after` underline expands from `scaleX(0)` to `scaleX(1)`.

### 2. Status Badge
```
◉ Online (Neon #d9fd3a)
⬤ Warning (Bright #F59E0B)
● Offline (Dim #7A7A66)
```

### 3. Metric Card
```
┌─────────────────┐
│ CPU Usage       │
│ 45.2%           │
│ ▁▂▃▄▅▆▇█        │
│ Last check: 30s │
└─────────────────┘
```
Cards elevate visually using `box-shadow` overrides and `--border-active` on hover.

---

## Responsive Design

### Breakpoints
```css
--mobile: 320px - 768px
--tablet: 769px - 1024px
--desktop: 1025px+
```

### Mobile Adaptations
- Sidebars transition into off-canvas or bottom-sheet navigation elements.
- Interactions convert hover-states to press-and-hold tactile responses.
- Modals take up full screen height/width.

---

## Accessibility

### WCAG 2.1 AA Compliance
- **Contrast Ratio**: High-contrast brand (`#d9fd3a` on `#11110B`) guarantees minimum 4.5:1 visibility.
- **Reduced Motion**: Respects `prefers-reduced-motion: reduce` by zeroing out translation limits, transition delays (`0.01ms`), and removing `.btn-primary` pulse.

---

## Animations & Transitions

### Micro-interactions
```css
.card {
  transition: all var(--duration-slow) var(--easing-emphasized);
  transform-style: preserve-3d;
  perspective: 1000px;
}

.card:hover {
  transform: translateY(-8px);
  border-color: var(--border-active);
  box-shadow: var(--shadow-hard), 0 0 30px rgba(217, 253, 58, 0.15);
}

.icon {
  transition: all var(--duration-normal) var(--easing-standard);
}

.card:hover .icon {
  transform: scale(1.1) rotate(5deg);
  filter: drop-shadow(0 0 8px rgba(217, 253, 58, 0.5));
}
```

### Input Field Flow
```css
.form-input {
  transition: all var(--duration-normal) var(--easing-standard);
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  color: var(--text-body);
}

.form-input:focus {
  transform: translateY(-2px);
  border-color: var(--border-active);
  box-shadow: 0 0 0 3px rgba(217, 253, 58, 0.2), 0 4px 12px rgba(0, 0, 0, 0.2);
}

.form-group:focus-within .form-label {
  color: var(--primary);
}
```

---

## Theme Structure

### Dark Mode Native
The application natively adheres to the dark, futuristic design language outlined by the Rayavriti guidelines. There is no traditional "light mode," but themes can be toggled by density (e.g. standard vs high-density data visualizations). Background properties are permanently tethered to the `#11110B` base model to maintain aesthetic fidelity.

---

## Error & Loading States

### Loading Profiles
- Futuristic wireframe placeholders.
- Neon glow sweeps over skeletons.

### Empty Data Profile
```
    ┌───────────────┐
    │   [Icon]      │
    │ No Data Stream│
    │ [Sync Stream] │
    └───────────────┘
```
Text assumes `--text-muted`, and icons animate with a slow, low-intensity pulse.

---

## Best Practices

1. **Futuristic Consistency**: Adhere strictly to the `League Spartan` + `Space Grotesk` font mix. Headers must command attention.
2. **Neon Hierarchy**: Use `#d9fd3a` sparingly to guarantee that interactive and focal elements strike accurately.
3. **Immersive Depth**: Utilize `--shadow-soft` and `--shadow-glow` combinations, as flat interfaces break the Rayavriti architectural immersion.
4. **Motion**: Ensure all hover events feel responsive and layered, taking advantage of the custom cubic-bezier easings.
