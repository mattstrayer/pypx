import { createHighlighter, type Highlighter } from "shiki";

let highlighterPromise: Promise<Highlighter> | null = null;

function getHighlighter(): Promise<Highlighter> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighter({
      themes: ["one-dark-pro"],
      langs: ["python"],
    });
  }
  return highlighterPromise;
}

export async function highlightPython(code: string): Promise<string> {
  if (!import.meta.server) {
    return `<pre><code class="language-python">${code}</code></pre>`;
  }

  const highlighter = await getHighlighter();
  return highlighter.codeToHtml(code, {
    lang: "python",
    theme: "one-dark-pro",
  });
}
