# QuickJS Stack Management

## Overview

QuickJS uses a unified stack model where operand stack, local variables, and arguments share the same memory region. This design simplifies memory management and enables efficient frame operations.

## Memory Layout

### Per-Frame Layout
```
+------------------+ <- local_buf (allocated base)
|    arg_buf       |  <- argc entries (arguments)
+------------------+
|    var_buf       |  <- var_count entries (local variables)
+------------------+
|    stack_buf     |  <- stack_size entries (operand stack)
+------------------+      ^
|    var_refs      |  <- var_ref_count pointers
+------------------+      | (separate allocation for refs)
```

### Memory Allocation
```c
alloca_size = sizeof(JSValue) * (arg_count + var_count + stack_size)
           + sizeof(JSVarRef *) * var_ref_count;

local_buf = alloca(alloca_size);
```

---

## Stack Frame Structure

### JSStackFrame Fields
```c
typedef struct JSStackFrame {
    // Function context
    JSValue cur_func;              // Current function object
    
    // Memory regions
    JSValue *arg_buf;              // Arguments (fixed size)
    JSValue *var_buf;              // Local variables (fixed size)
    JSValue *var_refs;             // Closure variable references
    
    // Execution state
    const uint8_t *cur_pc;         // Program counter (for backtrace)
    int arg_count;                  // Actual argument count
    
    // Mode flags
    int js_mode;                    // JS_MODE_STRICT, etc.
    
    // Generator support
    JSValue *cur_sp;               // Saved stack pointer for generators
    
    // Frame chain
    struct JSStackFrame *prev_frame;
} JSStackFrame;
```

---

## Stack Operations

### 1. Push Operations

```c
CASE(OP_push_i32):
    *sp++ = JS_NewInt32(ctx, get_u32(pc));
    pc += 4;
    BREAK;
```

```c
CASE(OP_push_const):
    *sp++ = JS_DupValue(ctx, b->cpool[get_u32(pc)]);
    pc += 4;
    BREAK;
```

```c
CASE(OP_undefined):
    *sp++ = JS_UNDEFINED;
    BREAK;
```

**Note:** `JS_DupValue` increments the reference count.

### 2. Pop Operations

```c
CASE(OP_drop):
    JS_FreeValue(ctx, sp[-1]);  // Free the value
    sp--;                        // Decrement stack pointer
    BREAK;
```

### 3. Dup Operations

```c
CASE(OP_dup):
    sp[0] = JS_DupValue(ctx, sp[-1]);  // Duplicate with refcount++
    sp++;                                   // Increment stack pointer
    BREAK;
```

### 4. Stack Permutation

These opcodes rearrange the stack without memory allocation:

```c
CASE(OP_swap):          // a b -> b a
    tmp = sp[-2];
    sp[-2] = sp[-1];
    sp[-1] = tmp;
    BREAK;

CASE(OP_nip):           // a b -> b
    JS_FreeValue(ctx, sp[-2]);
    sp[-2] = sp[-1];
    sp--;
    BREAK;

CASE(OP_rot3l):         // x a b -> a b x
    tmp = sp[-3];
    sp[-3] = sp[-2];
    sp[-2] = sp[-1];
    sp[-1] = tmp;
    BREAK;
```

---

## Variable Access

### Local Variables

```c
CASE(OP_get_loc):       // Get local[index]
    idx = get_u16(pc);
    pc += 2;
    sp[0] = JS_DupValue(ctx, var_buf[idx]);
    sp++;
    BREAK;

CASE(OP_put_loc):       // local[index] = pop()
    idx = get_u16(pc);
    pc += 2;
    set_value(ctx, &var_buf[idx], sp[-1]);
    sp--;
    BREAK;

CASE(OP_set_loc):       // local[index] = pop(), push pop()
    idx = get_u16(pc);
    pc += 2;
    set_value(ctx, &var_buf[idx], JS_DupValue(ctx, sp[-1]));
    BREAK;
```

**Key difference:**
- `put_loc`: Consumes top of stack (discards)
- `set_loc`: Keeps value on top of stack (duplicates)

### Arguments

Same pattern as locals, using `arg_buf` instead of `var_buf`.

### Closure Variables (var_refs)

```c
CASE(OP_get_var_ref):
    idx = get_u16(pc);
    pc += 2;
    val = *var_refs[idx]->pvalue;    // Dereference pointer
    sp[0] = JS_DupValue(ctx, val);
    sp++;
    BREAK;

CASE(OP_put_var_ref):
    idx = get_u16(pc);
    pc += 2;
    set_value(ctx, var_refs[idx]->pvalue, sp[-1]);
    sp--;
    BREAK;
```

---

## JSVarRef Structure

```c
typedef struct JSVarRef {
    int ref_count;          // Reference count
    JSStackFrame *stack_frame;  // Frame where var lives
    int var_idx;           // Index in frame's var_buf
    JSValue *pvalue;       // Pointer to actual value
} JSVarRef;
```

### Purpose
`JSVarRef` enables closures to reference variables from outer scopes. The pointer remains valid even when the outer function returns.

---

## Stack Overflow Check

```c
alloca_size = sizeof(JSValue) * (arg_count + var_count + stack_size)
           + sizeof(JSVarRef *) * var_ref_count;

if (js_check_stack_overflow(rt, alloca_size))
    return JS_ThrowStackOverflow(caller_ctx);

local_buf = alloca(alloca_size);
```

