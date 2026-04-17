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

	sym := convertFunction(fn, nil)

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

	sym := convertFunction(fn, nil)

	// Annotation return type wins over docstring
	if sym.Returns == nil || sym.Returns.Type != "QuerySet" {
		t.Errorf("Returns.Type = %v, want QuerySet", sym.Returns)
	}
	// Description comes from docstring
	if sym.Returns.Description != "A list of things" {
		t.Errorf("Returns.Description = %q", sym.Returns.Description)
	}
}

func TestConvertFunction_Raises(t *testing.T) {
	fn := &model.Function{
		Name: "delete",
		Docstring: &model.Docstring{
			Style: model.DocstringSphinx,
			Text:  "Delete the object.\n\n:raises ValueError: If pk is None\n:raises PermissionError: If not allowed",
			Raises: []*model.DocRaises{
				{Type: "ValueError", Description: "If pk is None"},
				{Type: "PermissionError", Description: "If not allowed"},
			},
		},
	}

	sym := convertFunction(fn, nil)

	if len(sym.Raises) != 2 {
		t.Fatalf("Raises len = %d, want 2", len(sym.Raises))
	}
	if sym.Raises[0].Type != "ValueError" {
		t.Errorf("Raises[0].Type = %q, want ValueError", sym.Raises[0].Type)
	}
	if sym.Raises[0].Description != "If pk is None" {
		t.Errorf("Raises[0].Description = %q", sym.Raises[0].Description)
	}
	if sym.Raises[1].Type != "PermissionError" {
		t.Errorf("Raises[1].Type = %q", sym.Raises[1].Type)
	}
}

func TestConvertFunction_NoRaises(t *testing.T) {
	fn := &model.Function{Name: "noop"}
	sym := convertFunction(fn, nil)
	if sym.Raises != nil {
		t.Errorf("Raises should be nil for function with no docstring raises, got %v", sym.Raises)
	}
}

func TestConvertClass_Methods(t *testing.T) {
	cls := &model.Class{
		Name: "QuerySet",
		Docstring: &model.Docstring{
			Style: model.DocstringSphinx,
			Text:  "A lazy database lookup for a set of objects.",
		},
		BaseClasses: []*model.TypeRef{{Name: "object"}},
		Methods: []*model.Function{
			{
				Name: "filter",
				Parameters: []*model.Parameter{
					{Name: "self", Kind: model.ParamPositionalOrKeyword},
					{
						Name: "kwargs",
						Kind: model.ParamVarKeyword,
						DocParam: &model.DocParam{
							Name:        "kwargs",
							Type:        "Any",
							Description: "Field lookups",
						},
					},
				},
				Docstring: &model.Docstring{
					Style: model.DocstringSphinx,
					Text:  "Return a new QuerySet filtered by the given lookups.",
				},
			},
			{
				Name: "first",
				Docstring: &model.Docstring{
					Style:   model.DocstringSphinx,
					Text:    "Return the first object matched.\n\n:rtype: Model or None",
					Returns: &model.DocReturn{Type: "Model or None"},
				},
			},
		},
	}

	sym := convertClass(cls, make(stubIndex), "mymod")

	if len(sym.Methods) != 2 {
		t.Fatalf("Methods len = %d, want 2", len(sym.Methods))
	}
	if sym.Methods[0].Name != "filter" {
		t.Errorf("Methods[0].Name = %q, want filter", sym.Methods[0].Name)
	}
	if sym.Methods[0].Kind != "function" {
		t.Errorf("Methods[0].Kind = %q, want function", sym.Methods[0].Kind)
	}
	// Docstring type backfill on method param (kwargs has no annotation, docstring type used)
	kwargsParam := sym.Methods[0].Parameters[1]
	if kwargsParam.Type != "Any" {
		t.Errorf("kwargs.Type = %q, want Any", kwargsParam.Type)
	}
	// Return type from docstring on method
	if sym.Methods[1].Returns == nil || sym.Methods[1].Returns.Type != "Model or None" {
		t.Errorf("first.Returns = %v, want type 'Model or None'", sym.Methods[1].Returns)
	}
	// Class docstring still present
	if sym.Docstring != "A lazy database lookup for a set of objects." {
		t.Errorf("class Docstring = %q", sym.Docstring)
	}
}

