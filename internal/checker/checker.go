// Package checker implements the strict Lynx type system.
package checker

import (
	"fmt"
	"strings"

	"lynx/internal/ast"
	"lynx/internal/diag"
	"lynx/internal/token"
)

// Kind is the kind of a Lynx type.
type Kind int

const (
	Invalid Kind = iota
	Number
	String
	Array
	Any
	Unknown
	Null
	Void
	Function
)

// Type is a Lynx value type. Arrays carry their element type internally so
// that indexing can be checked precisely, while the source annotation is
// simply Array. Functions carry their parameter and return types. Nullable
// records a trailing ? in a source annotation.
type Type struct {
	Kind     Kind
	Elem     *Type
	Params   []Type
	Returns  *Type
	Nullable bool
}

var (
	NumberType  = Type{Kind: Number}
	StringType  = Type{Kind: String}
	AnyType     = Type{Kind: Any}
	UnknownType = Type{Kind: Unknown}
	NullType    = Type{Kind: Null}
	VoidType    = Type{Kind: Void}
)

// ArrayOf returns an array type with the given element type.
func ArrayOf(elem Type) Type {
	return Type{Kind: Array, Elem: &elem}
}

// NullableOf returns a nullable version of t.
func NullableOf(t Type) Type {
	if t.Kind == Null {
		return t
	}
	t.Nullable = true
	return t
}

func (t Type) String() string {
	var base string
	switch t.Kind {
	case Number:
		base = "Number"
	case String:
		base = "String"
	case Array:
		if t.Elem == nil || t.Elem.Kind == Any {
			base = "Array"
		} else {
			base = fmt.Sprintf("Array<%s>", t.Elem.String())
		}
	case Any:
		base = "Any"
	case Unknown:
		base = "Unknown"
	case Null:
		base = "Null"
	case Void:
		base = "Void"
	case Function:
		parts := make([]string, len(t.Params))
		for i, param := range t.Params {
			parts[i] = param.String()
		}
		returns := "Any"
		if t.Returns != nil {
			returns = t.Returns.String()
		}
		base = fmt.Sprintf("(%s) -> %s", strings.Join(parts, ", "), returns)
	default:
		base = "Invalid"
	}
	if t.Nullable {
		base += "?"
	}
	return base
}

// Equal reports whether two types are identical.
func Equal(a, b Type) bool {
	if a.Kind != b.Kind || a.Nullable != b.Nullable {
		return false
	}
	switch a.Kind {
	case Array:
		if a.Elem == nil || b.Elem == nil {
			return a.Elem == b.Elem
		}
		return Equal(*a.Elem, *b.Elem)
	case Function:
		if len(a.Params) != len(b.Params) {
			return false
		}
		for i := range a.Params {
			if !Equal(a.Params[i], b.Params[i]) {
				return false
			}
		}
		if a.Returns == nil || b.Returns == nil {
			return a.Returns == b.Returns
		}
		return Equal(*a.Returns, *b.Returns)
	default:
		return true
	}
}

type variable struct {
	typ      Type
	constant bool
	pos      token.Position
}

type paramInfo struct {
	name       string
	typ        Type
	hasDefault bool
	variadic   bool
	pos        token.Position
}

type functionSignature struct {
	name     string
	params   []paramInfo
	byName   map[string]int
	variadic int // index of the variadic parameter, or -1
	returns  Type
	pos      token.Position
}

// Checker validates declarations, assignments, calls and return statements.
type Checker struct {
	scopes        []map[string]variable
	functions     map[string]*functionSignature
	signatures    map[*ast.FunctionDecl]*functionSignature
	errs          diag.List
	currentReturn *Type
}

