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
	Boolean
	Array
	Any
	Unknown
	Null
	Void
	Function
)

// Type is a Lynx value type. Arrays carry their element type, and functions
// carry their parameter and return types. Nullable records a trailing ?.
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
	BooleanType = Type{Kind: Boolean}
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

func nonNullable(t Type) Type {
	t.Nullable = false
	return t
}

func (t Type) String() string {
	var base string
	switch t.Kind {
	case Number:
		base = "Number"
	case String:
		base = "String"
	case Boolean:
		base = "Boolean"
	case Array:
		if t.Elem == nil {
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
	typ         Type
	declared    Type
	constant    bool
	initialized bool
	pos         token.Position
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
	variadic int
	returns  Type
	pos      token.Position
	builtin  bool
}

type flow struct {
	next         bool
	loopBreak    bool
	switchBreak  bool
	fallsThrough bool
}

// Checker validates declarations, expressions and control flow.
type Checker struct {
	scopes        []map[string]variable
	functions     map[string]*functionSignature
	signatures    map[*ast.FunctionDecl]*functionSignature
	errs          diag.List
	currentReturn *Type
	loopDepth     int
	breakables    []string
}

// Check type-checks a program. It returns all diagnostics found.
func Check(program *ast.Program) error {
	c := &Checker{
		functions:  make(map[string]*functionSignature),
		signatures: make(map[*ast.FunctionDecl]*functionSignature),
	}
	c.pushScope()
	c.installBuiltins()

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

func (c *Checker) installBuiltins() {
	for _, sig := range []*functionSignature{
		{
			name: "println", byName: map[string]int{"value": 0}, variadic: -1,
			params: []paramInfo{{name: "value", typ: AnyType}}, returns: VoidType, builtin: true,
		},
		{
			name: "toString", byName: map[string]int{"value": 0}, variadic: -1,
			params: []paramInfo{{name: "value", typ: AnyType}}, returns: StringType, builtin: true,
		},
	} {
		c.functions[sig.name] = sig
	}
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
		name: fn.Name, byName: make(map[string]int), variadic: -1,
		returns: VoidType, pos: fn.NamePos,
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

		info := paramInfo{name: param.Name, typ: typ, hasDefault: param.Default != nil, variadic: param.Variadic, pos: param.Position}
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
				c.errs.Add(diag.New(param.Default.Pos(), "default value of type %s is not assignable to parameter %q of type %s", defaultType.String(), param.Name, info.typ.String()))
			}
		}

		bodyType := info.typ
		if info.variadic {
			bodyType = ArrayOf(bodyType)
		}
		if _, exists := c.functions[param.Name]; exists {
			c.errs.Add(diag.New(param.NamePos, "parameter %q conflicts with function %q", param.Name, param.Name))
		}
		c.scopes[len(c.scopes)-1][param.Name] = variable{typ: bodyType, declared: bodyType, initialized: true, pos: param.NamePos}
	}

	bodyFlow := c.checkStatements(fn.Body.Statements)
	if sig.returns.Kind != Void && bodyFlow.next {
		c.errs.Add(diag.New(fn.NamePos, "function %q with return type %s does not return a value on every path", fn.Name, sig.returns.String()))
	}

	c.currentReturn = savedReturn
	c.popScope()
}

func (c *Checker) checkStatements(statements []ast.Stmt) flow {
	result := flow{next: true}
	for _, stmt := range statements {
		stmtFlow := c.checkStmt(stmt)
		if result.next {
			result.next = stmtFlow.next
		}
		result.loopBreak = result.loopBreak || stmtFlow.loopBreak
		result.switchBreak = result.switchBreak || stmtFlow.switchBreak
		result.fallsThrough = result.fallsThrough || stmtFlow.fallsThrough
	}
	return result
}

