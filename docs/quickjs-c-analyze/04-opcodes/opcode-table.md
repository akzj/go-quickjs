# QuickJS Opcode Table

## Source
`quickjs-opcode.h` - Macro-based opcode definition

## Opcode Format
Each opcode is defined with:
- `id`: Opcode identifier (e.g., `push_i32`)
- `size`: Byte size of opcode + operands
- `n_pop`: Number of stack values consumed
- `n_push`: Number of values pushed to stack
- `format`: Operand format (e.g., `i32`, `atom`, `const`)

## Format Types
| Format | Size | Description |
|--------|------|-------------|
| `none` | 0 | No operand |
| `none_int` | 0 | No operand, used for small integers |
| `i32` | 4 | 32-bit signed integer |
| `i8` | 1 | 8-bit signed integer |
| `u8` | 1 | 8-bit unsigned integer |
| `u16` | 2 | 16-bit unsigned integer |
| `const` | 4 | Constant pool index (32-bit) |
| `const8` | 1 | Constant pool index (8-bit) |
| `atom` | 4 | Atom index (32-bit) |
| `loc` | 2 | Local variable index (16-bit) |
| `arg` | 2 | Argument index (16-bit) |
| `var_ref` | 2 | Variable reference index (16-bit) |
| `label` | 4 | Jump offset (32-bit relative) |
| `npop` | 2 | Number of arguments (16-bit) |

---

## Category 1: Push Constants

| Opcode | Size | Pop | Push | Format | Description |
|--------|------|-----|------|--------|-------------|
| `push_i32` | 5 | 0 | 1 | i32 | Push 32-bit integer constant |
| `push_bigint_i32` | 5 | 0 | 1 | i32 | Push BigInt from 32-bit integer |
| `push_const` | 5 | 0 | 1 | const | Push from constant pool |
| `fclosure` | 5 | 0 | 1 | const | Create closure (must follow push_const) |
| `push_atom_value` | 5 | 0 | 1 | atom | Push atom as value |
| `private_symbol` | 5 | 0 | 1 | atom | Push private symbol |

### Short Opcodes (when SHORT_OPCODES defined)
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `push_minus1` | 1 | 0 | 1 | Push -1 |
| `push_0` | 1 | 0 | 1 | Push 0 |
| `push_1` | 1 | 0 | 1 | Push 1 |
| `push_2` | 1 | 0 | 1 | Push 2 |
| `push_3` | 1 | 0 | 1 | Push 3 |
| `push_4` | 1 | 0 | 1 | Push 4 |
| `push_5` | 1 | 0 | 1 | Push 5 |
| `push_6` | 1 | 0 | 1 | Push 6 |
| `push_7` | 1 | 0 | 1 | Push 7 |
| `push_i8` | 2 | 0 | 1 | i8 |
| `push_i16` | 3 | 0 | 1 | i16 |
| `push_const8` | 2 | 0 | 1 | const8 |
| `fclosure8` | 2 | 0 | 1 | const8 |
| `push_empty_string` | 1 | 0 | 1 | Push empty string |

---

## Category 2: Push Literals

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `undefined` | 1 | 0 | 1 | Push JS_UNDEFINED |
| `null` | 1 | 0 | 1 | Push JS_NULL |
| `push_true` | 1 | 0 | 1 | Push JS_TRUE |
| `push_false` | 1 | 0 | 1 | Push JS_FALSE |
| `push_this` | 1 | 0 | 1 | Push `this` (only at function start) |

---

## Category 3: Object Special Opcodes

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `object` | 1 | 0 | 1 | Create empty object |
| `special_object` | 2 | 0 | 1 | u8 argument specifies type |
| `rest` | 3 | 0 | 1 | u16: Create rest parameters array |

