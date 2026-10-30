import { useState, useMemo, Suspense } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getDevices, getLatestMetrics, deleteDevice } from '../api/client';
import { queryKeys } from '../api/queryKeys';
import type { Device, Metric } from '../api/types';
import DeviceAddModal from '../components/DeviceAddModal';
import ConfirmDialog from '../components/ConfirmDialog';
import StatCard from '../components/ui/StatCard';
import Button from '../components/ui/Button';
import SectionHeader from '../components/ui/SectionHeader';
import ErrorState from '../components/ui/ErrorState';
import { useToast } from '../components/ui/useToast';
import { DevicesSkeleton } from '../components/dashboard/DevicesSkeleton';
import { normalizeStatus, statusMeta } from '../utils/status';
import IconButton from '../components/ui/IconButton';
import { StatusPill } from '../components/ui/Status';
import { iconForProtocol } from '../utils/icons';
import { lazyWithRetry } from '../utils/lazyWithRetry';

// Pulls in recharts; keep it out of the device list's initial chunk.
const DeviceModal = lazyWithRetry(() => import('../components/DeviceModal'));

function statusOf(device: Device, metricsMap: Map<number, Metric>): string {
  return metricsMap.get(device.id)?.status || device.status || 'unknown';
}

export default function Devices() {
  const { addToast } = useToast();
  const location = useLocation();
  const queryClient = useQueryClient();
  const urlSearch = new URLSearchParams(location.search).get('search') || '';
  const [search, setSearch] = useState(urlSearch);
  const [syncedUrlSearch, setSyncedUrlSearch] = useState(urlSearch);
  const [statusFilter, setStatusFilter] = useState('all');
  const [showForm, setShowForm] = useState(false);
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Device | null>(null);

  // The header search navigates here with ?search=; adopt it when it changes.
  if (urlSearch !== syncedUrlSearch) {
    setSyncedUrlSearch(urlSearch);
    setSearch(urlSearch);
  }

  // Live status/metric updates are written into these caches by useLiveQueryBridge.
  const devicesQuery = useQuery({
    queryKey: queryKeys.devices.list(),
    queryFn: ({ signal }) => getDevices({ signal }),
  });
  const metricsQuery = useQuery({
    queryKey: queryKeys.metrics.latest,
    queryFn: ({ signal }) => getLatestMetrics({ signal }),
  });

  const devices = useMemo(() => devicesQuery.data?.data || [], [devicesQuery.data]);
  const metricsMap = useMemo(
    () => new Map((metricsQuery.data?.data || []).map((m) => [m.deviceId, m])),
    [metricsQuery.data],
  );

  const refreshDevices = () => queryClient.invalidateQueries({ queryKey: queryKeys.devices.all });

  const deleteMutation = useMutation({
    mutationFn: (device: Device) => deleteDevice(device.id),
    onSuccess: (_res, device) => {
      queryClient.setQueryData<{ data: Device[]; success: boolean }>(queryKeys.devices.list(), (prev) =>
        prev ? { ...prev, data: prev.data.filter((d) => d.id !== device.id) } : prev,
      );
      addToast(`Deleted "${device.name}"`, 'success');
      setDeleteTarget(null);
      void refreshDevices();
    },
    onError: () => addToast('Failed to delete device', 'error'),
  });

  const filtered = useMemo(() => devices.filter((d) => {
    const status = statusOf(d, metricsMap);
    const text = `${d.name} ${d.ipAddress} ${d.protocol}`.toLowerCase();
    const matchSearch = !search || text.includes(search.toLowerCase());
    let matchStatus = true;
    if (statusFilter !== 'all') matchStatus = normalizeStatus(status) === statusFilter;
    return matchSearch && matchStatus;
  }), [devices, metricsMap, search, statusFilter]);

  const handleDelete = (device: Device) => {
    setDeleteTarget(device);
  };

  const confirmDelete = () => {
    if (deleteTarget) deleteMutation.mutate(deleteTarget);
  };

  // Stats
  const total = devices.length;
  const up = useMemo(() => devices.filter((d) => normalizeStatus(statusOf(d, metricsMap)) === 'up').length, [devices, metricsMap]);
  const down = useMemo(() => devices.filter((d) => normalizeStatus(statusOf(d, metricsMap)) === 'down').length, [devices, metricsMap]);
  const warn = useMemo(() => devices.filter((d) => normalizeStatus(statusOf(d, metricsMap)) === 'warning').length, [devices, metricsMap]);

  return (
    <div>
      {devicesQuery.isPending ? (
        <DevicesSkeleton />
      ) : devicesQuery.isError ? (
        <ErrorState message="Failed to load devices. Please try again." onRetry={() => void devicesQuery.refetch()} />
      ) : (
      <>
      {/* Header */}
      <SectionHeader
        title="Devices"
        subtitle="Manage and monitor all connected network devices."
        action={
          <div className="flex gap-2">
            <Link to="/topology" className="inline-flex min-h-control items-center gap-2 rounded-md border border-line bg-surface-2 px-4 text-sm font-medium text-fg hover:bg-surface-3 hover:border-line-strong">
              <span className="material-symbols-outlined text-[18px]" aria-hidden="true">hub</span>Topology
            </Link>
            <Button onClick={() => setShowForm(!showForm)} icon="add_circle">Add device</Button>
          </div>
        }
      />

      {/* Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-6">
        <StatCard label="Total" value={total} icon="devices" />
        <StatCard label="Up" value={up} icon={statusMeta('up').icon} />
        <StatCard label="Down" value={down} icon={statusMeta('down').icon} color={down ? statusMeta('down').text : 'text-fg'} />
        <StatCard label="Warning" value={warn} icon={statusMeta('warning').icon} color={warn ? statusMeta('warning').text : 'text-fg'} />
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-4 mb-6">
        <div className="flex-1 min-w-48">
          <input type="search" aria-label="Search devices" value={search} onChange={(e) => setSearch(e.target.value)} className="w-full bg-surface-1 border border-line rounded-md px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:border-primary outline-none" placeholder="Search devices…" />
        </div>
        <select aria-label="Filter by status" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="bg-surface-1 border border-line rounded-md px-3 py-2.5 text-sm text-fg outline-none focus:border-primary">
          <option value="all">All statuses</option>
          <option value="up">Up</option>
          <option value="warning">Warning</option>
          <option value="down">Down</option>
          <option value="unknown">Unknown</option>
        </select>
      </div>

      {/* Device Cards Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filtered.map((device) => {
          const status = statusOf(device, metricsMap);
          const metric = metricsMap.get(device.id);
          const meta = statusMeta(status);
          return (
            <article
              key={device.id}
              className={`relative bg-surface-1 rounded-lg group overflow-hidden border border-line border-t-2 ${meta.borderT} hover:border-x-line-strong hover:border-b-line-strong hover:bg-surface-2 transition-colors duration-[var(--duration-fast)] flex flex-col card-stagger`}
              style={{ '--i': filtered.indexOf(device) } as React.CSSProperties}
            >
              <div className="p-5">
                <div className="flex justify-between items-start mb-5">
                  <div className="bg-inset p-3 rounded-md text-fg-muted">
                    <span className="material-symbols-outlined text-3xl" aria-hidden="true">{iconForProtocol(device.protocol)}</span>
                  </div>
                  <StatusPill status={meta} />
                </div>
                <h3 className="text-lg font-semibold mb-1 truncate">
                  {/* Stretched button: the whole card opens the device, the delete button sits above it. */}
                  <button
                    type="button"
                    onClick={() => setSelectedDevice(device)}
                    className="text-left rounded after:absolute after:inset-0 after:content-[''] focus-visible:after:rounded-lg focus-visible:after:outline-2 focus-visible:after:outline-focus"
                  >
                    {device.name}
                  </button>
                </h3>
                <code className="font-mono text-fg-muted text-xs mb-4 block truncate">{device.protocol === 'http' || device.protocol === 'https' ? `${device.protocol}://${device.ipAddress}` : device.ipAddress}{device.port > 0 && !['http','https'].includes(device.protocol) ? `:${device.port}` : ''}</code>
                <div className="space-y-2 text-xs">
                  <div className="flex justify-between"><span className="text-fg-muted">Protocol</span><span className="font-semibold uppercase">{device.protocol}</span></div>
                  <div className="flex justify-between"><span className="text-fg-muted">Interval</span><span className="font-mono font-semibold">{device.interval}s</span></div>
                  {metric && <div className="flex justify-between"><span className="text-fg-muted">Response</span><span className="font-mono font-semibold">{metric.responseTime ?? '—'}ms</span></div>}
                </div>
              </div>
              <div className="mt-auto border-t border-line px-5 py-2 flex justify-between items-center">
                <span className="text-xs text-fg-muted font-mono">{metric ? new Date(metric.timestamp || metric.createdAt).toLocaleTimeString() : 'No data'}</span>
                <IconButton
                  icon="delete"
                  label={`Delete ${device.name}`}
                  variant="danger"
                  size="sm"
                  className="relative"
                  onClick={() => handleDelete(device)}
                />
              </div>
            </article>
          );
        })}

        {/* Add New Card */}
        <button type="button" onClick={() => setShowForm(true)} className="rounded-lg border-2 border-dashed border-line hover:border-line-strong hover:bg-surface-1 transition-colors flex flex-col items-center justify-center p-10 text-center group min-h-[200px]">
          <span className="w-14 h-14 rounded-full bg-surface-2 flex items-center justify-center mb-4 group-hover:bg-surface-3 transition-colors">
            <span className="material-symbols-outlined text-3xl text-fg-muted group-hover:text-fg" aria-hidden="true">add</span>
          </span>
          <span className="text-lg font-semibold text-fg-muted group-hover:text-fg">Add device</span>
          <span className="text-fg-muted text-sm max-w-[200px] mx-auto mt-1">Deploy a new probe or add an existing node</span>
        </button>
      </div>
      {selectedDevice && (
        <Suspense fallback={null}>
        <DeviceModal
          device={selectedDevice}
          onClose={() => setSelectedDevice(null)}
          onDeleted={() => {
            setSelectedDevice(null);
            void refreshDevices();
          }}
        />
        </Suspense>
      )}
      <DeviceAddModal
        open={showForm}
        onClose={() => setShowForm(false)}
        onAdded={() => void refreshDevices()}
      />
      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete device?"
        message={`Are you sure you want to delete "${deleteTarget?.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        danger
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />
      </>
      )}
    </div>
  );
}
