import { useState, useEffect, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend } from 'recharts';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getDeviceMetrics, deleteDevice, getDevicePorts, scanDevicePorts, getHealthScore, getDevices } from '../api/client';
import { updateDevice } from '../api/devices';
import { queryKeys } from '../api/queryKeys';
import { useResourceList } from '../hooks/useResources';
import { useSocket } from '../hooks/useSocket';
import type { ResourceRow } from '../api/resources';
import type { Device, Metric, MetricMessagePayload, PortScanResult, TrafficInterfaceSample } from '../api/types';
import ConfirmDialog from './ConfirmDialog';
import { formatMbps } from '../utils/formatters';
import { TOOLTIP_STYLE } from '../utils/chartConfig';
import Sparkline from './ui/Sparkline';
import RingGauge from './ui/RingGauge';

interface TrafficPoint {
  time: string;
  inMbps: number;
  outMbps: number;
  totalMbps: number;
}

function parseMetricMessage(details?: Record<string, unknown> | null): MetricMessagePayload | null {
  if (!details) return null;
  try {
    if (typeof details === 'object' && details !== null && 'interfaces' in details) {
      return details;
    }
    return null;
  } catch {
    return null;
  }
}

function totalOctets(interfaces: TrafficInterfaceSample[] | undefined, key: 'inOctets' | 'outOctets') {
  return (interfaces || []).reduce((sum, iface) => sum + (Number(iface[key]) || 0), 0);
}

