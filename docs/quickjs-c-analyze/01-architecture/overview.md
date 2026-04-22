# QuickJS Architecture Overview

## Executive Summary

QuickJS is a compact JavaScript engine (~60K lines of C) featuring:
- **Reference-counted GC** with mark-and-sweep for cycles
- **NaN-boxing** for efficient JSValue representation (on 64-bit platforms)
- **Single-file monolithic design** — all code in `quickjs.c`
- **Register-based VM** with ~150 opcodes
- **Incremental parser** with operator precedence descent

## Core Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              JSRuntime (Singleton)                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌──────────────────┐  │
│  │ Atom Table  │  │ Class Table │  │ Shape Hash  │  │  GC Object List  │  │
│  │ (strings)   │  │ (40+ types) │  │ (property   │  │  (ref-counted    │  │
│  │             │  │             │  │  sharing)   │  │   objects)       │  │
│  └─────────────┘  └─────────────┘  └─────────────┘  └──────────────────┘  │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         JSContext (Per-thread)                         │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │  │
│  │  │ Global Obj  │  │  Prototype  │  │  Intrinsics │  │   Job      │  │  │
│  │  │             │  │   Cache      │  │  (Array,    │  │   Queue     │  │  │
│  │  │             │  │  (shapes)    │  │   String..) │  │             │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └────────────┘  │  │
│  │                                                                       │  │
│  │  ┌───────────────────────────────────────────────────────────────┐    │  │
│  │  │                    JSStackFrame (Call Stack)                   │    │  │
│  │  │   prev_frame → → → → → → → → → → → → → → → → → → → → → NULL  │    │  │
│  │  └───────────────────────────────────────────────────────────────┘    │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘

                           ┌─────────────────────┐
                           │   JSValue (64-bit)   │
                           │  ┌─────────────────┐ │
                           │  │  Tag (32 bits)  │ │
                           │  ├─────────────────┤ │
                           │  │  Payload (32/64) │ │
                           │  └─────────────────┘ │
                           └─────────────────────┘
                                    │
           ┌────────────────────────┼────────────────────────┐
           │                        │                        │
    ┌──────▼──────┐          ┌──────▼──────┐          ┌──────▼──────┐
    │  Primitive  │          │   Object    │          │   Function  │
    │  (immediate)│          │  (pointer)   │          │  (pointer)  │
    │             │          │              │          │             │
    │ - int       │          │ - JSObject*  │          │ - Bytecode  │
    │ - bool      │          │ - JSString*  │          │ - C Func    │
    │ - null      │          │ - JSBigInt*  │          │ - Bound     │
    │ - undefined │          │ - JSShape*   │          │             │
    └─────────────┘          └──────────────┘          └─────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                            Bytecode Pipeline                                 │
│                                                                             │
│  Source Code ──► Lexer ──► Token Stream ──► Parser ──► AST-like ──►        │
│                                                      Compiler                │
│                                                           │                 │
│                                                           ▼                 │
│                                                    Bytecode +               │
│                                                    Constant Pool            │
│                                                           │                 │
│                                                           ▼                 │
│                                                        VM Exec               │
│                                                    ┌─────────────┐          │
│                                                    │   Stack     │          │
│                                                    │   Registers │          │
│                                                    │   (frames)  │          │
│                                                    └─────────────┘          │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Key Statistics

| Metric | Value |
|--------|-------|
| Total Lines (quickjs.c) | 59,903 |
| Total Lines (quickjs.h) | 1,171 |
| Opcodes | ~150 |
| JSClass types | ~50 |
| Static functions | ~1,200 |
| Public API functions | ~200 |

## Data Flow

### 1. Runtime Initialization
```
JS_NewRuntime() → JS_NewContext() → JS_AddIntrinsicBaseObjects()
                              ↓
                    Global Object Created
                              ↓
                    Prototype Chain Built
                              ↓
                    Intrinsic Objects Added
```

### 2. Code Execution (eval/module)
```
JS_Eval(ctx, source, ...)
  → js_parse_program()      // Lexer + Parser
  → js_compile_function()    // AST → Bytecode
  → JS_EvalFunction()       // Execute bytecode
      → JS_CallInternal()
          → VM Loop (execute_bytecode)
```

### 3. Property Access
```
JS_GetProperty(ctx, obj, atom)
  → JS_GetPropertyInternal()
      → Check fast_array path
      → Check exotic handlers
      → Search shape chain (prototype traversal)
          → Return property value or undefined
```

### 4. Function Call
```
JS_Call(ctx, func, this, argc, argv)
  → JS_CallInternal()
      → Identify function type:
          ├── JS_CLASS_C_FUNCTION → js_call_c_function()
          ├── JS_CLASS_BYTECODE_FUNCTION → execute_bytecode()
          ├── JS_CLASS_BOUND_FUNCTION → js_call_bound_function()
          └── JS_CLASS_C_FUNCTION_DATA → js_call_c_function_data()
```

## Memory Layout

### JSValue (NaN-boxing on 64-bit)
```
┌────────────────────────────────┬────────────────────────┐
│  Tag (32 bits)                 │  Payload (32 bits)    │
├────────────────────────────────┼────────────────────────┤
│  0xFFFFFFFF = NaN box prefix   │                        │
│  ──────────────────────────────  │  ─────────────────────│
│  JS_TAG_INT (0):               │  int32 value          │
│  JS_TAG_BOOL (1):              │  0 or 1               │
│  JS_TAG_NULL (2):              │  0                    │
│  JS_TAG_UNDEFINED (3):         │  0                    │
│  JS_TAG_OBJECT (-1):          │  Object* pointer      │
│  JS_TAG_STRING (-7):          │  String* pointer      │
│  JS_TAG_FLOAT64 (8+):         │  double in payload    │
└────────────────────────────────┴────────────────────────┘
```

