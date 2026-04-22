# QuickJS Opcode Semantics

## Overview

This document provides detailed semantic analysis of QuickJS opcodes, explaining their behavior, stack effects, and implementation notes.

## Stack Notation

- `sp[-1]`: Top of stack (most recently pushed value)
- `sp[-2]`: Second value from top
- `->`: Transforms to
- `[a b c]`: Stack contents, left = bottom, right = top

---

## 1. Push Opcodes

### OP_push_i32
```
Format: i32 (4 bytes signed integer)
Stack: [..] -> [.. val]
```
**Behavior:**
1. Read 32-bit signed integer from bytecode
2. Create JSValue with JS_TAG_INT
3. Push to stack

**Go Implementation:**
```go
func (vm *VM) opPushI32() {
    val := vm.readI32()
    vm.push(js.NewInt32(vm.ctx, val))
}
```

### OP_push_const
```
Format: const (4 bytes constant pool index)
Stack: [..] -> [.. val]
```
**Behavior:**
1. Read 32-bit index from bytecode
2. Look up value in function's constant pool `b->cpool[idx]`
3. Duplicate value (increment refcount)
4. Push to stack

**Note:** Must be immediately followed by `fclosure` for closure creation.

### OP_fclosure
```
Format: const (4 bytes constant pool index)
Stack: [..] -> [.. closure]
Precondition: Previous opcode was push_const
```
**Behavior:**
1. Read 32-bit index from bytecode
2. Create closure from constant pool value
3. Call `js_closure(ctx, func_val, var_refs, sf, FALSE)`
4. Push resulting closure

**Go Implementation:**
```go
func (vm *VM) opFClosure() {
    idx := vm.readU32()
    funcVal := vm.b.Cpool[idx]
    closure := js.Closure(vm.ctx, funcVal, vm.varRefs, vm.sf, false)
    vm.push(closure)
}
```

### OP_undefined, OP_null, OP_push_true, OP_push_false
```
Format: none
Stack: [..] -> [.. constant]
```
**Behavior:** Push pre-defined singleton values.

---

## 2. Stack Manipulation Opcodes

### OP_drop
```
Stack: [.. a] -> [..]
```
**Behavior:**
1. Pop top value
2. Decrement refcount (JS_FreeValue)

### OP_nip (Nip = Remove 2nd value)
```
Stack: [.. a b] -> [.. b]
```
**Behavior:**
1. Free value at sp[-2]
2. Move sp[-1] to sp[-2]
3. Decrement sp

### OP_nip1
```
Stack: [.. a b c] -> [.. b c]
```
**Behavior:** Removes value at sp[-3], shifts remaining values.

### OP_dup
```
Stack: [.. a] -> [.. a a]
```
**Behavior:**
1. Duplicate sp[-1]
2. Increment refcount
3. Push duplicate

### OP_dup1
```
Stack: [.. a b] -> [.. a a b]
```
**Behavior:** Duplicate sp[-2] and push.

### OP_dup2
```
Stack: [.. a b] -> [.. a b a b]
```
**Behavior:** Duplicate both values.

### OP_insert2 (dup_x1)
```
Stack: [.. obj a] -> [.. a obj a]
```
**Implementation:** See C code pattern:
```c
sp[0] = sp[-1];
sp[-1] = sp[-2];
sp[-2] = JS_DupValue(ctx, sp[0]);
sp++;
```

### OP_swap
```
Stack: [.. a b] -> [.. b a]
```
**Behavior:** Swap top two values.

### OP_rot3l (Rotate Left 3)
```
Stack: [.. x a b] -> [.. a b x]
```
**Example:** For expression `(a, b, x)` result.
Same as C's `sp[-3] = sp[-2]; sp[-2] = sp[-1]; sp[-1] = tmp`

### OP_rot3r (Rotate Right 3)
```
Stack: [.. a b x] -> [.. x a b]
```

---

## 3. Variable Access

