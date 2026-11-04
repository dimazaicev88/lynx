// Package parser builds a Lynx syntax tree from tokens.
package parser

import (
	"fmt"

	"lynx/internal/ast"
	"lynx/internal/diag"
	"lynx/internal/token"
)

// Parser is a recursive descent parser for Lynx.
type Parser struct {
	tokens []token.Token
	pos    int
}

// Parse parses a complete token stream. The stream must end with an EOF token.
func Parse(tokens []token.Token) (*ast.Program, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("parser: empty token stream")
	}
	p := &Parser{tokens: tokens}
	return p.parseProgram()
}

func (p *Parser) parseProgram() (*ast.Program, error) {
	program := &ast.Program{}

	for !p.check(token.EOF) {
		if p.match(token.Semicolon) {
			continue
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		program.Statements = append(program.Statements, stmt)
	}

	return program, nil
}

func (p *Parser) parseStatement() (ast.Stmt, error) {
	switch {
	case p.check(token.Const), p.check(token.Var):
		return p.parseVarDecl()
	case p.check(token.Fun):
		return p.parseFunctionDecl()
	case p.check(token.Return):
		return p.parseReturn()
	case p.check(token.If):
		return p.parseIf()
	case p.check(token.For):
		return p.parseFor()
	case p.check(token.Switch):
		return p.parseSwitch()
	case p.check(token.Break):
		return p.parseSimpleControl(token.Break)
	case p.check(token.Continue):
		return p.parseSimpleControl(token.Continue)
	case p.check(token.Fallthrough):
		return p.parseSimpleControl(token.Fallthrough)
	case p.check(token.LBrace):
		return p.parseBlock()
	case p.check(token.Ident) && p.peek().Type == token.Assign:
		return p.parseAssign()
	case p.check(token.Ident) && p.peek().Type == token.LParen:
		return p.parseExprStmt()
	default:
		return nil, p.errorAt(p.current(), "expected a statement, found %s", describe(p.current()))
	}
}

func (p *Parser) parseVarDecl() (ast.Stmt, error) {
	keyword := p.advance()
	name, err := p.expect(token.Ident, "variable name")
	if err != nil {
		return nil, err
	}

	var typeExpr *ast.TypeExpr
	if p.match(token.Colon) {
		typeExpr, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}

	var value ast.Expr
	if p.match(token.Assign) {
		value, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}

	if keyword.Type == token.Const && value == nil {
		return nil, p.errorAt(keyword, "const declaration requires an initializer")
	}
	if value == nil && typeExpr == nil {
		return nil, p.errorAt(name, "variable %q requires a type annotation or an initializer", name.Literal)
	}

	p.match(token.Semicolon)

	return &ast.VarDecl{
		Position: keyword.Pos,
		Const:    keyword.Type == token.Const,
		Name:     name.Literal,
		NamePos:  name.Pos,
		Type:     typeExpr,
		Value:    value,
	}, nil
}

func (p *Parser) parseFunctionDecl() (ast.Stmt, error) {
	keyword := p.advance()
	name, err := p.expect(token.Ident, "function name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.LParen, "'('"); err != nil {
		return nil, err
	}

	params, err := p.parseParams()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RParen, "')'"); err != nil {
		return nil, err
	}

	var returnType *ast.TypeExpr
	if p.match(token.Colon) {
		returnType, err = p.parseType()
		if err != nil {
			return nil, err
		}
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	p.match(token.Semicolon)

	return &ast.FunctionDecl{
		Position:   keyword.Pos,
		Name:       name.Literal,
		NamePos:    name.Pos,
		Params:     params,
		ReturnType: returnType,
		Body:       body,
	}, nil
}

func (p *Parser) parseParams() ([]*ast.Param, error) {
	params := make([]*ast.Param, 0)

	if p.check(token.RParen) {
		return params, nil
	}

	for {
		name, err := p.expect(token.Ident, "parameter name")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Colon, "':' after parameter name"); err != nil {
			return nil, err
		}
		typ, err := p.parseType()
		if err != nil {
			return nil, err
		}

		variadic := p.match(token.Ellipsis)
		var defaultValue ast.Expr
		if p.match(token.Assign) {
			defaultValue, err = p.parseExpression()
			if err != nil {
				return nil, err
			}
		}

		params = append(params, &ast.Param{
			Position: name.Pos,
			Name:     name.Literal,
			NamePos:  name.Pos,
			Type:     typ,
			Variadic: variadic,
			Default:  defaultValue,
		})

		if p.match(token.Comma) {
			if p.check(token.RParen) {
				break
			}
			continue
		}
		break
	}

	return params, nil
}

