import { describe, expect, it } from 'vitest';
import type { DashboardLayout } from '../../../api/types';
import { LAYOUT_LIMITS, OVERVIEW_LAYOUT, addWidget, cloneLayout, layoutProblem, moveWidget, normalizeLayout, removeWidget, updateWidget } from '../layout';
import { WIDGETS } from '../widgets/catalog';
import { KNOWN_WIDGET_TYPES } from '../widgets/registry';

const base: DashboardLayout = {
  version: 1,
  widgets: [
    { id: 'a', type: 'stat.total-devices', w: 3, h: 1 },
    { id: 'b', type: 'stat.online', w: 3, h: 1 },
    { id: 'c', type: 'alerts.active', w: 6, h: 2 },
  ],
};
const ids = (l: DashboardLayout) => l.widgets.map((w) => w.id);

describe('dashboard layout helpers', () => {
  it('adds with a fresh unique id and removes by id', () => {
    const added = addWidget(base, { type: 'device.list', w: 4, h: 2 });
    expect(added.widgets).toHaveLength(4);
    const id = added.widgets[3]!.id;
    expect(id).toMatch(/^[A-Za-z0-9_-]{1,64}$/);
    expect(ids(base)).not.toContain(id);
    expect(ids(removeWidget(added, id))).toEqual(['a', 'b', 'c']);
    expect(base.widgets).toHaveLength(3); // inputs are never mutated
  });

  it('moves widgets, clamping at both ends', () => {
    expect(ids(moveWidget(base, 'c', 0))).toEqual(['c', 'a', 'b']);
    expect(ids(moveWidget(base, 'a', 1))).toEqual(['b', 'a', 'c']);
    expect(ids(moveWidget(base, 'a', 99))).toEqual(['b', 'c', 'a']);
    expect(ids(moveWidget(base, 'b', -5))).toEqual(['b', 'a', 'c']);
    expect(moveWidget(base, 'missing', 0)).toBe(base);
  });

  it('updates size and config of one widget', () => {
    const next = updateWidget(base, 'c', { w: 12, config: { title: 'Open' } });
    expect(next.widgets[2]).toEqual({ id: 'c', type: 'alerts.active', w: 12, h: 2, config: { title: 'Open' } });
    expect(next.widgets[0]).toBe(base.widgets[0]);
  });

  it('clones with new ids and deep-copied config', () => {
    const src = updateWidget(base, 'a', { config: { deviceIds: [1, 2] } });
    const copy = cloneLayout(src);
    expect(copy.widgets.map((w) => w.type)).toEqual(src.widgets.map((w) => w.type));
    expect(ids(copy)).not.toEqual(ids(src));
    (copy.widgets[0]!.config!.deviceIds as number[]).push(3);
    expect(src.widgets[0]!.config!.deviceIds).toEqual([1, 2]);
  });

  it('reports what the server would reject', () => {
    expect(layoutProblem(base)).toBeNull();
    expect(layoutProblem(updateWidget(base, 'a', { w: 13 }))).toMatch(/wider/);
    expect(layoutProblem(updateWidget(base, 'a', { h: 5 }))).toMatch(/height/);
    expect(layoutProblem(updateWidget(base, 'a', { config: { blob: 'x'.repeat(LAYOUT_LIMITS.maxConfigBytes) } }))).toMatch(/settings/);
    let many = base;
    while (many.widgets.length <= LAYOUT_LIMITS.maxWidgets) many = addWidget(many, { type: 'stat.online', w: 3, h: 1 });
    expect(layoutProblem(many)).toMatch(/at most 48/);
  });

  it('normalizes malformed server layouts to something renderable', () => {
    expect(normalizeLayout(null)).toEqual({ version: 1, widgets: [] });
    expect(normalizeLayout({ version: 2, widgets: [] })).toEqual({ version: 1, widgets: [] });
    expect(normalizeLayout({ version: 1, widgets: [{ id: 'x', type: 't', w: 1, h: 1 }, { id: 1 }, null] }).widgets).toEqual([{ id: 'x', type: 't', w: 1, h: 1 }]);
  });

  it('builds the overview only from catalog widgets that have components', () => {
    const catalog = new Set(WIDGETS.map((w) => w.type));
    for (const w of OVERVIEW_LAYOUT.widgets) {
      expect(catalog.has(w.type)).toBe(true);
      expect(KNOWN_WIDGET_TYPES).toContain(w.type);
    }
    expect(new Set(WIDGETS.map((w) => w.type))).toEqual(new Set(KNOWN_WIDGET_TYPES));
    expect(layoutProblem(OVERVIEW_LAYOUT)).toBeNull();
  });
});
