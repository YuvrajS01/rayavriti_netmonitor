import { memo } from 'react';
import type { InsightsResponse } from '../../api/types';

interface Props {
  insights: InsightsResponse | null;
}

function SmartInsightsInner({ insights }: Props) {
  const healthArray = insights?.health || [];

  return (
    <div className="xl:col-span-2 bg-surface-container-high rounded-lg p-5 border border-outline-variant/20">
      <div className="flex items-center justify-between gap-4 mb-4">
        <h3 className="text-sm font-headline font-bold uppercase tracking-wide">Smart Insights</h3>
        <span className="text-xs text-on-surface-variant uppercase tracking-wide">
          {insights?.generatedAt ? new Date(insights.generatedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : 'Pending'}
        </span>
      </div>
      <div role="list" className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {healthArray.slice(0, 4).map((item, idx) => {
          const isCritical = item.score < 40;
          const isWarn = item.score < 70;
          const color = isCritical ? 'text-error' : isWarn ? 'text-warning' : 'text-primary';
          const bg = isCritical ? 'bg-error/10 border-error/25' : isWarn ? 'bg-warning/10 border-warning/25' : 'bg-primary/10 border-primary/25';
          return (
            <div role="listitem" key={`${item.deviceId}-${idx}`} className={`rounded-lg border ${bg} p-3 flex gap-3 min-h-24`}>
              <span className={`material-symbols-outlined ${color} text-lg mt-0.5`}>{isCritical ? 'error' : isWarn ? 'warning' : 'tips_and_updates'}</span>
              <div className="min-w-0">
                <p className={`text-xs font-bold uppercase tracking-wide ${color}`}>{item.deviceName}</p>
                <p className="text-xs text-on-surface-variant mt-1 leading-relaxed">Score: {item.score.toFixed(2)} — {item.label}</p>
              </div>
            </div>
          );
        })}
        {healthArray.length === 0 && (
          <div className="md:col-span-2 py-8 text-center text-xs text-on-surface-variant">No anomalies or grouped risks detected</div>
        )}
      </div>
    </div>
  );
}

export const SmartInsights = memo(SmartInsightsInner);
