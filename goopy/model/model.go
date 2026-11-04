package model

// ParamKind is the kind of a function parameter.
type ParamKind string

const (
	ParamPositionalOnly      ParamKind = "positional_only"
	ParamPositionalOrKeyword ParamKind = "positional_or_keyword"
	ParamVarPositional       ParamKind = "var_positional"
	ParamKeywordOnly         ParamKind = "keyword_only"
	ParamVarKeyword          ParamKind = "var_keyword"
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
	Name        string       `json:"name"`
	Docstring   *Docstring   `json:"docstring,omitempty"`
	Functions   []*Function  `json:"functions,omitempty"`
	Classes     []*Class     `json:"classes,omitempty"`
	Attributes  []*Attribute `json:"attributes,omitempty"`
	TypeAliases []*TypeAlias `json:"type_aliases,omitempty"`
	Imports     []*TypeRef   `json:"imports,omitempty"`
	// Aliases lists names this module re-exports whose definition is published
	// in another module. Each definition is published once.
	Aliases []*Alias `json:"aliases,omitempty"`

	// The fields below are resolver input. ExtractPackage reads them and then
	// clears them. They never reach JSON.

	// ImportFroms records every import with its source module and level.
	ImportFroms []*Import `json:"-"`
	// Exports is the module's __all__, or nil if it has none.
	Exports *Exports `json:"-"`
	// Hidden holds definitions that are not public under the module's own rules.
	// A re-export from another module can still reach them.
	Hidden Definitions `json:"-"`
	// StubOnly holds definitions from a .pyi for names the .py may import. The
	// resolver adds them to the module if the import finds no definition.
	StubOnly Definitions `json:"-"`
}

// Alias is a public name whose definition is published elsewhere. Target is the
// dotted module and name of that definition.
type Alias struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

// Import is one name bound by an import statement. "from .a import b as c" is
// {Module: "a", Level: 1, Name: "b", Alias: "c"}. "from .a import *" has Name "*".
// "import a.b as c" has an empty Name because it binds a module.
type Import struct {
	Module string
	Level  int
	Name   string
	Alias  string
	// NotExported is true for a .pyi import that does not re-export its name.
	// __all__ can still publish the name.
	NotExported bool
}

// Exports is the statically known content of __all__.
type Exports struct {
	// Names lists the string literals in source order, without duplicates.
	Names []string
	// Refs lists dotted modules whose __all__ is included ("a.b" for a.b.__all__).
	Refs []string
	// Dynamic is true if some part of __all__ cannot be read statically.
	Dynamic bool
}

// Definitions groups the definition kinds of a module.
type Definitions struct {
	Functions   []*Function
	Classes     []*Class
	Attributes  []*Attribute
	TypeAliases []*TypeAlias
}

// Function represents a function or method.
type Function struct {
	Name       string       `json:"name"`
	Docstring  *Docstring   `json:"docstring,omitempty"`
	Parameters []*Parameter `json:"parameters,omitempty"`
	Returns    *TypeExpr    `json:"returns,omitempty"`
	IsAsync    bool         `json:"is_async"`
	IsMethod   bool         `json:"is_method"`
	Decorators []string     `json:"decorators,omitempty"`
	// Overloads lists the @overload signatures of this function.
	Overloads []*Function `json:"overloads,omitempty"`
	// Origin is the dotted module that defines this function, set on a re-export.
	Origin string `json:"origin,omitempty"`
	// OriginName is the name in Origin, set if the re-export renames the function.
	OriginName string `json:"-"`
}

// Class represents a class.
type Class struct {
	Name        string       `json:"name"`
	Docstring   *Docstring   `json:"docstring,omitempty"`
	BaseClasses []*TypeRef   `json:"base_classes,omitempty"`
	Methods     []*Function  `json:"methods,omitempty"`
	Attributes  []*Attribute `json:"attributes,omitempty"`
	Decorators  []string     `json:"decorators,omitempty"`
	Classes     []*Class     `json:"classes,omitempty"`
	Constructor *Function    `json:"constructor,omitempty"`
	// Origin is the dotted module that defines this class, set on a re-export.
	Origin string `json:"origin,omitempty"`
	// OriginName is the name in Origin, set if the re-export renames the class.
	OriginName string `json:"-"`
}

// Parameter represents a function parameter.
type Parameter struct {
	Name     string    `json:"name"`
	Kind     ParamKind `json:"kind"`
	Type     *TypeExpr `json:"type,omitempty"`
	Default  string    `json:"default,omitempty"`
	DocParam *DocParam `json:"doc,omitempty"`
}

// Attribute represents a class or module attribute.
type Attribute struct {
	Name       string     `json:"name"`
	Type       *TypeExpr  `json:"type,omitempty"`
	Value      string     `json:"value,omitempty"`
	Docstring  *Docstring `json:"docstring,omitempty"`
	IsProperty bool       `json:"is_property"`
	// Writable is true for a property that has a setter.
	Writable bool `json:"writable,omitempty"`
	// Instance is true for an attribute assigned only in __init__. Its Value is
	// the assigned expression, not a default.
	Instance bool `json:"instance,omitempty"`
	// Origin is the dotted module that defines this attribute, set on a re-export.
	Origin string `json:"origin,omitempty"`
	// StubTypeParam is true for "T = TypeVar(...)" in a .pyi. It is not API, so it
	// is public only if a __all__ lists it.
	StubTypeParam bool `json:"-"`
}

// TypeAlias represents a type alias (type X = ...).
type TypeAlias struct {
	Name  string    `json:"name"`
	Value *TypeExpr `json:"value"`
	// Origin is the dotted module that defines this alias, set on a re-export.
	Origin string `json:"origin,omitempty"`
}

// TypeRef represents a reference to a type (for imports, bases, etc).
type TypeRef struct {
	Name string    `json:"name"`
	Type *TypeExpr `json:"type,omitempty"`
}

// Docstring represents parsed docstring information.
type Docstring struct {
	Text    string         `json:"text"`
	Style   DocstringStyle `json:"style,omitempty"`
	Params  []*DocParam    `json:"params,omitempty"`
	Returns *DocReturn     `json:"returns,omitempty"`
	Yields  *DocReturn     `json:"yields,omitempty"`
	Raises  []*DocRaises   `json:"raises,omitempty"`
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
