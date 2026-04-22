package opcode

// Opcode represents a VM instruction
type Opcode uint8

// Opcode constants for Stage 1
// Organized by category for clarity
const (
	OP_invalid Opcode = iota

	// === Category 1: Push Constants ===
	OP_push_i32      // push 32-bit integer (5 bytes: 1 opcode + 4 operand)
	OP_push_const    // push from constant pool (5 bytes: opcode + 4 byte index)
	OP_push_atom_value // push atom as value

	// === Category 2: Push Literals ===
	OP_undefined     // push JS_UNDEFINED
	OP_null          // push JS_NULL
	OP_push_true     // push JS_TRUE
	OP_push_false    // push JS_FALSE

	// === Category 3: Arithmetic ===
	OP_add           // add two numbers
	OP_sub           // subtract
	OP_mul           // multiply
	OP_div           // divide
	OP_mod           // modulo
	OP_neg           // negate

	// === Category 4: Comparison ===
	OP_eq            // loose equality (==)
	OP_neq           // loose inequality (!=)
	OP_lt            // less than
	OP_lte           // less than or equal
	OP_gt            // greater than
	OP_gte           // greater than or equal
	OP_strict_eq     // strict equality (===)
	OP_strict_neq    // strict inequality (!==)

	// === Category 5: Stack Operations ===
	OP_drop          // pop and discard
	OP_dup           // duplicate top value

	// === Category 6: Control Flow ===
	OP_return        // return from function
	OP_ret           // return with value (alias for return)

	// === Category 7: Variables ===
	OP_get_var_undef  // get variable (u16 index), return undefined if not found
	OP_put_var        // set variable (u16 index), pop value
	OP_put_var_init   // init variable (u16 index), pop value (let/const)

	// === Category 8: Jump ===
	OP_goto           // unconditional jump (5 bytes: opcode + 4 byte offset)
	OP_if_false       // conditional jump (5 bytes: opcode + 4 byte offset)
	OP_if_true        // conditional jump (5 bytes: opcode + 4 byte offset)
)

// OpcodeSize returns the total size in bytes for an opcode with its operands
func OpcodeSize(op Opcode) int {
	switch op {
	case OP_push_i32:
		return 5 // 1 byte opcode + 4 bytes operand
	case OP_push_const, OP_push_atom_value:
		return 5 // 1 + 4
	case OP_undefined, OP_null, OP_push_true, OP_push_false,
		OP_add, OP_sub, OP_mul, OP_div, OP_mod, OP_neg,
		OP_eq, OP_neq, OP_lt, OP_lte, OP_gt, OP_gte,
		OP_strict_eq, OP_strict_neq, OP_drop, OP_dup,
		OP_return, OP_ret,
		OP_get_var_undef, OP_put_var, OP_put_var_init:
		return 1
	case OP_goto, OP_if_false, OP_if_true:
		return 5 // opcode + 4 byte offset
	default:
		return 1
	}
}

// StackEffect returns (pop, push) count for this opcode
func StackEffect(op Opcode) (pop, push int) {
	switch op {
	case OP_push_i32, OP_push_const, OP_push_atom_value,
		OP_undefined, OP_null, OP_push_true, OP_push_false:
		return 0, 1
	case OP_add, OP_sub, OP_mul, OP_div, OP_mod, OP_neg,
		OP_eq, OP_neq, OP_lt, OP_lte, OP_gt, OP_gte,
		OP_strict_eq, OP_strict_neq:
		return 2, 1
	case OP_drop:
		return 1, 0
	case OP_dup:
		return 1, 2
	case OP_return, OP_ret:
		return 0, 0 // May pop return value
	case OP_get_var_undef:
		return 0, 1 // Get variable, push value (or undefined)
	case OP_put_var, OP_put_var_init:
		return 1, 0 // Pop value, store it
	case OP_goto:
		return 0, 0 // No stack effect
	case OP_if_false, OP_if_true:
		return 1, 0 // Pop condition, no push
	default:
		return 0, 0
	}
}