func (p *Parser) parseReturn() (ast.Stmt, error) {
	keyword := p.advance()

	var value ast.Expr
	var err error
	if !p.check(token.Semicolon) && !p.check(token.RBrace) && !p.check(token.EOF) {
		value, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}

	p.match(token.Semicolon)

	return &ast.ReturnStmt{Position: keyword.Pos, Value: value}, nil
}

func (p *Parser) parseIf() (ast.Stmt, error) {
	keyword := p.advance()
	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	then, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	stmt := &ast.IfStmt{Position: keyword.Pos, Cond: cond, Then: then}
	if p.match(token.Else) {
		if p.check(token.If) {
			stmt.Else, err = p.parseIf()
		} else {
			stmt.Else, err = p.parseBlock()
		}
		if err != nil {
			return nil, err
		}
	}
	return stmt, nil
}

func (p *Parser) parseFor() (ast.Stmt, error) {
	keyword := p.advance()
	stmt := &ast.ForStmt{Position: keyword.Pos}
	if p.check(token.LBrace) {
		body, err := p.parseBlock()
		stmt.Body = body
		return stmt, err
	}

	threePart := false
	if p.check(token.Const) || p.check(token.Var) || (p.check(token.Ident) && p.peek().Type == token.Assign) {
		var err error
		stmt.Init, err = p.parseStatement()
		if err != nil {
			return nil, err
		}
		threePart = p.consumedSemicolon() || p.match(token.Semicolon)
		if !threePart {
			return nil, p.errorAt(p.current(), "expected ';' after for initializer")
		}
	} else if p.match(token.Semicolon) {
		threePart = true
	} else {
		cond, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Cond = cond
		threePart = p.match(token.Semicolon)
	}

	if threePart {
		if !p.check(token.Semicolon) {
			cond, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			stmt.Cond = cond
		}
		if _, err := p.expect(token.Semicolon, "';' after for condition"); err != nil {
			return nil, err
		}
		if !p.check(token.LBrace) {
			var err error
			switch {
			case p.check(token.Ident) && p.peek().Type == token.Assign:
				stmt.Post, err = p.parseAssign()
			case p.check(token.Ident) && p.peek().Type == token.LParen:
				stmt.Post, err = p.parseExprStmt()
			default:
				return nil, p.errorAt(p.current(), "expected assignment or call in for post statement")
			}
			if err != nil {
				return nil, err
			}
		}
	}

	body, err := p.parseBlock()
	stmt.Body = body
	return stmt, err
}

