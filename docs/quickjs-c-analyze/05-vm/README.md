# 05-vm Module

## Overview

This module documents the QuickJS virtual machine implementation.

## Files

| File | Description |
|------|-------------|
| `interpreter.md` | Core VM design and execution loop |
| `stack.md` | Stack management and memory layout |
| `go-impl.md` | Go implementation guide |
| `README.md` | This file |

## Architecture

```
┌─────────────────────────────────────────────┐
│                   Runtime                    │
│  ┌───────────────────────────────────────┐  │
│  │                 Context                 │  │
│  │  ┌─────────────────────────────────┐  │  │
│  │  │           Global Object          │  │  │
│  │  └─────────────────────────────────┘  │  │
│  └───────────────────────────────────────┘  │
│  ┌───────────────────────────────────────┐  │
│  │      Current Stack Frame               │  │
│  │  ┌──────┬──────┬───────────┬────────┐ │  │
│  │  │ Args │ Vars │   Stack   │VarRefs │ │  │
│  │  └──────┴──────┴───────────┴────────┘ │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

## Core Components

### JSStackFrame (lines 328-340)
```c
typedef struct JSStackFrame {
    struct JSStackFrame *prev_frame;  // Frame chain
    JSValue cur_func;                   // Current function
    JSValue *arg_buf;                  // Arguments
    JSValue *var_buf;                 // Local variables
    struct JSVarRef **var_refs;       // Closure refs
    const uint8_t *cur_pc;            // PC for backtrace
    int arg_count;
    int js_mode;                      // Strict mode, etc.
    JSValue *cur_sp;                  // For generators
} JSStackFrame;
```

### JSFunctionBytecode
```c
typedef struct {
    uint8_t *byte_code_buf;           // Instructions
    size_t byte_code_len;            // Length
    int var_count;                   // Local vars
    int arg_count;                   // Parameters
    int stack_size;                  // Max stack depth
    int var_ref_count;               // Captured vars
    JSValue *cpool;                  // Constants
    int cpool_count;
} JSFunctionBytecode;
```

## Execution Model

### Main Loop (JS_CallInternal, line 17372)
```c
for(;;) {
    SWITCH(pc) {
        CASE(OP_push): ...
        CASE(OP_add): ...
        ...
    }
}
```

### Control Flow
```
JS_CallInternal()
├── Setup frame
├── Loop
│   ├── Fetch opcode
│   ├── Execute
│   └── Handle exceptions
├── Exception handler
└── Cleanup and return
```

## Memory Layout

```
+------------------+ <- local_buf
| arg_buf (N)      | <- arguments
+------------------+
| var_buf (M)      | <- local variables
+------------------+
| stack (S)        | <- operand stack (grows up)
+------------------+
| var_refs (R)     | <- closure refs
+------------------+
```

## Key Design Decisions

### 1. Single-alloca Frame
Memory is allocated as one block with `alloca()`:
- Reduces GC pressure
- Keeps frame data cache-friendly
- No explicit deallocation needed

### 2. Dispatch Options
- **Switch-based** (default): Portable, slower
- **Computed goto** (DIRECT_DISPATCH): Faster, less portable

### 3. Reference Counting
- All values are reference-counted
- `JS_DupValue()` increments
- `JS_FreeValue()` decrements and frees

### 4. cur_pc for Backtraces
PC is saved before operations that might:
- Throw exceptions
- Call functions
- Yield (generators)

### 5. Exception Handling
Uses label-based control flow:
```c
CASE(OP_throw):
    JS_Throw(ctx, *--sp);
    goto exception;

exception:
    // Unwind stack, find catch
    // Or propagate to caller
```

### 6. Generator Support
- `cur_sp` saves stack position when yielding
- Frame persists until generator completes
- Resume restores pc, sp, and stack values

## Implementation Notes

### Go Translation

**Dispatch:** Use function table or switch
```go
var handlers [256]func(*VM) bool
handlers[OPAdd] = (*VM).opAdd
```

**Stack:** Use slice with index
```go
type VM struct {
    sp    int
    stack []Value
}
func (vm *VM) push(v Value) {
    vm.stack[vm.sp] = v.Dup()
    vm.sp++
}
```

**Frame:** Use linked list
```go
type Frame struct {
    Prev   *Frame
    Func   Value
    ArgBuf []Value
    VarBuf []Value
    Stack  []Value
}
```

### Performance Considerations

1. **Pre-allocate stack** to avoid growth
2. **Inline fast paths** for common operations
3. **Batch reference counting** where possible
4. **Avoid allocations** in hot paths

## Verification

**What was checked:**
- Read `JS_CallInternal` (line 17372-20136)
- Analyzed `JSStackFrame` structure
- Traced exception handling flow
- Reviewed generator support code
- Checked memory allocation patterns

**What was NOT checked:**
- Platform-specific dispatch optimization
- GC integration details
- Thread safety considerations
- Performance benchmarks

## Dependencies

This module depends on:
- **02-value-model**: Understanding JSValue representation
- **03-gc**: Garbage collection integration
- **04-opcodes**: Opcode implementations

## References

- `quickjs.c`: JS_CallInternal (line 17372)
- `quickjs.c`: JSStackFrame (line 328)
- `quickjs.c`: JSFunctionBytecode
- `quickjs-opcode.h`: Opcode definitions