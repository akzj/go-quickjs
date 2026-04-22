# QuickJS to Go Package Mapping

## Recommended Package Structure

Based on architecture analysis, here's the recommended Go package structure:

```
go-quickjs/
├── go.mod
│
├── pkg/
│   ├── runtime/           # JSRuntime equivalent
│   │   ├── runtime.go
│   │   ├── atom.go        # atom table
│   │   ├── shape.go       # shape hash table
│   │   ├── gc.go          # mark-and-sweep
│   │   └── job.go         # job queue (microtasks)
│   │
│   ├── context/           # JSContext equivalent
│   │   ├── context.go
│   │   ├── intrinsics.go  # built-in objects
│   │   ├── eval.go        # eval handling
│   │   └── module.go      # module system
│   │
│   ├── value/             # JSValue representation
│   │   ├── value.go       # core Value type
│   │   ├── number.go      # int/float handling
│   │   ├── tag.go         # type tags
│   │   └── conversions.go # ToString, ToNumber, etc.
│   │
│   ├── object/            # JSObject system
│   │   ├── object.go
│   │   ├── property.go
│   │   ├── shape.go
│   │   ├── prototype.go
│   │   └── descriptor.go
│   │
│   ├── string/           # JSString system
│   │   ├── string.go
│   │   ├── rope.go       # concatenation optimization
│   │   ├── atom.go       # atom functions
│   │   └── utf.go        # UTF handling
│   │
│   ├── symbol/           # JS Symbol
│   │   └── symbol.go
│   │
│   ├── function/         # Function objects
│   │   ├── function.go
│   │   ├── bytecode.go   # JSFunctionBytecode
│   │   ├── bound.go      # JSBoundFunction
│   │   ├── closure.go    # closure handling
│   │   ├── varref.go     # JSVarRef
│   │   └── frame.go      # StackFrame
│   │
│   ├── builtin/          # Built-in objects
│   │   ├── builtin.go    # base definitions
│   │   ├── object.go
│   │   ├── function.go
│   │   ├── array.go
│   │   ├── string.go
│   │   ├── number.go
│   │   ├── boolean.go
│   │   ├── symbol.go
│   │   ├── regexp.go
│   │   ├── date.go
│   │   ├── error.go
│   │   ├── arraybuffer.go
│   │   ├── typedarray.go
│   │   ├── map.go
│   │   ├── set.go
│   │   ├── weakmap.go
│   │   ├── weakset.go
│   │   ├── promise.go
│   │   ├── iterator.go
│   │   └── proxy.go
│   │
│   ├── vm/               # Virtual Machine
│   │   ├── vm.go         # main loop
│   │   ├── opcode.go     # opcode definitions
│   │   ├── stack.go      # operand stack
│   │   ├── call.go       # function call handling
│   │   └── interrupt.go  # interrupt handling
│   │
│   ├── parser/           # Lexer and Parser
│   │   ├── parser.go
│   │   ├── lexer.go
│   │   ├── token.go
│   │   ├── ast.go        # AST nodes
│   │   ├── expression.go
│   │   ├── statement.go
│   │   ├── function.go
│   │   ├── class.go
│   │   ├── module.go
│   │   └── error.go
│   │
│   ├── compiler/         # Bytecode compiler
│   │   ├── compiler.go
│   │   ├── emitter.go    # bytecode emission
│   │   ├── constant.go   # constant pool
│   │   ├── label.go      # label resolution
│   │   └── scope.go      # scope handling
│   │
│   └── api/              # Public API
│       ├── api.go        # main API functions
│       ├── eval.go
│       ├── call.go
│       ├── property.go
│       └── module.go
│
├── internal/
│   ├── test/
│   │   ├── bytecode_test.go
│   │   ├── parser_test.go
│   │   └── vm_test.go
│   └── fixtures/
│       └── testdata/
│
└── tools/
    ├── compare/          # Bytecode comparison tools
    │   ├── main.go
    │   ├── bytecode.go
    │   └── diff.go
    └── dump/
        ├── main.go
        └── disasm.go
```

---

## Type Mapping Table

