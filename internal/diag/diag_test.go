package diag

import (
	"strings"
	"testing"

	"lynx/internal/token"
)

func TestFormatSingleError(t *testing.T) {
	src := []byte("const a = 1;\nconst b = @;")
	err := New(
		token.Position{Filename: "test.lynx", Line: 2, Column: 11},
		"unexpected character %q", "@",
	)

	got := Format(err, src)
	want := "test.lynx:2:11: error: unexpected character \"@\"\n" +
		"const b = @;\n" +
		"          ^"
	if got != want {
		t.Errorf("Format() =\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatErrorList(t *testing.T) {
	src := []byte("const a = b;\nconst c = d;")
	errs := &List{}
	errs.Add(New(token.Position{Filename: "test.lynx", Line: 1, Column: 11}, "undefined variable %q", "b"))
	errs.Add(New(token.Position{Filename: "test.lynx", Line: 2, Column: 11}, "undefined variable %q", "d"))

	got := Format(errs, src)
	for _, want := range []string{
		"test.lynx:1:11: error: undefined variable \"b\"",
		"test.lynx:2:11: error: undefined variable \"d\"",
		"const a = b;",
		"const c = d;",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Format() missing %q:\n%s", want, got)
		}
	}
}

func TestFormatTabIndentation(t *testing.T) {
	src := []byte("\tconst b = @;")
	err := New(token.Position{Filename: "test.lynx", Line: 1, Column: 12}, "unexpected character")

	got := Format(err, src)
	lastLine := got[strings.LastIndex(got, "\n")+1:]
	if !strings.HasPrefix(lastLine, "\t") || !strings.HasSuffix(lastLine, "^") {
		t.Errorf("caret line = %q, want tab-indented caret", lastLine)
	}
}
