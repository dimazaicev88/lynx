package parser

import (
	"strings"
	"testing"

	"lynx/internal/ast"
	"lynx/internal/lexer"
	"lynx/internal/token"
)

func mustParse(t *testing.T, src string) *ast.Program {
	t.Helper()
	tokens, err := lexer.New("test.lynx", src).Lex()
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return program
}

func TestParseVarDecls(t *testing.T) {
	program := mustParse(t, `const a:Number=10; var b="x";`)
	if len(program.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(program.Statements))
	}

	first, ok := program.Statements[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("statement 0 is %T, want *ast.VarDecl", program.Statements[0])
	}
	if !first.Const || first.Name != "a" || first.Type == nil || first.Type.Name != "Number" {
		t.Errorf("unexpected first declaration: %#v", first)
	}
	if lit, ok := first.Value.(*ast.NumberLit); !ok || lit.Raw != "10" {
		t.Errorf("first value = %#v, want number 10", first.Value)
	}

	second, ok := program.Statements[1].(*ast.VarDecl)
	if !ok {
		t.Fatalf("statement 1 is %T, want *ast.VarDecl", program.Statements[1])
	}
	if second.Const || second.Name != "b" || second.Type != nil {
		t.Errorf("unexpected second declaration: %#v", second)
	}
	if lit, ok := second.Value.(*ast.StringLit); !ok || lit.Value != "x" {
		t.Errorf("second value = %#v, want string x", second.Value)
	}
}

func TestParsePrecedence(t *testing.T) {
	program := mustParse(t, `const x = 1 + 2 * 3; const y = 1 - 2 - 3;`)
	x := program.Statements[0].(*ast.VarDecl).Value.(*ast.BinaryExpr)
	if x.Op != "+" {
		t.Fatalf("root op = %q, want +", x.Op)
	}
	if right, ok := x.Right.(*ast.BinaryExpr); !ok || right.Op != "*" {
		t.Errorf("right side = %#v, want multiplication", x.Right)
	}

	y := program.Statements[1].(*ast.VarDecl).Value.(*ast.BinaryExpr)
	if y.Op != "-" {
		t.Fatalf("root op = %q, want -", y.Op)
	}
	if left, ok := y.Left.(*ast.BinaryExpr); !ok || left.Op != "-" {
		t.Errorf("left side = %#v, want subtraction", y.Left)
	}
}

func TestParseAssignment(t *testing.T) {
	program := mustParse(t, `var x = 1; x = x + 1;`)
	assign, ok := program.Statements[1].(*ast.AssignStmt)
	if !ok {
		t.Fatalf("statement 1 is %T, want *ast.AssignStmt", program.Statements[1])
	}
	if assign.Name != "x" {
		t.Errorf("assignment name = %q, want x", assign.Name)
	}
	if expr, ok := assign.Value.(*ast.BinaryExpr); !ok || expr.Op != "+" {
		t.Errorf("assignment value = %#v, want addition", assign.Value)
	}
}

func TestParseArrayAndPostfix(t *testing.T) {
	program := mustParse(t, `const v = arr[0][1].length;`)
	member, ok := program.Statements[0].(*ast.VarDecl).Value.(*ast.MemberExpr)
	if !ok {
		t.Fatalf("value is %T, want *ast.MemberExpr", program.Statements[0].(*ast.VarDecl).Value)
	}
	if member.Name != "length" {
		t.Errorf("member name = %q, want length", member.Name)
	}
	outer, ok := member.X.(*ast.IndexExpr)
	if !ok {
		t.Fatalf("member base is %T, want *ast.IndexExpr", member.X)
	}
	if _, ok := outer.X.(*ast.IndexExpr); !ok {
		t.Errorf("nested index base is %T, want *ast.IndexExpr", outer.X)
	}
}

