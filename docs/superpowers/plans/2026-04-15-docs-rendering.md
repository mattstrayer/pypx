# Docs Rendering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace plain-text docs rendering with semantically highlighted Python signatures and formatted docstrings with Shiki-highlighted code blocks.

**Architecture:** Two Vue components (`PySignature.vue`, `PyDocstring.vue`) render structured data from the existing API. A composable (`useDocstringFormat.ts`) parses raw docstring text into HTML with paragraphs, code spans, and code blocks. Shiki runs SSR-only to highlight code blocks. CSS custom properties define the token color palette.

**Tech Stack:** Vue 3, Nuxt 4, Shiki (SSR), Vitest, Tailwind CSS 4

**Spec:** `docs/superpowers/specs/2026-04-15-docs-rendering-design.md`

---

## Task 1: Add API handler fields (kind + default on DocParam)

**Files:**
- Modify: `api/internal/handler/docs.go`
- Modify: `web/app/types/api.ts`

- [ ] **Step 1: Add Kind and Default to Go DocParam**

In `api/internal/handler/docs.go`, update the `DocParam` struct:

```go
type DocParam struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description"`
	Kind        string `json:"kind,omitempty"`
	Default     string `json:"default,omitempty"`
}
```

Then update the `convertFunction` method to populate these fields. Find the loop that builds `sym.Parameters` and change:

```go
for _, p := range fn.Parameters {
	dp := DocParam{Name: p.Name}
	if p.Type != nil {
		dp.Type = p.Type.Raw
	}
	if p.DocParam != nil {
		dp.Description = p.DocParam.Description
	}
	dp.Kind = string(p.Kind)
	dp.Default = p.Default
	sym.Parameters = append(sym.Parameters, dp)
}
```

- [ ] **Step 2: Update TypeScript DocParam interface**

In `web/app/types/api.ts`, update `DocParam`:

```ts
export interface DocParam {
  name: string;
  type?: string;
  description: string;
  kind?: string;
  default?: string;
}
```

- [ ] **Step 3: Run API tests**

```bash
cd api && go test ./internal/handler/ -v -run TestDocs -count=1
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add api/internal/handler/docs.go web/app/types/api.ts
git commit -m "feat(api): add kind and default fields to DocParam"
```

---

## Task 2: CSS custom properties for Python token colors

**Files:**
- Modify: `web/app/assets/css/main.css`

- [ ] **Step 1: Add Python syntax color variables**

In `web/app/assets/css/main.css`, add to the existing `:root` block (after the `--color-brand-*` variables):

```css
/* Python syntax highlighting tokens */
--py-keyword: #c678dd;
--py-name: #e5c07b;
--py-param: #abb2bf;
--py-type: #61afef;
--py-default: #98c379;
--py-punct: #636d83;
--py-decorator: #e06c75;
```

- [ ] **Step 2: Commit**

```bash
git add web/app/assets/css/main.css
git commit -m "feat(web): add Python syntax token CSS custom properties"
```

---

## Task 3: PySignature component

**Files:**
- Create: `web/app/components/docs/PySignature.vue`
- Create: `web/app/components/docs/__tests__/PySignature.test.ts`

- [ ] **Step 1: Write tests**

Create `web/app/components/docs/__tests__/PySignature.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import PySignature from "../PySignature.vue";
import type { DocSymbol } from "~/types/api";

function makeSymbol(overrides: Partial<DocSymbol> = {}): DocSymbol {
  return {
    name: "hello",
    kind: "function",
    signature: "def hello(name: str) -> str",
    docstring: "",
    parameters: [{ name: "name", type: "str", description: "", kind: "positional_or_keyword" }],
    returns: { type: "str", description: "" },
    ...overrides,
  };
}

