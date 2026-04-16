/**
 * Formats raw Python docstrings into HTML with paragraphs, code spans, and code blocks.
 */

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function applyInlineFormatting(text: string): string {
  // Strip rst role prefixes like :func:`name` → <code>name</code>
  text = text.replace(/:[a-z]+:`([^`]+)`/g, "<code>$1</code>");
  // Double backticks → <code>
  text = text.replace(/``([^`]+?)``/g, "<code>$1</code>");
  // Single backticks → <code>
  text = text.replace(/`([^`]+?)`/g, "<code>$1</code>");
  return text;
}

function dedent(lines: string[]): string[] {
  if (lines.length === 0) return lines;

  // Find minimum indentation across all non-empty lines
  const nonEmpty = lines.filter((l) => l.trim().length > 0);
  if (nonEmpty.length === 0) return lines;

  const minIndent = Math.min(
    ...nonEmpty.map((l) => {
      const match = l.match(/^(\s*)/);
      return match ? match[1].length : 0;
    }),
  );

  if (minIndent === 0) return lines;

  return lines.map((l) => l.slice(minIndent));
}

type BlockType = "paragraph" | "code" | "directive" | "hidden";

interface Block {
  type: BlockType;
  lines: string[];
}

function classifyBlocks(rawBlocks: string[][], prevBlockLines: string[][]): Block[] {
  return rawBlocks.map((lines, i) => {
    const firstLine = lines[0] ?? "";

    // RST directive: starts with ".. word::" (possibly indented)
    if (/^\s*\.\.\s+\w+::/.test(firstLine)) {
      return { type: "directive", lines };
    }

    // Preceding block ended with "::" → this is a code block
    const prev = prevBlockLines[i];
    if (prev && prev.length > 0) {
      const lastPrevLine = prev[prev.length - 1];
      if (lastPrevLine.trimEnd().endsWith("::")) {
        return { type: "code", lines };
      }
    }

    // All lines indented 4+ spaces → code block (with exclusions for prose/rst)
    const nonEmpty = lines.filter((l) => l.trim().length > 0);
    if (nonEmpty.length > 0 && nonEmpty.every((l) => /^ {4}/.test(l))) {
      // Don't classify as code if it contains rst field markers
      const hasRstFields = nonEmpty.some((l) => /^\s*:[a-z]+[\s`]/.test(l.trim()));
      // Don't classify as code if it reads like prose: contains sentences
      // ending with punctuation AND no obvious Python syntax starters.
      const joined = nonEmpty.map((l) => l.trim()).join(" ");
      const looksLikeProse =
        (/[.!?](\s|$)/.test(joined) || /[.!?]$/.test(joined.trim())) &&
        !nonEmpty.some((l) => /^\s*>>>/.test(l)) &&
        !nonEmpty.some((l) => /^\s*(import |from |def |class |assert |raise )/.test(l.trim()));
      if (!hasRstFields && !looksLikeProse) {
        return { type: "code", lines };
      }
    }

    // Lines start with >>> → REPL code block
    if (nonEmpty.some((l) => /^\s*>>>/.test(l))) {
      return { type: "code", lines };
    }

    // Hide rst meta directives (:meta private:, :meta internal:, etc.)
    if (nonEmpty.some((l) => /^\s*:meta\s/.test(l.trim()))) {
      return { type: "hidden", lines };
    }

    return { type: "paragraph", lines };
  });
}

function renderBlock(block: Block): string {
  if (block.type === "hidden") {
    return "";
  }

  if (block.type === "code") {
    // Dedent the code block content
    const nonEmpty = block.lines.filter((l) => l.trim().length > 0);
    const minIndent =
      nonEmpty.length > 0
        ? Math.min(
            ...nonEmpty.map((l) => {
              const match = l.match(/^(\s*)/);
              return match ? match[1].length : 0;
            }),
          )
        : 0;

    const dedented = block.lines.map((l) => l.slice(minIndent));
    const content = escapeHtml(dedented.join("\n").trimEnd());
    return `<pre><code class="language-python">${content}</code></pre>`;
  }

  if (block.type === "directive") {
    // Parse ".. type:: content\n   continuation"
    const firstLine = block.lines[0] ?? "";
    const directiveMatch = firstLine.match(/^\s*\.\.\s+(\w+)::\s*(.*)/);
    if (directiveMatch) {
      const directiveType = escapeHtml(directiveMatch[1]);
      const rest = [directiveMatch[2], ...block.lines.slice(1).map((l) => l.trim())]
        .filter(Boolean)
        .join(" ");
      const content = escapeHtml(rest);
      const label = content ? `${directiveType}: ${content}` : directiveType;
      return `<p class="text-[11px] italic text-zinc-400 dark:text-zinc-600">${label}</p>`;
    }
    // Fallback
    const content = escapeHtml(block.lines.join(" ").trim());
    return `<p class="text-[11px] italic text-zinc-400 dark:text-zinc-600">${content}</p>`;
  }

  // Paragraph: join lines, escape, then apply inline formatting
  const text = block.lines.join(" ").trim();
  const escaped = escapeHtml(text);
  const formatted = applyInlineFormatting(escaped);
  return `<p>${formatted}</p>`;
}

export function formatDocstring(raw: string): string {
  if (!raw || !raw.trim()) return "";

  // Normalize line endings
  const normalized = raw.replace(/\r\n/g, "\n").replace(/\r/g, "\n");

  // Split into lines and dedent
  const lines = dedent(normalized.split("\n"));

  // Split into raw blocks by blank lines
  const rawBlocks: string[][] = [];
  let current: string[] = [];

  for (const line of lines) {
    if (line.trim() === "") {
      if (current.length > 0) {
        rawBlocks.push(current);
        current = [];
      }
    } else {
      current.push(line);
    }
  }
  if (current.length > 0) {
    rawBlocks.push(current);
  }

  if (rawBlocks.length === 0) return "";

  // Build prev-block lookup for "::" detection
  const prevBlockLines: string[][] = rawBlocks.map((_, i) => (i === 0 ? [] : rawBlocks[i - 1]));

  const blocks = classifyBlocks(rawBlocks, prevBlockLines);

  // Filter out paragraph blocks that immediately follow a "::" block
  // (the "::" marker paragraph that ends with "::" becomes the intro, don't double-render)
  // Actually we render them all — the "::" ending lines are part of the paragraph text,
  // just strip the trailing "::" from paragraphs that serve as code introducers.
  const rendered = blocks.map((block, i) => {
    // If this is a paragraph ending with "::" and next block is code, strip the "::"
    if (block.type === "paragraph") {
      const nextBlock = blocks[i + 1];
      if (nextBlock?.type === "code") {
        const lastLine = block.lines[block.lines.length - 1];
        if (lastLine.trimEnd().endsWith("::")) {
          // Strip trailing "::" (and potentially whitespace before it)
          const stripped = { ...block, lines: [...block.lines] };
          stripped.lines[stripped.lines.length - 1] = lastLine.trimEnd().replace(/\s*::$/, "");
          // If the line becomes empty after stripping, remove it
          if (stripped.lines[stripped.lines.length - 1].trim() === "") {
            stripped.lines.pop();
          }
          if (stripped.lines.length === 0) return null;
          return renderBlock(stripped);
        }
      }
    }
    return renderBlock(block);
  });

  return rendered.filter(Boolean).join("\n");
}
