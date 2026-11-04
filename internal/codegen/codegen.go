// Package codegen emits JavaScript from a checked Lynx syntax tree.
package codegen

import (
	"fmt"
	"strings"

	"lynx/internal/ast"
	"lynx/internal/token"
)

const (
	precLambda = iota + 1
	precOr
	precAnd
	precEquality
	precComparison
	precAdditive
	precMultiplicative
	precUnary
	precPostfix
	precPrimary
)

const indentUnit = "  "

// Generate translates a parsed Lynx program to JavaScript.
func Generate(program *ast.Program) string {
	g := &generator{functions: collectFunctions(program)}
	for _, stmt := range program.Statements {
		g.writeStmt(stmt)
	}
	return g.builder.String()
}

type generator struct {
	builder   strings.Builder
	indent    int
	functions map[string]*ast.FunctionDecl
}

func collectFunctions(program *ast.Program) map[string]*ast.FunctionDecl {
	functions := make(map[string]*ast.FunctionDecl)
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*ast.FunctionDecl); ok {
			functions[fn.Name] = fn
		}
	}
	return functions
}

func (g *generator) writeStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.VarDecl:
		g.writeIndent()
		g.writeVarDecl(s)
		g.builder.WriteString(";\n")
	case *ast.AssignStmt:
		g.writeIndent()
		g.writeAssign(s)
		g.builder.WriteString(";\n")
	case *ast.ExprStmt:
		g.writeIndent()
		g.writeExpr(s.X)
		g.builder.WriteString(";\n")
	case *ast.ReturnStmt:
		g.writeIndent()
		g.builder.WriteString("return")
		if s.Value != nil {
			g.builder.WriteByte(' ')
			g.writeExpr(s.Value)
		}
		g.builder.WriteString(";\n")
	case *ast.BlockStmt:
		g.writeIndent()
		g.writeBlock(s)
		g.builder.WriteByte('\n')
	case *ast.IfStmt:
		g.writeIndent()
		g.writeIf(s)
	case *ast.ForStmt:
		g.writeIndent()
		g.writeFor(s)
	case *ast.SwitchStmt:
		g.writeSwitch(s)
	case *ast.BreakStmt:
		g.writeIndent()
		g.builder.WriteString("break;\n")
	case *ast.ContinueStmt:
		g.writeIndent()
		g.builder.WriteString("continue;\n")
	case *ast.FallthroughStmt:
	case *ast.FunctionDecl:
		g.writeFunction(s)
	}
}

func (g *generator) writeVarDecl(decl *ast.VarDecl) {
	if decl.Const {
		g.builder.WriteString("const ")
	} else {
		g.builder.WriteString("let ")
	}
	g.builder.WriteString(decl.Name)
	if decl.Value != nil {
		g.builder.WriteString(" = ")
		g.writeExpr(decl.Value)
	}
}

func (g *generator) writeAssign(stmt *ast.AssignStmt) {
	g.builder.WriteString(stmt.Name)
	g.builder.WriteString(" = ")
	g.writeExpr(stmt.Value)
}

func (g *generator) writeBlock(block *ast.BlockStmt) {
	g.builder.WriteString("{\n")
	g.indent++
	for _, stmt := range block.Statements {
		g.writeStmt(stmt)
	}
	g.indent--
	g.writeIndent()
	g.builder.WriteByte('}')
}

func (g *generator) writeIf(stmt *ast.IfStmt) {
	g.builder.WriteString("if (")
	g.writeExpr(stmt.Cond)
	g.builder.WriteString(") ")
	g.writeBlock(stmt.Then)
	if stmt.Else == nil {
		g.builder.WriteByte('\n')
		return
	}
	g.builder.WriteString(" else ")
	switch branch := stmt.Else.(type) {
	case *ast.IfStmt:
		g.writeIf(branch)
	case *ast.BlockStmt:
		g.writeBlock(branch)
		g.builder.WriteByte('\n')
	}
}

func (g *generator) writeFor(stmt *ast.ForStmt) {
	if stmt.Init == nil && stmt.Cond == nil && stmt.Post == nil {
		g.builder.WriteString("for (;;) ")
		g.writeBlock(stmt.Body)
		g.builder.WriteByte('\n')
		return
	}

	g.builder.WriteString("for (")
	if stmt.Init != nil {
		switch init := stmt.Init.(type) {
		case *ast.VarDecl:
			g.writeVarDecl(init)
		case *ast.AssignStmt:
			g.writeAssign(init)
		}
	}
	g.builder.WriteString("; ")
	if stmt.Cond != nil {
		g.writeExpr(stmt.Cond)
	}
	g.builder.WriteString("; ")
	if stmt.Post != nil {
		switch post := stmt.Post.(type) {
		case *ast.AssignStmt:
			g.writeAssign(post)
		case *ast.ExprStmt:
			g.writeExpr(post.X)
		}
	}
	g.builder.WriteString(") ")
	g.writeBlock(stmt.Body)
	g.builder.WriteByte('\n')
}