func TestParseFunctionDecl(t *testing.T) {
	src := `fun example(b:String? = null, a:Number = 10): String? { return b }`
	stmt := mustParse(t, src).Statements[0]
	fn, ok := stmt.(*ast.FunctionDecl)
	if !ok {
		t.Fatalf("statement is %T, want *ast.FunctionDecl", stmt)
	}
	if fn.Name != "example" {
		t.Errorf("function name = %q, want example", fn.Name)
	}
	if len(fn.Params) != 2 {
		t.Fatalf("parameter count = %d, want 2", len(fn.Params))
	}

	first := fn.Params[0]
	if first.Name != "b" || first.Type.Name != "String" || !first.Type.Nullable {
		t.Errorf("unexpected first parameter: %#v", first)
	}
	if _, ok := first.Default.(*ast.NullLit); !ok {
		t.Errorf("first default = %T, want *ast.NullLit", first.Default)
	}

	second := fn.Params[1]
	if second.Name != "a" || second.Type.Name != "Number" || second.Type.Nullable {
		t.Errorf("unexpected second parameter: %#v", second)
	}
	if lit, ok := second.Default.(*ast.NumberLit); !ok || lit.Raw != "10" {
		t.Errorf("second default = %#v, want number 10", second.Default)
	}

	if fn.ReturnType == nil || fn.ReturnType.Name != "String" || !fn.ReturnType.Nullable {
		t.Errorf("return type = %#v, want String?", fn.ReturnType)
	}
	if len(fn.Body.Statements) != 1 {
		t.Fatalf("body statement count = %d, want 1", len(fn.Body.Statements))
	}
	ret, ok := fn.Body.Statements[0].(*ast.ReturnStmt)
	if !ok {
		t.Fatalf("body statement is %T, want *ast.ReturnStmt", fn.Body.Statements[0])
	}
	if ident, ok := ret.Value.(*ast.Ident); !ok || ident.Name != "b" {
		t.Errorf("return value = %#v, want identifier b", ret.Value)
	}
}

func TestParseVariadicParameter(t *testing.T) {
	fn := mustParse(t, `fun sum(numbers: Number...): Number { return numbers[0] }`).Statements[0].(*ast.FunctionDecl)
	if len(fn.Params) != 1 {
		t.Fatalf("parameter count = %d, want 1", len(fn.Params))
	}
	param := fn.Params[0]
	if !param.Variadic || param.Type.Name != "Number" {
		t.Errorf("parameter = %#v, want variadic Number", param)
	}
}

func TestParseCalls(t *testing.T) {
	src := `fun example(b:String? = null, a:Number = 10) {}
	example(b="test", a=10)
	example("test", 10)
	example()`
	program := mustParse(t, src)
	if len(program.Statements) != 4 {
		t.Fatalf("statement count = %d, want 4", len(program.Statements))
	}

	named := program.Statements[1].(*ast.ExprStmt).X.(*ast.CallExpr)
	if named.Callee != "example" || len(named.Args) != 2 {
		t.Fatalf("unexpected named call: %#v", named)
	}
	if named.Args[0].Name != "b" || named.Args[1].Name != "a" {
		t.Errorf("named arguments = %q, %q", named.Args[0].Name, named.Args[1].Name)
	}

	positional := program.Statements[2].(*ast.ExprStmt).X.(*ast.CallExpr)
	if len(positional.Args) != 2 || positional.Args[0].Name != "" || positional.Args[1].Name != "" {
		t.Errorf("unexpected positional call: %#v", positional)
	}

	empty := program.Statements[3].(*ast.ExprStmt).X.(*ast.CallExpr)
	if len(empty.Args) != 0 {
		t.Errorf("empty call has %d arguments", len(empty.Args))
	}
}

func TestParseCallInInitializer(t *testing.T) {
	decl := mustParse(t, `const x = f();`).Statements[0].(*ast.VarDecl)
	call, ok := decl.Value.(*ast.CallExpr)
	if !ok || call.Callee != "f" {
		t.Errorf("initializer = %#v, want call to f", decl.Value)
	}
}

func TestParseBareReturnAndBlocks(t *testing.T) {
	fn := mustParse(t, `fun f() { return; { const x = 1; } }`).Statements[0].(*ast.FunctionDecl)
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("body statement count = %d, want 2", len(fn.Body.Statements))
	}
	ret, ok := fn.Body.Statements[0].(*ast.ReturnStmt)
	if !ok || ret.Value != nil {
		t.Errorf("first body statement = %#v, want bare return", fn.Body.Statements[0])
	}
	if _, ok := fn.Body.Statements[1].(*ast.BlockStmt); !ok {
		t.Errorf("second body statement = %T, want *ast.BlockStmt", fn.Body.Statements[1])
	}
}

