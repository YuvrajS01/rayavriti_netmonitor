import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { getAlerts, getAlertCounts, ALERT_LIST_LIMIT } from '../api/alerts';
import { getDevices } from '../api/devices';
import { queryKeys } from '../api/queryKeys';
import type { Alert, AlertBulkAction, AlertCounts } from '../api/types';
import SectionHeader from '../components/ui/SectionHeader';
import ErrorState from '../components/ui/ErrorState';
import EmptyState from '../components/ui/EmptyState';
import Button from '../components/ui/Button';
import IconButton from '../components/ui/IconButton';
import Dialog from '../components/ui/Dialog';
import RelativeTime from '../components/ui/RelativeTime';
import { Tabs } from '../components/ui/Tabs';
import { StatusPill } from '../components/ui/Status';
import { DataTable } from '../components/ui/DataTable';
import { useDataTable, type RowId } from '../components/ui/useDataTable';
import type { Column, GroupOption } from '../components/ui/dataTableModel';
import AlertDrawer from '../components/AlertDrawer';
import { useTabParam, useTabsId } from '../hooks/useTabParam';
import { useAlertActions } from '../hooks/useAlertActions';
import { useResourceList } from '../hooks/useResources';
import { lifecycleMeta, severityMeta } from '../utils/status';
import { hasPermission } from '../navigation';
import { useSessionUser } from '../store/session';

const TAB_IDS = ['active', 'acknowledged', 'resolved'] as const;
type AlertTab = (typeof TAB_IDS)[number];

const EMPTY_COUNTS: AlertCounts = { active: 0, acknowledged: 0, resolved: 0 };
const EMPTY_ALERTS: Alert[] = [];
const SEVERITY_RANK: Record<string, number> = { critical: 0, warning: 1, info: 2 };

interface AlertRow extends Alert {
  location: string;
}

const SHORTCUTS: Array<[string, string]> = [
  ['j / ↓', 'Next alert'],
  ['k / ↑', 'Previous alert'],
  ['x / Space', 'Select or deselect'],
  ['a', 'Acknowledge (selection, or the current alert)'],
  ['r', 'Resolve (selection, or the current alert)'],
  ['n', 'Notes and assignee for the current alert'],
  ['Enter', 'Open the current alert'],
  ['Esc', 'Clear the selection'],
  ['?', 'Show these shortcuts'],
];

const GROUPS: GroupOption<AlertRow>[] = [
  { id: 'device', label: 'Device', key: (a) => a.deviceName || `Device ${a.deviceId}` },
  { id: 'location', label: 'Location', key: (a) => a.location || 'Unassigned' },
  { id: 'rule', label: 'Alert type', key: (a) => a.message.split(':')[0] ?? a.message },
];

function isTyping(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  return el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName);
}

