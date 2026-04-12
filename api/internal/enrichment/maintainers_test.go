package enrichment_test

import (
	"testing"

	"github.com/pypx/api/internal/enrichment"
	"github.com/pypx/api/internal/pypi"
)

func TestParseMaintainers(t *testing.T) {
	tests := []struct {
		name string
		info pypi.PackageInfo
		want []enrichment.Maintainer
	}{
		{
			name: "rfc2822 author email with name",
			info: pypi.PackageInfo{
				AuthorEmail: "Kenneth Reitz <me@kennethreitz.org>",
			},
			want: []enrichment.Maintainer{
				{Name: "Kenneth Reitz", Email: "me@kennethreitz.org"},
			},
		},
		{
			name: "multiple authors in email field",
			info: pypi.PackageInfo{
				AuthorEmail: "Alice <alice@example.com>, Bob <bob@example.com>",
			},
			want: []enrichment.Maintainer{
				{Name: "Alice", Email: "alice@example.com"},
				{Name: "Bob", Email: "bob@example.com"},
			},
		},
		{
			name: "bare email only",
			info: pypi.PackageInfo{
				AuthorEmail: "someone@example.com",
			},
			want: []enrichment.Maintainer{
				{Email: "someone@example.com"},
			},
		},
		{
			name: "separate author name field",
			info: pypi.PackageInfo{
				Author:      "Guido van Rossum",
				AuthorEmail: "",
			},
			want: []enrichment.Maintainer{
				{Name: "Guido van Rossum"},
			},
		},
		{
			name: "maintainer fields used when author is empty",
			info: pypi.PackageInfo{
				Author:          "",
				AuthorEmail:     "",
				Maintainer:      "Django Software Foundation",
				MaintainerEmail: "foundation@djangoproject.com",
			},
			want: []enrichment.Maintainer{
				{Name: "Django Software Foundation", Email: "foundation@djangoproject.com"},
			},
		},
		{
			name: "empty fields returns nil",
			info: pypi.PackageInfo{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enrichment.ParseMaintainers(tt.info)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseMaintainers() returned %d items, want %d\ngot:  %+v\nwant: %+v",
					len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("item %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractDocURL(t *testing.T) {
	tests := []struct {
		name        string
		projectURLs map[string]string
		want        string
	}{
		{
			name:        "documentation key",
			projectURLs: map[string]string{"Documentation": "https://docs.example.com"},
			want:        "https://docs.example.com",
		},
		{
			name:        "case insensitive docs key",
			projectURLs: map[string]string{"docs": "https://docs.example.com"},
			want:        "https://docs.example.com",
		},
		{
			name:        "no doc url returns empty",
			projectURLs: map[string]string{"Source": "https://github.com/example/pkg"},
			want:        "",
		},
		{
			name:        "nil map returns empty",
			projectURLs: nil,
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enrichment.ExtractDocURL(tt.projectURLs)
			if got != tt.want {
				t.Errorf("ExtractDocURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