---

## Generator Stack Management

When a generator yields, its stack is preserved:

```c
CASE(OP_yield):
    sf->cur_pc = pc;           // Save PC
    sf->cur_sp = sp;           // Save stack pointer
    ret_val = JS_NewInt32(ctx, FUNC_RET_YIELD);
    goto done_generator;

done_generator:
    sf->cur_pc = pc;
    sf->cur_sp = sp;
    // Don't clean up stack - preserved for resumption
    rt->current_stack_frame = sf->prev_frame;
    return ret_val;
```

When resuming:
```c
sp = sf->cur_sp;
sf->cur_sp = NULL;  // Mark as running
goto restart;
```

---

## Exception Stack Unwinding

During exception handling, the stack is unwound and values are freed:

```c
exception:
    // ... build backtrace ...
    
    // Unwind stack
    while (sp > stack_buf) {
        JSValue val = *--sp;
        JS_FreeValue(ctx, val);
        
        // Check for catch offset
        if (JS_VALUE_GET_TAG(val) == JS_TAG_CATCH_OFFSET) {
            int pos = JS_VALUE_GET_INT(val);
            if (pos == 0) {
                // Iterator: close it
                JS_IteratorClose(ctx, sp[-1], TRUE);
            } else {
                // Jump to catch block
                *sp++ = rt->current_exception;
                rt->current_exception = JS_UNINITIALIZED;
                pc = b->byte_code_buf + pos;
                goto restart;
            }
        }
    }
```

---

## Frame Setup

```c
// Allocate
alloca_size = sizeof(JSValue) * (arg_count + var_count + stack_size)
            + sizeof(JSVarRef *) * var_ref_count;
local_buf = alloca(alloca_size);

// Initialize arguments
if (arg_allocated_size) {
    arg_buf = local_buf;
    for(i = 0; i < n; i++)
        arg_buf[i] = JS_DupValue(caller_ctx, argv[i]);
    for(; i < b->arg_count; i++)
        arg_buf[i] = JS_UNDEFINED;
}

// Initialize variables
var_buf = local_buf + arg_allocated_size;
for(i = 0; i < b->var_count; i++)
    var_buf[i] = JS_UNDEFINED;

// Setup stack
stack_buf = var_buf + b->var_count;
sp = stack_buf;

// Setup var refs
sf->var_refs = (JSVarRef **)(stack_buf + b->stack_size);
for(i = 0; i < b->var_ref_count; i++)
    sf->var_refs[i] = NULL;
```

---

## Go Implementation

### Stack Type
```go
type VM struct {
    // ...
    sp      int     // Stack pointer index
    // ...
}

// Value operations
func (vm *VM) push(v Value) {
    vm.stack = append(vm.stack, v.Dup())
}

func (vm *VM) pop() Value {
    n := len(vm.stack) - 1
    v := vm.stack[n]
    vm.stack = vm.stack[:n]
    return v
}

func (vm *VM) top() Value {
    return vm.stack[len(vm.stack)-1]
}

func (vm *VM) setTop(v Value) {
    vm.stack[len(vm.stack)-1] = v
}
```

### Frame Type
```go
type Frame struct {
    Prev       *Frame
    Func       Value
    ArgBuf     []Value
    VarBuf     []Value
    VarRefs    []*VarRef
    CurPC      int
    ArgCount   int
    IsStrict   bool
    CurSP      int    // For generators
    Stack      []Value
}
```

### Memory Layout (Go)
```go
type FrameMemory struct {
    Args   []Value  // Arguments
    Vars   []Value  // Local variables  
    Stack  []Value  // Operand stack
    VarRefs []*VarRef // Closure refs
}
```

### Full Frame Setup
```go
func (vm *VM) setupFrame(fn *FunctionBytecode, argc int, argv []Value) *Frame {
    // Allocate combined memory
    totalSize := fn.ArgCount + fn.VarCount + fn.StackSize
    memory := make([]Value, totalSize)
    
    // Initialize args
    args := memory[:fn.ArgCount]
    for i := 0; i < fn.ArgCount; i++ {
        if i < argc {
            args[i] = argv[i].Dup()
        } else {
            args[i] = JSUndefined
        }
    }
    
    // Initialize vars to undefined
    vars := memory[fn.ArgCount : fn.ArgCount+fn.VarCount]
    for i := range vars {
        vars[i] = JSUndefined
    }
    
    // Stack starts after vars
    stack := memory[fn.ArgCount+fn.VarCount:]
    
    return &Frame{
        Func:     vm.currentFunc,
        ArgBuf:   args,
        VarBuf:   vars,
        Stack:    stack,
        VarRefs:  fn.VarRefs,
        ArgCount: argc,
        IsStrict: fn.IsStrict,
    }
}
```

---

## Summary

| Aspect | Description |
|--------|-------------|
| Stack growth | Upward (push = sp++, pop = --sp) |
| Memory model | Single alloca block per frame |
| Components | Arguments + Locals + Operand Stack + VarRefs |
| Closure support | JSVarRef pointers to outer frame variables |
| Generator support | cur_sp saves/restores stack position |
| Overflow check | Before alloca, via js_check_stack_overflow |
| Exception unwind | Free all values, look for catch offsets |