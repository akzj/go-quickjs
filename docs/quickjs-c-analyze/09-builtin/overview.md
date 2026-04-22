# QuickJS 内置对象概览

## 内置对象分类

### 1. 基础对象 (Primitive Wrappers)

| 对象 | 类 ID | 行号 | 内部表示 |
|------|-------|------|----------|
| Number | JS_CLASS_NUMBER | 1555 | double |
| String | JS_CLASS_STRING | 1556 | JSString* |
| Boolean | JS_CLASS_BOOLEAN | 1557 | BOOL |
| Symbol | JS_CLASS_SYMBOL | 1558 | JSAtom |
| BigInt | JS_CLASS_BIG_INT | 1585 | JSBigInt* |

### 2. 复合对象

| 对象 | 类 ID | 行号 | 备注 |
|------|-------|------|------|
| Object | JS_CLASS_OBJECT | 1533 | 属性哈希表 |
| Array | JS_CLASS_ARRAY | 1535 | 连续存储 |
| Function | JS_CLASS_BYTECODE_FUNCTION | 1539 | 字节码容器 |
| Date | JS_CLASS_DATE | 1561 | 双精度数 |
| RegExp | JS_CLASS_REGEXP | 1562 | 正则表达式 |

### 3. 集合对象

| 对象 | 类 ID | 行号 |
|------|-------|------|
| ArrayBuffer | JS_CLASS_ARRAY_BUFFER | 1576 |
| Map | JS_CLASS_MAP | 1579 |
| Set | JS_CLASS_SET | 1580 |
| WeakMap | JS_CLASS_WEAK_MAP | 1581 |
| WeakSet | JS_CLASS_WEAK_SET | 1582 |
| WeakRef | JS_CLASS_WEAK_REF | 1583 |

### 4. 结构化数据

| 对象 | 类 ID | 行号 |
|------|-------|------|
| JSON | - | 49710 (JS_AddIntrinsicJSON) |
| Promise | JS_CLASS_PROMISE | 1594 |
| Proxy | JS_CLASS_PROXY | 1603 |

### 5. 类型化数组

| 对象 | 类 ID | 行号 |
|------|-------|------|
| Uint8Array | JS_CLASS_UINT8_ARRAY | 1570 |
| Int8Array | JS_CLASS_INT8_ARRAY | 1571 |
| Uint16Array | JS_CLASS_UINT16_ARRAY | 1572 |
| ... | ... | ... |
| BigInt64Array | JS_CLASS_BIG_INT64_ARRAY | 1578 |

## 内置对象初始化顺序

```
JS_AddIntrinsicBasicObjects (行 55685)
├── Object.prototype
├── Function.prototype
├── Global Object
├── Error (及其子类)
└── Array (及其静态方法)

JS_AddIntrinsicBaseObjects (行 55817)
├── Object
├── Function
├── Iterator
├── Number
├── Boolean
├── String
├── Symbol
├── BigInt
├── Math
└── Date

JS_AddIntrinsicJSON (行 49710)
└── JSON

JS_AddIntrinsicMapSet (行 52593)
├── Map
├── Set
├── WeakMap
└── WeakSet

JS_AddIntrinsicProxy (行 50840)
└── Proxy

JS_AddIntrinsicTypedArrays (行 52600+)
└── TypedArray 家族

JS_AddIntrinsicPromise (行 52700+)
└── Promise, AsyncFunction
```

## 对象属性操作 API

### 属性获取 (行 7805)

```c
JSValue JS_GetPropertyInternal(JSContext *ctx, JSValueConst obj, 
                               JSAtom prop, JSValueConst receiver,
                               BOOL throw_ref_error);
```

查找顺序:
1. 自身属性
2. 原型链
3. Symbol.hasInstance (for class check)
4. Proxy handler
5. 抛出 ReferenceError

### 属性设置 (行 9258)

```c
int JS_SetPropertyInternal(JSContext *ctx, JSValueConst obj, JSAtom prop,
                           JSValue val, int flags);
```

标志位:
- `JS_PROP_UNDEFINED_WRITABLE`: 可写
- `JS_PROP_ENUMERABLE`: 可枚举
- `JS_PROP_CONFIGURABLE`: 可配置
- `JS_PROP_GETSET`: 访问器属性
- `JS_PROP_C_W_E`: 可配置 + 可写 + 可枚举

