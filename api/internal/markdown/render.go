package markdown

import (
	"bytes"

	"github.com/pypx/api/internal/safeurl"
	highlighting "github.com/yuin/goldmark-highlighting/v3"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/text"
	"github.com/yuin/goldmark/v2/util"
)

// urlAllowList replaces links and images whose URL safeurl refuses with their
// text. It runs last, so it sees every destination after the parser decodes
// entities and resolves references.
type urlAllowList struct{}

func (urlAllowList) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	source := reader.Source()
	var refused []ast.Node
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Link:
			if !safeurl.Link(v.Destination.Value(source)) {
				refused = append(refused, n)
			}
		case *ast.AutoLink:
			if !safeurl.Link(v.Destination.Value(source)) {
				refused = append(refused, n)
			}
		case *ast.Image:
			if !safeurl.Image(v.Destination.Value(source)) {
				refused = append(refused, n)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, n := range refused {
		parent := n.Parent()
		if auto, ok := n.(*ast.AutoLink); ok {
			parent.InsertBefore(n, ast.NewText(auto.Label))
		}
		for c := n.FirstChild(); c != nil; c = n.FirstChild() {
			parent.InsertBefore(n, c)
		}
		parent.RemoveChild(n)
	}
}

var mdParser = parser.New(
	parser.WithExtensions(
		extension.NewGFMParser(),
		highlighting.NewParser(),
	),
	parser.WithASTTransformers(util.Prioritized[parser.ASTTransformer](urlAllowList{}, 10000)),
)

var mdSafe = html.New(html.WithExtensions(
	extension.NewGFMHTMLRenderer(),
	highlighting.NewHTMLRenderer(
		highlighting.WithStyle("monokai"),
	),
))

// RenderSafe renders markdown to HTML, stripping raw HTML to prevent XSS.
// Use this for third-party content (changelogs, etc).
func RenderSafe(src string) (string, error) {
	if src == "" {
		return "", nil
	}

	source := []byte(src)
	var buf bytes.Buffer
	if err := mdSafe.Render(&buf, source, mdParser.Parse(source)); err != nil {
		return "", err
	}
	return buf.String(), nil
}
