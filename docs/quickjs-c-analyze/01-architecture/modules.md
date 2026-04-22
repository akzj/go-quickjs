# QuickJS Module Detailed Analysis

## Module 1: Value Model (JSValue)

### Location
- Header: `quickjs.h` lines 60-280
- Implementation: `quickjs.c` lines scattered

### Design Principle
Single 64-bit type represents all JavaScript values using tagged union / NaN-boxing.

### C Code Analysis

**Tag System:**
```c
enum {
    JS_TAG_FIRST       = -9,
    JS_TAG_BIG_INT     = -9,
    JS_TAG_SYMBOL      = -8,
    JS_TAG_STRING      = -7,
    JS_TAG_STRING_ROPE = -6,
    JS_TAG_MODULE      = -3,
    JS_TAG_FUNCTION_BYTECODE = -2,
    JS_TAG_OBJECT      = -1,
    
    JS_TAG_INT         = 0,
    JS_TAG_BOOL        = 1,
    JS_TAG_NULL        = 2,
    JS_TAG_UNDEFINED   = 3,
    JS_TAG_UNINITIALIZED = 4,
    JS_TAG_CATCH_OFFSET = 5,
    JS_TAG_EXCEPTION   = 6,
    JS_TAG_SHORT_BIG_INT = 7,
    JS_TAG_FLOAT64     = 8,
};
```

**NaN-Boxing (64-bit):**
```c
typedef uint64_t JSValue;

#define JS_VALUE_GET_TAG(v) (int)((v) >> 32)
#define JS_MKVAL(tag, val) (((uint64_t)(tag) << 32) | (uint32_t)(val))

// Float64 special handling using quiet NaN encoding
#define JS_FLOAT64_TAG_ADDEND (0x7ff80000 - JS_TAG_FIRST + 1)

static inline double JS_VALUE_GET_FLOAT64(JSValue v) {
    union { JSValue v; double d; } u;
    u.v = v;
    u.v += (uint64_t)JS_FLOAT64_TAG_ADDEND << 32;
    return u.d;
}
```

### Go Rewrite Recommendation

```go
// package value

// Tag represents the JSValue type tag
type Tag int32

const (
    TagInt         Tag = 0
    TagBool        Tag = 1
    TagNull        Tag = 2
    TagUndefined   Tag = 3
    TagObject      Tag = -1
    TagString      Tag = -7
    TagSymbol      Tag = -8
    TagBigInt      Tag = -9
    TagFloat64     Tag = 8  // and above
)

// Value is a tagged union representing any JS value
// Uses 64 bits: upper 32 bits for tag, lower 32/64 for payload
type Value struct {
    raw uint64
}

// Constructors for immediate values
func NewInt(v int32) Value
func NewBool(v bool) Value
func NewNull() Value
func NewUndefined() Value
func NewFloat64(v float64) Value
func NewString(s *String) Value
func NewObject(o *Object) Value
func NewSymbol(s *Symbol) Value

// Accessors
func (v Value) Tag() Tag
func (v Value) Int32() int32
func (v Value) Bool() bool
func (v Value) Float64() float64
func (v Value) String() *String
func (v Value) Object() *Object

// Special values
var Undefined = Value{raw: makeRaw(TagUndefined, 0)}
var Null = Value{raw: makeRaw(TagNull, 0)}
var True = Value{raw: makeRaw(TagBool, 1)}
var False = Value{raw: makeRaw(TagBool, 0)}
```

### Pitfalls
1. **Float64 NaN handling**: NaN payloads must be normalized
2. **Pointer alignment**: 64-bit pointers must fit in payload
3. **BigInt encoding**: Uses different tag on 32 vs 64-bit systems

---

## Module 2: String & Atom Management

### Location
- `quickjs.c` lines 501-3400

### Design Principle
- **String**: Reference-counted, UTF-8 or UTF-16 internal encoding, rope-based concatenation
- **Atom**: Interned strings for property names, enables integer comparison

### C Code Analysis

**JSString Structure:**
```c
struct JSString {
    JSRefCountHeader header;  // ref_count, must be first
    uint32_t len : 31;
    uint8_t is_wide_char : 1;  // 0=8bit, 1=16bit
    uint32_t hash : 30;
    uint8_t atom_type : 2;
    uint32_t hash_next;  // for atom chaining
    union {
        uint8_t str8[0];
        uint16_t str16[0];
    } u;
};
```