// Check type-checks a program. It returns all diagnostics found.
func Check(program *ast.Program) error {
	c := &Checker{
		functions:  make(map[string]*functionSignature),
		signatures: make(map[*ast.FunctionDecl]*functionSignature),
	}
	c.pushScope()

	c.collectFunctions(program)
	for _, stmt := range program.Statements {
		if _, ok := stmt.(*ast.FunctionDecl); ok {
			continue
		}
		c.checkStmt(stmt)
	}
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*ast.FunctionDecl); ok {
			c.checkFunction(fn)
		}
	}

	c.popScope()
	if !c.errs.Empty() {
		return &c.errs
	}
	return nil
}

func (c *Checker) collectFunctions(program *ast.Program) {
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDecl)
		if !ok {
			continue
		}
		sig := c.buildSignature(fn)
		c.signatures[fn] = sig
		if _, exists := c.functions[fn.Name]; exists {
			c.errs.Add(diag.New(fn.NamePos, "function %q is already declared", fn.Name))
			continue
		}
		c.functions[fn.Name] = sig
	}
}

func (c *Checker) buildSignature(fn *ast.FunctionDecl) *functionSignature {
	sig := &functionSignature{
		name:     fn.Name,
		byName:   make(map[string]int),
		variadic: -1,
		returns:  VoidType,
		pos:      fn.NamePos,
	}

	seen := make(map[string]bool)
	for i, param := range fn.Params {
		var typ Type
		if param.Type == nil {
			c.errs.Add(diag.New(param.Position, "parameter %q requires a type annotation", param.Name))
			typ = AnyType
		} else {
			var ok bool
			typ, ok = c.resolveType(param.Type)
			if !ok {
				typ = AnyType
			}
		}

		if seen[param.Name] {
			c.errs.Add(diag.New(param.NamePos, "parameter %q is already declared", param.Name))
		}
		seen[param.Name] = true

		info := paramInfo{
			name:       param.Name,
			typ:        typ,
			hasDefault: param.Default != nil,
			variadic:   param.Variadic,
			pos:        param.Position,
		}
		if param.Variadic {
			if sig.variadic >= 0 {
				c.errs.Add(diag.New(param.Position, "only one variadic parameter is allowed"))
			}
			if i != len(fn.Params)-1 {
				c.errs.Add(diag.New(param.Position, "variadic parameter %q must be last", param.Name))
			}
			if param.Default != nil {
				c.errs.Add(diag.New(param.Position, "variadic parameter %q cannot have a default value", param.Name))
			}
			sig.variadic = i
		}

		if _, exists := sig.byName[param.Name]; !exists {
			sig.byName[param.Name] = i
		}
		sig.params = append(sig.params, info)
	}

	if fn.ReturnType != nil {
		if typ, ok := c.resolveType(fn.ReturnType); ok {
			sig.returns = typ
		} else {
			sig.returns = AnyType
		}
	}

	return sig
}

func (c *Checker) checkFunction(fn *ast.FunctionDecl) {
	sig, ok := c.signatures[fn]
	if !ok {
		return
	}

	c.pushScope()
	savedReturn := c.currentReturn
	returns := sig.returns
	c.currentReturn = &returns

	for i, param := range fn.Params {
		info := sig.params[i]

		if param.Default != nil {
			defaultType := c.checkExpr(param.Default)
			if defaultType.Kind == Void {
				c.errs.Add(diag.New(param.Default.Pos(), "default value does not return a value"))
			} else if !info.variadic && !assignable(defaultType, info.typ) {
				c.errs.Add(diag.New(
					param.Default.Pos(),
					"default value of type %s is not assignable to parameter %q of type %s",
					defaultType.String(), param.Name, info.typ.String(),
				))
			}
		}

		bodyType := info.typ
		if info.variadic {
			bodyType = ArrayOf(bodyType)
		}
		if _, exists := c.functions[param.Name]; exists {
			c.errs.Add(diag.New(
				param.NamePos,
				"parameter %q conflicts with function %q",
				param.Name, param.Name,
			))
		}
		c.scopes[len(c.scopes)-1][param.Name] = variable{typ: bodyType, pos: param.NamePos}
	}

	for _, stmt := range fn.Body.Statements {
		c.checkStmt(stmt)
	}

	if sig.returns.Kind != Void && !blockReturns(fn.Body) {
		c.errs.Add(diag.New(
			fn.NamePos,
			"function %q with return type %s does not return a value",
			fn.Name, sig.returns.String(),
		))
	}

	c.currentReturn = savedReturn
	c.popScope()
}