### Local Variables (get_loc, put_loc, set_loc)
```
get_loc:  Stack: [..] -> [.. val]       Index from bytecode
put_loc:  Stack: [.. val] -> [..]        Index from bytecode
set_loc:  Stack: [.. val] -> [.. val]    Index from bytecode
```
**Key Difference:**
- `put_loc`: Set value AND pop (discard top)
- `set_loc`: Set value AND keep (duplicate on top)

**Implementation:**
```go
func (vm *VM) opGetLoc(idx int) {
    vm.push(js.DupValue(vm.ctx, vm.varBuf[idx]))
}

func (vm *VM) opPutLoc(idx int) {
    js.SetValue(vm.ctx, &vm.varBuf[idx], vm.sp[-1])
    vm.sp--
}

func (vm *VM) opSetLoc(idx int) {
    js.SetValue(vm.ctx, &vm.varBuf[idx], js.DupValue(vm.ctx, vm.sp[-1]))
}
```

### Variable References (Closure Variables)

When a function captures a variable from an outer scope, it's stored as a `JSVarRef`:

```
get_var_ref:  Stack: [..] -> [.. val]    Index from bytecode
put_var_ref:  Stack: [.. val] -> [..]    Index from bytecode
```
**Behavior:**
1. Look up `var_refs[idx]` (array of `JSVarRef*`)
2. Dereference `var_refs[idx]->pvalue` to get actual JSValue
3. Handle uninitialized checking if needed

**Uninitialized Check Variants:**
- `get_var_ref_check`: Error if value is JS_UNINITIALIZED
- `put_var_ref_check_init`: Only allow if currently uninitialized (TDZ)

---

## 4. Property Access

### OP_get_field
```
Format: atom (4 bytes)
Stack: [.. obj] -> [.. val]
```
**Behavior:**
1. Pop object
2. Look up property by atom
3. Push result (or exception)

**Go Implementation:**
```go
func (vm *VM) opGetField() {
    atom := vm.readAtom()
    obj := vm.pop()
    val := js.GetProperty(vm.ctx, obj, atom)
    if js.IsException(val) {
        vm.throw()
    }
    vm.push(val)
}
```

### OP_put_field
```
Format: atom (4 bytes)
Stack: [.. obj val] -> [..]
```
**Behavior:** Set property, pop both object and value.

### OP_get_array_el
```
Stack: [.. obj prop] -> [.. val]
```
**Behavior:** Get array element, handles sparse arrays.

---

## 5. Function Calls

### OP_call
```
Format: npop (2 bytes arg count)
Stack: [.. func arg0 arg1 .. argN-1] -> [.. result]
```
**Behavior:**
1. Read argument count from bytecode
2. Save current pc to `sf->cur_pc`
3. Call `JS_CallInternal(ctx, func, this=undefined, args, count, 0)`
4. Clean up stack (free args and func)
5. Push result

**Special:** Arguments are NOT counted in n_pop - they are implicit based on `npop` operand.

### OP_tail_call
```
Format: npop (2 bytes arg count)
Stack: [.. func arg0 arg1 .. argN-1] -> [] (no push)
```
**Behavior:** Same as call but doesn't push result, enabling tail-call optimization.
No new stack frame is created.

### OP_call_method
```
Format: npop (2 bytes arg count)
Stack: [.. obj method arg0 .. argN-1] -> [.. result]
```
**Behavior:**
1. sp[-1] = object, sp[-2] = method
2. Call with `this = sp[-2]`, function = sp[-1]
3. Different from `call` where this is undefined

### OP_call_constructor
```
Format: npop (2 bytes arg count)
Stack: [.. func newTarget arg0 .. argN-1] -> [.. result]
```
**Behavior:**
1. sp[-2] = constructor, sp[-1] = new.target
2. Call constructor internal with new.target

---

## 6. Arithmetic Opcodes

