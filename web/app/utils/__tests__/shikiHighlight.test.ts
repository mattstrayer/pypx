import { describe, it, expect, vi } from "vitest";
import githubLight from "shiki/themes/github-light.mjs";
import oneDarkPro from "shiki/themes/one-dark-pro.mjs";
import { COLOR_REPLACEMENTS, highlightPython } from "../shikiHighlight";

function parse(html: string): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = html;
  return root;
}

function luminance(hex: string): number {
  const [r, g, b] = [1, 3, 5]
    .map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
    .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!;
}

function contrast(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi! + 0.05) / (lo! + 0.05);
}

describe("theme colours", () => {
  it.each([
    ["github-light", githubLight, "#ffffff"],
    ["one-dark-pro", oneDarkPro, "#282c34"],
  ] as const)("%s meets 4.5:1 for every token colour", (name, theme, background) => {
    const replacements: Record<string, string> = COLOR_REPLACEMENTS[name] ?? {};
    const failing = (theme.tokenColors ?? [])
      .map((rule) => rule.settings.foreground?.toLowerCase())
      .filter((colour): colour is string => !!colour)
      .map((colour) => replacements[colour] ?? colour)
      .filter((colour) => contrast(colour, background) < 4.5);
    expect(failing).toEqual([]);
  });
});

describe("highlightPython", () => {
  it("emits token spans with both theme colours", async () => {
    const root = parse(await highlightPython('def f():\n    return "x"'));
    expect(root.querySelector("pre.shiki")).not.toBeNull();
    const style = root.querySelector(".line span")?.getAttribute("style") ?? "";
    expect(style).toContain("--shiki-light:");
    expect(style).toContain("--shiki-dark:");
  });

  it("applies the replacement colours", async () => {
    const root = parse(await highlightPython("# comment"));
    const style = root.querySelector(".line span")?.getAttribute("style")?.toLowerCase() ?? "";
    expect(style).not.toContain("#7f848e");
    expect(style).toContain(COLOR_REPLACEMENTS["one-dark-pro"]?.["#7f848e"]);
  });

  it("keeps the exact source text", async () => {
    const code = `s = "&lt;&amp;lt;"\nt = a < b > c & d\nu = "$&"`;
    const root = parse(await highlightPython(code));
    expect(root.querySelector("code")?.textContent).toBe(code);
  });

  it.each([
    `</code></pre><img src=x onerror="alert(1)">`,
    `<script>alert(1)</script>`,
    `"><svg onload=alert(1)>`,
    `# <iframe src="javascript:alert(1)"></iframe>`,
  ])("renders %s as text", async (code) => {
    const root = parse(await highlightPython(code));
    expect(root.querySelector("img, script, svg, iframe")).toBeNull();
    expect(root.querySelector("code")?.textContent).toBe(code);
  });

  // script-src in the Caddyfile has no 'wasm-unsafe-eval', so a WASM engine would fail.
  it("does not use WebAssembly", async () => {
    const calls = [
      vi.spyOn(WebAssembly, "instantiate"),
      vi.spyOn(WebAssembly, "compile"),
      vi.spyOn(WebAssembly, "Module"),
    ];
    await highlightPython("x = 1");
    for (const call of calls) expect(call).not.toHaveBeenCalled();
    vi.restoreAllMocks();
  });
});
