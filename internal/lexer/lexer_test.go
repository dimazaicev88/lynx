package lexer

import (
	"strings"
	"testing"

	"lynx/internal/token"
)

func TestLexDeclarations(t *testing.T) {
	src := `const a:Number=10; var b = "hi";`
	tokens, err := New("test.lynx", src).Lex()
	if err != nil {
		t.Fatalf("Lex() error = %v", err)
	}

	want := []token.Type{
		token.Const, token.Ident, token.Colon, token.Ident, token.Assign, token.Number, token.Semicolon,
		token.Var, token.Ident, token.Assign, token.String, token.Semicolon,
		token.EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(want), tokens)
	}
	for i, typ := range want {
		if tokens[i].Type != typ {
			t.Errorf("token %d type = %s, want %s", i, tokens[i].Type, typ)
		}
	}
	if tokens[10].Literal != "hi" {
		t.Errorf("string literal = %q, want %q", tokens[10].Literal, "hi")
	}
}

func TestLexCommentsNumbersAndEscapes(t *testing.T) {
	src := "// line comment\n/* block\ncomment */ const a = .5; const b = 1.5e3; const s = \"a\\n\\\"b\"; const q = 'x';"
	tokens, err := New("test.lynx", src).Lex()
	if err != nil {
		t.Fatalf("Lex() error = %v", err)
	}

	var literals []string
	for _, tok := range tokens {
		if tok.Type == token.Number || tok.Type == token.String {
			literals = append(literals, tok.Literal)
		}
	}
	want := []string{".5", "1.5e3", "a\n\"b", "x"}
	if len(literals) != len(want) {
		t.Fatalf("literals = %#v, want %#v", literals, want)
	}
	for i := range want {
		if literals[i] != want[i] {
			t.Errorf("literal %d = %q, want %q", i, literals[i], want[i])
		}
	}

	if tokens[0].Pos.Line != 3 || tokens[0].Pos.Column != 12 {
		t.Errorf("first token position = %s, want test.lynx:3:12", tokens[0].Pos)
	}
}

func TestLexFunctions(t *testing.T) {
	src := `fun f(a:String? = null, b:Number...):Number { return 1 }`
	tokens, err := New("test.lynx", src).Lex()
	if err != nil {
		t.Fatalf("Lex() error = %v", err)
	}

	want := []token.Type{
		token.Fun, token.Ident, token.LParen,
		token.Ident, token.Colon, token.Ident, token.Question, token.Assign, token.Null, token.Comma,
		token.Ident, token.Colon, token.Ident, token.Ellipsis,
		token.RParen, token.Colon, token.Ident, token.LBrace, token.Return, token.Number, token.RBrace,
		token.EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(want), tokens)
	}
	for i, typ := range want {
		if tokens[i].Type != typ {
			t.Errorf("token %d type = %s, want %s", i, tokens[i].Type, typ)
		}
	}
}

func TestLexLambda(t *testing.T) {
	src := `val sum = { x: Int, y: Int -> x + y }`
	tokens, err := New("test.lynx", src).Lex()
	if err != nil {
		t.Fatalf("Lex() error = %v", err)
	}

	want := []token.Type{
		token.Const, token.Ident, token.Assign, token.LBrace,
		token.Ident, token.Colon, token.Ident, token.Comma,
		token.Ident, token.Colon, token.Ident, token.Arrow,
		token.Ident, token.Plus, token.Ident, token.RBrace,
		token.EOF,
	}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(want), tokens)
	}
	for i, typ := range want {
		if tokens[i].Type != typ {
			t.Errorf("token %d type = %s, want %s", i, tokens[i].Type, typ)
		}
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"unterminated string", `const a = "x;`, "unterminated string literal"},
		{"newline in string", "const a = \"x\ny\";", "unterminated string literal"},
		{"unterminated comment", "const a = 1; /* nope", "unterminated block comment"},
		{"bad character", "const a = @;", "unexpected character"},
		{"bad number", "const a = 10abc;", "invalid number literal"},
		{"bad exponent", "const a = 1e;", "exponent has no digits"},
		{"bad escape", `const a = "a\q";`, "unknown escape sequence"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New("test.lynx", tt.src).Lex()
			if err == nil {
				t.Fatal("Lex() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
