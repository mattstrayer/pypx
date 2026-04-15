// Package lexer implements a Python lexer (Pass 1 of the goopy two-pass parser).
// It converts Python source bytes into a flat token stream, handling Python's
// indentation-sensitive syntax by emitting synthetic INDENT/DEDENT tokens.
package lexer

import (
	"github.com/pypx/goopy/token"
)

// Lexer tokenises Python source code.
type Lexer struct {
	src          []byte
	pos          int
	line         int // 1-based
	col          int // 0-based
	tokens       []token.Token // pending DEDENT burst
	indentStack  []int
	bracketDepth int
	atLineStart  bool
}

// New returns a new Lexer for the given source bytes.
func New(src []byte) *Lexer {
	return &Lexer{
		src:         src,
		line:        1,
		col:         0,
		indentStack: []int{0},
		atLineStart: true,
	}
}

// Next returns the next token from the source.
func (l *Lexer) Next() token.Token {
	// Drain any buffered DEDENT tokens first.
	if len(l.tokens) > 0 {
		t := l.tokens[0]
		l.tokens = l.tokens[1:]
		return t
	}

	for {
		// Check EOF before indentation handling to avoid infinite loops.
		if l.pos >= len(l.src) {
			return l.eofTokens()
		}

		// Handle indentation at the start of a logical line (not inside brackets).
		if l.atLineStart && l.bracketDepth == 0 {
			// Peek ahead: if the line is blank or comment-only, consume it and continue.
			if l.isBlankOrCommentLine() {
				l.consumeBlankLine()
				continue
			}
			l.atLineStart = false
			// Measure indent of current line.
			indent, advanced := l.measureIndent()
			top := l.indentStack[len(l.indentStack)-1]
			if indent > top {
				// Push new indent level, emit INDENT.
				l.indentStack = append(l.indentStack, indent)
				pos := l.currentPos()
				l.advance(advanced)
				return token.Token{Type: token.INDENT, Lit: "", Pos: pos}
			} else if indent < top {
				// Pop levels and buffer DEDENT tokens.
				pos := l.currentPos()
				l.advance(advanced)
				for len(l.indentStack) > 1 && l.indentStack[len(l.indentStack)-1] > indent {
					l.indentStack = l.indentStack[:len(l.indentStack)-1]
					l.tokens = append(l.tokens, token.Token{Type: token.DEDENT, Lit: "", Pos: pos})
				}
				if len(l.tokens) > 0 {
					t := l.tokens[0]
					l.tokens = l.tokens[1:]
					return t
				}
			} else {
				// Same indent level — just skip the whitespace.
				l.advance(advanced)
			}
		}

		ch := l.src[l.pos]

		// Skip spaces and tabs (not at line start — those were handled above).
		if ch == ' ' || ch == '\t' {
			l.pos++
			l.col++
			continue
		}

		// Explicit line continuation.
		if ch == '\\' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
			l.pos += 2
			l.line++
			l.col = 0
			// atLineStart stays false — continuation means we're in the middle of a logical line
			continue
		}
		if ch == '\\' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\r' && l.pos+2 < len(l.src) && l.src[l.pos+2] == '\n' {
			l.pos += 3
			l.line++
			l.col = 0
			continue
		}

		// Newline.
		if ch == '\n' || ch == '\r' {
			pos := l.currentPos()
			if ch == '\r' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
				l.pos++
			}
			l.pos++
			l.line++
			l.col = 0
			if l.bracketDepth > 0 {
				// Implicit line continuation inside brackets — suppress NEWLINE.
				// We are at the start of the next physical line but still in the
				// middle of a logical line, so do NOT set atLineStart.
				l.atLineStart = false
				continue
			}
			l.atLineStart = true
			return token.Token{Type: token.NEWLINE, Lit: "\n", Pos: pos}
		}

		// Comment.
		if ch == '#' {
			for l.pos < len(l.src) && l.src[l.pos] != '\n' && l.src[l.pos] != '\r' {
				l.pos++
				l.col++
			}
			continue
		}

		// String literals (must be before identifier check so prefixes like r/b/f/u are handled).
		if l.isStringStart() {
			return l.lexString()
		}

		// Identifiers and keywords.
		if isIdentStart(ch) {
			return l.lexIdent()
		}

		// Numbers.
		if isDigit(ch) || (ch == '.' && l.pos+1 < len(l.src) && isDigit(l.src[l.pos+1])) {
			return l.lexNumber()
		}

		// Operators and delimiters.
		return l.lexOperator()
	}
}

// eofTokens emits DEDENT tokens for any remaining indent levels, then EOF.
func (l *Lexer) eofTokens() token.Token {
	pos := l.currentPos()
	// Pop all indent levels beyond the base.
	for len(l.indentStack) > 1 {
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
		l.tokens = append(l.tokens, token.Token{Type: token.DEDENT, Lit: "", Pos: pos})
	}
	l.tokens = append(l.tokens, token.Token{Type: token.EOF, Lit: "", Pos: pos})
	t := l.tokens[0]
	l.tokens = l.tokens[1:]
	return t
}