### Fundamental Types

| C Type | Go Type | Package | Notes |
|--------|---------|---------|-------|
| `JSRuntime` | `Runtime` | `runtime` | Singleton per process |
| `JSContext` | `Context` | `context` | Per-thread isolation |
| `JSValue` | `Value` | `value` | Tagged union |
| `JSObject` | `Object` | `object` | Base for all objects |
| `JSClassID` | `ClassID` | `object` | uint32 |
| `JSAtom` | `Atom` | `atom` | Interned string |
| `JSFunctionBytecode` | `FunctionBytecode` | `function` | Compiled function |
| `JSStackFrame` | `StackFrame` | `function` | Call frame |
| `JSShape` | `Shape` | `object` | Property layout |

### Primitive Types

| C JSValue Tag | Go Value Constructor | Notes |
|--------------|---------------------|-------|
| `JS_TAG_INT` | `value.NewInt(int32)` | |
| `JS_TAG_BOOL` | `value.NewBool(bool)` | |
| `JS_TAG_NULL` | `value.Null` | singleton |
| `JS_TAG_UNDEFINED` | `value.Undefined` | singleton |
| `JS_TAG_FLOAT64` | `value.NewFloat64(float64)` | |
| `JS_TAG_STRING` | `value.NewString(*string.String)` | |
| `JS_TAG_SYMBOL` | `value.NewSymbol(*symbol.Symbol)` | |
| `JS_TAG_BIG_INT` | `value.NewBigInt(*big.Int)` | |
| `JS_TAG_OBJECT` | `value.NewObject(*object.Object)` | |

---

## C → Go Pattern Translations

### Pattern 1: Reference Counting

**C Pattern:**
```c
void JS_FreeValue(JSContext *ctx, JSValue v) {
    if (JS_VALUE_HAS_REF_COUNT(v)) {
        JSRefCountHeader *p = (JSRefCountHeader *)JS_VALUE_GET_PTR(v);
        if (--p->ref_count <= 0) {
            __JS_FreeValue(ctx, v);
        }
    }
}
```

**Go Pattern (Reference Counting with GC):**
```go
func (v Value) Free(ctx *Context) {
    if !v.HasRefCount() {
        return
    }
    obj := v.Object()
    if atomic.AddInt32(&obj.refCount, -1) <= 0 {
        ctx.runtime.freeObject(obj)
    }
}

// Or using Go's GC (simpler but less control):
type Object struct {
    header gcObjectHeader
    // ...
}

type gcObjectHeader struct {
    // No manual refcount - let Go GC handle it
    // But need to track for cycles
}
```

**Recommendation:** For initial implementation, use Go's GC. For performance, add manual reference counting later.

---

### Pattern 2: Tagged Union with Interface

**C Pattern:**
```c
typedef struct JSProperty {
    union {
        JSValue value;           // JS_PROP_NORMAL
        JSObject *getter;        // JS_PROP_GETSET
        JSVarRef *var_ref;       // JS_PROP_VARREF
    } u;
} JSProperty;
```

**Go Pattern (Interface):**
```go
type PropertyValue interface {
    isPropertyValue()
}

type PropertyValueNormal struct {
    value Value
}

type PropertyValueGetterSetter struct {
    Getter *Object  // or Value for functions
    Setter *Object
}

type PropertyValueVarRef struct {
    Ref *VarRef
}

type Property struct {
    Value PropertyValue
    Flags PropertyFlags
}
```

**Or (more efficient, tagged approach):**
```go
type PropertyValue struct {
    tag    PropertyValueTag
    value  Value      // for normal
    getter *Object    // for getter/setter
    setter *Object
    varRef *VarRef    // for varref
}

type PropertyValueTag uint8

const (
    PropertyValueNormal PropertyValueTag = iota
    PropertyValueGetterSetter
    PropertyValueVarRef
)
```

---

### Pattern 3: Error Handling with Exceptions

