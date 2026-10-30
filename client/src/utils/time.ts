import { useSyncExternalStore } from 'react';

/** Shared clock and relative-time formatting ("5m ago"). */

// One shared 15s ticker for every relative timestamp on screen.
const TICK_MS = 15_000;
let now = Date.now();
const listeners = new Set<() => void>();
let timer: ReturnType<typeof setInterval> | null = null;

function subscribe(listener: () => void) {
  listeners.add(listener);
  if (!timer) {
    timer = setInterval(() => {
      now = Date.now();
      listeners.forEach((l) => l());
    }, TICK_MS);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer) {
      clearInterval(timer);
      timer = null;
    }
  };
}

function snapshot() {
  // Refresh a stale value (no subscriber was ticking it); stable otherwise.
  const t = Date.now();
  if (t - now >= TICK_MS) now = t;
  return now;
}

/** Current time, re-rendering every 15s (shared timer). */
export function useNow(): number {
  return useSyncExternalStore(subscribe, snapshot, snapshot);
}

/** "just now", "5m ago", "3h ago", "2d ago"; future times read "in 5m". */
export function formatRelative(iso: string | number | Date, reference = Date.now()): string {
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t)) return '—';
  const diff = reference - t;
  const abs = Math.abs(diff);
  const fmt = (n: number, unit: string) => (diff >= 0 ? `${n}${unit} ago` : `in ${n}${unit}`);
  if (abs < 45_000) return 'just now';
  if (abs < 3_600_000) return fmt(Math.round(abs / 60_000), 'm');
  if (abs < 86_400_000) return fmt(Math.round(abs / 3_600_000), 'h');
  return fmt(Math.round(abs / 86_400_000), 'd');
}

export function formatAbsolute(iso: string | number | Date): string {
  const d = new Date(iso);
  return Number.isFinite(d.getTime()) ? d.toLocaleString() : '';
}