func (c *Checker) checkStmt(stmt ast.Stmt) flow {
	switch s := stmt.(type) {
	case *ast.VarDecl:
		c.checkVarDecl(s)
	case *ast.AssignStmt:
		c.checkAssign(s)
	case *ast.ExprStmt:
		c.checkExpr(s.X)
	case *ast.ReturnStmt:
		c.checkReturn(s)
		return flow{}
	case *ast.BlockStmt:
		return c.checkBlock(s)
	case *ast.IfStmt:
		return c.checkIf(s)
	case *ast.ForStmt:
		return c.checkFor(s)
	case *ast.SwitchStmt:
		return c.checkSwitch(s)
	case *ast.BreakStmt:
		if len(c.breakables) == 0 {
			c.errs.Add(diag.New(s.Position, "break statement outside a loop or switch"))
		}
		isLoop := len(c.breakables) > 0 && c.breakables[len(c.breakables)-1] == "loop"
		isSwitch := len(c.breakables) > 0 && c.breakables[len(c.breakables)-1] == "switch"
		return flow{loopBreak: isLoop, switchBreak: isSwitch}
	case *ast.ContinueStmt:
		if c.loopDepth == 0 {
			c.errs.Add(diag.New(s.Position, "continue statement outside a loop"))
		}
		return flow{}
	case *ast.FallthroughStmt:
		if len(c.breakables) == 0 || c.breakables[len(c.breakables)-1] != "switch" {
			c.errs.Add(diag.New(s.Position, "fallthrough statement outside a switch"))
		}
		return flow{fallsThrough: true}
	case *ast.FunctionDecl:
		c.errs.Add(diag.New(s.NamePos, "function declarations are only allowed at the top level"))
	}
	return flow{next: true}
}

func (c *Checker) checkBlock(block *ast.BlockStmt) flow {
	c.pushScope()
	result := c.checkStatements(block.Statements)
	c.popScope()
	return result
}

func (c *Checker) checkIf(stmt *ast.IfStmt) flow {
	c.checkCondition(stmt.Cond)
	base := cloneScopes(c.scopes)

	c.scopes = cloneScopes(base)
	c.refineScopes(stmt.Cond, true)
	thenFlow := c.checkBlock(stmt.Then)
	thenState := cloneScopes(c.scopes)

	elseFlow := flow{next: true}
	var elseState []map[string]variable
	if stmt.Else != nil {
		c.scopes = cloneScopes(base)
		c.refineScopes(stmt.Cond, false)
		elseFlow = c.checkStmt(stmt.Else)
		elseState = cloneScopes(c.scopes)
	} else {
		elseState = cloneScopes(base)
		c.scopes = elseState
		c.refineScopes(stmt.Cond, false)
		elseState = cloneScopes(c.scopes)
	}

	paths := make([][]map[string]variable, 0, 2)
	if thenFlow.next {
		paths = append(paths, thenState)
	}
	if elseFlow.next {
		paths = append(paths, elseState)
	}
	if len(paths) == 0 {
		c.scopes = base
	} else {
		c.scopes = mergeScopes(base, paths)
	}
	return flow{
		next:         thenFlow.next || elseFlow.next,
		loopBreak:    thenFlow.loopBreak || elseFlow.loopBreak,
		switchBreak:  thenFlow.switchBreak || elseFlow.switchBreak,
		fallsThrough: thenFlow.fallsThrough || elseFlow.fallsThrough,
	}
}

func (c *Checker) checkFor(stmt *ast.ForStmt) flow {
	c.pushScope()
	if stmt.Init != nil {
		c.checkStmt(stmt.Init)
	}
	if stmt.Cond != nil {
		c.checkCondition(stmt.Cond)
	}
	base := cloneScopes(c.scopes)
	c.refineScopes(stmt.Cond, true)

	c.loopDepth++
	c.breakables = append(c.breakables, "loop")
	bodyFlow := c.checkBlock(stmt.Body)
	if stmt.Post != nil {
		c.checkStmt(stmt.Post)
	}
	c.breakables = c.breakables[:len(c.breakables)-1]
	c.loopDepth--

	c.scopes = base
	c.popScope()
	infinite := stmt.Cond == nil
	if lit, ok := stmt.Cond.(*ast.BoolLit); ok && lit.Value {
		infinite = true
	}
	return flow{next: !infinite || bodyFlow.loopBreak}
}

