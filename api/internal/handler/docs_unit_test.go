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
