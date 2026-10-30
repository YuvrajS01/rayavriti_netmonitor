import { memo } from 'react';
import { ResourceBar } from './ResourceBar';
import type { SystemInfo } from '../../api/types';

interface Props {
  systemInfo: { cpu: number; memory: number; errorRate: number; raw?: SystemInfo };
  onExpand: () => void;
}

function ResourceLoadChartInner({ systemInfo, onExpand }: Props) {
  return (
    <div
      className="bg-surface-1 rounded-lg p-4 border border-line hover:border-line-strong transition-colors duration-200 cursor-pointer group"
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onExpand(); } }}
      onClick={onExpand}
    >
      <div className="flex justify-between items-center mb-3">
        <div>
          <h3 className="text-sm font-sans font-semibold text-fg group-hover:text-fg transition-colors">Server Load</h3>
          <p className="text-xs text-fg-muted">NetMonitor host</p>
        </div>
        <span className="material-symbols-outlined text-fg-muted group-hover:text-fg text-sm transition-colors">open_in_full</span>
      </div>
      <div className="space-y-4 mt-6">
        {systemInfo.raw ? (
          <>
            <ResourceBar label="CPU" value={systemInfo.cpu} color="var(--color-chart-1)" warn={systemInfo.cpu > 90} />
            <ResourceBar label="Memory" value={systemInfo.memory} color="var(--color-chart-2)" warn={systemInfo.memory > 90} />
          </>
        ) : (
          <p className="text-xs text-fg-muted">Waiting for host metrics…</p>
        )}
        <ResourceBar label="Devices down" value={systemInfo.errorRate} color="var(--color-error)" warn={systemInfo.errorRate > 80} />
      </div>
    </div>
  );
}

export const ResourceLoadChart = memo(ResourceLoadChartInner);
