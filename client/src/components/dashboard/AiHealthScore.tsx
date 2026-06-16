import { memo } from 'react';
import type { InsightsResponse } from '../../api/types';

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
    <div className={`bg-surface-container-high rounded-xl p-5 border border-outline-variant/20 flex flex-col items-center justify-center ${networkHealth < 40 ? 'glow-critical' : networkHealth < 65 ? 'glow-risk' : networkHealth < 85 ? 'glow-watch' : 'glow-healthy'}`}>
      <p className="text-[10px] text-on-surface-variant uppercase tracking-[0.2em] mb-3">AI Health Score</p>
      <div className="relative inline-flex items-center justify-center" style={{ width: 120, height: 120 }}>
        <svg width={120} height={120} className="transform -rotate-90">
          <circle cx={60} cy={60} r={52} fill="none" stroke="#26261d" strokeWidth={8} />
          <circle
            cx={60} cy={60} r={52}
            fill="none"
            stroke={networkHealth < 55 ? '#ff4444' : networkHealth < 75 ? '#f59e0b' : '#d9fd3a'}
            strokeWidth={8}
            strokeLinecap="round"
            strokeDasharray={2 * Math.PI * 52}
            className="gauge-ring"
            style={{
              '--gauge-circumference': 2 * Math.PI * 52,
              '--gauge-offset': 2 * Math.PI * 52 - (networkHealth / 100) * 2 * Math.PI * 52,
              strokeDashoffset: 2 * Math.PI * 52 - (networkHealth / 100) * 2 * Math.PI * 52,
              filter: `drop-shadow(0 0 6px ${networkHealth < 55 ? '#ff444440' : networkHealth < 75 ? '#f59e0b40' : '#d9fd3a40'})`,
            } as React.CSSProperties}
          />
        </svg>
        <span className={`absolute font-headline text-3xl font-black ${networkHealth < 55 ? 'text-error' : networkHealth < 75 ? 'text-amber-400' : 'text-primary'}`}>
          {networkHealth}
        </span>
      </div>
      {weakestDevice && (
        <div className="flex items-center gap-1 mt-2">
          <span className={`material-symbols-outlined text-sm ${weakestDevice.trend === 'improving' ? 'text-primary' : weakestDevice.trend === 'degrading' ? 'text-error trend-pulse' : 'text-on-surface-variant'}`}>
            {weakestDevice.trend === 'improving' ? 'trending_up' : weakestDevice.trend === 'degrading' ? 'trending_down' : 'trending_flat'}
          </span>
          <span className="text-[10px] uppercase tracking-widest text-on-surface-variant font-bold">
            {weakestDevice.trend || 'stable'}
          </span>
        </div>
      )}
      <p className="text-[10px] text-on-surface-variant mt-2 text-center">
        {weakestDevice ? `${weakestDevice.deviceName} needs watch` : 'Waiting for telemetry'}
      </p>
    </div>
  );
}

export const AiHealthScore = memo(AiHealthScoreInner);
