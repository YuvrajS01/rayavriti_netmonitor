import { useState, useMemo, useCallback, Suspense } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getDevices, getLatestMetrics, deleteDevice } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import type { Device, Metric } from '../api/types';
import DeviceAddModal from '../components/DeviceAddModal';
import StatCard from '../components/ui/StatCard';
import Button from '../components/ui/Button';
import SectionHeader from '../components/ui/SectionHeader';
import ErrorState from '../components/ui/ErrorState';
import EmptyState from '../components/ui/EmptyState';
import IconButton from '../components/ui/IconButton';
import RelativeTime from '../components/ui/RelativeTime';
import TypeToConfirmDialog from '../components/ui/TypeToConfirmDialog';
import { DataTable, DataTableToolbar } from '../components/ui/DataTable';
import { useDataTable } from '../components/ui/useDataTable';
import type { Column, GroupOption, TableView } from '../components/ui/dataTableModel';
import { useToast } from '../components/ui/useToast';
import { DevicesSkeleton } from '../components/dashboard/DevicesSkeleton';
import { STATUS_META, STATUS_SEVERITY_ORDER, normalizeStatus, statusMeta, statusRank, type Status } from '../utils/status';
import { StatusPill } from '../components/ui/Status';
import { iconForProtocol } from '../utils/icons';
import { lazyWithRetry } from '../utils/lazyWithRetry';
import { useResourceList } from '../hooks/useResources';
import { useSessionUser } from '../store/session';
import { hasPermission } from '../navigation';

// Pulls in recharts; keep it out of the device list's initial chunk.
const DeviceModal = lazyWithRetry(() => import('../components/DeviceModal'));

interface DeviceRow extends Device {
  live: Status;
  metric?: Metric;
  locationName: string;
}

const CAMERA_CATEGORIES = ['camera', 'nvr', 'cctv'];
const isCamera = (d: Device) => d.protocol === 'camera' || CAMERA_CATEGORIES.includes((d.deviceCategory || '').toLowerCase());
const isBiometric = (d: Device) => d.protocol === 'biometric' || (d.deviceCategory || '').toLowerCase() === 'biometric';


const VIEWS: TableView<DeviceRow>[] = [
  { id: 'all', label: 'All devices', state: { sort: { id: 'status', desc: false } }, builtIn: true },
  { id: 'attention', label: 'Needs attention', state: { sort: { id: 'status', desc: false } }, predicate: (d) => d.live === 'down' || d.live === 'warning', builtIn: true },
  { id: 'cameras', label: 'Cameras', state: { hidden: ['protocol'] }, predicate: isCamera, builtIn: true },
  { id: 'biometrics', label: 'Biometrics', state: { hidden: ['protocol'] }, predicate: isBiometric, builtIn: true },
];

const GROUPS: GroupOption<DeviceRow>[] = [
  { id: 'location', label: 'Location', key: (d) => d.locationName || 'Unassigned' },
  { id: 'status', label: 'Status', key: (d) => STATUS_META[d.live].label, compare: (a, b) => statusRank(a.toLowerCase()) - statusRank(b.toLowerCase()) },
  { id: 'protocol', label: 'Protocol', key: (d) => d.protocol.toUpperCase() },
];

const LAYOUT_KEY = 'nm-devices-layout';
const readLayout = (): 'table' | 'cards' => {
  try {
    return window.localStorage.getItem(LAYOUT_KEY) === 'cards' ? 'cards' : 'table';
  } catch {
    return 'table';
  }
};

const address = (d: Device) =>
  `${d.protocol === 'http' || d.protocol === 'https' ? `${d.protocol}://${d.ipAddress}` : d.ipAddress}${d.port > 0 && !['http', 'https'].includes(d.protocol) ? `:${d.port}` : ''}`;

