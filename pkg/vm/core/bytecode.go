package core

import (
	"github.com/akzj/go-quickjs/pkg/opcode"
	"github.com/akzj/go-quickjs/pkg/value"
)

// Bytecode represents compiled JavaScript code
type Bytecode struct {
	Code     []byte          // instruction bytes
	Pool     []value.JSValue // constant pool
	VarCount int            // number of local variables
	VarNames []string       // variable names (index -> name)
	ArgCount int            // number of arguments
}

// FunctionDef represents a function definition
type FunctionDef struct {
	Bytecode *Bytecode
	Name     string
	ArgCount int
	VarCount int
}

// Opcode constants for this package (references opcode package)
const (
	OpPushI32       = opcode.OP_push_i32
	OpPushConst     = opcode.OP_push_const
	OpAdd           = opcode.OP_add
	OpSub           = opcode.OP_sub
	OpMul           = opcode.OP_mul
	OpDiv           = opcode.OP_div
	OpMod           = opcode.OP_mod
	OpNeg           = opcode.OP_neg
	OpReturn        = opcode.OP_return
	OpUndefined     = opcode.OP_undefined
	OpNull          = opcode.OP_null
	OpPushTrue      = opcode.OP_push_true
	OpPushFalse     = opcode.OP_push_false
	OpDrop          = opcode.OP_drop
	OpDup           = opcode.OP_dup
	// Comparison
	OpEq          = opcode.OP_eq
	OpNeq         = opcode.OP_neq
	OpLt          = opcode.OP_lt
	OpLte         = opcode.OP_lte
	OpGt          = opcode.OP_gt
	OpGte         = opcode.OP_gte
	OpStrictEq    = opcode.OP_strict_eq
	OpStrictNeq   = opcode.OP_strict_neq
	// Variables
	OpGetVarUndef  = opcode.OP_get_var_undef
	OpPutVar       = opcode.OP_put_var
	OpPutVarInit   = opcode.OP_put_var_init
	// Control Flow
	OpGoto         = opcode.OP_goto
	OpIfFalse      = opcode.OP_if_false
	OpIfTrue       = opcode.OP_if_true
)