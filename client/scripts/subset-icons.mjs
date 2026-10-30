#!/usr/bin/env node
/**
 * Subset the Material Symbols icon font to the icons the app actually uses.
 *
 * The full variable font is ~744 KB of woff2 for ~3,800 icons; the UI uses a
 * few hundred. This script:
 *   1. collects candidate names from src/ (string literals and JSX text that
 *      look like icon names — a deliberate superset),
 *   2. shapes each candidate with HarfBuzz; a name is an icon when the font's
 *      `liga` feature turns it into a single glyph,
 *   3. subsets the font to the letters plus those ligature glyphs, with
 *      layout closure disabled so ligatures for unused icons are dropped,
 *      keeping the wght axis.
 *
 * Output (committed, regenerated on every build):
 *   src/assets/fonts/material-symbols-outlined.subset.woff2
 *   src/assets/fonts/material-symbols-icons.json   (the included names)
 *
 * Usage: node scripts/subset-icons.mjs [--check]
 *   --check  exit 1 if the committed manifest is out of date (no writes)
 */
import { readFile, readdir, writeFile, mkdir } from 'node:fs/promises';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import * as hb from 'harfbuzzjs';

const require = createRequire(import.meta.url);
const wawoff2 = require('wawoff2');

const clientDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const srcDir = path.join(clientDir, 'src');
const outDir = path.join(srcDir, 'assets', 'fonts');
const outFont = path.join(outDir, 'material-symbols-outlined.subset.woff2');
const outManifest = path.join(outDir, 'material-symbols-icons.json');
const sourceFont = require.resolve('@fontsource-variable/material-symbols-outlined/files/material-symbols-outlined-latin-wght-normal.woff2');
const subsetWasm = path.join(path.dirname(require.resolve('harfbuzzjs')), 'harfbuzz-subset.wasm');

// Characters icon names are spelled with (ligature inputs) plus space.
const NAME_CHARS = ' _0123456789abcdefghijklmnopqrstuvwxyz';
// hb-subset.h: HB_SUBSET_FLAGS_NO_LAYOUT_CLOSURE
const HB_SUBSET_FLAGS_NO_LAYOUT_CLOSURE = 0x200;
const HB_MEMORY_MODE_WRITABLE = 2;

const LITERAL = /(['"`])([a-z][a-z0-9_]+)\1/g;
const JSX_TEXT = />\s*([a-z][a-z0-9_]+)\s*</g;

async function* sourceFiles(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== '__tests__') yield* sourceFiles(full);
    } else if (/\.(tsx?|css)$/.test(entry.name)) {
      yield full;
    }
  }
}

export async function collectCandidates(dir = srcDir) {
  const names = new Set();
  for await (const file of sourceFiles(dir)) {
    const text = await readFile(file, 'utf8');
    for (const m of text.matchAll(LITERAL)) names.add(m[2]);
    for (const m of text.matchAll(JSX_TEXT)) names.add(m[1]);
  }
  return names;
}

/** Map candidate names to ligature glyph ids; non-icons are dropped. */
function resolveIcons(fontData, candidates) {
  const face = new hb.Face(new hb.Blob(fontData));
  const font = new hb.Font(face);
  const icons = new Map();
  for (const name of candidates) {
    const buffer = new hb.Buffer();
    buffer.addText(name);
    buffer.guessSegmentProperties();
    hb.shape(font, buffer);
    const infos = buffer.getGlyphInfos();
    if (infos.length === 1) icons.set(name, infos[0].codepoint);
  }
  return icons;
}

async function subsetFont(fontData, glyphIds) {
  const { instance } = await WebAssembly.instantiate(await readFile(subsetWasm));
  const ex = instance.exports;

  const fontPtr = ex.malloc(fontData.byteLength);
  new Uint8Array(ex.memory.buffer).set(fontData, fontPtr);
  const blob = ex.hb_blob_create(fontPtr, fontData.byteLength, HB_MEMORY_MODE_WRITABLE, 0, 0);
  const face = ex.hb_face_create(blob, 0);
  ex.hb_blob_destroy(blob);

  const input = ex.hb_subset_input_create_or_fail();
  if (!input) throw new Error('hb_subset_input_create_or_fail failed');
  const unicodes = ex.hb_subset_input_unicode_set(input);
  for (const ch of NAME_CHARS) ex.hb_set_add(unicodes, ch.codePointAt(0));
  const glyphs = ex.hb_subset_input_glyph_set(input);
  for (const gid of glyphIds) ex.hb_set_add(glyphs, gid);
  ex.hb_subset_input_set_flags(input, ex.hb_subset_input_get_flags(input) | HB_SUBSET_FLAGS_NO_LAYOUT_CLOSURE);

  const subset = ex.hb_subset_or_fail(face, input);
  ex.hb_subset_input_destroy(input);
  if (!subset) throw new Error('hb_subset_or_fail failed');

  const outBlob = ex.hb_face_reference_blob(subset);
  const ptr = ex.hb_blob_get_data(outBlob, 0);
  const len = ex.hb_blob_get_length(outBlob);
  const result = new Uint8Array(ex.memory.buffer, ptr, len).slice();

  ex.hb_blob_destroy(outBlob);
  ex.hb_face_destroy(subset);
  ex.hb_face_destroy(face);
  ex.free(fontPtr);
  return result;
}

async function main() {
  const check = process.argv.includes('--check');
  const fontData = await wawoff2.decompress(await readFile(sourceFont));
  const icons = resolveIcons(fontData, await collectCandidates());
  const names = [...icons.keys()].sort();
  const manifest = `${JSON.stringify(names, null, 2)}\n`;

  if (check) {
    const current = await readFile(outManifest, 'utf8').catch(() => '');
    if (current !== manifest) {
      console.error('material-symbols subset is out of date; run `npm run icons -w client`.');
      process.exit(1);
    }
    console.log(`material-symbols subset up to date (${names.length} icons).`);
    return;
  }

  const ttf = await subsetFont(fontData, icons.values());
  const woff2 = await wawoff2.compress(ttf);
  await mkdir(outDir, { recursive: true });
  await writeFile(outFont, woff2);
  await writeFile(outManifest, manifest);
  console.log(`material-symbols: ${names.length} icons, ${(woff2.byteLength / 1024).toFixed(1)} KB woff2`);
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  await main();
}
