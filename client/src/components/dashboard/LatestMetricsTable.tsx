import { memo } from 'react';
import type { Metric } from '../../api/types';
import { iconForProtocol } from '../../utils/icons';

interface Props {
  metrics: Metric[];
}

function LatestMetricsTableInner({ metrics }: Props) {
  const maxResponse = Math.max(1, ...metrics.map((m) => m.responseTime || 0));
  return (
    <div className="bg-surface-container-low rounded-lg p-5 border border-outline-variant/20 flex flex-col">
      <div className="flex items-center gap-2 mb-4">
        <span className="material-symbols-outlined text-on-surface-variant text-xl">speed</span>
        <h3 className="text-sm font-headline font-semibold text-on-surface">Latest Metrics</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="text-xs uppercase tracking-wide text-on-surface-variant border-b border-outline-variant/20">
              <th className="pb-3 font-medium">Device</th>
              <th className="pb-3 font-medium">Protocol</th>
              <th className="pb-3 font-medium">Status</th>
              <th className="pb-3 font-medium text-right">Response</th>
              <th className="pb-3 font-medium text-right">Time</th>
            </tr>
          </thead>
          <tbody className="text-sm">
            {metrics.slice(0, 15).map((m, i) => {
              const isDown = m.status === 'down';
              const isWarn = m.status === 'warning' || m.status === 'degraded';
              const dotColor = isDown ? 'var(--color-error)' : isWarn ? 'var(--color-warning)' : 'var(--color-success)';
              const sc = isDown ? 'text-error bg-error/10 border-error/20' : isWarn ? 'text-warning bg-warning/10 border-warning/20' : 'text-success bg-success/10 border-success/20';
              const statusIcon = isDown ? 'cancel' : isWarn ? 'warning' : 'check_circle';
              const barPct = Math.min(100, ((m.responseTime || 0) / maxResponse) * 100);

              return (
                <tr key={m.id || i} className="border-b border-outline-variant/10 hover:bg-surface-container/50 transition-colors group card-stagger" style={{ '--i': i } as React.CSSProperties}>
                  <td className="py-3 font-body font-medium text-on-surface">{m.deviceName}</td>
                  <td className="py-3 text-on-surface-variant text-xs uppercase tracking-wider">
                    <div className="flex items-center gap-1.5">
                      <span className="material-symbols-outlined text-[14px] opacity-70">
                        {iconForProtocol(m.protocol)}
                      </span>
                      {m.protocol || '-'}
                    </div>
                  </td>
                  <td className="py-3">
                    <div className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full border ${sc} text-xs font-medium uppercase tracking-wide`}>
                      <span className="material-symbols-outlined text-[14px]">{statusIcon}</span>
                      <span className={`w-1.5 h-1.5 rounded-full ${isDown ? 'glow-error' : ''}`} style={{ background: dotColor, boxShadow: isDown ? undefined : `0 0 6px ${dotColor}` }} />
                      {m.status}
                    </div>
                  </td>
                  <td className="py-3 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <span className="font-data text-on-surface">{m.responseTime ?? '-'}ms</span>
                      <span className="hidden sm:block h-1.5 w-16 rounded-full bg-surface-container overflow-hidden">
                        <span className="block h-full rounded-full transition-[width] duration-500" style={{ width: `${barPct}%`, background: dotColor }} />
                      </span>
                    </div>
                  </td>
                  <td className="py-3 text-right text-xs text-on-surface-variant font-data">{new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {metrics.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12">
            <span className="material-symbols-outlined text-4xl mb-2">monitoring</span>
            <p className="text-xs text-on-surface-variant uppercase tracking-wide">No metrics data yet</p>
          </div>
        )}
      </div>
    </div>
  );
}

export const LatestMetricsTable = memo(LatestMetricsTableInner);
