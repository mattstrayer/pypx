package handler

import (
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