### 属性描述符

```c
typedef struct JSPropertyDescriptor {
    int flags;
    JSValue value;
    JSValue getter;
    JSValue setter;
} JSPropertyDescriptor;
```

## Go 实现建议

### 1. 内置类注册

```go
type ClassID uint32

const (
    ClassObject ClassID = iota + 1
    ClassFunction
    ClassArray
    ClassNumber
    ClassString
    ClassBoolean
    ClassSymbol
    ClassBigInt
    ClassDate
    ClassRegExp
    // ... 更多类
)

type ClassDefinition struct {
    ClassID   ClassID
    ClassName string
    Finalizer func(rt *Runtime, val Value)
    GCMark    func(rt *Runtime, val Value, mark func(Value))
    Call      func(ctx *Context, this Value, args []Value) (Value, error)
}

func (rt *Runtime) RegisterClass(def ClassDefinition) error {
    if def.ClassID == 0 || int(def.ClassID) >= len(rt.classArray) {
        return errors.New("invalid class ID")
    }
    rt.classArray[def.ClassID] = def
    return nil
}
```

### 2. 内置构造函数工厂

```go
type CFunction struct {
    Call func(ctx *Context, this Value, args []Value) Value
    Magic int
}

type FunctionDefinition struct {
    Name       string
    Length     int
    Call       func(ctx *Context, this Value, args []Value) Value
    Magic      int
    Prototype  []FunctionDefinition  // 原型方法
    Static     []FunctionDefinition  // 静态方法
}

func (ctx *Context) NewCConstructor(
    classID ClassID,
    def FunctionDefinition,
) (*Object, error) {
    // 1. 创建原型对象
    proto := ctx.NewObject()
    proto.ClassID = classID
    
    // 2. 添加原型方法
    for _, method := range def.Prototype {
        proto.DefineMethod(ctx, method.Name, method.Call, method.Length)
    }
    
    // 3. 创建构造函数
    ctor := ctx.NewObject()
    ctor.ClassID = ClassCFunction
    ctor.Call = def.Call
    ctor.SetPrototype(proto)
    
    // 4. 设置 constructor 属性
    proto.Set(ctx, "constructor", ctor)
    
    // 5. 注册到全局对象
    ctx.GlobalObject.DefineProperty(ctx, def.Name, ctor)
    
    return ctor, nil
}
```

### 3. 属性定义

```go
type PropertyFlags int

const (
    PropWritable    PropertyFlags = 1 << 0
    PropEnumerable  PropertyFlags = 1 << 1
    PropConfigurable PropertyFlags = 1 << 2
    PropGetter      PropertyFlags = 1 << 3
    PropSetter      PropertyFlags = 1 << 4
    PropLength      PropertyFlags = 1 << 5  // special: length property
)

type Property struct {
    Value      Value
    Getter     *Object
    Setter     *Object
    Flags      PropertyFlags
}

func (obj *Object) DefineProperty(ctx *Context, name string, value Value, flags PropertyFlags) error {
    // 检查是否可配置
    if existing, ok := obj.props[name]; ok {
        if existing.Flags&PropConfigurable == 0 {
            return ctx.ThrowTypeError("property not configurable")
        }
    }
    obj.props[name] = &Property{
        Value: value.Dup(),
        Flags: flags,
    }
    return nil
}

func (obj *Object) DefineAccessor(ctx *Context, name string, getter, setter *Object, flags PropertyFlags) error {
    if existing, ok := obj.props[name]; ok {
        if existing.Flags&PropConfigurable == 0 {
            return ctx.ThrowTypeError("property not configurable")
        }
    }
    obj.props[name] = &Property{
        Getter: getter,
        Setter: setter,
        Flags:  flags | PropGetter | PropSetter,
    }
    return nil
}
```

## 陷阱规避

1. **初始化顺序**: Object.prototype 必须在 Function.prototype 之前
2. **constructor 属性**: 每个原型必须指回构造函数
3. **可配置属性**: 定义后不能修改 flags
4. **访问器属性**: getter/setter 必须同时设置或清除
5. **Symbol.toStringTag**: 默认不可枚举
6. **内置方法必须可配置**: ES 规范要求某些内置方法可被删除/覆盖
