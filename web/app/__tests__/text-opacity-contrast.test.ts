import { describe, it, expect } from "vitest";
import { readFileSync, globSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join, relative } from "node:path";
import { parse } from "vue/compiler-sfc";

// Why this test exists
// --------------------
// Four elements failed Lighthouse's color-contrast audit because they stacked
// an `opacity-*` utility on top of `text-muted`. The muted token already sits
// near the WCAG AA floor (4.63:1 light, 7.76:1 dark), so dimming it pushed
// small text to 4.22:1 in dark mode and 2.68:1 in light. Writing this test
// then surfaced a worse instance Lighthouse never saw, because it only audits
// the homepage: the sparkline axis labels on the package page were 2.27:1.
//
// That is the point. "Make this a bit quieter" reads as a styling tweak, not
// an accessibility regression, and an audit that samples one URL will not
// catch it. So forbid the combination across every component instead.
//
// Opacity on non-text elements (borders, rules, decorative glyphs) stays fine.
// This only fires when a text-colour utility and an opacity utility land on
// the same element, and it honours aria-hidden — including inherited, since
// aria-hidden applies to the whole subtree and WCAG 1.4.3 exempts decorative
// text. Getting that inheritance right is why this walks the template AST
// rather than pattern-matching the source.

const appDir = join(dirname(fileURLToPath(import.meta.url)), "..");

const TEXT_COLOR = /\btext-(muted|primary|brand|brand-light)\b|\btext-\[var\(--color-/;
const OPACITY = /\bopacity-\d{1,3}\b/;

const ELEMENT = 1;
const ATTRIBUTE = 6;

interface Node {
  type: number;
  tag?: string;
  props?: Array<{ type: number; name: string; value?: { content?: string } }>;
  children?: Node[];
  loc?: { start: { line: number } };
}

function staticAttr(node: Node, name: string): string | undefined {
  return node.props?.find((p) => p.type === ATTRIBUTE && p.name === name)?.value?.content;
}

/** Collect elements that dim a text-colour token, skipping aria-hidden subtrees. */
function findOffenders(node: Node, ariaHidden: boolean, out: string[]): void {
  let hidden = ariaHidden;

  if (node.type === ELEMENT) {
    if (staticAttr(node, "aria-hidden") === "true") hidden = true;

    if (!hidden) {
      const cls = staticAttr(node, "class") ?? "";
      if (TEXT_COLOR.test(cls) && OPACITY.test(cls)) {
        out.push(`line ${node.loc?.start.line}: <${node.tag} class="${cls}">`);
      }
    }
  }

  for (const child of node.children ?? []) findOffenders(child, hidden, out);
}

describe("text colour tokens are never dimmed with opacity", () => {
  const files = globSync("**/*.vue", { cwd: appDir })
    .map((f) => join(appDir, f))
    .filter((f) => !f.includes("__tests__"));

  it("finds Vue components to check", () => {
    expect(files.length).toBeGreaterThan(5);
  });

  it.each(files.map((f) => [relative(appDir, f), f] as const))("%s", (_name, file) => {
    const ast = parse(readFileSync(file, "utf8")).descriptor.template?.ast;
    if (!ast) return;

    const offenders: string[] = [];
    findOffenders(ast as unknown as Node, false, offenders);

    expect(
      offenders,
      `Dimming a text-colour token with opacity drops it below the WCAG AA ` +
        `contrast floor for small text. Use the token at full strength, or ` +
        `mark the element aria-hidden="true" if it is purely decorative.`,
    ).toEqual([]);
  });
});
