package model

// ParamKind is the kind of a function parameter.
type ParamKind string

const (
	ParamPositionalOnly    ParamKind = "positional_only"
	ParamPositionalOrKeyword ParamKind = "positional_or_keyword"
	ParamVarPositional     ParamKind = "var_positional"
	ParamKeywordOnly       ParamKind = "keyword_only"
	ParamVarKeyword        ParamKind = "var_keyword"
)

// DocstringStyle represents the format of a docstring.
type DocstringStyle string

const (
	DocstringGoogle DocstringStyle = "google"
	DocstringNumpy  DocstringStyle = "numpy"
	DocstringSphinx DocstringStyle = "sphinx"
	DocstringPlain  DocstringStyle = "plain"
)

// Package represents a Python package.
type Package struct {
	Name    string    `json:"name"`
	Version string    `json:"version"`
	Modules []*Module `json:"modules"`
}

// Module represents a Python module (file).
type Module struct {
	Name       string       `json:"name"`
	Docstring  *Docstring   `json:"docstring,omitempty"`
	Functions  []*Function  `json:"functions,omitempty"`
	Classes    []*Class     `json:"classes,omitempty"`
	Attributes []*Attribute `json:"attributes,omitempty"`
	TypeAliases []*TypeAlias `json:"type_aliases,omitempty"`
	Imports    []*TypeRef   `json:"imports,omitempty"`
}

// Function represents a function or method.
type Function struct {
	Name       string       `json:"name"`
	Docstring  *Docstring   `json:"docstring,omitempty"`
	Signature  *Function    `json:"signature,omitempty"`
	Parameters []*Parameter `json:"parameters,omitempty"`
	Returns    *TypeExpr    `json:"returns,omitempty"`
	IsAsync    bool         `json:"is_async"`
	IsMethod   bool         `json:"is_method"`
	Decorators []string     `json:"decorators,omitempty"`
}

// Class represents a class.
type Class struct {
	Name       string       `json:"name"`
	Docstring  *Docstring   `json:"docstring,omitempty"`
	BaseClasses []*TypeRef  `json:"base_classes,omitempty"`
	Methods    []*Function  `json:"methods,omitempty"`
	Attributes []*Attribute `json:"attributes,omitempty"`
	Decorators []string     `json:"decorators,omitempty"`
}

// Parameter represents a function parameter.
type Parameter struct {
	Name     string      `json:"name"`
	Kind     ParamKind   `json:"kind"`
	Type     *TypeExpr   `json:"type,omitempty"`
	Default  string      `json:"default,omitempty"`
	DocParam *DocParam   `json:"doc,omitempty"`
}

// Attribute represents a class or module attribute.
type Attribute struct {
	Name       string     `json:"name"`
	Type       *TypeExpr  `json:"type,omitempty"`
	Value      string     `json:"value,omitempty"`
	Docstring  *Docstring `json:"docstring,omitempty"`
	IsProperty bool       `json:"is_property"`
}

// TypeAlias represents a type alias (type X = ...).
type TypeAlias struct {
	Name  string    `json:"name"`
	Value *TypeExpr `json:"value"`
}

// TypeRef represents a reference to a type (for imports, bases, etc).
type TypeRef struct {
	Name string    `json:"name"`
	Type *TypeExpr `json:"type,omitempty"`
}

// Docstring represents parsed docstring information.
type Docstring struct {
	Text   string       `json:"text"`
	Style  DocstringStyle `json:"style,omitempty"`
	Params []*DocParam  `json:"params,omitempty"`
	Returns *DocReturn  `json:"returns,omitempty"`
	Raises  []*DocRaises `json:"raises,omitempty"`
}

// DocParam represents a parameter documented in a docstring.
type DocParam struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// DocReturn represents return value documentation.
type DocReturn struct {
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// DocRaises represents documented exception.
type DocRaises struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}