// isBlankOrCommentLine returns true if the current line (at l.pos) contains only
// whitespace and optionally a comment, followed by a newline or EOF.
func (l *Lexer) isBlankOrCommentLine() bool {
	i := l.pos
	for i < len(l.src) && (l.src[i] == ' ' || l.src[i] == '\t') {
		i++
	}
	if i >= len(l.src) {
		return true
	}
	return l.src[i] == '\n' || l.src[i] == '\r' || l.src[i] == '#'
}

// consumeBlankLine advances past a blank or comment-only line.
func (l *Lexer) consumeBlankLine() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' && l.src[l.pos] != '\r' {
		l.pos++
		l.col++
	}
	if l.pos < len(l.src) {
		if l.src[l.pos] == '\r' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
			l.pos++
		}
		l.pos++
		l.line++
		l.col = 0
	}
	// atLineStart stays true.
}

// measureIndent returns the indent level in spaces and how many bytes to advance.
// Tabs use Python's tab-stop rule: tab to the next multiple of 8.
func (l *Lexer) measureIndent() (spaces int, bytes int) {
	i := l.pos
	col := 0
	for i < len(l.src) {
		if l.src[i] == ' ' {
			col++
			i++
		} else if l.src[i] == '\t' {
			col += 8 - (col % 8)
			i++
		} else {
			break
		}
	}
	return col, i - l.pos
}

// advance moves l.pos forward by n bytes, updating col.
func (l *Lexer) advance(n int) {
	for i := 0; i < n; i++ {
		if l.pos >= len(l.src) {
			break
		}
		if l.src[l.pos] == '\t' {
			l.col += 8 - (l.col % 8)
		} else {
			l.col++
		}
		l.pos++
	}
}

// currentPos returns a Pos snapshot.
func (l *Lexer) currentPos() token.Pos {
	return token.Pos{Line: l.line, Col: l.col, Offset: l.pos}
}

// isStringStart returns true if the current position looks like the start of a string literal.
func (l *Lexer) isStringStart() bool {
	i := l.pos
	// Consume optional prefix chars.
	for i < len(l.src) && isStringPrefix(l.src[i]) {
		i++
	}
	return i < len(l.src) && (l.src[i] == '"' || l.src[i] == '\'')
}

func isStringPrefix(ch byte) bool {
	return ch == 'r' || ch == 'R' || ch == 'b' || ch == 'B' ||
		ch == 'f' || ch == 'F' || ch == 'u' || ch == 'U'
}

// lexString lexes a string literal (with optional prefix).
func (l *Lexer) lexString() token.Token {
	pos := l.currentPos()
	start := l.pos

	// Collect prefix.
	isFStr := false
	for l.pos < len(l.src) && isStringPrefix(l.src[l.pos]) {
		ch := l.src[l.pos]
		if ch == 'f' || ch == 'F' {
			isFStr = true
		}
		l.pos++
		l.col++
	}

	quote := l.src[l.pos]
	// Determine if triple-quoted.
	triple := false
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
		triple = true
		l.pos += 3
		l.col += 3
	} else {
		l.pos++
		l.col++
	}

	// Read string body.
	braceDepth := 0 // tracks { } inside f-string expressions
	for l.pos < len(l.src) {
		ch := l.src[l.pos]

		if ch == '\\' && l.pos+1 < len(l.src) {
			// Escape sequence — skip both chars (handle \n for line count).
			l.pos++
			l.col++
			next := l.src[l.pos]
			if next == '\n' {
				l.line++
				l.col = 0
			} else if next == '\r' {
				l.line++
				l.col = 0
				if l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
					l.pos++
				}
			} else {
				l.col++
			}
			l.pos++
			continue
		}

		if ch == '\n' || ch == '\r' {
			if !triple {
				// Unterminated single-line string — stop here.
				break
			}
			if ch == '\r' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
				l.pos++
			}
			l.pos++
			l.line++
			l.col = 0
			continue
		}

		// F-string brace tracking.
		if isFStr {
			if ch == '{' {
				// {{ is escaped brace, not an expression.
				if l.pos+1 < len(l.src) && l.src[l.pos+1] == '{' {
					l.pos += 2
					l.col += 2
					continue
				}
				braceDepth++
				l.pos++
				l.col++
				continue
			}
			if ch == '}' {
				if braceDepth > 0 {
					braceDepth--
					l.pos++
					l.col++
					continue
				}
				// }} is escaped closing brace.
				if l.pos+1 < len(l.src) && l.src[l.pos+1] == '}' {
					l.pos += 2
					l.col += 2
					continue
				}
				// Stray } outside expression — treat as literal char.
				l.pos++
				l.col++
				continue
			}
		}

		// Check for closing quote (only when not inside an f-string expression).
		if ch == quote && braceDepth == 0 {
			if triple {
				if l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
					l.pos += 3
					l.col += 3
					break
				}
			} else {
				l.pos++
				l.col++
				break
			}
		}

		if ch == '\t' {
			l.col += 8 - (l.col % 8)
		} else {
			l.col++
		}
		l.pos++
	}

	return token.Token{
		Type: token.STRING,
		Lit:  string(l.src[start:l.pos]),
		Pos:  pos,
	}
}

