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
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
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
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label="System resource analytics"
        tabIndex={-1}
        className="bg-surface-container-low border border-outline-variant/30 rounded-xl w-full max-w-4xl max-h-[90vh] overflow-hidden shadow-2xl flex flex-col outline-none"
        onClick={e => e.stopPropagation()}
      >
        <div className="p-6 border-b border-outline-variant/20 flex justify-between items-center bg-surface-container-high">
          <div className="flex items-center gap-3">
            <span className="material-symbols-outlined text-primary text-3xl">memory</span>
            <div>
              <h2 className="font-headline text-2xl font-black text-on-surface uppercase tracking-tight">System Resource Analytics</h2>
              <p className="text-on-surface-variant text-xs font-mono uppercase tracking-widest">Server node performance telemetry</p>
            </div>
          </div>
          <button onClick={onClose} className="p-2 hover:bg-surface-container-highest rounded-full transition-[background-color]" aria-label="Close dialog">
            <span className="material-symbols-outlined text-outline hover:text-on-surface">close</span>
          </button>
        </div>

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
              <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
                <div className="flex items-center gap-2 mb-4">
                  <span className="material-symbols-outlined text-primary">developer_board</span>
                  <h3 className="font-headline font-bold uppercase tracking-widest text-sm">Processor (CPU)</h3>
                </div>
                <div className="flex items-end justify-between mb-2">
                  <span className="text-4xl font-headline font-black text-on-surface">{raw.cpu.usage}%</span>
                  <span className="text-xs font-mono text-on-surface-variant pb-1">{raw.cpu.cores} Cores</span>
                </div>
                <div className="h-2 bg-surface-container-highest rounded w-full mb-4">
                  <div className="h-2 rounded transition-[width] duration-500 bg-primary" style={{ width: `${Math.min(100, raw.cpu.usage)}%` }} />
                </div>
                <div className="text-[10px] uppercase font-mono tracking-wider text-on-surface-variant bg-surface-container-highest/50 px-3 py-2 rounded-lg truncate">
                  {raw.cpu.model}
                </div>
              </div>

              <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
                <div className="flex items-center gap-2 mb-4">
                  <span className="material-symbols-outlined text-primary-dim">memory_alt</span>
                  <h3 className="font-headline font-bold uppercase tracking-widest text-sm">System Memory (RAM)</h3>
                </div>
                <div className="flex items-end justify-between mb-2">
                  <span className="text-4xl font-headline font-black text-on-surface">{raw.memory.percent}%</span>
                  <span className="text-xs font-mono text-on-surface-variant pb-1">{(raw.memory.used || 0).toFixed(1)}GB / {(raw.memory.total || 0).toFixed(1)}GB</span>
                </div>
                <div className="h-2 bg-surface-container-highest rounded w-full mb-4">
                  <div className="h-2 rounded transition-[width] duration-500 bg-primary-dim" style={{ width: `${Math.min(100, raw.memory.percent)}%` }} />
                </div>
              </div>

              {raw.disk && (
                <div className="bg-surface-container-high rounded-xl p-5 border border-outline-variant/20">
                  <div className="flex items-center gap-2 mb-4">
                    <span className="material-symbols-outlined text-secondary">hard_drive</span>
                    <h3 className="font-headline font-bold uppercase tracking-widest text-sm">Storage (Disk)</h3>
                  </div>
                  <div className="flex items-end justify-between mb-2">
                    <span className="text-4xl font-headline font-black text-on-surface">{raw.disk.percent}%</span>
                    <span className="text-xs font-mono text-on-surface-variant pb-1">{(raw.disk.used || 0).toFixed(1)}GB / {(raw.disk.total || 0).toFixed(1)}GB</span>
                  </div>
                  <div className="h-2 bg-surface-container-highest rounded w-full mb-4">
                    <div className="h-2 rounded transition-[width] duration-500 bg-secondary" style={{ width: `${Math.min(100, raw.disk.percent)}%` }} />
                  </div>
                </div>
              )}

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
