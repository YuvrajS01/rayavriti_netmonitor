import { useId, useState } from 'react';
import type { DashboardLayout } from '../../api/types';
import Dialog from '../ui/Dialog';
import { LAYOUT_LIMITS } from './layout';
import { WIDGET_CATEGORIES, WIDGETS, type WidgetMeta } from './widgets/catalog';

interface Props {
  layout: DashboardLayout;
  onClose: () => void;
  onAdd: (meta: WidgetMeta) => void;
}

/** Pick a widget to add. Single-instance widgets already present are disabled. */
export default function WidgetCatalogDialog({ layout, onClose, onAdd }: Props) {
  const [query, setQuery] = useState('');
  const searchId = useId();
  const baseId = useId();
  const present = new Set(layout.widgets.map((w) => w.type));
  const full = layout.widgets.length >= LAYOUT_LIMITS.maxWidgets;
  const q = query.trim().toLowerCase();
  const matches = WIDGETS.filter((w) => !q || w.label.toLowerCase().includes(q) || w.description.toLowerCase().includes(q));

  return (
    <Dialog open onClose={onClose} title="Add a widget" description={full ? `This dashboard has the maximum of ${LAYOUT_LIMITS.maxWidgets} widgets.` : 'Widgets are added at the end; drag or move them afterwards.'} size="lg">
      <label htmlFor={searchId} className="sr-only">Search widgets</label>
      <input
        id={searchId}
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search widgets"
        className="mb-4 w-full rounded-md border border-line bg-inset px-3 py-2 text-sm text-fg outline-none focus:border-primary"
      />
      {!matches.length && <p className="py-6 text-center text-sm text-fg-muted">No widgets match “{query}”.</p>}
      <div className="space-y-5">
        {WIDGET_CATEGORIES.map((category) => {
          const items = matches.filter((w) => w.category === category);
          if (!items.length) return null;
          return (
            <section key={category} aria-label={category}>
              <h3 className="mb-2 text-xs font-semibold text-fg-muted">{category}</h3>
              <ul className="grid gap-2 sm:grid-cols-2">
                {items.map((w) => {
                  const taken = w.single && present.has(w.type);
                  const ids = `${baseId}-${w.type}`;
                  return (
                    <li key={w.type}>
                      <button
                        type="button"
                        disabled={taken || full}
                        onClick={() => onAdd(w)}
                        aria-labelledby={`${ids}-name`}
                        aria-describedby={`${ids}-desc`}
                        className="flex h-full w-full items-start gap-3 rounded-md border border-line bg-surface-2 p-3 text-left transition-colors hover:border-line-strong hover:bg-surface-3 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        <span className="material-symbols-outlined text-[22px] text-primary" aria-hidden="true">{w.icon}</span>
                        <span className="min-w-0">
                          <span id={`${ids}-name`} className="block text-sm font-semibold text-fg">{w.label}</span>
                          <span id={`${ids}-desc`} className="block text-xs text-fg-muted">{taken ? 'Already on this dashboard.' : w.description}</span>
                        </span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            </section>
          );
        })}
      </div>
    </Dialog>
  );
}
