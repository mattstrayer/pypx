package textfmt

import (
	"strings"
	"testing"
)

func aliasInput() *DocsInput {
	return &DocsInput{
		Package: "pkg",
		Modules: []DocModuleInput{
			{Name: "pkg", Aliases: []DocAliasInput{
				{Name: "Client", Target: "pkg.client.Client"},
				{Name: "fetch", Target: "pkg.client.fetch"},
				{Name: "ghost", Target: "pkg.client.ghost"},
				{Name: "Fail", Target: "pkg.client.Fail"},
			}},
			{Name: "pkg.client",
				Functions:  []DocSymbolInput{{Name: "fetch", Signature: "def fetch()"}},
				Classes:    []DocSymbolInput{{Name: "Client", Signature: "class Client", Methods: []DocSymbolInput{{Name: "get", Signature: "def get()"}}}},
				Exceptions: []DocSymbolInput{{Name: "Fail", Signature: "class Fail(Exception)"}},
			},
		},
	}
}

func TestFormatSymbol_ResolvesAliases(t *testing.T) {
	tests := []struct{ path, want string }{
		{"pkg.Client", "### pkg.client.Client — class Client"},
		{"pkg.Client.get", "### pkg.client.Client.get — def get()"},
		{"pkg.fetch", "### pkg.client.fetch — def fetch()"},
		{"pkg.Fail", "### pkg.client.Fail — class Fail(Exception)"},
	}
	for _, tt := range tests {
		got, ok := FormatSymbol(aliasInput(), tt.path)
		if !ok || !strings.Contains(got, tt.want) {
			t.Errorf("FormatSymbol(%q) = %q, %v, want %q", tt.path, got, ok, tt.want)
		}
	}
	for _, path := range []string{"pkg.ghost", "pkg.Clientele", "pkg.Client.nope"} {
		if got, ok := FormatSymbol(aliasInput(), path); ok {
			t.Errorf("FormatSymbol(%q) = %q, want not found", path, got)
		}
	}
}

func TestFormatSymbol_DefinitionBeatsAlias(t *testing.T) {
	in := aliasInput()
	in.Modules[0].Functions = []DocSymbolInput{{Name: "fetch", Signature: "def own()"}}
	got, ok := FormatSymbol(in, "pkg.fetch")
	if !ok || !strings.Contains(got, "def own()") {
		t.Errorf("got %q, %v", got, ok)
	}
}

func TestFormatDocs_PrefixFollowsAliases(t *testing.T) {
	tests := []struct {
		prefix string
		want   []string
		reject []string
	}{
		{prefix: "pkg.Client", want: []string{"### pkg.client.Client — class Client", "### pkg.client.Client.get — def get()"}},
		{prefix: "pkg.Client.get", want: []string{"### pkg.client.Client.get — def get()"}, reject: []string{"class Client"}},
		{prefix: "pkg.fetch", want: []string{"### pkg.client.fetch — def fetch()"}},
		{prefix: "pkg.Fail", want: []string{"### pkg.client.Fail — class Fail(Exception)"}},
		{prefix: "pkg.client.fetch", want: []string{"### pkg.client.fetch"}, reject: []string{"class Client"}},
		// An alias with no block for its target still has a line, and no more.
		{prefix: "pkg.ghost", want: []string{"## pkg (module)", "Aliases:\n  ghost -> pkg.client.ghost\n"}, reject: []string{"###", "fetch"}},
		{prefix: "pkg.Clientele", want: []string{"# no symbols matching prefix=pkg.Clientele"}},
	}
	for _, tt := range tests {
		got := FormatDocs(aliasInput(), tt.prefix)
		for _, w := range tt.want {
			if !strings.Contains(got, w) {
				t.Errorf("prefix %q: missing %q in:\n%s", tt.prefix, w, got)
			}
		}
		for _, r := range tt.reject {
			if strings.Contains(got, r) {
				t.Errorf("prefix %q: unexpected %q in:\n%s", tt.prefix, r, got)
			}
		}
	}
}

// A prefix that covers both an alias and its target prints the target once.
func TestFormatDocs_AliasTargetNotRepeated(t *testing.T) {
	got := FormatDocs(aliasInput(), "pkg")
	if n := strings.Count(got, "### pkg.client.fetch"); n != 1 {
		t.Errorf("target printed %d times:\n%s", n, got)
	}
}

