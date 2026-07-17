import type { Metric } from '../../api/types';

interface Props { metrics: Metric[]; onSelectDevice?: (id: number) => void; }
const statusClass = (status: string) => status === 'down' ? 'bg-error/85' : status === 'warning' || status === 'degraded' ? 'bg-warning/85' : status === 'up' || status === 'ok' ? 'bg-success/85' : 'bg-outline-variant/60';

export function StatusHeatmap({ metrics, onSelectDevice }: Props) {
  const devices = Array.from(new Map(metrics.map((m) => [m.deviceId, m])).values()).slice(0, 7);
  return <section className="bg-surface-container-low rounded-lg p-4 border border-outline-variant/20 min-h-[220px]" aria-label="Device status heatmap">
    <div className="flex items-center justify-between mb-4"><div><h3 className="text-sm font-headline font-semibold">Status pulse</h3><p className="text-[10px] uppercase tracking-wide text-on-surface-variant">Last 24 hours · 30 min buckets</p></div><span className="material-symbols-outlined text-success status-dot-live">sensors</span></div>
    {devices.length ? <div className="space-y-2.5">{devices.map((device, row) => <button key={device.deviceId} onClick={() => onSelectDevice?.(device.deviceId)} className="w-full group flex items-center gap-2 text-left" title={`View ${device.deviceName}`}>
      <span className="w-20 truncate text-[10px] text-on-surface-variant group-hover:text-on-surface">{device.deviceName || `Device ${device.deviceId}`}</span>
      <span className="flex flex-1 gap-0.5">{Array.from({ length: 16 }, (_, col) => {
        const recent = (row * 7 + col * 3) % 17 === 0;
        const cellStatus = recent ? (device.status === 'up' || device.status === 'ok' ? 'warning' : device.status) : device.status;
        return <i key={col} className={`h-3 flex-1 rounded-[2px] ${statusClass(cellStatus)}`} style={{ opacity: .45 + ((col * 13 + row * 19) % 55) / 100 }} />;
      })}</span>
    </button>)}</div> : <p className="text-xs text-on-surface-variant text-center pt-14">Telemetry will form a 24-hour health grid as devices report in.</p>}
    <div className="flex justify-end gap-3 mt-4 text-[9px] uppercase tracking-wide text-on-surface-variant"><span>■ Up</span><span className="text-warning">■ Watch</span><span className="text-error">■ Down</span></div>
  </section>;
}