export default function Alerts() {
  const user = useSessionUser();
  const [params, setParams] = useSearchParams();
  const [tab, setTab] = useTabParam<AlertTab>(TAB_IDS);
  const tabsId = useTabsId();
  const { act, bulkAct, pending } = useAlertActions();
  const canAck = hasPermission(user, 'alerts.acknowledge');
  const canResolve = hasPermission(user, 'alerts.resolve');

  const severity = params.get('severity') ?? '';
  const assignee = params.get('assignee') ?? '';
  const q = params.get('q') ?? '';
  const setParam = useCallback((key: string, value: string) => {
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      if (value) next.set(key, value);
      else next.delete(key);
      return next;
    }, { replace: true });
  }, [setParams]);

  // Kept live by useLiveQueryBridge: alert:updated patches rows in place,
  // alert:triggered/resolved refetch.
  const alertsQuery = useQuery({
    queryKey: queryKeys.alerts.list(tab, ALERT_LIST_LIMIT),
    queryFn: ({ signal }) => getAlerts(tab, ALERT_LIST_LIMIT, { signal }),
  });
  const countsQuery = useQuery({ queryKey: queryKeys.alerts.counts(), queryFn: ({ signal }) => getAlertCounts({ signal }) });
  const devicesQuery = useQuery({ queryKey: queryKeys.devices.list(), queryFn: ({ signal }) => getDevices({ signal }), enabled: hasPermission(user, 'devices.read') });
  const locationsQuery = useResourceList('/locations', { enabled: hasPermission(user, 'locations.read') });
  const counts = countsQuery.data?.data ?? EMPTY_COUNTS;
  const alerts = alertsQuery.data?.data ?? EMPTY_ALERTS;

  const rows = useMemo<AlertRow[]>(() => {
    const locName = new Map((locationsQuery.data ?? []).map((l) => [Number(l.id), String(l.name)]));
    const deviceLoc = new Map((devicesQuery.data?.data ?? []).map((d) => [d.id, d.locationId != null ? locName.get(d.locationId) ?? '' : '']));
    return alerts
      .filter((a) => !severity || a.severity === severity)
      .filter((a) => !assignee || (assignee === 'me' ? a.assignedTo === user?.id : assignee === 'none' ? a.assignedTo == null : true))
      .map((a) => ({ ...a, location: deviceLoc.get(a.deviceId) ?? '' }));
  }, [alerts, severity, assignee, user?.id, devicesQuery.data, locationsQuery.data]);

  // Active alerts present when the page loaded; a critical that arrives
  // later gets the one-off "new" pulse.
  const [baseline, setBaseline] = useState<Set<number> | null>(null);
  if (baseline === null && tab === 'active' && alertsQuery.isSuccess) setBaseline(new Set(alerts.map((a) => a.id)));
  const isNew = useCallback((a: Alert) => a.severity === 'critical' && a.status === 'active' && baseline !== null && !baseline.has(a.id), [baseline]);

  const [drawer, setDrawer] = useState<{ id: number; note: boolean } | null>(null);
  const [activeId, setActiveId] = useState<RowId | null>(null);
  const [helpOpen, setHelpOpen] = useState(false);
  const [bulkConfirm, setBulkConfirm] = useState<{ action: AlertBulkAction; ids: number[] } | null>(null);
  const [bulkNote, setBulkNote] = useState('');

  const columns = useMemo<Column<AlertRow>[]>(() => [
    {
      id: 'severity', header: 'Severity', width: '120px', searchable: false,
      value: (a) => SEVERITY_RANK[a.severity] ?? 3,
      cell: (a) => <StatusPill status={severityMeta(a.severity)} />,
    },
    {
      id: 'message', header: 'Alert', width: 'minmax(220px,3fr)', hideable: false,
      value: (a) => a.message,
      cell: (a) => (
        <span className="flex min-w-0 items-center gap-2">
          <button type="button" tabIndex={-1} onClick={(e) => { e.stopPropagation(); setActiveId(a.id); setDrawer({ id: a.id, note: false }); }} className="truncate text-left font-medium text-fg hover:underline">{a.message}</button>
          {isNew(a) && <span className="shrink-0 text-xs font-medium text-fg-muted">New</span>}
        </span>
      ),
    },
    {
      id: 'device', header: 'Device', width: 'minmax(140px,1.5fr)',
      value: (a) => a.deviceName || `Device ${a.deviceId}`,
      cell: (a) => <Link to={`/devices/${a.deviceId}`} tabIndex={-1} onClick={(e) => e.stopPropagation()} className="text-fg-muted hover:text-fg hover:underline">{a.deviceName || `Device ${a.deviceId}`}</Link>,
    },
    { id: 'location', header: 'Location', width: 'minmax(120px,1fr)', value: (a) => a.location },
    { id: 'status', header: 'Status', width: '136px', searchable: false, value: (a) => a.status, cell: (a) => <StatusPill status={lifecycleMeta(a.status)} /> },
    { id: 'assignee', header: 'Assignee', width: 'minmax(110px,1fr)', value: (a) => a.assignedToName ?? '', cell: (a) => a.assignedToName ? <span>{a.assignedToName}</span> : <span className="text-fg-subtle">—</span> },
    { id: 'age', header: 'Raised', width: '104px', searchable: false, value: (a) => new Date(a.createdAt).getTime(), cell: (a) => <RelativeTime value={a.createdAt} className="text-fg-muted" /> },
    {
      id: 'actions', header: 'Actions', width: '136px', hideable: false,
      cell: (a) => (
        <span className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
          {a.status === 'active' && canAck && <IconButton icon="check" size="sm" tabIndex={-1} label={`Acknowledge ${a.message}`} onClick={() => act(a.id, 'acknowledge')} />}
          {a.status !== 'resolved' && canResolve && <IconButton icon="done_all" size="sm" tabIndex={-1} label={`Resolve ${a.message}`} onClick={() => act(a.id, 'resolve')} />}
          <IconButton icon="sticky_note_2" size="sm" tabIndex={-1} label={`Notes for ${a.message}`} onClick={() => { setActiveId(a.id); setDrawer({ id: a.id, note: true }); }} />
        </span>
      ),
    },
  ], [act, canAck, canResolve, isNew, setActiveId, setDrawer]);

  const table = useDataTable({
    data: rows,
    columns,
    getRowId: (a) => a.id,
    views: [{ id: 'all', label: 'Newest first', state: { sort: { id: 'age', desc: true } }, builtIn: true }],
    storageKey: user ? `alerts:${user.id}` : undefined,
    groups: GROUPS,
  });
  // The text filter lives in ?q= (device pages link here with it).
  if (table.state.query !== q) table.setState({ query: q });

  const visible = table.rows;
  const active = visible.find((a) => a.id === activeId);

  /** Selection if any, else the current row; only alerts the action applies to. */
  const targets = useCallback((action: AlertBulkAction): Alert[] => {
    const base = table.selectedRows.length ? table.selectedRows : active ? [active] : [];
    return base.filter((a) => (action === 'acknowledge' ? a.status === 'active' : a.status !== 'resolved'));
  }, [table.selectedRows, active]);

  const run = useCallback((action: AlertBulkAction) => {
    if ((action === 'acknowledge' && !canAck) || (action === 'resolve' && !canResolve)) return;
    const list = targets(action);
    if (list.length === 0) return;
    if (list.length === 1 && !table.selectedRows.length) act(list[0]!.id, action);
    else setBulkConfirm({ action, ids: list.map((a) => a.id) });
  }, [targets, act, canAck, canResolve, table.selectedRows.length, setBulkConfirm]);

  // Keyboard triage. Ignored while typing or while a dialog is open.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey || isTyping(event.target)) return;
      if (document.querySelector('dialog[open]')) return;
      const index = active ? visible.indexOf(active) : -1;
      const move = (delta: number) => {
        const next = visible[Math.min(visible.length - 1, Math.max(0, index + delta))];
        if (next) setActiveId(next.id);
      };
      switch (event.key) {
        case 'j': move(1); break;
        case 'k': move(-1); break;
        case 'x': if (active) table.toggleRow(active); break;
        case 'a': run('acknowledge'); break;
        case 'r': run('resolve'); break;
        case 'n': if (active) setDrawer({ id: active.id, note: true }); break;
        case '?': setHelpOpen(true); break;
        case 'Escape': if (table.selectedRows.length) table.clearSelection(); else return; break;
        default: return;
      }
      event.preventDefault();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [active, visible, run, table]);

  const drawerAlert = drawer ? alerts.find((a) => a.id === drawer.id) ?? null : null;
  const filtered = Boolean(severity || assignee || q);

  return (
    <div>
      <SectionHeader
        title="Alerts"
        subtitle="Live alert workbench. Select with x, acknowledge with a, resolve with r."
        action={<Button variant="secondary" icon="keyboard" onClick={() => setHelpOpen(true)}>Shortcuts</Button>}
      />

      <Tabs
        label="Alert status"
        idBase={tabsId}
        value={tab}
        onChange={(next) => { setTab(next); table.clearSelection(); setActiveId(null); }}
        className="mb-4"
        items={[
          { id: 'active', label: 'Active', badge: counts.active },
          { id: 'acknowledged', label: 'Acknowledged', badge: counts.acknowledged },
          { id: 'resolved', label: 'Resolved', badge: counts.resolved },
        ]}
      />

      <div role="search" aria-label="Filter alerts" className="mb-3 flex flex-wrap items-end gap-2">
        <label className="relative min-w-56 flex-1">
          <span className="sr-only">Search alert, device or location</span>
          <span className="material-symbols-outlined pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[18px] text-fg-subtle" aria-hidden="true">search</span>
          <input
            type="search"
            value={q}
            onChange={(e) => setParam('q', e.target.value)}
            placeholder="Search alert, device or location"
            className="h-control w-full rounded-md border border-line bg-surface-1 pl-9 pr-3 text-sm text-fg placeholder:text-fg-subtle outline-none focus:border-primary"
          />
        </label>
        <label className="flex items-center gap-2 text-xs text-fg-muted">
          Severity
          <select value={severity} onChange={(e) => setParam('severity', e.target.value)} className="h-control rounded-md border border-line bg-surface-1 px-3 text-sm text-fg outline-none focus:border-primary">
            <option value="">Any</option>
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
            <option value="info">Info</option>
          </select>
        </label>
        <label className="flex items-center gap-2 text-xs text-fg-muted">
          Assignee
          <select value={assignee} onChange={(e) => setParam('assignee', e.target.value)} className="h-control rounded-md border border-line bg-surface-1 px-3 text-sm text-fg outline-none focus:border-primary">
            <option value="">Anyone</option>
            <option value="me">Assigned to me</option>
            <option value="none">Unassigned</option>
          </select>
        </label>
        <label className="flex items-center gap-2 text-xs text-fg-muted">
          Group by
          <select value={table.state.groupBy ?? ''} onChange={(e) => table.setState({ groupBy: e.target.value || null })} className="h-control rounded-md border border-line bg-surface-1 px-3 text-sm text-fg outline-none focus:border-primary">
            <option value="">None</option>
            {GROUPS.map((g) => <option key={g.id} value={g.id}>{g.label}</option>)}
          </select>
        </label>
        {filtered && <Button variant="ghost" icon="filter_alt_off" onClick={() => setParams({}, { replace: true })}>Clear filters</Button>}
      </div>

      {alertsQuery.isError ? (
        <ErrorState message="Failed to load alerts. Please try again." onRetry={() => void alertsQuery.refetch()} />
      ) : alertsQuery.isPending ? (
        <div role="status" className="py-12 text-center text-sm text-fg-muted">Loading alerts…</div>
      ) : (
        <DataTable
          table={table}
          label={`${tab[0]!.toUpperCase()}${tab.slice(1)} alerts`}
          rowLabel={(a) => a.message}
          selectable={canAck || canResolve}
          activeId={activeId}
          onActiveIdChange={setActiveId}
          onRowOpen={(a) => setDrawer({ id: a.id, note: false })}
          rowClassName={(a) => `border-l-4 ${severityMeta(a.severity).borderL} ${isNew(a) ? 'alert-new-pulse' : ''}`}
          maxHeight="calc(100dvh - 330px)"
          empty={
            filtered
              ? <EmptyState icon="filter_alt_off" title="No alerts match" description="Change or clear the filters." />
              : <EmptyState icon="notifications_off" title={`No ${tab} alerts`} description={tab === 'active' ? 'Everything is quiet right now.' : undefined} />
          }
          bulkActions={(selected) => (
            <>
              {canAck && <Button size="sm" variant="secondary" icon="check" disabled={pending || !selected.some((a) => a.status === 'active')} onClick={() => run('acknowledge')}>Acknowledge</Button>}
              {canResolve && <Button size="sm" variant="primary-outline" icon="done_all" disabled={pending || !selected.some((a) => a.status !== 'resolved')} onClick={() => run('resolve')}>Resolve</Button>}
            </>
          )}
        />
      )}
      {alerts.length >= ALERT_LIST_LIMIT && (
        <p className="mt-1 text-xs text-fg-muted">Showing the newest {ALERT_LIST_LIMIT} {tab} alerts (the server's page limit).</p>
      )}

      {drawerAlert && (
        <AlertDrawer
          alert={drawerAlert}
          onClose={() => setDrawer(null)}
          canAssign={canAck}
          canNote={canAck}
          focusNote={drawer?.note}
        />
      )}

      <Dialog
        open={bulkConfirm != null}
        onClose={() => setBulkConfirm(null)}
        size="sm"
        title={`${bulkConfirm?.action === 'resolve' ? 'Resolve' : 'Acknowledge'} ${bulkConfirm?.ids.length ?? 0} alerts?`}
        footer={
          <>
            <Button variant="secondary" onClick={() => setBulkConfirm(null)}>Cancel</Button>
            <Button
              onClick={() => {
                if (!bulkConfirm) return;
                void bulkAct(bulkConfirm.ids, bulkConfirm.action, bulkNote.trim() || undefined).then(() => table.clearSelection(), () => {});
                setBulkConfirm(null);
                setBulkNote('');
              }}
            >
              {bulkConfirm?.action === 'resolve' ? 'Resolve' : 'Acknowledge'}
            </Button>
          </>
        }
      >
        <label htmlFor="bulk-note" className="mb-1.5 block text-sm text-fg-muted">Note (optional, added to each alert)</label>
        <textarea id="bulk-note" value={bulkNote} onChange={(e) => setBulkNote(e.target.value)} maxLength={2000} rows={3} className="w-full rounded-md border border-line bg-inset px-3 py-2 text-sm text-fg outline-none focus:border-primary" />
      </Dialog>

      <Dialog open={helpOpen} onClose={() => setHelpOpen(false)} size="sm" title="Keyboard shortcuts" description="Work when focus isn't in a text field.">
        <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
          {SHORTCUTS.map(([keys, what]) => (
            <div key={keys} className="contents">
              <dt><kbd className="rounded-sm border border-line-strong bg-surface-2 px-1.5 py-0.5 font-mono text-xs">{keys}</kbd></dt>
              <dd className="text-fg-muted">{what}</dd>
            </div>
          ))}
        </dl>
      </Dialog>
    </div>
  );
}