func (c *Checker) checkSwitch(stmt *ast.SwitchStmt) flow {
	valueType := BooleanType
	if stmt.Value != nil {
		valueType = c.checkExpr(stmt.Value)
		if valueType.Kind == Void {
			c.errs.Add(diag.New(stmt.Value.Pos(), "switch expression does not produce a value"))
		}
	}
	base := cloneScopes(c.scopes)

	c.breakables = append(c.breakables, "switch")
	flows := make([]flow, len(stmt.Cases))
	states := make([][]map[string]variable, len(stmt.Cases))
	hasDefault := false
	hasFallthrough := false
	for i, clause := range stmt.Cases {
		if clause.IsDefault {
			if hasDefault {
				c.errs.Add(diag.New(clause.Position, "switch has more than one default clause"))
			}
			hasDefault = true
		}
		for _, value := range clause.Values {
			caseType := c.checkExpr(value)
			if !comparable(valueType, caseType) {
				c.errs.Add(diag.New(value.Pos(), "cannot compare switch value of type %s with case value of type %s", valueType.String(), caseType.String()))
			}
		}

		c.scopes = cloneScopes(base)
		c.pushScope()
		flows[i] = c.checkStatements(clause.Body)
		c.popScope()
		states[i] = cloneScopes(c.scopes)
		if flows[i].fallsThrough {
			hasFallthrough = true
			last := len(clause.Body) > 0
			if last {
				_, last = clause.Body[len(clause.Body)-1].(*ast.FallthroughStmt)
			}
			if !last || i == len(stmt.Cases)-1 {
				c.errs.Add(diag.New(clause.Position, "fallthrough must be the last statement of a non-final case"))
			}
		}
	}
	c.breakables = c.breakables[:len(c.breakables)-1]

	canReach := make([]bool, len(flows))
	for i := len(flows) - 1; i >= 0; i-- {
		canReach[i] = flows[i].next || flows[i].switchBreak
		if flows[i].fallsThrough && i+1 < len(flows) {
			canReach[i] = canReach[i+1]
		}
	}
	next := !hasDefault
	loopBreak := false
	paths := make([][]map[string]variable, 0, len(flows)+1)
	if !hasDefault {
		paths = append(paths, base)
	}
	for i, f := range flows {
		loopBreak = loopBreak || f.loopBreak
		if canReach[i] {
			next = true
			paths = append(paths, states[i])
		}
	}
	if len(paths) == 0 {
		c.scopes = base
	} else if hasFallthrough {
		c.scopes = base
	} else {
		c.scopes = mergeScopes(base, paths)
	}
	return flow{next: next, loopBreak: loopBreak}
}

func (c *Checker) checkCondition(expr ast.Expr) {
	typ := c.checkExpr(expr)
	if typ.Kind != Boolean && typ.Kind != Any {
		c.errs.Add(diag.New(expr.Pos(), "condition must have type Boolean, got %s", typ.String()))
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
			c.errs.Add(diag.New(decl.Value.Pos(), "cannot assign a value of type %s to variable %q of type %s", valueType.String(), decl.Name, annotation.String()))
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
	c.scopes[len(c.scopes)-1][decl.Name] = variable{typ: finalType, declared: finalType, constant: decl.Const, initialized: hasValue, pos: decl.NamePos}
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
	if !assignable(valueType, info.declared) {
		c.errs.Add(diag.New(stmt.Value.Pos(), "cannot assign a value of type %s to variable %q of type %s", valueType.String(), stmt.Name, info.declared.String()))
	}
	c.updateVariable(stmt.Name, func(info *variable) {
		info.typ = info.declared
		info.initialized = true
	})
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
			c.errs.Add(diag.New(stmt.Position, "function with return type %s must return a value", returnType.String()))
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
		c.errs.Add(diag.New(stmt.Value.Pos(), "cannot return a value of type %s from a function returning %s", valueType.String(), returnType.String()))
	}
}

