import { useCallback, useId } from 'react';
import { useSearchParams } from 'react-router-dom';

/**
 * Tab state kept in the URL (`?tab=`), so tabs survive reloads, can be
 * linked to, and work with Back/Forward. Unknown values fall back to the
 * first tab. Changing tabs replaces the history entry.
 */
export function useTabParam<T extends string>(ids: readonly T[], param = 'tab'): [T, (next: T) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get(param);
  const value = (ids as readonly string[]).includes(raw ?? '') ? (raw as T) : ids[0]!;
  const setValue = useCallback(
    (next: T) => {
      setParams(
        (prev) => {
          const out = new URLSearchParams(prev);
          if (next === ids[0]) out.delete(param);
          else out.set(param, next);
          return out;
        },
        { replace: true },
      );
    },
    [ids, param, setParams],
  );
  return [value, setValue];
}

/** Stable id prefix for a Tabs/TabPanel pair. */
export function useTabsId(): string {
  return `tabs-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
}
