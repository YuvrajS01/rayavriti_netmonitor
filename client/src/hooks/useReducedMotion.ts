import { useSyncExternalStore } from 'react';

const QUERY = '(prefers-reduced-motion: reduce)';

function mediaQuery(): MediaQueryList | null {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function' ? window.matchMedia(QUERY) : null;
}

export function prefersReducedMotion(): boolean {
  return mediaQuery()?.matches ?? false;
}

function subscribe(onChange: () => void) {
  const mql = mediaQuery();
  mql?.addEventListener('change', onChange);
  return () => mql?.removeEventListener('change', onChange);
}

/**
 * True when the OS asks for reduced motion. CSS animations are already
 * neutralised in index.css; this covers JS-driven motion (rAF counters,
 * recharts `isAnimationActive`, canvas particles).
 */
export function useReducedMotion(): boolean {
  return useSyncExternalStore(subscribe, prefersReducedMotion, () => false);
}

/** Spread onto recharts series: `<Line {...useChartAnimation()} />`. */
export function useChartAnimation(): { isAnimationActive: boolean; animationDuration: number } {
  const reduced = useReducedMotion();
  return { isAnimationActive: !reduced, animationDuration: 400 };
}
