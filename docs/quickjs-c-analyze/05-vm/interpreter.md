# QuickJS Virtual Machine Architecture

## Source
`quickjs.c` lines 17372-20136 - `JS_CallInternal` function

## Overview

QuickJS uses a stack-based bytecode interpreter. The VM executes bytecode functions by:
1. Setting up a stack frame
2. Running an instruction dispatch loop
3. Handling exceptions through the frame chain

## VM Components

### 1. Runtime (JSRuntime)
```
typedef struct JSRuntime {
    ...
    JSStackFrame *current_stack_frame;  // Current execution frame
    JSValue current_exception;           // Pending exception
    ...
} JSRuntime;
```

### 2. Context (JSContext)
```
typedef struct JSContext {
    JSRuntime *rt;
    JSObject *global_obj;      // Global object
    JSObject *global_proto;    // Object.prototype
    JSAtom *atom_array;       // Atom table
    ...
} JSContext;
```

### 3. Stack Frame (JSStackFrame)
```c
typedef struct JSStackFrame {
    struct JSStackFrame *prev_frame;  // NULL if first stack frame
    JSValue cur_func;                  // Current function
    JSValue *arg_buf;                  // Arguments
    JSValue *var_buf;                 // Local variables
    struct JSVarRef **var_refs;       // Closure variable references
    const uint8_t *cur_pc;            // PC after call (for backtrace)
    int arg_count;
    int js_mode;                      // Strict mode flag
    JSValue *cur_sp;                   // For generators: saved stack pointer
} JSStackFrame;
```

### 4. Function Bytecode (JSFunctionBytecode)
```c
typedef struct JSFunctionBytecode {
    ...
    uint8_t *byte_code_buf;           // Bytecode instructions
    size_t byte_code_len;             // Bytecode length
    int var_count;                    // Number of local variables
    int arg_count;                     // Number of arguments
    int stack_size;                    // Maximum stack depth
    int var_ref_count;                 // Closure variable count
    int cpool_count;                    // Constant pool size
    JSValue *cpool;                    // Constant pool values
    ...
} JSFunctionBytecode;
```

---

## Memory Layout

When a function is called, memory is allocated as a single block:

```
+-------------------+ <- local_buf
| arg_buf (N args) |
+-------------------+
| var_buf (M vars)  |
+-------------------+
| stack_buf (S slots)| <- sp grows upward
+-------------------+
| var_refs (R refs)  | <- pointers to closed-over vars
+-------------------+
```

**Allocation:**
```c
alloca_size = sizeof(JSValue) * (arg_allocated_size + var_count + stack_size)
            + sizeof(JSVarRef*) * var_ref_count;
```

---

## VM Execution Flow

### Entry: JS_CallInternal

```c
static JSValue JS_CallInternal(JSContext *caller_ctx, JSValueConst func_obj,
                               JSValueConst this_obj, JSValueConst new_target,
                               int argc, JSValue *argv, int flags)
{
    JSRuntime *rt = caller_ctx->rt;
    JSStackFrame sf_s, *sf = &sf_s;
    const uint8_t *pc;
    int opcode;
    JSValue *sp, ret_val;
    
    // 1. Handle generator functions
    if (flags & JS_CALL_FLAG_GENERATOR) {
        // Resume existing generator
        sf = &s->frame;
        goto restart;
    }
    
    // 2. Handle non-bytecode functions (C functions, built-ins)
    if (p->class_id != JS_CLASS_BYTECODE_FUNCTION) {
        return call_func(...);  // Call class-specific call function
    }
    
    // 3. Get bytecode function
    b = p->u.func.function_bytecode;
    
    // 4. Allocate frame
    local_buf = alloca(alloca_size);
    
    // 5. Initialize frame
    sf->arg_buf = arg_buf;
    sf->var_buf = var_buf;
    sf->cur_func = func_obj;
    sf->prev_frame = rt->current_stack_frame;
    rt->current_stack_frame = sf;
    
    // 6. Initialize PC and stack
    pc = b->byte_code_buf;
    sp = stack_buf;
    
restart:
    // Main execution loop
    for(;;) {
        SWITCH(pc) {
            CASE(OP_push): ...
            CASE(OP_add): ...
            ...
        }
    }
    
exception:
    // Exception handling
    
done:
    // Cleanup and return
    rt->current_stack_frame = sf->prev_frame;
    return ret_val;
}
```