**C Pattern:**
```c
static __exception JSValue js_add(JSContext *ctx, JSValueConst op1, JSValueConst op2) {
    if (JS_IsInteger(op1) && JS_IsInteger(op2)) {
        int64_t res = (int64_t)JS_VALUE_GET_INT(op1) + (int64_t)JS_VALUE_GET_INT(op2);
        if (res == (int)res)
            return JS_NewInt32(ctx, res);
    }
    return JS_Add(ctx, op1, op2);  // may throw
}
```

**Go Pattern (Error-based):**
```go
func Add(ctx *Context, op1, op2 Value) (Value, error) {
    if op1.IsInt() && op2.IsInt() {
        res := int64(op1.Int32()) + int64(op2.Int32())
        if res >= math.MinInt32 && res <= math.MaxInt32 {
            return value.NewInt(int32(res)), nil
        }
    }
    return nil, ctx.Throw(NewTypeError("cannot add"))
}

// Usage:
result, err := Add(ctx, a, b)
if err != nil {
    return value.Exception
}
```

**Or (Panic/Recover for internal, error for API):**
```go
func (ctx *Context) try(f func() Value) (Value, bool) {
    defer func() {
        if r := recover(); r != nil {
            ctx.exception = r.(error)
        }
    }()
    return f(), true
}
```

---

### Pattern 4: Function Pointer Callbacks

**C Pattern:**
```c
typedef JSValue (*JSCFunction)(JSContext *ctx, JSValueConst this_val,
                               int argc, JSValueConst *argv);

typedef struct JSCFunctionListEntry {
    const char *name;
    uint8_t prop_flags;
    uint8_t def_type;
    int16_t magic;
    union {
        struct {
            uint8_t length;
            uint8_t cproto;
            JSCFunctionType cfunc;
        } func;
        // ...
    } u;
} JSCFunctionListEntry;
```

**Go Pattern (Interface + Struct):**
```go
type Function interface {
    Call(ctx *Context, this Value, args []Value) (Value, error)
}

// For builtin functions
type BuiltinFunc struct {
    Name    string
    Length  int
    Magic   int
    Impl    func(ctx *Context, this Value, args []Value, magic int) (Value, error)
}

func (f *BuiltinFunc) Call(ctx *Context, this Value, args []Value) (Value, error) {
    // Pad or truncate args to expected length
    return f.Impl(ctx, this, args, f.Magic)
}

// For builtin methods on objects
type BuiltinMethod struct {
    Name   string
    Impl   func(ctx *Context, this Value, args []Value, magic int) (Value, error)
}

// Property definition
type PropertyDef struct {
    Name      string
    Flags     PropertyFlags
    DefType   DefType
    Value     interface{}  // Value, *BuiltinFunc, *BuiltinMethod, etc.
    Magic     int
}
```

---

### Pattern 5: GC Mark Functions

**C Pattern:**
```c
static void js_object_mark(JSRuntime *rt, JSValueConst val, JS_MarkFunc *mark_func) {
    JSObject *p = JS_VALUE_GET_OBJ(val);
    // Mark shape
    mark_func(rt, &p->shape->header);
    // Mark prototype
    if (p->shape->proto)
        mark_func(rt, &p->shape->proto->header);
    // Mark properties
    for (int i = 0; i < p->shape->prop_count; i++) {
        if (JS_VALUE_HAS_REF_COUNT(p->prop[i].u.value))
            mark_func(rt, JS_VALUE_GET_PTR(p->prop[i].u.value));
    }
    // Mark type-specific data
    switch (p->class_id) {
    case JS_CLASS_ARRAY:
        // mark array elements
        // ...
    }
}
```

