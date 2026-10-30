import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { searchEntities, SEARCH_MIN_CHARS } from '../api/search';
import { getAlerts, ALERT_LIST_LIMIT } from '../api/alerts';
import { queryKeys } from '../api/queryKeys';
import type { Alert } from '../api/types';
import Dialog from './ui/Dialog';
import { StatusDot } from './ui/Status';
import { useDebouncedValue } from '../hooks/useDebouncedValue';
import { useAlertActions } from '../hooks/useAlertActions';
import { hasPermission, itemTarget, visibleGroups, visibleSections } from '../navigation';
import { useSessionUser } from '../store/session';
import { resolveTheme, setThemePreference } from '../theme/theme';
import { severityMeta } from '../utils/status';

export interface PaletteItem {
  id: string;
  group: 'Recent' | 'Pages' | 'Actions' | 'Devices' | 'Locations' | 'Alerts' | 'Search';
  label: string;
  hint?: string;
  icon: string;
  /** Status shape shown instead of the icon (devices, alerts). */
  status?: string;
  keywords?: string;
  to?: string;
  action?: 'toggle-theme' | 'ack-critical' | 'wall';
}

const RECENT_MAX = 5;
const recentKey = (userId: number | undefined) => `nm-palette-recent:${userId ?? 'anon'}`;

function loadRecent(userId: number | undefined): PaletteItem[] {
  try {
    const parsed: unknown = JSON.parse(window.localStorage.getItem(recentKey(userId)) ?? '[]');
    return Array.isArray(parsed) ? (parsed as PaletteItem[]).filter((i) => i && typeof i.label === 'string').slice(0, RECENT_MAX) : [];
  } catch {
    return [];
  }
}

function pushRecent(userId: number | undefined, item: PaletteItem) {
  const entry: PaletteItem = { ...item, group: 'Recent', id: `recent-${item.id}` };
  const next = [entry, ...loadRecent(userId).filter((r) => r.id !== entry.id)].slice(0, RECENT_MAX);
  try {
    window.localStorage.setItem(recentKey(userId), JSON.stringify(next));
  } catch {
    // Storage blocked: no recents, nothing else breaks.
  }
}

function matches(item: PaletteItem, needle: string): boolean {
  if (!needle) return true;
  const hay = `${item.label} ${item.hint ?? ''} ${item.keywords ?? ''}`.toLowerCase();
  return needle.split(/\s+/).every((word) => hay.includes(word));
}

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
  initialQuery?: string;
}

/**
 * ⌘K / Ctrl+K: jump to pages and settings sections, run actions, and find
 * devices, locations and active alerts (GET /search). WAI-ARIA combobox
 * with a grouped listbox; ↑/↓/Home/End move, Enter runs, Esc closes.
 */