func (g *generator) writeSwitch(stmt *ast.SwitchStmt) {
	g.writeIndent()
	g.builder.WriteString("switch (")
	if stmt.Value != nil {
		g.writeExpr(stmt.Value)
	} else {
		g.builder.WriteString("true")
	}
	g.builder.WriteString(") {\n")
	g.indent++
	for _, clause := range stmt.Cases {
		if clause.IsDefault {
			g.writeIndent()
			g.builder.WriteString("default: {\n")
		} else {
			for j, value := range clause.Values {
				g.writeIndent()
				g.builder.WriteString("case ")
				g.writeExpr(value)
				if j+1 < len(clause.Values) {
					g.builder.WriteString(":\n")
				} else {
					g.builder.WriteString(": {\n")
				}
			}
		}
		g.indent++
		for _, inner := range clause.Body {
			g.writeStmt(inner)
		}
		if !clauseEndsWithoutBreak(clause.Body) {
			g.writeIndent()
			g.builder.WriteString("break;\n")
		}
		g.indent--
		g.writeIndent()
		g.builder.WriteString("}\n")
	}
	g.indent--
	g.writeIndent()
	g.builder.WriteString("}\n")
}

func clauseEndsWithoutBreak(statements []ast.Stmt) bool {
	if len(statements) == 0 {
		return false
	}
	switch statements[len(statements)-1].(type) {
	case *ast.ReturnStmt, *ast.BreakStmt, *ast.ContinueStmt, *ast.FallthroughStmt:
		return true
	default:
		return false
	}
}

func (g *generator) writeFunction(fn *ast.FunctionDecl) {
	g.writeIndent()
	g.builder.WriteString("function ")
	g.builder.WriteString(fn.Name)
	g.builder.WriteByte('(')
	for i, param := range fn.Params {
		if i > 0 {
			g.builder.WriteString(", ")
		}
		if param.Variadic {
			g.builder.WriteString("...")
		}
		g.builder.WriteString(param.Name)
		if param.Default != nil {
			g.builder.WriteString(" = ")
			g.writeExpr(param.Default)
		}
	}
	g.builder.WriteString(") ")
	g.writeBlock(fn.Body)
	g.builder.WriteByte('\n')
}

func (g *generator) writeExpr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.NumberLit:
		g.builder.WriteString(e.Raw)
	case *ast.StringLit:
		g.builder.WriteString(quoteJS(e.Value))
	case *ast.BoolLit:
		if e.Value {
			g.builder.WriteString("true")
		} else {
			g.builder.WriteString("false")
		}
	case *ast.NullLit:
		g.builder.WriteString("null")
	case *ast.Ident:
		g.builder.WriteString(e.Name)
	case *ast.ArrayLit:
		g.builder.WriteByte('[')
		for i, element := range e.Elements {
			if i > 0 {
				g.builder.WriteString(", ")
			}
			g.writeExpr(element)
		}
		g.builder.WriteByte(']')
	case *ast.UnaryExpr:
		g.builder.WriteString(string(e.Op))
		if _, nested := e.X.(*ast.UnaryExpr); nested || precedence(e.X) < precUnary {
			g.builder.WriteByte('(')
			g.writeExpr(e.X)
			g.builder.WriteByte(')')
		} else {
			g.writeExpr(e.X)
		}
	case *ast.BinaryExpr:
		prec := precedence(e)
		g.writeOperand(e.Left, prec, false)
		g.builder.WriteByte(' ')
		g.builder.WriteString(binaryOperator(e.Op))
		g.builder.WriteByte(' ')
		g.writeOperand(e.Right, prec, true)
	case *ast.TypeTestExpr:
		g.writeTypeTest(e)
	case *ast.IndexExpr:
		g.builder.WriteByte('(')
		g.writeOperand(e.X, precPostfix, false)
		g.builder.WriteByte('[')
		g.writeExpr(e.Index)
		g.builder.WriteString("] ?? null)")
	case *ast.MemberExpr:
		g.writeOperand(e.X, precPostfix, false)
		g.builder.WriteByte('.')
		g.builder.WriteString(e.Name)
	case *ast.CallExpr:
		g.writeCall(e)
	case *ast.LambdaExpr:
		g.writeLambda(e)
	}
}

func binaryOperator(op token.Type) string {
	switch op {
	case token.Eq, token.StrictEq:
		return "==="
	case token.NotEq, token.StrictNe:
		return "!=="
	default:
		return string(op)
	}
}

