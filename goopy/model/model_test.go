package model

import (
	"encoding/json"
	"testing"
)

func TestParamKindValues(t *testing.T) {
	tests := []struct {
		kind ParamKind
		str  string
	}{
		{ParamPositionalOnly, "positional_only"},
		{ParamPositionalOrKeyword, "positional_or_keyword"},
		{ParamVarPositional, "var_positional"},
		{ParamKeywordOnly, "keyword_only"},
		{ParamVarKeyword, "var_keyword"},
	}

	for _, tt := range tests {
		if string(tt.kind) != tt.str {
			t.Errorf("ParamKind value = %q, want %q", string(tt.kind), tt.str)
		}
	}

	// Verify all are non-empty
	for _, tt := range tests {
		if string(tt.kind) == "" {
			t.Errorf("ParamKind %v is empty", tt)
		}
	}

	// Verify uniqueness
	seen := make(map[string]bool)
	for _, tt := range tests {
		if seen[tt.str] {
			t.Errorf("ParamKind value %q is not unique", tt.str)
		}
		seen[tt.str] = true
	}
}

func TestDocstringStyleValues(t *testing.T) {
	tests := []struct {
		style DocstringStyle
		str   string
	}{
		{DocstringGoogle, "google"},
		{DocstringNumpy, "numpy"},
		{DocstringSphinx, "sphinx"},
		{DocstringPlain, "plain"},
	}

	for _, tt := range tests {
		if string(tt.style) != tt.str {
			t.Errorf("DocstringStyle value = %q, want %q", string(tt.style), tt.str)
		}
	}

	// Verify all are non-empty
	for _, tt := range tests {
		if string(tt.style) == "" {
			t.Errorf("DocstringStyle %v is empty", tt)
		}
	}

	// Verify uniqueness
	seen := make(map[string]bool)
	for _, tt := range tests {
		if seen[tt.str] {
			t.Errorf("DocstringStyle value %q is not unique", tt.str)
		}
		seen[tt.str] = true
	}
}

func TestParameter(t *testing.T) {
	param := &Parameter{
		Name: "x",
		Kind: ParamPositionalOrKeyword,
		Type: &TypeExpr{
			Kind: TypeExprName,
			Name: "int",
			Raw:  "int",
		},
		Default: "0",
	}

	if param.Name != "x" {
		t.Errorf("Parameter.Name = %q, want %q", param.Name, "x")
	}

	if param.Kind != ParamPositionalOrKeyword {
		t.Errorf("Parameter.Kind = %q, want %q", param.Kind, ParamPositionalOrKeyword)
	}

	if param.Type.Kind != TypeExprName {
		t.Errorf("Parameter.Type.Kind = %v, want %v", param.Type.Kind, TypeExprName)
	}
}

func TestAttribute(t *testing.T) {
	attr := &Attribute{
		Name: "value",
		Type: &TypeExpr{
			Kind: TypeExprName,
			Name: "str",
			Raw:  "str",
		},
		Docstring: &Docstring{
			Text:  "A value attribute",
			Style: DocstringPlain,
		},
	}

	if attr.Name != "value" {
		t.Errorf("Attribute.Name = %q, want %q", attr.Name, "value")
	}

	if attr.Type.Kind != TypeExprName {
		t.Errorf("Attribute.Type.Kind = %v, want %v", attr.Type.Kind, TypeExprName)
	}

	if attr.Docstring.Text != "A value attribute" {
		t.Errorf("Attribute.Docstring.Text = %q", attr.Docstring.Text)
	}
}

func TestTypeAlias(t *testing.T) {
	alias := &TypeAlias{
		Name: "MyType",
		Value: &TypeExpr{
			Kind: TypeExprUnion,
			Raw:  "int | str",
		},
	}

	if alias.Name != "MyType" {
		t.Errorf("TypeAlias.Name = %q, want %q", alias.Name, "MyType")
	}

	if alias.Value.Kind != TypeExprUnion {
		t.Errorf("TypeAlias.Value.Kind = %v, want %v", alias.Value.Kind, TypeExprUnion)
	}
}

func TestTypeExprJSON(t *testing.T) {
	tests := []struct {
		name string
		expr *TypeExpr
	}{
		{
			name: "simple name",
			expr: &TypeExpr{
				Kind: TypeExprName,
				Name: "int",
				Raw:  "int",
			},
		},
		{
			name: "generic with args",
			expr: &TypeExpr{
				Kind: TypeExprGeneric,
				Name: "List",
				Args: []*TypeExpr{
					{
						Kind: TypeExprName,
						Name: "int",
						Raw:  "int",
					},
				},
				Raw: "List[int]",
			},
		},
		{
			name: "union",
			expr: &TypeExpr{
				Kind: TypeExprUnion,
				Elements: []*TypeExpr{
					{Kind: TypeExprName, Name: "int", Raw: "int"},
					{Kind: TypeExprName, Name: "str", Raw: "str"},
				},
				Raw: "int | str",
			},
		},
		{
			name: "optional",
			expr: &TypeExpr{
				Kind: TypeExprOptional,
				Args: []*TypeExpr{
					{Kind: TypeExprName, Name: "int", Raw: "int"},
				},
				Raw: "Optional[int]",
			},
		},
		{
			name: "callable",
			expr: &TypeExpr{
				Kind: TypeExprCallable,
				Args: []*TypeExpr{
					{Kind: TypeExprName, Name: "int", Raw: "int"},
				},
				Returns: &TypeExpr{Kind: TypeExprName, Name: "str", Raw: "str"},
				Raw:     "Callable[[int], str]",
			},
		},
		{
			name: "tuple",
			expr: &TypeExpr{
				Kind: TypeExprTuple,
				Elements: []*TypeExpr{
					{Kind: TypeExprName, Name: "int", Raw: "int"},
					{Kind: TypeExprEllipsis, Raw: "..."},
				},
				Raw: "Tuple[int, ...]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.expr)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			// Unmarshal back
			var unmarshaled TypeExpr
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}

			// Verify round-trip
			if unmarshaled.Kind != tt.expr.Kind {
				t.Errorf("Round-trip: Kind = %v, want %v", unmarshaled.Kind, tt.expr.Kind)
			}

			if unmarshaled.Name != tt.expr.Name {
				t.Errorf("Round-trip: Name = %q, want %q", unmarshaled.Name, tt.expr.Name)
			}

			if unmarshaled.Raw != tt.expr.Raw {
				t.Errorf("Round-trip: Raw = %q, want %q", unmarshaled.Raw, tt.expr.Raw)
			}
		})
	}
}

