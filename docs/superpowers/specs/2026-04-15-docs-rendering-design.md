# Docs Rendering Design Spec

## Goal

Improve the API documentation rendering on pypx from plain monospace text to semantically highlighted signatures and formatted docstrings with syntax-highlighted code blocks.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Signature rendering | Custom semantic highlighting from structured data | goopy returns parameters with name/type/default — color by role, not regex |
| Docstring rendering | Light formatting (custom parser) | Handles 90% of real docstrings without full rst complexity |
| Code block highlighting | Shiki (SSR-only, Python grammar) | Pre-rendered at build time, zero client JS, VS Code quality |
| Where rendering lives | Frontend (Vue components + composable) | Presentation concern, no API changes, no cache invalidation |
| Shiki loading | SSR/build-time only | Docs page uses useAsyncData — HTML arrives pre-colored, no FOUC |

## Architecture

### Data Flow

```
API Response (unchanged)
  │
  ├─ DocSymbol.parameters[] ──→ PySignature.vue ──→ colored <span> elements
  │  DocSymbol.returns                               (pure template, no deps)
  │  DocSymbol.name, kind
  │
  └─ DocSymbol.docstring ──→ PyDocstring.vue ──→ formatted HTML
                               │                    (paragraphs, code spans,
                               │                     highlighted code blocks)
                               │
                               ├─ useDocstringFormat.ts (text → HTML)
                               └─ Shiki SSR plugin (code blocks → highlighted HTML)
```

### No API Changes

The handler response shape stays exactly as-is. All rendering logic is frontend-only. This means:
- No cache invalidation needed
- No backend deployment required
- Easy to iterate on styling

## Components

### PySignature.vue

**Location:** `web/app/components/docs/PySignature.vue`

**Props:**
```ts
interface Props {
  symbol: DocSymbol
}
```

**Rendering rules:**

The component builds the signature from structured data, wrapping each token in a `<span>` with a CSS class:

| Token | CSS class | Color |
|-------|-----------|-------|
| `def`, `class`, `async` | `py-keyword` | `#c678dd` (purple) |
| Function/class name | `py-name` | `#e5c07b` (yellow) |
| Parameter names | `py-param` | `#abb2bf` (light gray) |
| Type annotations | `py-type` | `#61afef` (blue) |
| Default values | `py-default` | `#98c379` (green) |
| Punctuation `( ) , : -> = * **` | `py-punct` | `#636d83` (dim gray) |
| Decorators (if shown) | `py-decorator` | `#e06c75` (red) |

**Signature assembly logic:**

For functions:
```
[async ][def ]{name}({param}: {type}[ = {default}], ...) [-> {returns}]
```

For classes:
```
[class ]{name}[({base}, ...)]
```

Each segment is a `<span>` with the appropriate class. The component reads `symbol.parameters` to get structured name/type/default data rather than parsing the `symbol.signature` string.

**Parameter kind handling:**
- `*` separator before keyword-only params
- `**` prefix for var-keyword params
- `*` prefix for var-positional params
- `/` separator after positional-only params

These are derived from the `kind` field on each parameter in the API response (which goopy populates from `ParamKind`).

**Note:** The current API `DocParam` type does not include `kind` or `default`. The `DocSymbol.signature` string has this info baked in, but the structured parameters don't. Two options:
1. Enrich the API `DocParam` type with `kind` and `default` fields (requires API handler change)
2. Parse the signature string as a fallback for kind/default info

Recommendation: Option 1 — add `kind` and `default` to `DocParam` in the handler's conversion. This is a backward-compatible addition (new optional JSON fields).

### PyDocstring.vue

**Location:** `web/app/components/docs/PyDocstring.vue`

**Props:**
```ts
interface Props {
  text: string  // raw docstring text
}
```

**Rendering:** Calls `useDocstringFormat(text)` to get formatted HTML, renders with `v-html`.

### useDocstringFormat.ts

**Location:** `web/app/composables/useDocstringFormat.ts`

**Input:** Raw docstring string from the API.

**Output:** HTML string with paragraphs, code spans, and code blocks.

**Parsing rules (in order):**

1. **Trim** leading/trailing whitespace and normalize line endings.

2. **Dedent** — find the minimum indentation of non-empty lines (after the first line) and strip that many spaces from all lines. Python docstrings are typically indented to match the function body.

3. **Split into blocks** by blank lines (two consecutive newlines). Each block becomes either a paragraph or a code block.

4. **Code block detection** — a block is a code block if:
   - It follows a line ending with `::` (rst code block indicator)
   - All lines are indented by 4+ spaces relative to surrounding text
   - Lines start with `>>>` (Python REPL examples)

