# Go 内置对象实现指南

## 概述

本指南提供将 QuickJS C 内置对象移植到 Go 的最佳实践。

## 设计原则

### 1. 类型安全优先

```go
// ❌ 避免: 使用 interface{} 存储所有值
var anyValue interface{}

// ✅ 推荐: 使用强类型的值联合体
type Value struct {
    tag    ValueTag
    union  uint64
}
```

### 2. 值语义 vs 引用语义

```go
// 值类型 (immutable)
type NumberValue float64
type StringValue string

// 引用类型 (mutable)
type Object struct {
    mu       sync.RWMutex
    classID  ClassID
    proto    *Object
    props    map[string]*Property
}

func (v NumberValue) Add(other Value) Value {
    return NewNumberValue(float64(v) + other.Float64())
}
```

### 3. 接口抽象

```go
// 所有可调用对象实现 Callable
type Callable interface {
    Call(ctx *Context, this Value, args []Value) (Value, error)
}

// 所有可迭代对象实现 Iterable
type Iterable interface {
    GetIterator(ctx *Context) (Iterator, error)
}

// 所有可索引对象实现 Indexable
type Indexable interface {
    GetIndex(ctx *Context, idx uint32) (Value, bool)
    SetIndex(ctx *Context, idx uint32, val Value) error
}
```

## 内置对象实现模板

### 基础结构

```go
// builtin.go
package quickjs

// ClassID 是内置类的标识符
type ClassID uint16

const (
    ClassObject ClassID = iota + 1
    ClassFunction
    ClassArray
    ClassString
    ClassNumber
    ClassBoolean
    ClassSymbol
    ClassBigInt
    ClassDate
    ClassRegExp
    ClassError
    ClassMap
    ClassSet
    ClassWeakMap
    ClassWeakSet
    ClassWeakRef
    ClassArrayBuffer
    ClassTypedArray
    // ... 更多类
)

// ClassDefinition 定义类的元信息
type ClassDefinition struct {
    ClassID   ClassID
    ClassName string
    
    // 构造函数 (可选)
    Constructor func(ctx *Context, args []Value) (Value, error)
    
    // 原型方法
    PrototypeMethods []MethodDefinition
    
    // 静态方法
    StaticMethods []MethodDefinition
    
    // 内省 (Introspection)
    Call        func(ctx *Context, this Value, args []Value) (Value, error)
    HasInstance func(ctx *Context, this Value, other Value) bool
}

// MethodDefinition 定义方法
type MethodDefinition struct {
    Name       string
    Length     int
    Method     func(ctx *Context, this Value, args []Value) Value
    Attributes PropertyFlags
}
```

### 内置对象基类

```go
// base.go

// ObjectPrototype 是所有对象的基类
type ObjectPrototype struct {
    classID   ClassID
    prototype *ObjectPrototype
    props     map[string]*Property
    symbols   map[*Symbol]*Property
}

func (p *ObjectPrototype) Get(ctx *Context, key string) (Value, error) {
    if prop, ok := p.props[key]; ok {
        if prop.Getter != nil {
            return prop.Getter.Call(ctx, ValueOf(p), nil)
        }
        return prop.Value, nil
    }
    
    if p.prototype != nil {
        return p.prototype.Get(ctx, key)
    }
    
    return UndefinedValue, nil
}

func (p *ObjectPrototype) Set(ctx *Context, key string, value Value) error {
    if prop, ok := p.props[key]; ok {
        if prop.IsReadOnly() {
            return ctx.ThrowTypeError("property is read-only")
        }
        if prop.IsAccessor() {
            return prop.Setter.Call(ctx, ValueOf(p), []Value{value})
        }
        prop.Value = value
        return nil
    }
    
    p.props[key] = &Property{
        Value: value,
        Flags: PropWritable | PropEnumerable | PropConfigurable,
    }
    return nil
}

// toString 返回类的字符串表示
func (p *ObjectPrototype) toString() string {
    return "[object " + p.classID.Name() + "]"
}
```

## 具体实现示例

### Number 实现

