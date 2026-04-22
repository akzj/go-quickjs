package core

import (
	"testing"

	"github.com/akzj/go-quickjs/pkg/opcode"
	"github.com/akzj/go-quickjs/pkg/value"
)

func TestEvalInteger(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	// Compile "1 + 1" manually for Stage 1
	// Format: push_i32(1) push_i32(1) add return
	bc := &Bytecode{
		Code: []byte{
			byte(opcode.OP_push_i32), 1, 0, 0, 0, // push 1
			byte(opcode.OP_push_i32), 1, 0, 0, 0, // push 1
			byte(opcode.OP_add),
			byte(opcode.OP_return),
		},
		VarCount: 0,
	}

	result := ctx.RunBytecode(bc)

	if n, ok := result.(value.IntValue); !ok || int64(n) != 2 {
		t.Errorf("expected 2, got %v (type %T)", result, result)
	}
}

func TestEvalArithmetic(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	tests := []struct {
		name     string
		expected int64
		code     []byte
	}{
		{"1 + 1", 2, []byte{
			byte(opcode.OP_push_i32), 1, 0, 0, 0,
			byte(opcode.OP_push_i32), 1, 0, 0, 0,
			byte(opcode.OP_add),
			byte(opcode.OP_return),
		}},
		{"2 * 3", 6, []byte{
			byte(opcode.OP_push_i32), 2, 0, 0, 0,
			byte(opcode.OP_push_i32), 3, 0, 0, 0,
			byte(opcode.OP_mul),
			byte(opcode.OP_return),
		}},
		{"10 - 3", 7, []byte{
			byte(opcode.OP_push_i32), 10, 0, 0, 0,
			byte(opcode.OP_push_i32), 3, 0, 0, 0,
			byte(opcode.OP_sub),
			byte(opcode.OP_return),
		}},
		{"8 / 2", 4, []byte{
			byte(opcode.OP_push_i32), 8, 0, 0, 0,
			byte(opcode.OP_push_i32), 2, 0, 0, 0,
			byte(opcode.OP_div),
			byte(opcode.OP_return),
		}},
		{"7 % 3", 1, []byte{
			byte(opcode.OP_push_i32), 7, 0, 0, 0,
			byte(opcode.OP_push_i32), 3, 0, 0, 0,
			byte(opcode.OP_mod),
			byte(opcode.OP_return),
		}},
		{"negate -5", -5, []byte{
			byte(opcode.OP_push_i32), 5, 0, 0, 0,
			byte(opcode.OP_neg),
			byte(opcode.OP_return),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &Bytecode{Code: tt.code, VarCount: 0}
			result := ctx.RunBytecode(bc)

			if n, ok := result.(value.IntValue); !ok || int64(n) != tt.expected {
				t.Errorf("expected %d, got %v (type %T)", tt.expected, result, result)
			}
		})
	}
}

func TestEvalLiterals(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	tests := []struct {
		name     string
		code     []byte
		expected value.JSValue
	}{
		{"true", []byte{byte(opcode.OP_push_true), byte(opcode.OP_return)}, value.True()},
		{"false", []byte{byte(opcode.OP_push_false), byte(opcode.OP_return)}, value.False()},
		{"undefined", []byte{byte(opcode.OP_undefined), byte(opcode.OP_return)}, value.Undefined()},
		{"null", []byte{byte(opcode.OP_null), byte(opcode.OP_return)}, value.Null()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &Bytecode{Code: tt.code, VarCount: 0}
			result := ctx.RunBytecode(bc)

			if result.Tag() != tt.expected.Tag() {
				t.Errorf("expected tag %v, got %v", tt.expected.Tag(), result.Tag())
			}
		})
	}
}

