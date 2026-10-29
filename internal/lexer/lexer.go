// Package lexer turns Lynx source text into tokens.
package lexer

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"lynx/internal/diag"
	"lynx/internal/token"
)

// Lexer scans a Lynx source file.
type Lexer struct {
	src  string
	file string
	pos  int
	line int
	col  int
}

// New creates a lexer for src. filename is used in diagnostics.
func New(filename, src string) *Lexer {
	return &Lexer{src: src, file: filename, line: 1, col: 1}
}

// Lex scans the whole input.
func (l *Lexer) Lex() ([]token.Token, error) {
	tokens := make([]token.Token, 0, 32)

	for {
		if err := l.skipSpaceAndComments(); err != nil {
			return nil, err
		}

		pos := l.position()
		if l.eof() {
			tokens = append(tokens, token.Token{Type: token.EOF, Pos: pos})
			return tokens, nil
		}

		r := l.peek()
		switch {
		case isIdentStart(r):
			literal := l.scanIdentifier()
			tokens = append(tokens, token.Token{Type: token.Lookup(literal), Literal: literal, Pos: pos})
		case unicode.IsDigit(r):
			tok, err := l.scanNumber(pos)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		case r == '.' && unicode.IsDigit(l.peekNext()):
			tok, err := l.scanNumber(pos)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		case r == '"' || r == '\'':
			tok, err := l.scanString(pos)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		default:
			tok, err := l.scanOperator(pos)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
		}
	}
}

func (l *Lexer) position() token.Position {
	return token.Position{Filename: l.file, Offset: l.pos, Line: l.line, Column: l.col}
}

func (l *Lexer) eof() bool {
	return l.pos >= len(l.src)
}

func (l *Lexer) peek() rune {
	if l.eof() {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos:])
	return r
}

func (l *Lexer) peekNext() rune {
	if l.eof() {
		return 0
	}
	_, size := utf8.DecodeRuneInString(l.src[l.pos:])
	if l.pos+size >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos+size:])
	return r
}

func (l *Lexer) advance() rune {
	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	l.pos += size
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) match(want rune) bool {
	if l.peek() == want {
		l.advance()
		return true
	}
	return false
}

func (l *Lexer) skipSpaceAndComments() error {
	for !l.eof() {
		r := l.peek()
		switch {
		case r == ' ' || r == '\t' || r == '\r' || r == '\n':
			l.advance()
		case r == '/' && l.peekNext() == '/':
			for !l.eof() && l.peek() != '\n' {
				l.advance()
			}
		case r == '/' && l.peekNext() == '*':
			start := l.position()
			l.advance()
			l.advance()
			closed := false
			for !l.eof() {
				if l.peek() == '*' && l.peekNext() == '/' {
					l.advance()
					l.advance()
					closed = true
					break
				}
				l.advance()
			}
			if !closed {
				return diag.New(start, "unterminated block comment")
			}
		default:
			return nil
		}
	}
	return nil
}

func (l *Lexer) scanIdentifier() string {
	start := l.pos
	for !l.eof() && isIdentPart(l.peek()) {
		l.advance()
	}
	return l.src[start:l.pos]
}

func (l *Lexer) scanNumber(pos token.Position) (token.Token, error) {
	start := l.pos

	if l.peek() == '.' {
		l.advance()
		for !l.eof() && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	} else {
		for !l.eof() && unicode.IsDigit(l.peek()) {
			l.advance()
		}
		if l.peek() == '.' {
			l.advance()
			for !l.eof() && unicode.IsDigit(l.peek()) {
				l.advance()
			}
		}
	}

	if l.peek() == 'e' || l.peek() == 'E' {
		l.advance()
		if l.peek() == '+' || l.peek() == '-' {
			l.advance()
		}
		if !unicode.IsDigit(l.peek()) {
			return token.Token{}, diag.New(pos, "invalid number literal: exponent has no digits")
		}
		for !l.eof() && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}

	raw := l.src[start:l.pos]
	if !l.eof() && isIdentPart(l.peek()) {
		for !l.eof() && isIdentPart(l.peek()) {
			l.advance()
		}
		return token.Token{}, diag.New(pos, "invalid number literal %q", l.src[start:l.pos])
	}
	if _, err := strconv.ParseFloat(raw, 64); err != nil {
		return token.Token{}, diag.New(pos, "invalid number literal %q", raw)
	}

	return token.Token{Type: token.Number, Literal: raw, Pos: pos}, nil
}