**Go Pattern (Visitor/Interface):**
```go
type GCMarker interface {
    Mark(gc *GC)
}

func (obj *Object) Mark(gc *GC) {
    if obj.marked {
        return
    }
    obj.marked = true
    
    // Mark shape
    gc.MarkObject(obj.shape)
    
    // Mark prototype
    if obj.shape.proto != nil {
        gc.MarkObject(obj.shape.proto)
    }
    
    // Mark properties
    for _, prop := range obj.props {
        if prop.Value != nil {
            gc.MarkValue(prop.Value)
        }
    }
    
    // Mark type-specific data
    switch obj.classID {
    case ClassArray:
        gc.MarkValues(obj.u.Array.values)
    case ClassFunction:
        gc.MarkObject(obj.u.Func.bytecode)
        gc.MarkValues(obj.u.Func.varRefs...)
    }
}

// Or use a visitor pattern:
func (gc *GC) MarkObject(obj *Object) {
    // ... mark logic
}

// Runtime marks all roots:
func (gc *GC) markRoots() {
    // Mark context globals
    for _, ctx := range gc.runtime.contexts {
        gc.MarkObject(ctx.globalObj)
    }
    // Mark atoms
    for _, atom := range gc.runtime.atoms {
        gc.MarkValue(value.NewString(atom))
    }
}
```

---

### Pattern 6: Virtual Method Tables (Exotic Objects)

**C Pattern:**
```c
typedef struct JSClassExoticMethods {
    int (*get_own_property)(...);
    int (*get_own_property_names)(...);
    int (*delete_property)(...);
    int (*define_own_property)(...);
    int (*has_property)(...);
    JSValue (*get_property)(...);
    int (*set_property)(...);
    JSValue (*get_prototype)(...);
    int (*set_prototype)(...);
    int (*is_extensible)(...);
    int (*prevent_extensions)(...);
} JSClassExoticMethods;
```

**Go Pattern (Interface):**
```go
// ExoticMethods interface for objects with custom behavior
type ExoticMethods interface {
    GetOwnProperty(ctx *Context, prop Atom) (PropertyDescriptor, bool, error)
    GetOwnPropertyNames(ctx *Context) ([]Atom, error)
    Delete(ctx *Context, prop Atom, flags int) error
    DefineOwnProperty(ctx *Context, prop Atom, desc PropertyDescriptor) error
    HasProperty(ctx *Context, prop Atom) (bool, error)
    Get(ctx *Context, prop Atom, receiver Value) (Value, error)
    Set(ctx *Context, prop Atom, value Value, receiver Value) error
    GetPrototype(ctx *Context) *Object
    SetPrototype(ctx *Context, proto *Object) error
    IsExtensible(ctx *Context) bool
    PreventExtensions(ctx *Context) error
}

// Proxy implements ExoticMethods
type Proxy struct {
    Object
    target *Object
    handler *Object
}

func (p *Proxy) GetOwnProperty(ctx *Context, prop Atom) (PropertyDescriptor, bool, error) {
    // Forward to handler
    // ...
}
```

---

### Pattern 7: Dynamic Buffer (DynBuf)

**C Pattern:**
```c
typedef struct DynBuf {
    uint8_t *buf;
    size_t size;
    size_t allocated_size;
    void *opaque;
    int error;
} DynBuf;

static void dynbuf_putc(DynBuf *s, uint8_t c) {
    if (s->size < s->allocated_size) {
        s->buf[s->size++] = c;
    } else {
        dynbuf_realloc(s, s->size + 1);
        s->buf[s->size++] = c;
    }
}
```

**Go Pattern (bytes.Buffer):**
```go
type ByteBuffer struct {
    buf []byte
}

func (b *ByteBuffer) WriteByte(c byte) error {
    b.buf = append(b.buf, c)
    return nil
}

func (b *ByteBuffer) Write(p []byte) (n int, err error) {
    b.buf = append(b.buf, p...)
    return len(p), nil
}

func (b *ByteBuffer) Bytes() []byte {
    return b.buf
}

// Or just use bytes.Buffer from stdlib
var buf bytes.Buffer
buf.WriteByte(opcode)
binary.Write(&buf, binary.LittleEndian, uint32(index))
```

---

## Implementation Phases

Based on the architecture analysis, here's a recommended implementation order:

### Phase 1: Foundation
1. **value package** - JSValue, tags, basic operations
2. **runtime package** - Runtime singleton, atom table, gc
3. **context package** - Context creation, basic setup

### Phase 2: Object System
4. **object package** - Objects, properties, shape
5. **string package** - Strings, atoms
6. **symbol package** - Symbols

