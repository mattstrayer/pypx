package docstring

import (
	"testing"

	"github.com/pypx/goopy/model"
)

func TestParseNumPyBasic(t *testing.T) {
	raw := `Compute something useful.

Extended description of the function goes here.

Parameters
----------
x : int
    The first parameter.
y : str
    The second parameter.

Returns
-------
bool
    True if successful, False otherwise.

Raises
------
ValueError
    If x is negative.
`
	doc := parseNumPy(raw)

	if doc.Style != model.DocstringNumpy {
		t.Errorf("Style = %q, want %q", doc.Style, model.DocstringNumpy)
	}
	if doc.Text != raw {
		t.Errorf("Text not preserved")
	}

	// Parameters
	if len(doc.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(doc.Params))
	}
	if doc.Params[0].Name != "x" || doc.Params[0].Type != "int" {
		t.Errorf("Params[0] = {%q, %q}, want {x, int}", doc.Params[0].Name, doc.Params[0].Type)
	}
	if doc.Params[0].Description != "The first parameter." {
		t.Errorf("Params[0].Description = %q", doc.Params[0].Description)
	}
	if doc.Params[1].Name != "y" || doc.Params[1].Type != "str" {
		t.Errorf("Params[1] = {%q, %q}, want {y, str}", doc.Params[1].Name, doc.Params[1].Type)
	}

	// Returns
	if doc.Returns == nil {
		t.Fatal("Returns is nil")
	}
	if doc.Returns.Type != "bool" {
		t.Errorf("Returns.Type = %q, want bool", doc.Returns.Type)
	}
	if doc.Returns.Description != "True if successful, False otherwise." {
		t.Errorf("Returns.Description = %q", doc.Returns.Description)
	}

	// Raises
	if len(doc.Raises) != 1 {
		t.Fatalf("len(Raises) = %d, want 1", len(doc.Raises))
	}
	if doc.Raises[0].Type != "ValueError" {
		t.Errorf("Raises[0].Type = %q, want ValueError", doc.Raises[0].Type)
	}
	if doc.Raises[0].Description != "If x is negative." {
		t.Errorf("Raises[0].Description = %q", doc.Raises[0].Description)
	}
}

func TestParseNumPyOptionalType(t *testing.T) {
	raw := `Do something optional.

Parameters
----------
x : int
    Required integer.
y : str, optional
    Optional string, defaults to empty.
`
	doc := parseNumPy(raw)

	if len(doc.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(doc.Params))
	}
	if doc.Params[1].Type != "str, optional" {
		t.Errorf("Params[1].Type = %q, want %q", doc.Params[1].Type, "str, optional")
	}
	if doc.Params[1].Name != "y" {
		t.Errorf("Params[1].Name = %q, want y", doc.Params[1].Name)
	}
}

func TestParseNumPyMultiLineDesc(t *testing.T) {
	raw := `Function with multi-line parameter description.

Parameters
----------
data : list
    A list of values to process.
    Must be non-empty and contain only numbers.
`
	doc := parseNumPy(raw)

	if len(doc.Params) != 1 {
		t.Fatalf("len(Params) = %d, want 1", len(doc.Params))
	}
	p := doc.Params[0]
	if p.Name != "data" {
		t.Errorf("Params[0].Name = %q, want data", p.Name)
	}
	if p.Type != "list" {
		t.Errorf("Params[0].Type = %q, want list", p.Type)
	}
	want := "A list of values to process. Must be non-empty and contain only numbers."
	if p.Description != want {
		t.Errorf("Params[0].Description = %q, want %q", p.Description, want)
	}
}

func TestParseNumPyNoParams(t *testing.T) {
	raw := `Returns a constant.

Returns
-------
int
    Always returns 42.
`
	doc := parseNumPy(raw)

	if len(doc.Params) != 0 {
		t.Errorf("len(Params) = %d, want 0", len(doc.Params))
	}
	if doc.Returns == nil {
		t.Fatal("Returns is nil")
	}
	if doc.Returns.Type != "int" {
		t.Errorf("Returns.Type = %q, want int", doc.Returns.Type)
	}
	if doc.Returns.Description != "Always returns 42." {
		t.Errorf("Returns.Description = %q", doc.Returns.Description)
	}
}