---

## Instruction Dispatch

### Switch-based Dispatch (Default)

```c
#define SWITCH(pc)      switch (opcode = *pc++)
#define CASE(op)        case op
#define BREAK           break

for(;;) {
    SWITCH(pc) {
        CASE(OP_push_i32):
            *sp++ = JS_NewInt32(ctx, get_u32(pc));
            pc += 4;
            BREAK;
        CASE(OP_add):
            // ...
            BREAK;
    }
}
```

### Computed Goto Dispatch (Optional)

```c
#ifdef DIRECT_DISPATCH
static const void * const dispatch_table[256] = {
#define DEF(id, size, n_pop, n_push, f) &&case_OP_ ## id,
#include "quickjs-opcode.h"
    [ OP_COUNT ... 255 ] = &&case_default
};

#define SWITCH(pc)      goto *dispatch_table[opcode = *pc++]
#define CASE(op)        case_ ## op
#define BREAK           goto *dispatch_table[opcode = *pc++]

// Usage
SWITCH(pc);
case_OP_add:
    // ...
    BREAK;
#endif
```

---

## Exception Handling

### Exception Flow

1. **Exception is thrown** - `goto exception`
2. **Save PC** - `sf->cur_pc = pc` (for backtrace)
3. **Build backtrace** if needed
4. **Unwind stack** - clean up values on stack
5. **Look for handler** - scan for catch offsets
6. **Jump to handler** or propagate to caller

```c
exception:
    if (is_backtrace_needed(ctx, rt->current_exception)) {
        build_backtrace(ctx, rt->current_exception, ...);
    }
    
    // Find catch block
    while (sp > stack_buf) {
        JSValue val = *--sp;
        JS_FreeValue(ctx, val);
        if (JS_VALUE_GET_TAG(val) == JS_TAG_CATCH_OFFSET) {
            int pos = JS_VALUE_GET_INT(val);
            if (pos == 0) {
                // enumerator: close it
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
    
    // No handler found - return exception
    ret_val = JS_EXCEPTION;
    goto done;
```

---

## Generator Support

Generators use `cur_sp` to save/restore stack position:

```c
// When yielding:
CASE(OP_yield):
    sf->cur_pc = pc;
    sf->cur_sp = sp;
    ret_val = JS_NewInt32(ctx, FUNC_RET_YIELD);
    goto done_generator;

// When resuming:
sf->cur_sp = NULL;  // Running again
sf->cur_pc = pc;
```

---

## Interrupt Handling

```c
CASE(OP_goto):
    pc += (int32_t)get_u32(pc);
    if (unlikely(js_poll_interrupts(ctx)))
        goto exception;
    BREAK;
```

---

## Go Implementation Design

### Core Types

```go
package quickjs

// Runtime represents the JS execution context
type Runtime struct {
    ctx         *Context
    stackFrame  *StackFrame
    atoms       []Atom
    gc          *GC
    // ...
}

// Context represents a JavaScript context (realm)
type Context struct {
    rt          *Runtime
    globalObj   *Object
    globalProto *Object
    // ...
}

// StackFrame represents a function call frame
type StackFrame struct {
    PrevFrame   *StackFrame
    CurFunc     Value
    ArgBuf      []Value
    VarBuf      []Value
    VarRefs     []*VarRef
    CurPC       int  // PC for backtrace
    ArgCount    int
    IsStrict    bool
    CurSP       int  // For generators
}

// FunctionBytecode represents compiled function
type FunctionBytecode struct {
    Code        []byte
    VarCount    int
    ArgCount    int
    StackSize   int
    VarRefCount int
    ConstPool   []Value
    // ...
}
```

### VM Structure

```go
type VM struct {
    ctx     *Context
    sf      *StackFrame     // Current stack frame
    b       *FunctionBytecode
    pc      int             // Program counter (index into b.Code)
    sp      int             // Stack pointer (index into stack)
    stack   []Value         // Current frame's stack
    varRefs []*VarRef
    retVal  Value
}

// Run executes the bytecode
func (vm *VM) Run() Value {
    for {
        opcode := vm.fetch()
        switch opcode {
        case OPUndefined:
            vm.opUndefined()
        case OPPushI32:
            vm.opPushI32()
        case OPAdd:
            vm.opAdd()
        // ... etc
        }
    }
}
```