func (p *Parser) parseSwitch() (ast.Stmt, error) {
	keyword := p.advance()
	stmt := &ast.SwitchStmt{Position: keyword.Pos}
	var err error
	if !p.check(token.LBrace) {
		stmt.Value, err = p.parseExpression()
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(token.LBrace, "'{' after switch"); err != nil {
		return nil, err
	}

	for !p.check(token.RBrace) {
		if p.check(token.EOF) {
			return nil, p.errorAt(keyword, "unterminated switch; expected '}'")
		}
		clause := &ast.SwitchCase{Position: p.current().Pos}
		switch {
		case p.match(token.Case):
			for {
				value, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				clause.Values = append(clause.Values, value)
				if !p.match(token.Comma) {
					break
				}
			}
		case p.match(token.Default):
			clause.IsDefault = true
		default:
			return nil, p.errorAt(p.current(), "expected case or default in switch")
		}
		if _, err := p.expect(token.Colon, "':' after switch clause"); err != nil {
			return nil, err
		}

		for !p.check(token.Case) && !p.check(token.Default) && !p.check(token.RBrace) {
			if p.check(token.EOF) {
				return nil, p.errorAt(keyword, "unterminated switch; expected '}'")
			}
			if p.match(token.Semicolon) {
				continue
			}
			inner, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			clause.Body = append(clause.Body, inner)
		}
		stmt.Cases = append(stmt.Cases, clause)
	}
	p.advance()
	return stmt, nil
}

func (p *Parser) parseSimpleControl(typ token.Type) (ast.Stmt, error) {
	keyword := p.advance()
	p.match(token.Semicolon)
	switch typ {
	case token.Break:
		return &ast.BreakStmt{Position: keyword.Pos}, nil
	case token.Continue:
		return &ast.ContinueStmt{Position: keyword.Pos}, nil
	default:
		return &ast.FallthroughStmt{Position: keyword.Pos}, nil
	}
}

func (p *Parser) parseBlock() (*ast.BlockStmt, error) {
	open, err := p.expect(token.LBrace, "'{'")
	if err != nil {
		return nil, err
	}

	block := &ast.BlockStmt{Position: open.Pos}
	for !p.check(token.RBrace) {
		if p.check(token.EOF) {
			return nil, p.errorAt(open, "unterminated block; expected '}'")
		}
		if p.match(token.Semicolon) {
			continue
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		block.Statements = append(block.Statements, stmt)
	}
	p.advance()

	return block, nil
}

func (p *Parser) parseExprStmt() (ast.Stmt, error) {
	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	p.match(token.Semicolon)
	return &ast.ExprStmt{Position: value.Pos(), X: value}, nil
}

func (p *Parser) parseAssign() (ast.Stmt, error) {
	name := p.advance()
	p.advance()

	value, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	p.match(token.Semicolon)
	return &ast.AssignStmt{Position: name.Pos, Name: name.Literal, NamePos: name.Pos, Value: value}, nil
}

func (p *Parser) parseType() (*ast.TypeExpr, error) {
	if p.check(token.LParen) {
		return p.parseFunctionType()
	}

	name, err := p.expect(token.Ident, "type name")
	if err != nil {
		return nil, err
	}
	typ := &ast.TypeExpr{Position: name.Pos, Name: name.Literal}
	if p.match(token.Lt) {
		typ.Elem, err = p.parseType()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.Gt, "'>' after array element type"); err != nil {
			return nil, err
		}
	}
	typ.Nullable = p.match(token.Question)
	return typ, nil
}

func (p *Parser) parseFunctionType() (*ast.TypeExpr, error) {
	open := p.advance()

	params := make([]*ast.TypeExpr, 0)
	if !p.check(token.RParen) {
		for {
			typ, err := p.parseType()
			if err != nil {
				return nil, err
			}
			params = append(params, typ)

			if p.match(token.Comma) {
				if p.check(token.RParen) {
					break
				}
				continue
			}
			break
		}
	}

	if _, err := p.expect(token.RParen, "')'"); err != nil {
		return nil, err
	}
	if _, err := p.expect(token.Arrow, "'->'"); err != nil {
		return nil, err
	}
	returns, err := p.parseType()
	if err != nil {
		return nil, err
	}

	return &ast.TypeExpr{
		Position: open.Pos,
		Function: true,
		Params:   params,
		Returns:  returns,
	}, nil
}

func (p *Parser) parseExpression() (ast.Expr, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (ast.Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.check(token.OrOr) {
		op := p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Position: op.Pos, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (ast.Expr, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for p.check(token.AndAnd) {
		op := p.advance()
		right, err := p.parseEquality()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Position: op.Pos, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseEquality() (ast.Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.check(token.Eq) || p.check(token.NotEq) || p.check(token.StrictEq) || p.check(token.StrictNe) {
		op := p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Position: op.Pos, Op: op.Type, Left: left, Right: right}
	}
	return left, nil
}

func (p *Parser) parseComparison() (ast.Expr, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	for {
		if p.check(token.Is) || (p.check(token.Bang) && p.peek().Type == token.Is) {
			op := p.advance()
			negate := op.Type == token.Bang
			if negate {
				p.advance()
			}
			typ, err := p.parseType()
			if err != nil {
				return nil, err
			}
			left = &ast.TypeTestExpr{Position: op.Pos, X: left, Type: typ, Negate: negate}
			continue
		}
		if !p.check(token.Lt) && !p.check(token.LtEq) && !p.check(token.Gt) && !p.check(token.GtEq) {
			return left, nil
		}
		op := p.advance()
		right, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Position: op.Pos, Op: op.Type, Left: left, Right: right}
	}
}

func (p *Parser) parseAdditive() (ast.Expr, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}

	for p.check(token.Plus) || p.check(token.Minus) {
		op := p.advance()
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Position: op.Pos, Op: op.Type, Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseMultiplicative() (ast.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}

	for p.check(token.Star) || p.check(token.Slash) || p.check(token.Percent) {
		op := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Position: op.Pos, Op: op.Type, Left: left, Right: right}
	}

	return left, nil
}

func (p *Parser) parseUnary() (ast.Expr, error) {
	if p.check(token.Plus) || p.check(token.Minus) || p.check(token.Bang) {
		op := p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Position: op.Pos, Op: op.Type, X: operand}, nil
	}
	return p.parsePostfix()
}

func (p *Parser) parsePostfix() (ast.Expr, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	for {
		switch {
		case p.check(token.LParen):
			ident, ok := expr.(*ast.Ident)
			if !ok {
				return nil, p.errorAt(p.current(), "only direct function calls are supported")
			}
			p.advance()
			args, err := p.parseCallArgs()
			if err != nil {
				return nil, err
			}
			expr = &ast.CallExpr{
				Position:  ident.Position,
				Callee:    ident.Name,
				CalleePos: ident.Position,
				Args:      args,
			}
		case p.check(token.LBracket):
			bracket := p.advance()
			index, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.RBracket, "']'"); err != nil {
				return nil, err
			}
			expr = &ast.IndexExpr{Position: bracket.Pos, X: expr, Index: index}
		case p.check(token.Dot):
			p.advance()
			name, err := p.expect(token.Ident, "property name")
			if err != nil {
				return nil, err
			}
			expr = &ast.MemberExpr{Position: name.Pos, X: expr, Name: name.Literal, NamePos: name.Pos}
		default:
			return expr, nil
		}
	}
}

func (p *Parser) parseCallArgs() ([]*ast.CallArg, error) {
	args := make([]*ast.CallArg, 0)
	if p.check(token.RParen) {
		p.advance()
		return args, nil
	}

	namedSeen := false
	for {
		if p.check(token.Ident) && p.peek().Type == token.Assign {
			name := p.advance()
			p.advance()
			value, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.CallArg{
				Position: name.Pos,
				Name:     name.Literal,
				NamePos:  name.Pos,
				Value:    value,
			})
			namedSeen = true
		} else {
			if namedSeen {
				return nil, p.errorAt(p.current(), "positional argument cannot follow a named argument")
			}
			value, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, &ast.CallArg{Position: value.Pos(), Value: value})
		}

		if p.match(token.Comma) {
			if p.check(token.RParen) {
				break
			}
			continue
		}
		break
	}

	if _, err := p.expect(token.RParen, "')'"); err != nil {
		return nil, err
	}
	return args, nil
}

