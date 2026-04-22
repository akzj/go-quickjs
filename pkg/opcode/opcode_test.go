package opcode

import "testing"

func TestOpcodeSize(t *testing.T) {
	tests := []struct {
		op       Opcode
		expected int
	}{
		{OP_push_i32, 5},
		{OP_push_const, 5},
		{OP_undefined, 1},
		{OP_add, 1},
		{OP_return, 1},
	}

	for _, tt := range tests {
		if got := OpcodeSize(tt.op); got != tt.expected {
			t.Errorf("OpcodeSize(%v) = %d, want %d", tt.op, got, tt.expected)
		}
	}
}

func TestStackEffect(t *testing.T) {
	tests := []struct {
		op       Opcode
		wantPop  int
		wantPush int
	}{
		{OP_push_i32, 0, 1},
		{OP_undefined, 0, 1},
		{OP_add, 2, 1},
		{OP_drop, 1, 0},
		{OP_dup, 1, 2},
	}

	for _, tt := range tests {
		pop, push := StackEffect(tt.op)
		if pop != tt.wantPop || push != tt.wantPush {
			t.Errorf("StackEffect(%v) = (%d, %d), want (%d, %d)",
				tt.op, pop, push, tt.wantPop, tt.wantPush)
		}
	}
}

func TestReadWriteI32(t *testing.T) {
	code := make([]byte, 0, 4)
	WriteI32(&code, 0x12345678)

	pc := 0
	got := ReadI32(code, &pc)
	if got != 0x12345678 {
		t.Errorf("ReadI32() = 0x%08x, want 0x12345678", got)
	}
	if pc != 4 {
		t.Errorf("pc = %d, want 4", pc)
	}
}

func TestReadWriteU8(t *testing.T) {
	code := make([]byte, 0, 1)
	WriteU8(&code, 255)

	pc := 0
	got := ReadU8(code, &pc)
	if got != 255 {
		t.Errorf("ReadU8() = %d, want 255", got)
	}
	if pc != 1 {
		t.Errorf("pc = %d, want 1", pc)
	}
}
