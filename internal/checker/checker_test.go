package checker

import (
	"strings"
	"testing"

	"lynx/internal/lexer"
	"lynx/internal/parser"
)

func checkSource(t *testing.T, src string) error {
	t.Helper()
	tokens, err := lexer.New("test.lynx", src).Lex()
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	program, err := parser.Parse(tokens)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return Check(program)
}

func TestCheckValidPrograms(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "specification declarations",
			src: `const a=10;
			      const b:Number=10;
			      var c=10;
			      var d:Number=10;`,
		},
		{
			name: "declaration without initializer",
			src:  `var x: Number; x = 5;`,
		},
		{
			name: "inferred arrays",
			src:  `const a = [1, 2, 3]; const b = a[0]; const c = a.length;`,
		},
		{
			name: "nested arrays",
			src:  `const a = [[1, 2], [3, 4]]; const row = a[0]; if row != null { const b = row[1]; }`,
		},
		{
			name: "string concatenation",
			src:  `const s: String = "a" + "b" + 1;`,
		},
		{
			name: "any is dynamic",
			src:  `var x: Any = 10; x = "s"; const n: Number = x;`,
		},
		{
			name: "unknown values stay unknown",
			src:  `var x: Unknown = 10; const y: Unknown = x;`,
		},
		{
			name: "array annotation",
			src:  `var x: Array = [1, "two"]; x = ["three", 4];`,
		},
		{
			name: "mixed array becomes any",
			src:  `const a = [1, "two"]; var b: Any = a[0];`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSource(t, tt.src); err != nil {
				t.Errorf("Check() error = %v", err)
			}
		})
	}
}

func TestCheckErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"wrong initializer type", `const a: Number = "x";`, "cannot assign a value of type String"},
		{"assign to const", `const a = 1; a = 2;`, "cannot assign to constant"},
		{"assign wrong type", `var a: Number = 1; a = "x";`, "cannot assign a value of type String"},
		{"array annotation mismatch", `const a: Array = 1;`, "cannot assign a value of type Number"},
		{"unknown type name", `const a: Foo = 1;`, `unknown type "Foo"`},
		{"undefined variable", `const a = b;`, `undefined variable "b"`},
		{"duplicate declaration", `var a = 1; var a = 2;`, `variable "a" is already declared`},
		{"unknown is not concrete", `const x: Unknown = 1; const y: Number = x;`, "cannot assign a value of type Unknown"},
		{"unknown arithmetic", `const x: Unknown = 1; const y = x + 1;`, "type Unknown"},
		{"undefined operator", `const x = "a" - 1;`, `operator "-" is not defined`},
		{"index a number", `const x = 1[0];`, "cannot index a value of type Number"},
		{"string index", `const x = [1]["a"];`, "array index must be Number"},
		{"unknown property", `const x = [1].count;`, `unknown property "count"`},
		{"length on number", `const x = (1).length;`, "has no property .length"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSource(t, tt.src)
			if err == nil {
				t.Fatal("Check() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestCheckValidFunctions(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "specification functions",
			src: `fun example(b: String? = null, a: Number = 10): String? {
			        return b
			      }
			      const first: String? = example(b="test", a=10);
			      const second: String? = example("test", 10);
			      const third: String? = example();`,
		},
		{
			name: "variadic function",
			src:  `fun sum(numbers: Number...): Number? { return numbers[0] } const total: Number? = sum(1, 2, 3, 4);`,
		},
		{
			name: "void function",
			src:  `fun log(message: String) { } log("hello");`,
		},
		{
			name: "null and nullable",
			src: `const nothing: Null = null;
			      var maybe: String? = null;
			      maybe = "x";
			      const values = [1, null];`,
		},
		{
			name: "forward calls and globals",
			src: `const x: Number = add(1, 2);
			      fun add(a: Number, b: Number): Number { return a + b }
			      fun usesGlobal(): Number { return g }
			      const g: Number = 10;`,
		},
		{
			name: "defaults reference earlier parameters",
			src:  `fun make(a: Number, b: Number = a + 1): Number { return b } const value = make(1);`,
		},
		{
			name: "local block scope",
			src:  `fun f(a: Number): Number { const b = a + 1; { const c = b; } return b; }`,
		},
		{
			name: "named argument gap",
			src:  `fun f(a: Number = 1, b: Number = 2): Number { return b } const x = f(b=5);`,
		},
		{
			name: "nullable parameter and return",
			src:  `fun f(x: Number?): Number? { return x } const y = f(null);`,
		},
		{
			name: "nullable array elements",
			src:  `const values = [1, null]; const x: Number? = values[0];`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSource(t, tt.src); err != nil {
				t.Errorf("Check() error = %v", err)
			}
		})
	}
}

func TestCheckFunctionErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"wrong return type", `fun f(): Number { return "x" }`, "cannot return a value of type String"},
		{"bare return from non-void", `fun f(): Number { return }`, "must return a value"},
		{"value from void", `fun f() { return 1 }`, "without a return type"},
		{"missing return", `fun f(): Number { const x = 1; }`, "does not return a value"},
		{"return outside function", `return 1;`, "return statement outside a function"},
		{"undefined function", `g();`, `undefined function "g"`},
		{"wrong positional argument", `fun f(a: Number) {} f("x");`, "cannot use a value of type String as argument 1"},
		{"wrong named argument", `fun f(a: Number) {} f(a="x");`, `for parameter "a"`},
		{"unknown named parameter", `fun f(a: Number) {} f(b=1);`, `has no parameter "b"`},
		{"missing required argument", `fun f(a: Number) {} f();`, `missing argument "a"`},
		{"too many arguments", `fun f(a: Number) {} f(1, 2);`, "too many arguments"},
		{"duplicate named argument", `fun f(a: Number) {} f(a=1, a=2);`, `argument "a" is specified more than once`},
		{"variadic argument type", `fun f(numbers: Number...) {} f(1, "x");`, "cannot use a value of type String as argument 2"},
		{"variadic not last", `fun f(rest: Number..., a: Number) {}`, `variadic parameter "rest" must be last`},
		{"multiple variadic", `fun f(a: Number..., b: Number...) {}`, "only one variadic parameter is allowed"},
		{"variadic default", `fun f(rest: Number... = 1) {}`, "cannot have a default value"},
		{"variadic by name", `fun f(rest: Number...) {} f(rest=1);`, "cannot be passed by name"},
		{"duplicate parameter", `fun f(a: Number, a: Number) {}`, `parameter "a" is already declared`},
		{"duplicate function", `fun f() {} fun f() {}`, `function "f" is already declared`},
		{"variable conflicts with function", `const f = 1; fun f() {}`, `variable "f" conflicts with function "f"`},
		{"parameter conflicts with function", `fun f(f: Number) {}`, `parameter "f" conflicts with function "f"`},
		{"local variable conflicts with function", `fun g() { const g = 1; }`, `variable "g" conflicts with function "g"`},
		{"null argument to non-nullable", `fun f(a: String) {} f(null);`, "cannot use a value of type Null"},
		{"null variable", `const x: Number = null;`, "cannot assign a value of type Null"},
		{"nullable array element", `const values = [1, null]; const x: Number = values[0];`, "cannot assign a value of type Number?"},
		{"nullable arithmetic", `const x: Number? = 1; const y = x + 1;`, "may be null"},
		{"nullable index", `const x: Array? = null; const y = x[0];`, "may be null"},
		{"nullable length", `const x: String? = null; const y = x.length;`, "may be null"},
		{"void initializer", `fun f() {} const x = f();`, "does not exist"},
		{"void argument", `fun f() {} fun g(a: Number) {} g(f());`, "argument does not produce a value"},
		{"wrong default", `fun f(a: Number = "x") {}`, "default value of type String"},
		{"void default", `fun f() {} fun g(a: Number = f()) {}`, "default value does not return a value"},
		{"nested function", `fun outer() { fun inner() {} }`, "only allowed at the top level"},
		{"null return", `fun f(): Number { return null }`, "cannot return a value of type Null"},
		{"unknown parameter type", `fun f(a: Foo) {}`, `unknown type "Foo"`},
		{"void return expression", `fun f() {} fun g(): Number { return f() }`, "returned expression does not produce a value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSource(t, tt.src)
			if err == nil {
				t.Fatal("Check() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestCheckValidLambdas(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "specification lambda",
			src: `fun log(value: Number) {}
			      val sum = { x: Int, y: Int -> x + y }
			      log(sum(1, 5))`,
		},
		{
			name: "function type annotation",
			src:  `const sum: (Number, Number) -> Number = { x: Number, y: Number -> x + y } const r = sum(1, 2);`,
		},
		{
			name: "lambda argument",
			src:  `fun apply(f: (Number) -> Number, value: Number): Number { return f(value) } const r = apply({ n: Number -> n + 1 }, 10);`,
		},
		{
			name: "closure",
			src:  `const outer = { x: Number -> { y: Number -> x + y } } const add = outer(1); const r = add(2);`,
		},
		{
			name: "zero parameter lambda",
			src:  `const answer = { -> 42 } const r = answer();`,
		},
		{
			name: "lambda reassignment",
			src:  `var f = { x: Number -> x + 1 }; f = { y: Number -> y + 2 }; const r = f(1);`,
		},
		{
			name: "array of lambdas",
			src:  `const fs = [{ x: Number -> x }, { y: Number -> y }]; const f = fs[0]; if f != null { const r = f(1); }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSource(t, tt.src); err != nil {
				t.Errorf("Check() error = %v", err)
			}
		})
	}
}

func TestCheckLambdaErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"duplicate parameter", `const f = { x: Number, x: Number -> x };`, `parameter "x" is already declared`},
		{"value not callable", `const x = 1; x();`, `value "x" of type Number is not callable`},
		{"wrong argument type", `const f = { x: Number -> x }; f("s");`, `cannot use a value of type String as argument 1 of value "f"`},
		{"too many arguments", `const f = { x: Number -> x }; f(1, 2);`, "too many arguments"},
		{"missing argument", `const f = { x: Number, y: Number -> x }; f(1);`, `requires 2 arguments, got 1`},
		{"named argument", `const f = { x: Number -> x }; f(x=1);`, "named arguments are not supported for function values"},
		{"void lambda body", `fun log(v: Number) {} const f = { x: Number -> log(x) };`, "lambda body does not produce a value"},
		{"function type mismatch", `const f: (Number) -> Number = { x: Number -> "s" };`, "cannot assign a value of type (Number) -> String"},
		{"unknown parameter type", `const f = { x: Foo -> x };`, `unknown type "Foo"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSource(t, tt.src)
			if err == nil {
				t.Fatal("Check() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestCheckReportsMultipleErrors(t *testing.T) {
	err := checkSource(t, "const a: Number = \"x\"; const b = c;")
	if err == nil {
		t.Fatal("Check() succeeded, want errors")
	}
	list, ok := err.(interface{ Error() string })
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if count := strings.Count(list.Error(), "undefined variable") + strings.Count(list.Error(), "cannot assign"); count != 2 {
		t.Errorf("expected two diagnostics, got: %v", err)
	}
}