5. **Inline formatting** within paragraphs:
   - `` ``text`` `` (double backtick) → `<code>text</code>`
   - `` `text` `` (single backtick) → `<code>text</code>`
   - `:func:\`name\`` and similar rst roles → `<code>name</code>` (strip the role prefix)
   - `.. versionchanged::` and similar rst directives → render as a muted note or strip entirely

6. **Code blocks** → During SSR (`import.meta.server`), pass the code to Shiki for highlighting and emit the pre-colored HTML. During client hydration, skip Shiki entirely — the HTML is already rendered. Emit as `<pre><code class="language-python">...</code></pre>` either way.

7. **Paragraphs** → `<p>` tags with appropriate spacing.

**Edge cases:**
- Empty docstring → render nothing
- Single-line docstring → single `<p>`, no code block detection
- Docstring with only code → still wrap in code block
- Mixed indentation (tabs/spaces) → treat tab as 4 spaces

### Shiki Integration

**Approach:** Use the `shiki` package directly with a Nuxt server-side composable, not `@nuxt/content` (which is overkill — we don't need MDC or content management).

**Setup:**
- Add `shiki` to `package.json` dependencies
- Create a server utility or composable that initializes Shiki once with the Python grammar and a single dark theme
- The `useDocstringFormat` composable calls this during SSR to highlight code blocks
- On the client (hydration), code blocks are already HTML — no Shiki needed

**Theme:** Use a built-in Shiki theme that matches pypx's dark zinc palette. Good candidates:
- `one-dark-pro` — matches the color scheme we chose for signatures
- `github-dark` — clean, widely recognized
- `vitesse-dark` — minimal, modern

Recommendation: `one-dark-pro` since the signature colors were designed to match it.

**Grammar:** Load only `python` — no other languages needed. This keeps the WASM bundle small.

## CSS Custom Properties

Define a token color palette using CSS custom properties for theming:

```css
:root {
  --py-keyword: #c678dd;
  --py-name: #e5c07b;
  --py-param: #abb2bf;
  --py-type: #61afef;
  --py-default: #98c379;
  --py-punct: #636d83;
  --py-decorator: #e06c75;
}
```

Components use these via `color: var(--py-keyword)` etc. A future light theme would override these properties.

## API Handler Change

Add `kind` and `default` fields to `DocParam` in the API response:

```go
type DocParam struct {
    Name        string `json:"name"`
    Type        string `json:"type,omitempty"`
    Description string `json:"description"`
    Kind        string `json:"kind,omitempty"`    // positional_only, positional_or_keyword, var_positional, keyword_only, var_keyword
    Default     string `json:"default,omitempty"` // string representation of default value
}
```

Update `convertFunction` in `api/internal/handler/docs.go` to populate these from `model.Parameter.Kind` and `model.Parameter.Default`.

Update the TypeScript `DocParam` interface in `web/app/types/api.ts` to match.

## Files

### New
- `web/app/components/docs/PySignature.vue`
- `web/app/components/docs/PyDocstring.vue`
- `web/app/composables/useDocstringFormat.ts`

### Modified
- `web/app/pages/packages/[name]/docs.vue` — replace inline rendering with new components
- `web/app/types/api.ts` — add `kind` and `default` to `DocParam`
- `web/nuxt.config.ts` — Shiki setup if needed
- `web/package.json` — add `shiki` dependency
- `web/app/assets/css/main.css` — add `--py-*` custom properties
- `api/internal/handler/docs.go` — add `Kind` and `Default` to `DocParam`, populate in conversion

## Testing

- **PySignature.vue** — Vitest component tests: render a DocSymbol, verify correct spans and classes for each token type. Test edge cases: no params, keyword-only separator, `**kwargs`, async functions, classes with bases.

- **useDocstringFormat.ts** — Vitest unit tests: verify paragraph splitting, code block detection, inline code formatting, rst directive stripping, dedent logic. Use real docstrings from click/requests as fixtures.

- **PyDocstring.vue** — Vitest component test: verify v-html rendering, code blocks have `language-python` class.

- **Visual regression** — Manual check on pypx.app for 5 packages: click, requests, flask, rich, marshmallow.

## Out of Scope

- Full rst rendering (admonitions, tables, cross-references beyond `:func:`)
- Markdown rendering (most Python docstrings are rst or plain text)
- Light theme (future — CSS custom properties make this easy to add later)
- Class method rendering (DocSymbol doesn't include methods yet)
- Docstring parameter section rendering (already handled by the existing Parameters UI)
