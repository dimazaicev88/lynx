package parser

import (
	"testing"

	"lynx/internal/ast"
	"lynx/internal/token"
)

func TestParseControlFlow(t *testing.T) {
	program := mustParse(t, `if ready { println("yes") } else if false { break } else { continue }
for var i = 0; i < 3; i = i + 1 { println(toString(i)) }
for ready { break }
for { break }
switch value { case 1, 2: fallthrough default: break }`)
	if len(program.Statements) != 5 {
		t.Fatalf("statement count = %d, want 5", len(program.Statements))
	}
	if stmt, ok := program.Statements[0].(*ast.IfStmt); !ok || stmt.Else == nil {
		t.Fatalf("first statement = %#v, want if with else", program.Statements[0])
	}
	loop := program.Statements[1].(*ast.ForStmt)
	if loop.Init == nil || loop.Cond == nil || loop.Post == nil {
		t.Errorf("three-part loop = %#v", loop)
	}
	conditionLoop := program.Statements[2].(*ast.ForStmt)
	if conditionLoop.Cond == nil || conditionLoop.Init != nil || conditionLoop.Post != nil {
		t.Errorf("condition loop = %#v", conditionLoop)
	}
	infinite := program.Statements[3].(*ast.ForStmt)
	if infinite.Cond != nil || infinite.Init != nil || infinite.Post != nil {
		t.Errorf("infinite loop = %#v", infinite)
	}
	switchStmt := program.Statements[4].(*ast.SwitchStmt)
	if len(switchStmt.Cases) != 2 || len(switchStmt.Cases[0].Values) != 2 || !switchStmt.Cases[1].IsDefault {
		t.Errorf("switch = %#v", switchStmt)
	}
}

func TestParseBooleanPrecedenceAndTypeTest(t *testing.T) {
	decl := mustParse(t, `val result = !false || value is Number && value != null`).Statements[0].(*ast.VarDecl)
	or, ok := decl.Value.(*ast.BinaryExpr)
	if !ok || or.Op != token.OrOr {
		t.Fatalf("root = %#v, want ||", decl.Value)
	}
	and, ok := or.Right.(*ast.BinaryExpr)
	if !ok || and.Op != token.AndAnd {
		t.Fatalf("right = %#v, want &&", or.Right)
	}
	if _, ok := and.Left.(*ast.TypeTestExpr); !ok {
		t.Errorf("left side of && = %T, want type test", and.Left)
	}
}

func TestParseGenericArrayType(t *testing.T) {
	decl := mustParse(t, `val values: Array<Array<Number>?> = []`).Statements[0].(*ast.VarDecl)
	typ := decl.Type
	if typ == nil || typ.Name != "Array" || typ.Elem == nil {
		t.Fatalf("type = %#v", typ)
	}
	if typ.Elem.Name != "Array" || !typ.Elem.Nullable || typ.Elem.Elem == nil || typ.Elem.Elem.Name != "Number" {
		t.Errorf("element type = %#v", typ.Elem)
	}
}
