package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("monokai"),
		),
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

var mdSafe = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("monokai"),
		),
	),
)

// Render converts markdown source to HTML. Returns empty string for empty input.
func Render(src string) (string, error) {
	if src == "" {
		return "", nil
	}

	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderSafe renders markdown to HTML, stripping raw HTML to prevent XSS.
// Use this for third-party content (changelogs, etc).
func RenderSafe(src string) (string, error) {
	if src == "" {
		return "", nil
	}

	var buf bytes.Buffer
	if err := mdSafe.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
