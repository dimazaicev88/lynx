// Package diag provides source-positioned compiler diagnostics.
package diag

import (
	"fmt"
	"strings"

	"lynx/internal/token"
)

// Error is a single diagnostic attached to a source position.
type Error struct {
	Pos     token.Position
	Message string
}

// New creates a diagnostic with a formatted message.
func New(pos token.Position, format string, args ...any) *Error {
	return &Error{Pos: pos, Message: fmt.Sprintf(format, args...)}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos.String(), e.Message)
}

// List holds several diagnostics, for example all type errors in a file.
type List struct {
	Errors []*Error
}

func (l *List) Add(err *Error) {
	l.Errors = append(l.Errors, err)
}

func (l *List) Empty() bool {
	return len(l.Errors) == 0
}

func (l *List) Error() string {
	parts := make([]string, 0, len(l.Errors))
	for _, err := range l.Errors {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "\n")
}

// Format renders an error together with the offending source line and a caret.
func Format(err error, src []byte) string {
	if err == nil {
		return ""
	}

	switch e := err.(type) {
	case *Error:
		return formatOne(e, src)
	case *List:
		parts := make([]string, 0, len(e.Errors))
		for _, item := range e.Errors {
			parts = append(parts, formatOne(item, src))
		}
		return strings.Join(parts, "\n")
	default:
		return err.Error()
	}
}

func formatOne(e *Error, src []byte) string {
	var b strings.Builder
	b.WriteString(e.Pos.String())
	b.WriteString(": error: ")
	b.WriteString(e.Message)

	if e.Pos.Line < 1 {
		return b.String()
	}

	lines := strings.Split(string(src), "\n")
	if e.Pos.Line > len(lines) {
		return b.String()
	}

	line := strings.TrimSuffix(lines[e.Pos.Line-1], "\r")
	b.WriteByte('\n')
	b.WriteString(line)
	b.WriteByte('\n')

	column := e.Pos.Column
	if column < 1 {
		column = 1
	}
	runes := []rune(line)
	for i := 0; i < column-1; i++ {
		if i < len(runes) && runes[i] == '\t' {
			b.WriteByte('\t')
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteByte('^')
	return b.String()
}
