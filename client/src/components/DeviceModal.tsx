import { useState, useEffect, useCallback, useRef } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip, Legend } from 'recharts';
import { getDeviceMetrics, deleteDevice, getDevicePorts, scanDevicePorts } from '../api/client';
import { listPhase2, type Phase2Row } from '../api/phase2';
import { v1 } from '../api/http';
import { useSocket } from '../hooks/useSocket';
import type { Device, Metric, MetricMessagePayload, PortScanResult, TrafficInterfaceSample } from '../api/types';
import ConfirmDialog from './ConfirmDialog';
import { formatMbps } from '../utils/formatters';
import { TOOLTIP_STYLE } from '../utils/chartConfig';

interface TrafficPoint {
  time: string;
  inMbps: number;
  outMbps: number;
  totalMbps: number;
}

function parseMetricMessage(details?: Record<string, unknown> | null): MetricMessagePayload | null {
  if (!details) return null;
  try {
    return details as unknown as MetricMessagePayload;
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
    const prevPayload = parseMetricMessage(prev.details as Record<string, unknown> | null);
    const currPayload = parseMetricMessage(curr.details as Record<string, unknown> | null);
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

export default function DeviceModal({ device, onClose, onDeleted }: { device: Device; onClose: () => void; onDeleted: () => void }) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [ports, setPorts] = useState<PortScanResult[]>([]);
  const [locations, setLocations] = useState<Phase2Row[]>([]);
  const [locationId, setLocationId] = useState<string>(device.locationId != null ? String(device.locationId) : '');
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

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
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
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

  const loadData = useCallback(async () => {
    try {
      const [metricRes, portRes, locRes] = await Promise.all([
        getDeviceMetrics(device.id, 50),
        getDevicePorts(device.id),
        listPhase2('/locations')
      ]);
      setMetrics((metricRes.data || []).reverse());
      setPorts(portRes.data || []);
      setLocations(locRes.data || []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [device.id]);

  useEffect(() => {
    void Promise.resolve().then(loadData);
  }, [loadData]);

  useSocket({
    onMetricUpdate: (metric: Record<string, unknown>) => {
      const m = metric as unknown as Metric;
      if (m.deviceId === device.id) {
        setMetrics((prev) => {
          const updated = [...prev, m as Metric];
          if (updated.length > 50) updated.shift();
          return updated;
        });
      }
    }
  });

  const handleDelete = async () => {
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
      setPorts(res.data.results.map((result) => ({
        ...result,
        deviceId: device.id,
      })));
    } finally {
      setScanning(false);
    }
  };

  const handleLocationChange = async (newLocId: string) => {
    setLocationId(newLocId);
    try {
      await v1.put(`/devices/${device.id}`, {
        locationId: newLocId ? Number(newLocId) : null,
      });
    } catch {
      setLocationId(device.locationId != null ? String(device.locationId) : '');
    }
  };

  const chartData = metrics.map((m) => ({
    time: new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    response: m.responseTime ?? 0,
    status: m.status
  }));

  const latestMetric = metrics[metrics.length - 1];
  const latestPayload = parseMetricMessage(latestMetric?.details as Record<string, unknown> | null);
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
        className="bg-surface-container-low border border-outline-variant/30 rounded-lg w-full max-w-3xl max-h-[90vh] overflow-hidden flex flex-col outline-none"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-6 border-b border-outline-variant/20 flex justify-between items-start shrink-0">
          <div>
            <h2 className="font-headline text-3xl font-bold text-on-surface uppercase tracking-tight">{device.name}</h2>
            <p className="text-on-surface-variant text-sm font-mono">{device.protocol === 'http' || device.protocol === 'https' ? `${device.protocol}://${device.ipAddress}` : device.ipAddress}{device.port > 0 && !['http','https'].includes(device.protocol) ? `:${device.port}` : ''} ({device.protocol.toUpperCase()})</p>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-surface-container-highest rounded-full transition-colors" aria-label="Close dialog">
            <span className="material-symbols-outlined text-outline hover:text-on-surface">close</span>
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto flex-1 min-h-0">
           {/* Summary Stats */}
           <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
             <div className="bg-surface-container-high p-4 rounded-lg">
               <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Status</p>
               <p className={`font-bold uppercase ${latestMetric?.status === 'down' ? 'text-error' : (latestMetric?.status === 'warning' || latestMetric?.status === 'degraded' ? 'text-warning' : 'text-primary')}`}>{latestMetric?.status || 'UNKNOWN'}</p>
             </div>
             <div className="bg-surface-container-high p-4 rounded-lg">
               <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Response</p>
                <p className="font-bold text-on-surface">{latestMetric?.responseTime ?? '-'} ms</p>
             </div>
             <div className="bg-surface-container-high p-4 rounded-lg">
               <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Interval</p>
                <p className="font-bold text-on-surface">{device.interval}s</p>
             </div>
             <div className="bg-surface-container-high p-4 rounded-lg">
               <p className="text-[10px] text-on-surface-variant uppercase tracking-wide mb-1">Protocol</p>
                <p className="font-bold text-on-surface uppercase">{device.protocol}</p>
             </div>
           </div>

           <div className="bg-surface-container-high p-4 rounded-lg mb-6">
             <label className="block text-[10px] text-on-surface-variant uppercase tracking-wide mb-1.5">Location</label>
             <select
               value={locationId}
               onChange={(e) => handleLocationChange(e.target.value)}
               className="bg-surface-container-highest border border-outline-variant/20 rounded-lg px-3 py-2 text-sm text-on-surface outline-none focus:ring-1 focus:ring-primary w-full cursor-pointer"
             >
               <option value="">Unassigned</option>
               {locations.map((loc) => (
                 <option key={Number(loc.id)} value={String(loc.id)}>
                   {String(loc.name)}
                 </option>
               ))}
             </select>
           </div>

           <div className="bg-surface-container-high rounded-lg p-4 border border-outline-variant/20 mb-6">
             <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
               <div>
                 <h3 className="text-sm font-headline font-bold uppercase tracking-wide">Port Inventory</h3>
                 <p className="text-xs text-on-surface-variant mt-1">{openPorts.length} open of {ports.length || 0} scanned ports</p>
               </div>
               <button
                 onClick={handleScanPorts}
                 disabled={scanning}
                  className="bg-primary text-on-primary disabled:opacity-60 font-bold py-2.5 px-4 rounded-lg tracking-wide uppercase hover:bg-primary/90 transition-[background-color] text-xs flex items-center justify-center gap-2"
               >
                 <span className="material-symbols-outlined text-base">{scanning ? 'hourglass_top' : 'radar'}</span>
                 {scanning ? 'Scanning' : 'Scan Ports'}
               </button>
             </div>
             {ports.length === 0 ? (
               <div className="py-8 text-center text-xs text-on-surface-variant">No port scan results yet</div>
             ) : (
               <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                 {ports.slice(0, 16).map((port) => {
                    const service = port.service || 'Unknown';
                    const response = port.responseTime;
                   const isOpen = port.state === 'open';
                   return (
                     <div key={port.port} className={`flex items-center justify-between rounded-lg px-3 py-2 border ${isOpen ? 'border-primary/25 bg-primary/10' : 'border-outline-variant/15 bg-surface-container-low'}`}>
                       <div className="flex items-center gap-2 min-w-0">
                         <span className={`material-symbols-outlined text-base ${isOpen ? 'text-primary' : 'text-outline'}`}>{isOpen ? 'lock_open' : 'lock'}</span>
                         <div className="min-w-0">
                           <p className="text-sm font-bold text-on-surface">{port.port}</p>
                           <p className="text-[10px] text-on-surface-variant truncate">{service}</p>
                         </div>
                       </div>
                       <div className="text-right">
                         <p className={`text-[10px] font-bold uppercase tracking-wide ${isOpen ? 'text-primary' : 'text-outline'}`}>{port.state}</p>
                         {typeof response === 'number' && <p className="text-[10px] text-on-surface-variant">{response}ms</p>}
                       </div>
                     </div>
                   );
                 })}
               </div>
             )}
           </div>

           {/* Graph */}
           <div className="bg-surface-container-high rounded-lg p-4 border border-outline-variant/20 mb-6">
             <h3 className="text-sm font-headline font-bold mb-4 uppercase tracking-wide">Live Response Time</h3>
             {loading ? (
               <div className="h-64 flex items-center justify-center text-on-surface-variant text-sm">Loading data...</div>
             ) : chartData.length === 0 ? (
               <div className="h-64 flex items-center justify-center text-on-surface-variant text-sm">No metrics available</div>
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
              <div className="bg-surface-container-high rounded-lg p-4 border border-outline-variant/20 mb-6">
                <div className="flex flex-col lg:flex-row lg:items-start justify-between gap-4 mb-4">
                  <div>
                    <h3 className="text-sm font-headline font-bold uppercase tracking-wide">Live Traffic</h3>
                    <p className="text-xs text-on-surface-variant mt-1">{activeInterfaces.length ? `${activeInterfaces.length} SNMP interfaces reporting counters` : 'Waiting for SNMP interface counters'}</p>
                  </div>
                  <div className="grid grid-cols-3 gap-2 min-w-full lg:min-w-[320px]">
                    <div className="rounded-lg bg-surface-container-low px-3 py-2">
                      <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Inbound</p>
                      <p className="text-sm font-bold text-primary">{formatMbps(latestTraffic?.inMbps)}</p>
                    </div>
                    <div className="rounded-lg bg-surface-container-low px-3 py-2">
                      <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Outbound</p>
                      <p className="text-sm font-bold text-info">{formatMbps(latestTraffic?.outMbps)}</p>
                    </div>
                    <div className="rounded-lg bg-surface-container-low px-3 py-2">
                      <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">Total</p>
                      <p className="text-sm font-bold text-on-surface">{formatMbps(latestTraffic?.totalMbps)}</p>
                    </div>
                  </div>
                </div>
                {loading ? (
                  <div className="h-64 flex items-center justify-center text-on-surface-variant text-sm">Loading traffic...</div>
                ) : trafficData.length === 0 ? (
                  <div className="h-64 flex flex-col items-center justify-center text-center text-on-surface-variant text-sm px-6">
                    <span className="material-symbols-outlined text-3xl mb-2 text-outline">monitoring</span>
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
                     <div key={iface.index} className="flex items-center justify-between rounded-lg bg-surface-container-low border border-outline-variant/15 px-3 py-2">
                       <div className="min-w-0">
                         <p className="text-xs font-bold text-on-surface truncate">{iface.name}</p>
                         <p className="text-[10px] text-on-surface-variant uppercase tracking-wide">ifIndex {iface.index}</p>
                       </div>
                       <span className={`text-[10px] font-bold uppercase tracking-wide ${iface.operStatus === 1 ? 'text-primary' : 'text-outline'}`}>
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
              <button onClick={handleDelete} className="bg-error/10 text-error border border-error/30 font-bold py-3 px-6 rounded-lg tracking-wide uppercase hover:bg-error/20 transition-[background-color] w-full">
               Delete Device
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