func (c *Checker) checkStmt(stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.VarDecl:
		c.checkVarDecl(s)
	case *ast.AssignStmt:
		c.checkAssign(s)
	case *ast.ExprStmt:
		c.checkExpr(s.X)
	case *ast.ReturnStmt:
		c.checkReturn(s)
	case *ast.BlockStmt:
		c.pushScope()
		for _, inner := range s.Statements {
			c.checkStmt(inner)
		}
		c.popScope()
	case *ast.FunctionDecl:
		c.errs.Add(diag.New(s.NamePos, "function declarations are only allowed at the top level"))
	}
}

func (c *Checker) checkVarDecl(decl *ast.VarDecl) {
	var annotation *Type
	if decl.Type != nil {
		typ, ok := c.resolveType(decl.Type)
		if !ok {
			typ = AnyType
		}
		annotation = &typ
	}

	var valueType Type
	hasValue := decl.Value != nil
	if hasValue {
		valueType = c.checkExpr(decl.Value)
		if valueType.Kind == Void {
			c.errs.Add(diag.New(decl.Value.Pos(), "cannot assign a value that does not exist (function returns Void)"))
		}
	}

	finalType := AnyType
	if annotation != nil {
		if hasValue && !assignable(valueType, *annotation) {
			c.errs.Add(diag.New(
				decl.Value.Pos(),
				"cannot assign a value of type %s to variable %q of type %s",
				valueType.String(), decl.Name, annotation.String(),
			))
		}
		finalType = *annotation
	} else if hasValue {
		finalType = valueType
	}

	if _, exists := c.scopes[len(c.scopes)-1][decl.Name]; exists {
		c.errs.Add(diag.New(decl.NamePos, "variable %q is already declared", decl.Name))
		return
	}
	if _, exists := c.functions[decl.Name]; exists {
		c.errs.Add(diag.New(decl.NamePos, "variable %q conflicts with function %q", decl.Name, decl.Name))
		return
	}

	c.scopes[len(c.scopes)-1][decl.Name] = variable{
		typ:      finalType,
		constant: decl.Const,
		pos:      decl.NamePos,
	}
}

func (c *Checker) checkAssign(stmt *ast.AssignStmt) {
	valueType := c.checkExpr(stmt.Value)
	if valueType.Kind == Void {
		c.errs.Add(diag.New(stmt.Value.Pos(), "cannot assign a value that does not exist (function returns Void)"))
	}

	info, exists := c.lookup(stmt.Name)
	if !exists {
		c.errs.Add(diag.New(stmt.NamePos, "undefined variable %q", stmt.Name))
		return
	}
	if info.constant {
		c.errs.Add(diag.New(stmt.NamePos, "cannot assign to constant %q", stmt.Name))
		return
	}
	if !assignable(valueType, info.typ) {
		c.errs.Add(diag.New(
			stmt.Value.Pos(),
			"cannot assign a value of type %s to variable %q of type %s",
			valueType.String(), stmt.Name, info.typ.String(),
		))
	}
}

func (c *Checker) checkReturn(stmt *ast.ReturnStmt) {
	if c.currentReturn == nil {
		c.errs.Add(diag.New(stmt.Position, "return statement outside a function"))
		if stmt.Value != nil {
			c.checkExpr(stmt.Value)
		}
		return
	}

	returnType := *c.currentReturn
	if stmt.Value == nil {
		if returnType.Kind != Void {
			c.errs.Add(diag.New(
				stmt.Position,
				"function with return type %s must return a value",
				returnType.String(),
			))
		}
		return
	}

	valueType := c.checkExpr(stmt.Value)
	if returnType.Kind == Void {
		c.errs.Add(diag.New(stmt.Position, "cannot return a value from a function without a return type"))
		return
	}
	if valueType.Kind == Void {
		c.errs.Add(diag.New(stmt.Value.Pos(), "returned expression does not produce a value"))
		return
	}
	if !assignable(valueType, returnType) {
		c.errs.Add(diag.New(
			stmt.Value.Pos(),
			"cannot return a value of type %s from a function returning %s",
			valueType.String(), returnType.String(),
		))
	}
}