func (p *Parser) parsePrimary() (ast.Expr, error) {
	tok := p.current()

	switch tok.Type {
	case token.Number:
		p.advance()
		return &ast.NumberLit{Position: tok.Pos, Raw: tok.Literal}, nil
	case token.String:
		p.advance()
		return &ast.StringLit{Position: tok.Pos, Value: tok.Literal}, nil
	case token.Null:
		p.advance()
		return &ast.NullLit{Position: tok.Pos}, nil
	case token.True, token.False:
		p.advance()
		return &ast.BoolLit{Position: tok.Pos, Value: tok.Type == token.True}, nil
	case token.Ident:
		p.advance()
		return &ast.Ident{Position: tok.Pos, Name: tok.Literal}, nil
	case token.LBracket:
		return p.parseArray()
	case token.LBrace:
		return p.parseLambda()
	case token.LParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(token.RParen, "')'"); err != nil {
			return nil, err
		}
		return expr, nil
	default:
		return nil, p.errorAt(tok, "expected expression, found %s", describe(tok))
	}
}

func (p *Parser) parseLambda() (ast.Expr, error) {
	open := p.advance()

	params := make([]*ast.Param, 0)
	if !p.check(token.Arrow) {
		for {
			name, err := p.expect(token.Ident, "parameter name")
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(token.Colon, "':' after parameter name"); err != nil {
				return nil, err
			}
			typ, err := p.parseType()
			if err != nil {
				return nil, err
			}
			params = append(params, &ast.Param{
				Position: name.Pos,
				Name:     name.Literal,
				NamePos:  name.Pos,
				Type:     typ,
			})

			if p.match(token.Comma) {
				if p.check(token.Arrow) {
					break
				}
				continue
			}
			break
		}
	}

	if _, err := p.expect(token.Arrow, "'->'"); err != nil {
		return nil, err
	}

	body, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(token.RBrace, "'}'"); err != nil {
		return nil, err
	}

	return &ast.LambdaExpr{Position: open.Pos, Params: params, Body: body}, nil
}

