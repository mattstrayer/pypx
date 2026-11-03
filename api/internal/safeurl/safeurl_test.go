package safeurl_test

import (
	"testing"

	"github.com/pypx/api/internal/safeurl"
)

func TestLink(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"https", "https://example.com/a?b=c#d", true},
		{"http", "http://example.com", true},
		{"upper case scheme", "HTTPS://example.com", true},
		{"mailto", "mailto:a@example.com", true},
		{"ftp", "ftp://example.com/a.tar.gz", true},
		{"tel", "tel:+15551234", true},
		{"empty", "", true},
		{"fragment", "#install", true},
		{"relative path", "docs/index.html", true},
		{"dot relative", "./docs/index.html", true},
		{"root relative", "/docs", true},
		{"scheme relative", "//example.com/a", true},
		{"query", "?a=b:c", true},
		{"colon after slash", "docs/a:b", true},
		{"colon after hash", "#a:b", true},
		{"surrounding spaces", "  https://example.com  ", true},
		{"encoded colon stays relative", "javascript%3Aalert(1)", true},
		// "&" cannot start a scheme and renderers escape it, so this never decodes.
		{"entity stays relative", "&#106;avascript:alert(1)", true},

		{"javascript", "javascript:alert(1)", false},
		{"odd case", "JaVaScRiPt:alert(1)", false},
		{"leading space", "  javascript:alert(1)", false},
		{"leading newline", "\njavascript:alert(1)", false},
		{"tab in scheme", "java\tscript:alert(1)", false},
		{"newline in scheme", "java\nscript:alert(1)", false},
		{"carriage return in scheme", "java\rscript:alert(1)", false},
		{"leading control", "\x01javascript:alert(1)", false},
		{"control in scheme", "java\x01script:alert(1)", false},
		{"control after allowed scheme", "https://a\x00b", false},
		{"delete char", "\x7fjavascript:alert(1)", false},
		{"no-break space", " javascript:alert(1)", false},
		{"line separator", " javascript:alert(1)", false},
		{"vbscript", "vbscript:msgbox(1)", false},
		{"data html", "data:text/html,<script>alert(1)</script>", false},
		{"data image in a link", "data:image/png;base64,AAAA", false},
		{"blob", "blob:https://example.com/a", false},
		{"file", "file:///etc/passwd", false},
		{"filesystem", "filesystem:https://example.com/a", false},
		{"unknown", "foo:bar", false},
		{"empty scheme", ":alert(1)", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := safeurl.Link(c.url); got != c.want {
				t.Errorf("Link(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}

func TestImage(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"https", "https://img.shields.io/pypi/v/x.svg", true},
		{"http", "http://example.com/a.png", true},
		{"relative", "docs/a.png", true},
		{"png", "data:image/png;base64,AAAA", true},
		{"gif", "data:image/gif;base64,AAAA", true},
		{"jpeg", "data:image/jpeg;base64,AAAA", true},
		{"webp", "data:image/webp;base64,AAAA", true},
		{"odd case data", "DATA:IMAGE/PNG;base64,AAAA", true},

		{"svg data", "data:image/svg+xml;base64,AAAA", false},
		{"html data", "data:text/html,hi", false},
		{"png prefix trick", "data:image/pngx;base64,AAAA", false},
		{"tab in data", "da\tta:image/png;base64,AAAA", false},
		{"javascript", "javascript:alert(1)", false},
		{"mailto", "mailto:a@example.com", false},
		{"ftp", "ftp://example.com/a.png", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := safeurl.Image(c.url); got != c.want {
				t.Errorf("Image(%q) = %v, want %v", c.url, got, c.want)
			}
		})
	}
}
