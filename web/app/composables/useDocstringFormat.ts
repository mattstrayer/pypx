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

// What Sphinx displays for an escaped role or default-role target: the title of
// "Title <a.b>", the last part of "~a.b", and "!a.b" without its "!".
function roleDisplayText(target: string): string {
  const titled = target.match(/^(.+?)\s*&lt;.*&gt;$/);
  if (titled) return titled[1] ?? target;
  if (target.startsWith("~")) return target.split(".").at(-1)?.replace(/^~/, "") ?? target;
  return target.replace(/^!/, "");
}

// One pass, so markup inside a code span stays as written. Bold needs text that
// touches both pairs of stars, so "2 ** 3 ** 4" and "**kwargs" are left alone.
const INLINE_MARKUP =
  /(?::[a-zA-Z]+)+:`([^`]+)`|``([^`]+?)``|`([^`]+?)`|\*\*([^\s*](?:[^*]*[^\s*])?)\*\*/g;

/** Escapes one piece of docstring text and renders its reST inline markup as HTML. */
export function formatInline(text: string): string {
  return escapeHtml(text).replace(
    INLINE_MARKUP,
    (_, role?: string, literal?: string, single?: string, bold?: string) => {
      if (literal !== undefined) return `<code>${literal}</code>`;
      if (bold !== undefined) return `<strong>${bold}</strong>`;
      return `<code>${roleDisplayText(role ?? single ?? "")}</code>`;
    },
  );
}

// Same rule as Python's inspect.cleandoc: a docstring's first line starts right after
// the opening quotes, so its indent says nothing about the margin of the other lines.
function dedent(lines: string[]): string[] {
  const [first = "", ...rest] = lines;
  const indents = rest
    .filter((l) => l.trim().length > 0)
    .map((l) => l.match(/^\s*/)?.[0].length ?? 0);
  const margin = indents.length > 0 ? Math.min(...indents) : 0;
  return [first.trimStart(), ...rest.map((l) => l.slice(margin))];
}

// Code stays as source text until a renderer needs it, so no caller has to
// decode HTML to get it back.
type DocstringPart = { type: "html"; html: string } | { type: "code"; code: string };

function html(value: string): DocstringPart {
  return { type: "html", html: value };
}

function plainCodeBlock(code: string): string {
  return `<pre><code class="language-python">${escapeHtml(code)}</code></pre>`;
}

type BlockType = "paragraph" | "code" | "directive" | "hidden" | "heading";

interface Block {
  type: BlockType;
  lines: string[];
}

// A section title at the margin with a line of 3+ dashes under it, as NumPy writes
// "Examples" and "Notes".
function isSectionHeading(title: string | undefined, underline: string | undefined): boolean {
  return (
    title !== undefined &&
    underline !== undefined &&
    /^\S/.test(title) &&
    !title.startsWith(">>>") &&
    /^-{3,}\s*$/.test(underline)
  );
}

function classifyBlocks(rawBlocks: string[][], prevBlockLines: string[][]): Block[] {
  return rawBlocks.map((lines, i) => {
    const firstLine = lines[0] ?? "";

    const prevLine = prevBlockLines[i]?.at(-1)?.trimEnd() ?? "";
    if (lines.length === 2 && !prevLine.endsWith("::") && isSectionHeading(lines[0], lines[1])) {
      return { type: "heading", lines };
    }

    // RST directive: starts with ".. word::" (possibly indented)
    if (/^\s*\.\.\s+\w+::/.test(firstLine)) {
      return { type: "directive", lines };
    }

    // Preceding block ended with "::" → this is a code block
    const prev = prevBlockLines[i];
    if (prev && prev.length > 0) {
      const lastPrevLine = prev[prev.length - 1] ?? "";
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

function renderBlock(block: Block): DocstringPart | null {
  if (block.type === "hidden") {
    return null;
  }

  if (block.type === "heading") {
    return html(`<h4 class="section-label">${escapeHtml(block.lines[0] ?? "")}</h4>`);
  }

  if (block.type === "code") {
    // Dedent the code block content
    const nonEmpty = block.lines.filter((l) => l.trim().length > 0);
    const minIndent =
      nonEmpty.length > 0
        ? Math.min(
            ...nonEmpty.map((l) => {
              const match = l.match(/^(\s*)/);
              return match ? (match[1]?.length ?? 0) : 0;
            }),
          )
        : 0;

    const dedented = block.lines.map((l) => l.slice(minIndent));
    return { type: "code", code: dedented.join("\n").trimEnd() };
  }

  if (block.type === "directive") {
    // Parse ".. type:: content\n   continuation"
    const firstLine = block.lines[0] ?? "";
    const directiveMatch = firstLine.match(/^\s*\.\.\s+(\w+)::\s*(.*)/);
    if (directiveMatch) {
      const directiveType = escapeHtml(directiveMatch[1] ?? "");
      const rest = [directiveMatch[2] ?? "", ...block.lines.slice(1).map((l) => l.trim())]
        .filter(Boolean)
        .join(" ");
      const content = formatInline(rest);
      const label = content ? `${directiveType}: ${content}` : directiveType;
      return html(`<p class="text-[11px] italic text-zinc-400 dark:text-zinc-600">${label}</p>`);
    }
    // Fallback
    const content = escapeHtml(block.lines.join(" ").trim());
    return html(`<p class="text-[11px] italic text-zinc-400 dark:text-zinc-600">${content}</p>`);
  }

  // Paragraph: join lines, then apply inline formatting
  return html(`<p>${formatInline(block.lines.join(" ").trim())}</p>`);
}

function parseDocstring(raw: string): DocstringPart[] {
  if (!raw || !raw.trim()) return [];

  // Normalize line endings
  const normalized = raw.replace(/\r\n/g, "\n").replace(/\r/g, "\n");

  // Split into lines and dedent
  const lines = dedent(normalized.split("\n"));

  // Split into raw blocks by blank lines. A section heading is a block of its own,
  // even with no blank line after it. It must start a block, and not the literal
  // block after a "::", so dashes in code stay in the code.
  const rawBlocks: string[][] = [];
  let current: string[] = [];
  const afterLiteralMarker = () => rawBlocks.at(-1)?.at(-1)?.trimEnd().endsWith("::") ?? false;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i] ?? "";
    if (line.trim() === "") {
      if (current.length > 0) {
        rawBlocks.push(current);
        current = [];
      }
    } else if (
      current.length === 0 &&
      !afterLiteralMarker() &&
      isSectionHeading(line, lines[i + 1])
    ) {
      rawBlocks.push([line, lines[i + 1] ?? ""]);
      i++;
    } else {
      current.push(line);
    }
  }
  if (current.length > 0) {
    rawBlocks.push(current);
  }

  if (rawBlocks.length === 0) return [];

  // Build prev-block lookup for "::" detection
  const prevBlockLines: string[][] = rawBlocks.map((_, i) =>
    i === 0 ? [] : (rawBlocks[i - 1] ?? []),
  );

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
        const lastLine = block.lines[block.lines.length - 1] ?? "";
        if (lastLine.trimEnd().endsWith("::")) {
          // Strip trailing "::" (and potentially whitespace before it)
          const stripped = { ...block, lines: [...block.lines] };
          stripped.lines[stripped.lines.length - 1] = lastLine.trimEnd().replace(/\s*::$/, "");
          // If the line becomes empty after stripping, remove it
          if ((stripped.lines[stripped.lines.length - 1] ?? "").trim() === "") {
            stripped.lines.pop();
          }
          if (stripped.lines.length === 0) return null;
          return renderBlock(stripped);
        }
      }
    }
    return renderBlock(block);
  });

  return rendered.filter((part) => part !== null);
}

export function formatDocstring(raw: string): string {
  return parseDocstring(raw)
    .map((part) => (part.type === "code" ? plainCodeBlock(part.code) : part.html))
    .join("\n");
}

/** Like formatDocstring, but `highlight` renders each code block from its source text. */
export async function renderDocstring(
  raw: string,
  highlight: (code: string) => Promise<string>,
): Promise<string> {
  const rendered = await Promise.all(
    parseDocstring(raw).map((part) => (part.type === "code" ? highlight(part.code) : part.html)),
  );
  return rendered.join("\n");
}
