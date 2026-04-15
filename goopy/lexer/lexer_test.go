package lexer_test

import (
	"testing"

	"github.com/pypx/goopy/lexer"
	"github.com/pypx/goopy/token"
)

// collectTokens drains the lexer into a slice, stopping after EOF.
func collectTokens(l *lexer.Lexer) []token.Token {
	var tokens []token.Token
	for {
		t := l.Next()
		tokens = append(tokens, t)
		if t.Type == token.EOF {
			break
		}
	}
	return tokens
}

// types returns just the token types from a slice.
func types(tokens []token.Token) []token.Type {
	ts := make([]token.Type, len(tokens))
	for i, t := range tokens {
		ts[i] = t.Type
	}
	return ts
}

// lits returns a slice of Lit fields.
func lits(tokens []token.Token) []string {
	ss := make([]string, len(tokens))
	for i, t := range tokens {
		ss[i] = t.Lit
	}
	return ss
}

// ---- Task 4: Basic lexer tests ----

func TestEmptyInput(t *testing.T) {
	l := lexer.New([]byte(""))
	toks := collectTokens(l)
	if len(toks) != 1 || toks[0].Type != token.EOF {
		t.Fatalf("expected [EOF], got %v", types(toks))
	}
}

func TestSimpleDefWithPass(t *testing.T) {
	src := `def foo():
    pass
`
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestKeywords(t *testing.T) {
	src := "def class import from as if elif else async return raise pass None True False"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.DEF, token.CLASS, token.IMPORT, token.FROM, token.AS,
		token.IF, token.ELIF, token.ELSE, token.ASYNC, token.RETURN,
		token.RAISE, token.PASS, token.NONE, token.TRUE, token.FALSE,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestOperators(t *testing.T) {
	src := "-> := ** // == != <= >= << >> ..."
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.ARROW, token.WALRUS, token.DSTAR, token.DSLASH,
		token.EQ, token.NEQ, token.LTE, token.GTE, token.LSHIFT, token.RSHIFT,
		token.ELLIPSIS,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestSingleCharOperators(t *testing.T) {
	src := ": = | * . / @ , ; + - ~ & ^ %"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.COLON, token.ASSIGN, token.PIPE, token.STAR, token.DOT,
		token.SLASH, token.AT, token.COMMA, token.SEMI, token.PLUS,
		token.MINUS, token.TILDE, token.AMPER, token.CARET, token.PERCENT,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestEllipsis(t *testing.T) {
	l := lexer.New([]byte("..."))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.ELLIPSIS || toks[1].Type != token.EOF {
		t.Fatalf("expected [ELLIPSIS EOF], got %v", types(toks))
	}
	if toks[0].Lit != "..." {
		t.Fatalf("expected lit '...', got %q", toks[0].Lit)
	}
}

func TestNumbers(t *testing.T) {
	cases := []struct {
		src string
	}{
		{"42"},
		{"3.14"},
		{"1_000_000"},
		{"0xFF"},
		{"0o77"},
		{"0b1010"},
		{"1e10"},
		{"1.5e-3"},
		{"2j"},
		{"1.0J"},
	}
	for _, c := range cases {
		t.Run(c.src, func(t *testing.T) {
			l := lexer.New([]byte(c.src))
			toks := collectTokens(l)
			if len(toks) != 2 || toks[0].Type != token.NUMBER {
				t.Fatalf("src %q: expected [NUMBER EOF], got %v", c.src, types(toks))
			}
			if toks[0].Lit != c.src {
				t.Fatalf("src %q: expected lit %q, got %q", c.src, c.src, toks[0].Lit)
			}
		})
	}
}

func TestCommentDiscarded(t *testing.T) {
	src := "x = 1  # this is a comment\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{token.NAME, token.ASSIGN, token.NUMBER, token.NEWLINE, token.EOF}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestStarArgsSyntax(t *testing.T) {
	src := "def f(*args, **kwargs): pass\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.DEF, token.NAME, token.LPAREN,
		token.STAR, token.NAME, token.COMMA,
		token.DSTAR, token.NAME,
		token.RPAREN, token.COLON, token.PASS, token.NEWLINE,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestDecoratorAndDef(t *testing.T) {
	src := "@decorator\ndef foo(): pass\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.AT, token.NAME, token.NEWLINE,
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.PASS, token.NEWLINE,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestBasicIndentation(t *testing.T) {
	src := "def f():\n    return 1\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT,
		token.RETURN, token.NUMBER, token.NEWLINE,
		token.DEDENT,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

// ---- Task 5: Indentation edge cases ----

func TestNestedIndentation(t *testing.T) {
	src := "if a:\n    if b:\n        pass\n    x = 1\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.IF, token.NAME, token.COLON, token.NEWLINE,
		token.INDENT,
		token.IF, token.NAME, token.COLON, token.NEWLINE,
		token.INDENT,
		token.PASS, token.NEWLINE,
		token.DEDENT,
		token.NAME, token.ASSIGN, token.NUMBER, token.NEWLINE,
		token.DEDENT,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestMultipleDedent(t *testing.T) {
	src := "if a:\n    if b:\n        pass\nx = 1\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.IF, token.NAME, token.COLON, token.NEWLINE,
		token.INDENT,
		token.IF, token.NAME, token.COLON, token.NEWLINE,
		token.INDENT,
		token.PASS, token.NEWLINE,
		token.DEDENT, token.DEDENT,
		token.NAME, token.ASSIGN, token.NUMBER, token.NEWLINE,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestBlankLinesIgnored(t *testing.T) {
	src := "def f():\n\n    pass\n\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestCommentOnlyLinesIgnored(t *testing.T) {
	src := "def f():\n    # a comment\n    pass\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS, token.NEWLINE,
		token.DEDENT,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestBracketsImplicitContinuation(t *testing.T) {
	src := "x = (\n    1,\n    2\n)\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	// No INDENT/DEDENT/NEWLINE inside parens.
	want := []token.Type{
		token.NAME, token.ASSIGN, token.LPAREN,
		token.NUMBER, token.COMMA,
		token.NUMBER,
		token.RPAREN, token.NEWLINE,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestExplicitLineContinuation(t *testing.T) {
	src := "x = 1 +\\\n    2\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.NAME, token.ASSIGN, token.NUMBER, token.PLUS, token.NUMBER, token.NEWLINE,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

func TestEOFWithoutTrailingNewline(t *testing.T) {
	src := "def f():\n    pass"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.DEF, token.NAME, token.LPAREN, token.RPAREN, token.COLON, token.NEWLINE,
		token.INDENT, token.PASS,
		token.DEDENT,
		token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
}

// ---- Task 6: String literal edge cases ----

func TestTripleQuotedString(t *testing.T) {
	src := "\"\"\"hello\nworld\"\"\""
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.STRING {
		t.Fatalf("expected [STRING EOF], got %v", types(toks))
	}
	if toks[0].Lit != src {
		t.Fatalf("expected lit %q, got %q", src, toks[0].Lit)
	}
}

func TestTripleSingleQuotedString(t *testing.T) {
	src := "'''hello\nworld'''"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.STRING {
		t.Fatalf("expected [STRING EOF], got %v", types(toks))
	}
}

func TestSingleQuotedString(t *testing.T) {
	src := "'hello'"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.STRING {
		t.Fatalf("expected [STRING EOF], got %v", types(toks))
	}
	if toks[0].Lit != src {
		t.Fatalf("expected lit %q, got %q", src, toks[0].Lit)
	}
}

func TestRawString(t *testing.T) {
	src := `r"hello\nworld"`
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.STRING {
		t.Fatalf("expected [STRING EOF], got %v", types(toks))
	}
	if toks[0].Lit != src {
		t.Fatalf("expected lit %q, got %q", src, toks[0].Lit)
	}
}

func TestFString(t *testing.T) {
	src := `f"hello {name}"`
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.STRING {
		t.Fatalf("expected [STRING EOF], got %v", types(toks))
	}
	if toks[0].Lit != src {
		t.Fatalf("expected lit %q, got %q", src, toks[0].Lit)
	}
}

func TestFStringNestedBraces(t *testing.T) {
	// The f-string contains a subscript with a string key — the inner } should not
	// terminate the f-string.
	src := `f"result: {d['key']}"`
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.STRING {
		t.Fatalf("expected ONE STRING token + EOF, got %v", types(toks))
	}
	if toks[0].Lit != src {
		t.Fatalf("expected lit %q, got %q", src, toks[0].Lit)
	}
}

func TestByteString(t *testing.T) {
	src := `b"hello"`
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.STRING {
		t.Fatalf("expected [STRING EOF], got %v", types(toks))
	}
	if toks[0].Lit != src {
		t.Fatalf("expected lit %q, got %q", src, toks[0].Lit)
	}
}

func TestEscapedQuoteInString(t *testing.T) {
	src := `"hello\"world"`
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	if len(toks) != 2 || toks[0].Type != token.STRING {
		t.Fatalf("expected [STRING EOF], got %v", types(toks))
	}
	if toks[0].Lit != src {
		t.Fatalf("expected lit %q, got %q", src, toks[0].Lit)
	}
}

func TestStringPrefixCombinations(t *testing.T) {
	cases := []string{
		`rb"bytes"`,
		`br"bytes"`,
		`fr"raw fstring"`,
		`rf"raw fstring"`,
		`u"unicode"`,
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			l := lexer.New([]byte(src))
			toks := collectTokens(l)
			if len(toks) != 2 || toks[0].Type != token.STRING {
				t.Fatalf("src %q: expected [STRING EOF], got %v", src, types(toks))
			}
		})
	}
}

// ---- Position tests ----

func TestTokenPositions(t *testing.T) {
	src := "def foo"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	// def: line 1, col 0
	if toks[0].Pos.Line != 1 || toks[0].Pos.Col != 0 {
		t.Errorf("def: want line=1 col=0, got line=%d col=%d", toks[0].Pos.Line, toks[0].Pos.Col)
	}
	// foo: line 1, col 4
	if toks[1].Pos.Line != 1 || toks[1].Pos.Col != 4 {
		t.Errorf("foo: want line=1 col=4, got line=%d col=%d", toks[1].Pos.Line, toks[1].Pos.Col)
	}
}

func TestMultilinePosition(t *testing.T) {
	src := "x = 1\ny = 2\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	// y should be on line 2.
	var yTok token.Token
	for _, t2 := range toks {
		if t2.Lit == "y" {
			yTok = t2
			break
		}
	}
	if yTok.Pos.Line != 2 {
		t.Errorf("y: want line=2, got line=%d", yTok.Pos.Line)
	}
}

// ---- Literal content tests ----

func TestIdentifierLiteral(t *testing.T) {
	l := lexer.New([]byte("hello_world"))
	toks := collectTokens(l)
	if toks[0].Lit != "hello_world" {
		t.Errorf("expected lit 'hello_world', got %q", toks[0].Lit)
	}
}

func TestLits(t *testing.T) {
	_ = lits // used in helper, suppress unused warning
}

// ---- Augmented assignment operator tests ----

func TestAugmentedAssignPlusEq(t *testing.T) {
	src := "x += 1\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.NAME, token.PLUSEQ, token.NUMBER, token.NEWLINE, token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
	if toks[1].Lit != "+=" {
		t.Fatalf("expected lit '+=', got %q", toks[1].Lit)
	}
}

func TestAugmentedAssignDSlashEq(t *testing.T) {
	src := "x //= 2\n"
	l := lexer.New([]byte(src))
	toks := collectTokens(l)
	want := []token.Type{
		token.NAME, token.DSLASHEQ, token.NUMBER, token.NEWLINE, token.EOF,
	}
	if !equalTypes(types(toks), want) {
		t.Fatalf("expected %v\ngot      %v", want, types(toks))
	}
	if toks[1].Lit != "//=" {
		t.Fatalf("expected lit '//=', got %q", toks[1].Lit)
	}
}

func TestAugmentedAssignAllOperators(t *testing.T) {
	cases := []struct {
		src  string
		typ  token.Type
		lit  string
	}{
		{"x += 1", token.PLUSEQ, "+="},
		{"x -= 1", token.MINUSEQ, "-="},
		{"x *= 1", token.STAREQ, "*="},
		{"x /= 1", token.SLASHEQ, "/="},
		{"x //= 1", token.DSLASHEQ, "//="},
		{"x %= 1", token.PERCENTEQ, "%="},
		{"x **= 1", token.DSTAREQ, "**="},
		{"x &= 1", token.AMPEREQ, "&="},
		{"x |= 1", token.PIPEEQ, "|="},
		{"x ^= 1", token.CARETEQ, "^="},
		{"x >>= 1", token.RSHIFTEQ, ">>="},
		{"x <<= 1", token.LSHIFTEQ, "<<="},
	}
	for _, c := range cases {
		t.Run(c.lit, func(t *testing.T) {
			l := lexer.New([]byte(c.src))
			toks := collectTokens(l)
			// toks[0]=NAME, toks[1]=op, toks[2]=NUMBER, ...
			if len(toks) < 3 {
				t.Fatalf("expected at least 3 tokens, got %d: %v", len(toks), types(toks))
			}
			if toks[1].Type != c.typ {
				t.Errorf("expected token type %v, got %v", c.typ, toks[1].Type)
			}
			if toks[1].Lit != c.lit {
				t.Errorf("expected lit %q, got %q", c.lit, toks[1].Lit)
			}
		})
	}
}

// ---- Helper ----

func equalTypes(a, b []token.Type) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
