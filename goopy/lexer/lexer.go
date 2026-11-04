// Package lexer implements a Python lexer (Pass 1 of the goopy two-pass parser).
// It converts Python source bytes into a flat token stream, handling Python's
// indentation-sensitive syntax by emitting synthetic INDENT/DEDENT tokens.
package lexer

import (
	"bytes"
	"unicode"
	"unicode/utf8"

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

	fstringFailed bool // an f-string was invalid; lex the rest as plain strings
}

// New returns a new Lexer for the given source bytes.
func New(src []byte) *Lexer {
	// A byte order mark at the start of a file is not part of the source.
	src = bytes.TrimPrefix(src, []byte("\xef\xbb\xbf"))
	return &Lexer{
		src:         src,
		line:        1,
		col:         0,
		indentStack: []int{0},
		atLineStart: true,
	}
}

// Clone returns an independent copy of the lexer, for token lookahead.
func (l *Lexer) Clone() *Lexer {
	c := *l
	c.tokens = append([]token.Token(nil), l.tokens...)
	c.indentStack = append([]int(nil), l.indentStack...)
	return &c
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

		// Skip spaces, tabs and form feeds (not at line start — those were handled above).
		if ch == ' ' || ch == '\t' || ch == '\f' {
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

		// Identifiers and keywords (including multi-byte UTF-8 starts).
		if isIdentStart(ch) || ch >= 0x80 {
			// For multi-byte starts, verify the decoded rune is actually a letter.
			if ch >= 0x80 {
				r, _ := utf8.DecodeRune(l.src[l.pos:])
				if !isIdentStartRune(r) {
					return l.lexOperator()
				}
			}
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
	for i < len(l.src) && (l.src[i] == ' ' || l.src[i] == '\t' || l.src[i] == '\f') {
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
		} else if l.src[i] == '\f' {
			// As in CPython, a form feed resets the indent count.
			col = 0
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
		ch == 'f' || ch == 'F' || ch == 'u' || ch == 'U' ||
		ch == 't' || ch == 'T'
}

// maxFStringDepth bounds recursion on nested f-string fields, so that crafted input
// cannot overflow the goroutine stack. CPython's own limit is far lower.
const maxFStringDepth = 150

// lexString lexes a string literal (with optional prefix).
func (l *Lexer) lexString() token.Token {
	pos := l.currentPos()
	start := l.pos

	isFStr := l.skipStringPrefix()
	if isFStr && !l.fstringFailed {
		savedPos, savedLine, savedCol := l.pos, l.line, l.col
		if l.scanFString(0) {
			return token.Token{Type: token.STRING, Lit: string(l.src[start:l.pos]), Pos: pos}
		}
		// Invalid f-string. Lex it and any later ones as plain strings, so that
		// each byte of the file is scanned at most twice.
		l.fstringFailed = true
		l.pos, l.line, l.col = savedPos, savedLine, savedCol
	}
	l.scanPlainString()

	return token.Token{Type: token.STRING, Lit: string(l.src[start:l.pos]), Pos: pos}
}

// skipStringPrefix consumes a string prefix and reports whether it marks an f-string
// or a t-string. The two have the same syntax.
func (l *Lexer) skipStringPrefix() (isFStr bool) {
	for l.pos < len(l.src) && isStringPrefix(l.src[l.pos]) {
		ch := l.src[l.pos]
		if ch == 'f' || ch == 'F' || ch == 't' || ch == 'T' {
			isFStr = true
		}
		l.pos++
		l.col++
	}
	return isFStr
}

// openQuote consumes the opening quotes of a string.
func (l *Lexer) openQuote() (quote byte, triple bool) {
	quote = l.src[l.pos]
	if l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote {
		l.pos += 3
		l.col += 3
		return quote, true
	}
	l.pos++
	l.col++
	return quote, false
}

// atCloseQuote returns true at the closing quotes of a string.
func (l *Lexer) atCloseQuote(quote byte, triple bool) bool {
	if l.src[l.pos] != quote {
		return false
	}
	return !triple || (l.pos+2 < len(l.src) && l.src[l.pos+1] == quote && l.src[l.pos+2] == quote)
}

// closeQuote consumes the closing quotes of a string.
func (l *Lexer) closeQuote(triple bool) {
	n := 1
	if triple {
		n = 3
	}
	l.pos += n
	l.col += n
}

// step consumes one source character, and keeps the line and column correct.
func (l *Lexer) step() {
	switch ch := l.src[l.pos]; ch {
	case '\r':
		if l.pos+1 < len(l.src) && l.src[l.pos+1] == '\n' {
			l.pos++
		}
		l.line++
		l.col = 0
	case '\n':
		l.line++
		l.col = 0
	case '\t':
		l.col += 8 - (l.col % 8)
	default:
		l.col++
	}
	l.pos++
}

// scanPlainString consumes a string body with its quotes. An unterminated
// single-line string stops at the end of the line.
func (l *Lexer) scanPlainString() {
	quote, triple := l.openQuote()
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		switch {
		case ch == '\\' && l.pos+1 < len(l.src):
			l.step()
			l.step()
		case (ch == '\n' || ch == '\r') && !triple:
			return
		case l.atCloseQuote(quote, triple):
			l.closeQuote(triple)
			return
		default:
			l.step()
		}
	}
}

// scanFString consumes an f-string body with its quotes. It returns false if the
// string is invalid, and then the caller must restore the position.
func (l *Lexer) scanFString(depth int) bool {
	if depth > maxFStringDepth {
		return false
	}
	quote, triple := l.openQuote()
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		switch {
		case ch == '\\' && l.pos+1 < len(l.src):
			l.stepEscape()
		case (ch == '\n' || ch == '\r') && !triple:
			return false
		case l.atCloseQuote(quote, triple):
			l.closeQuote(triple)
			return true
		case ch == '{':
			if l.pos+1 < len(l.src) && l.src[l.pos+1] == '{' {
				l.step()
				l.step()
				continue
			}
			l.step()
			if !l.scanField(triple, depth+1) {
				return false
			}
		default:
			// Includes } and }}, which are literal text here.
			l.step()
		}
	}
	return false
}

// stepEscape consumes a backslash in the literal part of an f-string. A brace after
// the backslash keeps its meaning, even in a raw string: rf'\{{' is an escaped brace.
func (l *Lexer) stepEscape() {
	l.step()
	if next := l.src[l.pos]; next != '{' && next != '}' {
		l.step()
	}
}

// scanField consumes a replacement field, after its opening brace, up to and
// including the closing brace. It tracks brackets and nested strings, as the
// expression can hold both, and in Python 3.12+ it can hold the same quote as the
// enclosing string, comments, and newlines.
func (l *Lexer) scanField(triple bool, depth int) bool {
	brackets := 0
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		switch {
		case l.isStringStart():
			if l.skipStringPrefix() {
				if !l.scanFString(depth) {
					return false
				}
			} else {
				l.scanPlainString()
			}
		case ch == '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' && l.src[l.pos] != '\r' {
				l.step()
			}
		case ch == '(' || ch == '[' || ch == '{':
			brackets++
			l.step()
		case ch == ')' || ch == ']':
			brackets--
			l.step()
		case ch == '}':
			l.step()
			if brackets <= 0 {
				return true
			}
			brackets--
		case ch == ':' && brackets <= 0:
			l.step()
			return l.scanFormatSpec(triple, depth)
		default:
			l.step()
		}
	}
	return false
}