func (l *Lexer) scanString(pos token.Position) (token.Token, error) {
	quote := l.advance()
	var value strings.Builder

	for {
		if l.eof() {
			return token.Token{}, diag.New(pos, "unterminated string literal")
		}
		r := l.peek()
		if r == '\n' || r == '\r' {
			return token.Token{}, diag.New(pos, "unterminated string literal")
		}
		l.advance()
		if r == quote {
			return token.Token{Type: token.String, Literal: value.String(), Pos: pos}, nil
		}
		if r != '\\' {
			value.WriteRune(r)
			continue
		}

		escPos := l.position()
		if l.eof() {
			return token.Token{}, diag.New(pos, "unterminated string literal")
		}
		esc := l.advance()
		switch esc {
		case 'n':
			value.WriteByte('\n')
		case 'r':
			value.WriteByte('\r')
		case 't':
			value.WriteByte('\t')
		case 'b':
			value.WriteByte('\b')
		case 'f':
			value.WriteByte('\f')
		case 'v':
			value.WriteByte('\v')
		case '0':
			value.WriteByte(0)
		case '\\', '\'', '"', '/':
			value.WriteRune(esc)
		case 'x':
			code, err := l.scanHex(2, escPos)
			if err != nil {
				return token.Token{}, err
			}
			value.WriteRune(code)
		case 'u':
			code, err := l.scanHex(4, escPos)
			if err != nil {
				return token.Token{}, err
			}
			value.WriteRune(code)
		default:
			return token.Token{}, diag.New(escPos, "unknown escape sequence \\%c", esc)
		}
	}
}

func (l *Lexer) scanHex(count int, pos token.Position) (rune, error) {
	var value rune
	for i := 0; i < count; i++ {
		if l.eof() {
			return 0, diag.New(pos, "invalid escape sequence: expected %d hexadecimal digits", count)
		}
		r := l.advance()
		digit := hexValue(r)
		if digit < 0 {
			return 0, diag.New(pos, "invalid escape sequence: expected %d hexadecimal digits", count)
		}
		value = value*16 + rune(digit)
	}
	return value, nil
}

func (l *Lexer) scanOperator(pos token.Position) (token.Token, error) {
	r := l.advance()

	if r == '.' && l.peek() == '.' && l.peekNext() == '.' {
		l.advance()
		l.advance()
		return token.Token{Type: token.Ellipsis, Literal: "...", Pos: pos}, nil
	}

	if r == '-' && l.peek() == '>' {
		l.advance()
		return token.Token{Type: token.Arrow, Literal: "->", Pos: pos}, nil
	}

	single := map[rune]token.Type{
		'+': token.Plus,
		'-': token.Minus,
		'*': token.Star,
		'/': token.Slash,
		'%': token.Percent,
		':': token.Colon,
		',': token.Comma,
		';': token.Semicolon,
		'.': token.Dot,
		'?': token.Question,
		'[': token.LBracket,
		']': token.RBracket,
		'(': token.LParen,
		')': token.RParen,
		'{': token.LBrace,
		'}': token.RBrace,
	}
	if typ, ok := single[r]; ok {
		return token.Token{Type: typ, Literal: string(r), Pos: pos}, nil
	}

	switch r {
	case '=':
		if l.match('=') {
			if l.match('=') {
				return token.Token{Type: token.StrictEq, Literal: "===", Pos: pos}, nil
			}
			return token.Token{Type: token.Eq, Literal: "==", Pos: pos}, nil
		}
		return token.Token{Type: token.Assign, Literal: "=", Pos: pos}, nil
	case '!':
		if l.match('=') {
			if l.match('=') {
				return token.Token{Type: token.StrictNe, Literal: "!==", Pos: pos}, nil
			}
			return token.Token{Type: token.NotEq, Literal: "!=", Pos: pos}, nil
		}
		return token.Token{Type: token.Bang, Literal: "!", Pos: pos}, nil
	case '<':
		if l.match('=') {
			return token.Token{Type: token.LtEq, Literal: "<=", Pos: pos}, nil
		}
		return token.Token{Type: token.Lt, Literal: "<", Pos: pos}, nil
	case '>':
		if l.match('=') {
			return token.Token{Type: token.GtEq, Literal: ">=", Pos: pos}, nil
		}
		return token.Token{Type: token.Gt, Literal: ">", Pos: pos}, nil
	case '&':
		if l.match('&') {
			return token.Token{Type: token.AndAnd, Literal: "&&", Pos: pos}, nil
		}
	case '|':
		if l.match('|') {
			return token.Token{Type: token.OrOr, Literal: "||", Pos: pos}, nil
		}
	}

	return token.Token{}, diag.New(pos, "unexpected character %q", string(r))
}

func isIdentStart(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r)
}

func hexValue(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return int(r - '0')
	case r >= 'a' && r <= 'f':
		return int(r-'a') + 10
	case r >= 'A' && r <= 'F':
		return int(r-'A') + 10
	default:
		return -1
	}
}
