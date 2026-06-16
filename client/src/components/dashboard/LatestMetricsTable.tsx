import { memo } from 'react';
import type { Metric } from '../../api/types';
import { iconForProtocol } from '../../utils/icons';

interface Props {
  metrics: Metric[];
}

function LatestMetricsTableInner({ metrics }: Props) {
  return (
    <div className="bg-surface-container-high rounded-xl p-6 border border-outline-variant/20 flex flex-col shadow-lg">
      <div className="flex items-center gap-2 mb-6">
        <span className="material-symbols-outlined text-primary text-xl">speed</span>
        <h3 className="text-sm font-headline font-bold uppercase tracking-widest text-on-surface">Latest Metrics</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="text-[10px] uppercase tracking-widest text-on-surface-variant border-b border-outline-variant/20">
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
              const sc = isDown ? 'text-error bg-error/10 border-error/20' : isWarn ? 'text-amber-400 bg-amber-400/10 border-amber-400/20' : 'text-primary bg-primary/10 border-primary/20';
              const statusIcon = isDown ? 'cancel' : isWarn ? 'warning' : 'check_circle';

              return (
                <tr key={m.id || i} className="border-b border-outline-variant/10 hover:bg-surface-container-highest/50 transition-[background-color] group">
                  <td className="py-3 font-headline font-semibold text-on-surface group-hover:text-primary transition-[color]">{m.deviceName}</td>
                  <td className="py-3 text-on-surface-variant text-xs uppercase tracking-wider">
                    <div className="flex items-center gap-1.5">
                      <span className="material-symbols-outlined text-[14px] opacity-70">
                        {iconForProtocol(m.protocol)}
                      </span>
                      {m.protocol || '-'}
                    </div>
                  </td>
                  <td className="py-3">
                    <div className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border ${sc} text-[10px] font-bold uppercase tracking-widest`}>
                      <span className="material-symbols-outlined text-[14px]">{statusIcon}</span>
                      {m.status}
                    </div>
                  </td>
                  <td className="py-3 text-right font-mono text-on-surface">{m.responseTime ?? '-'}ms</td>
                  <td className="py-3 text-right text-xs text-on-surface-variant font-mono">{new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {metrics.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 opacity-50">
            <span className="material-symbols-outlined text-4xl mb-2">monitoring</span>
            <p className="text-xs text-on-surface-variant uppercase tracking-widest">No metrics data yet</p>
          </div>
        )}
      </div>
    </div>
  );
}

export const LatestMetricsTable = memo(LatestMetricsTableInner);
