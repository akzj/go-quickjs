# QuickJS 值模型 Go 实现建议

## 1. 设计选择

### 1.1 NaN Boxing vs Tagged Union

**C 原始方案**: NaN Boxing (64 位整数)
```
┌────────────────────────────────────────┐
│  uint64 (64 bits)                      │
├──────────────────────┬─────────────────┤
│  tag: int32         │  payload        │
│  JS_TAG_*           │  int/ptr/bool   │
└──────────────────────┴─────────────────┘
```

**Go 方案选择**: Tagged Union (推荐)

```go
// 值模型: 使用 interface{} 作为基础，或自定义结构体
type Value interface {
    getTag() Tag
}

// 立即数 (无堆分配)
type IntValue int64
type BoolValue bool
type UndefinedValue struct{}
type NullValue struct{}
type ExceptionValue struct{}

// 堆对象引用
type StringValue struct {
    v *String
}
type ObjectValue struct {
    v *Object
}
type BigIntValue struct {
    v *BigInt
}
```

### 1.2 为什么不用 NaN Boxing?

| 原因 | 说明 |
|------|------|
| Go 无位操作访问 | Go 不能直接操作浮点数的位表示来提取 tag |
| unsafe.Pointer 复杂 | 虽然可以用，但增加了复杂性 |
| GC 配合困难 | Go 的 GC 不理解自定义位编码 |
| 性能差异不大 | 现代 CPU 上 tagged union 同样高效 |

## 2. 核心类型定义

### 2.1 Tag 定义

```go
package quickjs

// Tag 表示值的类型
type Tag int32

const (
    // Heap 对象 (需要 GC 跟踪)
    TagBigInt           Tag = -9
    TagSymbol           Tag = -8
    TagString           Tag = -7
    TagStringRope       Tag = -6
    TagModule           Tag = -3  // 内部使用
    TagFunctionBytecode  Tag = -2  // 内部使用
    TagObject           Tag = -1
    
    // 立即数 (无需 GC)
    TagInt              Tag = 0
    TagBool             Tag = 1
    TagNull             Tag = 2
    TagUndefined        Tag = 3
    TagUninitialized    Tag = 4
    TagCatchOffset      Tag = 5
    TagException        Tag = 6
    TagShortBigInt      Tag = 7
    TagFloat64          Tag = 8
)

// hasRefCount returns true if this tag requires reference counting
func (t Tag) HasRefCount() bool {
    return int32(t) >= int32(TagBigInt)  // 负数 tag 需要
}
```

### 2.2 Value 接口

```go
// Value represents any JavaScript value
type Value interface {
    tag() Tag
    
    // Type checks
    IsInt() bool
    IsFloat64() bool
    IsBool() bool
    IsNull() bool
    IsUndefined() bool
    IsException() bool
    IsString() bool
    IsSymbol() bool
    IsObject() bool
    IsBigInt() bool
    IsNumber() bool
    
    // Get primitive values (panic if wrong type)
    Int32() int32
    Float64() float64
    Bool() bool
    String() *String
    Object() *Object
    BigInt() *BigInt
    
    // For internal use
    asRaw() uint64
}

//go:generate stringer -type=Tag -linecomment
```

### 2.3 立即数实现

```go
// Undefined represents JavaScript undefined
type UndefinedValue struct{}

func (UndefinedValue) tag() Tag            { return TagUndefined }
func (UndefinedValue) IsUndefined() bool   { return true }
func (UndefinedValue) IsNull() bool        { return false }
func (UndefinedValue) IsNumber() bool      { return false }
func (UndefinedValue) IsObject() bool     { return false }
func (UndefinedValue) IsString() bool     { return false }
func (UndefinedValue) IsBool() bool       { return false }
func (UndefinedValue) IsException() bool   { return false }
func (UndefinedValue) IsBigInt() bool     { return false }
func (UndefinedValue) IsSymbol() bool     { return false }
func (UndefinedValue) IsFloat64() bool    { return false }
func (UndefinedValue) asRaw() uint64       { return 0 }  // placeholder

var Undefined = UndefinedValue{}

// Null represents JavaScript null
type NullValue struct{}

func (NullValue) tag() Tag         { return TagNull }
func (NullValue) IsNull() bool     { return true }
func (NullValue) asRaw() uint64    { return 0 }

var Null = NullValue{}

// BoolValue represents a JavaScript boolean
type BoolValue bool

func (b BoolValue) tag() Tag     { return TagBool }
func (b BoolValue) Bool() bool   { return bool(b) }
func (b BoolValue) IsBool() bool { return true }
func (b BoolValue) asRaw() uint64 {
    if b {
        return 1
    }
    return 0
}

var True = BoolValue(true)
var False = BoolValue(false)

// IntValue represents a JavaScript integer
type IntValue int32

func (i IntValue) tag() Tag      { return TagInt }
func (i IntValue) Int32() int32 { return int32(i) }
func (i IntValue) IsInt() bool  { return true }
func (i IntValue) IsNumber() bool { return true }
func (i IntValue) asRaw() uint64 { return uint64(i) }
```