### OP_add (Fast Path)
```
Stack: [.. op1 op2] -> [.. result]
```
**Behavior - Fast Path for Integers:**
```c
if (JS_VALUE_IS_BOTH_INT(op1, op2)) {
    int64_t r = JS_VALUE_GET_INT(op1) + JS_VALUE_GET_INT(op2);
    if ((int)r != r) {
        // Overflow - use float
        sp[-2] = __JS_NewFloat64(ctx, (double)r);
    } else {
        sp[-2] = JS_NewInt32(ctx, r);
    }
}
```
**String Concatenation:**
If both operands are strings, use `JS_ConcatString` for string concat.

### OP_add_loc
```
Format: loc8 (1 byte index)
Stack: [.. val] -> [.. result]
```
**Behavior:** Add to local variable at index. Optimized for `x += 1`.

### OP_sub, OP_mul, OP_div, OP_mod
Same fast-path pattern with integer operations.

**Division Note:**
```c
CASE(OP_div):
    if (JS_VALUE_IS_BOTH_INT(op1, op2)) {
        v1 = JS_VALUE_GET_INT(op1);
        v2 = JS_VALUE_GET_INT(op2);
        sp[-2] = JS_NewFloat64(ctx, (double)v1 / (double)v2); // Always float!
        sp--;
    } else {
        goto binary_arith_slow;
    }
```

### OP_pow
Always uses slow path `js_binary_arith_slow`.

---

## 7. Comparison Opcodes

All comparison opcodes use the slow path `js_binary_arith_slow` unless optimized.

---

## 8. Control Flow

### OP_goto
```
Format: label (4 bytes relative offset)
Stack: [..] -> [..]
```
**Behavior:**
1. Read 32-bit signed offset
2. `pc += offset` (relative from end of instruction)
3. Check for interrupts

### OP_if_false
```
Format: label (4 bytes relative offset)
Stack: [.. val] -> [..]
```
**Behavior:**
1. Pop value
2. Convert to boolean (fast path for int/bool/null/undefined)
3. If false, jump (pc += offset)
4. Always consume the value

### OP_catch
```
Format: label (4 bytes relative offset)
Stack: [..] -> [.. catch_offset]
```
**Behavior:** Push special catch offset value. This is later used during exception handling.

### OP_gosub / OP_ret
For finally block execution:
1. `gosub` pushes return address (pc + diff)
2. `ret` pops address and jumps

---

## 9. Exception Handling

### OP_throw
```
Stack: [.. exc] -> []
```
**Behavior:**
1. Pop exception value
2. Call `JS_Throw(ctx, exception)`
3. `goto exception`

### Exception Handler Logic
When exception occurs:
1. Save current `pc` to `sf->cur_pc`
2. Walk stack looking for catch offsets
3. If found, push exception and jump to handler
4. If not found, propagate to caller

---

## 10. Generator/Await Opcodes

### OP_yield
```
Stack: [.. val] -> [.. sent]
```
**Behavior:**
1. Save current state (pc, sp to sf)
2. Return special value FUNC_RET_YIELD
3. On resume, sp[-1] becomes the `.value` from IteratorResult

### OP_await
```
Stack: [.. promise] -> [.. result]
```
**Behavior:** Similar to yield but handles promise resolution.

---

## 11. Iterator Opcodes

### OP_for_of_start
```
Stack: [.. iterable] -> [.. iter next done]
```
**Behavior:**
1. Get iterator from iterable using Symbol.iterator
2. Call iterator.next()
3. Push iterator, next method, done flag

### OP_for_of_next
```
Format: u8 (magic)
Stack: [.. iter next done] -> [.. iter next done obj value]
```
**Behavior:**
1. Call `next()` method
2. Check if done
3. If done, jump; else push value

---

## 12. Class Opcodes

### OP_define_class
```
Format: atom_u8
Stack: [.. parent ctor] -> [.. ctor proto]
```
**Behavior:**
1. Create class prototype object
2. Set constructor
3. Set prototype chain
4. Push class pair

---

## 13. Special Opcodes

### OP_push_this
```
Stack: [..] -> [.. this]
```
**Constraint:** Only used at function start.

