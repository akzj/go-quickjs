# QuickJS VM Go Implementation Guide

## Overview

This document provides implementation guidance for porting QuickJS's bytecode VM to Go.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│                      Runtime                           │
│  ┌─────────────────────────────────────────────────┐  │
│  │                  Context                         │  │
│  │  ┌───────────────────────────────────────────┐  │  │
│  │  │               Global Object                │  │  │
│  │  └───────────────────────────────────────────┘  │  │
│  │  ┌───────────────────────────────────────────┐  │  │
│  │  │           Atom Table                      │  │  │
│  │  └───────────────────────────────────────────┘  │  │
│  └─────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────┐  │
│  │           Current Stack Frame                   │  │
│  │  ┌─────────┬─────────┬────────────────┬────────┐ │  │
│  │  │  Args   │  Vars   │    Stack       │VarRefs │ │  │
│  │  └─────────┴─────────┴────────────────┴────────┘ │  │
│  └─────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

---

## Core Types

### Opcode Enum

```go
package quickjs

// Opcode represents a QuickJS bytecode opcode
type Opcode int

const (
    OPInvalid           Opcode = iota
    OPPushI32                    // push_i32
    OPPushConst                  // push_const
    OPFClosure                   // fclosure
    OPPushAtomValue               // push_atom_value
    OPUndefined                   // undefined
    OPNull                        // null
    OPPushThis                    // push_this
    OPPushTrue                    // push_true
    OPPushFalse                   // push_false
    OPDrop                        // drop
    OPDup                         // dup
    // ... etc
    OPAdd                         // add
    OPSub                         // sub
    OPMul                         // mul
    OPDiv                         // div
    OPMod                         // mod
    OPPow                         // pow
    OPEqual                       // eq
    OPLessThan                    // lt
    // ... etc
    OPGoto                        // goto
    OPIfFalse                     // if_false
    OPIfTrue                      // if_true
    OPCall                        // call
    OPTailCall                    // tail_call
    OPReturn                      // return
    // ... etc
    OPOpCount                     // total count
)
```

### Value Type

```go
package quickjs

// Value represents a JavaScript value
type Value struct {
    tag  uint32  // Tag indicating type
    ptr  uint64  // Pointer or inline data
}

// Tag constants
const (
    TagInt       = 0  // 32-bit integer
    TagBool      = 1
    TagNull      = 2
    TagUndefined = 3
    TagObject    = 4
    TagString    = 5
    TagFloat64   = 7
    // ... etc
)

// Tag bits for small integers (0-31)
const smallIntTag = 0

func NewInt32(v int32) Value {
    return Value{tag: TagInt, ptr: uint64(uint32(v))}
}

func (v Value) Int32() int32 {
    return int32(uint32(v.ptr))
}

func (v Value) IsInt() bool {
    return v.tag == TagInt
}

func (v Value) IsFloat() bool {
    return v.tag == TagFloat64
}

func (v Value) IsObject() bool {
    return v.tag == TagObject
}

func (v Value) IsString() bool {
    return v.tag == TagString
}

func (v Value) IsException() bool {
    return v.tag == TagException
}

// Dup creates a new reference to this value
func (v Value) Dup() Value {
    if v.isManaged() {
        atomic.AddInt32(&v.getHeader().refCount, 1)
    }
    return v
}

// Free releases a reference
func (v Value) Free(ctx *Context) {
    if v.isManaged() {
        hdr := v.getHeader()
        if atomic.AddInt32(&hdr.refCount, -1) == 0 {
            freeValue(ctx, v)
        }
    }
}
```

### Object Type

```go
// Object represents a JavaScript object
type Object struct {
    GCHeader
    Class    Class   // Object class
    Proto    *Object // Prototype
    Properties map[string]Value // Own properties
    // For functions:
    Bytecode *FunctionBytecode
    VarRefs  []*VarRef
    // For arrays:
    Values   []Value
}

type Class struct {
    ID      int
    Call    func(*Context, Value, Value, int, []Value) Value // [[Call]]
    Construct func(*Context, Value, Value, int, []Value) Value // [[Construct]]
    // ... other methods
}
```

