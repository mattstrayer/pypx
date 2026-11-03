import { createHighlighter, type Highlighter } from "shiki";
import { plainCodeBlock } from "~/composables/useDocstringFormat";

let highlighterPromise: Promise<Highlighter> | null = null;

function getHighlighter(): Promise<Highlighter> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighter({
      themes: ["one-dark-pro", "github-light"],
      langs: ["python"],
    });
  }
  return highlighterPromise;
}

export async function highlightPython(code: string): Promise<string> {
  if (!import.meta.server) {
    return plainCodeBlock(code);
  }

  const highlighter = await getHighlighter();
  return highlighter.codeToHtml(code, {
    lang: "python",
    themes: {
      dark: "one-dark-pro",
      light: "github-light",
    },
  });
}
