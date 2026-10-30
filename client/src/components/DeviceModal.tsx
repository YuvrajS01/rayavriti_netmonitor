import { useState } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend, CartesianGrid } from 'recharts';
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
import { AXIS_TICK_STYLE, GRID_COLOR, LEGEND_STYLE, SERIES_COLORS, TOOLTIP_ITEM_STYLE, TOOLTIP_LABEL_STYLE, TOOLTIP_STYLE, legendFormatter } from '../utils/chartConfig';
import { statusMeta } from '../utils/status';
import { useChartAnimation } from '../hooks/useReducedMotion';
import RingGauge from './ui/RingGauge';
import Dialog from './ui/Dialog';
import Button from './ui/Button';
import { StatusPill } from './ui/Status';

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
  const anim = useChartAnimation();
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

  const statusMetaNow = statusMeta(latestMetric?.status);
  const inputClass = 'w-full rounded-md border border-line bg-inset px-3 py-2 text-sm text-fg outline-none focus:border-primary';
  const address = `${device.protocol === 'http' || device.protocol === 'https' ? `${device.protocol}://${device.ipAddress}` : device.ipAddress}${device.port > 0 && !['http', 'https'].includes(device.protocol) ? `:${device.port}` : ''}`;

  return (
    <>
      <Dialog
        open
        onClose={onClose}
        size="xl"
        title={device.name}
        description={<span className="font-mono">{address} · <span className="uppercase">{device.protocol}</span></span>}
        headerActions={
          <div className="flex items-center gap-2 pr-1" title={healthScore == null ? 'No health score has been computed for this device yet' : undefined}>
            {healthScore != null ? (
              <RingGauge value={healthScore} size={48} strokeWidth={5} decimals={0} valueClassName="text-sm" label="AI health" />
            ) : (
              <span className="flex h-12 w-12 items-center justify-center rounded-full border border-line text-sm text-fg-muted" aria-label="AI health: not computed yet">—</span>
            )}
            <span className="hidden text-xs text-fg-muted sm:block">AI health</span>
          </div>
        }
        footer={
          <>
            <Button variant="danger-outline" icon="delete" onClick={handleDelete} className="mr-auto">Delete device</Button>
            <Button variant="secondary" onClick={onClose}>Close</Button>
          </>
        }
      >
        {/* Summary */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-6">
          <div className="bg-surface-2 p-4 rounded-md">
            <p className="text-xs text-fg-muted mb-1.5">Status</p>
            <StatusPill status={statusMetaNow} size="md" />
          </div>
          <div className="bg-surface-2 p-4 rounded-md">
            <p className="text-xs text-fg-muted mb-1">Response</p>
            <p className="font-mono font-semibold text-fg">{latestMetric?.responseTime ?? '—'} ms</p>
          </div>
          <div className="bg-surface-2 p-4 rounded-md">
            <p className="text-xs text-fg-muted mb-1">Interval</p>
            <p className="font-mono font-semibold text-fg">{device.interval}s</p>
          </div>
          <div className="bg-surface-2 p-4 rounded-md">
            <p className="text-xs text-fg-muted mb-1">Protocol</p>
            <p className="font-semibold text-fg uppercase">{device.protocol}</p>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          <div>
            <label htmlFor={`dev-${device.id}-location`} className="block text-xs text-fg-muted mb-1.5">Location</label>
            <select
              id={`dev-${device.id}-location`}
              value={locationId}
              onChange={(e) => handleLocationChange(e.target.value)}
              className={`${inputClass} cursor-pointer`}
            >
              <option value="">Unassigned</option>
              {locations.map((loc) => (
                <option key={Number(loc.id)} value={String(loc.id)}>
                  {String(loc.name)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor={`dev-${device.id}-parent`} className="block text-xs text-fg-muted mb-1.5">Parent device (dependency)</label>
            <select
              id={`dev-${device.id}-parent`}
              value={parentDeviceId}
              onChange={(e) => handleParentChange(e.target.value)}
              className={`${inputClass} cursor-pointer`}
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
          </div>
          <div>
            <label htmlFor={`dev-${device.id}-port`} className="block text-xs text-fg-muted mb-1.5">Dependency port</label>
            <input
              id={`dev-${device.id}-port`}
              value={dependencyPort}
              onChange={(e) => handleDependencyPortChange(e.target.value)}
              placeholder="e.g. eth0, 1/0/1"
              className={`${inputClass} font-mono`}
            />
          </div>
        </div>

        {(device.manufacturer || device.model || device.deviceCategory) && (
          <dl className="mb-6 border-y border-line py-4 grid grid-cols-2 md:grid-cols-3 gap-4">
            {device.manufacturer && <div><dt className="text-xs text-fg-muted">Vendor</dt><dd className="mt-1 font-semibold text-sm">{device.manufacturer}</dd></div>}
            {device.model && <div><dt className="text-xs text-fg-muted">Model</dt><dd className="mt-1 font-semibold text-sm">{device.model}</dd></div>}
            {device.deviceCategory && <div><dt className="text-xs text-fg-muted">Profile</dt><dd className="mt-1 font-semibold text-sm capitalize">{device.deviceCategory}</dd></div>}
          </dl>
        )}

        <section aria-labelledby={`dev-${device.id}-ports`} className="rounded-lg border border-line p-4 mb-6">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
            <div>
              <h3 id={`dev-${device.id}-ports`} className="text-sm font-semibold">Port inventory</h3>
              <p className="text-xs text-fg-muted mt-1">{openPorts.length} open of {ports.length || 0} scanned ports</p>
            </div>
            <Button size="sm" variant="secondary" icon={scanning ? 'hourglass_top' : 'radar'} onClick={handleScanPorts} disabled={scanning}>
              {scanning ? 'Scanning…' : 'Scan ports'}
            </Button>
          </div>
          {ports.length === 0 ? (
            <div className="py-8 text-center text-sm text-fg-muted">No port scan results yet</div>
          ) : (
            <ul className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {ports.slice(0, 16).map((port) => {
                const service = port.service || 'Unknown';
                const response = port.responseTime;
                const isOpen = port.state === 'open';
                return (
                  <li key={port.port} className="flex items-center justify-between rounded-md border border-line bg-surface-2 px-3 py-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <span className={`material-symbols-outlined text-base ${isOpen ? 'text-fg' : 'text-fg-subtle'}`} aria-hidden="true">{isOpen ? 'lock_open' : 'lock'}</span>
                      <div className="min-w-0">
                        <p className="font-mono text-sm font-semibold text-fg">{port.port}</p>
                        <p className="text-xs text-fg-muted truncate">{service}</p>
                      </div>
                    </div>
                    <div className="text-right">
                      <p className={`capitalize text-xs font-semibold ${isOpen ? 'text-fg' : 'text-fg-subtle'}`}>{port.state}</p>
                      {typeof response === 'number' && <p className="font-mono text-xs text-fg-muted">{response}ms</p>}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        <section aria-labelledby={`dev-${device.id}-resp`} className="rounded-lg border border-line p-4 mb-6">
          <h3 id={`dev-${device.id}-resp`} className="text-sm font-semibold mb-4">Live response time</h3>
          {loading ? (
            <div className="h-64 flex items-center justify-center text-fg-muted text-sm">Loading data…</div>
          ) : chartData.length === 0 ? (
            <div className="h-64 flex items-center justify-center text-fg-muted text-sm">No metrics available</div>
          ) : (
            <ResponsiveContainer width="100%" height={256}>
              <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                <CartesianGrid stroke={GRID_COLOR} vertical={false} />
                <XAxis dataKey="time" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} />
                <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}ms`} />
                <Tooltip contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} />
                <Line name="Response" type="monotone" dataKey="response" stroke={SERIES_COLORS[0]} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls {...anim} />
              </LineChart>
            </ResponsiveContainer>
          )}
        </section>

        {supportsTraffic && (
          <section aria-labelledby={`dev-${device.id}-traffic`} className="rounded-lg border border-line p-4">
            <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-4 mb-4">
              <div>
                <h3 id={`dev-${device.id}-traffic`} className="text-sm font-semibold">Live traffic</h3>
                <p className="text-xs text-fg-muted mt-1">{activeInterfaces.length ? `${activeInterfaces.length} SNMP interfaces reporting counters` : 'Waiting for SNMP interface counters'}</p>
              </div>
              <dl className="grid grid-cols-3 gap-2 min-w-full lg:min-w-[320px]">
                <div className="rounded-md bg-surface-2 px-3 py-2">
                  <dt className="text-xs text-fg-muted">Inbound</dt>
                  <dd className="font-mono text-sm font-semibold text-fg">{formatMbps(latestTraffic?.inMbps)}</dd>
                </div>
                <div className="rounded-md bg-surface-2 px-3 py-2">
                  <dt className="text-xs text-fg-muted">Outbound</dt>
                  <dd className="font-mono text-sm font-semibold text-fg">{formatMbps(latestTraffic?.outMbps)}</dd>
                </div>
                <div className="rounded-md bg-surface-2 px-3 py-2">
                  <dt className="text-xs text-fg-muted">Total</dt>
                  <dd className="font-mono text-sm font-semibold text-fg">{formatMbps(latestTraffic?.totalMbps)}</dd>
                </div>
              </dl>
            </div>
            {loading ? (
              <div className="h-64 flex items-center justify-center text-fg-muted text-sm">Loading traffic…</div>
            ) : trafficData.length === 0 ? (
              <div className="h-64 flex flex-col items-center justify-center text-center text-fg-muted text-sm px-6">
                <span className="material-symbols-outlined text-3xl mb-2 text-fg-subtle" aria-hidden="true">monitoring</span>
                <p>No traffic samples yet</p>
                <p className="text-xs mt-1 max-w-md">SNMP routers and switches need at least two successful polls with interface counters before rates can be calculated.</p>
              </div>
            ) : (
              <ResponsiveContainer width="100%" height={256}>
                <LineChart data={trafficData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                  <CartesianGrid stroke={GRID_COLOR} vertical={false} />
                  <XAxis dataKey="time" tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} />
                  <YAxis tick={AXIS_TICK_STYLE} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}M`} />
                  <Tooltip formatter={(value) => formatMbps(Number(value))} contentStyle={TOOLTIP_STYLE} labelStyle={TOOLTIP_LABEL_STYLE} itemStyle={TOOLTIP_ITEM_STYLE} />
                  <Legend wrapperStyle={LEGEND_STYLE} formatter={legendFormatter} />
                  <Line name="Inbound" type="monotone" dataKey="inMbps" stroke={SERIES_COLORS[0]} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls {...anim} />
                  <Line name="Outbound" type="monotone" dataKey="outMbps" stroke={SERIES_COLORS[1]} strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls {...anim} />
                </LineChart>
              </ResponsiveContainer>
            )}
            {activeInterfaces.length > 0 && (
              <ul className="mt-4 grid grid-cols-1 sm:grid-cols-2 gap-2">
                {activeInterfaces.slice(0, 6).map((iface) => (
                  <li key={iface.index} className="flex items-center justify-between rounded-md bg-surface-2 border border-line px-3 py-2">
                    <div className="min-w-0">
                      <p className="text-xs font-semibold text-fg truncate">{iface.name}</p>
                      <p className="font-mono text-xs text-fg-muted">ifIndex {iface.index}</p>
                    </div>
                    {iface.operStatus === 1 ? <StatusPill status="up" /> : <StatusPill status="unknown">Seen</StatusPill>}
                  </li>
                ))}
              </ul>
            )}
          </section>
        )}
      </Dialog>
      <ConfirmDialog
        open={showDeleteConfirm}
        title="Delete device?"
        message={`Are you sure you want to delete "${device.name}"? This action cannot be undone.`}
        confirmLabel="Delete"
        danger
        onConfirm={confirmDelete}
        onCancel={() => setShowDeleteConfirm(false)}
      />
    </>
  );
}