func (c *Checker) checkExpr(expr ast.Expr) Type {
	switch e := expr.(type) {
	case *ast.NumberLit:
		return NumberType
	case *ast.StringLit:
		return StringType
	case *ast.NullLit:
		return NullType
	case *ast.Ident:
		info, exists := c.lookup(e.Name)
		if !exists {
			c.errs.Add(diag.New(e.Position, "undefined variable %q", e.Name))
			return AnyType
		}
		return info.typ
	case *ast.ArrayLit:
		return c.checkArray(e)
	case *ast.UnaryExpr:
		return c.checkUnary(e)
	case *ast.BinaryExpr:
		return c.checkBinary(e)
	case *ast.IndexExpr:
		return c.checkIndex(e)
	case *ast.MemberExpr:
		return c.checkMember(e)
	case *ast.CallExpr:
		return c.checkCall(e)
	case *ast.LambdaExpr:
		return c.checkLambda(e)
	default:
		c.errs.Add(diag.New(expr.Pos(), "unsupported expression"))
		return AnyType
	}
}

func (c *Checker) checkArray(lit *ast.ArrayLit) Type {
	if len(lit.Elements) == 0 {
		return ArrayOf(AnyType)
	}

	result := c.checkExpr(lit.Elements[0])
	for _, element := range lit.Elements[1:] {
		result = join(result, c.checkExpr(element))
	}
	return ArrayOf(result)
}

func (c *Checker) checkUnary(expr *ast.UnaryExpr) Type {
	operand := c.checkExpr(expr.X)
	if operand.Kind == Unknown {
		c.errs.Add(diag.New(expr.Position, "cannot apply operator %q to a value of type Unknown", expr.Op))
		return AnyType
	}
	if operand.Kind == Any {
		return AnyType
	}
	if operand.Nullable || operand.Kind == Null {
		c.errs.Add(diag.New(expr.Position, "cannot apply operator %q to a value of type %s because it may be null", expr.Op, operand.String()))
		return AnyType
	}
	if operand.Kind == Number {
		return NumberType
	}
	c.errs.Add(diag.New(expr.Position, "operator %q is not defined for type %s", expr.Op, operand.String()))
	return AnyType
}

func (c *Checker) checkBinary(expr *ast.BinaryExpr) Type {
	left := c.checkExpr(expr.Left)
	right := c.checkExpr(expr.Right)

	if left.Kind == Unknown || right.Kind == Unknown {
		c.errs.Add(diag.New(expr.Position, "cannot apply operator %q to a value of type Unknown", expr.Op))
		return AnyType
	}
	if left.Kind == Any || right.Kind == Any {
		return AnyType
	}
	if left.Nullable || right.Nullable || left.Kind == Null || right.Kind == Null {
		c.errs.Add(diag.New(
			expr.Position,
			"operator %q cannot be applied to %s and %s because a value may be null",
			expr.Op, left.String(), right.String(),
		))
		return AnyType
	}

	if expr.Op == token.Plus {
		if left.Kind == Number && right.Kind == Number {
			return NumberType
		}
		if left.Kind == String && (right.Kind == String || right.Kind == Number) {
			return StringType
		}
		if right.Kind == String && (left.Kind == String || left.Kind == Number) {
			return StringType
		}
	} else if left.Kind == Number && right.Kind == Number {
		return NumberType
	}

	c.errs.Add(diag.New(expr.Position, "operator %q is not defined for %s and %s", expr.Op, left.String(), right.String()))
	return AnyType
}

