package model

// TypeExprKind is the kind of a type expression.
type TypeExprKind string

const (
	TypeExprName      TypeExprKind = "name"
	TypeExprGeneric   TypeExprKind = "generic"
	TypeExprUnion     TypeExprKind = "union"
	TypeExprOptional  TypeExprKind = "optional"
	TypeExprCallable  TypeExprKind = "callable"
	TypeExprTuple     TypeExprKind = "tuple"
	TypeExprLiteral   TypeExprKind = "literal"
	TypeExprNone      TypeExprKind = "none"
	TypeExprEllipsis  TypeExprKind = "ellipsis"
	TypeExprUnpack TypeExprKind = "unpack"
)

// TypeExpr represents a type expression.
// It can represent simple names, generics, unions, callables, etc.
type TypeExpr struct {
	// Kind indicates the kind of type expression
	Kind TypeExprKind `json:"kind"`

	// Name is used for simple names (e.g., "int", "str") and generics (e.g., "List" in List[int])
	Name string `json:"name,omitempty"`

	// Args is used for generic types (e.g., [int] in List[int]) or callable arguments
	Args []*TypeExpr `json:"args,omitempty"`

	// Elements is used for unions (int | str) or tuples (Tuple[int, str, ...])
	Elements []*TypeExpr `json:"elements,omitempty"`

	// Returns is used for callable types (Callable[[int], str])
	Returns *TypeExpr `json:"returns,omitempty"`

	// Value is used for literal types
	Value any `json:"value,omitempty"`

	// Raw is the raw source text of the type expression (always preserved)
	Raw string `json:"raw"`
}
