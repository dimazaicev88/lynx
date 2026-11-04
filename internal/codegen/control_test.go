package codegen

import (
	"strings"
	"testing"
)

func TestGenerateControlFlow(t *testing.T) {
	src := `fun classify(value: Number?): String {
  if value != null && value > 0 {
    return "positive"
  } else if value == null {
    return "null"
  } else {
    return "other"
  }
}

for var i = 0; i < 3; i = i + 1 {
  if i == 1 { continue }
  println(toString(i))
}

switch 2 {
case 0:
  println("zero")
case 1, 2:
  fallthrough
default:
  println("other")
}`
	got := generate(t, src)
	checks := []string{
		`if (value !== null && value > 0) {`,
		`} else if (value === null) {`,
		`for (let i = 0; i < 3; i = i + 1) {`,
		`console.log(String(i));`,
		`switch (2) {`,
		"case 1:\n  case 2: {",
		`console.log("other");`,
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Generate() output does not contain %q:\n%s", want, got)
		}
	}
}

func TestGenerateSafeIndexAndTypeTests(t *testing.T) {
	got := generate(t, `val x = values[0]
val isText = value is String
val notNumber = value !is Number
val present = true || false`)
	checks := []string{
		`const x = (values[0] ?? null);`,
		`const isText = (typeof value === "string");`,
		`const notNumber = !(typeof value === "number");`,
		`const present = true || false;`,
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("Generate() output does not contain %q:\n%s", want, got)
		}
	}
}
