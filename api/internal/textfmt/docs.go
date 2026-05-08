package textfmt

// DocsInput mirrors handler.DocsResponse. Defined here so textfmt does not
// need to import handler (avoids an import cycle). Fields use the same JSON
// names and shapes; the handler converts before calling these formatters.
type DocsInput struct {
	Package     string
	Version     string
	Available   bool
	StubPackage string
	Modules     []DocModuleInput
}

type DocModuleInput struct {
	Name       string
	Functions  []DocSymbolInput
	Classes    []DocSymbolInput
	Exceptions []DocSymbolInput
}

type DocSymbolInput struct {
	Name       string
	Kind       string // "function" | "class" | "method" | "exception"
	Signature  string
	Docstring  string
	Parameters []DocParamInput
	Returns    *DocReturnInput
	Raises     []DocRaiseInput
	Methods    []DocSymbolInput
}

type DocParamInput struct {
	Name        string
	Type        string
	Description string
	Kind        string
	Default     string
}

type DocReturnInput struct {
	Type        string
	Description string
}

type DocRaiseInput struct {
	Type        string
	Description string
}