### 2.4 堆对象引用

```go
// ObjectValue wraps a heap-allocated JS object
type ObjectValue struct {
    v *Object
}

func (o ObjectValue) tag() Tag       { return TagObject }
func (o ObjectValue) Object() *Object { return o.v }
func (o ObjectValue) IsObject() bool { return true }
func (o ObjectValue) IsNull() bool    { return false }
func (o ObjectValue) IsNumber() bool  { return false }
func (o ObjectValue) asRaw() uint64   { return 0 }  // not used directly

// StringValue wraps a heap-allocated JS string
type StringValue struct {
    v *String
}

func (s StringValue) tag() Tag         { return TagString }
func (s StringValue) String() *String  { return s.v }
func (s StringValue) IsString() bool   { return true }
func (s StringValue) IsNull() bool    { return false }
func (s StringValue) IsNumber() bool  { return false }
```

### 2.5 Float64 值

```go
// Float64Value represents a heap-allocated JavaScript number
type Float64Value struct {
    v float64
}

func (f Float64Value) tag() Tag          { return TagFloat64 }
func (f Float64Value) Float64() float64 { return f.v }
func (f Float64Value) IsFloat64() bool   { return true }
func (f Float64Value) IsNumber() bool   { return true }
```

## 3. 工厂函数

```go
// NewInt creates an integer value, possibly optimizing small floats
func (ctx *Context) NewInt(i int32) Value {
    return IntValue(i)
}

func (ctx *Context) NewInt64(i int64) Value {
    if i >= math.MinInt32 && i <= math.MaxInt32 {
        return IntValue(int32(i))
    }
    return ctx.newFloat64(float64(i))
}

func (ctx *Context) NewFloat64(f float64) Value {
    // Check if it can be represented as int32
    if f >= float64(math.MinInt32) && f <= float64(math.MaxInt32) {
        if float64(int32(f)) == f {
            return IntValue(int32(f))
        }
    }
    // Need heap allocation for actual float
    return &FloatBox{value: f}
}

func (ctx *Context) NewBool(b bool) Value {
    if b {
        return True
    }
    return False
}

func (ctx *Context) NewString(s []byte) Value {
    str := ctx.rt.allocString(s)
    return StringValue{v: str}
}

func (ctx *Context) NewObject(class *JSClass) Value {
    obj := ctx.rt.allocObject(class)
    return ObjectValue{v: obj}
}
```

## 4. 类型判断优化

```go
// Fast path for common type checks
func (v Value) IsInt() bool {
    _, ok := v.(IntValue)
    return ok
}

func (v Value) IsNumber() bool {
    switch v.(type) {
    case IntValue, *FloatBox:
        return true
    }
    return false
}

func (v Value) IsObject() bool {
    _, ok := v.(ObjectValue)
    return ok
}
```

## 5. 引用计数设计

### 5.1 GC 对象基类

```go
// GCObject is the base for all heap-allocated JS values
type GCObject struct {
    refCount int32
    // Link for GC list management
    next *GCObject
    prev *GCObject
}

// AddRef increments the reference count
func (o *GCObject) AddRef() {
    atomic.AddInt32(&o.refCount, 1)
}

// Release decrements the reference count and returns true if freed
func (o *GCObject) Release(ctx *Context) bool {
    if atomic.AddInt32(&o.refCount, -1) == 0 {
        o.free(ctx)
        return true
    }
    return false
}

// free is called when refCount reaches 0
func (o *GCObject) free(ctx *Context) {
    // Subclass should override to free children
    ctx.runtime.freeObject(o)
}
```

### 5.2 JSObject 引用计数

```go
type Object struct {
    GCObject
    class    *JSClass
    shape    *Shape
    property []Property
    // ...
}

func (o *Object) free(ctx *Context) {
    // Free all property values
    for i := range o.property {
        ctx.FreeValue(propertyValue(o.property[i]))
    }
    // Free shape (may be shared)
    o.shape.Release(ctx)
    // Free class-specific data
    if o.class.Finalizer != nil {
        o.class.Finalizer(ctx.runtime, o)
    }
}
```