```go
// number.go

type NumberPrototype struct {
    ObjectPrototype
    value float64
}

var numberMethods = []MethodDefinition{
    {"toString", 1, numberToString, 0},
    {"valueOf", 0, numberValueOf, 0},
    {"toFixed", 1, numberToFixed, 0},
    {"toExponential", 1, numberToExponential, 0},
    {"toPrecision", 1, numberToPrecision, 0},
}

func numberToString(ctx *Context, this Value, args []Value) Value {
    radix := 10
    if len(args) > 0 {
        radix = int(args[0].ToUint32())
    }
    if radix < 2 || radix > 36 {
        return ctx.ThrowRangeError("radix must be between 2 and 36")
    }
    
    n := this.ToFloat64()
    if n == 0 && math.IsInf(n, 0) == false && math.IsNaN(n) == false {
        return NewStringValue("0")
    }
    
    if math.IsInf(n, 0) {
        if n > 0 {
            return NewStringValue("Infinity")
        }
        return NewStringValue("-Infinity")
    }
    
    if math.IsNaN(n) {
        return NewStringValue("NaN")
    }
    
    return NewStringValue(strconv.FormatFloat(n, 'f', -1, 64))
}

func numberValueOf(ctx *Context, this Value, args []Value) Value {
    return NewNumberValue(this.ToFloat64())
}

func numberToFixed(ctx *Context, this Value, args []Value) Value {
    digits := 0
    if len(args) > 0 {
        digits = int(args[0].ToUint32())
    }
    if digits < 0 || digits > 100 {
        return ctx.ThrowRangeError("digits must be between 0 and 100")
    }
    
    n := this.ToFloat64()
    format := fmt.Sprintf("%%.%df", digits)
    return NewStringValue(fmt.Sprintf(format, n))
}

// Number 构造函数
func NumberConstructor(ctx *Context, args []Value) Value {
    if len(args) == 0 {
        return NewNumberValue(0)
    }
    return NewNumberValue(args[0].ToFloat64())
}

// 初始化 Number 对象
func initNumber(ctx *Context) error {
    // 创建 Number.prototype
    numProto := &NumberPrototype{
        ObjectPrototype: ObjectPrototype{
            classID: ClassNumber,
        },
    }
    numProto.props = map[string]*Property{
        "constructor": {Value: ctx.NumberCtor, Flags: PropNonWritable},
        "toString":    {Value: NewNativeFunction(numberToString, 1), Flags: PropNonEnumerable},
        "valueOf":    {Value: NewNativeFunction(numberValueOf, 0), Flags: PropNonEnumerable},
        "toFixed":    {Value: NewNativeFunction(numberToFixed, 1), Flags: PropNonEnumerable},
    }
    
    // 设置 [Symbol.toStringTag]
    numProto.symbols[SymToStringTag] = &Property{
        Value:  NewStringValue("Number"),
        Flags: PropNonConfigurable | PropNonEnumerable,
    }
    
    // 创建 Number 构造函数
    ctx.NumberCtor = NewNativeFunction(NumberConstructor, 1)
    ctx.NumberCtor.SetPrototype(ValueOf(numProto))
    ctx.NumberCtor.DefineProperty("prototype", ValueOf(numProto), PropReadOnly)
    ctx.NumberCtor.DefineProperty("MAX_VALUE", NewNumberValue(math.MaxFloat64), PropReadOnly)
    ctx.NumberCtor.DefineProperty("MIN_VALUE", NewNumberValue(math.SmallestNonzeroFloat64), PropReadOnly)
    ctx.NumberCtor.DefineProperty("NaN", NewNumberValue(math.NaN()), PropReadOnly)
    ctx.NumberCtor.DefineProperty("POSITIVE_INFINITY", NewNumberValue(math.Inf(1)), PropReadOnly)
    ctx.NumberCtor.DefineProperty("NEGATIVE_INFINITY", NewNumberValue(math.Inf(-1)), PropReadOnly)
    
    // 注册到全局对象
    ctx.GlobalObject.DefineProperty("Number", ValueOf(ctx.NumberCtor), PropDefault)
    
    return nil
}
```

### Array 实现