describe("PySignature", () => {
  it("renders function keyword", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    const keywords = wrapper.findAll(".py-keyword");
    expect(keywords.length).toBeGreaterThan(0);
    expect(keywords[0].text()).toBe("def");
  });

  it("renders function name", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    expect(wrapper.find(".py-name").text()).toBe("hello");
  });

  it("renders parameter name", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    expect(wrapper.find(".py-param").text()).toBe("name");
  });

  it("renders type annotation", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    const types = wrapper.findAll(".py-type");
    expect(types.some((t) => t.text() === "str")).toBe(true);
  });

  it("renders return type", async () => {
    const wrapper = await mountSuspended(PySignature, { props: { symbol: makeSymbol() } });
    expect(wrapper.text()).toContain("->");
  });

  it("renders async function", async () => {
    const wrapper = await mountSuspended(PySignature, {
      props: {
        symbol: makeSymbol({
          name: "fetch",
          signature: "async def fetch(url: str) -> bytes",
        }),
      },
    });
    expect(wrapper.find(".py-keyword").text()).toBe("async");
  });

  it("renders class with bases", async () => {
    const wrapper = await mountSuspended(PySignature, {
      props: {
        symbol: {
          name: "MyError",
          kind: "class",
          signature: "class MyError(ValueError)",
          docstring: "",
          parameters: [],
          returns: undefined,
        },
      },
    });
    const keywords = wrapper.findAll(".py-keyword");
    expect(keywords[0].text()).toBe("class");
    expect(wrapper.find(".py-name").text()).toBe("MyError");
  });

  it("renders default value", async () => {
    const wrapper = await mountSuspended(PySignature, {
      props: {
        symbol: makeSymbol({
          parameters: [
            { name: "timeout", type: "float", description: "", kind: "keyword_only", default: "30.0" },
          ],
        }),
      },
    });
    expect(wrapper.find(".py-default").text()).toBe("30.0");
  });

  it("renders keyword-only separator", async () => {
    const wrapper = await mountSuspended(PySignature, {
      props: {
        symbol: makeSymbol({
          parameters: [
            { name: "x", type: "int", description: "", kind: "positional_or_keyword" },
            { name: "y", type: "int", description: "", kind: "keyword_only" },
          ],
        }),
      },
    });
    expect(wrapper.text()).toContain("*");
  });

  it("renders var keyword params", async () => {
    const wrapper = await mountSuspended(PySignature, {
      props: {
        symbol: makeSymbol({
          parameters: [{ name: "kwargs", type: undefined, description: "", kind: "var_keyword" }],
        }),
      },
    });
    expect(wrapper.text()).toContain("**kwargs");
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd web && npx vitest run app/components/docs/__tests__/PySignature.test.ts
```

Expected: FAIL (component doesn't exist)

- [ ] **Step 3: Implement PySignature.vue**

Create `web/app/components/docs/PySignature.vue`:

```vue
<script setup lang="ts">
import type { DocSymbol, DocParam } from "~/types/api";

const props = defineProps<{
  symbol: DocSymbol;
}>();

const isAsync = computed(() => props.symbol.signature.startsWith("async "));
const isClass = computed(() => props.symbol.kind === "class" || props.symbol.kind === "exception");
const keyword = computed(() => (isClass.value ? "class" : "def"));

// Extract base classes from the signature string for class symbols.
// Classes don't have parameters in the structured data, but the signature has bases.
const baseClasses = computed(() => {
  if (!isClass.value) return [];
  const match = props.symbol.signature.match(/\(([^)]+)\)/);
  if (!match) return [];
  return match[1].split(",").map((b) => b.trim());
});

// Group parameters by kind to insert separators.
interface ParamGroup {
  kind: "separator" | "param";
  separator?: string;
  param?: DocParam;
}

const paramGroups = computed<ParamGroup[]>(() => {
  const params = props.symbol.parameters ?? [];
  const groups: ParamGroup[] = [];
  let needsSlash = false;
  let needsStar = false;

  for (const p of params) {
    const kind = p.kind ?? "positional_or_keyword";

    // Insert / after positional-only params end.
    if (needsSlash && kind !== "positional_only") {
      groups.push({ kind: "separator", separator: "/" });
      needsSlash = false;
    }

    // Insert * before first keyword-only param (if no var_positional preceded it).
    if (kind === "keyword_only" && needsStar) {
      groups.push({ kind: "separator", separator: "*" });
      needsStar = false;
    }

    if (kind === "positional_only") {
      needsSlash = true;
      needsStar = true;
    } else if (kind === "positional_or_keyword") {
      needsStar = true;
    } else if (kind === "var_positional") {
      needsStar = false; // * already shown as *args
    }

    groups.push({ kind: "param", param: p });
  }

  // Trailing / if all params were positional-only.
  if (needsSlash) {
    groups.push({ kind: "separator", separator: "/" });
  }

  return groups;
});
</script>

<template>
  <div
    class="rounded-md border border-zinc-800 bg-zinc-900 px-4 py-2.5 font-mono text-[11px] leading-relaxed overflow-x-auto"
  >
    <!-- async keyword -->
    <span v-if="isAsync" class="py-keyword">async</span>
    <span v-if="isAsync" class="py-punct"> </span>

    <!-- def/class keyword -->
    <span class="py-keyword">{{ keyword }}</span>
    <span class="py-punct"> </span>

    <!-- function/class name -->
    <span class="py-name">{{ symbol.name }}</span>

    <!-- Parameters (functions) -->
    <template v-if="!isClass && paramGroups.length > 0">
      <span class="py-punct">(</span>
      <template v-for="(group, i) in paramGroups" :key="i">
        <span v-if="i > 0" class="py-punct">, </span>

        <!-- Separator: * or / -->
        <template v-if="group.kind === 'separator'">
          <span class="py-punct">{{ group.separator }}</span>
        </template>

        <!-- Parameter -->
        <template v-else-if="group.param">
          <span v-if="group.param.kind === 'var_positional'" class="py-punct">*</span>
          <span v-if="group.param.kind === 'var_keyword'" class="py-punct">**</span>
          <span class="py-param">{{ group.param.name }}</span>
          <template v-if="group.param.type">
            <span class="py-punct">: </span>
            <span class="py-type">{{ group.param.type }}</span>
          </template>
          <template v-if="group.param.default">
            <span class="py-punct"> = </span>
            <span class="py-default">{{ group.param.default }}</span>
          </template>
        </template>
      </template>
      <span class="py-punct">)</span>
    </template>

    <!-- Empty parens for no-param functions -->
    <template v-else-if="!isClass">
      <span class="py-punct">()</span>
    </template>

    <!-- Base classes (classes) -->
    <template v-if="isClass && baseClasses.length > 0">
      <span class="py-punct">(</span>
      <template v-for="(base, i) in baseClasses" :key="i">
        <span v-if="i > 0" class="py-punct">, </span>
        <span class="py-type">{{ base }}</span>
      </template>
      <span class="py-punct">)</span>
    </template>

    <!-- Return type -->
    <template v-if="!isClass && symbol.returns?.type">
      <span class="py-punct"> -> </span>
      <span class="py-type">{{ symbol.returns.type }}</span>
    </template>
  </div>
</template>

<style scoped>
.py-keyword { color: var(--py-keyword); }
.py-name { color: var(--py-name); }
.py-param { color: var(--py-param); }
.py-type { color: var(--py-type); }
.py-default { color: var(--py-default); }
.py-punct { color: var(--py-punct); }
.py-decorator { color: var(--py-decorator); }
</style>
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd web && npx vitest run app/components/docs/__tests__/PySignature.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/app/components/docs/PySignature.vue web/app/components/docs/__tests__/PySignature.test.ts
git commit -m "feat(web): add PySignature component with semantic highlighting"
```

---

## Task 4: useDocstringFormat composable

**Files:**
- Create: `web/app/composables/useDocstringFormat.ts`
- Create: `web/app/composables/__tests__/useDocstringFormat.test.ts`

- [ ] **Step 1: Write tests**

Create `web/app/composables/__tests__/useDocstringFormat.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { formatDocstring } from "../useDocstringFormat";

describe("formatDocstring", () => {
  it("returns empty string for empty input", () => {
    expect(formatDocstring("")).toBe("");
    expect(formatDocstring("   ")).toBe("");
  });

  it("wraps single line in paragraph", () => {
    const result = formatDocstring("Hello world.");
    expect(result).toBe("<p>Hello world.</p>");
  });

  it("splits on blank lines into paragraphs", () => {
    const result = formatDocstring("First paragraph.\n\nSecond paragraph.");
    expect(result).toContain("<p>First paragraph.</p>");
    expect(result).toContain("<p>Second paragraph.</p>");
  });

  it("converts double backticks to code spans", () => {
    const result = formatDocstring("Use ``foo`` to do things.");
    expect(result).toContain("<code>foo</code>");
  });

  it("converts single backticks to code spans", () => {
    const result = formatDocstring("Use `bar` for that.");
    expect(result).toContain("<code>bar</code>");
  });

  it("strips rst role prefixes", () => {
    const result = formatDocstring("See :func:`print` and :class:`str`.");
    expect(result).toContain("<code>print</code>");
    expect(result).toContain("<code>str</code>");
    expect(result).not.toContain(":func:");
  });

  it("dedents indented docstrings", () => {
    const result = formatDocstring("    Summary line.\n\n    Extended description.");
    expect(result).toContain("<p>Summary line.</p>");
    expect(result).toContain("<p>Extended description.</p>");
  });

  it("detects indented code blocks", () => {
    const input = "Example::\n\n    foo()\n    bar()";
    const result = formatDocstring(input);
    expect(result).toContain('<pre><code class="language-python">');
    expect(result).toContain("foo()");
    expect(result).toContain("bar()");
  });

  it("detects >>> REPL blocks", () => {
    const input = "Example:\n\n    >>> print('hello')\n    hello";
    const result = formatDocstring(input);
    expect(result).toContain('<pre><code class="language-python">');
  });

  it("strips rst directives as muted notes", () => {
    const result = formatDocstring(".. versionchanged:: 2.0\n   The default changed.");
    expect(result).toContain("versionchanged");
    // Should render as a note, not raw rst
    expect(result).not.toContain(".. version");
  });

  it("handles real click.echo docstring", () => {
    const input = `Print a message plus a newline to the given file or stdout. On
first call, if \`\`err\`\` is false, a check is done whether the
file object has been set to non-blocking mode.

Example::

    click.echo('Hello World!')
    click.echo(b'\\xe2\\x98\\x83', nl=False)`;
    const result = formatDocstring(input);
    expect(result).toContain("<p>");
    expect(result).toContain("<code>err</code>");
    expect(result).toContain('<pre><code class="language-python">');
    expect(result).toContain("click.echo");
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd web && npx vitest run app/composables/__tests__/useDocstringFormat.test.ts
```

Expected: FAIL (module doesn't exist)

- [ ] **Step 3: Implement useDocstringFormat.ts**

Create `web/app/composables/useDocstringFormat.ts`:

```ts
/**
 * Formats a raw Python docstring into HTML with paragraphs, code spans,
 * and code blocks. Code blocks are emitted as <pre><code class="language-python">
 * for Shiki to highlight during SSR.
 */
export function formatDocstring(raw: string): string {
  if (!raw || !raw.trim()) return "";

  let text = raw.replace(/\r\n/g, "\n").replace(/\r/g, "\n");
  text = dedent(text);

  const blocks = splitBlocks(text);
  return blocks.map((block) => renderBlock(block)).join("\n");
}

/** Remove common leading indentation from all lines. */
function dedent(text: string): string {
  const lines = text.split("\n");
  // Find minimum indentation of non-empty lines (skip first line which may have no indent).
  let minIndent = Infinity;
  for (let i = 1; i < lines.length; i++) {
    const line = lines[i];
    if (line.trim() === "") continue;
    const indent = line.match(/^(\s*)/)?.[1].length ?? 0;
    if (indent < minIndent) minIndent = indent;
  }
  if (minIndent === Infinity || minIndent === 0) {
    return text.trim();
  }
  return lines
    .map((line, i) => (i === 0 ? line.trimStart() : line.slice(minIndent)))
    .join("\n")
    .trim();
}

interface Block {
  type: "paragraph" | "code" | "directive";
  content: string;
  directiveType?: string;
}

/** Split text into blocks separated by blank lines. Classify each block. */
function splitBlocks(text: string): Block[] {
  const blocks: Block[] = [];
  const rawBlocks = text.split(/\n\n+/);
  let nextIsCode = false;

  for (const raw of rawBlocks) {
    const trimmed = raw.trim();
    if (!trimmed) continue;

    // Check for rst directive (.. directive:: args)
    const directiveMatch = trimmed.match(/^\.\.\s+(\w+)::\s*(.*)/);
    if (directiveMatch) {
      const rest = trimmed.slice(directiveMatch[0].length).trim();
      blocks.push({
        type: "directive",
        content: rest || directiveMatch[2] || "",
        directiveType: directiveMatch[1],
      });
      continue;
    }

    // Code block: preceded by :: or all lines indented or lines start with >>>
    if (nextIsCode || isCodeBlock(trimmed)) {
      blocks.push({ type: "code", content: trimmed.replace(/^\s{4}/gm, "") });
      nextIsCode = false;
      continue;
    }

    // Check if this paragraph ends with :: (next block is code)
    if (trimmed.endsWith("::")) {
      const label = trimmed.slice(0, -2).trim();
      if (label) {
        blocks.push({ type: "paragraph", content: label + ":" });
      }
      nextIsCode = true;
      continue;
    }

    blocks.push({ type: "paragraph", content: trimmed });
    nextIsCode = false;
  }

  return blocks;
}

/** Check if a block looks like code (all lines indented or REPL). */
function isCodeBlock(text: string): boolean {
  const lines = text.split("\n");
  const allIndented = lines.every((l) => l.startsWith("    ") || l.trim() === "");
  const hasRepl = lines.some((l) => l.trimStart().startsWith(">>>"));
  return allIndented || hasRepl;
}

/** Render a single block to HTML. */
function renderBlock(block: Block): string {
  switch (block.type) {
    case "code":
      return `<pre><code class="language-python">${escapeHtml(block.content)}</code></pre>`;
    case "directive":
      return `<p class="text-[11px] italic text-zinc-600">${escapeHtml(block.directiveType ?? "")}: ${formatInline(block.content)}</p>`;
    case "paragraph":
      return `<p>${formatInline(block.content)}</p>`;
  }
}

/** Format inline elements: backticks, rst roles. */
function formatInline(text: string): string {
  let result = escapeHtml(text);
  // rst roles: :func:`name` → <code>name</code>
  result = result.replace(/:\w+:`([^`]+)`/g, "<code>$1</code>");
  // Double backticks: ``code`` → <code>code</code>
  result = result.replace(/``([^`]+)``/g, "<code>$1</code>");
  // Single backticks: `code` → <code>code</code>
  result = result.replace(/`([^`]+)`/g, "<code>$1</code>");
  return result;
}

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd web && npx vitest run app/composables/__tests__/useDocstringFormat.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/app/composables/useDocstringFormat.ts web/app/composables/__tests__/useDocstringFormat.test.ts
git commit -m "feat(web): add useDocstringFormat composable for docstring parsing"
```

---

## Task 5: Shiki integration for code block highlighting

**Files:**
- Modify: `web/package.json`
- Create: `web/app/utils/shikiHighlight.ts`

- [ ] **Step 1: Install shiki**

```bash
cd web && npm install shiki
```

- [ ] **Step 2: Create SSR-only highlight utility**

Create `web/app/utils/shikiHighlight.ts`:

```ts
import { createHighlighter, type Highlighter } from "shiki";

let highlighterPromise: Promise<Highlighter> | null = null;

/** Get or create the singleton Shiki highlighter (SSR-only). */
function getHighlighter(): Promise<Highlighter> {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighter({
      themes: ["one-dark-pro"],
      langs: ["python"],
    });
  }
  return highlighterPromise;
}

/**
 * Highlight Python code using Shiki. Only runs during SSR.
 * Returns the input wrapped in <pre><code> on the client.
 */
export async function highlightPython(code: string): Promise<string> {
  if (!import.meta.server) {
    // Client-side: return plain code block (already highlighted from SSR).
    return `<pre><code class="language-python">${code}</code></pre>`;
  }

  const highlighter = await getHighlighter();
  return highlighter.codeToHtml(code, {
    lang: "python",
    theme: "one-dark-pro",
  });
}
```

- [ ] **Step 3: Commit**

```bash
git add web/app/utils/shikiHighlight.ts web/package.json web/package-lock.json
git commit -m "feat(web): add Shiki SSR utility for Python code highlighting"
```

---

## Task 6: PyDocstring component

**Files:**
- Create: `web/app/components/docs/PyDocstring.vue`
- Create: `web/app/components/docs/__tests__/PyDocstring.test.ts`

- [ ] **Step 1: Write test**

Create `web/app/components/docs/__tests__/PyDocstring.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { mountSuspended } from "@nuxt/test-utils/runtime";
import PyDocstring from "../PyDocstring.vue";

describe("PyDocstring", () => {
  it("renders nothing for empty text", async () => {
    const wrapper = await mountSuspended(PyDocstring, { props: { text: "" } });
    expect(wrapper.html()).toBe("<!--v-if-->");
  });

  it("renders paragraphs", async () => {
    const wrapper = await mountSuspended(PyDocstring, {
      props: { text: "Hello world.\n\nSecond paragraph." },
    });
    expect(wrapper.findAll("p").length).toBeGreaterThanOrEqual(2);
  });

  it("renders code spans", async () => {
    const wrapper = await mountSuspended(PyDocstring, {
      props: { text: "Use ``foo`` for this." },
    });
    expect(wrapper.find("code").text()).toBe("foo");
  });

  it("renders code blocks", async () => {
    const wrapper = await mountSuspended(PyDocstring, {
      props: { text: "Example::\n\n    foo()\n    bar()" },
    });
    expect(wrapper.find("pre").exists()).toBe(true);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd web && npx vitest run app/components/docs/__tests__/PyDocstring.test.ts
```

Expected: FAIL

- [ ] **Step 3: Implement PyDocstring.vue**

Create `web/app/components/docs/PyDocstring.vue`:

```vue
<script setup lang="ts">
import { formatDocstring } from "~/composables/useDocstringFormat";
import { highlightPython } from "~/utils/shikiHighlight";

const props = defineProps<{
  text: string;
}>();

const formattedHtml = computedAsync(async () => {
  if (!props.text?.trim()) return "";

  let html = formatDocstring(props.text);

  // During SSR, replace code blocks with Shiki-highlighted versions.
  if (import.meta.server) {
    const codeBlockRegex = /<pre><code class="language-python">([\s\S]*?)<\/code><\/pre>/g;
    const matches = [...html.matchAll(codeBlockRegex)];
    for (const match of matches) {
      const code = match[1]
        .replace(/&amp;/g, "&")
        .replace(/&lt;/g, "<")
        .replace(/&gt;/g, ">")
        .replace(/&quot;/g, '"');
      const highlighted = await highlightPython(code);
      html = html.replace(match[0], highlighted);
    }
  }

  return html;
}, "");
</script>

<template>
  <div
    v-if="formattedHtml"
    class="docstring-content text-sm leading-relaxed text-zinc-400"
    v-html="formattedHtml"
  />
</template>

<style scoped>
.docstring-content :deep(p) {
  margin-bottom: 0.75rem;
}
.docstring-content :deep(code) {
  background: #27272a;
  padding: 1px 4px;
  border-radius: 3px;
  color: var(--py-name);
  font-size: 0.85em;
  font-family: var(--font-mono);
}
.docstring-content :deep(pre) {
  background: #0f0f10;
  border: 1px solid #27272a;
  border-radius: 6px;
  padding: 12px 16px;
  margin-bottom: 0.75rem;
  overflow-x: auto;
}
.docstring-content :deep(pre code) {
  background: transparent;
  padding: 0;
  border-radius: 0;
  color: inherit;
  font-size: 11px;
}
</style>
```

Note: `computedAsync` is from VueUse which is already installed via `@vueuse/nuxt`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd web && npx vitest run app/components/docs/__tests__/PyDocstring.test.ts
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/app/components/docs/PyDocstring.vue web/app/components/docs/__tests__/PyDocstring.test.ts
git commit -m "feat(web): add PyDocstring component with Shiki code highlighting"
```

---

## Task 7: Wire components into docs page

**Files:**
- Modify: `web/app/pages/packages/[name]/docs.vue`

- [ ] **Step 1: Replace inline signature rendering**

In `web/app/pages/packages/[name]/docs.vue`, replace the signature div:

```html
<!-- OLD -->
<div class="mb-3 rounded-md border border-zinc-800 bg-zinc-900 px-4 py-2.5 font-mono text-[11px] leading-relaxed text-violet-300">
  {{ sym.signature }}
</div>

<!-- NEW -->
<PySignature :symbol="sym" class="mb-3" />
```

- [ ] **Step 2: Replace inline docstring rendering**

Replace the docstring paragraph:

```html
<!-- OLD -->
<p v-if="sym.docstring" class="mb-3 text-sm leading-relaxed text-zinc-400">
  {{ sym.docstring }}
</p>

<!-- NEW -->
<PyDocstring v-if="sym.docstring" :text="sym.docstring" class="mb-3" />
```

- [ ] **Step 3: Run the dev server and verify visually**

```bash
cd web && npm run dev
```

Open http://localhost:3000/packages/click/docs and verify:
- Signatures show colored tokens (purple keyword, yellow name, blue types, green defaults)
- Docstrings show formatted paragraphs with code spans and highlighted code blocks
- No visual regressions in layout, sidebar, or parameter sections

- [ ] **Step 4: Run all frontend tests**

```bash
cd web && npx vitest run
```

Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/app/pages/packages/[name]/docs.vue
git commit -m "feat(web): wire PySignature and PyDocstring into docs page"
```

---

## Summary

| Task | What it produces |
|------|-----------------|
| 1 | API returns `kind` and `default` on DocParam |
| 2 | CSS custom properties for Python token colors |
| 3 | PySignature.vue — semantic signature highlighting |
| 4 | useDocstringFormat.ts — docstring text → HTML |
| 5 | Shiki SSR utility for Python code blocks |
| 6 | PyDocstring.vue — formatted docstrings with highlighted code |
| 7 | Wire components into the docs page |