### Phase 3: VM Foundation
7. **vm package** - Stack, frames, basic opcodes
8. **function package** - FunctionBytecode, closures

### Phase 4: Built-ins (Prerequisites)
9. **builtin package** - Base setup, intrinsics
10. Individual builtin packages (Object, Function, Array)

### Phase 5: Parser & Compiler
11. **parser package** - Lexer, parser, AST
12. **compiler package** - Bytecode emission

### Phase 6: Full VM Integration
13. Connect parser → compiler → vm
14. Implement remaining built-ins

### Phase 7: Advanced Features
15. **promise package** - Promises, async/await
16. **module package** - ES Modules
17. Iterator protocols, generators

---

## Key Design Decisions

### Decision 1: NaN Boxing vs Tagged Interface

**Option A: NaN Boxing (like C QuickJS)**
```go
type Value struct {
    raw uint64
}
```

**Pros:** Memory efficient, matches C semantics exactly
**Cons:** Complex bit manipulation, hard to extend

**Option B: Tagged Interface (Go idiomatic)**
```go
type Value interface {
    tag()
}

type intValue int32
type objectValue *Object
type stringValue *String
// etc.
```

**Pros:** Idiomatic Go, type-safe, garbage collected
**Cons:** Higher memory usage, different semantics

**Recommendation:** **Option B** for Go rewrite. Simpler implementation, better Go integration.

---

### Decision 2: GC Strategy

**Option A: Reference Counting (like QuickJS)**
- Explicit `AddRef()`/`Release()` everywhere
- Cycle detection with weak refs

**Option B: Go GC**
- Trust Go's garbage collector
- No manual reference counting
- Need to mark GC roots explicitly

**Recommendation:** **Option B** for simplicity. Add reference counting later for performance if needed.

---

### Decision 3: Error Handling

**Option A: Errors (Go idiomatic)**
```go
func Add(ctx *Context, a, b Value) (Value, error)
```

**Option B: Exceptions (like QuickJS)**
```go
func Add(ctx *Context, a, b Value) Value
// Panics on error, ctx stores exception
```

**Recommendation:** **Option A** internally, **Option B** at API boundary. Internal functions return errors for clean control flow. API wraps in try/catch pattern.

---

### Decision 4: Opcode Dispatch

**Option A: Switch Statement**
```go
for {
    switch opcode := code[pc]; opcode {
    case OpAdd:
        // ...
    }
}
```

**Option B: Computed Goto (C extension)**
```go
// Not directly available in Go
// Could use function pointers
```

**Option C: Table-Driven**
```go
var handlers = map[Opcode]func(*VM){
    OpAdd: (*VM).handleAdd,
}
```

**Recommendation:** **Option A** for simplicity. Consider asm (plan9 or external asm) for performance later.

---

## Common Pitfalls

### Pitfall 1: Prototype Chain Search
```go
// WRONG: O(n) prototype chain traversal
func (obj *Object) Get(prop Atom) Value {
    for o := obj; o != nil; o = o.Prototype() {
        if val := o.getOwnProperty(prop); val != nil {
            return val
        }
    }
    return value.Undefined
}

// RIGHT: Use shape hash for fast lookup
func (obj *Object) Get(prop Atom) Value {
    for shape := obj.shape; shape != nil; shape = shape.protoShape {
        if idx := shape.findProperty(prop); idx >= 0 {
            return obj.props[idx].Value
        }
    }
    return value.Undefined
}
```

### Pitfall 2: Stack Overflow in Recursion
```go
// WRONG: Deep recursion in parsing
func parseExpression(tokens []Token) Node {
    if isBinaryOp(token) {
        left := parseExpression(tokens)
        return &BinaryNode{left, parseExpression(tokens)}
    }
}

// RIGHT: Iterative with explicit stack for complex cases
// Parser handles this naturally with recursive descent,
// but be careful with deeply nested expressions
```

