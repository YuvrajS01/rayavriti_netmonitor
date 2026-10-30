import type { ReactNode } from 'react';
import type { SystemInfo } from '../api/types';
import Dialog from './ui/Dialog';
import EmptyState from './ui/EmptyState';

interface ResourceLoadModalProps {
  systemInfo: { cpu: number; memory: number; errorRate: number; raw?: SystemInfo };
  onClose: () => void;
}

/** A usage meter: neutral accent fill, value always printed next to it. */
function Meter({ icon, title, value, detail, children }: { icon: string; title: string; value: number; detail?: ReactNode; children?: ReactNode }) {
  const pct = Math.max(0, Math.min(100, value));
  return (
    <section aria-label={title} className="bg-surface-1 rounded-lg p-5 border border-line">
      <div className="flex items-center gap-2 mb-4">
        <span className="material-symbols-outlined text-fg-muted" aria-hidden="true">{icon}</span>
        <h3 className="font-semibold text-sm">{title}</h3>
      </div>
      <div className="flex items-end justify-between mb-2">
        <span className="text-3xl font-semibold tabular-nums text-fg">{value}%</span>
        {detail && <span className="text-xs font-mono text-fg-muted pb-1">{detail}</span>}
      </div>
      <div className="h-2 bg-surface-3 rounded-full w-full" role="meter" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct} aria-label={`${title} usage`}>
        <div className="h-2 rounded-full bg-primary transition-[width] duration-[var(--duration-normal)]" style={{ width: `${pct}%` }} />
      </div>
      {children}
    </section>
  );
}

export default function ResourceLoadModal({ systemInfo, onClose }: ResourceLoadModalProps) {
  const { raw } = systemInfo;

  return (
    <Dialog
      open
      onClose={onClose}
      size="xl"
      title="System resource analytics"
      description="Server node performance telemetry"
      bodyClassName="p-6 bg-inset"
    >
      {!raw ? (
        <EmptyState
          icon="cloud_off"
          title="Detailed telemetry unavailable"
          description="The core system node is either offline or the telemetry collector agent is not transmitting detailed hardware utilization."
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Meter icon="developer_board" title="Processor (CPU)" value={raw.cpu.usage} detail={`${raw.cpu.cores} cores`}>
            <p className="mt-4 text-xs font-mono text-fg-muted bg-surface-2 px-3 py-2 rounded-md truncate">{raw.cpu.model}</p>
          </Meter>

          <Meter
            icon="memory_alt"
            title="System memory (RAM)"
            value={raw.memory.percent}
            detail={`${(raw.memory.used || 0).toFixed(1)}GB / ${(raw.memory.total || 0).toFixed(1)}GB`}
          />

          {raw.disk && (
            <Meter
              icon="hard_drive"
              title="Storage (disk)"
              value={raw.disk.percent}
              detail={`${(raw.disk.used || 0).toFixed(1)}GB / ${(raw.disk.total || 0).toFixed(1)}GB`}
            />
          )}

          <section aria-label="Uptime and load" className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col justify-center">
            <dl className="grid grid-cols-2 gap-4">
              <div>
                <dt className="flex items-center gap-2 mb-2 text-xs font-semibold text-fg-muted">
                  <span className="material-symbols-outlined text-sm" aria-hidden="true">schedule</span>
                  System uptime
                </dt>
                <dd className="font-mono text-lg text-fg">
                  {Math.floor((raw.uptime || 0) / 86400)}d {Math.floor(((raw.uptime || 0) % 86400) / 3600)}h {Math.floor(((raw.uptime || 0) % 3600) / 60)}m
                </dd>
              </div>
              <div>
                <dt className="flex items-center gap-2 mb-2 text-xs font-semibold text-fg-muted">
                  <span className="material-symbols-outlined text-sm" aria-hidden="true">analytics</span>
                  Load average
                </dt>
                <dd className="font-mono text-sm text-fg mt-1">
                  {raw.loadAvg ? raw.loadAvg.map((l: number) => l.toFixed(2)).join('  ') : '—'}
                </dd>
              </div>
            </dl>
          </section>
        </div>
      )}
    </Dialog>
  );
}