**Special Object Types (u8 argument):**
- `OP_SPECIAL_OBJECT_ARGUMENTS` (0)
- `OP_SPECIAL_OBJECT_MAPPED_ARGUMENTS` (1)
- `OP_SPECIAL_OBJECT_THIS_FUNC` (2)
- `OP_SPECIAL_OBJECT_NEW_TARGET` (3)
- `OP_SPECIAL_OBJECT_HOME_OBJECT` (4)
- `OP_SPECIAL_OBJECT_VAR_OBJECT` (5)
- `OP_SPECIAL_OBJECT_IMPORT_META` (6)

---

## Category 4: Stack Manipulation

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `drop` | 1 | 1 | 0 | Discard top value |
| `nip` | 1 | 2 | 1 | `a b -> b` (remove 2nd value) |
| `nip1` | 1 | 3 | 2 | `a b c -> b c` |
| `dup` | 1 | 1 | 2 | `a -> a a` |
| `dup1` | 1 | 2 | 3 | `a b -> a a b` |
| `dup2` | 1 | 2 | 4 | `a b -> a b a b` |
| `dup3` | 1 | 3 | 6 | `a b c -> a b c a b c` |
| `insert2` | 1 | 2 | 3 | `obj a -> a obj a` (dup_x1) |
| `insert3` | 1 | 3 | 4 | `obj prop a -> a obj prop a` (dup_x2) |
| `insert4` | 1 | 4 | 5 | `this obj prop a -> a this obj prop a` (dup_x3) |
| `perm3` | 1 | 3 | 3 | `obj a b -> a obj b` (213) |
| `perm4` | 1 | 4 | 4 | `obj prop a b -> a obj prop b` |
| `perm5` | 1 | 5 | 5 | `this obj prop a b -> a this obj prop b` |
| `swap` | 1 | 2 | 2 | `a b -> b a` |
| `swap2` | 1 | 4 | 4 | `a b c d -> c d a b` |
| `rot3l` | 1 | 3 | 3 | `x a b -> a b x` (231) |
| `rot3r` | 1 | 3 | 3 | `a b x -> x a b` (312) |
| `rot4l` | 1 | 4 | 4 | `x a b c -> a b c x` |
| `rot5l` | 1 | 5 | 5 | `x a b c d -> a b c d x` |

### Short Opcodes
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `get_loc0` | 1 | 0 | 1 | Get local[0] |
| `get_loc1` | 1 | 0 | 1 | Get local[1] |
| `get_loc2` | 1 | 0 | 1 | Get local[2] |
| `get_loc3` | 1 | 0 | 1 | Get local[3] |
| `put_loc0` | 1 | 1 | 0 | Set local[0], pop |
| `put_loc1` | 1 | 1 | 0 | Set local[1], pop |
| `put_loc2` | 1 | 1 | 0 | Set local[2], pop |
| `put_loc3` | 1 | 1 | 0 | Set local[3], pop |
| `set_loc0` | 1 | 1 | 1 | Set local[0], keep value |
| `set_loc1` | 1 | 1 | 1 | Set local[1], keep value |
| `set_loc2` | 1 | 1 | 1 | Set local[2], keep value |
| `set_loc3` | 1 | 1 | 1 | Set local[3], keep value |
| `get_arg0-3`, `put_arg0-3`, `set_arg0-3` | 1 | varies | varies | Same pattern for arguments |
| `get_var_ref0-3`, `put_var_ref0-3`, `set_var_ref0-3` | 1 | varies | varies | Same pattern for var refs |

---

## Category 5: Variable Access

