import { memo } from 'react';
import type { InsightsResponse } from '../../api/types';
import RingGauge from '../ui/RingGauge';
import Sparkline from '../ui/Sparkline';

interface Props {
  networkHealth: number;
  insights: InsightsResponse | null;
}

function AiHealthScoreInner({ networkHealth, insights }: Props) {
  const healthArray = insights?.health || [];
  const weakestDevice = healthArray.length
    ? [...healthArray].sort((a, b) => a.score - b.score)[0]
    : undefined;

  return (
    <div className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col items-center justify-center">
      <p className="text-xs text-fg-muted font-medium mb-3">AI health score</p>
      <RingGauge value={networkHealth} label="network" showParticles />
      {weakestDevice && (
        <div className="flex items-center gap-1 mt-2">
          <span className={`material-symbols-outlined text-sm ${weakestDevice.trend === 'improving' ? 'text-success' : weakestDevice.trend === 'degrading' ? 'text-error trend-pulse' : 'text-fg-muted'}`}>
            {weakestDevice.trend === 'improving' ? 'trending_up' : weakestDevice.trend === 'degrading' ? 'trending_down' : 'trending_flat'}
          </span>
          <span className="text-xs text-fg-muted font-medium">
            {weakestDevice.trend || 'stable'}
          </span>
        </div>
      )}
      <p className="text-xs text-fg-muted mt-2 text-center">
        {weakestDevice ? `${weakestDevice.deviceName} needs watch` : 'Waiting for telemetry'}
      </p>
      <Sparkline data={healthArray.map((item) => item.score).slice(-12)} color="var(--color-success)" className="mt-2" />
    </div>
  );
}

export const AiHealthScore = memo(AiHealthScoreInner);
