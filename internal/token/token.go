// Package token defines the lexical tokens of the Lynx language.
package token

import "fmt"

// Type identifies the kind of a token.
type Type string

const (
	Illegal Type = "ILLEGAL"
	EOF     Type = "EOF"

	Ident  Type = "IDENT"
	Number Type = "NUMBER"
	String Type = "STRING"

	Assign Type = "="

	Plus     Type = "+"
	Minus    Type = "-"
	Star     Type = "*"
	Slash    Type = "/"
	Percent  Type = "%"
	Bang     Type = "!"
	Eq       Type = "=="
	NotEq    Type = "!="
	StrictEq Type = "==="
	StrictNe Type = "!=="
	Lt       Type = "<"
	Gt       Type = ">"
	LtEq     Type = "<="
	GtEq     Type = ">="
	AndAnd   Type = "&&"
	OrOr     Type = "||"

	Colon     Type = ":"
	Comma     Type = ","
	Semicolon Type = ";"
	Dot       Type = "."
	Question  Type = "?"
	Ellipsis  Type = "..."
	Arrow     Type = "->"
	LBracket  Type = "["
	RBracket  Type = "]"
	LParen    Type = "("
	RParen    Type = ")"
	LBrace    Type = "{"
	RBrace    Type = "}"

	Const       Type = "CONST"
	Var         Type = "VAR"
	Fun         Type = "FUN"
	Return      Type = "RETURN"
	Null        Type = "NULL"
	True        Type = "TRUE"
	False       Type = "FALSE"
	If          Type = "IF"
	Else        Type = "ELSE"
	For         Type = "FOR"
	Break       Type = "BREAK"
	Continue    Type = "CONTINUE"
	Switch      Type = "SWITCH"
	Case        Type = "CASE"
	Default     Type = "DEFAULT"
	Fallthrough Type = "FALLTHROUGH"
	Is          Type = "IS"
)

// Position is a source location. Lines and columns are 1-based.
type Position struct {
	Filename string
	Offset   int
	Line     int
	Column   int
}

func (p Position) String() string {
	if p.Filename != "" {
		return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Token is a single lexical unit.
type Token struct {
	Type    Type
	Literal string
	Pos     Position
}

// Lookup maps an identifier to a keyword token type.
func Lookup(ident string) Type {
	switch ident {
	case "const", "val":
		return Const
	case "var":
		return Var
	case "fun":
		return Fun
	case "return":
		return Return
	case "null":
		return Null
	case "true":
		return True
	case "false":
		return False
	case "if":
		return If
	case "else":
		return Else
	case "for":
		return For
	case "break":
		return Break
	case "continue":
		return Continue
	case "switch":
		return Switch
	case "case":
		return Case
	case "default":
		return Default
	case "fallthrough":
		return Fallthrough
	case "is":
		return Is
	default:
		return Ident
	}
}
