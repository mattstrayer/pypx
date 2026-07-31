import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

// Why this test exists
// --------------------
// `--color-muted` was picked to clear WCAG AA against `--color-base`, and it
// did (4.63:1). But muted text is also rendered on `--color-raised` — the
// KbdHint chip, the header ⌘K hint, version pills, maintainer tags — where the
// same colour was only 4.39:1. Lighthouse never reported it, because it audits
// in dark mode by default and the shortfall only exists in light mode.
//
// The structural fault is that a token's contrast was validated against one
// surface and then used on several. So check every foreground token against
// every surface it can actually sit on, in both modes, rather than trusting
// that whoever adds the next pairing will redo the arithmetic.
//
// Surfaces are limited to those used as backgrounds behind text. `--color-
// subtle` is excluded deliberately: it only ever paints hairline rules and a
// grid gap, never a text background. If that changes, add it here.

const cssPath = join(dirname(fileURLToPath(import.meta.url)), "..", "assets", "css", "main.css");

const AA_NORMAL_TEXT = 4.5;

const SURFACES = ["base", "surface", "raised"] as const;
const FOREGROUNDS = ["muted", "primary", "brand"] as const;

function relativeLuminance(hex: string): number {
  const channels = hex
    .replace("#", "")
    .match(/../g)!
    .map((c) => parseInt(c, 16) / 255);
  const [r, g, b] = channels.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [relativeLuminance(a), relativeLuminance(b)].sort((x, y) => y - x);
  return (hi! + 0.05) / (lo! + 0.05);
}

/** Pull the `--color-*: #hex;` declarations out of one CSS block. */
function tokensIn(css: string, selector: string): Record<string, string> {
  const start = css.indexOf(selector);
  if (start === -1) throw new Error(`selector ${selector} not found in main.css`);
  const block = css.slice(start, css.indexOf("}", start));

  const out: Record<string, string> = {};
  for (const [, name, value] of block.matchAll(/--color-([\w-]+):\s*(#[0-9a-fA-F]{6})\s*;/g)) {
    out[name!] = value!;
  }
  return out;
}

describe("design tokens meet WCAG AA on every surface they are used on", () => {
  const css = readFileSync(cssPath, "utf8");
  const modes = {
    light: tokensIn(css, ":root {"),
    dark: tokensIn(css, ".dark {"),
  };

  for (const [mode, tokens] of Object.entries(modes)) {
    describe(mode, () => {
      it("defines every token the check needs", () => {
        for (const name of [...SURFACES, ...FOREGROUNDS]) {
          expect(tokens[name], `--color-${name} missing in ${mode}`).toMatch(/^#[0-9a-f]{6}$/i);
        }
      });

      for (const fg of FOREGROUNDS) {
        for (const surface of SURFACES) {
          it(`${fg} on ${surface}`, () => {
            const ratio = contrast(tokens[fg]!, tokens[surface]!);
            expect(
              Number(ratio.toFixed(2)),
              `--color-${fg} (${tokens[fg]}) on --color-${surface} (${tokens[surface]})`,
            ).toBeGreaterThanOrEqual(AA_NORMAL_TEXT);
          });
        }
      }
    });
  }
});

// Sibling of the fault above, one layer out. `--color-primary` was validated
// against the app's own surfaces, but `.prose code` also matches `pre > code`,
// where the surface is not a token at all: goldmark-highlighting inline-styles
// the <pre> with Monokai's #272822 and its matching #f8f8f2 foreground. An
// explicit colour on the child clobbered that, painting #18181b on #272822 —
// 1.19:1. The token test could not see it because the background never appears
// in main.css, and dark mode masked it because --color-primary is light there.
//
// So assert the ownership rule directly: highlighted blocks inherit their
// foreground from whoever set the background.
describe("highlighted code blocks do not inherit app text colour", () => {
  const css = readFileSync(cssPath, "utf8");

  it("resets colour to inherit on pre > code", () => {
    const start = css.indexOf("& pre code {");
    expect(start, "`& pre code` block not found in main.css").toBeGreaterThan(-1);
    const block = css.slice(start, css.indexOf("}", start));

    expect(block).toMatch(/color:\s*inherit\s*;/);
    // A --color-* token here would re-introduce the clobber.
    expect(block).not.toMatch(/color:\s*var\(--color-/);
  });
});
