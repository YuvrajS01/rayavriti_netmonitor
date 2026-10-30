import { memo, useMemo } from 'react';
import type { InsightsResponse } from '../../api/types';
import RingGauge from '../ui/RingGauge';
import { StatusPill } from '../ui/Status';
import { healthMeta } from '../../utils/status';

interface Props {
  networkHealth: number;
  insights: InsightsResponse | null;
}

const TREND_ICON = { improving: 'trending_up', degrading: 'trending_down', stable: 'trending_flat' } as const;

function AiHealthScoreInner({ networkHealth, insights }: Props) {
  const weakestDevice = useMemo(() => {
    const items = insights?.health ?? [];
    return items.length ? items.reduce((worst, item) => (item.score < worst.score ? item : worst)) : undefined;
  }, [insights?.health]);
  const trend = weakestDevice?.trend ?? 'stable';

  return (
    <section aria-labelledby="ai-health-heading" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col items-center justify-center gap-3">
      <h3 id="ai-health-heading" className="self-start text-sm font-semibold text-fg">AI health score</h3>
      <RingGauge value={networkHealth} label="Network health" />
      {weakestDevice ? (
        <div className="w-full rounded-md bg-surface-2 px-3 py-2 text-xs">
          <p className="text-fg-muted">Weakest device</p>
          <div className="mt-1 flex items-center justify-between gap-2">
            <span className="truncate text-sm font-medium text-fg">{weakestDevice.deviceName}</span>
            <StatusPill status={healthMeta(weakestDevice.score)} />
          </div>
          <p className="mt-1 inline-flex items-center gap-1 text-fg-muted">
            <span className="material-symbols-outlined text-sm" aria-hidden="true">{TREND_ICON[trend] ?? 'trending_flat'}</span>
            Trend {trend}
          </p>
        </div>
      ) : (
        <p className="text-xs text-fg-muted text-center">Waiting for telemetry</p>
      )}
    </section>
  );
}

export const AiHealthScore = memo(AiHealthScoreInner);
