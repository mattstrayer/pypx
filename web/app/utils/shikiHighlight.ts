import { createHighlighterCore, type HighlighterCore } from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";

// Docstring code is 11px, and these theme colours fall under 4.5:1 on their
// theme's background. Each keeps its hue where it has one. The near-white
// github-light colours expect a background the CSS removes, so they use the
// theme's text colour.
export const COLOR_REPLACEMENTS: Record<string, Record<string, string>> = {
  "github-light": {
    "#e36209": "#bd4b00",
    "#f6f8fa": "#24292e",
    "#fafbfc": "#24292e",
  },
  "one-dark-pro": {
    "#5c6370": "#969ba5",
    "#7f848e": "#969ba5",
    "#be5046": "#e8818a",
    "#e06c75": "#e67c85",
    "#f44747": "#ff8080",
  },
};

let highlighterPromise: Promise<HighlighterCore> | null = null;

// Fine-grained imports keep other languages and themes out of the build. The
// JavaScript engine avoids WASM, which script-src in the Caddyfile does not allow.
function getHighlighter(): Promise<HighlighterCore> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighterCore({
      themes: [import("shiki/themes/one-dark-pro.mjs"), import("shiki/themes/github-light.mjs")],
      langs: [import("shiki/langs/python.mjs")],
      engine: createJavaScriptRegexEngine(),
    });
  }
  return highlighterPromise;
}

export async function highlightPython(code: string): Promise<string> {
  const highlighter = await getHighlighter();
  return highlighter.codeToHtml(code, {
    lang: "python",
    themes: {
      dark: "one-dark-pro",
      light: "github-light",
    },
    // main.css picks a colour from these variables for the active colour mode.
    defaultColor: false,
    colorReplacements: COLOR_REPLACEMENTS,
  });
}
