package codegen

import (
	"testing"

	"lynx/internal/lexer"
	"lynx/internal/parser"
)

func generate(t *testing.T, src string) string {
	t.Helper()
	tokens, err := lexer.New("test.lynx", src).Lex()
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	program, err := parser.Parse(tokens)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return Generate(program)
}

func TestGenerateStatements(t *testing.T) {
	src := "const a = 10;\n" +
		"var b:Number = 10;\n" +
		"b = b + 1;\n" +
		"const s = \"a\\n\\\"b\";\n" +
		"const q = 'it\\'s';\n" +
		"const arr = [1, 2, 3];\n" +
		"const x = arr[0];\n" +
		"const n = arr.length;\n"

	want := "const a = 10;\n" +
		"let b = 10;\n" +
		"b = b + 1;\n" +
		"const s = \"a\\n\\\"b\";\n" +
		"const q = \"it's\";\n" +
		"const arr = [1, 2, 3];\n" +
		"const x = (arr[0] ?? null);\n" +
		"const n = arr.length;\n"

	if got := generate(t, src); got != want {
		t.Errorf("Generate() =\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateParentheses(t *testing.T) {
	src := "const a = (1 + 2) * 3;\n" +
		"const b = 1 + 2 * 3;\n" +
		"const c = 1 - (2 - 3);\n" +
		"const d = 10 / (2 * 5);\n" +
		"const e = -(-1);\n" +
		"const f = (1 + 2)[0];\n"

	want := "const a = (1 + 2) * 3;\n" +
		"const b = 1 + 2 * 3;\n" +
		"const c = 1 - (2 - 3);\n" +
		"const d = 10 / (2 * 5);\n" +
		"const e = -(-1);\n" +
		"const f = ((1 + 2)[0] ?? null);\n"

	if got := generate(t, src); got != want {
		t.Errorf("Generate() =\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateFunctions(t *testing.T) {
	src := `fun example(b:String? = null, a:Number = 10):String? {
	  return b
	}
	example(b="test", a=10)
	example("test", 10)
	example()

	fun variadic(numbers: Number...):Number {
	  return numbers[0]
	}
	variadic(1, 2, 3, 4)

	fun noop() {
	  return
	}
	`

	want := "function example(b = null, a = 10) {\n" +
		"  return b;\n" +
		"}\n" +
		"example(\"test\", 10);\n" +
		"example(\"test\", 10);\n" +
		"example();\n" +
		"function variadic(...numbers) {\n" +
		"  return (numbers[0] ?? null);\n" +
		"}\n" +
		"variadic(1, 2, 3, 4);\n" +
		"function noop() {\n" +
		"  return;\n" +
		"}\n"

	if got := generate(t, src); got != want {
		t.Errorf("Generate() =\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateNamedArguments(t *testing.T) {
	src := `fun f(a:Number = 1, b:Number = 2):Number {
	  return b
	}
	f(b=5)
	f(a=3, b=4)
	f(b=5, a=3)`

	want := "function f(a = 1, b = 2) {\n" +
		"  return b;\n" +
		"}\n" +
		"f(undefined, 5);\n" +
		"f(3, 4);\n" +
		"f(3, 5);\n"

	if got := generate(t, src); got != want {
		t.Errorf("Generate() =\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateNestedBlocks(t *testing.T) {
	src := `fun f():Number {
	  {
	    const x = 1;
	  }
	  return 1
	}`

	want := "function f() {\n" +
		"  {\n" +
		"    const x = 1;\n" +
		"  }\n" +
		"  return 1;\n" +
		"}\n"

	if got := generate(t, src); got != want {
		t.Errorf("Generate() =\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateLambdas(t *testing.T) {
	src := `val sum = { x: Int, y: Int -> x + y }
	log(sum(1, 5))

	fun apply(f: (Number) -> Number, value: Number): Number {
	  return f(value)
	}
	const result = apply({ n: Number -> n + 1 }, 10)

	const outer = { x: Number -> { y: Number -> x + y } }
	const add = outer(1)
	const answer = { -> 42 }`

	want := "const sum = (x, y) => x + y;\n" +
		"log(sum(1, 5));\n" +
		"function apply(f, value) {\n" +
		"  return f(value);\n" +
		"}\n" +
		"const result = apply((n) => n + 1, 10);\n" +
		"const outer = (x) => (y) => x + y;\n" +
		"const add = outer(1);\n" +
		"const answer = () => 42;\n"

	if got := generate(t, src); got != want {
		t.Errorf("Generate() =\n%s\nwant:\n%s", got, want)
	}
}

func TestGenerateEmptyProgram(t *testing.T) {
	if got := generate(t, ""); got != "" {
		t.Errorf("Generate() = %q, want empty string", got)
	}
}