// scanFormatSpec consumes a format spec, after its colon, up to and including the
// closing brace of the field. Quotes have no meaning in a format spec, and it can
// hold nested fields.
func (l *Lexer) scanFormatSpec(triple bool, depth int) bool {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		switch {
		case ch == '\\' && l.pos+1 < len(l.src):
			l.stepEscape()
		case (ch == '\n' || ch == '\r') && !triple:
			return false
		case ch == '{':
			if depth > maxFStringDepth {
				return false
			}
			l.step()
			if !l.scanField(triple, depth+1) {
				return false
			}
		case ch == '}':
			l.step()
			return true
		default:
			l.step()
		}
	}
	return false
}

// lexIdent lexes an identifier or keyword.
// Handles both ASCII and multi-byte UTF-8 identifiers (e.g. Greek letters).
func (l *Lexer) lexIdent() token.Token {
	pos := l.currentPos()
	start := l.pos
	for l.pos < len(l.src) {
		b := l.src[l.pos]
		if b < 0x80 {
			// ASCII fast path.
			if !isIdentContinue(b) {
				break
			}
			l.pos++
			l.col++
		} else {
			// Multi-byte UTF-8: decode a full rune and check it.
			r, size := utf8.DecodeRune(l.src[l.pos:])
			if !isIdentContinueRune(r) {
				break
			}
			l.pos += size
			l.col++
		}
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

	// Exponent part. Only digits make it an exponent: 1else starts a keyword.
	if n := l.exponentLen(); n > 0 {
		l.pos += n
		l.col += n
	}

	// Complex suffix.
	l.consumeComplexSuffix()

	return token.Token{Type: token.NUMBER, Lit: string(l.src[start:l.pos]), Pos: pos}
}

// exponentLen returns the length of the exponent at the current position, or 0.
func (l *Lexer) exponentLen() int {
	i := l.pos
	if i >= len(l.src) || (l.src[i] != 'e' && l.src[i] != 'E') {
		return 0
	}
	i++
	if i < len(l.src) && (l.src[i] == '+' || l.src[i] == '-') {
		i++
	}
	if i >= len(l.src) || !isDigit(l.src[i]) {
		return 0
	}
	for i < len(l.src) && (isDigit(l.src[i]) || l.src[i] == '_') {
		i++
	}
	return i - l.pos
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
		case '@':
			if next == '=' {
				l.pos++
				l.col++
				return token.Token{Type: token.ATEQ, Lit: "@=", Pos: pos}
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

// isIdentStartRune approximates the XID_Start property for a non-ASCII rune.
func isIdentStartRune(r rune) bool {
	return r != utf8.RuneError && (unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_ID_Start, r))
}

// isIdentContinueRune approximates the XID_Continue property for a non-ASCII rune.
func isIdentContinueRune(r rune) bool {
	return isIdentStartRune(r) || unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Other_ID_Continue)
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}
