// Package ast defines the abstract syntax tree of Lynx.
package ast

import "lynx/internal/token"

// Node is any syntax tree node.
type Node interface {
	Pos() token.Position
}

// Stmt is a statement.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is an expression.
type Expr interface {
	Node
	exprNode()
}

// Program is a complete Lynx source file.
type Program struct {
	Statements []Stmt
}

// VarDecl declares a constant or a mutable variable.
type VarDecl struct {
	Position token.Position
	Const    bool
	Name     string
	NamePos  token.Position
	Type     *TypeExpr
	Value    Expr
}

func (d *VarDecl) Pos() token.Position { return d.Position }
func (d *VarDecl) stmtNode()           {}

// AssignStmt assigns a new value to a mutable variable.
type AssignStmt struct {
	Position token.Position
	Name     string
	NamePos  token.Position
	Value    Expr
}

func (s *AssignStmt) Pos() token.Position { return s.Position }
func (s *AssignStmt) stmtNode()           {}

// ExprStmt evaluates an expression for its side effects, usually a call.
type ExprStmt struct {
	Position token.Position
	X        Expr
}

func (s *ExprStmt) Pos() token.Position { return s.Position }
func (s *ExprStmt) stmtNode()           {}

// BlockStmt is a braces-delimited sequence of statements.
type BlockStmt struct {
	Position   token.Position
	Statements []Stmt
}

func (s *BlockStmt) Pos() token.Position { return s.Position }
func (s *BlockStmt) stmtNode()           {}

// ReturnStmt returns from the enclosing function.
type ReturnStmt struct {
	Position token.Position
	Value    Expr
}

func (s *ReturnStmt) Pos() token.Position { return s.Position }
func (s *ReturnStmt) stmtNode()           {}

// Param is a function parameter.
type Param struct {
	Position token.Position
	Name     string
	NamePos  token.Position
	Type     *TypeExpr
	Variadic bool
	Default  Expr
}

func (p *Param) Pos() token.Position { return p.Position }

// FunctionDecl declares a function.
type FunctionDecl struct {
	Position   token.Position
	Name       string
	NamePos    token.Position
	Params     []*Param
	ReturnType *TypeExpr
	Body       *BlockStmt
}

func (d *FunctionDecl) Pos() token.Position { return d.Position }
func (d *FunctionDecl) stmtNode()           {}

// TypeExpr is a type annotation such as Number, Array, String? or
// (Number, Number) -> Number.
type TypeExpr struct {
	Position token.Position
	Name     string
	Nullable bool
	Function bool
	Params   []*TypeExpr
	Returns  *TypeExpr
}

func (t *TypeExpr) Pos() token.Position { return t.Position }

// NumberLit is a numeric literal.
type NumberLit struct {
	Position token.Position
	Raw      string
}

func (e *NumberLit) Pos() token.Position { return e.Position }
func (e *NumberLit) exprNode()           {}

// StringLit is a string literal.
type StringLit struct {
	Position token.Position
	Value    string
}

func (e *StringLit) Pos() token.Position { return e.Position }
func (e *StringLit) exprNode()           {}

// NullLit is the null literal.
type NullLit struct {
	Position token.Position
}

func (e *NullLit) Pos() token.Position { return e.Position }
func (e *NullLit) exprNode()           {}

// CallArg is an argument in a function call. Name is empty for positional
// arguments.
type CallArg struct {
	Position token.Position
	Name     string
	NamePos  token.Position
	Value    Expr
}

func (a *CallArg) Pos() token.Position { return a.Position }

// CallExpr calls a named function.
type CallExpr struct {
	Position  token.Position
	Callee    string
	CalleePos token.Position
	Args      []*CallArg
}

func (e *CallExpr) Pos() token.Position { return e.Position }
func (e *CallExpr) exprNode()           {}

// LambdaExpr is an anonymous function such as
// { x: Number, y: Number -> x + y }. The body is a single expression whose
// value is returned by the function.
type LambdaExpr struct {
	Position token.Position
	Params   []*Param
	Body     Expr
}

func (e *LambdaExpr) Pos() token.Position { return e.Position }
func (e *LambdaExpr) exprNode()           {}

// Ident references a declared variable.
type Ident struct {
	Position token.Position
	Name     string
}

func (e *Ident) Pos() token.Position { return e.Position }
func (e *Ident) exprNode()           {}

// ArrayLit is an array literal such as [1, 2, 3].
type ArrayLit struct {
	Position token.Position
	Elements []Expr
}

func (e *ArrayLit) Pos() token.Position { return e.Position }
func (e *ArrayLit) exprNode()           {}

// UnaryExpr is a prefix operation such as -x or +x.
type UnaryExpr struct {
	Position token.Position
	Op       token.Type
	X        Expr
}

func (e *UnaryExpr) Pos() token.Position { return e.Position }
func (e *UnaryExpr) exprNode()           {}

// BinaryExpr is an infix operation.
type BinaryExpr struct {
	Position token.Position
	Op       token.Type
	Left     Expr
	Right    Expr
}

func (e *BinaryExpr) Pos() token.Position { return e.Position }
func (e *BinaryExpr) exprNode()           {}

// IndexExpr accesses an array or string element, for example values[0].
type IndexExpr struct {
	Position token.Position
	X        Expr
	Index    Expr
}

func (e *IndexExpr) Pos() token.Position { return e.Position }
func (e *IndexExpr) exprNode()           {}

// MemberExpr accesses a property such as values.length.
type MemberExpr struct {
	Position token.Position
	X        Expr
	Name     string
	NamePos  token.Position
}

func (e *MemberExpr) Pos() token.Position { return e.Position }
func (e *MemberExpr) exprNode()           {}