**JSStringRope (for concatenation):**
```c
typedef struct JSStringRope {
    JSRefCountHeader header;
    uint32_t len;
    uint8_t is_wide_char;
    uint8_t depth;
    JSValue left;
    JSValue right;
} JSStringRope;
```

**Atom Table:**
```c
struct JSRuntime {
    int atom_hash_size;
    int atom_count;
    uint32_t *atom_hash;
    JSAtomStruct **atom_array;  // JSAtomStruct is JSString
    int atom_free_index;
};
```

**Atom Kinds:**
```c
typedef enum {
    JS_ATOM_TYPE_STRING = 1,
    JS_ATOM_TYPE_GLOBAL_SYMBOL,
    JS_ATOM_TYPE_SYMBOL,
    JS_ATOM_TYPE_PRIVATE,
} JSAtomTypeEnum;
```

### Go Rewrite Recommendation

```go
// package string

// String represents a JavaScript string
type String struct {
    header refCountHeader
    len    uint32
    isWide bool
    hash   uint32
    data   []byte  // UTF-8 or []uint16
}

// Rope for efficient concatenation
type Rope struct {
    String
    left  Value
    right Value
    depth uint8
}

// package atom

// Atom is an interned string (for property names)
type Atom uint32

const ATOM_NULL Atom = 0

// Atom table managed by Runtime
type AtomTable struct {
    mu          sync.Mutex
    hashSize    int
    count       int
    hash        []uint32
    array       []*stringValue  // interned strings
    freeIndex   int
}
```

### Pitfalls
1. **Rope flattening**: Must flatten before GC mark phase
2. **Atom deletion**: Complex, requires careful hash table management
3. **UTF-16 surrogate pairs**: Handling in is_wide_char mode

---

## Module 3: Object System

### Location
- `quickjs.c` lines 880-1200 (types), 5000-6400 (implementation)

