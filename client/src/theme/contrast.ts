/**
 * WCAG 2.x contrast helpers. Used by the token contrast test
 * (src/__tests__/contrast.test.ts); kept dependency-free so it can also be
 * run ad hoc from a script.
 */

export type RGB = [number, number, number];

export function parseHex(hex: string): RGB {
  const clean = hex.trim().replace('#', '');
  const full = clean.length === 3 ? clean.split('').map((c) => c + c).join('') : clean;
  if (!/^[0-9a-fA-F]{6}$/.test(full)) throw new Error(`Not a hex colour: ${hex}`);
  return [0, 2, 4].map((i) => Number.parseInt(full.slice(i, i + 2), 16)) as RGB;
}

function channel(value: number): number {
  const c = value / 255;
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}

export function relativeLuminance([r, g, b]: RGB): number {
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

export function contrastRatio(a: RGB, b: RGB): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

/** Composites `top` at `alpha` over an opaque `bottom` (sRGB, like the browser). */
export function mix(top: RGB, bottom: RGB, alpha: number): RGB {
  return top.map((c, i) => Math.round(c * alpha + (bottom[i] ?? 0) * (1 - alpha))) as RGB;
}
