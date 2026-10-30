// @vitest-environment node
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

const clientDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

describe('material symbols subset', () => {
  it('includes every icon referenced in src (run `npm run icons -w client` if this fails)', () => {
    expect(() =>
      execFileSync(process.execPath, ['scripts/subset-icons.mjs', '--check'], { cwd: clientDir, stdio: 'pipe' }),
    ).not.toThrow();
  });
});