function buildTrafficData(metrics: Metric[]): TrafficPoint[] {
  const points: TrafficPoint[] = [];
  for (let i = 1; i < metrics.length; i += 1) {
    const prev = metrics[i - 1];
    const curr = metrics[i];
    if (!prev || !curr) continue;
    const prevPayload = parseMetricMessage(prev.details);
    const currPayload = parseMetricMessage(curr.details);
    if (!prevPayload?.interfaces?.length || !currPayload?.interfaces?.length) continue;

    const seconds = (new Date(curr.timestamp || curr.createdAt).getTime() - new Date(prev.timestamp || prev.createdAt).getTime()) / 1000;
    if (!Number.isFinite(seconds) || seconds <= 0) continue;

    const inDelta = totalOctets(currPayload.interfaces, 'inOctets') - totalOctets(prevPayload.interfaces, 'inOctets');
    const outDelta = totalOctets(currPayload.interfaces, 'outOctets') - totalOctets(prevPayload.interfaces, 'outOctets');
    if (inDelta < 0 || outDelta < 0) continue;

    const inMbps = (inDelta * 8) / seconds / 1_000_000;
    const outMbps = (outDelta * 8) / seconds / 1_000_000;
    points.push({
      time: new Date(curr.timestamp || curr.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
      inMbps: Math.round(inMbps * 100) / 100,
      outMbps: Math.round(outMbps * 100) / 100,
      totalMbps: Math.round((inMbps + outMbps) * 100) / 100
    });
  }
  return points.slice(-50);
}

const METRIC_HISTORY = 50;
const EMPTY_METRICS: Metric[] = [];
const EMPTY_PORTS: PortScanResult[] = [];
const EMPTY_LOCATIONS: ResourceRow[] = [];
const EMPTY_DEVICES: Device[] = [];

export default function DeviceModal({ device, onClose, onDeleted }: { device: Device; onClose: () => void; onDeleted: () => void }) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const queryClient = useQueryClient();
  const [locationId, setLocationId] = useState<string>(device.locationId != null ? String(device.locationId) : '');
  const [parentDeviceId, setParentDeviceId] = useState<string>(device.parentDeviceId != null ? String(device.parentDeviceId) : '');
  const [dependencyPort, setDependencyPort] = useState<string>(device.dependencyPort || '');
  const [scanning, setScanning] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  const metricsKey = queryKeys.devices.metrics(device.id, METRIC_HISTORY);
  // Oldest-first history; live samples are appended to this cache entry below.
  const metricsQuery = useQuery({
    queryKey: metricsKey,
    queryFn: ({ signal }) => getDeviceMetrics(device.id, METRIC_HISTORY, { signal }).then((r) => [...(r.data || [])].reverse()),
  });
  const portsKey = queryKeys.devices.ports(device.id);
  const portsQuery = useQuery({
    queryKey: portsKey,
    queryFn: ({ signal }) => getDevicePorts(device.id, { signal }).then((r) => r.data || []),
  });
  const locationsQuery = useResourceList('/locations');
  const devicesQuery = useQuery({
    queryKey: queryKeys.devices.list(),
    queryFn: ({ signal }) => getDevices({ signal }),
  });
  const healthQuery = useQuery({
    queryKey: queryKeys.devices.health(device.id),
    queryFn: ({ signal }) => getHealthScore(device.id, { signal }),
    retry: false,
  });

  const loading = metricsQuery.isPending || portsQuery.isPending;
  const metrics = metricsQuery.data || EMPTY_METRICS;
  const ports = portsQuery.data || EMPTY_PORTS;
  const locations = locationsQuery.data || EMPTY_LOCATIONS;
  const allDevices = devicesQuery.data?.data || EMPTY_DEVICES;
  const healthScore = healthQuery.data?.data?.score ?? null;

  useEffect(() => {
    previousFocus.current = document.activeElement as HTMLElement;
    dialogRef.current?.focus();

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
        return;
      }
      if (e.key !== 'Tab' || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      previousFocus.current?.focus();
    };
  }, [onClose]);

  useSocket({
    onMetricUpdate: (m) => {
      if (m.deviceId !== device.id) return;
      queryClient.setQueryData<Metric[]>(metricsKey, (prev) => (prev ? [...prev, m].slice(-METRIC_HISTORY) : prev));
    }
  });

  const handleDelete = () => {
    setShowDeleteConfirm(true);
  };

  const confirmDelete = async () => {
    await deleteDevice(device.id);
    setShowDeleteConfirm(false);
    onDeleted();
  };

  const handleScanPorts = async () => {
    setScanning(true);
    try {
      const res = await scanDevicePorts(device.id);
      queryClient.setQueryData<PortScanResult[]>(portsKey, res.data.results.map((result) => ({
        ...result,
        deviceId: device.id,
      })));
    } finally {
      setScanning(false);
    }
  };

  const persistDependencyFields = async (locId: string, parentId: string, port: string) => {
    await updateDevice(device.id, {
      locationId: locId ? Number(locId) : null,
      parentDeviceId: parentId ? Number(parentId) : null,
      dependencyPort: port,
    });
    void queryClient.invalidateQueries({ queryKey: queryKeys.devices.list() });
  };

  const handleLocationChange = async (newLocId: string) => {
    const prev = locationId;
    setLocationId(newLocId);
    try {
      await persistDependencyFields(newLocId, parentDeviceId, dependencyPort);
    } catch {
      setLocationId(device.locationId != null ? String(device.locationId) : prev || '');
    }
  };

  const handleParentChange = async (newParentId: string) => {
    const prev = parentDeviceId;
    setParentDeviceId(newParentId);
    try {
      await persistDependencyFields(locationId, newParentId, dependencyPort);
    } catch {
      setParentDeviceId(device.parentDeviceId != null ? String(device.parentDeviceId) : prev || '');
    }
  };

  const handleDependencyPortChange = async (port: string) => {
    const prev = dependencyPort;
    setDependencyPort(port);
    try {
      await persistDependencyFields(locationId, parentDeviceId, port);
    } catch {
      setDependencyPort(device.dependencyPort || prev || '');
    }
  };

  const chartData = metrics.map((m) => ({
    time: new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    response: m.responseTime ?? 0,
    status: m.status
  }));

  const latestMetric = metrics[metrics.length - 1];
  const latestPayload = parseMetricMessage(latestMetric?.details);
  const trafficData = buildTrafficData(metrics);
  const latestTraffic = trafficData[trafficData.length - 1];
  const supportsTraffic = device.protocol === 'snmp';
  const activeInterfaces = latestPayload?.interfaces || [];
  const openPorts = ports.filter((port) => port.state === 'open');

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/60" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={`Device details for ${device.name}`}
        tabIndex={-1}
        className="bg-surface-1 border border-line rounded-lg w-full max-w-3xl max-h-[90vh] overflow-hidden flex flex-col outline-none"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-6 border-b border-line flex justify-between items-start shrink-0">
          <div>
            <h2 className="font-sans text-3xl font-semibold text-fg">{device.name}</h2>
            <p className="text-fg-muted text-sm font-mono">{device.protocol === 'http' || device.protocol === 'https' ? `${device.protocol}://${device.ipAddress}` : device.ipAddress}{device.port > 0 && !['http','https'].includes(device.protocol) ? `:${device.port}` : ''} ({device.protocol.toUpperCase()})</p>
            <Sparkline data={metrics.map(metric => metric.responseTime ?? 0)} color="var(--color-info)" className="mt-2" />
          </div>
          <div className="flex items-center gap-3">
            <div className="text-center">
              {healthScore != null ? (
                <RingGauge value={healthScore} size={54} strokeWidth={5} decimals={0} valueClassName="text-lg" />
              ) : (
                <span className="flex h-[54px] w-[54px] items-center justify-center rounded-full border border-line text-sm text-fg-muted" title="No health score has been computed for this device yet">—</span>
              )}
              <span className="block text-xs text-fg-muted mt-1">AI health</span>
            </div>
            <button onClick={onClose} className="p-2 hover:bg-inset rounded-full transition-colors" aria-label="Close dialog"><span className="material-symbols-outlined text-fg-subtle hover:text-fg">close</span></button>
          </div>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto flex-1 min-h-0">
           {/* Summary Stats */}
           <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
             <div className="bg-surface-1 p-4 rounded-lg">
               <p className="text-xs text-fg-muted mb-1">Status</p>
               <p className={`font-semibold uppercase ${latestMetric?.status === 'down' ? 'text-error' : (latestMetric?.status === 'warning' || latestMetric?.status === 'degraded' ? 'text-warning' : 'text-primary')}`}>{latestMetric?.status || 'UNKNOWN'}</p>
             </div>
             <div className="bg-surface-1 p-4 rounded-lg">
               <p className="text-xs text-fg-muted mb-1">Response</p>
                <p className="font-semibold text-fg">{latestMetric?.responseTime ?? '-'} ms</p>
             </div>
             <div className="bg-surface-1 p-4 rounded-lg">
               <p className="text-xs text-fg-muted mb-1">Interval</p>
                <p className="font-semibold text-fg">{device.interval}s</p>
             </div>
             <div className="bg-surface-1 p-4 rounded-lg">
               <p className="text-xs text-fg-muted mb-1">Protocol</p>
                <p className="font-semibold text-fg uppercase">{device.protocol}</p>
             </div>
           </div>

           <div className="bg-surface-1 p-4 rounded-lg mb-6">
             <label className="block text-xs text-fg-muted mb-1.5">Location</label>
             <select
               value={locationId}
               onChange={(e) => handleLocationChange(e.target.value)}
               className="bg-inset border border-line rounded-lg px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-primary w-full cursor-pointer"
             >
               <option value="">Unassigned</option>
               {locations.map((loc) => (
                 <option key={Number(loc.id)} value={String(loc.id)}>
                   {String(loc.name)}
                 </option>
               ))}
             </select>
           </div>

            <div className="bg-surface-1 p-4 rounded-lg mb-6">
              <label className="block text-xs text-fg-muted mb-1.5">Parent device (dependency)</label>
              <select
                value={parentDeviceId}
                onChange={(e) => handleParentChange(e.target.value)}
                className="bg-inset border border-line rounded-lg px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-primary w-full cursor-pointer"
              >
                <option value="">None (root)</option>
                {allDevices
                  .filter((d) => d.id !== device.id)
                  .map((d) => (
                    <option key={d.id} value={String(d.id)}>
                      {d.name} ({d.ipAddress})
                    </option>
                  ))}
              </select>
              <label className="block text-xs text-fg-muted mb-1.5 mt-3">Dependency port</label>
              <input
                value={dependencyPort}
                onChange={(e) => handleDependencyPortChange(e.target.value)}
                placeholder="e.g. eth0, 1/0/1"
                className="bg-inset border border-line rounded-lg px-3 py-2 text-sm text-fg outline-none focus:ring-1 focus:ring-primary w-full"
              />
            </div>

		   {(device.manufacturer || device.model || device.deviceCategory) && (
			 <div className="mb-6 border-y border-line py-4 grid grid-cols-2 md:grid-cols-3 gap-4">
			   {device.manufacturer && <div><p className="text-xs text-fg-muted">Vendor</p><p className="mt-1 font-semibold text-sm">{device.manufacturer}</p></div>}
			   {device.model && <div><p className="text-xs text-fg-muted">Model</p><p className="mt-1 font-semibold text-sm">{device.model}</p></div>}
			   {device.deviceCategory && <div><p className="text-xs text-fg-muted">Profile</p><p className="mt-1 font-semibold text-sm capitalize">{device.deviceCategory}</p></div>}
			 </div>
		   )}

           <div className="bg-surface-1 rounded-lg p-4 border border-line mb-6">
             <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
               <div>
                 <h3 className="text-sm font-sans font-semibold">Port inventory</h3>
                 <p className="text-xs text-fg-muted mt-1">{openPorts.length} open of {ports.length || 0} scanned ports</p>
               </div>
               <button
                 onClick={handleScanPorts}
                 disabled={scanning}
                  className="bg-primary text-on-primary disabled:opacity-60 font-semibold py-2.5 px-4 rounded-lg hover:bg-primary/90 transition-colors text-xs flex items-center justify-center gap-2"
               >
                 <span className="material-symbols-outlined text-base">{scanning ? 'hourglass_top' : 'radar'}</span>
                 {scanning ? 'Scanning' : 'Scan Ports'}
               </button>
             </div>
             {ports.length === 0 ? (
               <div className="py-8 text-center text-xs text-fg-muted">No port scan results yet</div>
             ) : (
               <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                 {ports.slice(0, 16).map((port) => {
                    const service = port.service || 'Unknown';
                    const response = port.responseTime;
                   const isOpen = port.state === 'open';
                   return (
                     <div key={port.port} className={`flex items-center justify-between rounded-lg px-3 py-2 border ${isOpen ? 'border-primary/25 bg-primary/10' : 'border-line bg-surface-1'}`}>
                       <div className="flex items-center gap-2 min-w-0">
                         <span className={`material-symbols-outlined text-base ${isOpen ? 'text-primary' : 'text-fg-subtle'}`}>{isOpen ? 'lock_open' : 'lock'}</span>
                         <div className="min-w-0">
                           <p className="text-sm font-semibold text-fg">{port.port}</p>
                           <p className="text-xs text-fg-muted truncate">{service}</p>
                         </div>
                       </div>
                       <div className="text-right">
                         <p className={`capitalize text-xs font-semibold ${isOpen ? 'text-primary': 'text-fg-subtle'}`}>{port.state}</p>
                         {typeof response === 'number' && <p className="text-xs text-fg-muted">{response}ms</p>}
                       </div>
                     </div>
                   );
                 })}
               </div>
             )}
           </div>

           {/* Graph */}
           <div className="bg-surface-1 rounded-lg p-4 border border-line mb-6">
             <h3 className="text-sm font-sans font-semibold mb-4">Live response time</h3>
             {loading ? (
               <div className="h-64 flex items-center justify-center text-fg-muted text-sm">Loading data...</div>
             ) : chartData.length === 0 ? (
               <div className="h-64 flex items-center justify-center text-fg-muted text-sm">No metrics available</div>
             ) : (
               <ResponsiveContainer width="100%" height={256}>
                 <LineChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                   <XAxis dataKey="time" tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} />
                   <YAxis tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}ms`} />
                    <Tooltip
                       contentStyle={TOOLTIP_STYLE}
                     />
                     <Line type="monotone" dataKey="response" stroke="var(--color-primary)" strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </div>

            {supportsTraffic && (
              <div className="bg-surface-1 rounded-lg p-4 border border-line mb-6">
                <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-4 mb-4">
                  <div>
                    <h3 className="text-sm font-sans font-semibold">Live traffic</h3>
                    <p className="text-xs text-fg-muted mt-1">{activeInterfaces.length ? `${activeInterfaces.length} SNMP interfaces reporting counters` : 'Waiting for SNMP interface counters'}</p>
                  </div>
                  <div className="grid grid-cols-3 gap-2 min-w-full lg:min-w-[320px]">
                    <div className="rounded-lg bg-surface-1 px-3 py-2">
                      <p className="text-xs text-fg-muted">Inbound</p>
                      <p className="text-sm font-semibold text-primary">{formatMbps(latestTraffic?.inMbps)}</p>
                    </div>
                    <div className="rounded-lg bg-surface-1 px-3 py-2">
                      <p className="text-xs text-fg-muted">Outbound</p>
                      <p className="text-sm font-semibold text-info">{formatMbps(latestTraffic?.outMbps)}</p>
                    </div>
                    <div className="rounded-lg bg-surface-1 px-3 py-2">
                      <p className="text-xs text-fg-muted">Total</p>
                      <p className="text-sm font-semibold text-fg">{formatMbps(latestTraffic?.totalMbps)}</p>
                    </div>
                  </div>
                </div>
                {loading ? (
                  <div className="h-64 flex items-center justify-center text-fg-muted text-sm">Loading traffic...</div>
                ) : trafficData.length === 0 ? (
                  <div className="h-64 flex flex-col items-center justify-center text-center text-fg-muted text-sm px-6">
                    <span className="material-symbols-outlined text-3xl mb-2 text-fg-subtle">monitoring</span>
                    <p>No traffic samples yet</p>
                    <p className="text-xs mt-1 max-w-md">SNMP routers and switches need at least two successful polls with interface counters before rates can be calculated.</p>
                  </div>
                ) : (
                  <ResponsiveContainer width="100%" height={256}>
                    <LineChart data={trafficData} margin={{ top: 10, right: 10, left: -16, bottom: 0 }}>
                      <XAxis dataKey="time" tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} />
                      <YAxis tick={{ fill: '#77766d', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}M`} />
                       <Tooltip
                         formatter={(value) => formatMbps(Number(value))}
                         contentStyle={TOOLTIP_STYLE}
                       />
                     <Legend wrapperStyle={{ fontSize: 11, color: '#c9c6b8' }} />
                     <Line name="Inbound" type="monotone" dataKey="inMbps" stroke="#d9fd3a" strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls />
                     <Line name="Outbound" type="monotone" dataKey="outMbps" stroke="#7dd3fc" strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls />
                   </LineChart>
                 </ResponsiveContainer>
               )}
               {activeInterfaces.length > 0 && (
                 <div className="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-2">
                   {activeInterfaces.slice(0, 6).map((iface) => (
                     <div key={iface.index} className="flex items-center justify-between rounded-lg bg-surface-1 border border-line px-3 py-2">
                       <div className="min-w-0">
                         <p className="text-xs font-semibold text-fg truncate">{iface.name}</p>
                         <p className="text-xs text-fg-muted">ifIndex {iface.index}</p>
                       </div>
                       <span className={`capitalize text-xs font-semibold ${iface.operStatus === 1 ? 'text-primary': 'text-fg-subtle'}`}>
                         {iface.operStatus === 1 ? 'Up' : 'Seen'}
                       </span>
                     </div>
                   ))}
                 </div>
               )}
             </div>
           )}

           {/* Actions */}
           <div className="flex gap-4">
              <button onClick={handleDelete} className="bg-error/10 text-error border border-error/30 font-semibold py-3 px-6 rounded-lg hover:bg-error/20 transition-colors w-full">
               Delete device
             </button>
           </div>
        </div>
      </div>
      <ConfirmDialog
        open={showDeleteConfirm}
        title="Delete Device"
        message={`Are you sure you want to delete "${device.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        danger
        onConfirm={confirmDelete}
        onCancel={() => setShowDeleteConfirm(false)}
      />
    </div>
  );
}
