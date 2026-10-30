/**
 * Reads a resolved colour token (e.g. `readToken('status-down')`) for code
 * that cannot use `var()`, such as <canvas> drawing. Call it at draw time so
 * the current theme applies; re-draw on THEME_CHANGE_EVENT.
 */
export function readToken(name: string, fallback = '#888888'): string {
  if (typeof window === 'undefined') return fallback;
  const value = getComputedStyle(document.documentElement).getPropertyValue(`--color-${name}`).trim();
  return value || fallback;
}

/** Same as readToken, with an alpha channel (for canvas fills/strokes). */
export function readTokenAlpha(name: string, alpha: number, fallback = '#888888'): string {
  const hex = readToken(name, fallback).replace('#', '');
  if (!/^[0-9a-fA-F]{6}$/.test(hex)) return readToken(name, fallback);
  const [r, g, b] = [0, 2, 4].map((i) => Number.parseInt(hex.slice(i, i + 2), 16));
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}