// lexIdent lexes an identifier or keyword.
func (l *Lexer) lexIdent() token.Token {
	pos := l.currentPos()
	start := l.pos
	for l.pos < len(l.src) && isIdentContinue(l.src[l.pos]) {
		l.pos++
		l.col++
	}
	lit := string(l.src[start:l.pos])
	typ, _ := token.LookupKeyword(lit)
	return token.Token{Type: typ, Lit: lit, Pos: pos}
}

// lexNumber lexes an integer or float literal.
func (l *Lexer) lexNumber() token.Token {
	pos := l.currentPos()
	start := l.pos

	ch := l.src[l.pos]

	if ch == '0' && l.pos+1 < len(l.src) {
		next := l.src[l.pos+1]
		if next == 'x' || next == 'X' {
			// Hex literal.
			l.pos += 2
			l.col += 2
			for l.pos < len(l.src) && isHexDigit(l.src[l.pos]) || (l.pos < len(l.src) && l.src[l.pos] == '_') {
				l.pos++
				l.col++
			}
			l.consumeComplexSuffix()
			return token.Token{Type: token.NUMBER, Lit: string(l.src[start:l.pos]), Pos: pos}
		}
		if next == 'o' || next == 'O' {
			// Octal literal.
			l.pos += 2
			l.col += 2
			for l.pos < len(l.src) && (l.src[l.pos] >= '0' && l.src[l.pos] <= '7' || l.src[l.pos] == '_') {
				l.pos++
				l.col++
			}
			l.consumeComplexSuffix()
			return token.Token{Type: token.NUMBER, Lit: string(l.src[start:l.pos]), Pos: pos}
		}
		if next == 'b' || next == 'B' {
			// Binary literal.
			l.pos += 2
			l.col += 2
			for l.pos < len(l.src) && (l.src[l.pos] == '0' || l.src[l.pos] == '1' || l.src[l.pos] == '_') {
				l.pos++
				l.col++
			}
			l.consumeComplexSuffix()
			return token.Token{Type: token.NUMBER, Lit: string(l.src[start:l.pos]), Pos: pos}
		}
	}

	// Integer or float part before decimal point.
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.pos++
		l.col++
	}

	// Decimal part.
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		l.pos++
		l.col++
		for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '_') {
			l.pos++
			l.col++
		}
	}

	// Exponent part.
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.pos++
		l.col++
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.pos++
			l.col++
		}
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.pos++
			l.col++
		}
	}

	// Complex suffix.
	l.consumeComplexSuffix()

	return token.Token{Type: token.NUMBER, Lit: string(l.src[start:l.pos]), Pos: pos}
}

func (l *Lexer) consumeComplexSuffix() {
	if l.pos < len(l.src) && (l.src[l.pos] == 'j' || l.src[l.pos] == 'J') {
		l.pos++
		l.col++
	}
}