func (c *Checker) checkIndex(expr *ast.IndexExpr) Type {
	base := c.checkExpr(expr.X)
	index := c.checkExpr(expr.Index)

	if base.Nullable || base.Kind == Null || index.Nullable || index.Kind == Null {
		c.errs.Add(diag.New(expr.Position, "cannot index with a value that may be null"))
		return AnyType
	}

	if index.Kind != Number && index.Kind != Any {
		c.errs.Add(diag.New(expr.Index.Pos(), "array index must be Number, got %s", index.String()))
	}

	switch base.Kind {
	case Array:
		if base.Elem == nil {
			return AnyType
		}
		return *base.Elem
	case String:
		return StringType
	case Any:
		return AnyType
	case Unknown:
		c.errs.Add(diag.New(expr.Position, "cannot index a value of type Unknown"))
		return AnyType
	default:
		c.errs.Add(diag.New(expr.Position, "cannot index a value of type %s", base.String()))
		return AnyType
	}
}

func (c *Checker) checkMember(expr *ast.MemberExpr) Type {
	base := c.checkExpr(expr.X)

	if expr.Name != "length" {
		c.errs.Add(diag.New(expr.NamePos, "unknown property %q; only .length is supported", expr.Name))
		return AnyType
	}

	if base.Kind == Any {
		return AnyType
	}
	if base.Nullable || base.Kind == Null {
		c.errs.Add(diag.New(expr.Position, "cannot access .length on a value of type %s because it may be null", base.String()))
		return AnyType
	}

	switch base.Kind {
	case Array, String:
		return NumberType
	case Unknown:
		c.errs.Add(diag.New(expr.Position, "cannot access .length on a value of type Unknown"))
		return AnyType
	default:
		c.errs.Add(diag.New(expr.Position, "type %s has no property .length", base.String()))
		return AnyType
	}
}

func (c *Checker) checkLambda(lambda *ast.LambdaExpr) Type {
	c.pushScope()

	params := make([]Type, 0, len(lambda.Params))
	seen := make(map[string]bool)
	for _, param := range lambda.Params {
		var typ Type
		if param.Type == nil {
			c.errs.Add(diag.New(param.Position, "lambda parameter %q requires a type annotation", param.Name))
			typ = AnyType
		} else {
			var ok bool
			typ, ok = c.resolveType(param.Type)
			if !ok {
				typ = AnyType
			}
		}

		if seen[param.Name] {
			c.errs.Add(diag.New(param.NamePos, "parameter %q is already declared", param.Name))
		}
		seen[param.Name] = true

		c.scopes[len(c.scopes)-1][param.Name] = variable{typ: typ, pos: param.NamePos}
		params = append(params, typ)
	}

	bodyType := c.checkExpr(lambda.Body)
	if bodyType.Kind == Void {
		c.errs.Add(diag.New(lambda.Body.Pos(), "lambda body does not produce a value"))
	}

	c.popScope()

	returns := bodyType
	return Type{Kind: Function, Params: params, Returns: &returns}
}