### Dispatch Options

**Option 1: Simple Switch**
```go
func (vm *VM) Run() Value {
    for {
        switch vm.fetch() {
        case OPUndefined:
            vm.push(JSUndefined)
        case OPPushI32:
            vm.push(vm.readI32())
        // ...
        }
    }
}
```

**Option 2: Function Table**
```go
type OpHandler func(*VM)

var handlers [256]OpHandler

func init() {
    handlers[OPUndefined] = (*VM).opUndefined
    handlers[OPAdd] = (*VM).opAdd
    // ...
}

func (vm *VM) Run() Value {
    for {
        handlers[vm.fetch()](vm)
    }
}
```

### Stack Operations

```go
func (vm *VM) push(v Value) {
    vm.stack = append(vm.stack, v.Dup())
}

func (vm *VM) pop() Value {
    n := len(vm.stack) - 1
    v := vm.stack[n]
    vm.stack = vm.stack[:n]
    return v
}

func (vm *VM) dup() {
    v := vm.pop()
    vm.push(v)
    vm.push(v.Dup())
}

func (vm *VM) drop() {
    vm.stack[len(vm.stack)-1].Free()
    vm.stack = vm.stack[:len(vm.stack)-1]
}
```

### Call Implementation

```go
func (vm *VM) call(argc int) Value {
    // Get function and arguments from stack
    args := vm.stack[len(vm.stack)-argc:]
    fn := vm.stack[len(vm.stack)-argc-1]
    
    // Save PC
    vm.sf.CurPC = vm.pc
    
    // Create new frame
    newFrame := &StackFrame{
        PrevFrame: vm.sf,
        CurFunc:   fn,
        ArgBuf:    args,
        VarBuf:    make([]Value, fn.Bytecode.VarCount),
        VarRefs:   fn.VarRefs,
        ArgCount:  argc,
        IsStrict:  fn.Bytecode.IsStrict,
    }
    
    // Initialize vars to undefined
    for i := range newFrame.VarBuf {
        newFrame.VarBuf[i] = JSUndefined
    }
    
    // Switch frame
    vm.sf = newFrame
    vm.stack = append(vm.stack[:0], vm.stack[:fn.Bytecode.StackSize]...)
    vm.pc = 0
    vm.b = fn.Bytecode
    
    // Execute
    result := vm.Run()
    
    // Restore frame
    vm.sf = newFrame.PrevFrame
    
    return result
}
```

---

## Key Implementation Notes

### 1. PC Points to Next Instruction
After fetching opcode, PC is at operand. After reading operand, PC is at next opcode.

### 2. Stack Grows Upward
- Push: `*sp++ = value`
- Pop: `value = *--sp`

### 3. Refcount Management
- Every `push` increments refcount via `JS_DupValue`
- Every `pop` does not automatically free (caller's responsibility)
- `set_value` handles refcount correctly

### 4. cur_pc for Backtrace
`sf->cur_pc` is saved before any operation that might:
- Call a function
- Trigger an exception
- Yield (generators)

### 5. Local Allocation with alloca()
Memory is allocated as a single block with `alloca()`. This:
- Avoids GC pressure for short-lived frames
- Keeps all frame data together in memory
- Requires no explicit deallocation

---

## Optimization Opportunities

### 1. Short Opcodes
Enable with `#define SHORT_OPCODES` for smaller bytecode.

### 2. Computed Goto
Enable with `#define DIRECT_DISPATCH` for faster dispatch.

### 3. Inline Integer Operations
Many opcodes have fast paths for integer-only cases.

### 4. Stack Check
`js_check_stack_overflow()` prevents stack overflow.

---

## References

- `quickjs.c`: JS_CallInternal (line 17372)
- `quickjs.c`: JSStackFrame (line 328)
- `quickjs.c`: JSFunctionBytecode
- `quickjs-opcode.h`: Opcode definitions