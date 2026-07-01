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
      className="bg-surface-container-low rounded-lg p-4 border border-outline-variant/20 hover:border-outline transition-colors duration-200 cursor-pointer group"
      role="button"
      tabIndex={0}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onExpand(); } }}
      onClick={onExpand}
    >
      <div className="flex justify-between items-center mb-3">
        <h3 className="text-sm font-headline font-semibold text-on-surface group-hover:text-on-surface transition-colors">Resource Load</h3>
        <span className="material-symbols-outlined text-on-surface-variant group-hover:text-on-surface text-sm transition-colors">open_in_full</span>
      </div>
      <div className="space-y-4 mt-6">
        <ResourceBar label="CPU" value={systemInfo.cpu} color="var(--color-chart-1)" />
        <ResourceBar label="Memory" value={systemInfo.memory} color="var(--color-chart-2)" />
        <ResourceBar label="Error Rate" value={systemInfo.errorRate} color="var(--color-error)" />
      </div>
    </div>
  );
}

export const ResourceLoadChart = memo(ResourceLoadChartInner);
