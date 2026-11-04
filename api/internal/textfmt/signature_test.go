package textfmt_test

import (
	"strings"
	"testing"

	"github.com/pypx/api/internal/textfmt"
)

// A default or annotation can hold a multi-line string, as fastapi's Doc("""...""")
// does. Each text format is line based, so a signature must stay on one line.
func TestSignatureStaysOnOneLine(t *testing.T) {
	const sig = "def f(a: Annotated[str, Doc(\"\"\"\n    text\r\n\"\"\"]) -> None"
	const flat = "def f(a: Annotated[str, Doc(\"\"\"     text \"\"\"]) -> None"

	docs := &textfmt.DocsInput{Available: true, Modules: []textfmt.DocModuleInput{{
		Name:      "m",
		Functions: []textfmt.DocSymbolInput{{Name: "f", Kind: "function", Signature: sig}},
	}}}
	if got := textfmt.FormatDocs(docs, ""); !strings.Contains(got, "### m.f — "+flat+"\n") {
		t.Errorf("docs.txt:\n%s", got)
	}
	if got := textfmt.FormatSymbols(docs, "", "", 10); !strings.Contains(got, "m.f\tfunction\t"+flat+"\n") {
		t.Errorf("symbols.txt:\n%s", got)
	}
	diff := textfmt.FormatDiff(&textfmt.DiffInput{
		Package:    "m",
		From:       "1",
		To:         "2",
		APIChanges: textfmt.APIDiff{Changed: []textfmt.APIChange{{Path: "m.f", FromSig: sig, ToSig: sig + " [x]"}}},
	})
	if want := "(was: " + flat + "; now: " + flat + " [x])\n"; !strings.Contains(diff, want) {
		t.Errorf("diff.txt:\n%s", diff)
	}
}
