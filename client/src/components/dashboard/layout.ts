import type { DashboardLayout, DashboardWidget } from '../../api/types';

/** Limits mirrored from the server (backend/internal/models/dashboard_layout.go). */
export const LAYOUT_LIMITS = {
  columns: 12,
  maxHeight: 4,
  maxWidgets: 48,
  maxConfigBytes: 4 << 10,
  maxLayoutBytes: 64 << 10,
  maxNameLength: 100,
} as const;

/** Width presets offered in the editor (columns of 12). */
export const WIDTH_PRESETS = [3, 4, 6, 8, 12] as const;
/** Height steps offered in the editor; a minimum height, rows grow to fit. */
export const HEIGHT_STEPS = [1, 2, 3, 4] as const;

export const WIDTH_LABEL: Record<number, string> = { 3: 'Quarter', 4: 'Third', 6: 'Half', 8: 'Two thirds', 12: 'Full' };
export const HEIGHT_LABEL: Record<number, string> = { 1: 'Compact', 2: 'Standard', 3: 'Tall', 4: 'Extra tall' };

export const emptyLayout = (): DashboardLayout => ({ version: 1, widgets: [] });

/** A short random widget id, unique within the layout. */
export function newWidgetId(layout: DashboardLayout): string {
  const taken = new Set(layout.widgets.map((w) => w.id));
  for (;;) {
    const id = `w-${Math.random().toString(36).slice(2, 8)}`;
    if (!taken.has(id)) return id;
  }
}

export function addWidget(layout: DashboardLayout, widget: Omit<DashboardWidget, 'id'>): DashboardLayout {
  return { ...layout, widgets: [...layout.widgets, { ...widget, id: newWidgetId(layout) }] };
}

export function removeWidget(layout: DashboardLayout, id: string): DashboardLayout {
  return { ...layout, widgets: layout.widgets.filter((w) => w.id !== id) };
}

export function updateWidget(layout: DashboardLayout, id: string, patch: Partial<Omit<DashboardWidget, 'id'>>): DashboardLayout {
  return { ...layout, widgets: layout.widgets.map((w) => (w.id === id ? { ...w, ...patch } : w)) };
}

/** Moves a widget to `to` (an index in the resulting order), clamped to the ends. */
export function moveWidget(layout: DashboardLayout, id: string, to: number): DashboardLayout {
  const from = layout.widgets.findIndex((w) => w.id === id);
  if (from < 0) return layout;
  const widgets = [...layout.widgets];
  const [moved] = widgets.splice(from, 1);
  const index = Math.max(0, Math.min(widgets.length, to));
  widgets.splice(index, 0, moved!);
  return { ...layout, widgets };
}

/** Copies a layout with fresh widget ids (duplicating a dashboard). */
export function cloneLayout(layout: DashboardLayout): DashboardLayout {
  const out = emptyLayout();
  for (const w of layout.widgets) out.widgets.push({ ...w, id: newWidgetId(out), config: w.config ? structuredClone(w.config) : undefined });
  return out;
}

const byteLength = (value: unknown) => new TextEncoder().encode(JSON.stringify(value)).length;

/**
 * The first problem the server would reject the layout for, or null. The
 * editor checks before saving so the user gets the message in context.
 */
export function layoutProblem(layout: DashboardLayout): string | null {
  if (layout.widgets.length > LAYOUT_LIMITS.maxWidgets) return `A dashboard can have at most ${LAYOUT_LIMITS.maxWidgets} widgets.`;
  for (const w of layout.widgets) {
    if (w.w < 1 || w.w > LAYOUT_LIMITS.columns) return 'A widget is wider than the grid.';
    if (w.h < 1 || w.h > LAYOUT_LIMITS.maxHeight) return 'A widget has an invalid height.';
    if (w.config && byteLength(w.config) > LAYOUT_LIMITS.maxConfigBytes) return 'A widget’s settings are too large.';
  }
  if (byteLength(layout) > LAYOUT_LIMITS.maxLayoutBytes) return 'This dashboard is too large to save.';
  return null;
}

/** Reads a layout from the server defensively: anything malformed becomes empty. */
export function normalizeLayout(raw: unknown): DashboardLayout {
  const value = raw as Partial<DashboardLayout> | null;
  if (!value || value.version !== 1 || !Array.isArray(value.widgets)) return emptyLayout();
  return {
    version: 1,
    widgets: value.widgets.filter(
      (w): w is DashboardWidget => !!w && typeof w.id === 'string' && typeof w.type === 'string' && Number.isInteger(w.w) && Number.isInteger(w.h),
    ),
  };
}

/**
 * The built-in Overview: today's monitoring overview as a layout. It is not
 * stored; duplicating it is how a user starts customising.
 */
export const OVERVIEW_LAYOUT: DashboardLayout = {
  version: 1,
  widgets: [
    { id: 'stat-total', type: 'stat.total-devices', w: 3, h: 1 },
    { id: 'stat-online', type: 'stat.online', w: 3, h: 1 },
    { id: 'stat-uptime', type: 'stat.uptime', w: 3, h: 1 },
    { id: 'stat-alerts', type: 'stat.active-alerts', w: 3, h: 1 },
    { id: 'health', type: 'health.score', w: 4, h: 2 },
    { id: 'insights', type: 'insights', w: 8, h: 2 },
    { id: 'response', type: 'response.slowest', w: 8, h: 2 },
    { id: 'distribution', type: 'status.distribution', w: 4, h: 2 },
    { id: 'heatmap', type: 'status.heatmap', w: 4, h: 2 },
    { id: 'topology', type: 'topology.mini', w: 8, h: 2 },
    { id: 'resources', type: 'resource.load', w: 6, h: 2 },
    { id: 'by-status', type: 'response.by-status', w: 6, h: 2 },
    { id: 'latest', type: 'metrics.latest', w: 6, h: 2 },
    { id: 'alerts', type: 'alerts.active', w: 6, h: 2 },
  ],
};
