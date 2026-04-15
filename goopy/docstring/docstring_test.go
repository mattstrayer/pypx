package docstring

import (
	"testing"

	"github.com/pypx/goopy/model"
)

func TestDetectGoogle(t *testing.T) {
	raw := "Short summary.\n\nArgs:\n    x: Description.\n"
	if got := Detect(raw); got != model.DocstringGoogle {
		t.Errorf("Detect() = %q, want %q", got, model.DocstringGoogle)
	}
}

func TestDetectNumPy(t *testing.T) {
	raw := "Short summary.\n\nParameters\n----------\nx : int\n    Description.\n"
	if got := Detect(raw); got != model.DocstringNumpy {
		t.Errorf("Detect() = %q, want %q", got, model.DocstringNumpy)
	}
}

func TestDetectSphinx(t *testing.T) {
	raw := "Short summary.\n\n:param x: Description.\n:type x: int\n"
	if got := Detect(raw); got != model.DocstringSphinx {
		t.Errorf("Detect() = %q, want %q", got, model.DocstringSphinx)
	}
}

func TestDetectPlain(t *testing.T) {
	raw := "Just a simple description."
	if got := Detect(raw); got != model.DocstringPlain {
		t.Errorf("Detect() = %q, want %q", got, model.DocstringPlain)
	}
}

func TestParsePlain(t *testing.T) {
	raw := "Short summary.\n\nExtended description\nspanning two lines."
	doc := Parse(raw)
	if doc.Text != "Short summary.\n\nExtended description\nspanning two lines." {
		t.Errorf("Text = %q", doc.Text)
	}
	if doc.Style != model.DocstringPlain {
		t.Errorf("Style = %q, want %q", doc.Style, model.DocstringPlain)
	}
}

func TestParseSummaryOnly(t *testing.T) {
	raw := "Just a summary."
	doc := Parse(raw)
	if doc.Text != "Just a summary." {
		t.Errorf("Text = %q", doc.Text)
	}
	if len(doc.Params) != 0 {
		t.Errorf("Params should be empty, got %d", len(doc.Params))
	}
}
