package rst_test

import (
	"strings"
	"testing"

	"github.com/pypx/api/internal/rst"
)

// run is a helper: call Render and assert wantContain is present (or output is empty).
func run(t *testing.T, input, wantContain string, wantEmpty bool) {
	t.Helper()
	got, err := rst.Render(input)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if wantEmpty {
		if strings.TrimSpace(got) != "" {
			t.Errorf("want empty output, got %q", got)
		}
		return
	}
	if !strings.Contains(got, wantContain) {
		t.Errorf("output does not contain %q\nfull output: %q", wantContain, got)
	}
}

// ── Basic cases ──────────────────────────────────────────────────────────────

func TestRender_Empty(t *testing.T) {
	run(t, "", "", true)
}

func TestRender_Paragraph(t *testing.T) {
	run(t, "Hello world.\n", "<p>Hello world.</p>", false)
}

func TestRender_MultipleParagraphs(t *testing.T) {
	input := "First paragraph.\n\nSecond paragraph.\n"
	got, _ := rst.Render(input)
	if !strings.Contains(got, "<p>First paragraph.</p>") {
		t.Errorf("missing first paragraph in %q", got)
	}
	if !strings.Contains(got, "<p>Second paragraph.</p>") {
		t.Errorf("missing second paragraph in %q", got)
	}
}

func TestRender_HTMLEscaping(t *testing.T) {
	got, _ := rst.Render(`Use <tag> & "quotes".`)
	if !strings.Contains(got, "&lt;tag&gt;") {
		t.Errorf("< > not escaped in %q", got)
	}
	if !strings.Contains(got, "&amp;") {
		t.Errorf("& not escaped in %q", got)
	}
}

// ── Inline markup ─────────────────────────────────────────────────────────────

func TestRender_Bold(t *testing.T) {
	run(t, "This is **bold** text.", "<strong>bold</strong>", false)
}

func TestRender_Italic(t *testing.T) {
	run(t, "This is *italic* text.", "<em>italic</em>", false)
}

func TestRender_InlineCode(t *testing.T) {
	run(t, "Use `my_func()` here.", "<code>my_func()</code>", false)
}

func TestRender_Role(t *testing.T) {
	run(t, "See :func:`requests.get` for details.", "<code>requests.get</code>", false)
}

func TestRender_RoleMod(t *testing.T) {
	run(t, "Use :mod:`os.path` to join paths.", "<code>os.path</code>", false)
}

func TestRender_RoleClass(t *testing.T) {
	run(t, "Subclass :class:`Exception`.", "<code>Exception</code>", false)
}

// ── Headings ──────────────────────────────────────────────────────────────────

func TestRender_H1(t *testing.T) {
	run(t, "My Title\n========\n", "<h1>My Title</h1>", false)
}

func TestRender_H2(t *testing.T) {
	run(t, "Section\n-------\n", "<h2>Section</h2>", false)
}

func TestRender_H3(t *testing.T) {
	run(t, "Subsection\n~~~~~~~~~~\n", "<h3>Subsection</h3>", false)
}

func TestRender_H4(t *testing.T) {
	run(t, "Sub-subsection\n^^^^^^^^^^^^^^\n", "<h4>Sub-subsection</h4>", false)
}

func TestRender_H5Default(t *testing.T) {
	// Any other character maps to h5.
	run(t, "Deep\n''''\n", "<h5>Deep</h5>", false)
}

func TestRender_HeadingWithInline(t *testing.T) {
	// Inline markup inside a heading should be processed.
	run(t, "Title with **bold**\n===================\n", "<strong>bold</strong>", false)
}

// ── Directives ────────────────────────────────────────────────────────────────

func TestRender_CodeBlock(t *testing.T) {
	run(t, ".. code-block:: python\n\n   x = 1\n   print(x)\n", "<pre><code", false)
}

func TestRender_CodeBlockContent(t *testing.T) {
	run(t, ".. code-block:: python\n\n   x = 1\n", "x = 1", false)
}

func TestRender_CodeBlockLanguageClass(t *testing.T) {
	run(t, ".. code-block:: python\n\n   pass\n", `class="language-python"`, false)
}

