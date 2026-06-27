import { useEffect } from 'react';

/**
 * An async-aware useEffect that provides an AbortSignal for cleanup.
 * The signal is aborted when the component unmounts or deps change.
 *
 * Usage:
 *   useAsyncEffect(async (signal) => {
 *     const res = await fetchData();
 *     if (!signal.aborted) setData(res);
 *   }, []);
 */
export function useAsyncEffect(
  effect: (signal: AbortSignal) => Promise<void>,
  deps: React.DependencyList,
) {
  useEffect(() => {
    const controller = new AbortController();
    effect(controller.signal);
    return () => controller.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);
}