### Function Bytecode

```go
// FunctionBytecode represents compiled JavaScript function
type FunctionBytecode struct {
    Code        []byte           // Bytecode instructions
    CodeLen     int              // Length of bytecode
    
    // Function metadata
    ArgCount    int              // Number of parameters
    VarCount    int              // Number of local variables
    StackSize   int              // Maximum stack depth
    VarRefCount int              // Number of captured variables
    
    // Constant pool
    ConstPool   []Value          // Constants used by bytecode
    
    // Scoping
    IsStrict    bool
    IsAsync     bool
    IsGenerator bool
    
    // For methods
    HomeObject  *Object
    
    // Realm (context)
    Realm       *Context
}
```

### Stack Frame

```go
// Frame represents a function call frame
type Frame struct {
    Prev       *Frame    // Previous frame (caller)
    Func       Value      // Current function value
    
    // Memory regions (point into allocated memory)
    ArgBuf     []Value    // Arguments
    VarBuf     []Value    // Local variables
    VarRefs    []*VarRef  // Captured variables
    
    // Execution state
    PC         int        // Program counter
    ArgCount   int        // Actual argument count
    
    // Mode
    IsStrict   bool
    
    // For generators
    SavedSP    int        // Saved stack pointer
    
    // Runtime info
    Realm      *Context
}
```

---

## VM Implementation

### VM Structure

```go
// VM represents the QuickJS virtual machine
type VM struct {
    ctx  *Context
    rt   *Runtime
    
    // Current frame
    frame *Frame
    
    // Bytecode being executed
    bc *FunctionBytecode
    
    // Current execution position
    pc int
    
    // Stack (part of frame, but convenient accessor)
    sp int // Stack pointer
}

// Stack accessor methods
func (vm *VM) stack() []Value {
    if vm.frame == nil {
        return nil
    }
    base := len(vm.frame.ArgBuf) + len(vm.frame.VarBuf)
    return vm.frame.ArgBuf[base:][vm.frame.ArgCount:][:0] // Trick to get slice header
}

// For simplicity, use a separate stack slice
type VM struct {
    ctx  *Context
    rt   *Runtime
    frame *Frame
    bc   *FunctionBytecode
    pc   int
    sp   int
    stack []Value  // Separate stack for simplicity
}
```

### Fetch and Dispatch

```go
// Run starts the VM execution loop
func (vm *VM) Run() Value {
    for {
        opcode := vm.fetch()
        if !vm.execute(opcode) {
            break
        }
    }
    return vm.retVal
}

// fetch reads the next opcode
func (vm *VM) fetch() Opcode {
    op := Opcode(vm.bc.Code[vm.pc])
    vm.pc++
    return op
}

// execute runs a single opcode
func (vm *VM) execute(op Opcode) bool {
    switch op {
    case OPUndefined:
        vm.push(JSUndefined)
    case OPPushI32:
        vm.push(vm.readI32())
    case OPDrop:
        vm.pop().Free(vm.ctx)
    case OPDup:
        vm.push(vm.stack[vm.sp-1].Dup())
    case OPAdd:
        return vm.opAdd()
    case OPGoto:
        vm.opGoto()
    case OPCall:
        return vm.opCall()
    case OPReturn:
        return vm.opReturn()
    // ... etc
    default:
        vm.throwTypeError("invalid opcode")
        return false
    }
    return true
}
```

### Stack Operations

```go
func (vm *VM) push(v Value) {
    if vm.sp >= len(vm.stack) {
        // Expand stack
        newStack := make([]Value, len(vm.stack)*2)
        copy(newStack, vm.stack)
        vm.stack = newStack
    }
    vm.stack[vm.sp] = v.Dup()
    vm.sp++
}

func (vm *VM) pop() Value {
    vm.sp--
    return vm.stack[vm.sp]
}

func (vm *VM) top() Value {
    return vm.stack[vm.sp-1]
}

func (vm *VM) setTop(v Value) {
    vm.stack[vm.sp-1] = v
}
```

