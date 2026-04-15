package docstring

import (
	"testing"

	"github.com/pypx/goopy/model"
)

func TestParseGoogleBasic(t *testing.T) {
	raw := `Short summary.

Extended description here.

Args:
    x: The first param.
    y: The second param.

Returns:
    str: The result description.

Raises:
    ValueError: If x is negative.`

	doc := parseGoogle(raw)

	if doc.Style != model.DocstringGoogle {
		t.Errorf("Style = %q, want %q", doc.Style, model.DocstringGoogle)
	}
	if doc.Text != raw {
		t.Errorf("Text not preserved")
	}

	// Params
	if len(doc.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(doc.Params))
	}
	if doc.Params[0].Name != "x" {
		t.Errorf("Params[0].Name = %q, want %q", doc.Params[0].Name, "x")
	}
	if doc.Params[0].Description != "The first param." {
		t.Errorf("Params[0].Description = %q", doc.Params[0].Description)
	}
	if doc.Params[1].Name != "y" {
		t.Errorf("Params[1].Name = %q, want %q", doc.Params[1].Name, "y")
	}

	// Returns
	if doc.Returns == nil {
		t.Fatal("Returns is nil")
	}
	if doc.Returns.Type != "str" {
		t.Errorf("Returns.Type = %q, want %q", doc.Returns.Type, "str")
	}
	if doc.Returns.Description != "The result description." {
		t.Errorf("Returns.Description = %q", doc.Returns.Description)
	}

	// Raises
	if len(doc.Raises) != 1 {
		t.Fatalf("len(Raises) = %d, want 1", len(doc.Raises))
	}
	if doc.Raises[0].Type != "ValueError" {
		t.Errorf("Raises[0].Type = %q, want %q", doc.Raises[0].Type, "ValueError")
	}
	if doc.Raises[0].Description != "If x is negative." {
		t.Errorf("Raises[0].Description = %q", doc.Raises[0].Description)
	}
}

func TestParseGoogleMultiLineParam(t *testing.T) {
	raw := `Do something.

Args:
    x: The first param description that
        continues on the next line.
    y: Short param.`

	doc := parseGoogle(raw)

	if len(doc.Params) != 2 {
		t.Fatalf("len(Params) = %d, want 2", len(doc.Params))
	}

	want := "The first param description that continues on the next line."
	if doc.Params[0].Description != want {
		t.Errorf("Params[0].Description = %q, want %q", doc.Params[0].Description, want)
	}
	if doc.Params[1].Name != "y" {
		t.Errorf("Params[1].Name = %q, want %q", doc.Params[1].Name, "y")
	}
}

func TestParseGoogleNoParams(t *testing.T) {
	raw := `Get the current value.

Returns:
    int: The current value.`

	doc := parseGoogle(raw)

	if len(doc.Params) != 0 {
		t.Errorf("len(Params) = %d, want 0", len(doc.Params))
	}
	if doc.Returns == nil {
		t.Fatal("Returns is nil")
	}
	if doc.Returns.Type != "int" {
		t.Errorf("Returns.Type = %q, want %q", doc.Returns.Type, "int")
	}
	if doc.Returns.Description != "The current value." {
		t.Errorf("Returns.Description = %q", doc.Returns.Description)
	}
}

func TestParseGoogleParamWithType(t *testing.T) {
	raw := `Process data.

Args:
    x (int): An integer input.
    name (str): A string name.
    items (list): A list of things.`

	doc := parseGoogle(raw)

	if len(doc.Params) != 3 {
		t.Fatalf("len(Params) = %d, want 3", len(doc.Params))
	}

	cases := []struct {
		name  string
		typ   string
		desc  string
	}{
		{"x", "int", "An integer input."},
		{"name", "str", "A string name."},
		{"items", "list", "A list of things."},
	}

	for i, tc := range cases {
		p := doc.Params[i]
		if p.Name != tc.name {
			t.Errorf("Params[%d].Name = %q, want %q", i, p.Name, tc.name)
		}
		if p.Type != tc.typ {
			t.Errorf("Params[%d].Type = %q, want %q", i, p.Type, tc.typ)
		}
		if p.Description != tc.desc {
			t.Errorf("Params[%d].Description = %q, want %q", i, p.Description, tc.desc)
		}
	}
}