### GC Object Hierarchy
```
JSGCObjectHeader (base)
  ├── ref_count: int
  ├── gc_obj_type: enum
  ├── mark: bool
  └── link: list_head

Derived Objects:
  ├── JSObject (class_id determines variant union)
  ├── JSFunctionBytecode
  ├── JSShape (property hash optimization)
  ├── JSVarRef (closure variable references)
  ├── JSContext
  └── JSModuleDef
```

## Module Boundaries (as identified by grep)

### Category 1: Value Operations (~line 1191-1250)
- `JS_ToPrimitiveFree`, `JS_ToStringFree`, `JS_ToBoolFree`
- `JS_ToInt32Free`, `JS_ToFloat64Free`
- Type coercion and conversion

### Category 2: Object System (~line 1216-1336)
- `JS_NewObject*`, `JS_GetProperty*`, `JS_SetProperty*`
- `JS_GetOwnProperty*`, `JS_CreateProperty`
- Shape-based property optimization

### Category 3: GC/Memory (~line 2300-3300, 6410-7232)
- `JS_MarkContext`, `JS_RunGCInternal`
- `js_*_finalizer`, `js_*_mark` for all object types
- Reference counting helpers

### Category 4: Function Calls (~line 1100-1115)
- `JS_CallInternal`, `JS_CallConstructorInternal`
- `js_call_c_function`, `js_call_bound_function`

### Category 5: String/Atom Management (~line 2564-3400)
- `JS_NewAtom*`, `JS_DupAtom`, `JS_FreeAtom`
- `JS_AtomToValue`, `JS_AtomGetStr`
- String interning via atom table

### Category 6: Parser (~line 21764-27427)
- `js_parse_*` functions (expression, function, class, etc.)
- `JSToken` lexer, `JSParseState` parser state
- Operator precedence descent parsing

### Category 7: Compiler (~line 23878-43200)
- `js_compile_*` functions
- `JSFunctionDef` intermediate representation
- Bytecode emission with label resolution

### Category 8: VM Execution (~line 33283-40000)
- `execute_instructions` main loop
- Opcode handlers (switch-case based)
- Stack frame management

### Category 9: Built-in Objects (~line 10000-20400)
- Array, String, Number, Boolean, Symbol
- Date, RegExp, Promise, Map, Set
- TypedArrays, ArrayBuffer

### Category 10: Async/Coroutine (~line 20450-21300)
- Generator functions
- Async functions
- Promise handling

### Category 11: Module System (~line 2269-29600)
- `JSModuleDef` structure
- Import/export resolution
- Dynamic import

## Public API Entry Points (quickjs.h)

### Runtime Management
```c
JSRuntime *JS_NewRuntime(void);
void JS_FreeRuntime(JSRuntime *rt);
void *JS_GetRuntimeOpaque(JSRuntime *rt);
void JS_SetRuntimeOpaque(JSRuntime *rt, void *opaque);
```

### Context Management
```c
JSContext *JS_NewContext(JSRuntime *rt);
void JS_FreeContext(JSContext *s);
JSContext *JS_DupContext(JSContext *ctx);
JSRuntime *JS_GetRuntime(JSContext *ctx);
```

### Evaluation
```c
JSValue JS_Eval(JSContext *ctx, const char *input, size_t input_len,
                const char *filename, int eval_flags);
JSValue JS_EvalThis(JSContext *ctx, JSValueConst this_obj,
                    const char *input, size_t input_len,
                    const char *filename, int eval_flags);
JSValue JS_Call(JSContext *ctx, JSValueConst func_obj, JSValueConst this_obj,
                int argc, JSValueConst *argv);
```

### Value Operations
```c
JSValue JS_NewString(JSContext *ctx, const char *str);
JSValue JS_NewObject(JSContext *ctx);
JSValue JS_NewArray(JSContext *ctx);
JSValue JS_GetProperty(JSContext *ctx, JSValueConst this_obj, JSAtom prop);
int JS_SetProperty(JSContext *ctx, JSValueConst this_obj, JSAtom prop, JSValue val);
```

### Type Conversions
```c
int JS_ToBool(JSContext *ctx, JSValueConst val);
int JS_ToInt32(JSContext *ctx, int32_t *pres, JSValueConst val);
int JS_ToFloat64(JSContext *ctx, double *pres, JSValueConst val);
JSValue JS_ToString(JSContext *ctx, JSValueConst val);
```

## Critical Design Patterns

### 1. Tagged Union for JSValue
All JavaScript values are 64-bit with a tag distinguishing:
- Immediate values (int, bool, null, undefined)
- Object references (pointer + negative tag)
- Float64 (NaN-boxed)

### 2. Shape-based Property Optimization
Objects sharing the same property structure share a `JSShape`:
- Hash table of `JSShape` objects
- Properties stored in `JSShapeProperty` array
- Enables fast shape comparison and prototype chain traversal

### 3. Atom Interning
Strings used as property names are interned:
- Atom table with hash-based lookup
- Reduces string comparison to integer comparison
- Symbols have special atom types

### 4. Reference Counting + GC
- Immediate RC for most objects
- Mark-and-sweep for cycle detection
- Weak references for WeakMap/WeakSet

### 5. Closure Variable References
Variables captured by closures become `JSVarRef`:
- Point either to stack (during execution) or heap (after closure)
- Detachment mechanism for stack frame cleanup