func TestRender_CodeDirectiveAlias(t *testing.T) {
	run(t, ".. code:: bash\n\n   echo hi\n", "<pre><code", false)
}

func TestRender_SourcecodeDirectiveAlias(t *testing.T) {
	run(t, ".. sourcecode:: go\n\n   fmt.Println()\n", "<pre><code", false)
}

func TestRender_Note(t *testing.T) {
	run(t, ".. note::\n\n   This is a note.\n", "rst-note", false)
}

func TestRender_Warning(t *testing.T) {
	run(t, ".. warning::\n\n   Be careful.\n", "rst-warning", false)
}

func TestRender_Caution(t *testing.T) {
	run(t, ".. caution::\n\n   Proceed carefully.\n", "rst-warning", false)
}

func TestRender_Danger(t *testing.T) {
	run(t, ".. danger::\n\n   Do not do this.\n", "rst-warning", false)
}

func TestRender_Attention(t *testing.T) {
	run(t, ".. attention::\n\n   Pay attention.\n", "rst-warning", false)
}

func TestRender_Image(t *testing.T) {
	run(t, ".. image:: /path/to/img.png\n", "<img", false)
}

func TestRender_ImageAlt(t *testing.T) {
	// image directive with :alt: body option.
	input := ".. image:: /path/to/img.png\n   :alt: my image\n"
	run(t, input, `alt="my image"`, false)
}

func TestRender_ImageJavascriptBlocked(t *testing.T) {
	// javascript: URLs must be filtered.
	got, _ := rst.Render(".. image:: javascript:alert(1)\n")
	if strings.Contains(got, "javascript:") {
		t.Errorf("javascript: URL should be blocked, got %q", got)
	}
}

func TestRender_ToctreeOmitted(t *testing.T) {
	run(t, ".. toctree::\n\n   page1\n   page2\n", "", true)
}

func TestRender_ContentsOmitted(t *testing.T) {
	run(t, ".. contents::\n\n   Overview\n", "", true)
}

func TestRender_UnknownDirectiveRendersBody(t *testing.T) {
	// Unknown directives fall through to render their body as a paragraph.
	run(t, ".. versionadded:: 1.2\n\n   Added in version 1.2.\n", "Added in version 1.2.", false)
}

// ── Hyperlink targets ─────────────────────────────────────────────────────────

func TestRender_HyperlinkTargetNoLoop(t *testing.T) {
	// .. _label: url has no :: — must not infinite-loop and must pass through surrounding text.
	run(t, ".. _colorama: https://example.com\n\nSome text.\n", "<p>Some text.</p>", false)
}

func TestRender_MultipleHyperlinkTargets(t *testing.T) {
	run(t, ".. _foo:\n.. _bar:\n\nParagraph.\n", "<p>Paragraph.</p>", false)
}

// ── Embedded hyperlinks ───────────────────────────────────────────────────────

func TestRender_EmbeddedHyperlink(t *testing.T) {
	run(t,
		"See the `Quickstart <https://example.com/quickstart>`_ for details.\n",
		`<a href="https://example.com/quickstart">Quickstart</a>`,
		false,
	)
}

func TestRender_EmbeddedHyperlinkText(t *testing.T) {
	// Link text should be the display text, not the URL.
	got, _ := rst.Render("Read the `User Guide <https://docs.example.com>`_ now.\n")
	if !strings.Contains(got, ">User Guide</a>") {
		t.Errorf("expected 'User Guide' as link text, got %q", got)
	}
}

func TestRender_EmbeddedHyperlinkJavascriptBlocked(t *testing.T) {
	got, _ := rst.Render("Click `here <javascript:alert(1)>`_.\n")
	if strings.Contains(got, "javascript:") {
		t.Errorf("javascript: URL should be blocked, got %q", got)
	}
}

