import css from '../index.css?raw';
import { describe, expect, it } from 'vitest';
import { contrastRatio, mix, parseHex, type RGB } from '../theme/contrast';

/**
 * Reads the colour tokens straight out of index.css (dark values from the
 * @theme block, light overrides from :root[data-theme="light"]) and checks
 * the pairs the UI actually renders against WCAG AA:
 *   - text: 4.5:1
 *   - UI marks (status dots, chart series, focus ring): 3:1
 * If you change a token and this fails, tune the value — don't loosen the test.
 */

function block(startMarker: string): string {
  const start = css.indexOf(startMarker);
  if (start < 0) throw new Error(`Missing CSS block: ${startMarker}`);
  const open = css.indexOf('{', start);
  const close = css.indexOf('\n}', open);
  return css.slice(open + 1, close);
}

function readTokens(body: string): Record<string, string> {
  const tokens: Record<string, string> = {};
  for (const match of body.matchAll(/--color-([\w-]+):\s*([^;]+);/g)) {
    const [, name, value] = match;
    if (name && value) tokens[name] = value.trim();
  }
  return tokens;
}

const darkRaw = readTokens(block('@theme {'));
const lightRaw = { ...darkRaw, ...readTokens(block(':root[data-theme="light"] {')) };

function resolver(raw: Record<string, string>) {
  return (name: string): RGB => {
    let value = raw[name];
    for (let i = 0; i < 5 && value?.startsWith('var('); i++) {
      value = raw[value.replace(/^var\(--color-([\w-]+)\)$/, '$1')];
    }
    if (!value) throw new Error(`Unknown token --color-${name}`);
    return parseHex(value);
  };
}

const STATUSES = ['status-up', 'status-warning', 'status-down', 'status-unknown', 'status-maintenance', 'status-info'];
const TEXT = ['fg', 'fg-muted', 'fg-subtle'];
const SURFACES = ['canvas', 'surface-1', 'surface-2'];
const CHARTS = Array.from({ length: 8 }, (_, i) => `chart-${i + 1}`);

describe.each([
  ['dark', resolver(darkRaw)],
  ['light', resolver(lightRaw)],
] as const)('%s theme contrast', (_theme, color) => {
  const pairs: [string, string, number][] = [];
  for (const surface of SURFACES) {
    for (const text of TEXT) pairs.push([text, surface, 4.5]);
    // Status colours are used as text ("Down", "3 critical") as well as marks.
    for (const status of STATUSES) pairs.push([status, surface, 4.5]);
    pairs.push(['primary', surface, 4.5]);
    pairs.push(['focus', surface, 3]);
  }
  pairs.push(['on-primary', 'primary', 4.5]);
  // Solid danger button and solid status pills.
  for (const status of STATUSES) pairs.push(['on-status', status, 4.5]);
  for (const chart of CHARTS) pairs.push([chart, 'surface-1', 3]);

  it.each(pairs)('%s on %s ≥ %s:1', (fg, bg, min) => {
    expect(contrastRatio(color(fg), color(bg))).toBeGreaterThanOrEqual(min);
  });

  // Tinted chips (StatusPill): status text on a 12% wash of itself, over a
  // card or a nested panel.
  const chipCases = STATUSES.flatMap((status) => ['surface-1', 'surface-2'].map((surface) => [status, surface] as const));
  it.each(chipCases)('%s chip text over %s ≥ 4.5:1', (status, surface) => {
    const chip = mix(color(status), color(surface), 0.12);
    expect(contrastRatio(color(status), chip)).toBeGreaterThanOrEqual(4.5);
  });

  it('structural borders are distinguishable from the canvas', () => {
    expect(contrastRatio(color('line-strong'), color('surface-1'))).toBeGreaterThanOrEqual(1.8);
  });
});