func TestFormatDocs_DefinitionBeatsAlias(t *testing.T) {
	in := aliasInput()
	in.Modules[0].Functions = []DocSymbolInput{{Name: "fetch", Signature: "def own()"}}
	got := FormatDocs(in, "pkg.fetch")
	if !strings.Contains(got, "def own()") || strings.Contains(got, "def fetch()") {
		t.Errorf("got:\n%s", got)
	}
}

func TestFormatDocs_AliasesBlock(t *testing.T) {
	got := FormatDocs(aliasInput(), "")
	want := "## pkg (module)\n\nAliases:\n" +
		"  Client -> pkg.client.Client\n" +
		"  fetch -> pkg.client.fetch\n" +
		"  ghost -> pkg.client.ghost\n" +
		"  Fail -> pkg.client.Fail\n\n\n" +
		"## pkg.client (module)\n\n### pkg.client.fetch"
	if !strings.HasPrefix(got, want) {
		t.Errorf("got:\n%s\nwant prefix:\n%s", got, want)
	}
	if strings.Count(got, "Aliases:") != 1 {
		t.Errorf("want one block, because pkg.client has no aliases:\n%s", got)
	}

	// The block sits above the symbols of a module that has both.
	in := aliasInput()
	in.Modules[1].Aliases = []DocAliasInput{{Name: "other", Target: "pkg.Client"}}
	got = FormatDocs(in, "pkg.client")
	if i, j := strings.Index(got, "  other -> pkg.Client\n"), strings.Index(got, "### pkg.client.fetch"); i < 0 || j < i {
		t.Errorf("block must come first:\n%s", got)
	}
	if strings.Contains(got, "fetch -> ") {
		t.Errorf("prefix must filter the block:\n%s", got)
	}
}

func TestFormatDocs_DefinedIn(t *testing.T) {
	in := &DocsInput{Modules: []DocModuleInput{{
		Name: "attr",
		Functions: []DocSymbolInput{
			{Name: "asdict", Signature: "def asdict()", Docstring: "Doc.", Origin: "attr._funcs"},
			{Name: "local", Signature: "def local()"},
		},
	}}}
	got := FormatDocs(in, "")
	if !strings.Contains(got, "### attr.asdict — def asdict()\nDefined in: attr._funcs\nDoc.\n") {
		t.Errorf("line must follow the heading:\n%s", got)
	}
	if strings.Count(got, "Defined in:") != 1 {
		t.Errorf("only a symbol with an origin gets the line:\n%s", got)
	}
	out, ok := FormatSymbol(in, "attr.asdict")
	if !ok || !strings.Contains(out, "Defined in: attr._funcs\n") {
		t.Errorf("FormatSymbol = %q, %v", out, ok)
	}
}

func TestFormatAlias(t *testing.T) {
	got, ok := FormatAlias(aliasInput(), "pkg.ghost")
	if !ok || got != "### pkg.ghost -> pkg.client.ghost\n\n" {
		t.Errorf("got %q, %v", got, ok)
	}
	for _, path := range []string{"ghost", "pkg.ghost.x", "pkg.client.ghost"} {
		if got, ok := FormatAlias(aliasInput(), path); ok {
			t.Errorf("FormatAlias(%q) = %q, want not found", path, got)
		}
	}
}

func TestFormatSymbols_ListsAliases(t *testing.T) {
	got := FormatSymbols(aliasInput(), "", "", 100)
	if !strings.Contains(got, "pkg.Client\talias\t-> pkg.client.Client\n") {
		t.Errorf("alias line missing:\n%s", got)
	}
	got = FormatSymbols(aliasInput(), "", "class", 100)
	if strings.Contains(got, "alias") {
		t.Errorf("kind filter must exclude aliases:\n%s", got)
	}
	got = FormatSymbols(aliasInput(), "fetch", "alias", 100)
	if !strings.Contains(got, "pkg.fetch\talias\t-> pkg.client.fetch\n") || strings.Contains(got, "pkg.Client") {
		t.Errorf("q and kind filter:\n%s", got)
	}
}