### Local Variables
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `get_loc` | 3 | 0 | 1 | loc (u16): Get local variable |
| `put_loc` | 3 | 1 | 0 | loc (u16): Set local variable, pop |
| `set_loc` | 3 | 1 | 1 | loc (u16): Set local variable, keep |
| `get_arg` | 3 | 0 | 1 | arg (u16): Get argument |
| `put_arg` | 3 | 1 | 0 | arg (u16): Set argument, pop |
| `set_arg` | 3 | 1 | 1 | arg (u16): Set argument, keep |
| `get_loc_check` | 3 | 0 | 1 | loc (u16): Get with uninit check |
| `put_loc_check` | 3 | 1 | 0 | loc (u16): Set with uninit check |
| `set_loc_check` | 3 | 1 | 1 | loc (u16): Set with uninit check |
| `put_loc_check_init` | 3 | 1 | 0 | loc (u16): Init check only |
| `get_loc_checkthis` | 3 | 0 | 1 | loc (u16): Get with uninit check for `this` |
| `set_loc_uninitialized` | 3 | 0 | 0 | loc (u16): Set to uninitialized |
| `close_loc` | 3 | 0 | 0 | loc (u16): Close variable for closure |

### Closures and Scope
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `get_var` | 3 | 0 | 1 | var_ref (u16): Get from scope |
| `get_var_undef` | 3 | 0 | 1 | var_ref (u16): Get, push undefined if missing |
| `put_var` | 3 | 1 | 0 | var_ref (u16): Set in scope |
| `put_var_init` | 3 | 1 | 0 | var_ref (u16): Init check (lexical) |
| `get_var_ref` | 3 | 0 | 1 | var_ref (u16): Get from var ref |
| `put_var_ref` | 3 | 1 | 0 | var_ref (u16): Set via var ref |
| `set_var_ref` | 3 | 1 | 1 | var_ref (u16): Set via var ref, keep |
| `get_var_ref_check` | 3 | 0 | 1 | var_ref (u16): Get with uninit check |
| `put_var_ref_check` | 3 | 1 | 0 | var_ref (u16): Set with uninit check |
| `put_var_ref_check_init` | 3 | 1 | 0 | var_ref (u16): Init check only |

### Reference Opcodes
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `get_ref_value` | 1 | 2 | 3 | Get reference value (this, prop) |
| `put_ref_value` | 1 | 3 | 0 | Set reference value (this, prop, val) |
| `get_super` | 1 | 1 | 1 | Get super property |
| `get_super_value` | 1 | 3 | 1 | this obj prop -> value |
| `put_super_value` | 1 | 4 | 0 | this obj prop value -> |

---

## Category 6: Property Access

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `get_field` | 5 | 1 | 1 | atom: Get object property |
| `get_field2` | 5 | 1 | 2 | atom: Get property, dup object |
| `put_field` | 5 | 2 | 0 | atom: Set object property |
| `get_private_field` | 1 | 2 | 1 | obj prop -> value |
| `put_private_field` | 1 | 3 | 0 | obj value prop -> |
| `define_private_field` | 1 | 3 | 1 | obj prop value -> obj |
| `get_array_el` | 1 | 2 | 1 | Get array element |
| `get_array_el2` | 1 | 2 | 2 | Get array element, dup array |
| `get_array_el3` | 1 | 2 | 3 | Get array element, dup both |
| `put_array_el` | 1 | 3 | 0 | Set array element |
| `define_field` | 5 | 2 | 1 | atom: Define property |
| `define_array_el` | 1 | 3 | 2 | Define array element |

---

## Category 7: Object/Class Definition

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `set_name` | 5 | 1 | 1 | atom: Set property name on object |
| `set_name_computed` | 1 | 2 | 2 | Computed property name |
| `set_proto` | 1 | 2 | 1 | Set prototype |
| `set_home_object` | 1 | 2 | 2 | Set home object for method |
| `define_method` | 6 | 2 | 1 | atom_u8: Define method |
| `define_method_computed` | 2 | 3 | 1 | u8: Define method with computed name |
| `define_class` | 6 | 2 | 2 | atom_u8: Define class (parent ctor -> ctor proto) |
| `define_class_computed` | 6 | 3 | 3 | atom_u8: Class with computed name |
| `copy_data_properties` | 2 | 3 | 3 | u8: Copy properties |
| `check_brand` | 1 | 2 | 2 | this_obj func -> this_obj func |
| `add_brand` | 1 | 2 | 0 | this_obj home_obj -> |