func (c *Checker) checkExpr(expr ast.Expr) Type {
	switch e := expr.(type) {
	case *ast.NumberLit:
		return NumberType
	case *ast.StringLit:
		return StringType
	case *ast.BoolLit:
		return BooleanType
	case *ast.NullLit:
		return NullType
	case *ast.Ident:
		info, exists := c.lookup(e.Name)
		if !exists {
			c.errs.Add(diag.New(e.Position, "undefined variable %q", e.Name))
			return AnyType
		}
		if !info.initialized {
			c.errs.Add(diag.New(e.Position, "variable %q is used before initialization", e.Name))
		}
		return info.typ
	case *ast.ArrayLit:
		return c.checkArray(e)
	case *ast.UnaryExpr:
		return c.checkUnary(e)
	case *ast.BinaryExpr:
		return c.checkBinary(e)
	case *ast.TypeTestExpr:
		c.checkExpr(e.X)
		c.resolveType(e.Type)
		return BooleanType
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
	if expr.Op == token.Bang {
		if operand.Kind != Boolean && operand.Kind != Any {
			c.errs.Add(diag.New(expr.Position, "operator %q is not defined for type %s", expr.Op, operand.String()))
		}
		if operand.Kind == Any {
			return AnyType
		}
		return BooleanType
	}
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
	if expr.Op == token.AndAnd || expr.Op == token.OrOr {
		saved := cloneScopes(c.scopes)
		c.refineScopes(expr.Left, expr.Op == token.AndAnd)
		right := c.checkExpr(expr.Right)
		c.scopes = saved
		if left.Kind != Boolean && left.Kind != Any {
			c.errs.Add(diag.New(expr.Left.Pos(), "logical operator requires Boolean, got %s", left.String()))
		}
		if right.Kind != Boolean && right.Kind != Any {
			c.errs.Add(diag.New(expr.Right.Pos(), "logical operator requires Boolean, got %s", right.String()))
		}
		if left.Kind == Any || right.Kind == Any {
			return AnyType
		}
		return BooleanType
	}

	right := c.checkExpr(expr.Right)
	switch expr.Op {
	case token.Eq, token.NotEq, token.StrictEq, token.StrictNe:
		if !comparable(left, right) {
			c.errs.Add(diag.New(expr.Position, "cannot compare values of types %s and %s", left.String(), right.String()))
		}
		return BooleanType
	case token.Lt, token.LtEq, token.Gt, token.GtEq:
		if (left.Kind != Number && left.Kind != Any) || (right.Kind != Number && right.Kind != Any) || left.Nullable || right.Nullable {
			c.errs.Add(diag.New(expr.Position, "operator %q is not defined for %s and %s", expr.Op, left.String(), right.String()))
		}
		return BooleanType
	}

	if left.Kind == Unknown || right.Kind == Unknown {
		c.errs.Add(diag.New(expr.Position, "cannot apply operator %q to a value of type Unknown", expr.Op))
		return AnyType
	}
	if left.Kind == Any || right.Kind == Any {
		return AnyType
	}
	if left.Nullable || right.Nullable || left.Kind == Null || right.Kind == Null {
		c.errs.Add(diag.New(expr.Position, "operator %q cannot be applied to %s and %s because a value may be null", expr.Op, left.String(), right.String()))
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

func comparable(left, right Type) bool {
	if left.Kind == Any || right.Kind == Any || left.Kind == Unknown || right.Kind == Unknown {
		return true
	}
	if left.Kind == Null {
		return right.Kind == Null || right.Nullable
	}
	if right.Kind == Null {
		return left.Kind == Null || left.Nullable
	}
	return left.Kind == right.Kind
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
			return NullableOf(AnyType)
		}
		return NullableOf(*base.Elem)
	case String:
		return NullableOf(StringType)
	case Any:
		return NullableOf(AnyType)
	case Unknown:
		c.errs.Add(diag.New(expr.Position, "cannot index a value of type Unknown"))
	default:
		c.errs.Add(diag.New(expr.Position, "cannot index a value of type %s", base.String()))
	}
	return AnyType
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
	default:
		c.errs.Add(diag.New(expr.Position, "type %s has no property .length", base.String()))
	}
	return AnyType
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
		c.scopes[len(c.scopes)-1][param.Name] = variable{typ: typ, declared: typ, initialized: true, pos: param.NamePos}
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
		if !info.initialized {
			c.errs.Add(diag.New(call.CalleePos, "variable %q is used before initialization", call.Callee))
		}
		return c.checkValueCall(call, info.typ)
	}
	sig, exists := c.functions[call.Callee]
	if !exists {
		c.errs.Add(diag.New(call.CalleePos, "undefined function %q", call.Callee))
		c.checkCallArguments(call)
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
				c.errs.Add(diag.New(arg.Position, "too many arguments in call to %q: expected at most %d", call.Callee, len(sig.params)))
				continue
			}
			if !assignable(argType, expected) {
				c.errs.Add(diag.New(arg.Position, "cannot use a value of type %s as argument %d of function %q; expected %s", argType.String(), index+1, call.Callee, expected.String()))
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
			c.errs.Add(diag.New(arg.Position, "cannot use a value of type %s for parameter %q of function %q; expected %s", argType.String(), arg.Name, call.Callee, sig.params[index].typ.String()))
		}
		supplied[index] = true
	}

	for index, param := range sig.params {
		if param.variadic || param.hasDefault || supplied[index] {
			continue
		}
		c.errs.Add(diag.New(call.Position, "missing argument %q in call to function %q", param.name, call.Callee))
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
		c.errs.Add(diag.New(call.CalleePos, "value %q of type %s is not callable", call.Callee, callee.String()))
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
			c.errs.Add(diag.New(arg.Position, "too many arguments in call to value %q: expected at most %d", call.Callee, len(callee.Params)))
			continue
		}
		if !assignable(argType, callee.Params[index]) {
			c.errs.Add(diag.New(arg.Position, "cannot use a value of type %s as argument %d of value %q; expected %s", argType.String(), index+1, call.Callee, callee.Params[index].String()))
		}
	}
	if len(call.Args) < len(callee.Params) {
		expected := fmt.Sprintf("%d arguments", len(callee.Params))
		if len(callee.Params) == 1 {
			expected = "1 argument"
		}
		c.errs.Add(diag.New(call.Position, "call to value %q requires %s, got %d", call.Callee, expected, len(call.Args)))
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
		if expr.Elem != nil {
			c.errs.Add(diag.New(expr.Position, "function types cannot have element parameters"))
			return AnyType, false
		}
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
		return Type{Kind: Function, Params: params, Returns: &returns, Nullable: expr.Nullable}, ok
	}

	var typ Type
	switch expr.Name {
	case "Number", "Int":
		typ = NumberType
	case "String":
		typ = StringType
	case "Boolean":
		typ = BooleanType
	case "Array":
		typ = ArrayOf(AnyType)
		if expr.Elem != nil {
			elem, ok := c.resolveType(expr.Elem)
			if !ok {
				return AnyType, false
			}
			typ = ArrayOf(elem)
		}
	case "Any":
		typ = AnyType
	case "Unknown":
		typ = UnknownType
	case "Null":
		typ = NullType
	default:
		c.errs.Add(diag.New(expr.Position, "unknown type %q; expected Number, Int, String, Boolean, Array, Any, Unknown or Null", expr.Name))
		return AnyType, false
	}
	if expr.Elem != nil && expr.Name != "Array" {
		c.errs.Add(diag.New(expr.Position, "only Array can have an element type"))
		return AnyType, false
	}
	if expr.Nullable {
		typ = NullableOf(typ)
	}
	return typ, true
}

