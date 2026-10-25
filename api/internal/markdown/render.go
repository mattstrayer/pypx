package markdown

import (
	"bytes"

	highlighting "github.com/yuin/goldmark-highlighting/v3"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
)

var mdParser = parser.New(
	parser.WithExtensions(
		extension.GFMParser,
		highlighting.Parser,
	),
)

func newRenderer(opts ...html.Option) html.Renderer {
	return html.New(append(opts, html.WithExtensions(
		extension.GFMHTMLRenderer,
		highlighting.NewHTMLRenderer(
			highlighting.WithStyle("monokai"),
		),
	))...)
}

var (
	md     = newRenderer(html.WithUnsafe())
	mdSafe = newRenderer()
)

func convert(r html.Renderer, src string) (string, error) {
	source := []byte(src)
	var buf bytes.Buffer
	if err := r.Render(&buf, source, mdParser.Parse(source)); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Render converts markdown source to HTML. Returns empty string for empty input.
func Render(src string) (string, error) {
	if src == "" {
		return "", nil
	}

	return convert(md, src)
}

// RenderSafe renders markdown to HTML, stripping raw HTML to prevent XSS.
// Use this for third-party content (changelogs, etc).
func RenderSafe(src string) (string, error) {
	if src == "" {
		return "", nil
	}

	return convert(mdSafe, src)
}