export default function CommandPalette({ open, onClose, initialQuery = '' }: CommandPaletteProps) {
  const navigate = useNavigate();
  const user = useSessionUser();
  const queryClient = useQueryClient();
  const { bulkAct } = useAlertActions();
  const [query, setQuery] = useState(initialQuery);
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const [recent] = useState(() => loadRecent(user?.id));
  const needle = query.trim().toLowerCase();
  const debounced = useDebouncedValue(query.trim(), 200);

  const search = useQuery({
    queryKey: queryKeys.search.query(debounced),
    queryFn: ({ signal }) => searchEntities(debounced, 8, { signal }),
    enabled: open && debounced.length >= SEARCH_MIN_CHARS && hasPermission(user, 'devices.read'),
    staleTime: 15_000,
    retry: false,
  });

  const staticItems = useMemo<PaletteItem[]>(() => {
    const pages: PaletteItem[] = [];
    for (const group of visibleGroups(user)) {
      for (const item of group.items) {
        const sections = visibleSections(item.sections, user);
        pages.push({ id: `page-${item.id}`, group: 'Pages', label: item.label, hint: group.label, icon: item.icon, to: itemTarget(item, user), keywords: item.keywords });
        for (const s of sections) {
          // The item entry already goes to its first section.
          if (s.to === itemTarget(item, user)) continue;
          pages.push({ id: `page-${item.id}-${s.id}`, group: 'Pages', label: `${item.label} › ${s.label}`, hint: group.label, icon: s.icon, to: s.to, keywords: s.keywords });
        }
      }
    }
    const actions: PaletteItem[] = [
      { id: 'act-theme', group: 'Actions', label: 'Toggle theme', hint: 'Dark / light', icon: 'contrast', action: 'toggle-theme', keywords: 'dark light mode appearance' },
      { id: 'act-wall', group: 'Actions', label: 'Open wall mode', hint: 'NOC display', icon: 'tv', action: 'wall', keywords: 'noc fullscreen display' },
    ];
    if (hasPermission(user, 'alerts.acknowledge')) {
      actions.unshift({ id: 'act-ack-critical', group: 'Actions', label: 'Acknowledge all critical alerts', icon: 'done_all', action: 'ack-critical', keywords: 'ack' });
    }
    return [...pages, ...actions];
  }, [user]);

  const items = useMemo<PaletteItem[]>(() => {
    const out: PaletteItem[] = [];
    if (!needle) out.push(...recent);
    out.push(...staticItems.filter((i) => matches(i, needle)).slice(0, needle ? 12 : staticItems.length));
    const results = debounced.length >= SEARCH_MIN_CHARS && debounced.toLowerCase() === needle ? search.data : undefined;
    if (results) {
      for (const d of results.devices) {
        out.push({ id: `dev-${d.id}`, group: 'Devices', label: d.name, hint: [d.ipAddress, d.locationName].filter(Boolean).join(' · '), icon: 'devices', status: d.status, to: `/devices/${d.id}` });
      }
      for (const l of results.locations) {
        out.push({ id: `loc-${l.id}`, group: 'Locations', label: l.name, hint: l.path || l.type, icon: 'apartment', to: `/devices?search=${encodeURIComponent(l.name)}` });
      }
      for (const a of results.alerts) {
        out.push({ id: `alert-${a.id}`, group: 'Alerts', label: a.message, hint: a.deviceName, icon: 'notifications', status: severityMeta(a.severity).status, to: `/alerts?q=${encodeURIComponent(a.deviceName || a.message)}` });
      }
    }
    if (needle.length >= SEARCH_MIN_CHARS) {
      out.push({ id: 'search-devices', group: 'Search', label: `Search devices for “${query.trim()}”`, icon: 'search', to: `/devices?search=${encodeURIComponent(query.trim())}` });
    }
    return out;
  }, [needle, recent, staticItems, search.data, debounced, query]);

  const index = Math.min(activeIndex, Math.max(0, items.length - 1));
  const active = items[index];

  useEffect(() => {
    if (active) document.getElementById(`palette-opt-${active.id}`)?.scrollIntoView?.({ block: 'nearest' });
  }, [active]);

  const run = async (item: PaletteItem) => {
    pushRecent(user?.id, item);
    onClose();
    if (item.to) {
      void navigate(item.to);
      return;
    }
    if (item.action === 'toggle-theme') setThemePreference(resolveTheme() === 'dark' ? 'light' : 'dark');
    else if (item.action === 'wall') void navigate('/wall');
    else if (item.action === 'ack-critical') {
      const res = await queryClient.fetchQuery({
        queryKey: queryKeys.alerts.list('active', ALERT_LIST_LIMIT),
        queryFn: ({ signal }) => getAlerts('active', ALERT_LIST_LIMIT, { signal }),
        staleTime: 5_000,
      });
      const ids = (res.data ?? ([] as Alert[])).filter((a) => a.severity === 'critical').map((a) => a.id);
      if (ids.length) await bulkAct(ids, 'acknowledge').catch(() => undefined);
    }
  };

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'ArrowDown') setActiveIndex((index + 1) % Math.max(1, items.length));
    else if (event.key === 'ArrowUp') setActiveIndex((index - 1 + items.length) % Math.max(1, items.length));
    else if (event.key === 'Home' && event.ctrlKey) setActiveIndex(0);
    else if (event.key === 'End' && event.ctrlKey) setActiveIndex(items.length - 1);
    else if (event.key === 'Enter' && active) void run(active);
    else return;
    event.preventDefault();
  };

  const groups = items.reduce<Array<{ name: PaletteItem['group']; items: Array<{ item: PaletteItem; i: number }> }>>((acc, item, i) => {
    const last = acc[acc.length - 1];
    if (last?.name === item.group) last.items.push({ item, i });
    else acc.push({ name: item.group, items: [{ item, i }] });
    return acc;
  }, []);

  const searching = search.isFetching && debounced.length >= SEARCH_MIN_CHARS;
  return (
    <Dialog open={open} onClose={onClose} title="Command palette" hideHeader placement="top" size="lg" initialFocusRef={inputRef} bodyClassName="p-0">
      <div className="flex items-center gap-2 border-b border-line px-4">
        <span className="material-symbols-outlined text-[20px] text-fg-subtle" aria-hidden="true">search</span>
        <input
          ref={inputRef}
          role="combobox"
          aria-expanded="true"
          aria-controls="palette-listbox"
          aria-activedescendant={active ? `palette-opt-${active.id}` : undefined}
          aria-autocomplete="list"
          aria-label="Search pages, actions, devices, locations and alerts"
          value={query}
          onChange={(e) => { setQuery(e.target.value); setActiveIndex(0); }}
          onKeyDown={onKeyDown}
          placeholder="Jump to a page, run an action, or find a device…"
          className="h-14 min-w-0 flex-1 bg-transparent text-base text-fg placeholder:text-fg-subtle outline-none"
        />
        {searching && <span role="status" className="text-xs text-fg-muted">Searching…</span>}
        <kbd className="hidden rounded-sm border border-line-strong px-1.5 py-0.5 font-mono text-xs text-fg-muted sm:inline">Esc</kbd>
      </div>
      <div ref={listRef} id="palette-listbox" role="listbox" aria-label="Results" className="max-h-[56dvh] overflow-y-auto p-2">
        {items.length === 0 && <p className="px-3 py-6 text-center text-sm text-fg-muted">No matches.</p>}
        {groups.map((g) => (
          <div key={g.name} role="group" aria-labelledby={`palette-group-${g.name}`} className="mb-1">
            <div id={`palette-group-${g.name}`} className="px-3 pb-1 pt-2 text-xs font-semibold text-fg-muted">{g.name}</div>
            {g.items.map(({ item, i }) => (
              <div
                key={item.id}
                id={`palette-opt-${item.id}`}
                role="option"
                aria-selected={i === index}
                onMouseMove={() => { if (i !== index) setActiveIndex(i); }}
                onClick={() => void run(item)}
                className={`flex h-10 cursor-pointer items-center gap-3 rounded-md px-3 text-sm ${i === index ? 'bg-surface-3 text-fg' : 'text-fg-muted'}`}
              >
                {item.status ? (
                  <span className="flex w-5 justify-center"><StatusDot status={item.status} label="" /></span>
                ) : (
                  <span className="material-symbols-outlined w-5 text-[18px]" aria-hidden="true">{item.icon}</span>
                )}
                <span className="min-w-0 flex-1 truncate text-fg">{item.label}</span>
                {item.hint && <span className="truncate text-xs text-fg-muted">{item.hint}</span>}
              </div>
            ))}
          </div>
        ))}
        {search.isError && debounced.length >= SEARCH_MIN_CHARS && (
          <p className="px-3 py-2 text-xs text-fg-muted">Entity search isn't available on this server; pages and actions still work.</p>
        )}
      </div>
      <div className="flex flex-wrap gap-x-4 gap-y-1 border-t border-line px-4 py-2 text-xs text-fg-muted">
        <span><kbd className="font-mono">↑↓</kbd> move</span>
        <span><kbd className="font-mono">Enter</kbd> open</span>
        <span><kbd className="font-mono">Esc</kbd> close</span>
        <span className="ml-auto">⌘K / Ctrl+K</span>
      </div>
    </Dialog>
  );
}
