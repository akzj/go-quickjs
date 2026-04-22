package core

import (
	"fmt"
	"github.com/akzj/go-quickjs/pkg/opcode"
	"github.com/akzj/go-quickjs/pkg/value"
)

// VM is the JavaScript virtual machine
type VM struct {
	ctx   *JSContext
	stack []value.JSValue
	frame *StackFrame
}

// NewVM creates a new VM instance
func NewVM(ctx *JSContext) *VM {
	return &VM{
		ctx:   ctx,
		stack: make([]value.JSValue, 0, 64),
		frame: nil,
	}
}

// Run executes bytecode and returns the result
func (vm *VM) Run() value.JSValue {
	if vm.frame == nil {
		return value.Undefined()
	}

	for vm.frame.PC < len(vm.frame.Bytecode.Code) {
		op := opcode.Opcode(vm.frame.Bytecode.Code[vm.frame.PC])
		vm.frame.PC++

		if !vm.executeOp(op) {
			// VM stopped (return or error)
			break
		}
	}

	if len(vm.stack) > 0 {
		return vm.stack[len(vm.stack)-1]
	}
	return value.Undefined()
}

// executeOp returns false if VM should stop
func (vm *VM) executeOp(op opcode.Opcode) bool {
	switch op {
	case opcode.OP_push_i32:
		v := opcode.ReadI32(vm.frame.Bytecode.Code, &vm.frame.PC)
		vm.push(value.NewInt(int64(v)))

	case opcode.OP_push_const:
		idx := opcode.ReadU32(vm.frame.Bytecode.Code, &vm.frame.PC)
		if int(idx) < len(vm.frame.Bytecode.Pool) {
			vm.push(vm.frame.Bytecode.Pool[idx])
		} else {
			vm.push(value.Undefined())
		}

	case opcode.OP_undefined:
		vm.push(value.Undefined())

	case opcode.OP_null:
		vm.push(value.Null())

	case opcode.OP_push_true:
		vm.push(value.True())

	case opcode.OP_push_false:
		vm.push(value.False())

	case opcode.OP_add:
		rhs := vm.pop()
		lhs := vm.pop()
		vm.push(value.Add(lhs, rhs))

	case opcode.OP_sub:
		rhs := vm.pop()
		lhs := vm.pop()
		vm.push(value.Sub(lhs, rhs))

	case opcode.OP_mul:
		rhs := vm.pop()
		lhs := vm.pop()
		vm.push(value.Mul(lhs, rhs))

	case opcode.OP_div:
		rhs := vm.pop()
		lhs := vm.pop()
		vm.push(value.Div(lhs, rhs))

	case opcode.OP_mod:
		rhs := vm.pop()
		lhs := vm.pop()
		vm.push(value.Mod(lhs, rhs))

	case opcode.OP_neg:
		v := vm.pop()
		vm.push(value.Sub(value.NewInt(0), v))

	case opcode.OP_eq:
		rhs := vm.pop()
		lhs := vm.pop()
		if value.Lt(lhs, rhs) || value.StrictEq(lhs, rhs) {
			vm.push(value.True())
		} else {
			vm.push(value.False())
		}

	case opcode.OP_neq:
		rhs := vm.pop()
		lhs := vm.pop()
		if !value.StrictEq(lhs, rhs) {
			vm.push(value.True())
		} else {
			vm.push(value.False())
		}

	case opcode.OP_strict_eq:
		rhs := vm.pop()
		lhs := vm.pop()
		if value.StrictEq(lhs, rhs) {
			vm.push(value.True())
		} else {
			vm.push(value.False())
		}

	case opcode.OP_strict_neq:
		rhs := vm.pop()
		lhs := vm.pop()
		if !value.StrictEq(lhs, rhs) {
			vm.push(value.True())
		} else {
			vm.push(value.False())
		}

	case opcode.OP_lt:
		rhs := vm.pop()
		lhs := vm.pop()
		if value.Lt(lhs, rhs) {
			vm.push(value.True())
		} else {
			vm.push(value.False())
		}

	case opcode.OP_lte:
		rhs := vm.pop()
		lhs := vm.pop()
		if value.Lte(lhs, rhs) {
			vm.push(value.True())
		} else {
			vm.push(value.False())
		}

	case opcode.OP_gt:
		rhs := vm.pop()
		lhs := vm.pop()
		if value.Gt(lhs, rhs) {
			vm.push(value.True())
		} else {
			vm.push(value.False())
		}

	case opcode.OP_gte:
		rhs := vm.pop()
		lhs := vm.pop()
		if value.Gte(lhs, rhs) {
			vm.push(value.True())
		} else {
			vm.push(value.False())
		}

	case opcode.OP_drop:
		vm.pop()

	case opcode.OP_dup:
		v := vm.peek()
		vm.push(v)

	// === Category 7: Variables ===
	case opcode.OP_get_var_undef:
		idx := opcode.ReadU16(vm.frame.Bytecode.Code, &vm.frame.PC)
		// Look up variable by index
		if int(idx) < len(vm.frame.Locals) {
			vm.push(vm.frame.Locals[idx])
		} else {
			vm.push(value.Undefined())
		}

	case opcode.OP_put_var, opcode.OP_put_var_init:
		idx := opcode.ReadU16(vm.frame.Bytecode.Code, &vm.frame.PC)
		v := vm.pop()
		// Set local variable by index
		if idx >= uint16(len(vm.frame.Locals)) {
			// Expand locals array
			newLocals := make([]value.JSValue, idx+1)
			copy(newLocals, vm.frame.Locals)
			vm.frame.Locals = newLocals
		}
		vm.frame.Locals[idx] = v

	// === Category 8: Control Flow ===
	case opcode.OP_goto:
		offset := opcode.ReadI32(vm.frame.Bytecode.Code, &vm.frame.PC)
		vm.frame.PC += int(offset)

	case opcode.OP_if_false:
		offset := opcode.ReadI32(vm.frame.Bytecode.Code, &vm.frame.PC)
		v := vm.pop()
		if !value.ToBool(v) {
			vm.frame.PC += int(offset)
		}

	case opcode.OP_if_true:
		offset := opcode.ReadI32(vm.frame.Bytecode.Code, &vm.frame.PC)
		v := vm.pop()
		if value.ToBool(v) {
			vm.frame.PC += int(offset)
		}

	case opcode.OP_return, opcode.OP_ret:
		// For simple eval, just return
		return false

	default:
		panic(fmt.Sprintf("unimplemented opcode: %v", op))
	}
	return true
}

func (vm *VM) push(v value.JSValue) {
	vm.stack = append(vm.stack, v)
}

func (vm *VM) pop() value.JSValue {
	n := len(vm.stack)
	v := vm.stack[n-1]
	vm.stack = vm.stack[:n-1]
	return v
}

func (vm *VM) peek() value.JSValue {
	n := len(vm.stack)
	if n == 0 {
		return value.Undefined()
	}
	return vm.stack[n-1]
}