func TestRender_EmbeddedHyperlinkMultipleInParagraph(t *testing.T) {
	// Multiple hyperlinks in one paragraph.
	input := "See `foo <https://foo.com>`_ and `bar <https://bar.com>`_.\n"
	got, _ := rst.Render(input)
	if !strings.Contains(got, `href="https://foo.com"`) {
		t.Errorf("missing foo link in %q", got)
	}
	if !strings.Contains(got, `href="https://bar.com"`) {
		t.Errorf("missing bar link in %q", got)
	}
}

// ── Substitution definitions & references ─────────────────────────────────────

func TestRender_SubstitutionImage(t *testing.T) {
	input := ".. |badge| image:: https://img.shields.io/badge/foo.svg\n\n|badge|\n"
	run(t, input, `<img src="https://img.shields.io/badge/foo.svg"`, false)
}

func TestRender_SubstitutionWithTarget(t *testing.T) {
	input := ".. |badge| image:: https://img.shields.io/badge/foo.svg\n" +
		"   :target: https://example.com\n\n|badge|\n"
	run(t, input, `<a href="https://example.com">`, false)
}

func TestRender_SubstitutionWithAlt(t *testing.T) {
	input := ".. |badge| image:: https://img.shields.io/badge/foo.svg\n" +
		"   :alt: my alt text\n\n|badge|\n"
	run(t, input, `alt="my alt text"`, false)
}

func TestRender_SubstitutionDefsNotRenderedDirectly(t *testing.T) {
	// The definition lines themselves should not produce output paragraphs.
	input := ".. |foo| image:: https://example.com/foo.svg\n" +
		".. |bar| image:: https://example.com/bar.svg\n\n" +
		"Some text.\n"
	got, _ := rst.Render(input)
	if strings.Contains(got, ":target:") || strings.Contains(got, ".. |") {
		t.Errorf("substitution definition leaked into output: %q", got)
	}
	if !strings.Contains(got, "<p>Some text.</p>") {
		t.Errorf("missing surrounding paragraph in %q", got)
	}
}

