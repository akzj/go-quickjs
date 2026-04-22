package core

import (
	"github.com/akzj/go-quickjs/pkg/value"
)

// StackFrame represents a function call frame
type StackFrame struct {
	Prev      *StackFrame
	PC        int
	Locals_   []value.JSValue
	Bytecode_ *Bytecode // core.Bytecode
}

// NewFrame creates a new stack frame
func NewFrame(bc *Bytecode) *StackFrame {
	return &StackFrame{
		PC:        0,
		Locals_:   make([]value.JSValue, bc.VarCount),
		Bytecode_: bc,
	}
}

// Bytecode returns the bytecode being executed
func (f *StackFrame) Bytecode() []byte {
	if f.Bytecode_ != nil {
		return f.Bytecode_.Code
	}
	return nil
}

// PoolLen returns the constant pool length
func (f *StackFrame) PoolLen() int {
	if f.Bytecode_ != nil {
		return len(f.Bytecode_.Pool)
	}
	return 0
}

// PoolVal returns the value at pool index
func (f *StackFrame) PoolVal(idx int) value.JSValue {
	if f.Bytecode_ != nil && idx < len(f.Bytecode_.Pool) {
		return f.Bytecode_.Pool[idx]
	}
	return value.Undefined()
}

// LocalsLen returns the locals array length
func (f *StackFrame) LocalsLen() int {
	return len(f.Locals_)
}

// LocalsVal returns the value at locals index
func (f *StackFrame) LocalsVal(idx int) value.JSValue {
	if idx < len(f.Locals_) {
		return f.Locals_[idx]
	}
	return value.Undefined()
}

// SetLocal sets a local variable
func (f *StackFrame) SetLocal(idx int, v value.JSValue) {
	if idx >= len(f.Locals_) {
		newLocals := make([]value.JSValue, idx+1)
		copy(newLocals, f.Locals_)
		f.Locals_ = newLocals
	}
	f.Locals_[idx] = v
}
