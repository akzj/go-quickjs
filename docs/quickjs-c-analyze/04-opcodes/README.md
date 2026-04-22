# 04-opcodes Module

## Overview

This module documents the QuickJS bytecode instruction set (`quickjs-opcode.h`).

## Files

| File | Description |
|------|-------------|
| `opcode-table.md` | Complete opcode table with stack effects |
| `opcode-semantics.md` | Detailed semantics for each opcode category |
| `README.md` | This file |

## Opcode Categories

1. **Push Opcodes**: Push constants, literals, atoms
2. **Stack Manipulation**: dup, swap, rotate, drop, nip
3. **Variable Access**: get/put locals, args, closure refs
4. **Property Access**: get/put field, array element, private field
5. **Function Calls**: call, tail_call, call_method, constructor
6. **Control Flow**: goto, if_true/false, try/catch
7. **Arithmetic**: add, sub, mul, div, mod, pow
8. **Comparison**: lt, gt, eq, strict_eq, etc.
9. **Logical**: and, or, not, lnot, bitwise ops
10. **Async/Generator**: yield, await, async_yield_star
11. **Iteration**: for_in, for_of, iterator ops
12. **Class**: define_class, check_ctor, private fields

## Key Design Decisions

### Stack-based Architecture
- All operations work on a value stack
- Most opcodes have fixed pop/push counts
- Call opcodes use `npop` format for variable arg count

### Short Opcodes
- When `SHORT_OPCODES` is defined, single-byte variants exist
- Encode small integers 0-7 in opcode value
- Encode local/arg indices 0-3 in opcode value
- Reduces bytecode size for common patterns

### Operand Formats
| Format | Bytes | Usage |
|--------|-------|-------|
| `none` | 0 | No operand |
| `i8/i16/i32` | 1/2/4 | Signed integers |
| `u8/u16` | 1/2 | Unsigned integers |
| `loc/arg` | 2 | Local/arg index |
| `atom` | 4 | Atom index |
| `const` | 4 | Constant pool index |
| `label` | 4 | Jump offset |
| `npop` | 2 | Argument count |

### Temporary Opcodes
Some opcodes exist only during compilation:
- `enter_scope` / `leave_scope`: Phase 1 → 2
- `label`: Phase 1 → 3 (debug info)
- `scope_*`: Scope resolution, removed in phase 2

### Opcode Ordering
Certain opcodes must be adjacent for lookup optimization:
- `get_var_undef` < `get_var` < `put_var` < `put_var_init`
- `if_false` < `if_true` < `goto`
- `push_const` < `fclosure`

## Implementation Notes

### Reference Counting
Every push duplicates the value:
```c
sp[0] = JS_DupValue(ctx, sp[-1]);  // Increment refcount
sp++;
```

Every pop must consider freeing:
```c
JS_FreeValue(ctx, sp[-1]);  // Decrement refcount
sp--;
```

### cur_pc for Backtrace
Before any operation that might:
- Throw an exception
- Call another function
- Yield (generators)

Update `sf->cur_pc = pc` to enable accurate backtraces.

### Exception Handling
- Exceptions use `goto exception` pattern
- Stack is unwound looking for catch offsets
- Catch offsets are special values on stack

## Go Implementation

See `05-vm/go-impl.md` for detailed Go implementation guidance.

Key types:
```go
type Opcode int

type VM struct {
    ctx   *Context
    frame *Frame
    pc    int
    sp    int
    stack []Value
}
```

## Verification

**What was checked:**
- Read `quickjs-opcode.h` completely
- Verified opcode categories
- Analyzed operand formats
- Reviewed stack effects for all opcodes

**What was NOT checked:**
- Generated opcode values (computed by macros)
- Edge cases in slow paths
- Platform-specific behavior

## References

- `quickjs-opcode.h` - Opcode definitions
- `quickjs.c` lines ~17430-20136 - Opcode implementations