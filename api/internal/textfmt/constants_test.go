package textfmt

import (
	"strings"
	"testing"
)

func constantInput() *DocsInput {
	return &DocsInput{
		Package: "pkg",
		Modules: []DocModuleInput{
			{
				Name:      "pkg",
				Docstring: "The package.",
				Functions: []DocSymbolInput{{Name: "f", Signature: "def f()"}},
				Aliases:   []DocAliasInput{{Name: "LIMIT", Target: "pkg.core.LIMIT"}},
			},
			{
				Name:      "pkg.core",
				Docstring: "The core.",
				Functions: []DocSymbolInput{{Name: "g", Signature: "def g()"}},
				Constants: []DocAttributeInput{
					{Name: "LIMIT", Type: "int", Default: "10"},
					{Name: "TEXT", Default: "\"a\nb\""},
				},
			},
		},
	}
}

func TestFormatDocs_ModuleDocstringAndConstants(t *testing.T) {
	got := FormatDocs(constantInput(), "")
	want := "## pkg.core (module)\n\nThe core.\n\nConstants:\n  LIMIT: int = 10\n  TEXT = \"a b\"\n\n### pkg.core.g"
	if !strings.Contains(got, want) {
		t.Errorf("output = %q, want it to contain %q", got, want)
	}
	if !strings.Contains(got, "## pkg (module)\n\nThe package.\n\nAliases:\n  LIMIT -> pkg.core.LIMIT\n") {
		t.Errorf("output = %q, want the docstring above the aliases", got)
	}
}

func TestFormatDocs_PrefixSelectsConstants(t *testing.T) {
	tests := []struct {
		prefix string
		want   []string
		reject []string
	}{
		{
			prefix: "pkg.core",
			want:   []string{"The core.", "LIMIT: int = 10", "### pkg.core.g"},
			reject: []string{"The package."},
		},
		{
			// A prefix inside a module selects the constant and not the docstring.
			prefix: "pkg.core.LIMIT",
			want:   []string{"## pkg.core (module)", "Constants:\n  LIMIT: int = 10\n"},
			reject: []string{"The core.", "TEXT", "### pkg.core.g"},
		},
		{
			prefix: "pkg.core.g",
			want:   []string{"### pkg.core.g"},
			reject: []string{"The core.", "Constants:"},
		},
		{
			// An alias prefix follows to the constant it targets.
			prefix: "pkg.LIMIT",
			want:   []string{"LIMIT -> pkg.core.LIMIT", "LIMIT: int = 10"},
			reject: []string{"The core.", "The package."},
		},
	}
	for _, tt := range tests {
		got := FormatDocs(constantInput(), tt.prefix)
		for _, s := range tt.want {
			if !strings.Contains(got, s) {
				t.Errorf("prefix %q: output = %q, want %q", tt.prefix, got, s)
			}
		}
		for _, s := range tt.reject {
			if strings.Contains(got, s) {
				t.Errorf("prefix %q: output = %q, must not contain %q", tt.prefix, got, s)
			}
		}
	}
}

func TestFormatSymbol_Constant(t *testing.T) {
	tests := []struct{ path, want string }{
		{"pkg.core.LIMIT", "### pkg.core.LIMIT — LIMIT: int = 10\n\n"},
		{"pkg.LIMIT", "### pkg.core.LIMIT — LIMIT: int = 10\n\n"}, // By alias.
		{"pkg.core.TEXT", "### pkg.core.TEXT — TEXT = \"a b\"\n\n"},
	}
	for _, tt := range tests {
		if got, ok := FormatSymbol(constantInput(), tt.path); !ok || got != tt.want {
			t.Errorf("FormatSymbol(%q) = %q, %v, want %q", tt.path, got, ok, tt.want)
		}
	}
	if got, ok := FormatSymbol(constantInput(), "pkg.core.NOPE"); ok {
		t.Errorf("FormatSymbol(NOPE) = %q, want not found", got)
	}
}

func TestFormatSymbol_FunctionBeatsConstant(t *testing.T) {
	in := constantInput()
	in.Modules[1].Functions = []DocSymbolInput{{Name: "LIMIT", Signature: "def LIMIT()"}}
	got, ok := FormatSymbol(in, "pkg.core.LIMIT")
	if !ok || !strings.Contains(got, "def LIMIT()") {
		t.Errorf("FormatSymbol = %q, %v", got, ok)
	}
}

func TestFormatSymbols_ConstantKind(t *testing.T) {
	got := FormatSymbols(constantInput(), "", "constant", 100)
	want := "# path\tkind\tsignature\npkg.core.LIMIT\tconstant\tLIMIT: int = 10\npkg.core.TEXT\tconstant\tTEXT = \"a b\"\n"
	if got != want {
		t.Errorf("FormatSymbols = %q, want %q", got, want)
	}
	if all := FormatSymbols(constantInput(), "LIMIT", "", 100); !strings.Contains(all, "pkg.core.LIMIT\tconstant\t") || !strings.Contains(all, "pkg.LIMIT\talias\t") {
		t.Errorf("unfiltered = %q", all)
	}
}