```go
// array.go

type ArrayPrototype struct {
    ObjectPrototype
    length uint32
    data   []Value
}

func (a *ArrayPrototype) GetLength() uint32 {
    return a.length
}

func (a *ArrayPrototype) SetLength(length uint32) {
    if length < a.length {
        for i := length; i < a.length; i++ {
            a.data[i].Free()
        }
        a.data = a.data[:length]
    } else if length > a.length {
        if uint32(cap(a.data)) < length {
            newData := make([]Value, length, length*2)
            copy(newData, a.data)
            a.data = newData
        } else {
            a.data = a.data[:length]
        }
    }
    a.length = length
    // 更新 length 属性
    a.props["length"] = &Property{
        Value:  NewUint32Value(length),
        Flags:  PropWritable | PropNonEnumerable,
    }
}

func (a *ArrayPrototype) GetIndex(idx uint32) (Value, bool) {
    if idx < a.length {
        return a.data[idx], true
    }
    return UndefinedValue, false
}

func (a *ArrayPrototype) SetIndex(idx uint32, val Value) {
    if idx >= a.length {
        a.SetLength(idx + 1)
    }
    a.data[idx] = val
}

// Array 方法
func arrayPush(ctx *Context, this Value, args []Value) Value {
    arr := this.ToObject().(*ArrayPrototype)
    for _, arg := range args {
        arr.SetIndex(arr.GetLength(), arg)
    }
    return NewUint32Value(arr.GetLength())
}

func arrayPop(ctx *Context, this Value, args []Value) Value {
    arr := this.ToObject().(*ArrayPrototype)
    length := arr.GetLength()
    if length == 0 {
        return UndefinedValue
    }
    length--
    val := arr.data[length]
    arr.SetLength(length)
    return val
}

func arrayMap(ctx *Context, this Value, args []Value) Value {
    if len(args) == 0 {
        return ctx.ThrowTypeError("callback is required")
    }
    
    callback := args[0]
    T := UndefinedValue
    if len(args) > 1 {
        T = args[1]
    }
    
    arr := this.ToObject().(*ArrayPrototype)
    result := NewArray()
    result.SetLength(arr.GetLength())
    
    length := arr.GetLength()
    for i := uint32(0); i < length; i++ {
        if val, ok := arr.GetIndex(i); ok {
            mapped := callback.Call(ctx, T, []Value{val, NewUint32Value(i), this.Dup()})
            result.SetIndex(i, mapped)
        }
    }
    
    return ValueOf(result)
}

func arrayIndexOf(ctx *Context, this Value, args []Value) Value {
    s := ctx.ToString(this)
    search := ""
    if len(args) > 0 {
        search = ctx.ToString(args[0]).String()
    }
    
    pos := 0
    if len(args) > 1 {
        pos = int(args[1].ToInteger())
        if pos < 0 {
            pos = 0
        }
    }
    
    sStr := s.String()
    idx := strings.Index(sStr[pos:], search)
    if idx < 0 {
        return NewIntValue(-1)
    }
    return NewIntValue(int64(pos + idx))
}

// 初始化 Array
func initArray(ctx *Context) error {
    // Array.prototype
    arrProto := &ArrayPrototype{
        ObjectPrototype: ObjectPrototype{
            classID: ClassArray,
        },
        data: make([]Value, 0, 16),
    }
    
    // 设置方法
    arrProto.props["push"] = NewNativeFunctionProp(arrayPush, 1)
    arrProto.props["pop"] = NewNativeFunctionProp(arrayPop, 0)
    arrProto.props["map"] = NewNativeFunctionProp(arrayMap, 1)
    arrProto.props["indexOf"] = NewNativeFunctionProp(arrayIndexOf, 1)
    // ... 更多方法
    
    // Array constructor
    ctx.ArrayCtor = NewNativeFunction(func(ctx *Context, args []Value) Value {
        if len(args) == 0 {
            return ValueOf(NewArray())
        }
        if len(args) == 1 && args[0].IsNumber() {
            length := args[0].ToUint32()
            arr := NewArray()
            arr.SetLength(length)
            return ValueOf(arr)
        }
        arr := NewArray()
        for i, arg := range args {
            arr.SetIndex(uint32(i), arg)
        }
        return ValueOf(arr)
    }, 1)
    
    // 静态方法
    ctx.ArrayCtor.DefineProperty("isArray", NewNativeFunction(func(ctx *Context, args []Value) Value {
        if len(args) == 0 {
            return FalseValue
        }
        return NewBoolValue(args[0].IsArray())
    }, 1), PropDefault)
    
    ctx.ArrayCtor.SetPrototype(ValueOf(arrProto))
    
    // 注册到全局
    ctx.GlobalObject.DefineProperty("Array", ValueOf(ctx.ArrayCtor), PropDefault)
    
    return nil
}
```

## 属性标志

```go
type PropertyFlags uint32

const (
    PropNone PropertyFlags = 0
    
    // 基本标志
    PropWritable     PropertyFlags = 1 << 0
    PropEnumerable   PropertyFlags = 1 << 1
    PropConfigurable PropertyFlags = 1 << 2
    
    // 访问器
    PropGetter       PropertyFlags = 1 << 3
    PropSetter       PropertyFlags = 1 << 4
    
    // 常用组合
    PropDefault      PropertyFlags = PropWritable | PropEnumerable | PropConfigurable
    PropReadOnly     PropertyFlags = PropEnumerable | PropConfigurable
    PropNonWritable  PropertyFlags = PropEnumerable | PropConfigurable
    PropNonEnumerable PropertyFlags = PropConfigurable
    
    // 特殊
    PropLength       PropertyFlags = 1 << 5
)
```

## 陷阱规避清单

1. **全局对象初始化顺序**:
   - Object.prototype 必须首先初始化
   - Function.prototype 需要 Object.prototype
   - 其他构造函数需要 Function.prototype

2. **constructor 属性**:
   ```go
   // 每个原型必须指向构造函数
   proto.props["constructor"] = &Property{Value: ctor}
   ```

3. **Symbol.toStringTag**:
   ```go
   // ES6+ 要求
   proto.symbols[SymToStringTag] = &Property{Value: NewStringValue("ClassName")}
   ```

4. **length 属性**:
   ```go
   // 函数对象的 length 是形参数量
   funcObj.DefineProperty("length", NewUint32Value(funcDef.ArgCount), PropReadOnly)
   ```

5. **NaN/±Infinity**:
   ```go
   // NaN 比较必须使用 math.IsNaN
   if math.IsNaN(float64(n)) { ... }
   ```

6. **BigInt 安全**:
   ```go
   // BigInt 操作需要溢出检查
   if result > math.MaxInt64 || result < math.MinInt64 {
       return ctx.ThrowRangeError("BigInt overflow")
   }
   ```

7. **TypedArray 边界**:
   ```go
   // 索引必须边界检查
   if idx < 0 || idx >= byteLength/bytesPerElement {
       return ctx.ThrowRangeError("index out of bounds")
   }
   ```

8. **Promise 微任务**:
   ```go
   // Promise 必须异步执行
   ctx.EnqueueMicrotask(func() {
       // 执行 resolve/reject 回调
   })
   ```
