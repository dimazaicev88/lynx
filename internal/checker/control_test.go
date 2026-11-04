package checker

import (
	"strings"
	"testing"
)

func TestCheckControlFlowAndNarrowing(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{"boolean operators", `val flag = !false || 1 < 2 && 3 >= 3`},
		{"if returns on every path", `fun f(x: Number): String { if x > 0 { return "positive" } else { return "other" } }`},
		{"nullable early return", `fun f(value: String?): Number { if value == null { return 0 } return value.length }`},
		{"nullable short circuit", `fun f(value: Number?): Boolean { return value != null && value > 0 }`},
		{"unknown type test", `fun f(value: Unknown): String { if value is String { return value } return "other" }`},
		{"negated type test", `fun f(value: Unknown): Number { if value !is Number { return 0 } return value }`},
		{"definite assignment in branches", `var x: Number; if true { x = 1 } else { x = 2 } println(x)`},
		{"for loop", `for var i = 0; i < 3; i = i + 1 { if i == 1 { continue } println(toString(i)) }`},
		{"condition loop", `var i = 0; for i < 3 { i = i + 1 }`},
		{"infinite loop return", `fun f(): Number { for { return 1 } }`},
		{"switch returns", `fun f(x: Number): String { switch x { case 0: return "zero" default: return "other" } }`},
		{"switch break reaches continuation", `fun f(x: Number): Number { switch x { case 0: return 0 default: break } return 1 }`},
		{"definite assignment in switch", `var x: Number; switch 1 { case 0: x = 0 default: x = 1 } println(x)`},
		{"expressionless switch", `fun f(x: Number): String { switch { case x < 0: return "negative" default: return "other" } }`},
		{"generic array and safe index", `fun f(values: Array<Number>): Number { val first = values[0]; if first == null { return 0 } return first }`},
		{"builtins", `println(toString(42))`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkSource(t, tt.src); err != nil {
				t.Errorf("Check() error = %v", err)
			}
		})
	}
}

func TestCheckControlFlowErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"number condition", `if 1 {}`, "condition must have type Boolean"},
		{"incompatible equality", `val x = 1 == "1"`, "cannot compare values of types Number and String"},
		{"invalid logical operand", `val x = true && 1`, "logical operator requires Boolean"},
		{"missing return path", `fun f(x: Boolean): Number { if x { return 1 } }`, "does not return a value on every path"},
		{"uninitialized read", `var x: Number; println(x)`, "used before initialization"},
		{"initialization on one path", `var x: Number; if true { x = 1 } println(x)`, "used before initialization"},
		{"nullable without check", `fun f(x: String?): Number { return x.length }`, "may be null"},
		{"unknown without check", `fun f(x: Unknown): String { return x }`, "cannot return a value of type Unknown"},
		{"array element mismatch", `val x: Array<Number> = ["one"]`, "cannot assign a value of type Array<String>"},
		{"safe index result", `val x: Number = [1][0]`, "cannot assign a value of type Number?"},
		{"break outside loop", `break`, "break statement outside"},
		{"continue outside loop", `continue`, "continue statement outside"},
		{"incompatible switch case", `switch 1 { case "one": break }`, "cannot compare switch value"},
		{"switch may not initialize", `var x: Number; switch 1 { case 0: x = 0 } println(x)`, "used before initialization"},
		{"duplicate default", `switch 1 { default: break default: break }`, "more than one default"},
		{"final fallthrough", `switch 1 { case 1: fallthrough }`, "fallthrough must be the last statement"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSource(t, tt.src)
			if err == nil {
				t.Fatal("Check() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %q, want substring %q", err, tt.want)
			}
		})
	}
}
