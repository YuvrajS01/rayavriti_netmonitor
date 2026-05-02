import type { SystemInfo } from '../api/types';

interface ResourceLoadModalProps {
  systemInfo: { cpu: number; memory: number; errorRate: number; raw?: SystemInfo };
  onClose: () => void;
}

export default function ResourceLoadModal({ systemInfo, onClose }: ResourceLoadModalProps) {
  const { raw } = systemInfo;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md" onClick={onClose}>
      <div 
        className="bg-surface-container-low border border-outline-variant/30 rounded-xl w-full max-w-4xl max-h-[90vh] overflow-hidden shadow-2xl flex flex-col"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="p-6 border-b border-outline-variant/20 flex justify-between items-center bg-surface-container-high">
          <div className="flex items-center gap-3">
            <span className="material-symbols-outlined text-primary text-3xl">memory</span>
            <div>
              <h2 className="font-headline text-2xl font-black text-on-surface uppercase tracking-tight">System Resource Analytics</h2>
              <p className="text-on-surface-variant text-xs font-mono uppercase tracking-widest">Server node performance telemetry</p>
            </div>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-surface-container-highest rounded-full transition-colors">
            <span className="material-symbols-outlined text-outline hover:text-on-surface">close</span>
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto flex-1 bg-surface-container-lowest">
          {!raw ? (
            <div className="flex flex-col items-center justify-center py-16 opacity-50">
              <span className="material-symbols-outlined text-6xl mb-4">cloud_off</span>
              <p className="text-sm font-headline uppercase tracking-widest text-on-surface-variant">Detailed telemetry unavailable</p>
              <p className="text-xs text-on-surface-variant mt-2 max-w-md text-center">
                The core system node is either offline or the telemetry collector agent is not transmitting detailed hardware utilization.
              </p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* CPU Box */}
              <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
                <div className="flex items-center gap-2 mb-4">
                  <span className="material-symbols-outlined text-[var(--color-cpu,#d9fd3a)]">developer_board</span>
                  <h3 className="font-headline font-bold uppercase tracking-widest text-sm">Processor (CPU)</h3>
                </div>
                
                <div className="flex items-end justify-between mb-2">
                  <span className="text-4xl font-headline font-black text-on-surface">{raw.cpu.usage}%</span>
                  <span className="text-xs font-mono text-on-surface-variant pb-1">{raw.cpu.cores} Cores</span>
                </div>
                <div className="h-2 bg-surface-container-highest rounded w-full mb-4">
                  <div className="h-2 rounded transition-all duration-500 bg-[var(--color-cpu,#d9fd3a)]" style={{ width: `${Math.min(100, raw.cpu.usage)}%` }} />
                </div>
                <div className="text-[10px] uppercase font-mono tracking-wider text-on-surface-variant bg-surface-container-highest/50 px-3 py-2 rounded-lg truncate">
                  {raw.cpu.model}
                </div>
              </div>

              {/* Memory Box */}
              <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
                <div className="flex items-center gap-2 mb-4">
                  <span className="material-symbols-outlined text-[var(--color-ram,#cbee29)]">memory_alt</span>
                  <h3 className="font-headline font-bold uppercase tracking-widest text-sm">System Memory (RAM)</h3>
                </div>
                
                <div className="flex items-end justify-between mb-2">
                  <span className="text-4xl font-headline font-black text-on-surface">{raw.memory.percent}%</span>
                  <span className="text-xs font-mono text-on-surface-variant pb-1">{(raw.memory.used || 0).toFixed(1)}GB / {(raw.memory.total || 0).toFixed(1)}GB</span>
                </div>
                <div className="h-2 bg-surface-container-highest rounded w-full mb-4">
                  <div className="h-2 rounded transition-all duration-500 bg-[var(--color-ram,#cbee29)]" style={{ width: `${Math.min(100, raw.memory.percent)}%` }} />
                </div>
              </div>

              {/* Disk Box */}
              {raw.disk && (
                <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
                  <div className="flex items-center gap-2 mb-4">
                    <span className="material-symbols-outlined text-[var(--color-disk,#6ee7f7)]">hard_drive</span>
                    <h3 className="font-headline font-bold uppercase tracking-widest text-sm">Storage (Disk)</h3>
                  </div>
                  
                  <div className="flex items-end justify-between mb-2">
                    <span className="text-4xl font-headline font-black text-on-surface">{raw.disk.percent}%</span>
                    <span className="text-xs font-mono text-on-surface-variant pb-1">{(raw.disk.used || 0).toFixed(1)}GB / {(raw.disk.total || 0).toFixed(1)}GB</span>
                  </div>
                  <div className="h-2 bg-surface-container-highest rounded w-full mb-4">
                    <div className="h-2 rounded transition-all duration-500 bg-[var(--color-disk,#6ee7f7)]" style={{ width: `${Math.min(100, raw.disk.percent)}%` }} />
                  </div>
                </div>
              )}

              {/* Uptime & Load */}
              <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20 flex flex-col justify-center">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <div className="flex items-center gap-2 mb-2">
                      <span className="material-symbols-outlined text-on-surface-variant text-sm">schedule</span>
                      <h3 className="font-headline font-bold uppercase tracking-widest text-xs text-on-surface-variant">System Uptime</h3>
                    </div>
                    <div className="font-mono text-lg text-on-surface">
                      {Math.floor((raw.uptime || 0) / 86400)}d {Math.floor(((raw.uptime || 0) % 86400) / 3600)}h {Math.floor(((raw.uptime || 0) % 3600) / 60)}m
                    </div>
                  </div>
                  <div>
                    <div className="flex items-center gap-2 mb-2">
                      <span className="material-symbols-outlined text-on-surface-variant text-sm">analytics</span>
                      <h3 className="font-headline font-bold uppercase tracking-widest text-xs text-on-surface-variant">Load Average</h3>
                    </div>
                    <div className="font-mono text-sm text-on-surface mt-1">
                      {raw.loadAvg ? raw.loadAvg.map((l: number) => l.toFixed(2)).join('  ') : '-'}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
