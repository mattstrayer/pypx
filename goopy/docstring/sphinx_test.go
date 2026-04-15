package docstring

import (
	"testing"

	"github.com/pypx/goopy/model"
)

// TestParseSphinxBasic covers a full docstring with :param, :type, :returns, :rtype, and :raises.
func TestParseSphinxBasic(t *testing.T) {
	raw := `Short summary.

Extended description paragraph.

:param x: The first param.
:type x: int
:param y: The second param.
:type y: str
:returns: The result description.
:rtype: bool
:raises ValueError: If x is negative.
:raises TypeError: If y is not a string.`

	doc := parseSphinx(raw)

	if doc.Style != model.DocstringSphinx {
		t.Errorf("Style = %q, want %q", doc.Style, model.DocstringSphinx)
	}
	if doc.Text != raw {
		t.Errorf("Text not preserved")
	}

	// Params
	if len(doc.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(doc.Params))
	}
	assertParam(t, doc.Params[0], "x", "int", "The first param.")
	assertParam(t, doc.Params[1], "y", "str", "The second param.")

	// Returns
	if doc.Returns == nil {
		t.Fatal("Returns is nil")
	}
	if doc.Returns.Description != "The result description." {
		t.Errorf("Returns.Description = %q, want %q", doc.Returns.Description, "The result description.")
	}
	if doc.Returns.Type != "bool" {
		t.Errorf("Returns.Type = %q, want %q", doc.Returns.Type, "bool")
	}

	// Raises
	if len(doc.Raises) != 2 {
		t.Fatalf("len(Raises) = %d, want 2", len(doc.Raises))
	}
	assertRaises(t, doc.Raises[0], "ValueError", "If x is negative.")
	assertRaises(t, doc.Raises[1], "TypeError", "If y is not a string.")
}

// TestParseSphinxInlineType covers the ":param type name:" inline-type syntax.
func TestParseSphinxInlineType(t *testing.T) {
	raw := `:param int x: The x value.
:param str y: The y value.`

	doc := parseSphinx(raw)

	if len(doc.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(doc.Params))
	}
	assertParam(t, doc.Params[0], "x", "int", "The x value.")
	assertParam(t, doc.Params[1], "y", "str", "The y value.")
}

// TestParseSphinxMultiLine covers multi-line param and return descriptions.
func TestParseSphinxMultiLine(t *testing.T) {
	raw := `:param x: First line of description.
    Continuation of description.
    Third line.
:returns: First line of return.
    Return continuation.`

	doc := parseSphinx(raw)

	if len(doc.Params) != 1 {
		t.Fatalf("len(Params) = %d, want 1", len(doc.Params))
	}
	wantDesc := "First line of description. Continuation of description. Third line."
	if doc.Params[0].Description != wantDesc {
		t.Errorf("Params[0].Description = %q, want %q", doc.Params[0].Description, wantDesc)
	}

	if doc.Returns == nil {
		t.Fatal("Returns is nil")
	}
	wantReturn := "First line of return. Return continuation."
	if doc.Returns.Description != wantReturn {
		t.Errorf("Returns.Description = %q, want %q", doc.Returns.Description, wantReturn)
	}
}

// TestParseSphinxNoType covers params documented without separate :type lines.
func TestParseSphinxNoType(t *testing.T) {
	raw := `:param alpha: Alpha value.
:param beta: Beta value.
:returns: Sum of alpha and beta.`

	doc := parseSphinx(raw)

	if len(doc.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(doc.Params))
	}
	assertParam(t, doc.Params[0], "alpha", "", "Alpha value.")
	assertParam(t, doc.Params[1], "beta", "", "Beta value.")

	if doc.Returns == nil {
		t.Fatal("Returns is nil")
	}
	if doc.Returns.Description != "Sum of alpha and beta." {
		t.Errorf("Returns.Description = %q", doc.Returns.Description)
	}
	if doc.Returns.Type != "" {
		t.Errorf("Returns.Type = %q, want empty", doc.Returns.Type)
	}
}

// TestParseSphinxTypeBeforeParam covers :type appearing before :param for the same name.
func TestParseSphinxTypeBeforeParam(t *testing.T) {
	raw := `:type x: int
:param x: The x value.`

	doc := parseSphinx(raw)

	if len(doc.Params) != 1 {
		t.Fatalf("len(Params) = %d, want 1", len(doc.Params))
	}
	assertParam(t, doc.Params[0], "x", "int", "The x value.")
}

// TestParseSphinxReturnAlias covers the ":return:" alias (no 's').
func TestParseSphinxReturnAlias(t *testing.T) {
	raw := `:return: The computed value.
:rtype: float`

	doc := parseSphinx(raw)

	if doc.Returns == nil {
		t.Fatal("Returns is nil")
	}
	if doc.Returns.Description != "The computed value." {
		t.Errorf("Returns.Description = %q", doc.Returns.Description)
	}
	if doc.Returns.Type != "float" {
		t.Errorf("Returns.Type = %q, want float", doc.Returns.Type)
	}
}

// helpers

func assertParam(t *testing.T, p *model.DocParam, name, typ, desc string) {
	t.Helper()
	if p.Name != name {
		t.Errorf("param Name = %q, want %q", p.Name, name)
	}
	if p.Type != typ {
		t.Errorf("param[%s].Type = %q, want %q", name, p.Type, typ)
	}
	if p.Description != desc {
		t.Errorf("param[%s].Description = %q, want %q", name, p.Description, desc)
	}
}

func assertRaises(t *testing.T, r *model.DocRaises, typ, desc string) {
	t.Helper()
	if r.Type != typ {
		t.Errorf("raises Type = %q, want %q", r.Type, typ)
	}
	if r.Description != desc {
		t.Errorf("raises[%s].Description = %q, want %q", typ, r.Description, desc)
	}
}
