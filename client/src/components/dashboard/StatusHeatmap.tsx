import { useMemo } from 'react';
import type { Metric } from '../../api/types';

interface Props { metrics: Metric[]; onSelectDevice?: (id: number) => void; }

const CELLS = 16;
const MAX_DEVICES = 7;

const statusClass = (status: string) => status === 'down' ? 'bg-error/85' : status === 'warning' || status === 'degraded' ? 'bg-warning/85' : status === 'up' || status === 'ok' ? 'bg-success/85' : 'bg-outline-variant/60';

// One cell per real poll result, oldest left. Devices that have only been
// polled a few times this session show empty placeholder cells.
export function StatusHeatmap({ metrics, onSelectDevice }: Props) {
  const rows = useMemo(() => {
    const byDevice = new Map<number, Metric[]>();
    for (const m of metrics) {
      const list = byDevice.get(m.deviceId) ?? [];
      list.push(m);
      byDevice.set(m.deviceId, list);
    }
    const ts = (m: Metric) => new Date(m.timestamp || m.createdAt).getTime();
    return Array.from(byDevice.values())
      .map((list) => list.sort((a, b) => ts(a) - ts(b)).slice(-CELLS))
      .slice(0, MAX_DEVICES);
  }, [metrics]);

  return <section className="bg-surface-container-low rounded-lg p-4 border border-outline-variant/20 min-h-[220px]" aria-label="Recent poll results per device">
    <div className="flex items-center justify-between mb-4"><div><h3 className="text-sm font-headline font-semibold">Status pulse</h3><p className="text-xs text-on-surface-variant">Last {CELLS} polls per device · newest on the right</p></div><span className="material-symbols-outlined text-success status-dot-live">sensors</span></div>
    {rows.length ? <div className="space-y-2.5">{rows.map((polls) => {
      const latest = polls[polls.length - 1];
      return <button key={latest.deviceId} onClick={() => onSelectDevice?.(latest.deviceId)} className="w-full group flex items-center gap-2 text-left" title={`View ${latest.deviceName}`}>
        <span className="w-20 truncate text-xs text-on-surface-variant group-hover:text-on-surface">{latest.deviceName || `Device ${latest.deviceId}`}</span>
        <span className="flex flex-1 gap-0.5">
          {Array.from({ length: CELLS - polls.length }, (_, i) => <i key={`empty-${i}`} className="h-3 flex-1 rounded-[2px] border border-outline-variant/30" />)}
          {polls.map((m, i) => <i key={`${m.timestamp}-${i}`} className={`h-3 flex-1 rounded-[2px] ${statusClass(m.status)}`} title={`${m.status} · ${new Date(m.timestamp || m.createdAt).toLocaleTimeString()}`} />)}
        </span>
      </button>;
    })}</div> : <p className="text-xs text-on-surface-variant text-center pt-14">Poll results will appear here as devices report in.</p>}
    <div className="flex justify-end gap-3 mt-4 text-xs text-on-surface-variant"><span className="text-success">■ Up</span><span className="text-warning">■ Warning</span><span className="text-error">■ Down</span></div>
  </section>;
}
