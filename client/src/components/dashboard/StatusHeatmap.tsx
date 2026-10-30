import { useMemo } from 'react';
import type { Metric } from '../../api/types';
import { STATUS_META, statusMeta } from '../../utils/status';
import { StatusShapeIcon } from '../ui/Status';

interface Props { metrics: Metric[]; onSelectDevice?: (id: number) => void; }

const CELLS = 16;
const MAX_DEVICES = 7;
const LEGEND = ['up', 'warning', 'down', 'unknown'] as const;

// One cell per real poll result, oldest left. Devices that have only been
// polled a few times this session show empty placeholder cells. Each cell
// carries the status shape, so the strip reads without colour.
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

  return (
    <section className="bg-surface-1 rounded-lg p-5 border border-line min-h-[220px]" aria-labelledby="status-pulse-heading">
      <div className="mb-4">
        <h3 id="status-pulse-heading" className="text-sm font-semibold text-fg">Status pulse</h3>
        <p className="text-xs text-fg-muted">Last {CELLS} polls per device · newest on the right</p>
      </div>
      {rows.length ? (
        <div className="space-y-2">
          {rows.map((polls) => {
            const latest = polls[polls.length - 1];
            if (!latest) return null;
            const name = latest.deviceName || `Device ${latest.deviceId}`;
            return (
              <button
                key={latest.deviceId}
                type="button"
                onClick={() => onSelectDevice?.(latest.deviceId)}
                className="w-full group flex items-center gap-2 rounded-sm text-left"
                aria-label={`${name}: latest ${statusMeta(latest.status).label}. View device`}
              >
                <span className="w-24 truncate text-xs text-fg-muted group-hover:text-fg">{name}</span>
                <span className="flex flex-1 gap-0.5" aria-hidden="true">
                  {Array.from({ length: CELLS - polls.length }, (_, i) => (
                    <i key={`empty-${i}`} className="h-4 flex-1 rounded-[2px] border border-line" />
                  ))}
                  {polls.map((m, i) => {
                    const meta = statusMeta(m.status);
                    return (
                      <i
                        key={`${m.timestamp}-${i}`}
                        className={`grid h-4 flex-1 place-items-center rounded-[2px] ${meta.chip} ${meta.text}`}
                        title={`${meta.label} · ${new Date(m.timestamp || m.createdAt).toLocaleTimeString()}`}
                      >
                        <StatusShapeIcon shape={meta.shape} size={7} />
                      </i>
                    );
                  })}
                </span>
              </button>
            );
          })}
        </div>
      ) : (
        <p className="text-xs text-fg-muted text-center pt-14">Poll results will appear here as devices report in.</p>
      )}
      <div className="flex flex-wrap justify-end gap-x-3 gap-y-1 mt-4 text-xs text-fg-muted">
        {LEGEND.map((s) => (
          <span key={s} className="inline-flex items-center gap-1">
            <span className={STATUS_META[s].text}><StatusShapeIcon shape={STATUS_META[s].shape} size={8} /></span>
            {STATUS_META[s].label}
          </span>
        ))}
      </div>
    </section>
  );
}
