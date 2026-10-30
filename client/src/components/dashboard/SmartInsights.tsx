import { memo, useMemo } from 'react';
import type { InsightsResponse } from '../../api/types';
import { healthMeta } from '../../utils/status';
import { dedupeInsights } from '../../utils/insights';
import { StatusPill } from '../ui/Status';

interface Props {
  insights: InsightsResponse | null;
}

function SmartInsightsInner({ insights }: Props) {
  const items = useMemo(() => dedupeInsights(insights?.health ?? []).slice(0, 4), [insights?.health]);

  return (
    <section aria-labelledby="smart-insights-heading" className="xl:col-span-2 bg-surface-1 rounded-lg p-5 border border-line">
      <div className="flex items-center justify-between gap-4 mb-4">
        <h3 id="smart-insights-heading" className="text-sm font-semibold text-fg">Smart insights</h3>
        <span className="text-xs text-fg-muted">
          {insights?.generatedAt ? new Date(insights.generatedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : 'Pending'}
        </span>
      </div>
      <ul className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {items.map((item) => {
          const meta = healthMeta(item.score);
          return (
            <li key={item.deviceId} className="rounded-md border border-line bg-surface-2 p-3 flex flex-col gap-2 min-h-20">
              <div className="flex items-start justify-between gap-2">
                <p className="text-sm font-semibold text-fg truncate">{item.deviceName}</p>
                <StatusPill status={meta} />
              </div>
              <p className="text-xs text-fg-muted">
                Health score <span className="font-mono text-fg">{item.score.toFixed(0)}</span>
                <span className="text-fg-subtle"> / 100</span>
              </p>
            </li>
          );
        })}
        {items.length === 0 && (
          <li className="md:col-span-2 py-8 text-center text-sm text-fg-muted">No anomalies or grouped risks detected</li>
        )}
      </ul>
    </section>
  );
}

export const SmartInsights = memo(SmartInsightsInner);