### Design Principle
- **Shape**: Property structure sharing (like V8's "hidden classes")
- **Property Descriptor**: Flexible property storage (value, getter/setter, varref, autoinit)
- **Prototype Chain**: Linked list traversal for property lookup

### C Code Analysis

**JSShape (property structure):**
```c
struct JSShape {
    JSGCObjectHeader header;
    uint8_t is_hashed;
    uint32_t hash;
    uint32_t prop_hash_mask;
    int prop_size;
    int prop_count;
    int deleted_prop_count;
    JSShape *shape_hash_next;
    JSObject *proto;
    JSShapeProperty prop[0];  // flexible array
};

typedef struct JSShapeProperty {
    uint32_t hash_next : 26;
    uint32_t flags : 6;  // JS_PROP_xxx
    JSAtom atom;
} JSShapeProperty;
```

**JSObject:**
```c
struct JSObject {
    union {
        JSGCObjectHeader header;
        struct {
            uint8_t extensible : 1;
            uint8_t is_exotic : 1;  // custom property handlers
            uint8_t fast_array : 1;  // array optimization
            uint8_t is_constructor : 1;
            uint16_t class_id;
        };
    };
    uint32_t weakref_count;
    JSShape *shape;  // prototype + properties
    JSProperty *prop;
    union {
        void *opaque;
        // 30+ different structures based on class_id
        struct { JSObject *func; JSVarRef **var_refs; } func;
        struct { void *ptr; uint32_t count; } array;
        JSRegExp regexp;
        // ... many more
    } u;
};
```

**JSProperty:**
```c
typedef struct JSProperty {
    union {
        JSValue value;       // JS_PROP_NORMAL
        struct {             // JS_PROP_GETSET
            JSObject *getter;
            JSObject *setter;
        } getset;
        JSVarRef *var_ref;   // JS_PROP_VARREF (closure)
        struct {             // JS_PROP_AUTOINIT
            uintptr_t realm_and_id;
            void *opaque;
        } init;
    } u;
} JSProperty;
```

### Go Rewrite Recommendation

```go
// package object

// Object represents a JavaScript object
type Object struct {
    header   gcObjectHeader
    classID  uint16
    flags    objectFlags
    shape    *Shape
    props    []Property  // sparse array, aligned with shape
    u        objectUnion // class-specific data
}

type objectFlags uint8
const (
    FlagExtensible   objectFlags = 1 << iota
    FlagIsExotic
    FlagFastArray
    FlagIsConstructor
)

// Shape stores prototype and property layout
type Shape struct {
    header      gcObjectHeader
    isHashed    bool
    hash        uint32
    propHashMask uint32
    propSize    int
    propCount   int
    proto       *Object
    props       []ShapeProperty  // parallel to Object.props
}

type ShapeProperty struct {
    HashNext uint32  // index or 0 if last
    Flags    PropertyFlags
    Atom     atom.Atom
}

type PropertyFlags uint8
const (
    FlagConfigurable PropertyFlags = 1 << iota
    FlagWritable
    FlagEnumerable
    // ... plus type mask
)

// Different object types
type ArrayData struct {
    values []Value
    length uint32
}

type FunctionData struct {
    bytecode *FunctionBytecode
    varRefs []*VarRef
    homeObject *Object
}
```

### Pitfalls
1. **Shape transition**: Adding/removing properties creates new shape
2. **Prototype mutation**: Must update shape hash
3. **Property enumeration**: Order matters (insertion order for non-integer keys)
4. **Delete behavior**: Can mark property as deleted, affecting enumeration

---

## Module 4: Function Objects

### Location
- `quickjs.c` lines 614-680 (types), 5467-5600 (creation)

### Design Principle
Three function types: bytecode functions, C functions, and bound functions.

### C Code Analysis

**JSFunctionBytecode:**
```c
typedef struct JSFunctionBytecode {
    JSGCObjectHeader header;
    uint8_t js_mode;
    uint8_t has_prototype : 1;
    uint8_t has_simple_parameter_list : 1;
    uint8_t func_kind : 2;
    uint8_t *byte_code_buf;
    int byte_code_len;
    JSAtom func_name;
    JSBytecodeVarDef *vardefs;
    JSClosureVar *closure_var;
    uint16_t arg_count;
    uint16_t var_count;
    uint16_t stack_size;
    uint16_t var_ref_count;
    JSContext *realm;
    JSValue *cpool;  // constant pool
    int cpool_count;
} JSFunctionBytecode;
```

**JSBytecodeVarDef:**
```c
typedef struct JSBytecodeVarDef {
    JSAtom var_name;
    uint8_t is_arg;
    uint8_t is_lexical;
    uint8_t is_const;
    uint16_t var_ref_idx;
} JSBytecodeVarDef;
```

**JSClosureVar:**
```c
typedef struct JSClosureVar {
    JSAtom var_name;
    uint8_t is_arg : 1;
    uint8_t is_lexical : 1;
    uint8_t is_const : 1;
    uint8_t kind : 2;  // JS_CLOSURE_xxx
    int var_idx;
} JSClosureVar;
```

**JSBoundFunction:**
```c
typedef struct JSBoundFunction {
    JSValue func_obj;
    JSValue this_val;
    int argc;
    JSValue argv[0];  // bound arguments
} JSBoundFunction;
```

### Go Rewrite Recommendation

```go
// package function

// FunctionBytecode represents a compiled JS function
type FunctionBytecode struct {
    header      gcObjectHeader
    jsMode     uint8
    flags      bytecodeFuncFlags
    code       []byte
    name       atom.Atom
    argCount   uint16
    varCount   uint16
    stackSize  uint16
    varRefs    []*VarDef
    closureVars []*ClosureVar
    constPool  []value.Value
    debug      *DebugInfo
}

// StackFrame represents function execution context
type StackFrame struct {
    Prev      *StackFrame
    Func      value.Value
    ArgBuf    []value.Value
    VarBuf    []value.Value
    VarRefs   []*VarRef
    CurPC     uintptr  // program counter
    ArgCount  int
    // For generators:
    CurSP    []value.Value  // stack pointer when suspended
}

// BoundFunction wraps a function with pre-bound this and args
type BoundFunction struct {
    header   gcObjectHeader
    funcObj  value.Value
    thisVal  value.Value
    argc     int
    argv     []value.Value
}
```

### Pitfalls
1. **Closure capture**: Variables must be hoisted to heap when captured
2. **Rest parameters**: Become array at call site
3. **arguments object**: Lazy creation, aliased with actual parameters
4. **Generator state**: Must preserve stack on suspension

---

## Module 5: Virtual Machine

### Location
- `quickjs.c` lines 33283-43000 (VM loop)
- `quickjs-opcode.h` (opcode definitions)

### Design Principle
- **Register-based** VM with stack for expression evaluation
- **Short opcodes** for common cases (reduces bytecode size)
- **Computed goto** or switch-based dispatch

### C Code Analysis

**Main VM Loop (simplified):**
```c
static JSValue execute_bytecode(JSContext *ctx, JSValueConst func_obj, ...)
{
    JSStackFrame *sf = &ctx->current_stack_frame;
    uint8_t *pc = sf->cur_pc;
    // ... setup
    
    for (;;) {
        int opcode = *pc++;
        switch (opcode) {
        case OP_push_i32:
            // ...
        case OP_add:
            // ...
        case OP_call:
            // ...
        }
        // Interrupt check every N instructions
        if (--ctx->interrupt_counter <= 0) {
            // handle interrupt
        }
    }
}
```

**Stack Frame Layout:**
```
+------------------+
| return address   |
+------------------+
| previous frame   |
+------------------+
| function object  |
+------------------+
| arg count        |
+------------------+
| arguments        | ← arg_buf points here
+------------------+
| local variables  | ← var_buf points here
+------------------+
| temporaries      | ← stack grows here
+------------------+
```

### Opcode Categories (from quickjs-opcode.h):

**Stack Operations:**
- `push_i32`, `push_const`, `push_atom_value`
- `undefined`, `null`, `push_true`, `push_false`
- `drop`, `nip`, `dup`, `swap`, `rot3l`

**Function Calls:**
- `call`, `call_method`, `call_constructor`
- `tail_call`, `tail_call_method`
- `return`, `return_undef`

**Property Access:**
- `get_var`, `put_var` (module-level)
- `get_field`, `put_field`
- `get_array_el`, `put_array_el`

**Control Flow:**
- `goto`, `goto_if_true`, `goto_if_false`
- `catch`, `throw`

**Operators:**
- `add`, `sub`, `mul`, `div`, `mod`
- `neg`, `not`, `increment`, `decrement`
- `eq`, `neq`, `lt`, `lte`, `gt`, `gte`

### Go Rewrite Recommendation

```go
// package vm

// VM executes bytecode
type VM struct {
    ctx    *Context
    frames []*StackFrame
    stack  []value.Value
}

func (vm *VM) Execute(fn *function.FunctionBytecode) (value.Value, error) {
    frame := vm.pushFrame(fn)
    defer vm.popFrame()
    
    for {
        opcode := Opcode(frame.code[frame.pc])
        frame.pc++
        
        switch opcode {
        case OpPushI32:
            v := readI32(frame.code, &frame.pc)
            vm.push(value.NewInt(v))
        case OpAdd:
            rhs := vm.pop()
            lhs := vm.pop()
            result, err := binaryOp(vm.ctx, "+", lhs, rhs)
            if err != nil {
                return value.Exception, err
            }
            vm.push(result)
        // ... etc
        }
        
        // Interrupt check
        if vm.ctx.interruptCounter--; vm.ctx.interruptCounter <= 0 {
            if err := vm.handleInterrupt(); err != nil {
                return value.Exception, err
            }
        }
    }
}

func (vm *VM) push(v value.Value) {
    vm.stack = append(vm.stack, v)
}

func (vm *VM) pop() value.Value {
    n := len(vm.stack) - 1
    v := vm.stack[n]
    vm.stack[n] = value.Undefined  // help GC
    vm.stack = vm.stack[:n]
    return v
}
```

### Pitfalls
1. **Stack overflow**: Must check against limit before push
2. **Tail call optimization**: Must not grow stack
3. **Generator suspension**: Must save/restore entire frame state
4. **OPCODES vs short opcodes**: Two versions for size optimization

---

## Module 6: Garbage Collection

### Location
- `quickjs.c` lines 6410-7232 (mark and sweep)
- Reference counting throughout

### Design Principle
- **Reference counting** for immediate reclamation
- **Mark-and-sweep** to collect cycles
- **Weak references** for WeakMap/WeakSet

### C Code Analysis

**GC Phases:**
```c
typedef enum {
    JS_GC_PHASE_NONE,
    JS_GC_PHASE_DECREF,      // decrement refcounts
    JS_GC_PHASE_REMOVE_WEAK, // remove weak references
    JS_GC_PHASE_MARK,         // mark reachable
    JS_GC_PHASE_FINALIZE,     // call finalizers
} JSGCPhaseEnum;
```

**Mark Function Pattern:**
```c
static void js_object_mark(JSRuntime *rt, JSValueConst val, JS_MarkFunc *mark_func) {
    JSObject *p = JS_VALUE_GET_OBJ(val);
    // Mark the shape
    mark_func(rt, &p->shape->header);
    // Mark prototype
    if (p->shape->proto)
        mark_func(rt, &p->shape->proto->header);
    // Mark properties
    for (int i = 0; i < p->shape->prop_count; i++) {
        if (p->prop[i].u.value)
            JS_MarkValue(rt, p->prop[i].u.value, mark_func);
    }
    // Mark type-specific data
    // ...
}
```

### Go Rewrite Recommendation

```go
// package gc

// GC manages memory reclamation
type GC struct {
    rt           *Runtime
    phase        GCPhase
    markBits     []uint64
    finalizeQueue []finalizerTask
}

type GCPhase int
const (
    GCPhaseNone GCPhase = iota
    GCPhaseDecref
    GCPhaseRemoveWeak
    GCPhaseMark
    GCPhaseFinalize
)

func (gc *GC) Run() {
    gc.phase = GCPhaseMark
    gc.markAll()
    
    gc.phase = GCPhaseRemoveWeak
    gc.removeWeakRefs()
    
    gc.phase = GCPhaseFinalize
    gc.finalizeUnreachable()
    
    gc.phase = GCPhaseNone
}

func (gc *GC) markValue(v value.Value) {
    if !v.IsGCObject() {
        return
    }
    obj := v.GCObject()
    if obj.marked {
        return
    }
    obj.marked = true
    gc.markQueue = append(gc.markQueue, obj)
}
```

### Pitfalls
1. **Cycle detection**: Reference counting alone misses cycles
2. **Finalization order**: Must respect reference dependencies
3. **Weak refs**: Must be cleared before object is freed
4. **Incremental GC**: Not implemented (stop-the-world)

---

## Module 7: Parser

### Location
- `quickjs.c` lines 21376-27427 (types + implementation)

### Design Principle
- **Recursive descent parser** with operator precedence
- **Token-based** lexer with lookahead
- Single token of lookahead sufficient

### C Code Analysis

**Token Types:**
```c
typedef struct JSToken {
    int val;  // token type (TOK_xxx)
    const uint8_t *ptr;
    union {
        struct { JSValue str; int sep; } str;
        struct { JSValue val; } num;
        struct { JSAtom atom; BOOL has_escape; BOOL is_reserved; } ident;
        struct { JSValue body, flags; } regexp;
    } u;
} JSToken;
```

**Parser State:**
```c
typedef struct JSParseState {
    JSContext *ctx;
    const char *filename;
    JSToken token;
    BOOL got_lf;  // line feed before current token
    const uint8_t *buf_start;
    const uint8_t *buf_ptr;
    const uint8_t *buf_end;
    JSFunctionDef *cur_func;  // current function being parsed
} JSParseState;
```

**Expression Parsing (Pratt parser):**
```c
static __exception int js_parse_assign_expr(JSParseState *s) {
    return js_parse_assign_expr2(s, 0);
}

static __exception int js_parse_assign_expr2(JSParseState *s, int parse_flags) {
    int tok = js_parse_unary(s, parse_flags);
    // ... 
    switch (tok) {
    case '=':
        // assignment
    case '+=':
    case '-=':
        // compound assignment
    }
}

static __exception int js_parse_expr_binary(JSParseState *s, int level) {
    // Operator precedence levels
    static const uint8_t prec[] = { /* ... */ };
    for (;;) {
        int op = s->token.val;
        if (prec[op] < level) break;
        // parse right side
        js_parse_expr_binary(s, prec[op] + 1);
    }
}
```

### Go Rewrite Recommendation

```go
// package parser

type Token struct {
    Kind TokenKind
    Pos  Position
    Lit  []byte
    
    // Union fields
    NumberValue float64
    StringValue *stringValue
    AtomValue   atom.Atom
}

type TokenKind int

const (
    TokEOL TokenKind = iota
    TokError
    TokUndef
    TokNull
    TokTrue
    TokFalse
    TokNumber
    TokString
    TokIdent
    TokKeyword  // and many specific keywords
    // Operators
    TokAdd
    TokSub
    TokMul
    TokDiv
    // ... 100+ token types
)

type Parser struct {
    ctx    *Context
    input  []byte
    pos    int
    token  Token
    ahead  Token  // one token lookahead
    
    // Function being parsed
    curFunc *function.FunctionDef
}

func (p *Parser) parseExpression() (Node, error) {
    return p.parseAssignExpr()
}

func (p *Parser) parseAssignExpr() (Node, error) {
    lhs, err := p.parseUnaryExpr()
    if err != nil {
        return nil, err
    }
    
    switch p.token.Kind {
    case TokenAssign:
        p.next()
        rhs, err := p.parseAssignExpr()
        return &AssignNode{lhs, rhs}, err
    case TokenPlusEq, TokenMinusEq, ...:
        // compound assignment
    default:
        return lhs, nil
    }
}

func (p *Parser) parseBinaryExpr(minPrec int) (Node, error) {
    lhs, err := p.parseUnaryExpr()
    if err != nil {
        return nil, err
    }
    
    for {
        prec := precedence(p.token.Kind)
        if prec < minPrec {
            break
        }
        op := p.token.Kind
        p.next()
        rhs, err := p.parseBinaryExpr(prec + 1)
        if err != nil {
            return nil, err
        }
        lhs = &BinaryNode{Op: op, LHS: lhs, RHS: rhs}
    }
    return lhs, nil
}
```

### Pitfalls
1. **Automatic semicolon insertion**: Complex rules at line breaks
2. **Unicode escapes**: \uXXXX in identifiers
3. **Template literals**: Complex multi-part parsing
4. **Hashbang detection**: #! at start of script

---

## Module 8: Compiler

### Location
- `quickjs.c` lines 23878-43200

### Design Principle
- **Tree-walking compilation** from parse nodes
- **Bytecode emission** with label/relocation tables
- **Constant pooling** for strings, numbers, functions

### C Code Analysis

**Function Definition (compiler IR):**
```c
typedef struct JSFunctionDef {
    JSContext *ctx;
    struct JSFunctionDef *parent;
    
    // Variables
    JSVarDef *vars;
    int var_count;
    JSVarDef *args;
    int arg_count;
    int var_ref_count;
    
    // Scopes
    int scope_level;
    JSVarScope *scopes;
    int scope_count;
    
    // Bytecode output
    DynBuf byte_code;
    LabelSlot *label_slots;
    JumpSlot *jump_slots;
    
    // Constants
    JSValue *cpool;
    int cpool_count;
    
    // Closure
    JSClosureVar *closure_var;
    int closure_var_count;
} JSFunctionDef;
```

**Compilation Example:**
```c
static int js_emit_op(JSCompileState *s, int op) {
    dynbuf_putc(&s->fd->byte_code, op);
    s->last_opcode_pos = dynbuf_size(&s->fd->byte_code) - 1;
    return 0;
}

static int js_emit_u16(JSCompileState *s, int val) {
    uint8_t buf[2];
    write_u16(buf, val);
    dynbuf_put(&s->fd->byte_code, buf, 2);
    return 0;
}
```

### Go Rewrite Recommendation

```go
// package compiler

type Compiler struct {
    ctx     *Context
    current *function.FunctionDef
    consts  []value.Value
    labels  []Label
}

type Label struct {
    refCount int
    pos      int   // first phase
    pos2     int   // second phase
    addr     int   // final address
}

func (c *Compiler) compile(node Node) error {
    switch n := node.(type) {
    case *NumberLiteral:
        idx := c.addConstant(value.NewFloat64(n.Value))
        return c.emit(OpPushConst, idx)
    case *BinaryExpr:
        if err := c.compile(n.Left); err != nil {
            return err
        }
        if err := c.compile(n.Right); err != nil {
            return err
        }
        return c.emitOp(opForBinary(n.Op))
    // ... etc
    }
}

func (c *Compiler) emit(op Opcode, args ...interface{}) error {
    if err := c.writeByte(byte(op)); err != nil {
        return err
    }
    for _, arg := range args {
        if err := c.writeArg(arg); err != nil {
            return err
        }
    }
    return nil
}

func (c *Compiler) resolveLabels() {
    // Two-pass: first calculate sizes, then fix addresses
}
```

### Pitfalls
1. **Label resolution**: Two-pass needed for forward jumps
2. **Short vs long opcodes**: Size affects jump offsets
3. **Constant folding**: Already done? Or at compile time?
4. **Stack depth tracking**: Required for OP_CALL with variable args

---

## Module 9: Built-in Objects (Array, String, etc.)

### Location
- `quickjs.c` lines ~10000-20400

### Design Principle
- **C functions** exposed to JavaScript via `JS_NewCFunction*`
- **Prototype chain** for inheritance
- **Intrinsic objects** added per-context

### C Code Analysis

**Array Built-in:**
```c
static JSValue js_array_constructor(JSContext *ctx, JSValueConst new_target,
                                    int argc, JSValueConst *argv) {
    JSValue obj;
    if (argc == 0) {
        obj = JS_NewArray(ctx);
    } else if (argc == 1) {
        if (JS_IsNumber(argv[0])) {
            // Array(len) - sparse array
        } else {
            // Array(...items)
        }
    }
    // ...
}
```

**String Built-in:**
```c
static JSValue js_string_fromCharCode(JSContext *ctx, JSValueConst this_val,
                                       int argc, JSValueConst *argv) {
    // Convert codes to string
}
```

### Go Rewrite Recommendation

```go
// package builtin

// Register intrinsics for a context
func SetupIntrinsics(ctx *Context) error {
    // Object
    ctx.SetIntrinsic("Object", objectFunctions, objectPrototype)
    
    // Array
    ctx.SetIntrinsic("Array", arrayFunctions, arrayPrototype)
    ctx.SetIntrinsic("Array.prototype", arrayPrototype)
    
    // String
    ctx.SetIntrinsic("String", stringFunctions, stringPrototype)
    
    // etc.
    return nil
}

var arrayFunctions = []FunctionDef{
    {"isArray", 1, jsIsArray},
    {"from", 1, jsArrayFrom, FunctionMagic},
}

// Implementation uses VM helpers
func jsArrayFrom(ctx *Context, this value.Value, args []value.Value, magic int) (value.Value, error) {
    // Implementation
}
```

### Pitfalls
1. **Prototype pollution**: Carefully control property descriptors
2. **Species constructor**: Symbol.species for methods returning new instances
3. **TypedArray optimizations**: Shared memory, detachable buffers

---

## Module 10: Exception Handling

### Location
- `quickjs.c` lines 7233-7350

### Design Principle
- Exception stored in runtime current_exception
- Longjmp-style transfer (but not using setjmp)
- Each call site checks for exception

### C Code Analysis

```c
JSValue JS_Throw(JSContext *ctx, JSValue obj) {
    ctx->rt->current_exception = obj;
    return JS_EXCEPTION;
}

#define __exception __attribute__((returns_nonnull))

static __exception JSValue js_add(JSContext *ctx, JSValueConst op1, JSValueConst op2) {
    if (JS_IsInteger(op1) && JS_IsInteger(op2)) {
        // fast path
    }
    // fallback with exception check
    return JS_Add(ctx, op1, op2);
}
```

### Go Rewrite Recommendation

```go
// Exceptions use panic/recover internally, but as values at API boundary

func (ctx *Context) Throw(err *Error) value.Value {
    ctx.runtime.currentException = value.NewObject(err)
    return value.Exception
}

func (ctx *Context) NewError(kind ErrorKind, msg string) *Error {
    return &Error{
        kind: kind,
        msg:  msg,
    }
}

type ErrorKind int

const (
    ErrorSyntax ErrorKind = iota
    ErrorType
    ErrorReference
    ErrorRange
    ErrorURI
    ErrorEval
    ErrorInternal
)

// At each operation that can throw:
func add(ctx *Context, a, b value.Value) (value.Value, error) {
    ai, aok := a.TryInt()
    bi, bok := b.TryInt()
    if aok && bok {
        // fast path
        return value.NewInt(ai + bi), nil
    }
    // slower path with possible exception
    return nil, ctx.Throw(ctx.NewError(ErrorType, "cannot add"))
}
```

### Pitfalls
1. **Exception masking**: finally blocks must run even if exception
2. **Async exceptions**: Promises reject with exceptions
3. **Host exceptions**: C++ exceptions must not cross API boundary

---

## Module 11: Promise / Async

### Location
- `quickjs.c` lines 20450-21300, 29684-33200

### Design Principle
- Promise is an object with state (pending/fulfilled/rejected)
- Async functions compiled to state machine
- Job queue for microtask processing

### C Code Analysis

```c
typedef enum JSPromiseStateEnum {
    JS_PROMISE_PENDING,
    JS_PROMISE_FULFILLED,
    JS_PROMISE_REJECTED,
} JSPromiseStateEnum;
```

**Async Function State Machine:**
```c
typedef struct JSAsyncFunctionState {
    JSGCObjectHeader header;
    JSValue this_val;
    int argc;
    BOOL throw_flag;
    BOOL is_completed;
    JSValue resolving_funcs[2];  // resolve, reject
    JSStackFrame frame;
} JSAsyncFunctionState;
```

### Go Rewrite Recommendation

```go
// package async

type PromiseState int

const (
    PromisePending   PromiseState = iota
    PromiseFulfilled
    PromiseRejected
)

type Promise struct {
    header   gcObjectHeader
    state    PromiseState
    result   value.Value
    reactions []Reaction
    mu       sync.Mutex
}

type Reaction struct {
    promise    *Promise
    onFulfilled value.Value  // function
    onRejected  value.Value  // function
}

func (ctx *Context) NewPromise() (*Promise, value.Value, value.Value) {
    p := &Promise{}
    resolve := ctx.NewNativeFunction("resolve", func(this value.Value, args []value.Value) value.Value {
        p.fulfill(args[0])
        return value.Undefined
    })
    reject := ctx.NewNativeFunction("reject", func(this value.Value, args []value.Value) value.Value {
        p.reject(args[0])
        return value.Undefined
    })
    return p, value.NewObject(resolve), value.NewObject(reject)
}
```

### Pitfalls
1. **Unhandled rejection tracking**: Must call host callback
2. **Promise resolution**: Can be another promise (thenable unwrapping)
3. **Async iteration**: Complex state machine

---

## Module 12: Module System

### Location
- `quickjs.c` lines 2269, 29509-30600

### Design Principle
- **ES Modules**: Static import/export analysis
- **Dynamic import()**: Returns promise
- **Module records** track status

### C Code Analysis

```c
typedef enum {
    JS_MODULE_STATUS_UNLINKED,
    JS_MODULE_STATUS_LINKING,
    JS_MODULE_STATUS_LINKED,
    JS_MODULE_STATUS_EVALUATING,
    JS_MODULE_STATUS_EVALUATING_ASYNC,
    JS_MODULE_STATUS_EVALUATED,
} JSModuleStatus;

struct JSModuleDef {
    JSGCObjectHeader header;
    JSAtom module_name;
    
    JSReqModuleEntry *req_module_entries;
    JSExportEntry *export_entries;
    JSImportEntry *import_entries;
    
    JSValue module_ns;
    JSValue func_obj;
    JSModuleStatus status : 8;
    
    // For async modules
    int64_t async_evaluation_timestamp;
    JSValue promise;
};
```

### Go Rewrite Recommendation

```go
// package module

type ModuleStatus int

const (
    ModuleUnlinked ModuleStatus = iota
    ModuleLinking
    ModuleLinked
    ModuleEvaluating
    ModuleEvaluatingAsync
    ModuleEvaluated
)

type Module struct {
    header   gcObjectHeader
    name     atom.Atom
    status   ModuleStatus
    ns       *Object           // namespace object
    func_    *function.Function
    // Imports/exports
    imports  []ImportEntry
    exports  []ExportEntry
}

type ImportEntry struct {
    Module    *Module
    LocalName atom.Atom
    ImportName atom.Atom
}

type ExportEntry struct {
    LocalName atom.Atom
    ExportName atom.Atom
    VarRef    *VarRef
}
```

### Pitfalls
1. **Circular imports**: DFS with cycle detection
2. **Top-level await**: Can make module async
3. **Import attributes**: Using with import specifier

---

## Summary: Module Dependencies

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Module Dependency Graph                       │
└─────────────────────────────────────────────────────────────────────────┘

                    ┌──────────────┐
                    │   Runtime    │
                    │  (singleton) │
                    └──────┬───────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
          ▼                ▼                ▼
    ┌──────────┐    ┌───────────┐    ┌──────────┐
    │  Atom    │    │  Context  │    │   GC     │
    │  Table   │    │           │    │          │
    └──────────┘    └─────┬─────┘    └──────────┘
                          │
          ┌───────────────┼───────────────┐
          │               │               │
          ▼               ▼               ▼
    ┌──────────┐    ┌───────────┐    ┌──────────┐
    │  Object  │    │  String   │    │ Function │
    │  System  │    │           │    │ Objects  │
    └────┬─────┘    └───────────┘    └────┬─────┘
         │                                 │
         │         ┌───────────────────────┤
         │         │                       │
         ▼         ▼                       ▼
    ┌─────────────────────────────────────────┐
    │              Built-ins                   │
    │  (Array, String, Number, Promise, ...)   │
    └─────────────────────────────────────────┘
                          │
                          ▼
    ┌─────────────────────────────────────────┐
    │              Parser                      │
    │  (Token → AST-like → FunctionDef)        │
    └─────────────────────────────────────────┘
                          │
                          ▼
    ┌─────────────────────────────────────────┐
    │              Compiler                     │
    │  (FunctionDef → Bytecode + ConstPool)    │
    └─────────────────────────────────────────┘
                          │
                          ▼
    ┌─────────────────────────────────────────┐
    │           VM (execute_bytecode)           │
    │  (Stack frames, op dispatch, calls)       │
    └─────────────────────────────────────────┘

    ┌─────────────────────────────────────────┐
    │              Modules                      │
    │  (Import/export, loading, evaluation)      │
    └─────────────────────────────────────────┘
```
