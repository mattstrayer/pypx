package textfmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pypx/api/internal/textfmt"
)

func fixtureDocs() *textfmt.DocsInput {
	return &textfmt.DocsInput{
		Package:   "httpx",
		Version:   "0.27.0",
		Available: true,
		Modules: []textfmt.DocModuleInput{
			{
				Name: "httpx",
				Functions: []textfmt.DocSymbolInput{
					{
						Name:      "get",
						Kind:      "function",
						Signature: "def get(url: str, **kwargs) -> Response",
						Docstring: "Send a GET request.",
						Parameters: []textfmt.DocParamInput{
							{Name: "url", Type: "str", Description: "Request URL."},
							{Name: "kwargs", Type: "Any", Kind: "var_keyword"},
						},
						Returns: &textfmt.DocReturnInput{Type: "Response", Description: "The response object."},
					},
				},
				Classes: []textfmt.DocSymbolInput{
					{
						Name:      "Client",
						Kind:      "class",
						Signature: "class Client(BaseClient)",
						Docstring: "An HTTP client.",
						Methods: []textfmt.DocSymbolInput{
							{
								Name:      "get",
								Kind:      "method",
								Signature: "def get(self, url: str) -> Response",
								Docstring: "Send a GET request from this client.",
								Parameters: []textfmt.DocParamInput{
									{Name: "self"},
									{Name: "url", Type: "str", Description: "Request URL."},
								},
								Returns: &textfmt.DocReturnInput{Type: "Response"},
							},
						},
					},
				},
			},
			{
				Name: "httpx._auth",
				Functions: []textfmt.DocSymbolInput{
					{
						Name:      "build_auth_header",
						Kind:      "function",
						Signature: "def build_auth_header(creds: tuple) -> str",
						Docstring: "Build the Authorization header value.",
					},
				},
			},
		},
	}
}

func TestFormatDocs(t *testing.T) {
	got := textfmt.FormatDocs(fixtureDocs(), "")
	goldenPath := filepath.Join("testdata", "docs_sample.golden")
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update)", err)
	}
	if got != string(want) {
		t.Errorf("mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestFormatDocsPrefix(t *testing.T) {
	got := textfmt.FormatDocs(fixtureDocs(), "httpx.Client")
	goldenPath := filepath.Join("testdata", "docs_sample_prefix.golden")
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update)", err)
	}
	if got != string(want) {
		t.Errorf("mismatch.\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
	}
}

func TestFormatDocsPrefixNoMatch(t *testing.T) {
	got := textfmt.FormatDocs(fixtureDocs(), "nonexistent.symbol")
	want := "# no symbols matching prefix=nonexistent.symbol\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
