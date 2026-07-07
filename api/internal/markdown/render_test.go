package markdown

import (
	"strings"
	"testing"
)

// run is a helper: call RenderSafe, assert no error, check wantContain is present.
func run(t *testing.T, input, wantContain string) {
	t.Helper()
	got, err := RenderSafe(input)
	if err != nil {
		t.Fatalf("RenderSafe() error: %v", err)
	}
	if !strings.Contains(got, wantContain) {
		t.Errorf("output does not contain %q\nfull output: %q", wantContain, got)
	}
}

func runEmpty(t *testing.T, input string) {
	t.Helper()
	got, err := RenderSafe(input)
	if err != nil {
		t.Fatalf("RenderSafe() error: %v", err)
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

// ── Basic cases ───────────────────────────────────────────────────────────────

func TestRender_Empty(t *testing.T) {
	runEmpty(t, "")
}

func TestRender_Paragraph(t *testing.T) {
	run(t, "Hello, world!", "<p>Hello, world!</p>")
}

func TestRender_MultipleParagraphs(t *testing.T) {
	got, _ := RenderSafe("First.\n\nSecond.\n")
	if !strings.Contains(got, "<p>First.</p>") {
		t.Errorf("missing first paragraph in %q", got)
	}
	if !strings.Contains(got, "<p>Second.</p>") {
		t.Errorf("missing second paragraph in %q", got)
	}
}

// ── Headings ──────────────────────────────────────────────────────────────────

func TestRender_H1(t *testing.T) {
	run(t, "# My Package", "<h1>My Package</h1>")
}

func TestRender_H2(t *testing.T) {
	run(t, "## Installation", "<h2>Installation</h2>")
}

func TestRender_H3(t *testing.T) {
	run(t, "### Usage", "<h3>Usage</h3>")
}

func TestRender_H4(t *testing.T) {
	run(t, "#### Details", "<h4>Details</h4>")
}

func TestRender_ATXHeadingAlt(t *testing.T) {
	// Setext-style headings (underline syntax).
	run(t, "My Title\n========\n", "<h1>My Title</h1>")
}

// ── Inline markup ─────────────────────────────────────────────────────────────

func TestRender_Bold(t *testing.T) {
	run(t, "This is **bold**.", "<strong>bold</strong>")
}

func TestRender_Italic(t *testing.T) {
	run(t, "This is *italic*.", "<em>italic</em>")
}

func TestRender_InlineCode(t *testing.T) {
	run(t, "Use `requests.get()` here.", "<code>requests.get()</code>")
}

func TestRender_Link(t *testing.T) {
	run(t, "[Click here](https://example.com)", `href="https://example.com"`)
}

func TestRender_LinkText(t *testing.T) {
	got, _ := RenderSafe("[Click here](https://example.com)")
	if !strings.Contains(got, ">Click here</a>") {
		t.Errorf("expected link text, got %q", got)
	}
}

func TestRender_Image(t *testing.T) {
	run(t, "![alt text](https://example.com/img.png)", "<img")
}

func TestRender_ImageAlt(t *testing.T) {
	run(t, "![my alt](https://example.com/img.png)", `alt="my alt"`)
}

// ── GFM extensions ────────────────────────────────────────────────────────────

func TestRender_GFM_Strikethrough(t *testing.T) {
	run(t, "~~deleted~~", "<del>deleted</del>")
}

func TestRender_GFM_Autolink(t *testing.T) {
	run(t, "Visit https://example.com for info.", `<a href="https://example.com"`)
}

func TestRender_GFM_Table(t *testing.T) {
	input := "| Header | Value |\n|--------|-------|\n| Foo    | Bar   |\n"
	run(t, input, "<table>")
}

func TestRender_GFM_TableContent(t *testing.T) {
	input := "| Name | Version |\n|------|----------|\n| pkg  | 1.2.3    |\n"
	got, _ := RenderSafe(input)
	if !strings.Contains(got, "<td>") {
		t.Errorf("expected table cells in %q", got)
	}
}

func TestRender_GFM_TaskList(t *testing.T) {
	run(t, "- [x] done\n- [ ] todo\n", `type="checkbox"`)
}

// ── Code blocks ───────────────────────────────────────────────────────────────

func TestRender_FencedCodeBlock(t *testing.T) {
	run(t, "```python\nprint('hello')\n```", "<pre")
}

func TestRender_FencedCodeBlockContent(t *testing.T) {
	run(t, "```python\nprint('hello')\n```", "print")
}

func TestRender_FencedCodeBlockSyntaxHighlight(t *testing.T) {
	// Goldmark-highlighting wraps tokens in spans.
	run(t, "```python\nprint('hello')\n```", "<span")
}

func TestRender_IndentedCodeBlock(t *testing.T) {
	run(t, "    pip install pkg\n", "<code>")
}

// ── Raw HTML stripped (RenderSafe is the only renderer) ───────────────────────

func TestRenderSafe_RawHTMLStripped(t *testing.T) {
	// goldmark's safe mode replaces raw HTML with "<!-- raw HTML omitted -->".
	got, _ := RenderSafe(`<div class="banner">Hello</div>`)
	if strings.Contains(got, "<div") {
		t.Errorf("raw HTML must not pass through RenderSafe, got %q", got)
	}
}

func TestRenderSafe_RawHTMLBadgeStripped(t *testing.T) {
	// Raw <img> badge tags must not survive either.
	input := `<img src="https://img.shields.io/pypi/v/pkg.svg" alt="PyPI version">`
	got, _ := RenderSafe(input)
	if strings.Contains(got, "<img") {
		t.Errorf("raw HTML must not pass through RenderSafe, got %q", got)
	}
}

// ── Lists ──────────────────────────────────────────────────────────────────────

func TestRender_UnorderedList(t *testing.T) {
	run(t, "- item one\n- item two\n", "<ul>")
}

func TestRender_UnorderedListItems(t *testing.T) {
	got, _ := RenderSafe("- first\n- second\n")
	if !strings.Contains(got, "<li>first</li>") {
		t.Errorf("expected list items in %q", got)
	}
}

func TestRender_OrderedList(t *testing.T) {
	run(t, "1. first\n2. second\n", "<ol>")
}

func TestRender_NestedList(t *testing.T) {
	input := "- outer\n  - inner\n"
	run(t, input, "<ul>")
}

// ── Blockquote ────────────────────────────────────────────────────────────────

func TestRender_Blockquote(t *testing.T) {
	run(t, "> This is a quote.\n", "<blockquote>")
}

// ── Horizontal rule ───────────────────────────────────────────────────────────

func TestRender_HorizontalRule(t *testing.T) {
	run(t, "---\n", "<hr")
}

// ── Real-world package README patterns ───────────────────────────────────────

// TestRender_RequestsStyle tests a README similar to the requests library.
func TestRender_RequestsStyle(t *testing.T) {
	input := "# Requests\n\n" +
		"[![PyPI](https://img.shields.io/pypi/v/requests.svg)](https://pypi.org/project/requests/)\n\n" +
		"**Requests** is an elegant HTTP library for Python.\n\n" +
		"## Installation\n\n" +
		"```bash\npip install requests\n```\n\n" +
		"## Quickstart\n\n" +
		"```python\nimport requests\nr = requests.get('https://httpbin.org/get')\n```\n"
	got, _ := RenderSafe(input)
	if !strings.Contains(got, "<h1>Requests</h1>") {
		t.Errorf("missing h1 in %q", got)
	}
	if !strings.Contains(got, "<strong>Requests</strong>") {
		t.Errorf("missing bold in %q", got)
	}
	if !strings.Contains(got, "<h2>Installation</h2>") {
		t.Errorf("missing Installation heading in %q", got)
	}
	if !strings.Contains(got, "pip install requests") {
		t.Errorf("missing code block content in %q", got)
	}
}

// TestRender_TypicalBadgeREADME tests a README with shield.io badge images
// embedded as markdown image links — very common in Python packages.
func TestRender_TypicalBadgeREADME(t *testing.T) {
	input := "# mypackage\n\n" +
		"[![Tests](https://github.com/org/pkg/actions/badge.svg)](https://github.com/org/pkg/actions)\n" +
		"[![Coverage](https://codecov.io/badge.svg)](https://codecov.io/gh/org/pkg)\n\n" +
		"A great Python package.\n"
	got, _ := RenderSafe(input)
	if !strings.Contains(got, `href="https://github.com/org/pkg/actions"`) {
		t.Errorf("missing github link in %q", got)
	}
	if !strings.Contains(got, "<p>A great Python package.</p>") {
		t.Errorf("missing description paragraph in %q", got)
	}
}

// TestRender_ChangelogStyle tests a changelog-style README common in packages
// that put their changelog in the README.
func TestRender_ChangelogStyle(t *testing.T) {
	input := "## Changelog\n\n" +
		"### 2.0.0 (2024-01-01)\n\n" +
		"- Breaking: removed `old_api()`\n" +
		"- Added `new_api()`\n\n" +
		"### 1.9.0 (2023-06-01)\n\n" +
		"- Fixed bug in parser\n"
	got, _ := RenderSafe(input)
	if !strings.Contains(got, "<h2>Changelog</h2>") {
		t.Errorf("missing Changelog heading in %q", got)
	}
	if !strings.Contains(got, "<h3>") {
		t.Errorf("missing version subheadings in %q", got)
	}
	if !strings.Contains(got, "<code>old_api()</code>") {
		t.Errorf("missing inline code in %q", got)
	}
}

// TestRender_HTMLEscaping ensures that bare HTML special chars in Markdown text
// are properly escaped.
func TestRender_HTMLEscaping(t *testing.T) {
	// Angle brackets that are NOT raw HTML should be escaped.
	got, _ := RenderSafe("Type `Dict[str, int]` for a typed dict.")
	// The <, > inside backticks should not produce broken HTML.
	if !strings.Contains(got, "<code>") {
		t.Errorf("expected code tag in %q", got)
	}
}