func (c *Checker) checkCall(call *ast.CallExpr) Type {
	if info, exists := c.lookup(call.Callee); exists {
		return c.checkValueCall(call, info.typ)
	}

	sig, exists := c.functions[call.Callee]
	if !exists {
		c.errs.Add(diag.New(call.CalleePos, "undefined function %q", call.Callee))
		for _, arg := range call.Args {
			c.checkExpr(arg.Value)
		}
		return AnyType
	}

	supplied := make([]bool, len(sig.params))
	positional := 0

	for _, arg := range call.Args {
		if arg.Name == "" {
			index := positional
			positional++

			argType := c.checkExpr(arg.Value)
			if argType.Kind == Void {
				c.errs.Add(diag.New(arg.Value.Pos(), "argument does not produce a value"))
				continue
			}

			expected, ok := c.argumentType(sig, index)
			if !ok {
				c.errs.Add(diag.New(
					arg.Position,
					"too many arguments in call to %q: expected at most %d",
					call.Callee, len(sig.params),
				))
				continue
			}
			if !assignable(argType, expected) {
				c.errs.Add(diag.New(
					arg.Position,
					"cannot use a value of type %s as argument %d of function %q; expected %s",
					argType.String(), index+1, call.Callee, expected.String(),
				))
			}
			if index < len(supplied) {
				supplied[index] = true
			}
			continue
		}

		index, found := sig.byName[arg.Name]
		if !found {
			c.errs.Add(diag.New(arg.NamePos, "function %q has no parameter %q", call.Callee, arg.Name))
			c.checkExpr(arg.Value)
			continue
		}
		if sig.params[index].variadic {
			c.errs.Add(diag.New(arg.NamePos, "variadic parameter %q cannot be passed by name", arg.Name))
			c.checkExpr(arg.Value)
			continue
		}
		if supplied[index] {
			c.errs.Add(diag.New(arg.NamePos, "argument %q is specified more than once", arg.Name))
			c.checkExpr(arg.Value)
			continue
		}

		argType := c.checkExpr(arg.Value)
		if argType.Kind == Void {
			c.errs.Add(diag.New(arg.Value.Pos(), "argument does not produce a value"))
			continue
		}
		if !assignable(argType, sig.params[index].typ) {
			c.errs.Add(diag.New(
				arg.Position,
				"cannot use a value of type %s for parameter %q of function %q; expected %s",
				argType.String(), arg.Name, call.Callee, sig.params[index].typ.String(),
			))
		}
		supplied[index] = true
	}

	for index, param := range sig.params {
		if param.variadic || param.hasDefault || supplied[index] {
			continue
		}
		c.errs.Add(diag.New(
			call.Position,
			"missing argument %q in call to function %q",
			param.name, call.Callee,
		))
	}

	return sig.returns
}

func (c *Checker) checkValueCall(call *ast.CallExpr, callee Type) Type {
	if callee.Nullable || callee.Kind == Null {
		c.errs.Add(diag.New(call.CalleePos, "cannot call %q because it may be null", call.Callee))
		c.checkCallArguments(call)
		return AnyType
	}
	if callee.Kind != Function {
		c.errs.Add(diag.New(
			call.CalleePos,
			"value %q of type %s is not callable",
			call.Callee, callee.String(),
		))
		c.checkCallArguments(call)
		return AnyType
	}

	positional := 0
	for _, arg := range call.Args {
		argType := c.checkExpr(arg.Value)
		if argType.Kind == Void {
			c.errs.Add(diag.New(arg.Value.Pos(), "argument does not produce a value"))
			continue
		}
		if arg.Name != "" {
			c.errs.Add(diag.New(arg.NamePos, "named arguments are not supported for function values"))
			continue
		}

		index := positional
		positional++
		if index >= len(callee.Params) {
			c.errs.Add(diag.New(
				arg.Position,
				"too many arguments in call to value %q: expected at most %d",
				call.Callee, len(callee.Params),
			))
			continue
		}
		if !assignable(argType, callee.Params[index]) {
			c.errs.Add(diag.New(
				arg.Position,
				"cannot use a value of type %s as argument %d of value %q; expected %s",
				argType.String(), index+1, call.Callee, callee.Params[index].String(),
			))
		}
	}

	if len(call.Args) < len(callee.Params) {
		expected := fmt.Sprintf("%d arguments", len(callee.Params))
		if len(callee.Params) == 1 {
			expected = "1 argument"
		}
		c.errs.Add(diag.New(
			call.Position,
			"call to value %q requires %s, got %d",
			call.Callee, expected, len(call.Args),
		))
	}

	if callee.Returns == nil {
		return AnyType
	}
	return *callee.Returns
}

func (c *Checker) checkCallArguments(call *ast.CallExpr) {
	for _, arg := range call.Args {
		c.checkExpr(arg.Value)
	}
}