### Reading Operands

```go
func (vm *VM) readU8() uint8 {
    v := vm.bc.Code[vm.pc]
    vm.pc++
    return v
}

func (vm *VM) readI8() int8 {
    return int8(vm.readU8())
}

func (vm *VM) readU16() uint16 {
    b := vm.bc.Code[vm.pc:vm.pc+2]
    vm.pc += 2
    return uint16(b[0]) | uint16(b[1])<<8
}

func (vm *VM) readI16() int16 {
    return int16(vm.readU16())
}

func (vm *VM) readU32() uint32 {
    b := vm.bc.Code[vm.pc:vm.pc+4]
    vm.pc += 4
    return uint32(b[0]) | uint32(b[1])<<8 | 
           uint32(b[2])<<16 | uint32(b[3])<<24
}

func (vm *VM) readI32() int32 {
    return int32(vm.readU32())
}

func (vm *VM) readAtom() Atom {
    return Atom(vm.readU32())
}
```

### Arithmetic Operations

```go
func (vm *VM) opAdd() bool {
    rhs := vm.pop()
    lhs := vm.pop()
    result := addValues(vm.ctx, lhs, rhs)
    lhs.Free(vm.ctx)
    rhs.Free(vm.ctx)
    if result.IsException() {
        vm.throw(result)
        return false
    }
    vm.push(result)
    return true
}

func addValues(ctx *Context, lhs, rhs Value) Value {
    // Fast path: both integers
    if lhs.IsInt() && rhs.IsInt() {
        l := lhs.Int32()
        r := rhs.Int32()
        result := int64(l) + int64(r)
        if int32(result) == result {
            return NewInt32(int32(result))
        }
        return NewFloat64(float64(result))
    }
    
    // Fast path: both floats
    if lhs.IsFloat() && rhs.IsFloat() {
        return NewFloat64(lhs.Float64() + rhs.Float64())
    }
    
    // String concatenation
    if lhs.IsString() && rhs.IsString() {
        return concatStrings(ctx, lhs, rhs)
    }
    
    // Slow path: handle other types
    return addValuesSlow(ctx, lhs, rhs)
}

func (vm *VM) opSub() bool {
    rhs := vm.pop()
    lhs := vm.pop()
    // Similar to add but with subtraction
    // ... implementation
    return true
}
```

### Function Call

```go
func (vm *VM) opCall() bool {
    argc := int(vm.readU16())
    args := vm.stack[vm.sp-argc-1 : vm.sp]
    fn := vm.stack[vm.sp-argc-1]
    
    // Save PC for backtrace
    vm.frame.PC = vm.pc
    
    // Create new frame
    if !vm.call(fn, vm.top(), argc, args) {
        return false
    }
    
    // Clean up stack
    for i := 0; i <= argc; i++ {
        vm.stack[vm.sp-argc-1+i].Free(vm.ctx)
    }
    vm.sp -= argc + 1
    vm.push(vm.retVal)
    return true
}

func (vm *VM) call(fn, this Value, argc int, args []Value) bool {
    obj := fn.GetObject()
    
    // Check if it's a bytecode function
    if obj.Class.ID == ClassBytecodeFunction {
        bc := obj.Bytecode
        
        // Allocate new frame
        frame := &Frame{
            Prev:     vm.frame,
            Func:     fn,
            ArgCount: argc,
            IsStrict: bc.IsStrict,
            Realm:    vm.ctx,
            Bytecode: bc,
        }
        
        // Allocate and initialize locals
        total := bc.ArgCount + bc.VarCount + bc.StackSize
        mem := make([]Value, total)
        
        frame.ArgBuf = mem[:bc.ArgCount]
        frame.VarBuf = mem[bc.ArgCount:bc.ArgCount+bc.VarCount]
        frame.Stack = mem[bc.ArgCount+bc.VarCount:]
        
        // Copy arguments
        for i := 0; i < bc.ArgCount; i++ {
            if i < argc {
                frame.ArgBuf[i] = args[i].Dup()
            } else {
                frame.ArgBuf[i] = JSUndefined
            }
        }
        
        // Initialize vars to undefined
        for i := range frame.VarBuf {
            frame.VarBuf[i] = JSUndefined
        }
        
        // Setup var refs
        frame.VarRefs = obj.VarRefs
        
        // Switch to new frame
        vm.frame = frame
        vm.bc = bc
        vm.pc = 0
        vm.sp = 0
        vm.stack = frame.Stack
        
        return true
    }
    
    // Call native function
    result := obj.Class.Call(vm.ctx, fn, this, argc, args)
    if result.IsException() {
        vm.retVal = result
        return false
    }
    vm.retVal = result
    return true
}
```

