import { useState, useEffect, useCallback } from 'react';
import { LineChart, Line, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { getDeviceMetrics, deleteDevice, getDevicePorts, scanDevicePorts } from '../api/client';
import { useSocket } from '../hooks/useSocket';
import type { Device, Metric, PortScanResult } from '../api/types';

export default function DeviceModal({ device, onClose, onDeleted }: { device: Device; onClose: () => void; onDeleted: () => void }) {
  const [metrics, setMetrics] = useState<Metric[]>([]);
  const [ports, setPorts] = useState<PortScanResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [scanning, setScanning] = useState(false);

  const loadData = useCallback(async () => {
    try {
      const [metricRes, portRes] = await Promise.all([
        getDeviceMetrics(device.id, 50),
        getDevicePorts(device.id)
      ]);
      setMetrics((metricRes.data || []).reverse()); // Oldest to newest for the chart
      setPorts(portRes.data || []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [device.id]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  useSocket({
    onMetricUpdate: (metric: Record<string, unknown>) => {
      const m = metric as unknown as Metric;
      if (m.device_id === device.id) {
        setMetrics((prev) => {
          const updated = [...prev, m as Metric];
          if (updated.length > 50) updated.shift();
          return updated;
        });
      }
    }
  });

  const handleDelete = async () => {
    if (!confirm('Delete this device?')) return;
    await deleteDevice(device.id);
    onDeleted();
  };

  const handleScanPorts = async () => {
    setScanning(true);
    try {
      const res = await scanDevicePorts(device.id);
      setPorts(res.data.results.map((result) => ({
        ...result,
        device_id: device.id,
        service_guess: result.serviceGuess,
        response_time: result.responseTime
      })));
    } finally {
      setScanning(false);
    }
  };

  const chartData = metrics.map((m) => ({
    time: new Date(m.timestamp || m.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    response: m.response_time ?? 0,
    status: m.status
  }));

  const latestMetric = metrics[metrics.length - 1];
  const openPorts = ports.filter((port) => port.status === 'open');

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm" onClick={onClose}>
      <div 
        className="bg-surface-container-low border border-outline-variant/30 rounded-xl w-full max-w-3xl overflow-hidden shadow-2xl flex flex-col"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-6 border-b border-outline-variant/20 flex justify-between items-start">
          <div>
            <h2 className="font-headline text-3xl font-black text-on-surface uppercase tracking-tight">{device.name}</h2>
            <p className="text-on-surface-variant text-sm font-mono">{device.host}:{device.port} ({device.protocol.toUpperCase()})</p>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-surface-container-highest rounded-full transition-colors">
            <span className="material-symbols-outlined text-outline hover:text-on-surface">close</span>
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto max-h-[70vh]">
           {/* Summary Stats */}
           <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
             <div className="bg-surface-container-high p-4 rounded-lg">
               <p className="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1">Status</p>
               <p className={`font-bold uppercase ${latestMetric?.status === 'down' ? 'text-error' : (latestMetric?.status === 'warning' || latestMetric?.status === 'degraded' ? 'text-amber-500' : 'text-primary')}`}>{latestMetric?.status || 'UNKNOWN'}</p>
             </div>
             <div className="bg-surface-container-high p-4 rounded-lg">
               <p className="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1">Response</p>
               <p className="font-bold text-on-surface">{latestMetric?.response_time ?? '-'} ms</p>
             </div>
             <div className="bg-surface-container-high p-4 rounded-lg">
               <p className="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1">Interval</p>
               <p className="font-bold text-on-surface">{device.interval_seconds}s</p>
             </div>
             <div className="bg-surface-container-high p-4 rounded-lg">
               <p className="text-[10px] text-on-surface-variant uppercase tracking-widest mb-1">Protocol</p>
               <p className="font-bold text-on-surface uppercase">{device.protocol}</p>
             </div>
           </div>

           <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 mb-6">
             <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 mb-4">
               <div>
                 <h3 className="text-sm font-headline font-bold uppercase tracking-widest">Port Inventory</h3>
                 <p className="text-xs text-on-surface-variant mt-1">{openPorts.length} open of {ports.length || 0} scanned ports</p>
               </div>
               <button
                 onClick={handleScanPorts}
                 disabled={scanning}
                 className="bg-primary text-on-primary disabled:opacity-60 font-bold py-2.5 px-4 rounded-lg tracking-widest uppercase hover:brightness-110 active:scale-95 transition-all text-xs flex items-center justify-center gap-2"
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
                   const service = port.service_guess || port.serviceGuess || 'Unknown';
                   const response = port.response_time ?? port.responseTime;
                   const isOpen = port.status === 'open';
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
                         <p className={`text-[10px] font-bold uppercase tracking-widest ${isOpen ? 'text-primary' : 'text-outline'}`}>{port.status}</p>
                         {typeof response === 'number' && <p className="text-[10px] text-on-surface-variant">{response}ms</p>}
                       </div>
                     </div>
                   );
                 })}
               </div>
             )}
           </div>

           {/* Graph */}
           <div className="bg-surface-container-high rounded-xl p-4 border border-outline-variant/20 mb-6">
             <h3 className="text-sm font-headline font-bold mb-4 uppercase tracking-widest">Live Response Time</h3>
             {loading ? (
               <div className="h-64 flex items-center justify-center text-on-surface-variant text-sm animate-pulse">Loading data...</div>
             ) : chartData.length === 0 ? (
               <div className="h-64 flex items-center justify-center text-on-surface-variant text-sm">No metrics available</div>
             ) : (
               <ResponsiveContainer width="100%" height={256}>
                 <LineChart data={chartData} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                   <XAxis dataKey="time" tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} />
                   <YAxis tick={{ fill: '#8a8a78', fontSize: 10 }} tickLine={false} axisLine={false} tickFormatter={(v) => `${v}ms`} />
                   <Tooltip
                     contentStyle={{ background: '#1a1a13', border: '1px solid #494840', borderRadius: '8px', fontSize: '12px', color: '#f4f1e6' }}
                   />
                   <Line type="monotone" dataKey="response" stroke="#d9fd3a" strokeWidth={2} dot={false} activeDot={{ r: 4 }} connectNulls />
                 </LineChart>
               </ResponsiveContainer>
             )}
           </div>

           {/* Actions */}
           <div className="flex gap-4">
             <button onClick={handleDelete} className="bg-error/10 text-error border border-error/30 font-bold py-3 px-6 rounded-lg tracking-widest uppercase hover:bg-error/20 active:scale-95 transition-all w-full">
               Delete Device
             </button>
           </div>
        </div>
      </div>
    </div>
  );
}
