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
import { statusTextColor, statusBgColor } from '../utils/colors';
import { iconForProtocol } from '../utils/icons';
import { lazyWithRetry } from '../utils/lazyWithRetry';

// Pulls in recharts; keep it out of the device list's initial chunk.
const DeviceModal = lazyWithRetry(() => import('../components/DeviceModal'));

function statusOf(device: Device, metricsMap: Map<number, Metric>): string {
  return metricsMap.get(device.id)?.status || device.status || 'unknown';
}

const STATUS_GROUP_HOVER_TEXT: Record<string, string> = {
  down: 'group-hover:text-error',
  warning: 'group-hover:text-warning',
  degraded: 'group-hover:text-warning',
  unknown: 'group-hover:text-fg-subtle',
};

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
    if (statusFilter === 'up') matchStatus = status === 'up' || status === 'ok';
    else if (statusFilter === 'warning') matchStatus = status === 'warning' || status === 'degraded';
    else if (statusFilter === 'down') matchStatus = status === 'down';
    else if (statusFilter === 'unknown') matchStatus = status === 'unknown';
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
  const up = useMemo(() => devices.filter((d) => { const s = statusOf(d, metricsMap); return s === 'up' || s === 'ok'; }).length, [devices, metricsMap]);
  const down = useMemo(() => devices.filter((d) => statusOf(d, metricsMap) === 'down').length, [devices, metricsMap]);
  const warn = useMemo(() => devices.filter((d) => { const s = statusOf(d, metricsMap); return s === 'warning' || s === 'degraded'; }).length, [devices, metricsMap]);

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
        title="My Devices"
        subtitle="Manage and monitor all connected network devices."
        action={
          <div className="flex gap-2"><Link to="/devices/topology" className="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold border border-line rounded-md text-fg-muted hover:text-fg hover:border-primary/50"><span className="material-symbols-outlined text-base">hub</span>Topology</Link><Button onClick={() => setShowForm(!showForm)} icon="add_circle">Add Device</Button></div>
        }
      />

      {/* Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-6">
        <StatCard label="Total" value={total} />
        <StatCard label="Online" value={up} />
        <StatCard label="Down" value={down} color="text-error" />
        <StatCard label="Warning" value={warn} color="text-warning" />
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-4 mb-6">
        <div className="flex-1 min-w-48">
          <input value={search} onChange={(e) => setSearch(e.target.value)} className="w-full bg-inset border border-line rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus:ring-1 focus:ring-primary outline-none" placeholder="Search devices..." />
        </div>
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="bg-inset border border-line rounded-lg px-3 py-2.5 text-xs text-fg outline-none focus:ring-1 focus:ring-primary">
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
          const hoverText = STATUS_GROUP_HOVER_TEXT[status] || 'group-hover:text-fg';
          return (
            <div 
              key={device.id} 
              role="button"
              tabIndex={0}
              onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setSelectedDevice(device); } }}
              className={`bg-surface-1 rounded-lg group overflow-hidden border border-line hover:border-primary/35 transition-[transform,border-color,box-shadow] duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-canvas/40 flex flex-col cursor-pointer card-stagger`}
              style={{ '--i': filtered.indexOf(device) } as React.CSSProperties}
              onClick={() => setSelectedDevice(device)}
            >
              <div className="p-6">
                <div className="flex justify-between items-start mb-6">
                  <div className={`bg-inset p-3 rounded-lg ${statusTextColor(status)} ${status === 'up' || status === 'ok' ? 'glow-success' : ''}`}>
                    <span className="material-symbols-outlined text-3xl">{iconForProtocol(device.protocol)}</span>
                  </div>
                  <div className={`flex items-center gap-2 px-3 py-1 rounded-full ${statusBgColor(status)}/10`}>
                    {(status === 'up' || status === 'ok') && <span className={`w-1.5 h-1.5 rounded-full ${statusBgColor(status)}`} />}
                    <span className={`text-xs font-semibold uppercase tracking-wide ${statusTextColor(status)}`}>{status.toUpperCase()}</span>
                  </div>
                </div>
                <h3 className={`font-sans text-xl font-semibold mb-1 ${hoverText} transition-colors`}>{device.name}</h3>
                <code className="text-fg-muted text-xs mb-4 block">{device.protocol === 'http' || device.protocol === 'https' ? `${device.protocol}://${device.ipAddress}` : device.ipAddress}{device.port > 0 && !['http','https'].includes(device.protocol) ? `:${device.port}` : ''}</code>
                <div className="space-y-2 text-xs">
                  <div className="flex justify-between"><span className="text-fg-muted uppercase tracking-wide">Protocol</span><span className="font-semibold">{device.protocol.toUpperCase()}</span></div>
                  <div className="flex justify-between"><span className="text-fg-muted uppercase tracking-wide">Interval</span><span className="font-semibold">{device.interval}s</span></div>
                  {metric && <div className="flex justify-between"><span className="text-fg-muted uppercase tracking-wide">Response</span><span className="font-semibold">{metric.responseTime ?? '-'}ms</span></div>}
                </div>
              </div>
              <div className="mt-auto bg-surface-1 p-4 flex justify-between items-center">
                <span className="text-[10px] text-fg-muted uppercase">{metric ? new Date(metric.timestamp || metric.createdAt).toLocaleTimeString() : 'No data'}</span>
                <button 
                  onClick={(e) => { e.stopPropagation(); handleDelete(device); }} 
                  className="text-error text-[10px] font-semibold uppercase tracking-wide hover:bg-error/10 px-2 py-1 rounded transition-colors"
                >
                  Delete
                </button>
              </div>
            </div>
          );
        })}

        {/* Add New Card */}
        <div onClick={() => setShowForm(true)} role="button" tabIndex={0} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setShowForm(true); } }} className="bg-surface-1 rounded-lg border-2 border-dashed border-line hover:border-line-strong/50 transition-colors duration-200 duration-300 flex flex-col items-center justify-center p-12 text-center group cursor-pointer min-h-[200px]">
          <div className="w-16 h-16 rounded-full bg-surface-1 flex items-center justify-center mb-4 group-hover:bg-primary/10 transition-colors">
            <span className="material-symbols-outlined text-3xl text-fg-subtle group-hover:text-fg">add</span>
          </div>
          <h3 className="font-sans text-lg font-semibold text-fg-subtle group-hover:text-fg tracking-wide">Add Device</h3>
          <p className="text-fg-muted text-xs max-w-[120px] mx-auto mt-2">Deploy a new probe or add an existing node</p>
        </div>
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
        title="Delete Device"
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
