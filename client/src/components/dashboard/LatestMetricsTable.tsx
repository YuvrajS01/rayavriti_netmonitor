import { memo } from 'react';
import type { Metric } from '../../api/types';
import { iconForProtocol } from '../../utils/icons';
import { StatusPill } from '../ui/Status';
import EmptyState from '../ui/EmptyState';

interface Props {
  metrics: Metric[];
}

function LatestMetricsTableInner({ metrics }: Props) {
  const maxResponse = Math.max(1, ...metrics.map((m) => m.responseTime || 0));
  return (
    <section aria-labelledby="latest-metrics-heading" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col">
      <h3 id="latest-metrics-heading" className="text-sm font-semibold text-fg mb-4">Latest metrics</h3>
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="text-xs text-fg-muted border-b border-line">
              <th scope="col" className="pb-3 font-medium">Device</th>
              <th scope="col" className="pb-3 font-medium">Protocol</th>
              <th scope="col" className="pb-3 font-medium">Status</th>
              <th scope="col" className="pb-3 font-medium text-right">Response</th>
              <th scope="col" className="pb-3 font-medium text-right">Time</th>
            </tr>
          </thead>
          <tbody className="text-sm">
            {metrics.slice(0, 15).map((m, i) => {
              const barPct = Math.min(100, ((m.responseTime || 0) / maxResponse) * 100);
              return (
                <tr key={m.id || i} className="border-b border-line last:border-b-0 hover:bg-surface-2 transition-colors">
                  <td className="py-2.5 pr-3 font-medium text-fg">{m.deviceName}</td>
                  <td className="py-2.5 pr-3 text-xs text-fg-muted">
                    <div className="flex items-center gap-1.5">
                      <span className="material-symbols-outlined text-[16px]" aria-hidden="true">{iconForProtocol(m.protocol)}</span>
                      <span className="uppercase">{m.protocol || '-'}</span>
                    </div>
                  </td>
                  <td className="py-2.5 pr-3"><StatusPill status={m.status} /></td>
                  <td className="py-2.5 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <span className="font-mono text-fg">{m.responseTime ?? '-'} ms</span>
                      <span className="hidden sm:block h-1.5 w-16 rounded-full bg-surface-3 overflow-hidden" aria-hidden="true">
                        <span className="block h-full rounded-full bg-chart-1" style={{ width: `${barPct}%` }} />
                      </span>
                    </div>
                  </td>
                  <td className="py-2.5 pl-3 text-right text-xs text-fg-muted font-mono">
                    {new Date(m.timestamp || m.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {metrics.length === 0 && <EmptyState icon="monitoring" title="No metrics data yet" size="compact" />}
      </div>
    </section>
  );
}

export const LatestMetricsTable = memo(LatestMetricsTableInner);