---

## Category 8: Function Calls

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `call` | 3 | 1+N | 1 | npop (u16): Call function (args not counted in n_pop) |
| `tail_call` | 3 | 1+N | 0 | npop (u16): Tail call (no return value) |
| `call_method` | 3 | 2+N | 1 | npop (u16): Method call |
| `tail_call_method` | 3 | 2+N | 0 | npop (u16): Tail method call |
| `call_constructor` | 3 | 2+N | 1 | npop (u16): Constructor call |
| `array_from` | 3 | N | 1 | npop (u16): Create array from args |
| `apply` | 3 | 3 | 1 | u16 (magic): Function.apply |
| `return` | 1 | 1 | 0 | Return with value |
| `return_undef` | 1 | 0 | 0 | Return undefined |
| `return_async` | 1 | 1 | 0 | Return from async function |

### Short Opcodes
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `call0` | 1 | 1 | 1 | Call with 0 extra args |
| `call1` | 1 | 1 | 1 | Call with 1 extra arg |
| `call2` | 1 | 1 | 1 | Call with 2 extra args |
| `call3` | 1 | 1 | 1 | Call with 3 extra args |

---

## Category 9: Control Flow

### Unconditional Jump
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `goto` | 5 | 0 | 0 | label: Relative jump |

### Short Opcodes
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `goto8` | 2 | 0 | 0 | label8: 8-bit offset |
| `goto16` | 3 | 0 | 0 | label16: 16-bit offset |

### Conditional Jump
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `if_true` | 5 | 1 | 0 | label: Jump if true (must follow if_false) |
| `if_false` | 5 | 1 | 0 | label: Jump if false |

### Short Opcodes
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `if_true8` | 2 | 1 | 0 | label8: Jump if true |
| `if_false8` | 2 | 1 | 0 | label8: Jump if false |

### Exception Handling
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `catch` | 5 | 0 | 1 | label: Push catch offset |
| `gosub` | 5 | 0 | 0 | label: Execute finally block |
| `ret` | 1 | 1 | 0 | Return from finally |
| `nip_catch` | 1 | 2 | 1 | Remove catch context |

---

## Category 10: Iteration

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `for_in_start` | 1 | 1 | 1 | Start for-in loop |
| `for_in_next` | 1 | 1 | 3 | Next for-in iteration |
| `for_of_start` | 1 | 1 | 3 | Start for-of loop |
| `for_of_next` | 2 | 3 | 5 | u8: Next for-of iteration |
| `for_await_of_start` | 1 | 1 | 3 | Start for-await-of |
| `for_await_of_next` | 1 | 3 | 4 | Next for-await-of iteration |
| `iterator_check_object` | 1 | 1 | 1 | Verify iterable |
| `iterator_get_value_done` | 1 | 2 | 3 | Get value or done |
| `iterator_close` | 1 | 3 | 0 | Close iterator |
| `iterator_next` | 1 | 4 | 4 | Call iterator.next |
| `iterator_call` | 2 | 4 | 5 | u8: Call with magic |

---

## Category 11: Async/Generator

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `initial_yield` | 1 | 0 | 0 | Start generator |
| `yield` | 1 | 1 | 2 | Yield value |
| `yield_star` | 1 | 1 | 2 | Yield* delegation |
| `async_yield_star` | 1 | 1 | 2 | Async yield* delegation |
| `await` | 1 | 1 | 1 | Await promise |

---

## Category 12: Binary Arithmetic

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `add` | 1 | 2 | 1 | Addition |
| `sub` | 1 | 2 | 1 | Subtraction |
| `mul` | 1 | 2 | 1 | Multiplication |
| `div` | 1 | 2 | 1 | Division |
| `mod` | 1 | 2 | 1 | Modulo |
| `pow` | 1 | 2 | 1 | Power |

