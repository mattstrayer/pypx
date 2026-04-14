package token

//go:generate stringer -type=Type

type Type int

const (
	// Special tokens
	EOF Type = iota
	ILLEGAL
	NEWLINE
	INDENT
	DEDENT

	// Literals
	NAME
	NUMBER
	STRING

	// Keywords
	DEF
	CLASS
	IMPORT
	FROM
	AS
	IF
	ELIF
	ELSE
	ASYNC
	RETURN
	RAISE
	PASS
	NONE
	TRUE
	FALSE
	TYPE
	FOR
	WHILE
	TRY
	EXCEPT
	FINALLY
	WITH
	MATCH
	CASE
	YIELD
	DEL
	GLOBAL
	NONLOCAL
	ASSERT
	LAMBDA
	AND
	OR
	NOT
	IN
	IS

	// Operators
	ARROW
	COLON
	ASSIGN
	PIPE
	STAR
	DSTAR
	DOT
	ELLIPSIS
	SLASH
	AT
	COMMA
	SEMI
	PLUS
	MINUS
	WALRUS
	EQ
	NEQ
	LT
	GT
	LTE
	GTE
	TILDE
	AMPER
	CARET
	PERCENT
	LSHIFT
	RSHIFT
	DSLASH

	// Delimiters
	LPAREN
	RPAREN
	LBRACK
	RBRACK
	LBRACE
	RBRACE
)

// keywords maps string identifiers to their token types
var keywords = map[string]Type{
	"def":      DEF,
	"class":    CLASS,
	"import":   IMPORT,
	"from":     FROM,
	"as":       AS,
	"if":       IF,
	"elif":     ELIF,
	"else":     ELSE,
	"async":    ASYNC,
	"return":   RETURN,
	"raise":    RAISE,
	"pass":     PASS,
	"None":     NONE,
	"True":     TRUE,
	"False":    FALSE,
	"type":     TYPE,
	"for":      FOR,
	"while":    WHILE,
	"try":      TRY,
	"except":   EXCEPT,
	"finally":  FINALLY,
	"with":     WITH,
	"match":    MATCH,
	"case":     CASE,
	"yield":    YIELD,
	"del":      DEL,
	"global":   GLOBAL,
	"nonlocal": NONLOCAL,
	"assert":   ASSERT,
	"lambda":   LAMBDA,
	"and":      AND,
	"or":       OR,
	"not":      NOT,
	"in":       IN,
	"is":       IS,
}

// LookupKeyword returns the token type for a keyword, or NAME if not a keyword.
// The second return value is true if the identifier is a keyword.
func LookupKeyword(ident string) (Type, bool) {
	if typ, ok := keywords[ident]; ok {
		return typ, true
	}
	return NAME, false
}

// Pos represents a position in the source code.
type Pos struct {
	Line   int // 1-based line number
	Col    int // 0-based column number
	Offset int // 0-based byte offset
}

// Token represents a single token in the source code.
type Token struct {
	Type Type
	Lit  string
	Pos  Pos
}
