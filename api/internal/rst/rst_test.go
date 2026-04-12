package rst_test

import (
	"strings"
	"testing"

	"github.com/pypx/api/internal/rst"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantContain string
		wantEmpty   bool
	}{
		{
			name:      "empty input",
			input:     "",
			wantEmpty: true,
		},
		{
			name:        "plain paragraph",
			input:       "Hello world.\n",
			wantContain: "<p>Hello world.</p>",
		},
		{
			name:        "bold inline",
			input:       "This is **bold** text.",
			wantContain: "<strong>bold</strong>",
		},
		{
			name:        "italic inline",
			input:       "This is *italic* text.",
			wantContain: "<em>italic</em>",
		},
		{
			name:        "code inline",
			input:       "Use `my_func()` here.",
			wantContain: "<code>my_func()</code>",
		},
		{
			name:        "role inline",
			input:       "See :func:`requests.get` for details.",
			wantContain: "<code>requests.get</code>",
		},
		{
			name:        "h1 heading",
			input:       "My Title\n========\n",
			wantContain: "<h1>My Title</h1>",
		},
		{
			name:        "h2 heading",
			input:       "Section\n-------\n",
			wantContain: "<h2>Section</h2>",
		},
		{
			name:        "code-block directive",
			input:       ".. code-block:: python\n\n   x = 1\n   print(x)\n",
			wantContain: "<pre><code",
		},
		{
			name:        "code-block content",
			input:       ".. code-block:: python\n\n   x = 1\n",
			wantContain: "x = 1",
		},
		{
			name:        "note directive",
			input:       ".. note::\n\n   This is a note.\n",
			wantContain: "rst-note",
		},
		{
			name:        "warning directive",
			input:       ".. warning::\n\n   Be careful.\n",
			wantContain: "rst-warning",
		},
		{
			name:        "image directive",
			input:       ".. image:: /path/to/img.png\n",
			wantContain: "<img",
		},
		{
			name:      "toctree omitted",
			input:     ".. toctree::\n\n   page1\n   page2\n",
			wantEmpty: true,
		},
		{
			name:        "bullet list",
			input:       "- item one\n- item two\n",
			wantContain: "<li>",
		},
		{
			name:        "html escaping",
			input:       "Use <tag> & 'quotes'.",
			wantContain: "&lt;tag&gt;",
		},
		{
			name:        "literal block",
			input:       "Example::\n\n   some code\n   more code\n",
			wantContain: "<pre><code>",
		},
		{
			name:        "literal block content",
			input:       "Example::\n\n   hello world\n",
			wantContain: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rst.Render(tt.input)
			if err != nil {
				t.Fatalf("Render() error: %v", err)
			}
			if tt.wantEmpty {
				if strings.TrimSpace(got) != "" {
					t.Errorf("Render() = %q, want empty", got)
				}
				return
			}
			if !strings.Contains(got, tt.wantContain) {
				t.Errorf("Render() = %q\nwant substring: %q", got, tt.wantContain)
			}
		})
	}
}