### Local Variants
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `add_loc` | 2 | 1 | 0 | Add to local (8-bit index) |
| `inc_loc` | 2 | 0 | 0 | Increment local (8-bit) |
| `dec_loc` | 2 | 0 | 0 | Decrement local (8-bit) |

---

## Category 13: Unary Arithmetic

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `neg` | 1 | 1 | 1 | Negation |
| `plus` | 1 | 1 | 1 | Unary plus |
| `inc` | 1 | 1 | 2 | Pre-increment (return new value) |
| `dec` | 1 | 1 | 2 | Pre-decrement |
| `post_inc` | 1 | 1 | 2 | Post-increment |
| `post_dec` | 1 | 1 | 2 | Post-decrement |

---

## Category 14: Comparison

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `lt` | 1 | 2 | 1 | Less than |
| `lte` | 1 | 2 | 1 | Less than or equal |
| `gt` | 1 | 2 | 1 | Greater than |
| `gte` | 1 | 2 | 1 | Greater than or equal |
| `instanceof` | 1 | 2 | 1 | instanceof check |
| `in` | 1 | 2 | 1 | Property in object |
| `eq` | 1 | 2 | 1 | Equality |
| `neq` | 1 | 2 | 1 | Inequality |
| `strict_eq` | 1 | 2 | 1 | Strict equality |
| `strict_neq` | 1 | 2 | 1 | Strict inequality |

---

## Category 15: Logical

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `not` | 1 | 1 | 1 | Bitwise NOT |
| `lnot` | 1 | 1 | 1 | Logical NOT |
| `and` | 1 | 2 | 1 | Bitwise AND |
| `xor` | 1 | 2 | 1 | Bitwise XOR |
| `or` | 1 | 2 | 1 | Bitwise OR |
| `shl` | 1 | 2 | 1 | Left shift |
| `shr` | 1 | 2 | 1 | Right shift (unsigned) |
| `sar` | 1 | 2 | 1 | Right shift (signed) |

---

## Category 16: Type Checking

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `typeof` | 1 | 1 | 1 | Type of operator |
| `delete` | 1 | 2 | 1 | Delete property |
| `delete_var` | 5 | 0 | 1 | atom: Delete variable |
| `to_object` | 1 | 1 | 1 | Convert to object |
| `to_propkey` | 1 | 1 | 1 | Convert to property key |
| `is_undefined_or_null` | 1 | 1 | 1 | Check undefined or null |
| `private_in` | 1 | 2 | 1 | Check private field |

### Short Opcodes
| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `is_undefined` | 1 | 1 | 1 | Check undefined |
| `is_null` | 1 | 1 | 1 | Check null |
| `typeof_is_undefined` | 1 | 1 | 1 | typeof === 'undefined' |
| `typeof_is_function` | 1 | 1 | 1 | typeof === 'function' |
| `get_length` | 1 | 1 | 1 | Get length property |

---

## Category 17: Exceptions

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `throw` | 1 | 1 | 0 | Throw exception |
| `throw_error` | 6 | 0 | 0 | atom_u8: Throw error |

---

## Category 18: Class/Constructor

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `check_ctor_return` | 1 | 1 | 2 | Check constructor return |
| `check_ctor` | 1 | 0 | 0 | Verify called with `new` |
| `init_ctor` | 1 | 0 | 1 | Initialize constructor |

---

## Category 19: Eval/Special

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `eval` | 5 | N+1 | 1 | Direct eval (npop_u16) |
| `apply_eval` | 3 | 2 | 1 | u16: Apply to eval |
| `regexp` | 1 | 2 | 1 | Create RegExp |
| `import` | 1 | 2 | 1 | Dynamic import |

---

## Category 20: With/Scope

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `with_get_var` | 10 | 1 | 0 | atom_label_u8 |
| `with_put_var` | 10 | 2 | 1 | atom_label_u8 |
| `with_delete_var` | 10 | 1 | 0 | atom_label_u8 |
| `with_make_ref` | 10 | 1 | 0 | atom_label_u8 |
| `with_get_ref` | 10 | 1 | 0 | atom_label_u8 |

