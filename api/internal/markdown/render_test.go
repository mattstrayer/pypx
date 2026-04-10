package markdown

import (
	"strings"
	"testing"
)

func TestRender_EmptyInput(t *testing.T) {
	got, err := Render("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestRender_PlainText(t *testing.T) {
	got, err := Render("Hello, world!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "<p>Hello, world!</p>") {
		t.Errorf("expected paragraph wrapping, got %q", got)
	}
}

func TestRender_Heading(t *testing.T) {
	got, err := Render("# My Package")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "<h1>My Package</h1>") {
		t.Errorf("expected h1 tag, got %q", got)
	}
}

func TestRender_GFM_Strikethrough(t *testing.T) {
	got, err := Render("~~deleted~~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "<del>deleted</del>") {
		t.Errorf("expected strikethrough, got %q", got)
	}
}

func TestRender_GFM_Autolink(t *testing.T) {
	got, err := Render("Visit https://example.com for info.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, `<a href="https://example.com"`) {
		t.Errorf("expected autolink, got %q", got)
	}
}

func TestRender_CodeBlock(t *testing.T) {
	input := "```python\nprint('hello')\n```"
	got, err := Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Syntax highlighting wraps tokens in spans with style attributes
	if !strings.Contains(got, "<pre") {
		t.Errorf("expected pre tag for code block, got %q", got)
	}
	if !strings.Contains(got, "print") {
		t.Errorf("expected code content, got %q", got)
	}
}

func TestRender_RawHTML(t *testing.T) {
	input := "<div class=\"banner\">Hello</div>"
	got, err := Render(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, `<div class="banner">Hello</div>`) {
		t.Errorf("expected raw HTML to pass through, got %q", got)
	}
}