func TestConvertClass_NoMethods(t *testing.T) {
	cls := &model.Class{Name: "Empty"}
	sym := convertClass(cls, make(stubIndex), "mymod")
	if sym.Methods != nil {
		t.Errorf("Methods should be nil for class with no methods, got %v", sym.Methods)
	}
}

func TestConvertFunction_StubTypeBackfill(t *testing.T) {
	// Source function has no type annotation; stub has type info.
	src := &model.Function{
		Name: "filter",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "kwargs", Kind: model.ParamVarKeyword},
		},
	}
	stub := &model.Function{
		Name: "filter",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "kwargs", Kind: model.ParamVarKeyword, Type: &model.TypeExpr{Raw: "Any"}},
		},
		Returns: &model.TypeExpr{Raw: "QuerySet[_QS]"},
	}

	sym := convertFunction(src, stub)

	if sym.Returns == nil {
		t.Fatal("convertFunction: Returns should be backfilled from stub")
	}
	if sym.Returns.Type != "QuerySet[_QS]" {
		t.Errorf("convertFunction: Returns.Type = %q, want %q", sym.Returns.Type, "QuerySet[_QS]")
	}
	// self has no type — stub has none either
	if sym.Parameters[0].Type != "" {
		t.Errorf("convertFunction: self.Type = %q, want \"\"", sym.Parameters[0].Type)
	}
	// kwargs filled from stub
	if sym.Parameters[1].Type != "Any" {
		t.Errorf("convertFunction: kwargs.Type = %q, want \"Any\"", sym.Parameters[1].Type)
	}
}

func TestConvertFunction_SourceAnnotationWinsOverStub(t *testing.T) {
	src := &model.Function{
		Name: "save",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "value", Kind: model.ParamPositionalOrKeyword, Type: &model.TypeExpr{Raw: "str"}},
		},
		Returns: &model.TypeExpr{Raw: "None"},
	}
	stub := &model.Function{
		Name: "save",
		Parameters: []*model.Parameter{
			{Name: "self", Kind: model.ParamPositionalOrKeyword},
			{Name: "value", Kind: model.ParamPositionalOrKeyword, Type: &model.TypeExpr{Raw: "Any"}},
		},
		Returns: &model.TypeExpr{Raw: "int"},
	}

	sym := convertFunction(src, stub)

	// Source annotation wins
	if sym.Parameters[1].Type != "str" {
		t.Errorf("convertFunction: value.Type = %q, want \"str\" (source wins)", sym.Parameters[1].Type)
	}
	if sym.Returns == nil || sym.Returns.Type != "None" {
		t.Errorf("convertFunction: Returns.Type = %q, want \"None\" (source wins)", sym.Returns.Type)
	}
}

func TestConvertFunction_NilStub(t *testing.T) {
	src := &model.Function{
		Name:       "simple",
		Parameters: []*model.Parameter{{Name: "x", Kind: model.ParamPositionalOrKeyword}},
	}
	// Should not panic with nil stub
	sym := convertFunction(src, nil)
	if sym.Name != "simple" {
		t.Errorf("convertFunction with nil stub: Name = %q, want \"simple\"", sym.Name)
	}
}

func TestConvertFunction_DocstringReturnWinsOverStub(t *testing.T) {
	src := &model.Function{
		Name: "fetch",
		Docstring: &model.Docstring{
			Returns: &model.DocReturn{Type: "Response"},
		},
	}
	stub := &model.Function{
		Name:    "fetch",
		Returns: &model.TypeExpr{Raw: "Any"},
	}
	sym := convertFunction(src, stub)
	if sym.Returns == nil || sym.Returns.Type != "Response" {
		t.Errorf("docstring return should win over stub: Returns = %v", sym.Returns)
	}
}
