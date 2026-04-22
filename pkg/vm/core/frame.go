package core

import (
	"github.com/akzj/go-quickjs/pkg/value"
)

// StackFrame represents a function call frame
type StackFrame struct {
	Prev     *StackFrame
	Func     interface{} // function being executed
	PC       int         // program counter
	SP       int         // stack pointer (index into frame stack)
	BP       int         // base pointer (for local variable access)
	Locals   []value.JSValue // local variables
	Args     []value.JSValue // arguments
	Bytecode *Bytecode   // function's bytecode
}

// NewFrame creates a new stack frame
func NewFrame(bc *Bytecode) *StackFrame {
	return &StackFrame{
		PC:       0,
		SP:       0,
		BP:       0,
		Bytecode: bc,
		Locals:   make([]value.JSValue, bc.VarCount),
	}
}

// GetLocal returns the value of a local variable
func (f *StackFrame) GetLocal(idx int) value.JSValue {
	if idx < len(f.Locals) {
		return f.Locals[idx]
	}
	return value.Undefined()
}

// SetLocal sets a local variable
func (f *StackFrame) SetLocal(idx int, v value.JSValue) {
	if idx >= len(f.Locals) {
		// Expand locals if needed
		newLocals := make([]value.JSValue, idx+1)
		copy(newLocals, f.Locals)
		f.Locals = newLocals
	}
	f.Locals[idx] = v
}