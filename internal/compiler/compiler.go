// Package compiler wires the Lynx lexer, parser, checker and code generator.
package compiler

import (
	"lynx/internal/checker"
	"lynx/internal/codegen"
	"lynx/internal/lexer"
	"lynx/internal/parser"
)

// Compile translates Lynx source into JavaScript.
func Compile(filename string, src []byte) (string, error) {
	tokens, err := lexer.New(filename, string(src)).Lex()
	if err != nil {
		return "", err
	}

	program, err := parser.Parse(tokens)
	if err != nil {
		return "", err
	}

	if err := checker.Check(program); err != nil {
		return "", err
	}

	return codegen.Generate(program), nil
}