func TestEvalComparison(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	tests := []struct {
		name     string
		expected bool
		code     []byte
	}{
		{"1 < 2", true, []byte{
			byte(opcode.OP_push_i32), 1, 0, 0, 0,
			byte(opcode.OP_push_i32), 2, 0, 0, 0,
			byte(opcode.OP_lt),
			byte(opcode.OP_return),
		}},
		{"2 > 1", true, []byte{
			byte(opcode.OP_push_i32), 2, 0, 0, 0,
			byte(opcode.OP_push_i32), 1, 0, 0, 0,
			byte(opcode.OP_gt),
			byte(opcode.OP_return),
		}},
		{"1 === 1", true, []byte{
			byte(opcode.OP_push_i32), 1, 0, 0, 0,
			byte(opcode.OP_push_i32), 1, 0, 0, 0,
			byte(opcode.OP_strict_eq),
			byte(opcode.OP_return),
		}},
		{"1 !== 2", true, []byte{
			byte(opcode.OP_push_i32), 1, 0, 0, 0,
			byte(opcode.OP_push_i32), 2, 0, 0, 0,
			byte(opcode.OP_strict_neq),
			byte(opcode.OP_return),
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &Bytecode{Code: tt.code, VarCount: 0}
			result := ctx.RunBytecode(bc)

			if b, ok := result.(value.BoolValue); !ok || bool(b) != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvalSimpleExpression(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	tests := []struct {
		source   string
		expected int64
	}{
		{"1 + 1", 2},
		{"2 * 3", 6},
		{"10 - 4", 6},
		{"8 / 2", 4},
		{"7 % 3", 1},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := ctx.CompileAndRun(tt.source)

			if n, ok := result.(value.IntValue); !ok || int64(n) != tt.expected {
				t.Errorf("expected %d, got %v (type %T)", tt.expected, result, result)
			}
		})
	}
}

func TestStackOperations(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	// dup: push 5, dup -> stack [5, 5]
	bc := &Bytecode{
		Code: []byte{
			byte(opcode.OP_push_i32), 5, 0, 0, 0,
			byte(opcode.OP_dup),
			byte(opcode.OP_add), // adds them: 5 + 5 = 10
			byte(opcode.OP_return),
		},
		VarCount: 0,
	}

	result := ctx.RunBytecode(bc)

	if n, ok := result.(value.IntValue); !ok || int64(n) != 10 {
		t.Errorf("expected 10, got %v (type %T)", result, result)
	}
}

func TestVariables(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	tests := []struct {
		name     string
		source   string
		expected int64
	}{
		{"var declaration", "var x = 5; x", 5},
		{"var multiple", "var x = 2; var y = 3; x + y", 5},
		{"var reassign", "var x = 1; x = 5; x", 5},
		{"let basic", "let x = 10; x", 10},
		{"let reassign", "let x = 1; x = 2; x", 2},
		{"simple expression", "var x = 1; x + 1", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctx.CompileAndRun(tt.source)
			if n, ok := result.(value.IntValue); !ok || int64(n) != tt.expected {
				t.Errorf("expected %d, got %v (type %T)", tt.expected, result, result)
			}
		})
	}
}

func TestIfStatement(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	tests := []struct {
		name     string
		source   string
		expected int64
	}{
		{"if true", "if (1) 5", 5},
		{"if else true", "if (1) 5 else 10", 5},
		{"if else false", "if (0) 5 else 10", 10},
		{"if var true", "var x = 1; if (x) 100 else 200", 100},
		{"if var false", "var x = 0; if (x) 100 else 200", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ctx.CompileAndRun(tt.source)
			if n, ok := result.(value.IntValue); !ok || int64(n) != tt.expected {
				t.Errorf("%s: expected %d, got %v", tt.source, tt.expected, result)
			}
		})
	}
}

func TestWhileLoop(t *testing.T) {
	rt := NewRuntime()
	ctx := NewContext(rt)

	// Test: while (0) 1 - condition is false immediately
	// Should return undefined (while has no value)
	result := ctx.CompileAndRun("while (0) 1")
	if _, ok := result.(value.IntValue); ok {
		t.Errorf("while (0) 1: expected undefined, got int %v", result)
	}

	// Test: var i = 0; while (i < 0) i = i + 1; i
	// Condition false immediately, should return 0
	result2 := ctx.CompileAndRun("var i = 0; while (i < 0) i = i + 1; i")
	if n, ok := result2.(value.IntValue); !ok || int64(n) != 0 {
		t.Errorf("while (i < 0): expected 0, got %v", result2)
	}

	// Test: while (i < 3) i = i + 1; i - was broken, now fixed
	result3 := ctx.CompileAndRun("var i = 0; while (i < 3) i = i + 1; i")
	if n, ok := result3.(value.IntValue); !ok || int64(n) != 3 {
		t.Errorf("while (i < 3) i = i + 1: expected 3, got %v", result3)
	}
}

func TestFunctionDeclaration(t *testing.T) {
	ctx := NewContext(nil)

	// Test: function f() { return 42; } f()
	result := ctx.CompileAndRun("function f() { return 42; } f()")
	if n, ok := result.(value.IntValue); !ok || int64(n) != 42 {
		t.Errorf("function f() { return 42; } f(): expected 42, got %v", result)
	}

	// Test: function add() { return 5 + 3; } add()
	result2 := ctx.CompileAndRun("function add() { return 5 + 3; } add()")
	if n, ok := result2.(value.IntValue); !ok || int64(n) != 8 {
		t.Errorf("function add() { return 5 + 3; } add(): expected 8, got %v", result2)
	}

}
