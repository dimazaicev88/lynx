package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lynx/internal/diag"
)

func TestCompileSpecificationExamples(t *testing.T) {
	src := `const a=10;
const b:Number=10;
var c=10;
var d:Number=10;`

	want := "const a = 10;\n" +
		"const b = 10;\n" +
		"let c = 10;\n" +
		"let d = 10;\n"

	got, err := Compile("test.lynx", []byte(src))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if got != want {
		t.Errorf("Compile() =\n%s\nwant:\n%s", got, want)
	}
}

func TestCompileFunctions(t *testing.T) {
	src := `fun answer(): Number {
	  return 42
	}
	const value = answer()
	`

	want := "function answer() {\n" +
		"  return 42;\n" +
		"}\n" +
		"const value = answer();\n"

	got, err := Compile("test.lynx", []byte(src))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if got != want {
		t.Errorf("Compile() =\n%s\nwant:\n%s", got, want)
	}
}

func TestCompileLambda(t *testing.T) {
	src := `val sum = { x: Int, y: Int -> x + y }
	fun log(value: Number) {}
	log(sum(1, 5))
	`

	want := "const sum = (x, y) => x + y;\n" +
		"function log(value) {\n" +
		"}\n" +
		"log(sum(1, 5));\n"

	got, err := Compile("test.lynx", []byte(src))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if got != want {
		t.Errorf("Compile() =\n%s\nwant:\n%s", got, want)
	}
}

func TestCompileExamples(t *testing.T) {
	files, err := filepath.Glob("../../examples/*.lynx")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no example files found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			js, err := Compile(file, src)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if strings.TrimSpace(js) == "" {
				t.Error("Compile() produced empty output")
			}
		})
	}
}

func TestCompileTypeErrorWithSourceSnippet(t *testing.T) {
	src := []byte(`const a: Number = "x";`)
	_, err := Compile("test.lynx", src)
	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}

	formatted := diag.Format(err, src)
	if !strings.Contains(formatted, "test.lynx:1:19: error: cannot assign a value of type String") {
		t.Errorf("diagnostic = %q", formatted)
	}
	if !strings.Contains(formatted, "const a: Number = \"x\";\n") || !strings.Contains(formatted, "^") {
		t.Errorf("diagnostic lacks source snippet:\n%s", formatted)
	}
}

func TestCompileSyntaxError(t *testing.T) {
	_, err := Compile("test.lynx", []byte(`fun f() {`))
	if err == nil {
		t.Fatal("Compile() succeeded, want error")
	}
	if !strings.Contains(err.Error(), "unterminated block") {
		t.Errorf("error = %q", err)
	}
}