func (c *Checker) refineScopes(expr ast.Expr, truth bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.Bang {
			c.refineScopes(e.X, !truth)
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.AndAnd:
			if truth {
				c.refineScopes(e.Left, true)
				c.refineScopes(e.Right, true)
			}
		case token.OrOr:
			if !truth {
				c.refineScopes(e.Left, false)
				c.refineScopes(e.Right, false)
			}
		case token.Eq, token.StrictEq, token.NotEq, token.StrictNe:
			ident, other := nullComparison(e)
			if ident == nil {
				return
			}
			_ = other
			equal := e.Op == token.Eq || e.Op == token.StrictEq
			isNull := equal == truth
			c.updateVariable(ident.Name, func(info *variable) {
				if isNull {
					info.typ = NullType
				} else {
					info.typ = nonNullable(info.typ)
				}
			})
		}
	case *ast.TypeTestExpr:
		ident, ok := e.X.(*ast.Ident)
		if !ok {
			return
		}
		target, valid := c.resolveType(e.Type)
		if !valid {
			return
		}
		positive := truth != e.Negate
		c.updateVariable(ident.Name, func(info *variable) {
			if positive {
				info.typ = target
			} else if target.Kind == Null {
				info.typ = nonNullable(info.typ)
			} else if info.typ.Nullable && info.typ.Kind == target.Kind {
				info.typ = NullType
			}
		})
	}
}

func nullComparison(expr *ast.BinaryExpr) (*ast.Ident, ast.Expr) {
	if ident, ok := expr.Left.(*ast.Ident); ok {
		if _, ok := expr.Right.(*ast.NullLit); ok {
			return ident, expr.Right
		}
	}
	if ident, ok := expr.Right.(*ast.Ident); ok {
		if _, ok := expr.Left.(*ast.NullLit); ok {
			return ident, expr.Left
		}
	}
	return nil, nil
}

func cloneScopes(scopes []map[string]variable) []map[string]variable {
	result := make([]map[string]variable, len(scopes))
	for i, scope := range scopes {
		result[i] = make(map[string]variable, len(scope))
		for name, info := range scope {
			result[i][name] = info
		}
	}
	return result
}

func mergeScopes(base []map[string]variable, paths [][]map[string]variable) []map[string]variable {
	if len(paths) == 1 {
		return cloneScopes(paths[0])
	}
	result := cloneScopes(paths[0])
	for i, scope := range base {
		for name, baseInfo := range scope {
			info := result[i][name]
			info.typ = baseInfo.typ
			info.initialized = true
			sameType := true
			for _, path := range paths {
				pathInfo, exists := path[i][name]
				if !exists {
					info.initialized = false
					sameType = false
					continue
				}
				info.initialized = info.initialized && pathInfo.initialized
				if !Equal(pathInfo.typ, paths[0][i][name].typ) {
					sameType = false
				}
			}
			if sameType {
				info.typ = paths[0][i][name].typ
			}
			result[i][name] = info
		}
	}
	return result
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

func (c *Checker) updateVariable(name string, update func(*variable)) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if info, ok := c.scopes[i][name]; ok {
			update(&info)
			c.scopes[i][name] = info
			return
		}
	}
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
