// Package codegen emits JavaScript from a checked Lynx syntax tree.
package codegen

import (
	"fmt"
	"strings"

	"lynx/internal/ast"
	"lynx/internal/token"
)

const (
	precLambda         = 1
	precAdditive       = 2
	precMultiplicative = 3
	precUnary          = 4
	precPostfix        = 5
	precPrimary        = 6
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
		if s.Const {
			g.builder.WriteString("const ")
		} else {
			g.builder.WriteString("let ")
		}
		g.builder.WriteString(s.Name)
		if s.Value != nil {
			g.builder.WriteString(" = ")
			g.writeExpr(s.Value)
		}
		g.builder.WriteString(";\n")
	case *ast.AssignStmt:
		g.writeIndent()
		g.builder.WriteString(s.Name)
		g.builder.WriteString(" = ")
		g.writeExpr(s.Value)
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
		g.builder.WriteString("{\n")
		g.indent++
		for _, inner := range s.Statements {
			g.writeStmt(inner)
		}
		g.indent--
		g.writeIndent()
		g.builder.WriteString("}\n")
	case *ast.FunctionDecl:
		g.writeFunction(s)
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
	g.builder.WriteString(") {\n")
	g.indent++
	for _, stmt := range fn.Body.Statements {
		g.writeStmt(stmt)
	}
	g.indent--
	g.writeIndent()
	g.builder.WriteString("}\n")
}

func (g *generator) writeExpr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.NumberLit:
		g.builder.WriteString(e.Raw)
	case *ast.StringLit:
		g.builder.WriteString(quoteJS(e.Value))
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
		g.builder.WriteString(string(e.Op))
		g.builder.WriteByte(' ')
		g.writeOperand(e.Right, prec, true)
	case *ast.IndexExpr:
		g.writeOperand(e.X, precPostfix, false)
		g.builder.WriteByte('[')
		g.writeExpr(e.Index)
		g.builder.WriteByte(']')
	case *ast.MemberExpr:
		g.writeOperand(e.X, precPostfix, false)
		g.builder.WriteByte('.')
		g.builder.WriteString(e.Name)
	case *ast.CallExpr:
		g.builder.WriteString(e.Callee)
		g.builder.WriteByte('(')
		g.writeCallArgs(e)
		g.builder.WriteByte(')')
	case *ast.LambdaExpr:
		g.writeLambda(e)
	}
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

	paramIndex := make(map[string]int, len(fn.Params))
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
		if e.Op == token.Plus || e.Op == token.Minus {
			return precAdditive
		}
		return precMultiplicative
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