func TestModuleJSON(t *testing.T) {
	module := &Module{
		Name: "mymodule",
		Docstring: &Docstring{
			Text:  "A test module",
			Style: DocstringGoogle,
		},
		Functions: []*Function{
			{
				Name: "foo",
				Parameters: []*Parameter{
					{
						Name: "x",
						Kind: ParamPositionalOrKeyword,
						Type: &TypeExpr{
							Kind: TypeExprName,
							Name: "int",
							Raw:  "int",
						},
					},
				},
				Returns: &TypeExpr{
					Kind: TypeExprName,
					Name: "int",
					Raw:  "int",
				},
				Docstring: &Docstring{
					Text:  "Does something",
					Style: DocstringGoogle,
				},
			},
		},
		Classes: []*Class{
			{
				Name: "MyClass",
				Attributes: []*Attribute{
					{
						Name: "value",
						Type: &TypeExpr{
							Kind: TypeExprName,
							Name: "str",
							Raw:  "str",
						},
					},
				},
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(module)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Unmarshal back
	var unmarshaled Module
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify round-trip
	if unmarshaled.Name != module.Name {
		t.Errorf("Round-trip: Name = %q, want %q", unmarshaled.Name, module.Name)
	}

	if len(unmarshaled.Functions) != 1 {
		t.Errorf("Round-trip: Functions len = %d, want 1", len(unmarshaled.Functions))
	}

	if len(unmarshaled.Classes) != 1 {
		t.Errorf("Round-trip: Classes len = %d, want 1", len(unmarshaled.Classes))
	}

	if unmarshaled.Functions[0].Name != "foo" {
		t.Errorf("Round-trip: Function name = %q, want %q", unmarshaled.Functions[0].Name, "foo")
	}

	if unmarshaled.Classes[0].Name != "MyClass" {
		t.Errorf("Round-trip: Class name = %q, want %q", unmarshaled.Classes[0].Name, "MyClass")
	}
}

func TestTypeExprKindValues(t *testing.T) {
	tests := []struct {
		kind TypeExprKind
		str  string
	}{
		{TypeExprName, "name"},
		{TypeExprGeneric, "generic"},
		{TypeExprUnion, "union"},
		{TypeExprOptional, "optional"},
		{TypeExprCallable, "callable"},
		{TypeExprTuple, "tuple"},
		{TypeExprLiteral, "literal"},
		{TypeExprNone, "none"},
		{TypeExprEllipsis, "ellipsis"},
		{TypeExprUnpack, "unpack"},
	}

	for _, tt := range tests {
		if string(tt.kind) != tt.str {
			t.Errorf("TypeExprKind value = %q, want %q", string(tt.kind), tt.str)
		}
	}

	// Verify all are non-empty
	for _, tt := range tests {
		if string(tt.kind) == "" {
			t.Errorf("TypeExprKind %v is empty", tt)
		}
	}

	// Verify uniqueness
	seen := make(map[string]bool)
	for _, tt := range tests {
		if seen[tt.str] {
			t.Errorf("TypeExprKind value %q is not unique", tt.str)
		}
		seen[tt.str] = true
	}
}

func TestDocParam(t *testing.T) {
	param := &DocParam{
		Name:        "x",
		Type:        "int",
		Description: "The input value",
	}

	if param.Name != "x" {
		t.Errorf("DocParam.Name = %q, want %q", param.Name, "x")
	}

	if param.Type != "int" {
		t.Errorf("DocParam.Type = %q, want %q", param.Type, "int")
	}

	if param.Description != "The input value" {
		t.Errorf("DocParam.Description = %q", param.Description)
	}
}

func TestDocReturn(t *testing.T) {
	docReturn := &DocReturn{
		Type:        "str",
		Description: "The output",
	}

	if docReturn.Type != "str" {
		t.Errorf("DocReturn.Type = %q, want %q", docReturn.Type, "str")
	}

	if docReturn.Description != "The output" {
		t.Errorf("DocReturn.Description = %q", docReturn.Description)
	}
}

func TestDocRaises(t *testing.T) {
	raises := &DocRaises{
		Type:        "ValueError",
		Description: "When value is invalid",
	}

	if raises.Type != "ValueError" {
		t.Errorf("DocRaises.Type = %q, want %q", raises.Type, "ValueError")
	}

	if raises.Description != "When value is invalid" {
		t.Errorf("DocRaises.Description = %q", raises.Description)
	}
}