func (p *Parser) parseArray() (ast.Expr, error) {
	open := p.advance()
	elements := make([]ast.Expr, 0)

	if !p.check(token.RBracket) {
		for {
			element, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			elements = append(elements, element)

			if p.match(token.Comma) {
				if p.check(token.RBracket) {
					break
				}
				continue
			}
			break
		}
	}

	if _, err := p.expect(token.RBracket, "']'"); err != nil {
		return nil, err
	}

	return &ast.ArrayLit{Position: open.Pos, Elements: elements}, nil
}

func (p *Parser) current() token.Token {
	return p.tokens[p.pos]
}

func (p *Parser) peek() token.Token {
	if p.pos+1 >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[p.pos+1]
}

func (p *Parser) advance() token.Token {
	tok := p.current()
	if tok.Type != token.EOF {
		p.pos++
	}
	return tok
}

func (p *Parser) check(typ token.Type) bool {
	return p.current().Type == typ
}

func (p *Parser) match(typ token.Type) bool {
	if !p.check(typ) {
		return false
	}
	p.advance()
	return true
}

func (p *Parser) consumedSemicolon() bool {
	return p.pos > 0 && p.tokens[p.pos-1].Type == token.Semicolon
}

func (p *Parser) expect(typ token.Type, what string) (token.Token, error) {
	if !p.check(typ) {
		return token.Token{}, p.errorAt(p.current(), "expected %s, found %s", what, describe(p.current()))
	}
	return p.advance(), nil
}

func (p *Parser) errorAt(tok token.Token, format string, args ...any) error {
	return diag.New(tok.Pos, format, args...)
}

func describe(tok token.Token) string {
	switch tok.Type {
	case token.EOF:
		return "end of file"
	case token.Ident:
		return fmt.Sprintf("identifier %q", tok.Literal)
	case token.Number:
		return fmt.Sprintf("number %s", tok.Literal)
	case token.String:
		return "string literal"
	case token.Const, token.Var, token.Fun, token.Return, token.Null,
		token.True, token.False, token.If, token.Else, token.For, token.Break,
		token.Continue, token.Switch, token.Case, token.Default, token.Fallthrough, token.Is:
		return fmt.Sprintf("keyword %q", tok.Literal)
	default:
		return fmt.Sprintf("%q", tok.Literal)
	}
}
