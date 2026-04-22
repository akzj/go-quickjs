package opcode

// Opcode represents a VM instruction
type Opcode uint8

// Opcode constants
const (
	OP_invalid Opcode = iota

	// === Category 1: Push Constants ===
	OP_push_i32
	OP_push_const
	OP_push_atom_value

	// === Category 2: Push Literals ===
	OP_undefined
	OP_null
	OP_push_true
	OP_push_false

	// === Category 3: Arithmetic ===
	OP_add
	OP_sub
	OP_mul
	OP_div
	OP_mod
	OP_neg

	// === Category 4: Comparison ===
	OP_eq
	OP_neq
	OP_lt
	OP_lte
	OP_gt
	OP_gte
	OP_strict_eq
	OP_strict_neq

	// === Category 5: Stack Operations ===
	OP_drop
	OP_dup

	// === Category 6: Control Flow ===
	OP_return
	OP_ret

	// === Category 7: Variables ===
	OP_get_var_undef
	OP_get_prop
	OP_put_var
	OP_put_var_init

	// === Category 8: Function Calls ===
	OP_call
	OP_call0
	OP_call1
	OP_call2
	OP_push_func
	OP_array
	OP_array_push
	OP_array_pop

	// === Category 9: Jump ===
	OP_goto
	OP_if_false
	OP_if_true

	// === Category 10: Increment/Decrement ===
	OP_post_inc
	OP_post_dec
)

// OpcodeSize returns the total size in bytes for an opcode with its operands
func OpcodeSize(op Opcode) int {
	switch op {
	case OP_push_i32, OP_push_func:
		return 5
	case OP_push_const, OP_push_atom_value:
		return 5
	case OP_call:
		return 3
	case OP_array, OP_array_push, OP_array_pop:
		return 1
	case OP_undefined, OP_null, OP_push_true, OP_push_false,
		OP_add, OP_sub, OP_mul, OP_div, OP_mod, OP_neg,
		OP_eq, OP_neq, OP_lt, OP_lte, OP_gt, OP_gte,
		OP_strict_eq, OP_strict_neq, OP_drop, OP_dup,
		OP_return, OP_ret,
		OP_get_var_undef, OP_get_prop, OP_put_var, OP_put_var_init,
		OP_call0, OP_call1, OP_call2:
		return 1
	case OP_goto, OP_if_false, OP_if_true:
		return 5
	case OP_post_inc, OP_post_dec:
		return 1
	default:
		return 1
	}
}

// StackEffect returns (pop, push) count for this opcode
func StackEffect(op Opcode) (pop, push int) {
	switch op {
	case OP_push_i32, OP_push_const, OP_push_atom_value, OP_push_func,
		OP_undefined, OP_null, OP_push_true, OP_push_false:
		return 0, 1
	case OP_add, OP_sub, OP_mul, OP_div, OP_mod, OP_neg,
		OP_eq, OP_neq, OP_lt, OP_lte, OP_gt, OP_gte,
		OP_strict_eq, OP_strict_neq:
		return 2, 1
	case OP_drop:
		return 1, 0
	case OP_dup:
		return 1, 2 // pop 1 (value), push 2 (original + duplicate)
	case OP_return, OP_ret:
		return 1, 0
	case OP_get_var_undef:
		return 0, 1
	case OP_put_var, OP_put_var_init:
		return 1, 0
	case OP_goto:
		return 0, 0
	case OP_if_false, OP_if_true:
		return 1, 0
	case OP_post_inc, OP_post_dec:
		return 1, 1 // pop value, push old value (for i++)
	case OP_call, OP_call0, OP_call1, OP_call2:
		return 2, 1
	case OP_array:
		return 2, 1 // pop n + n elements, push array
	case OP_array_push:
		return 2, 1 // pop arr, pop value, push new length
	case OP_array_pop:
		return 1, 1 // pop arr, push removed value
	default:
		return 0, 0
	}
}