## 6. Value vs Go 类型

| JS 类型 | Go 类型 | 堆分配 |
|--------|---------|--------|
| integer | `IntValue` | 无 |
| boolean | `BoolValue` | 无 |
| null | `NullValue` | 无 |
| undefined | `UndefinedValue` | 无 |
| exception | `ExceptionValue` | 无 |
| string | `StringValue` + `*String` | 是 |
| number | `Float64Value` + `*FloatBox` | 是 |
| bigint | `BigIntValue` + `*BigInt` | 是 |
| symbol | `SymbolValue` + `*Symbol` | 是 |
| object | `ObjectValue` + `*Object` | 是 |

## 7. 关键差异: C vs Go

| 方面 | C 实现 | Go 实现 |
|------|--------|---------|
| 值表示 | 64 位整数 | interface{} / 自定义类型 |
| Float64 | NaN boxing | 堆分配 |
| 引用计数 | 显式 RC | 显式 RC + runtime GC |
| 字符串 | 特殊内存管理 | Go 原生字符串 + 额外结构 |
| GC 触发 | 手动 + 阈值 | runtime GC + 手动优化 |

## 8. 测试策略

### 8.1 值模型测试

```go
func TestValueTypes(t *testing.T) {
    ctx := NewContext()
    
    // Test integers
    v := ctx.NewInt(42)
    require.Equal(t, int32(42), v.Int32())
    require.True(t, v.IsInt())
    require.True(t, v.IsNumber())
    require.False(t, v.IsObject())
    
    // Test floats
    v = ctx.NewFloat64(3.14)
    require.True(t, v.IsFloat64())
    require.InDelta(t, 3.14, v.Float64(), 0.001)
    
    // Test small float optimization
    v = ctx.NewFloat64(42.0)
    require.True(t, v.IsInt())  // Should be optimized to int
    
    // Test booleans
    require.Same(t, True, ctx.NewBool(true))
    require.Same(t, False, ctx.NewBool(false))
    
    // Test null/undefined
    require.Same(t, Undefined, ctx.Undefined())
    require.Same(t, Null, ctx.Null())
}
```

### 8.2 类型转换测试

```go
func TestConversions(t *testing.T) {
    ctx := NewContext()
    
    // ToNumber
    tests := []struct {
        input    interface{}
        expected float64
        isNaN    bool
    }{
        {true, 1.0, false},
        {false, 0.0, false},
        {"42", 42.0, false},
        {"", 0.0, false},
        {nil, 0.0, false},
        {undefined, math.NaN(), true},
    }
    
    for _, tt := range tests {
        v := ctx.ValueOf(tt.input)
        n, err := v.ToFloat64()
        if tt.isNaN {
            require.True(t, math.IsNaN(n), "input: %v", tt.input)
        } else {
            require.Equal(t, tt.expected, n, "input: %v", tt.input)
        }
    }
}
```

## 9. 潜在陷阱

### 9.1 别名问题

C 中 JSValue 是值语义，复制不增加引用计数:
```c
JSValue v1 = ctx->global_obj;
JSValue v2 = v1;  // 复制指针，不增加 ref_count
```

Go 中必须明确:
```go
v1 := ctx.Global()      // v1 是值副本
v2 := v1                // v2 也是副本
// 但如果返回的是 *Object，需要手动 AddRef/Release
```

### 9.2 接口 nil 检查

```go
// 危险的: interface{} 可以是 (*ObjectValue)(nil)
var v Value = (*ObjectValue)(nil)
if v != nil {  // true! 接口本身不是 nil
    // ...
}

// 安全的: 检查内部指针
if v, ok := v.(ObjectValue); ok && v.v != nil {
    // ...
}
```

### 9.3 Float64 优化

C 的 `JS_NewFloat64` 会测试是否可以用 int 表示:
```c
// Go 也要这样做
func (ctx *Context) NewFloat64(f float64) Value {
    if f == float64(int32(f)) {
        return IntValue(int32(f))
    }
    return &FloatBox{value: f}
}
```

## 10. 代码结构建议

```
quickjs/
├── value.go          # Value 接口, 基础实现
├── value_int.go      # IntValue, BoolValue
├── value_float.go    # Float64Value, FloatBox
├── value_string.go   # StringValue, String
├── value_object.go   # ObjectValue, Object
├── value_bigint.go   # BigIntValue, BigInt
├── gc_object.go      # GCObject 基类
├── conversion.go     # ToInt, ToFloat64, ToString 等
└── value_test.go     # 测试
```