func (c *Checker) argumentType(sig *functionSignature, index int) (Type, bool) {
	if sig.variadic >= 0 && index >= sig.variadic {
		return sig.params[sig.variadic].typ, true
	}
	if index < 0 || index >= len(sig.params) {
		return Type{}, false
	}
	return sig.params[index].typ, true
}

func (c *Checker) resolveType(expr *ast.TypeExpr) (Type, bool) {
	if expr.Function {
		ok := true
		params := make([]Type, len(expr.Params))
		for i, param := range expr.Params {
			typ, valid := c.resolveType(param)
			if !valid {
				ok = false
				typ = AnyType
			}
			params[i] = typ
		}

		returns := AnyType
		if expr.Returns != nil {
			typ, valid := c.resolveType(expr.Returns)
			if !valid {
				ok = false
			} else {
				returns = typ
			}
		}

		return Type{Kind: Function, Params: params, Returns: &returns}, ok
	}

	var typ Type
	switch expr.Name {
	case "Number", "Int":
		typ = NumberType
	case "String":
		typ = StringType
	case "Array":
		typ = ArrayOf(AnyType)
	case "Any":
		typ = AnyType
	case "Unknown":
		typ = UnknownType
	case "Null":
		typ = NullType
	default:
		c.errs.Add(diag.New(
			expr.Position,
			"unknown type %q; expected Number, Int, String, Array, Any, Unknown or Null",
			expr.Name,
		))
		return AnyType, false
	}

	if expr.Nullable {
		typ = NullableOf(typ)
	}
	return typ, true
}

func (c *Checker) pushScope() {
	c.scopes = append(c.scopes, make(map[string]variable))
}

func (c *Checker) popScope() {
	c.scopes = c.scopes[:len(c.scopes)-1]
}

func (c *Checker) lookup(name string) (variable, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if info, ok := c.scopes[i][name]; ok {
			return info, true
		}
	}
	return variable{}, false
}

func assignable(src, dst Type) bool {
	if src.Kind == Void || dst.Kind == Void {
		return src.Kind == dst.Kind
	}
	if dst.Kind == Any || dst.Kind == Unknown {
		return true
	}
	if src.Kind == Any {
		return true
	}
	if src.Kind == Unknown {
		return false
	}
	if src.Kind == Invalid || dst.Kind == Invalid {
		return true
	}
	if src.Kind == Null {
		return dst.Kind == Null || dst.Nullable
	}
	if dst.Kind == Null {
		return false
	}
	if src.Nullable && !dst.Nullable {
		return false
	}
	if src.Kind != dst.Kind {
		return false
	}
	if src.Kind == Function {
		return Equal(src, dst)
	}
	if src.Kind != Array {
		return true
	}
	if dst.Elem == nil || dst.Elem.Kind == Any {
		return true
	}
	if src.Elem == nil {
		return false
	}
	return assignable(*src.Elem, *dst.Elem)
}

func join(a, b Type) Type {
	if Equal(a, b) {
		return a
	}
	if a.Kind == Null {
		return NullableOf(b)
	}
	if b.Kind == Null {
		return NullableOf(a)
	}
	if a.Kind == b.Kind {
		if a.Kind == Array {
			if a.Elem == nil || b.Elem == nil {
				result := ArrayOf(AnyType)
				result.Nullable = a.Nullable || b.Nullable
				return result
			}
			result := ArrayOf(join(*a.Elem, *b.Elem))
			result.Nullable = a.Nullable || b.Nullable
			return result
		}
		if a.Kind == Any || a.Kind == Unknown {
			return AnyType
		}
		if a.Kind == Function {
			return AnyType
		}
		result := a
		result.Nullable = a.Nullable || b.Nullable
		return result
	}
	return AnyType
}

func blockReturns(block *ast.BlockStmt) bool {
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.BlockStmt:
			if blockReturns(s) {
				return true
			}
		}
	}
	return false
}