func (g *generator) writeTypeTest(expr *ast.TypeTestExpr) {
	if expr.Negate {
		g.builder.WriteByte('!')
	}
	g.builder.WriteByte('(')
	name := expr.Type.Name
	switch {
	case expr.Type.Function:
		g.builder.WriteString("typeof ")
		g.writeExpr(expr.X)
		g.builder.WriteString(" === \"function\"")
	case name == "Number" || name == "Int" || name == "String" || name == "Boolean":
		g.builder.WriteString("typeof ")
		g.writeExpr(expr.X)
		g.builder.WriteString(" === \"")
		if name == "Number" || name == "Int" {
			g.builder.WriteString("number")
		} else if name == "String" {
			g.builder.WriteString("string")
		} else {
			g.builder.WriteString("boolean")
		}
		g.builder.WriteByte('"')
	case name == "Array":
		g.builder.WriteString("Array.isArray(")
		g.writeExpr(expr.X)
		g.builder.WriteByte(')')
	case name == "Null":
		g.writeExpr(expr.X)
		g.builder.WriteString(" === null")
	case name == "Any" || name == "Unknown":
		g.builder.WriteString("true")
	default:
		g.builder.WriteString("false")
	}
	g.builder.WriteByte(')')
}

func (g *generator) writeLambda(lambda *ast.LambdaExpr) {
	g.builder.WriteByte('(')
	for i, param := range lambda.Params {
		if i > 0 {
			g.builder.WriteString(", ")
		}
		g.builder.WriteString(param.Name)
	}
	g.builder.WriteString(") => ")
	g.writeExpr(lambda.Body)
}

func (g *generator) writeCall(call *ast.CallExpr) {
	switch call.Callee {
	case "println":
		g.builder.WriteString("console.log")
	case "toString":
		g.builder.WriteString("String")
	default:
		g.builder.WriteString(call.Callee)
	}
	g.builder.WriteByte('(')
	g.writeCallArgs(call)
	g.builder.WriteByte(')')
}

func (g *generator) writeCallArgs(call *ast.CallExpr) {
	hasNamed := false
	for _, arg := range call.Args {
		if arg.Name != "" {
			hasNamed = true
			break
		}
	}
	fn, hasFunction := g.functions[call.Callee]
	if !hasNamed || !hasFunction {
		g.writeArgumentValues(call.Args)
		return
	}

	positional := make([]*ast.CallArg, 0, len(call.Args))
	named := make(map[string]ast.Expr)
	for _, arg := range call.Args {
		if arg.Name == "" {
			positional = append(positional, arg)
		} else {
			named[arg.Name] = arg.Value
		}
	}

	paramIndex := make(map[string]int)
	last := len(positional) - 1
	for i, param := range fn.Params {
		paramIndex[param.Name] = i
	}
	for name := range named {
		if index, ok := paramIndex[name]; ok && index > last {
			last = index
		}
	}

	first := true
	writeValue := func(value ast.Expr) {
		if !first {
			g.builder.WriteString(", ")
		}
		first = false
		if value == nil {
			g.builder.WriteString("undefined")
			return
		}
		g.writeExpr(value)
	}
	for i := 0; i <= last; i++ {
		if i < len(positional) {
			writeValue(positional[i].Value)
			continue
		}
		if i < len(fn.Params) {
			if value, ok := named[fn.Params[i].Name]; ok {
				writeValue(value)
				continue
			}
		}
		writeValue(nil)
	}
}

func (g *generator) writeArgumentValues(args []*ast.CallArg) {
	for i, arg := range args {
		if i > 0 {
			g.builder.WriteString(", ")
		}
		g.writeExpr(arg.Value)
	}
}

func (g *generator) writeOperand(expr ast.Expr, parentPrecedence int, right bool) {
	prec := precedence(expr)
	needParens := prec < parentPrecedence || (right && prec == parentPrecedence)
	if needParens {
		g.builder.WriteByte('(')
		g.writeExpr(expr)
		g.builder.WriteByte(')')
		return
	}
	g.writeExpr(expr)
}

func (g *generator) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.builder.WriteString(indentUnit)
	}
}

func precedence(expr ast.Expr) int {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		switch e.Op {
		case token.OrOr:
			return precOr
		case token.AndAnd:
			return precAnd
		case token.Eq, token.NotEq, token.StrictEq, token.StrictNe:
			return precEquality
		case token.Lt, token.LtEq, token.Gt, token.GtEq:
			return precComparison
		case token.Plus, token.Minus:
			return precAdditive
		default:
			return precMultiplicative
		}
	case *ast.TypeTestExpr:
		return precComparison
	case *ast.UnaryExpr:
		return precUnary
	case *ast.LambdaExpr:
		return precLambda
	case *ast.IndexExpr, *ast.MemberExpr, *ast.CallExpr:
		return precPostfix
	default:
		return precPrimary
	}
}

func quoteJS(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\v':
			b.WriteString(`\v`)
		case '\u2028':
			b.WriteString(`\u2028`)
		case '\u2029':
			b.WriteString(`\u2029`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
