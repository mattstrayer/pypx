package token

import (
	"testing"
)

func TestTypeString(t *testing.T) {
	tests := []struct {
		typ      Type
		expected string
	}{
		{EOF, "EOF"},
		{ILLEGAL, "ILLEGAL"},
		{NEWLINE, "NEWLINE"},
		{INDENT, "INDENT"},
		{DEDENT, "DEDENT"},
		{NAME, "NAME"},
		{NUMBER, "NUMBER"},
		{STRING, "STRING"},
		{DEF, "DEF"},
		{CLASS, "CLASS"},
		{IMPORT, "IMPORT"},
		{FROM, "FROM"},
		{AS, "AS"},
		{IF, "IF"},
		{ELIF, "ELIF"},
		{ELSE, "ELSE"},
		{ASYNC, "ASYNC"},
		{RETURN, "RETURN"},
		{RAISE, "RAISE"},
		{PASS, "PASS"},
		{NONE, "NONE"},
		{TRUE, "TRUE"},
		{FALSE, "FALSE"},
		{TYPE, "TYPE"},
		{FOR, "FOR"},
		{WHILE, "WHILE"},
		{TRY, "TRY"},
		{EXCEPT, "EXCEPT"},
		{FINALLY, "FINALLY"},
		{WITH, "WITH"},
		{MATCH, "MATCH"},
		{CASE, "CASE"},
		{YIELD, "YIELD"},
		{DEL, "DEL"},
		{GLOBAL, "GLOBAL"},
		{NONLOCAL, "NONLOCAL"},
		{ASSERT, "ASSERT"},
		{LAMBDA, "LAMBDA"},
		{AND, "AND"},
		{OR, "OR"},
		{NOT, "NOT"},
		{IN, "IN"},
		{IS, "IS"},
		{ARROW, "ARROW"},
		{COLON, "COLON"},
		{ASSIGN, "ASSIGN"},
		{PIPE, "PIPE"},
		{STAR, "STAR"},
		{DSTAR, "DSTAR"},
		{DOT, "DOT"},
		{ELLIPSIS, "ELLIPSIS"},
		{SLASH, "SLASH"},
		{AT, "AT"},
		{COMMA, "COMMA"},
		{SEMI, "SEMI"},
		{PLUS, "PLUS"},
		{MINUS, "MINUS"},
		{WALRUS, "WALRUS"},
		{EQ, "EQ"},
		{NEQ, "NEQ"},
		{LT, "LT"},
		{GT, "GT"},
		{LTE, "LTE"},
		{GTE, "GTE"},
		{TILDE, "TILDE"},
		{AMPER, "AMPER"},
		{CARET, "CARET"},
		{PERCENT, "PERCENT"},
		{LSHIFT, "LSHIFT"},
		{RSHIFT, "RSHIFT"},
		{DSLASH, "DSLASH"},
		{LPAREN, "LPAREN"},
		{RPAREN, "RPAREN"},
		{LBRACK, "LBRACK"},
		{RBRACK, "RBRACK"},
		{LBRACE, "LBRACE"},
		{RBRACE, "RBRACE"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.typ.String()
			if got != tt.expected {
				t.Errorf("Type(%d).String() = %q, want %q", tt.typ, got, tt.expected)
			}
		})
	}
}

func TestLookupKeyword(t *testing.T) {
	tests := []struct {
		ident     string
		typ       Type
		isKeyword bool
	}{
		// Keywords
		{"def", DEF, true},
		{"class", CLASS, true},
		{"import", IMPORT, true},
		{"from", FROM, true},
		{"as", AS, true},
		{"if", IF, true},
		{"elif", ELIF, true},
		{"else", ELSE, true},
		{"async", ASYNC, true},
		{"return", RETURN, true},
		{"raise", RAISE, true},
		{"pass", PASS, true},
		{"None", NONE, true},
		{"True", TRUE, true},
		{"False", FALSE, true},
		{"type", TYPE, true},
		{"for", FOR, true},
		{"while", WHILE, true},
		{"try", TRY, true},
		{"except", EXCEPT, true},
		{"finally", FINALLY, true},
		{"with", WITH, true},
		{"match", MATCH, true},
		{"case", CASE, true},
		{"yield", YIELD, true},
		{"del", DEL, true},
		{"global", GLOBAL, true},
		{"nonlocal", NONLOCAL, true},
		{"assert", ASSERT, true},
		{"lambda", LAMBDA, true},
		{"and", AND, true},
		{"or", OR, true},
		{"not", NOT, true},
		{"in", IN, true},
		{"is", IS, true},

		// Non-keywords
		{"foo", NAME, false},
		{"bar", NAME, false},
		{"mydef", NAME, false},
		{"defx", NAME, false},
		{"myclass", NAME, false},
		{"", NAME, false},
	}

	for _, tt := range tests {
		t.Run(tt.ident, func(t *testing.T) {
			typ, isKeyword := LookupKeyword(tt.ident)
			if typ != tt.typ {
				t.Errorf("LookupKeyword(%q) type = %v, want %v", tt.ident, typ, tt.typ)
			}
			if isKeyword != tt.isKeyword {
				t.Errorf("LookupKeyword(%q) isKeyword = %v, want %v", tt.ident, isKeyword, tt.isKeyword)
			}
		})
	}
}

func TestToken(t *testing.T) {
	tok := Token{
		Type: NAME,
		Lit:  "foo",
		Pos: Pos{
			Line:   1,
			Col:    0,
			Offset: 0,
		},
	}

	if tok.Type != NAME {
		t.Errorf("Token.Type = %v, want %v", tok.Type, NAME)
	}

	if tok.Lit != "foo" {
		t.Errorf("Token.Lit = %q, want %q", tok.Lit, "foo")
	}

	if tok.Pos.Line != 1 {
		t.Errorf("Token.Pos.Line = %d, want 1", tok.Pos.Line)
	}

	if tok.Pos.Col != 0 {
		t.Errorf("Token.Pos.Col = %d, want 0", tok.Pos.Col)
	}

	if tok.Pos.Offset != 0 {
		t.Errorf("Token.Pos.Offset = %d, want 0", tok.Pos.Offset)
	}
}

func TestPos(t *testing.T) {
	pos := Pos{
		Line:   5,
		Col:    10,
		Offset: 42,
	}

	if pos.Line != 5 {
		t.Errorf("Pos.Line = %d, want 5", pos.Line)
	}

	if pos.Col != 10 {
		t.Errorf("Pos.Col = %d, want 10", pos.Col)
	}

	if pos.Offset != 42 {
		t.Errorf("Pos.Offset = %d, want 42", pos.Offset)
	}
}