### Pitfall 3: Closure Variable Escape
```go
// WRONG: Capturing loop variable
func compile() []bytecode {
    var closures []*Bytecode
    for i := 0; i < 10; i++ {
        closures = append(closures, &Bytecode{captures: func() int { return i }})
    }
    // All closures capture the same i!
}

// RIGHT: Capture loop variable explicitly
func compile() []*Bytecode {
    closures := make([]*Bytecode, 10)
    for i := 0; i < 10; i++ {
        captured := i  // copy
        closures[i] = &Bytecode{captures: func() int { return captured }}
    }
}
```

### Pitfall 4: Concurrent Context Access
```go
// WRONG: Sharing context across goroutines
func worker(ctx *Context) {
    JS_Eval(ctx, "...")
}

go worker(ctx1)
go worker(ctx2)  // Both access same context!

// RIGHT: Each goroutine gets its own context
rt := JS_NewRuntime()
go func() {
    ctx := JS_NewContext(rt)
    JS_Eval(ctx, "...")
}()
```

---

## Testing Strategy

### 1. Bytecode Comparison (BDD - Bytecode-Driven Development)
```
tools/compare/
├── main.go          # Compare bytecode output
├── bytecode.go      # Parse/print bytecode
└── diff.go          # Diff visualization
```

### 2. Test Fixtures
```
tests/
├── fixtures/
│   ├── arithmetic.js
│   ├── functions.js
│   ├── closures.js
│   ├── objects.js
│   ├── arrays.js
│   ├── strings.js
│   ├── async/
│   │   ├── promise.js
│   │   ├── async-function.js
│   │   └── generator.js
│   └── modules/
│       ├── import.js
│       ├── export.js
│       └── circular.js
└── quickjs_test.go
```

### 3. Test Harness
```go
func TestBytecodeEquivalence(t *testing.T) {
    cases := []struct {
        name   string
        source string
    }{
        {"add", "1 + 2"},
        {"closure", "(function() { var x = 1; return function() { return x; }; })()"},
        // ...
    }
    
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            // Get C QuickJS bytecode
            cBytecode := runCQuickJS(tc.source)
            
            // Get Go QuickJS bytecode
            goBytecode := runGoQuickJS(tc.source)
            
            // Compare
            if !bytes.Equal(cBytecode, goBytecode) {
                t.Fatalf("Bytecode mismatch:\nC:      %x\nGo:     %x\nDiff:   %x",
                    cBytecode, goBytecode, diff(cBytecode, goBytecode))
            }
        })
    }
}
```

---

## Performance Considerations

### 1. Value Representation
- NaN boxing saves memory but complicates code
- Consider 128-bit for better efficiency on 64-bit systems
- Use `sync.Pool` for frequently allocated objects

### 2. Property Access
- Shape-based caching is critical for performance
- Inline caches in VM (like V8's ICs)
- Fast paths for common property names

### 3. Function Calls
- Tail call optimization reduces stack usage
- Fast path for known call targets
- Inlining of built-in methods

### 4. GC
- Generational GC for young objects
- Write barriers for incremental marking
- Relaxed memory model for better parallelism

---

## Appendix: QuickJS → Go Type Cheatsheet

```
JSRuntime        → runtime.Runtime
JSContext        → context.Context
JSValue          → value.Value
JSObject         → object.Object
JSString         → string.String
JSAtom           → atom.Atom
JSFunctionBytecode → function.FunctionBytecode
JSStackFrame     → function.StackFrame
JSProperty       → object.Property
JSShape          → object.Shape
JSClassDef       → object.ClassDef
JSClassID        → object.ClassID

JS_NewRuntime    → runtime.New()
JS_FreeRuntime   → runtime.Close()
JS_NewContext    → context.New(runtime)
JS_FreeContext   → context.Close()
JS_Eval          → context.Eval()
JS_Call          → context.Call()
JS_GetProperty   → object.Object.Get()
JS_SetProperty   → object.Object.Set()
JS_NewObject     → object.New(context)
JS_NewArray      → array.New(context)

JS_MKVAL         → value.NewInt(), value.NewBool()
JS_VALUE_GET_TAG → value.Tag()
JS_VALUE_GET_INT → value.Int32()

js_malloc        → runtime.Alloc()
js_free          → runtime.Free()
```