---

## Category 21: Make Reference

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `make_loc_ref` | 7 | 0 | 2 | atom_u16: Make local ref |
| `make_arg_ref` | 7 | 0 | 2 | atom_u16: Make argument ref |
| `make_var_ref_ref` | 7 | 0 | 2 | atom_u16: Make var ref ref |
| `make_var_ref` | 5 | 0 | 2 | atom: Make var ref |

---

## Category 22: Miscellaneous

| Opcode | Size | Pop | Push | Description |
|--------|------|-----|------|-------------|
| `nop` | 1 | 0 | 0 | No operation |
| `invalid` | 1 | 0 | 0 | Invalid (never emitted) |

---

## Temporary Opcodes (Removed During Compilation)

These opcodes are emitted during compilation and removed in later phases:

| Opcode | Size | Phase | Description |
|--------|------|-------|-------------|
| `enter_scope` | 3 | Phase 1 | Push scope, removed in Phase 2 |
| `leave_scope` | 3 | Phase 1 | Pop scope, removed in Phase 2 |
| `label` | 5 | Phase 1 | Mark jump target, removed in Phase 3 |
| `line_num` | 5 | Phase 1 | Line number, removed in Phase 3 |
| `scope_get_var_undef` | 7 | Phase 1 | Scope get, removed in Phase 2 |
| `scope_get_var` | 7 | Phase 1 | Scope get, removed in Phase 2 |
| `scope_put_var` | 7 | Phase 1 | Scope put, removed in Phase 2 |
| `scope_delete_var` | 7 | Phase 1 | Scope delete, removed in Phase 2 |
| `scope_make_ref` | 11 | Phase 1 | Scope ref, removed in Phase 2 |
| `scope_get_ref` | 7 | Phase 1 | Scope ref get, removed in Phase 2 |
| `scope_put_var_init` | 7 | Phase 1 | Scope init, removed in Phase 2 |
| `scope_get_var_checkthis` | 7 | Phase 1 | Scope check this, removed in Phase 2 |
| `scope_get_private_field` | 7 | Phase 1 | Private get, removed in Phase 2 |
| `scope_get_private_field2` | 7 | Phase 1 | Private get, removed in Phase 2 |
| `scope_put_private_field` | 7 | Phase 1 | Private put, removed in Phase 2 |
| `scope_in_private_field` | 7 | Phase 1 | Private in, removed in Phase 2 |
| `get_field_opt_chain` | 5 | Phase 1 | Optional chaining, removed in Phase 2 |
| `get_array_el_opt_chain` | 1 | Phase 1 | Optional chaining, removed in Phase 2 |
| `set_class_name` | 5 | Phase 1 | Class name, removed in Phase 2 |

---

## Opcode Ordering Constraints

Some opcodes must be in specific order:

1. **Variable Access**: `get_var_undef` < `get_var` < `put_var` < `put_var_init`
2. **Control Flow**: `if_false` < `if_true` < `goto`
3. **Closures**: `push_const` < `fclosure`
4. **With**: `with_get_var` < `with_put_var` < `with_delete_var` < `with_make_ref` < `with_get_ref`
5. **Short opcodes**: `fclosure8` must follow `push_const8`

---

## Special Notes

1. **npop format**: Many call opcodes use `npop` format where the stack pop count is determined by an operand, not a fixed value. Arguments are NOT counted in n_pop.

2. **Short opcodes**: When `SHORT_OPCODES` is defined, single-byte opcodes replace common patterns. These are optimizations that reduce bytecode size.

3. **Scope opcodes**: These are compiled away in Phase 2, replaced by `get_var_*`/`put_var_*` opcodes with resolved variable reference indices.

4. **Generator support**: Generator functions use `cur_sp` in JSStackFrame to save/restore stack position when yielding.