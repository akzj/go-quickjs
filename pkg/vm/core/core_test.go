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