// lexOperator lexes an operator or delimiter token.
func (l *Lexer) lexOperator() token.Token {
	pos := l.currentPos()
	ch := l.src[l.pos]
	l.pos++
	l.col++

	// Multi-char operators.
	if l.pos < len(l.src) {
		next := l.src[l.pos]
		switch ch {
		case '+':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.PLUSEQ, Lit: "+=", Pos: pos}
			}
		case '-':
			if next == '>' {
				l.pos++
				l.col++
				return token.Token{Type: token.ARROW, Lit: "->", Pos: pos}
			}
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.MINUSEQ, Lit: "-=", Pos: pos}
			}
		case ':':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.WALRUS, Lit: ":=", Pos: pos}
			}
		case '*':
			if next == '*' {
				l.pos++
				l.col++
				// Check for **=
				if l.pos < len(l.src) && l.src[l.pos] == '=' {
					l.pos++
					l.col++
					return token.Token{Type: token.DSTAREQ, Lit: "**=", Pos: pos}
				}
				return token.Token{Type: token.DSTAR, Lit: "**", Pos: pos}
			}
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.STAREQ, Lit: "*=", Pos: pos}
			}
		case '/':
			if next == '/' {
				l.pos++
				l.col++
				// Check for //=
				if l.pos < len(l.src) && l.src[l.pos] == '=' {
					l.pos++
					l.col++
					return token.Token{Type: token.DSLASHEQ, Lit: "//=", Pos: pos}
				}
				return token.Token{Type: token.DSLASH, Lit: "//", Pos: pos}
			}
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.SLASHEQ, Lit: "/=", Pos: pos}
			}
		case '%':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.PERCENTEQ, Lit: "%=", Pos: pos}
			}
		case '&':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.AMPEREQ, Lit: "&=", Pos: pos}
			}
		case '|':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.PIPEEQ, Lit: "|=", Pos: pos}
			}
		case '^':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.CARETEQ, Lit: "^=", Pos: pos}
			}
		case '=':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.EQ, Lit: "==", Pos: pos}
			}
		case '!':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.NEQ, Lit: "!=", Pos: pos}
			}
		case '<':
			if next == '<' {
				l.pos++
				l.col++
				// Check for <<=
				if l.pos < len(l.src) && l.src[l.pos] == '=' {
					l.pos++
					l.col++
					return token.Token{Type: token.LSHIFTEQ, Lit: "<<=", Pos: pos}
				}
				return token.Token{Type: token.LSHIFT, Lit: "<<", Pos: pos}
			}
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.LTE, Lit: "<=", Pos: pos}
			}
		case '>':
			if next == '>' {
				l.pos++
				l.col++
				// Check for >>=
				if l.pos < len(l.src) && l.src[l.pos] == '=' {
					l.pos++
					l.col++
					return token.Token{Type: token.RSHIFTEQ, Lit: ">>=", Pos: pos}
				}
				return token.Token{Type: token.RSHIFT, Lit: ">>", Pos: pos}
			}
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.GTE, Lit: ">=", Pos: pos}
			}
		case '.':
			if next == '.' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '.' {
				l.pos += 2
				l.col += 2
				return token.Token{Type: token.ELLIPSIS, Lit: "...", Pos: pos}
			}
		}
	}

	// Single-char operators.
	switch ch {
	case '(':
		l.bracketDepth++
		return token.Token{Type: token.LPAREN, Lit: "(", Pos: pos}
	case ')':
		if l.bracketDepth > 0 {
			l.bracketDepth--
		}
		return token.Token{Type: token.RPAREN, Lit: ")", Pos: pos}
	case '[':
		l.bracketDepth++
		return token.Token{Type: token.LBRACK, Lit: "[", Pos: pos}
	case ']':
		if l.bracketDepth > 0 {
			l.bracketDepth--
		}
		return token.Token{Type: token.RBRACK, Lit: "]", Pos: pos}
	case '{':
		l.bracketDepth++
		return token.Token{Type: token.LBRACE, Lit: "{", Pos: pos}
	case '}':
		if l.bracketDepth > 0 {
			l.bracketDepth--
		}
		return token.Token{Type: token.RBRACE, Lit: "}", Pos: pos}
	case ':':
		return token.Token{Type: token.COLON, Lit: ":", Pos: pos}
	case '=':
		return token.Token{Type: token.ASSIGN, Lit: "=", Pos: pos}
	case '|':
		return token.Token{Type: token.PIPE, Lit: "|", Pos: pos}
	case '*':
		return token.Token{Type: token.STAR, Lit: "*", Pos: pos}
	case '.':
		return token.Token{Type: token.DOT, Lit: ".", Pos: pos}
	case '/':
		return token.Token{Type: token.SLASH, Lit: "/", Pos: pos}
	case '@':
		return token.Token{Type: token.AT, Lit: "@", Pos: pos}
	case ',':
		return token.Token{Type: token.COMMA, Lit: ",", Pos: pos}
	case ';':
		return token.Token{Type: token.SEMI, Lit: ";", Pos: pos}
	case '+':
		return token.Token{Type: token.PLUS, Lit: "+", Pos: pos}
	case '-':
		return token.Token{Type: token.MINUS, Lit: "-", Pos: pos}
	case '~':
		return token.Token{Type: token.TILDE, Lit: "~", Pos: pos}
	case '&':
		return token.Token{Type: token.AMPER, Lit: "&", Pos: pos}
	case '^':
		return token.Token{Type: token.CARET, Lit: "^", Pos: pos}
	case '%':
		return token.Token{Type: token.PERCENT, Lit: "%", Pos: pos}
	case '<':
		return token.Token{Type: token.LT, Lit: "<", Pos: pos}
	case '>':
		return token.Token{Type: token.GT, Lit: ">", Pos: pos}
	case '!':
		return token.Token{Type: token.ILLEGAL, Lit: "!", Pos: pos}
	}

	return token.Token{Type: token.ILLEGAL, Lit: string(ch), Pos: pos}
}

// Helper predicates.

func isIdentStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isIdentContinue(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
