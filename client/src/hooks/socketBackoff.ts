const RECONNECT_BASE_DELAY = 1000;
const RECONNECT_MAX_DELAY = 30000;
/** High-frequency events are coalesced and delivered at most this often. */
export const BATCH_WINDOW_MS = 250;

/**
 * Exponential backoff with "equal jitter": half the capped delay is fixed,
 * the other half random, so a fleet of tabs doesn't reconnect in lockstep
 * after a server restart.
 */
export function reconnectDelay(attempt: number, random: () => number = Math.random): number {
  const capped = Math.min(RECONNECT_BASE_DELAY * 2 ** attempt, RECONNECT_MAX_DELAY);
  return Math.round(capped / 2 + random() * (capped / 2));
}