func TestParseOptionalSemicolons(t *testing.T) {
	program := mustParse(t, "const a = 1\nfun f(){ return }\nf()")
	if len(program.Statements) != 3 {
		t.Fatalf("statement count = %d, want 3", len(program.Statements))
	}
}

func TestParseNamedArgumentErrors(t *testing.T) {
	tokens, err := lexer.New("test.lynx", `fun f(a:Number) {} f(a=1, 2)`).Lex()
	if err != nil {
		t.Fatalf("lex error: %v", err)
	}
	_, err = Parse(tokens)
	if err == nil {
		t.Fatal("Parse() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "positional argument cannot follow a named argument") {
		t.Errorf("error = %q", err)
	}
}

func TestParseLambda(t *testing.T) {
	program := mustParse(t, `val sum = { x: Int, y: Int -> x + y }`)
	if len(program.Statements) != 1 {
		t.Fatalf("statement count = %d, want 1", len(program.Statements))
	}

	decl, ok := program.Statements[0].(*ast.VarDecl)
	if !ok {
		t.Fatalf("statement is %T, want *ast.VarDecl", program.Statements[0])
	}
	if !decl.Const || decl.Name != "sum" {
		t.Errorf("declaration = %#v, want const sum", decl)
	}

	lambda, ok := decl.Value.(*ast.LambdaExpr)
	if !ok {
		t.Fatalf("initializer is %T, want *ast.LambdaExpr", decl.Value)
	}
	if len(lambda.Params) != 2 {
		t.Fatalf("parameter count = %d, want 2", len(lambda.Params))
	}
	if lambda.Params[0].Name != "x" || lambda.Params[0].Type.Name != "Int" {
		t.Errorf("first parameter = %#v", lambda.Params[0])
	}
	if lambda.Params[1].Name != "y" || lambda.Params[1].Type.Name != "Int" {
		t.Errorf("second parameter = %#v", lambda.Params[1])
	}

	binary, ok := lambda.Body.(*ast.BinaryExpr)
	if !ok || binary.Op != token.Plus {
		t.Fatalf("body = %#v, want x + y", lambda.Body)
	}
}

func TestParseEmptyParameterLambda(t *testing.T) {
	decl := mustParse(t, `const answer = { -> 42 }`).Statements[0].(*ast.VarDecl)
	lambda, ok := decl.Value.(*ast.LambdaExpr)
	if !ok {
		t.Fatalf("initializer is %T, want *ast.LambdaExpr", decl.Value)
	}
	if len(lambda.Params) != 0 {
		t.Errorf("parameter count = %d, want 0", len(lambda.Params))
	}
}

func TestParseFunctionType(t *testing.T) {
	fn := mustParse(t, `fun apply(f: (Number, Number) -> Number): Number { return f(1, 2) }`).Statements[0].(*ast.FunctionDecl)
	if len(fn.Params) != 1 {
		t.Fatalf("parameter count = %d, want 1", len(fn.Params))
	}

	typ := fn.Params[0].Type
	if typ == nil || !typ.Function {
		t.Fatalf("parameter type = %#v, want function type", typ)
	}
	if len(typ.Params) != 2 || typ.Params[0].Name != "Number" || typ.Params[1].Name != "Number" {
		t.Errorf("function type parameters = %#v", typ.Params)
	}
	if typ.Returns == nil || typ.Returns.Name != "Number" {
		t.Errorf("function type return = %#v, want Number", typ.Returns)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"unterminated function body", `fun f() {`, "unterminated block"},
		{"const without value", `const a;`, "const declaration requires an initializer"},
		{"var without type or value", `var a;`, "requires a type annotation or an initializer"},
		{"missing type", `var a: = 1;`, "expected type name"},
		{"missing expression", `const a = ;`, "expected expression"},
		{"missing bracket", `const a = [1, 2;`, "expected ']'"},
		{"missing paren", `const a = (1;`, "expected ')'"},
		{"bare expression", `1 + 2;`, "expected a statement"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lexer.New("test.lynx", tt.src).Lex()
			if err != nil {
				t.Fatalf("lex error: %v", err)
			}
			_, err = Parse(tokens)
			if err == nil {
				t.Fatal("Parse() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
