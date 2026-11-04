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