export default function Devices() {
  const { addToast } = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useSessionUser();
  const [params, setParams] = useSearchParams();
  const urlSearch = params.get('search') || '';
  const [syncedUrlSearch, setSyncedUrlSearch] = useState<string | null>(null);
  const [layout, setLayout] = useState(readLayout);
  const [showFilters, setShowFilters] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [peek, setPeek] = useState<Device | null>(null);
  const [bulkDelete, setBulkDelete] = useState<DeviceRow[] | null>(null);
  const canWrite = hasPermission(user, 'devices.write');
  const canDelete = hasPermission(user, 'devices.delete');

  // Live status/metric updates are written into these caches by useLiveQueryBridge.
  const devicesQuery = useQuery({ queryKey: queryKeys.devices.list(), queryFn: ({ signal }) => getDevices({ signal }) });
  const metricsQuery = useQuery({ queryKey: queryKeys.metrics.latest, queryFn: ({ signal }) => getLatestMetrics({ signal }) });
  const locationsQuery = useResourceList('/locations', { enabled: hasPermission(user, 'locations.read') });

  const rows = useMemo<DeviceRow[]>(() => {
    const metrics = new Map((metricsQuery.data?.data || []).map((m) => [m.deviceId, m]));
    const locations = new Map((locationsQuery.data || []).map((l) => [Number(l.id), String(l.name)]));
    return (devicesQuery.data?.data || []).map((d) => {
      const metric = metrics.get(d.id);
      return {
        ...d,
        metric,
        live: normalizeStatus(metric?.status || d.status),
        locationName: d.locationId != null ? locations.get(d.locationId) ?? '' : '',
      };
    });
  }, [devicesQuery.data, metricsQuery.data, locationsQuery.data]);

  const protocols = useMemo(() => [...new Set(rows.map((r) => r.protocol))].sort(), [rows]);

  const columns = useMemo<Column<DeviceRow>[]>(() => [
    {
      id: 'status',
      header: 'Status',
      value: (d) => statusRank(d.live),
      cell: (d) => <StatusPill status={d.live} />,
      filter: STATUS_SEVERITY_ORDER.map((s) => ({ value: String(statusRank(s)), label: STATUS_META[s].label })),
      width: '128px',
      searchable: false,
    },
    {
      id: 'name',
      header: 'Name',
      value: (d) => d.name,
      cell: (d) => (
        <Link to={`/devices/${d.id}`} className="font-medium text-fg hover:underline" tabIndex={-1}>
          {d.name}
        </Link>
      ),
      filter: 'text',
      width: 'minmax(180px,2fr)',
      hideable: false,
    },
    { id: 'address', header: 'Address', value: (d) => address(d), cell: (d) => <code className="font-mono text-xs text-fg-muted">{address(d)}</code>, filter: 'text', width: 'minmax(160px,1.5fr)' },
    { id: 'protocol', header: 'Protocol', value: (d) => d.protocol, cell: (d) => <span className="uppercase">{d.protocol}</span>, filter: protocols.map((p) => ({ value: p, label: p.toUpperCase() })), width: '112px' },
    { id: 'location', header: 'Location', value: (d) => d.locationName, filter: 'text', width: 'minmax(140px,1fr)' },
    { id: 'response', header: 'Response', value: (d) => d.metric?.responseTime ?? null, cell: (d) => (d.metric?.responseTime != null ? <span className="font-mono">{Math.round(d.metric.responseTime)} ms</span> : '—'), align: 'end', width: '112px', searchable: false },
    { id: 'lastPoll', header: 'Last poll', value: (d) => (d.metric ? new Date(d.metric.timestamp || d.metric.createdAt).getTime() : null), cell: (d) => <RelativeTime value={d.metric ? d.metric.timestamp || d.metric.createdAt : null} className="text-fg-muted" />, width: '112px', searchable: false },
    { id: 'category', header: 'Category', value: (d) => d.deviceCategory || '', width: '120px' },
    { id: 'vendor', header: 'Vendor', value: (d) => [d.manufacturer, d.model].filter(Boolean).join(' '), width: 'minmax(120px,1fr)' },
    { id: 'serial', header: 'Serial / asset', value: (d) => [d.serialNumber, d.assetTag].filter(Boolean).join(' · '), width: 'minmax(120px,1fr)' },
    {
      id: 'actions',
      header: 'Actions',
      hideable: false,
      width: '64px',
      cell: (d) => <IconButton icon="visibility" label={`Quick look at ${d.name}`} size="sm" tabIndex={-1} onClick={(e) => { e.stopPropagation(); setPeek(d); }} />,
    },
  ], [protocols]);

  const views = useMemo(() => VIEWS.map((v) => ({ ...v, state: { ...v.state, hidden: [...(v.state.hidden ?? []), 'category', 'vendor', 'serial'] } })), []);

  const viewId = params.get('view');
  const setViewId = useCallback((id: string) => {
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      if (id === 'all') next.delete('view');
      else next.set('view', id);
      next.delete('search');
      return next;
    }, { replace: true });
  }, [setParams]);

  const table = useDataTable({
    data: rows,
    columns,
    getRowId: (d) => d.id,
    views,
    viewId,
    onViewChange: setViewId,
    storageKey: user ? `devices:${user.id}` : undefined,
    groups: GROUPS,
  });

  // The header search navigates here with ?search=; adopt it when it changes.
  if (urlSearch !== (syncedUrlSearch ?? '')) {
    setSyncedUrlSearch(urlSearch);
    table.setState({ query: urlSearch });
  }

  const refreshDevices = () => queryClient.invalidateQueries({ queryKey: queryKeys.devices.all });

  const deleteMany = useMutation({
    mutationFn: async (targets: DeviceRow[]) => {
      const results = await Promise.allSettled(targets.map((d) => deleteDevice(d.id)));
      return { deleted: targets.filter((_, i) => results[i]!.status === 'fulfilled'), failed: results.filter((r) => r.status === 'rejected').length };
    },
    onSuccess: ({ deleted, failed }) => {
      const gone = new Set(deleted.map((d) => d.id));
      queryClient.setQueryData<{ data: Device[]; success: boolean }>(queryKeys.devices.list(), (prev) =>
        prev ? { ...prev, data: prev.data.filter((d) => !gone.has(d.id)) } : prev,
      );
      table.clearSelection();
      setBulkDelete(null);
      if (deleted.length) addToast(`Deleted ${deleted.length} device${deleted.length === 1 ? '' : 's'}`, 'success');
      if (failed) addToast(`${failed} device${failed === 1 ? '' : 's'} could not be deleted`, 'error');
      void refreshDevices();
    },
  });

  const counts = useMemo(() => {
    const c = { up: 0, down: 0, warning: 0 } as Record<string, number>;
    for (const r of rows) c[r.live] = (c[r.live] ?? 0) + 1;
    return c;
  }, [rows]);

  const changeLayout = (next: 'table' | 'cards') => {
    setLayout(next);
    try { window.localStorage.setItem(LAYOUT_KEY, next); } catch { /* per-tab only */ }
  };

  const openInTabs = (targets: DeviceRow[]) => {
    for (const d of targets.slice(0, 10)) window.open(`/devices/${d.id}`, '_blank', 'noopener');
  };

  if (devicesQuery.isPending) return <DevicesSkeleton />;
  if (devicesQuery.isError) return <ErrorState message="Failed to load devices. Please try again." onRetry={() => void devicesQuery.refetch()} />;

  const addProtocol = table.viewId === 'cameras' ? 'camera' : table.viewId === 'biometrics' ? 'biometric' : undefined;
  const empty = rows.length === 0
    ? <EmptyState icon="devices" title="No devices yet" description="Add a device or run discovery to start monitoring." action={canWrite ? <Button icon="add_circle" onClick={() => setShowForm(true)}>Add device</Button> : undefined} />
    : <EmptyState icon="filter_alt_off" title="No devices match" description="Change the search, filters or view." />;

  return (
    <div>
      <SectionHeader
        title="Devices"
        subtitle="Every monitored device. Select rows for bulk actions; press Enter to open one."
        action={
          <>
            <Link to="/devices/sensors" className="inline-flex min-h-control items-center gap-2 rounded-md border border-line bg-surface-2 px-4 text-sm font-medium text-fg hover:border-line-strong hover:bg-surface-3">
              <span className="material-symbols-outlined text-[18px]" aria-hidden="true">sensors</span>Sensor overview
            </Link>
            {canWrite && <Button onClick={() => setShowForm(true)} icon="add_circle">Add device</Button>}
          </>
        }
      />

      <div className="mb-6 grid grid-cols-2 gap-4 md:grid-cols-4">
        <StatCard label="Total" value={rows.length} icon="devices" />
        <StatCard label="Up" value={counts.up ?? 0} icon={statusMeta('up').icon} />
        <StatCard label="Down" value={counts.down ?? 0} icon={statusMeta('down').icon} color={counts.down ? statusMeta('down').text : 'text-fg'} />
        <StatCard label="Warning" value={counts.warning ?? 0} icon={statusMeta('warning').icon} color={counts.warning ? statusMeta('warning').text : 'text-fg'} />
      </div>

      <DataTableToolbar table={table} searchLabel="Search name, IP, location…" showFilters={layout === 'table' && showFilters} onToggleFilters={layout === 'table' ? () => setShowFilters((v) => !v) : undefined}>
        <div role="group" aria-label="Layout" className="flex gap-0.5 rounded-md border border-line bg-inset p-0.5">
          <IconButton icon="table_rows" label="Table view" size="sm" pressed={layout === 'table'} onClick={() => changeLayout('table')} className={layout === 'table' ? 'bg-surface-3 text-fg' : ''} />
          <IconButton icon="grid_view" label="Card view" size="sm" pressed={layout === 'cards'} onClick={() => changeLayout('cards')} className={layout === 'cards' ? 'bg-surface-3 text-fg' : ''} />
        </div>
      </DataTableToolbar>

      {layout === 'table' ? (
        <DataTable
          table={table}
          label="Devices"
          rowLabel={(d) => d.name}
          selectable
          showFilters={showFilters}
          onRowOpen={(d) => void navigate(`/devices/${d.id}`)}
          empty={empty}
          groupHeader={(g) => {
            const worst = g.rows.reduce<Status>((w, r) => (statusRank(r.live) < statusRank(w) ? r.live : w), 'up');
            return (
              <>
                <StatusPill status={worst}>{g.label}</StatusPill>
                <span className="text-xs font-normal text-fg-muted tabular-nums">{g.rows.length} devices</span>
              </>
            );
          }}
          bulkActions={(selected) => (
            <>
              <Button size="sm" variant="secondary" icon="open_in_new" onClick={() => openInTabs(selected)}>
                {selected.length > 10 ? 'Open first 10' : 'Open'}
              </Button>
              {canDelete && (
                <Button size="sm" variant="danger-outline" icon="delete" onClick={() => setBulkDelete(selected)}>Delete…</Button>
              )}
            </>
          )}
        />
      ) : table.rows.length === 0 ? (
        empty
      ) : (
        <ul className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="Devices">
          {table.rows.map((d) => {
            const meta = statusMeta(d.live);
            return (
              <li key={d.id} className={`relative flex flex-col rounded-lg border border-line border-t-2 bg-surface-1 ${meta.borderT} transition-colors hover:border-x-line-strong hover:border-b-line-strong hover:bg-surface-2`}>
                <div className="p-5">
                  <div className="mb-4 flex items-start justify-between">
                    <span className="material-symbols-outlined rounded-md bg-inset p-2.5 text-2xl text-fg-muted" aria-hidden="true">{iconForProtocol(d.protocol)}</span>
                    <StatusPill status={meta} />
                  </div>
                  <h2 className="mb-1 truncate text-lg font-semibold">
                    <Link to={`/devices/${d.id}`} className="rounded after:absolute after:inset-0 after:content-[''] focus-visible:after:rounded-lg focus-visible:after:outline-2 focus-visible:after:outline-focus">{d.name}</Link>
                  </h2>
                  <code className="mb-3 block truncate font-mono text-xs text-fg-muted">{address(d)}</code>
                  <dl className="grid grid-cols-2 gap-y-1.5 text-xs">
                    <dt className="text-fg-muted">Location</dt><dd className="truncate text-right">{d.locationName || '—'}</dd>
                    <dt className="text-fg-muted">Response</dt><dd className="text-right font-mono">{d.metric?.responseTime != null ? `${Math.round(d.metric.responseTime)} ms` : '—'}</dd>
                    <dt className="text-fg-muted">Last poll</dt><dd className="text-right"><RelativeTime value={d.metric ? d.metric.timestamp || d.metric.createdAt : null} /></dd>
                  </dl>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {peek && (
        <Suspense fallback={null}>
          <DeviceModal device={peek} onClose={() => setPeek(null)} onDeleted={() => { setPeek(null); void refreshDevices(); }} />
        </Suspense>
      )}
      <DeviceAddModal key={addProtocol ?? "any"} open={showForm} onClose={() => setShowForm(false)} onAdded={() => void refreshDevices()} initialProtocol={addProtocol} />
      <TypeToConfirmDialog
        open={!!bulkDelete}
        title={`Delete ${bulkDelete?.length ?? 0} devices?`}
        message={
          <>
            <p>This removes the devices, their metrics history and port scans. It cannot be undone.</p>
            <p className="mt-2 font-mono text-xs text-fg">{bulkDelete?.slice(0, 6).map((d) => d.name).join(', ')}{(bulkDelete?.length ?? 0) > 6 ? ` and ${bulkDelete!.length - 6} more` : ''}</p>
          </>
        }
        phrase={`delete ${bulkDelete?.length ?? 0}`}
        confirmLabel="Delete devices"
        busy={deleteMany.isPending}
        onConfirm={() => bulkDelete && deleteMany.mutate(bulkDelete)}
        onCancel={() => setBulkDelete(null)}
      />
    </div>
  );
}
