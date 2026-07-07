package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
)

var mdSafe = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("monokai"),
		),
	),
)

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