### Return

```go
func (vm *VM) opReturn() bool {
    retVal := vm.pop()
    
    // Restore previous frame
    prev := vm.frame.Prev
    if prev == nil {
        // Returning from top-level
        vm.retVal = retVal
        return false
    }
    
    // Save return value
    vm.retVal = retVal
    
    // Close var refs if needed
    if vm.frame.Bytecode.VarRefCount > 0 {
        closeVarRefs(vm.frame)
    }
    
    // Free local variables
    for i := 0; i < vm.sp; i++ {
        vm.stack[i].Free(vm.ctx)
    }
    
    // Restore frame
    vm.frame = prev
    vm.bc = prev.Bytecode
    vm.pc = prev.PC
    vm.stack = prev.Stack
    
    return true
}
```

### Control Flow

```go
func (vm *VM) opGoto() bool {
    offset := vm.readI32()
    vm.pc += int(offset)
    
    // Check for interrupts
    if vm.ctx.rt.shouldInterrupt() {
        vm.throw(JSException)
        return false
    }
    return true
}

func (vm *VM) opIfFalse() bool {
    offset := vm.readI32()
    cond := vm.pop()
    
    isFalse := false
    if cond.tag <= TagUndefined {
        // Fast path for primitives
        isFalse = cond.Int32() == 0
    } else {
        // Slow path: call ToBoolean
        isFalse = !toBoolean(cond)
    }
    cond.Free(vm.ctx)
    
    if isFalse {
        vm.pc += int(offset)
    }
    return true
}
```

### Exception Handling

```go
func (vm *VM) throw(val Value) {
    vm.ctx.rt.currentException = val
    vm.frame.PC = vm.pc
    
    // Try to find a handler
    for {
        // Look for catch offset on stack
        for i := 0; i < vm.sp; i++ {
            if vm.stack[i].tag == TagCatchOffset {
                // Found handler
                offset := vm.stack[i].Int32()
                
                // Clear exception
                vm.ctx.rt.currentException = JSUndefined
                
                // Push exception value
                vm.stack[i] = vm.ctx.rt.currentException.Dup()
                vm.sp = i + 1
                
                // Jump to handler
                vm.pc = int(offset)
                return
            }
        }
        
        // No handler in this frame
        if vm.frame.Prev == nil {
            // Propagate to caller
            break
        }
        
        // Unwind to previous frame
        vm.unwindFrame()
    }
}

func (vm *VM) unwindFrame() {
    // Free all values on stack
    for i := 0; i < vm.sp; i++ {
        vm.stack[i].Free(vm.ctx)
    }
    
    // Restore previous frame
    vm.frame = vm.frame.Prev
    if vm.frame != nil {
        vm.bc = vm.frame.Bytecode
        vm.pc = vm.frame.PC
        vm.stack = vm.frame.Stack
        vm.sp = vm.frame.SavedSP
    }
}
```

### Generator Support

```go
type Generator struct {
    frame *Frame
    pc    int
    sp    int
    stack []Value
}

func (vm *VM) opYield() bool {
    // Save current state
    gen := &Generator{
        frame: vm.frame,
        pc:    vm.pc,
        sp:    vm.sp,
        stack: make([]Value, vm.sp),
    }
    copy(gen.stack, vm.stack[:vm.sp])
    
    // Get yielded value
    value := vm.pop()
    gen.value = value
    
    vm.retVal = vm.ctx.makeGeneratorObject(gen)
    return false // Exit VM
}

func (vm *VM) resumeGenerator(gen *Generator, sent Value) bool {
    // Restore frame
    vm.frame = gen.frame
    vm.pc = gen.pc
    vm.sp = gen.sp
    copy(vm.stack[:vm.sp], gen.stack)
    
    // Push sent value (what generator receives from .next())
    vm.push(sent)
    
    // Continue execution
    return true
}
```