**Behavior:**
```c
if (!(b->js_mode & JS_MODE_STRICT)) {
    if (tag == JS_TAG_OBJECT) {
        // Normal case: keep 'this'
    } else if (tag == JS_TAG_NULL || JS_TAG_UNDEFINED) {
        // Boxing: use global object
        val = JS_DupValue(ctx, ctx->global_obj);
    } else {
        // Boxing: convert to object
        val = JS_ToObject(ctx, this_obj);
    }
} else {
    // Strict mode: keep as-is
}
```

### OP_check_ctor
```
Stack: [..] -> []
```
**Behavior:** If `new_target` is undefined, throw "class constructors must be invoked with 'new'"

---

## 14. Slow Path Operations

Many opcodes have a "slow path" that handles non-integer cases:

```c
CASE(OP_add):
    if (fast_path_holds) {
        // fast int or string concat
    } else {
        sf->cur_pc = pc;
        if (js_add_slow(ctx, sp))
            goto exception;
        sp--;
    }
```

The `sf->cur_pc` update is critical - it ensures that if the slow path throws, the backtrace shows the correct location.

---

## 15. Opcode Encoding Summary

### Variable-length Opcodes
| Opcode Prefix | Total Size | Operand Size |
|--------------|------------|--------------|
| `OP_push_i32` | 5 | 4 |
| `OP_push_const` | 5 | 4 |
| `OP_push_atom_value` | 5 | 4 |
| `OP_get_loc` | 3 | 2 |
| `OP_call` | 3 | 2 |
| `OP_if_false` | 5 | 4 |
| `OP_goto` | 5 | 4 |

### Single-byte Opcodes
| Opcode | Description |
|--------|-------------|
| `OP_undefined` | Push undefined |
| `OP_null` | Push null |
| `OP_drop` | Pop and discard |
| `OP_add` | Add |
| etc. | |

---

## 16. Short Opcodes (Optimization)

When `SHORT_OPCODES` is defined, common patterns are replaced:

```c
CASE(OP_push_0):
    *sp++ = JS_NewInt32(ctx, opcode - OP_push_0); // 0-7 encoded in opcode
    BREAK;

CASE(OP_get_loc0):
    *sp++ = JS_DupValue(ctx, var_buf[0]);
    BREAK;
```

This reduces bytecode size by encoding small indices in the opcode itself.

---

## 17. Go Implementation Notes

### Dispatch Table Approach
QuickJS supports two dispatch modes:

**1. Switch-based (default):**
```c
SWITCH(pc) {
    CASE(OP_add): ...
    CASE(OP_sub): ...
}
```

**2. Computed goto (DIRECT_DISPATCH):**
```c
static const void *dispatch_table[256] = {
#define DEF(id, size, n_pop, n_push, f) &&case_OP_##id,
#include "quickjs-opcode.h"
};
goto *dispatch_table[opcode];
```

**Go Equivalent:**
- Use `goto` with label map (requires function)
- Or use `append` + switch in loop
- Or use opcode handler functions with interface/switch

### Reference Counting
All push operations duplicate values:
```go
func (vm *VM) push(v JSValue) {
    vm.sp[0] = js.DupValue(vm.ctx, v) // Increment refcount
    vm.sp++
}
```

All pop operations free:
```go
func (vm *VM) pop() JSValue {
    vm.sp--
    return vm.sp[0] // Caller must call FreeValue
}
```

---

## Key Implementation Patterns

### 1. Setting values (put_*)
```go
// set_value handles both regular values and var refs
js.SetValue(ctx, &dest, value)
// Equivalent to: dest = value; refcount handled
```

### 2. DupValue before push
```go
vm.push(js.DupValue(ctx, someValue))
```

### 3. FreeValue after pop
```go
val := vm.pop()
js.FreeValue(ctx, val)
```

### 4. Exception checking
```go
if js.IsException(val) {
    goto exception // in Go: return JS_EXCEPTION
}
```

### 5. Updating cur_pc before slow calls
```go
vm.sf.CurPC = vm.pc
if vm.jsAddSlow(sp) {
    return vm.throw()
}
vm.sp--
```