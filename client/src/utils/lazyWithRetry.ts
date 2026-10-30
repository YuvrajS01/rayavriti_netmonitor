import { lazy, type ComponentType } from 'react';

const RELOAD_FLAG = 'netmonitor:chunk-reload';
const RETRY_DELAY_MS = 500;

/**
 * True for the errors browsers/Vite raise when a lazily imported chunk is
 * missing — typically because a deploy replaced the hashed assets while
 * this tab was still running the old build.
 */
export function isChunkLoadError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  return /Failed to fetch dynamically imported module|error loading dynamically imported module|Importing a module script failed|Unable to preload CSS|ChunkLoadError|Loading chunk [\w-]+ failed/i
    .test(`${error.name}: ${error.message}`);
}

function readFlag(): boolean {
  try { return sessionStorage.getItem(RELOAD_FLAG) === '1'; } catch { return true; }
}

function writeFlag(value: boolean) {
  try {
    if (value) sessionStorage.setItem(RELOAD_FLAG, '1');
    else sessionStorage.removeItem(RELOAD_FLAG);
  } catch { /* storage unavailable: reload guard degrades to "never reload" */ }
}

/**
 * Reload the page once per tab session to pick up the new build. Returns
 * false if a reload was already attempted (so the caller surfaces the error
 * instead of looping).
 */
export function reloadOnceForNewBuild(): boolean {
  if (readFlag()) return false;
  writeFlag(true);
  window.location.reload();
  return true;
}

/**
 * React.lazy with chunk-failure recovery: retry the import once (transient
 * network errors), then reload the page once (stale deploy). A further
 * failure propagates to the nearest error boundary.
 */
export function lazyWithRetry<P extends object>(factory: () => Promise<{ default: ComponentType<P> }>) {
  return lazy(async () => {
    try {
      const mod = await factory();
      writeFlag(false);
      return mod;
    } catch (error) {
      if (!isChunkLoadError(error)) throw error;
      await new Promise((resolve) => setTimeout(resolve, RETRY_DELAY_MS));
      try {
        const mod = await factory();
        writeFlag(false);
        return mod;
      } catch (retryError) {
        // Never resolves: the page is reloading.
        if (reloadOnceForNewBuild()) return new Promise<{ default: ComponentType<P> }>(() => {});
        throw retryError;
      }
    }
  });
}