func TestRender_SubstitutionUnknownPassthrough(t *testing.T) {
	// A |ref| with no matching definition should pass through as literal text (not crash).
	got, err := rst.Render("Some |unknown| reference.\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not panic and should still render surrounding text.
	if !strings.Contains(got, "reference") {
		t.Errorf("expected surrounding text, got %q", got)
	}
}

func TestRender_SubstitutionJavascriptBlocked(t *testing.T) {
	input := ".. |evil| image:: javascript:alert(1)\n\n|evil|\n"
	got, _ := rst.Render(input)
	if strings.Contains(got, "javascript:") {
		t.Errorf("javascript: URL should be blocked, got %q", got)
	}
}

// ── URL scheme allowlist (data:/file:/etc. rejected) ──────────────────────────

func TestRender_EmbeddedHyperlinkDataBlocked(t *testing.T) {
	got, _ := rst.Render("Click `x <data:text/html,<script>alert(1)</script>>`_.\n")
	if strings.Contains(got, "data:") {
		t.Errorf("data: URL should be blocked, got %q", got)
	}
}

func TestRender_ImageDataBlocked(t *testing.T) {
	got, _ := rst.Render(".. image:: data:image/svg+xml;base64,AAAA\n")
	if strings.Contains(got, "data:") {
		t.Errorf("data: image URL should be blocked, got %q", got)
	}
}

func TestRender_EmbeddedHyperlinkHTTPSAllowed(t *testing.T) {
	got, _ := rst.Render("See `Site <https://example.com>`_.\n")
	if !strings.Contains(got, `<a href="https://example.com"`) {
		t.Errorf("https URL should render a link, got %q", got)
	}
}

func TestRender_EmbeddedHyperlinkRelativeAllowed(t *testing.T) {
	got, _ := rst.Render("See `docs <docs/index.html>`_.\n")
	if !strings.Contains(got, `<a href="docs/index.html"`) {
		t.Errorf("relative URL should render a link, got %q", got)
	}
}

// ── Real-world package patterns ───────────────────────────────────────────────

// TestRender_SetuptoolsStyle tests the badge-heavy pattern used by setuptools,
// pip, and many PyPA packages.
func TestRender_SetuptoolsStyle(t *testing.T) {
	input := ".. |pypi| image:: https://img.shields.io/pypi/v/pkg.svg\n" +
		"   :target: https://pypi.org/project/pkg\n\n" +
		".. |tests| image:: https://github.com/org/pkg/badge.svg\n" +
		"   :target: https://github.com/org/pkg/actions\n" +
		"   :alt: tests\n\n" +
		"|pypi| |tests|\n\n" +
		"Description here.\n"
	got, _ := rst.Render(input)
	if !strings.Contains(got, `href="https://pypi.org/project/pkg"`) {
		t.Errorf("missing pypi link in %q", got)
	}
	if !strings.Contains(got, `alt="tests"`) {
		t.Errorf("missing alt text in %q", got)
	}
	if !strings.Contains(got, "<p>Description here.</p>") {
		t.Errorf("missing description paragraph in %q", got)
	}
}

// TestRender_ColoramaStyle tests the RST that previously caused an infinite loop.
// colorama uses hyperlink targets (.. _name: url) before paragraphs.
func TestRender_ColoramaStyle(t *testing.T) {
	input := ".. _colorama: https://example.com\n\nSome text.\n"
	run(t, input, "<p>Some text.</p>", false)
}

// TestRender_RequestsStyle tests the common pattern of RST with headings,
// paragraphs, and inline hyperlinks — used by requests, httpx, and others.
func TestRender_RequestsStyle(t *testing.T) {
	input := "Requests\n========\n\n" +
		"**Requests** is an elegant HTTP library.\n\n" +
		"See the `documentation <https://docs.python-requests.org>`_ for details.\n\n" +
		"Installation\n------------\n\n" +
		"Install via pip::\n\n" +
		"   pip install requests\n"
	got, _ := rst.Render(input)
	if !strings.Contains(got, "<h1>Requests</h1>") {
		t.Errorf("missing h1 in %q", got)
	}
	if !strings.Contains(got, "<strong>Requests</strong>") {
		t.Errorf("missing bold in %q", got)
	}
	if !strings.Contains(got, `href="https://docs.python-requests.org"`) {
		t.Errorf("missing link in %q", got)
	}
	if !strings.Contains(got, "<h2>Installation</h2>") {
		t.Errorf("missing h2 in %q", got)
	}
	if !strings.Contains(got, "pip install requests") {
		t.Errorf("missing literal block content in %q", got)
	}
}

// ── Lists ──────────────────────────────────────────────────────────────────────

func TestRender_BulletListDash(t *testing.T) {
	run(t, "- item one\n- item two\n", "<li>", false)
}

func TestRender_BulletListStar(t *testing.T) {
	run(t, "* item one\n* item two\n", "<li>", false)
}

func TestRender_BulletListItemContent(t *testing.T) {
	got, _ := rst.Render("- first\n- second\n")
	if !strings.Contains(got, "<li>first</li>") {
		t.Errorf("expected list items, got %q", got)
	}
}

func TestRender_BulletListWithInlineMarkup(t *testing.T) {
	run(t, "- Use **pip** to install.\n- Run `python setup.py`.\n", "<strong>pip</strong>", false)
}

// ── Literal blocks ────────────────────────────────────────────────────────────

func TestRender_LiteralBlock(t *testing.T) {
	run(t, "Example::\n\n   some code\n   more code\n", "<pre><code>", false)
}

func TestRender_LiteralBlockContent(t *testing.T) {
	run(t, "Example::\n\n   hello world\n", "hello world", false)
}

func TestRender_LiteralBlockOnlyColons(t *testing.T) {
	// A line of just "::" introduces a literal block with no preceding text.
	got, _ := rst.Render("::\n\n   standalone block\n")
	if !strings.Contains(got, "standalone block") {
		t.Errorf("expected literal block content, got %q", got)
	}
}

func TestRender_LiteralBlockPreservesIndentation(t *testing.T) {
	input := "Code::\n\n   def foo():\n       return 1\n"
	got, _ := rst.Render(input)
	// The indentation relative to the block base should be preserved.
	if !strings.Contains(got, "def foo():") {
		t.Errorf("missing code content in %q", got)
	}
}
