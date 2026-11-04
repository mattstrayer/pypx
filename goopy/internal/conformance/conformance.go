// Package conformance compares goopy's parser output with CPython's ast module.
package conformance

import (
	"sort"
	"strings"

	"github.com/pypx/goopy/ast"
	"github.com/pypx/goopy/parser"
)

// Def is one function or class definition reachable without entering a function body.
type Def struct {
	Q      string   `json:"q"`
	Params []string `json:"params"`
	Doc    bool     `json:"doc"`
}

// Parse returns definitions reachable without entering a function body, and parse errors.
func Parse(src []byte) ([]Def, []error) {
	p := parser.New(src)
	mod := p.Parse()
	var errs []error
	for _, e := range p.Errors() {
		errs = append(errs, e)
	}
	defs := []Def{}
	if mod != nil {
		walk(mod.Body, "", &defs)
	}
	Sort(defs)
	return defs, errs
}

// Sort orders defs by Q, then by joined params.
func Sort(defs []Def) {
	sort.SliceStable(defs, func(i, j int) bool {
		if defs[i].Q != defs[j].Q {
			return defs[i].Q < defs[j].Q
		}
		return strings.Join(defs[i].Params, "\x00") < strings.Join(defs[j].Params, "\x00")
	})
}

func walk(stmts []ast.Stmt, prefix string, out *[]Def) {
	for _, s := range stmts {
		switch n := s.(type) {
		case *ast.FunctionDef:
			*out = append(*out, Def{Q: prefix + n.Name, Params: params(n.Args), Doc: hasDoc(n.Body)})
		case *ast.ClassDef:
			*out = append(*out, Def{Q: prefix + n.Name, Params: []string{}, Doc: hasDoc(n.Body)})
			walk(n.Body, prefix+n.Name+".", out)
		case *ast.PassThrough:
			walk(n.Body, prefix, out)
		case *ast.If:
			walk(n.Body, prefix, out)
			walk(n.Orelse, prefix, out)
		}
	}
}

func params(a *ast.Arguments) []string {
	out := []string{}
	if a == nil {
		return out
	}
	for _, x := range a.PosOnlyArgs {
		out = append(out, x.Name)
	}
	for _, x := range a.Args {
		out = append(out, x.Name)
	}
	if a.VarArg != nil {
		out = append(out, "*"+a.VarArg.Name)
	}
	for _, x := range a.KwOnlyArgs {
		out = append(out, x.Name)
	}
	if a.VarKwArg != nil {
		out = append(out, "**"+a.VarKwArg.Name)
	}
	return out
}

func hasDoc(body []ast.Stmt) bool {
	if len(body) == 0 {
		return false
	}
	e, ok := body[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	c, ok := e.Value.(*ast.Constant)
	return ok && c.Kind == "str"
}
