import { useEffect, useRef } from 'react';
import type { SystemInfo } from '../api/types';

interface ResourceLoadModalProps {
  systemInfo: { cpu: number; memory: number; errorRate: number; raw?: SystemInfo };
  onClose: () => void;
}

export default function ResourceLoadModal({ systemInfo, onClose }: ResourceLoadModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const { raw } = systemInfo;

  useEffect(() => {
    previousFocus.current = document.activeElement as HTMLElement;
    dialogRef.current?.focus();

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
        return;
      }
      if (e.key !== 'Tab' || !dialogRef.current) return;
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (!first || !last) return;
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      previousFocus.current?.focus();
    };
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pt-20 bg-black/80" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="System resource analytics"
        tabIndex={-1}
        className="bg-surface-1 border border-line rounded-lg w-full max-w-4xl max-h-[90vh] overflow-hidden flex flex-col outline-none"
        onClick={e => e.stopPropagation()}
      >
        <div className="p-6 border-b border-line flex justify-between items-center bg-surface-1 shrink-0">
          <div className="flex items-center gap-3">
            <span className="material-symbols-outlined text-primary text-3xl">memory</span>
            <div>
              <h2 className="font-sans text-2xl font-semibold text-fg uppercase tracking-tight">System Resource Analytics</h2>
              <p className="text-fg-muted text-xs font-mono uppercase tracking-wide">Server node performance telemetry</p>
            </div>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-inset rounded-full transition-colors" aria-label="Close dialog">
            <span className="material-symbols-outlined text-fg-subtle hover:text-fg">close</span>
          </button>
        </div>

        <div className="p-6 overflow-y-auto flex-1 bg-inset">
          {!raw ? (
            <div className="flex flex-col items-center justify-center py-16 opacity-50">
              <span className="material-symbols-outlined text-6xl mb-4">cloud_off</span>
              <p className="text-sm font-sans uppercase tracking-wide text-fg-muted">Detailed telemetry unavailable</p>
              <p className="text-xs text-fg-muted mt-2 max-w-md text-center">
                The core system node is either offline or the telemetry collector agent is not transmitting detailed hardware utilization.
              </p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div className="bg-surface-1 rounded-lg p-5 border border-line">
                <div className="flex items-center gap-2 mb-4">
                  <span className="material-symbols-outlined text-primary">developer_board</span>
                  <h3 className="font-sans font-semibold uppercase tracking-wide text-sm">Processor (CPU)</h3>
                </div>
                <div className="flex items-end justify-between mb-2">
                  <span className="text-4xl font-sans font-semibold text-fg">{raw.cpu.usage}%</span>
                  <span className="text-xs font-mono text-fg-muted pb-1">{raw.cpu.cores} Cores</span>
                </div>
                <div className="h-2 bg-inset rounded w-full mb-4">
                  <div className="h-2 rounded transition-[width] duration-500 bg-primary" style={{ width: `${Math.min(100, raw.cpu.usage)}%` }} />
                </div>
                <div className="text-[10px] uppercase font-mono tracking-wider text-fg-muted bg-inset/50 px-3 py-2 rounded-lg truncate">
                  {raw.cpu.model}
                </div>
              </div>

              <div className="bg-surface-1 rounded-lg p-5 border border-line">
                <div className="flex items-center gap-2 mb-4">
                  <span className="material-symbols-outlined text-primary-hover">memory_alt</span>
                  <h3 className="font-sans font-semibold uppercase tracking-wide text-sm">System Memory (RAM)</h3>
                </div>
                <div className="flex items-end justify-between mb-2">
                  <span className="text-4xl font-sans font-semibold text-fg">{raw.memory.percent}%</span>
                  <span className="text-xs font-mono text-fg-muted pb-1">{(raw.memory.used || 0).toFixed(1)}GB / {(raw.memory.total || 0).toFixed(1)}GB</span>
                </div>
                <div className="h-2 bg-inset rounded w-full mb-4">
                  <div className="h-2 rounded transition-[width] duration-500 bg-primary-hover" style={{ width: `${Math.min(100, raw.memory.percent)}%` }} />
                </div>
              </div>

              {raw.disk && (
                <div className="bg-surface-1 rounded-lg p-5 border border-line">
                  <div className="flex items-center gap-2 mb-4">
                    <span className="material-symbols-outlined text-chart-5">hard_drive</span>
                    <h3 className="font-sans font-semibold uppercase tracking-wide text-sm">Storage (Disk)</h3>
                  </div>
                  <div className="flex items-end justify-between mb-2">
                    <span className="text-4xl font-sans font-semibold text-fg">{raw.disk.percent}%</span>
                    <span className="text-xs font-mono text-fg-muted pb-1">{(raw.disk.used || 0).toFixed(1)}GB / {(raw.disk.total || 0).toFixed(1)}GB</span>
                  </div>
                  <div className="h-2 bg-inset rounded w-full mb-4">
                    <div className="h-2 rounded transition-[width] duration-500 bg-chart-5" style={{ width: `${Math.min(100, raw.disk.percent)}%` }} />
                  </div>
                </div>
              )}

              <div className="bg-surface-1 rounded-lg p-5 border border-line flex flex-col justify-center">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <div className="flex items-center gap-2 mb-2">
                      <span className="material-symbols-outlined text-fg-muted text-sm">schedule</span>
                      <h3 className="font-sans font-semibold uppercase tracking-wide text-xs text-fg-muted">System Uptime</h3>
                    </div>
                    <div className="font-mono text-lg text-fg">
                      {Math.floor((raw.uptime || 0) / 86400)}d {Math.floor(((raw.uptime || 0) % 86400) / 3600)}h {Math.floor(((raw.uptime || 0) % 3600) / 60)}m
                    </div>
                  </div>
                  <div>
                    <div className="flex items-center gap-2 mb-2">
                      <span className="material-symbols-outlined text-fg-muted text-sm">analytics</span>
                      <h3 className="font-sans font-semibold uppercase tracking-wide text-xs text-fg-muted">Load Average</h3>
                    </div>
                    <div className="font-mono text-sm text-fg mt-1">
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
