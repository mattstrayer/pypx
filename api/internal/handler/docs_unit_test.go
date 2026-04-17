package handler

import (
	"strings"
	"testing"

	"github.com/pypx/goopy/model"
)

func TestCleanDocstringText_Sphinx(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text: `Validate the given value.

Does some important work here.

:param value: The value to validate
:type value: str
:returns: None
:rtype: None
:raises ValueError: If value is empty`,
	}
	got := cleanDocstringText(ds)
	want := "Validate the given value.\n\nDoes some important work here."
	if got != want {
		t.Errorf("cleanDocstringText(sphinx) =\n%q\nwant:\n%q", got, want)
	}
}

func TestCleanDocstringText_Google(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringGoogle,
		Text: `Fetch the queryset.

Extended description here.

Args:
    pk: Primary key
    qs: Queryset

Returns:
    QuerySet: The filtered queryset`,
	}
	got := cleanDocstringText(ds)
	want := "Fetch the queryset.\n\nExtended description here."
	if got != want {
		t.Errorf("cleanDocstringText(google) =\n%q\nwant:\n%q", got, want)
	}
}

func TestCleanDocstringText_Plain(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringPlain,
		Text:  "Just a plain description with no fields.",
	}
	got := cleanDocstringText(ds)
	want := "Just a plain description with no fields."
	if got != want {
		t.Errorf("cleanDocstringText(plain) = %q, want %q", got, want)
	}
}

func TestCleanDocstringText_Nil(t *testing.T) {
	if got := cleanDocstringText(nil); got != "" {
		t.Errorf("cleanDocstringText(nil) = %q, want empty", got)
	}
}

func TestCleanDocstringText_NoFields(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text:  "One-liner with no field markers.",
	}
	got := cleanDocstringText(ds)
	if got != "One-liner with no field markers." {
		t.Errorf("unexpected: %q", got)
	}
}

func TestCleanDocstringText_NumPy(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringNumpy,
		Text: `Compute the result.

More detail here.

Parameters
----------
x : int
    The input value.

Returns
-------
int
    The output value.`,
	}
	got := cleanDocstringText(ds)
	want := "Compute the result.\n\nMore detail here."
	if got != want {
		t.Errorf("cleanDocstringText(numpy) =\n%q\nwant:\n%q", got, want)
	}
}

func TestCleanDocstringText_SphinxProseColon(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text: `Return the result, which is an integer.

Note: this always returns a positive value.

:param x: The input
:type x: int`,
	}
	got := cleanDocstringText(ds)
	// "Note: this..." starts with "Note" not ":", so it must not be truncated.
	if !strings.Contains(got, "Note: this always returns a positive value.") {
		t.Errorf("prose colon was incorrectly truncated: %q", got)
	}
	// The :param line must be stripped.
	if strings.Contains(got, ":param") {
		t.Errorf(":param line not stripped: %q", got)
	}
}

func TestCleanDocstringText_EmptyText(t *testing.T) {
	ds := &model.Docstring{
		Style: model.DocstringSphinx,
		Text:  "",
	}
	got := cleanDocstringText(ds)
	if got != "" {
		t.Errorf("cleanDocstringText(empty text) = %q, want empty", got)
	}
}

func TestConvertFunction_DocstringTypeBackfill(t *testing.T) {
	fn := &model.Function{
		Name: "validate",
		Parameters: []*model.Parameter{
			{
				Name: "value",
				Kind: model.ParamPositionalOrKeyword,
				// No annotation type
				DocParam: &model.DocParam{
					Name:        "value",
					Type:        "str",
					Description: "The value to validate",
				},
			},
			{
				Name: "strict",
				Kind: model.ParamPositionalOrKeyword,
				// Has annotation — should NOT be overridden
				Type: &model.TypeExpr{Raw: "bool"},
				DocParam: &model.DocParam{
					Name: "strict",
					Type: "int", // docstring says int but annotation says bool — annotation wins
				},
			},
		},
		Docstring: &model.Docstring{
			Style: model.DocstringSphinx,
			Text:  "Validate the value.\n\n:param value: The value\n:type value: str",
			Returns: &model.DocReturn{
				Type:        "None",
				Description: "Nothing returned",
			},
		},
	}

	sym := convertFunction(fn)

	// value param: annotation absent, docstring type used
	if sym.Parameters[0].Type != "str" {
		t.Errorf("param[0].Type = %q, want str", sym.Parameters[0].Type)
	}
	if sym.Parameters[0].Description != "The value to validate" {
		t.Errorf("param[0].Description = %q", sym.Parameters[0].Description)
	}

	// strict param: annotation present, annotation wins
	if sym.Parameters[1].Type != "bool" {
		t.Errorf("param[1].Type = %q, want bool (annotation wins)", sym.Parameters[1].Type)
	}

	// Returns: no annotation on fn, docstring return used
	if sym.Returns == nil {
		t.Fatal("Returns should not be nil")
	}
	if sym.Returns.Type != "None" {
		t.Errorf("Returns.Type = %q, want None", sym.Returns.Type)
	}
	if sym.Returns.Description != "Nothing returned" {
		t.Errorf("Returns.Description = %q", sym.Returns.Description)
	}
}

func TestConvertFunction_AnnotationTakesPrecedence(t *testing.T) {
	fn := &model.Function{
		Name:    "fetch",
		Returns: &model.TypeExpr{Raw: "QuerySet"},
		Docstring: &model.Docstring{
			Style: model.DocstringSphinx,
			Returns: &model.DocReturn{
				Type:        "list",
				Description: "A list of things",
			},
		},
	}

	sym := convertFunction(fn)

	// Annotation return type wins over docstring
	if sym.Returns == nil || sym.Returns.Type != "QuerySet" {
		t.Errorf("Returns.Type = %v, want QuerySet", sym.Returns)
	}
	// Description comes from docstring
	if sym.Returns.Description != "A list of things" {
		t.Errorf("Returns.Description = %q", sym.Returns.Description)
	}
}
