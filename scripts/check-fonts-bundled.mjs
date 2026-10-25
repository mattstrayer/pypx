#!/usr/bin/env node
// check-fonts-bundled.mjs — verify the brand fonts survive the Nuxt build.
//
// Why this exists: Geist and Geist Mono were previously pulled in with an
// `@import url(...)` to Google Fonts at the top of main.css. Vite stripped
// that import during the build, so the deployed CSS shipped zero @font-face
// rules and every page silently rendered in the system fallback. Nothing
// failed — not a test, not a lint, not a Lighthouse audit — because a missing
// webfont is invisible to everything except a human looking at the page.
//
// @nuxt/fonts now self-hosts the files, but the underlying hazard remains:
// font loading is a build-time side effect with no runtime error. This script
// turns that silent failure into a loud one, and it does so by asserting on
// the build output rather than on the source, so it catches any future cause
// — a module removal, a config regression, an upstream provider change — not
// just the specific @import mistake that started it.
//
// Run from the repo root or from web/ after `pnpm build`.

import { readdirSync, readFileSync, existsSync } from 'node:fs';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const buildDir = join(repoRoot, 'web', '.output');
const publicDir = join(buildDir, 'public');
const serverChunksDir = join(buildDir, 'server', 'chunks');

const REQUIRED_FAMILIES = ['Geist', 'Geist Mono'];

const errors = [];

if (!existsSync(publicDir)) {
  console.error(`error: no build output at ${publicDir} — run \`pnpm build\` in web/ first.`);
  process.exit(2);
}

function filesUnder(dir, pattern) {
  if (!existsSync(dir)) return [];
  return readdirSync(dir, { recursive: true, withFileTypes: true })
    .filter((e) => e.isFile() && pattern.test(e.name))
    .map((e) => join(e.parentPath, e.name));
}

// @nuxt/fonts has moved its output between releases: 0.x put @font-face in
// the client CSS and the files in /_fonts/; 1.x inlines @font-face into the
// SSR HTML (so it lives in the server bundle) and emits files under
// /_nuxt/fonts/. Search the whole build output rather than one layout, and
// assert on what the rules reference, so the next move cannot break this
// check while the fonts still load.
const sources = [
  ...filesUnder(publicDir, /\.css$/),
  ...filesUnder(serverChunksDir, /\.m?js$/),
].map((f) => readFileSync(f, 'utf8'));

// 1. The build must declare @font-face for every brand family.
const faces = sources.flatMap((src) => src.match(/@font-face\s*\{[^}]*\}/g) ?? []);

function familyOf(face) {
  const m = face.match(/font-family:\s*\\?["']?([^"';}\\]+)/);
  return m ? m[1].trim() : '';
}

for (const family of REQUIRED_FAMILIES) {
  // Match the family name exactly, not as a prefix: "Geist Mono" and the
  // metric-matched "Geist Fallback: Arial" faces both start with "Geist".
  if (!faces.some((face) => familyOf(face) === family)) {
    errors.push(`no @font-face declares font-family: ${family}`);
  }
}

// 2. Every font file a brand @font-face points at must be emitted, or the
//    rule 404s and the page falls back to the system font.
const brandFaces = faces.filter((face) => REQUIRED_FAMILIES.includes(familyOf(face)));
const fontUrls = new Set(
  brandFaces.flatMap((face) => [...face.matchAll(/url\(\\?["']?([^"')\\]+)/g)].map((m) => m[1])),
);
if (brandFaces.length > 0 && fontUrls.size === 0) {
  errors.push('brand @font-face rules reference no font files');
}
for (const url of fontUrls) {
  if (/^https?:/.test(url)) continue; // reported by check 3
  if (!existsSync(join(publicDir, url))) {
    errors.push(`@font-face references ${url}, which is not in ${publicDir}`);
  }
}

// 3. Nothing may point at a third-party font host. Those requests would be
//    blocked by our CSP (font-src 'self') and would put a render-blocking
//    third party back on the critical path.
const thirdParty = sources.flatMap(
  (src) => src.match(/https?:\/\/[^"')\s]*(?:googleapis|gstatic)\.com[^"')\s]*/g) ?? [],
);
if (thirdParty.length > 0) {
  errors.push(`build output references a third-party font host: ${thirdParty[0]}`);
}

if (errors.length > 0) {
  console.error('FAIL: brand fonts are not correctly bundled\n');
  for (const e of errors) console.error(`  - ${e}`);
  console.error('\nSee web/nuxt.config.ts (fonts) and web/app/assets/css/main.css.');
  process.exit(1);
}

console.log(
  `ok: ${brandFaces.length} brand @font-face rules, ${fontUrls.size} font files, no third-party font hosts`,
);
