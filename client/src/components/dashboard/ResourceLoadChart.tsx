import { memo } from 'react';
import { ResourceBar } from './ResourceBar';
import type { SystemInfo } from '../../api/types';

interface Props {
  systemInfo: { cpu: number; memory: number; errorRate: number; raw?: SystemInfo };
  onExpand: () => void;
}

function ResourceLoadChartInner({ systemInfo, onExpand }: Props) {
  return (
    <button
      type="button"
      onClick={onExpand}
      aria-label="Server load: open host resource details"
      className="group w-full bg-surface-1 rounded-lg p-5 border border-line text-left transition-colors duration-[var(--duration-fast)] hover:border-line-strong hover:bg-surface-2"
    >
      <div className="flex justify-between items-start mb-5">
        <div>
          <h3 className="text-sm font-semibold text-fg">Server load</h3>
          <p className="text-xs text-fg-muted">NetMonitor host</p>
        </div>
        <span className="material-symbols-outlined text-[18px] text-fg-muted group-hover:text-fg transition-colors" aria-hidden="true">open_in_full</span>
      </div>
      <div className="space-y-4">
        {systemInfo.raw ? (
          <>
            <ResourceBar label="CPU" value={systemInfo.cpu} color="var(--color-chart-1)" warn={systemInfo.cpu > 90} />
            <ResourceBar label="Memory" value={systemInfo.memory} color="var(--color-chart-5)" warn={systemInfo.memory > 90} />
          </>
        ) : (
          <p className="text-xs text-fg-muted">Waiting for host metrics…</p>
        )}
        <ResourceBar label="Devices down" value={systemInfo.errorRate} color="var(--color-status-down)" warn={systemInfo.errorRate > 80} />
      </div>
    </button>
  );
}

export const ResourceLoadChart = memo(ResourceLoadChartInner);