---

## Key Design Decisions

### 1. Separate Stack vs. Embedded

**C approach:** Single alloca block with args, vars, and stack together.

**Go options:**
- Separate slice for stack (simpler)
- Single slice like C (more efficient)

**Recommendation:** Start with separate stack for simplicity.

### 2. Opcode Dispatch

Options:
1. Simple switch (as shown above)
2. Function table: `handlers [256]func(*VM) bool`
3. Computed goto (not available in Go)

**Recommendation:** Function table for better performance after initial implementation.

### 3. Value Representation

Options:
1. Tagged pointer (as shown above)
2. interface{} style
3. Go generics

**Recommendation:** Tagged pointer for efficient memory use and GC compatibility.

### 4. Reference Counting

C uses manual refcounting. Go options:
1. Manual refcounting (like C)
2. Go GC (automatic)
3. Hybrid (refcount for cycles, GC for others)

**Recommendation:** Start with manual refcounting to match C behavior exactly. This is critical for correct GC integration.

---

## Testing Strategy

### 1. Opcode Tests
Test each opcode in isolation:
```go
func TestAddInts(t *testing.T) {
    vm := NewVM()
    vm.push(NewInt32(1))
    vm.push(NewInt32(2))
    vm.opAdd()
    result := vm.pop()
    assert.Equal(t, int32(3), result.Int32())
}
```

### 2. Bytecode Execution Tests
```go
func TestSimpleFunction(t *testing.T) {
    vm := NewVM()
    bc := &FunctionBytecode{
        Code: []byte{
            byte(OPPushI32), 0, 0, 0, 1,  // push 1
            byte(OPPushI32), 0, 0, 0, 2,  // push 2
            byte(OPAdd),                    // add
            byte(OPReturn),                 // return
        },
        StackSize: 2,
    }
    vm.bc = bc
    vm.Run()
    assert.Equal(t, int32(3), vm.retVal.Int32())
}
```

### 3. Reference Tests
Test closure behavior:
```go
func TestClosure(t *testing.T) {
    // Create a closure that captures a variable
    // Verify the captured value persists
}
```

### 4. Exception Tests
Test try/catch and error propagation.

---

## Performance Considerations

### 1. Stack Pre-allocation
```go
// Instead of growing dynamically
stack := make([]Value, 1024)  // Pre-allocate
```

### 2. Inline Fast Paths
```go
// For common operations
func (v Value) AddInt32(r int32) Value {
    if v.tag == TagInt {
        result := int64(v.Int32()) + int64(r)
        if int32(result) == result {
            return NewInt32(int32(result))
        }
        return NewFloat64(float64(result))
    }
    return addSlow(v, NewInt32(r))
}
```

### 3. Batch Free
```go
// Free multiple values at once
func freeRange(ctx *Context, vals []Value) {
    for _, v := range vals {
        v.Free(ctx)
    }
}
```

---

## Project Structure

```
quickjs/
├── opcode.go           // Opcode constants and types
├── value.go            // Value type and operations
├── object.go           // Object and related types
├── bytecode.go         // FunctionBytecode struct
├── frame.go            // Frame struct
├── vm.go               // VM struct and main loop
├── ops_*.go            // Opcode implementations
├── builtins.go         // Built-in functions
└── generated/
    └── opcode_table.go // Generated opcode table
```

---

## Next Steps

1. Implement core Value and Object types
2. Implement FunctionBytecode and Frame
3. Implement VM with basic opcodes
4. Add reference counting
5. Implement function call/return
6. Add exception handling
7. Add generator support
8. Implement built-in objects and functions
9. Add garbage collection
10. Integration testing