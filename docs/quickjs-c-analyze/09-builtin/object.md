# Object 内置对象实现

## 类定义 (行 1533)

```c
#define JS_CLASS_OBJECT 1
```

## 对象结构

### JSObject (行 1500-1532)

```c
typedef struct JSObject {
    JSGCObjectHeader header;  // GC 头，必须为首
    JSClassID class_id : 16;
    JSType type : 8;          // 对象类型
    uint8_t prototype_flavor : 2;  // 原型来源
    BOOL extensible : 1;
    BOOL fast_array_flag : 1;
    BOOL is_exotic : 1;      // 有特殊行为
    BOOL forbid_builtin_method : 1;
    
    union {
        struct {
            JSShape *shape;     // 属性布局
            JSValue *prop;      // 连续属性数组
            int prop_size;     // 属性数组大小
            int total_count;   // 总属性数
            uint32_t hash;      // 形状哈希
        } object;
        
        JSFunctionBytecode *func;
        struct {
            JSValue target;
            JSValue handler;
        } proxy;
        
        // 更多联合成员...
    } u;
} JSObject;
```

## Object.prototype 方法 (行 40424-41320)

```c
static const JSCFunctionListEntry js_object_funcs[] = {
    JS_CFUNC_DEF("create", 2, js_object_create),
    JS_CFUNC_DEF("defineProperty", 3, js_object_defineProperty),
    JS_CFUNC_DEF("defineProperties", 2, js_object_defineProperties),
    JS_CFUNC_DEF("getOwnPropertyDescriptor", 2, js_object_getOwnPropertyDescriptor),
    JS_CFUNC_DEF("getOwnPropertyDescriptors", 1, js_object_getOwnPropertyDescriptors),
    JS_CFUNC_DEF("getOwnPropertyNames", 1, js_object_getOwnPropertyNames),
    JS_CFUNC_DEF("getPrototypeOf", 1, js_object_getPrototypeOf),
    JS_CFUNC_DEF("setPrototypeOf", 2, js_object_setPrototypeOf),
    JS_CFUNC_DEF("is", 2, js_object_is),
    JS_CFUNC_DEF("seal", 1, js_object_seal),
    JS_CFUNC_DEF("freeze", 1, js_object_freeze),
    JS_CFUNC_DEF("isSealed", 1, js_object_isSealed),
    JS_CFUNC_DEF("isFrozen", 1, js_object_isFrozen),
    JS_CFUNC_DEF("keys", 1, js_object_keys),
};

static const JSCFunctionListEntry js_object_proto_funcs[] = {
    JS_CFUNC_DEF("hasOwnProperty", 1, js_object_hasOwnProperty),
    JS_CFUNC_DEF("toString", 0, js_object_toString),
    JS_CFUNC_DEF("valueOf", 0, js_object_valueOf),
    JS_CFUNC_DEF("toLocaleString", 0, js_object_toLocaleString),
    JS_CFUNC_DEF("isPrototypeOf", 1, js_object_isPrototypeOf),
    JS_CFUNC_DEF("propertyIsEnumerable", 1, js_object_propertyIsEnumerable),
    JS_CFUNC_DEF("__defineGetter__", 2, js_object_defineGetter),
    JS_CFUNC_DEF("__defineSetter__", 2, js_object_defineSetter),
    JS_CFUNC_DEF("__lookupGetter__", 1, js_object_defineLookupGetter),
    JS_CFUNC_DEF("__lookupSetter__", 1, js_object_defineLookupSetter),
    JS_CGETSET_DEF("__proto__", js_object_getProto, js_object_setProto),
};
```

## 核心实现分析

### Object.create (行 39516)

```c
static JSValue js_object_constructor(JSContext *ctx, JSValueConst new_target,
                                     int argc, JSValueConst *argv)
{
    JSValue proto, obj;
    
    // Object() 调用: 返回空对象
    // Object.create(proto): 创建以 proto 为原型的对象
    
    if (argc > 0) {
        proto = argv[0];
        if (JS_IsNull(proto))
            proto = JS_NULL;
        else if (!JS_IsObject(proto))
            return JS_ThrowTypeError(ctx, "proto must be an object or null");
    } else {
        proto = JS_GetActiveFunctionProto(ctx);
    }
    
    obj = JS_NewObjectProto(ctx, proto);
    return obj;
}
```

### hasOwnProperty (行 10353-10370)

```c
static JSValue js_object_hasOwnProperty(JSContext *ctx, JSValueConst this_val,
                                        int argc, JSValueConst *argv)
{
    JSValue prop, obj;
    int ret;
    
    obj = JS_ToObject(ctx, this_val);
    
    if (JS_IsString(prop)) {
        ret = JS_HasProperty(ctx, obj, JS_ATOM_String, argv[0]);
    } else if (JS_IsSymbol(prop)) {
        ret = JS_HasProperty(ctx, obj, JS_ATOM_Symbol, argv[0]);
    } else {
        prop = JS_ToPropertyKey(ctx, prop);
        ret = JS_HasProperty(ctx, obj, JS_ATOM_Symbol, prop);
    }
    
    JS_FreeValue(ctx, obj);
    return JS_NewBool(ctx, ret);
}
```

### toString (行 10400-10490)

```c
static JSValue js_object_toString(JSContext *ctx, JSValueConst this_val, int argc, ...)
{
    JSValue tag, obj;
    StringBuffer b = {0};
    BOOL is_array, is_function;
    
    obj = JS_ToObject(ctx, this_val);
    
    // 获取 tag
    tag = JS_GetProperty(ctx, obj, JS_ATOM_Symbol_toStringTag);
    if (JS_IsException(tag))
        goto exception;
    
    if (JS_IsString(tag)) {
        // 自定义 tag
        js_string_concat(&b, tag);
    } else if (is_array) {
        // Array 特殊处理
        js_string_concat(&b, JS_AtomToString(ctx, JS_ATOM_Array));
    } else if (is_function) {
        // Function 特殊处理
        js_string_concat(&b, JS_AtomToString(ctx, JS_ATOM_Function));
    } else {
        // 普通对象
        js_string_concat(&b, JS_AtomToString(ctx, JS_ATOM_Object));
    }
    
    // "[object Tag]"
    js_string_concat(&b, JS_AtomToString(ctx, JS_ATOM_closed));
    js_string_concat(&b, tag);
    js_string_concat(&b, JS_AtomToString(ctx, JS_ATOM_closed));
    
    return b.string;
}
```

## Go 实现建议

### 1. Object 结构

```go
type Object struct {
    gcHeader
    ClassID      ClassID
    Type         ObjectType
    Prototype    *Object
    Extensible   bool
    Properties   map[string]*Property  // 字符串属性
    Symbols      map[*Symbol]*Property  // Symbol 属性
    FastArray    []Value                // 数组优化
}

type Property struct {
    Value    Value
    Getter   *Object
    Setter   *Object
    Flags    PropertyFlags
}

type PropertyFlags uint8

const (
    PropWritable    PropertyFlags = 1 << iota
    PropEnumerable
    PropConfigurable
    PropGetter
    PropSetter
)
```

### 2. Object 创建

```go
func (ctx *Context) NewObject() *Object {
    return &Object{
        ClassID:    ClassObject,
        Type:       ObjectTypeOrdinary,
        Prototype:  ctx.ClassPrototypes[ClassObject],
        Extensible: true,
        Properties: make(map[string]*Property),
        Symbols:    make(map[*Symbol]*Property),
    }
}

func (ctx *Context) NewObjectProto(proto *Object) *Object {
    return &Object{
        ClassID:    ClassObject,
        Type:       ObjectTypeOrdinary,
        Prototype:  proto,
        Extensible: true,
        Properties: make(map[string]*Property),
        Symbols:    make(map[*Symbol]*Property),
    }
}
```

### 3. 属性操作

```go
func (obj *Object) Get(ctx *Context, key string) (Value, error) {
    // 1. 查找自有属性
    if prop, ok := obj.Properties[key]; ok {
        if prop.Flags&PropGetter != 0 {
            return prop.Getter.Call(ctx, obj, nil)
        }
        return prop.Value, nil
    }
    
    // 2. 查找原型链
    if obj.Prototype != nil {
        return obj.Prototype.Get(ctx, key)
    }
    
    return UndefinedValue, nil
}

func (obj *Object) Set(ctx *Context, key string, value Value) error {
    // 1. 查找自有属性
    if prop, ok := obj.Properties[key]; ok {
        if prop.Flags&PropWritable == 0 {
            return ctx.ThrowTypeError("property not writable")
        }
        if prop.Flags&PropGetter != 0 {
            // 访问器属性
            return prop.Setter.Call(ctx, obj, []Value{value})
        }
        prop.Value = value
        return nil
    }
    
    // 2. 如果不可扩展
    if !obj.Extensible {
        return ctx.ThrowTypeError("object not extensible")
    }
    
    // 3. 添加新属性
    obj.Properties[key] = &Property{
        Value:  value,
        Flags:  PropWritable | PropEnumerable | PropConfigurable,
    }
    return nil
}

func (obj *Object) HasProperty(ctx *Context, key string) bool {
    if _, ok := obj.Properties[key]; ok {
        return true
    }
    if obj.Prototype != nil {
        return obj.Prototype.HasProperty(ctx, key)
    }
    return false
}
```

### 4. Object.prototype 方法

```go
func ObjectCreate(ctx *Context, proto Value, args []Value) Value {
    if len(args) == 0 || JS_IsNull(args[0]) {
        proto = ctx.ClassPrototypes[ClassObject]
    } else if !JS_IsObject(args[0]) {
        return ctx.ThrowTypeError("proto must be object or null")
    } else {
        proto = args[0]
    }
    return ValueOf(ctx.NewObjectProto(proto.ToObject(ctx)))
}

func ObjectHasOwnProperty(ctx *Context, this Value, args []Value) Value {
    obj := this.ToObject(ctx)
    key := ctx.ToPropertyKey(args[0])
    return NewBoolValue(obj.HasOwnProperty(ctx, key))
}

func ObjectToString(ctx *Context, this Value, args []Value) Value {
    obj := this.ToObject(ctx)
    
    // 获取 Symbol.toStringTag
    tag := obj.Get(ctx, SymToStringTag)
    if JS_IsString(tag) {
        return tag
    }
    
    // 确定默认 tag
    var tagStr string
    switch obj.ClassID {
    case ClassArray:
        tagStr = "Array"
    case ClassFunction:
        tagStr = "Function"
    case ClassString:
        tagStr = "String"
    case ClassBoolean:
        tagStr = "Boolean"
    case ClassNumber:
        tagStr = "Number"
    case ClassDate:
        tagStr = "Date"
    case ClassRegExp:
        tagStr = "RegExp"
    default:
        tagStr = "Object"
    }
    
    return NewStringValue("[object " + tagStr + "]")
}
```

## 陷阱规避

1. **Object.create(null)**: 创建无原型对象，必须特殊处理属性查找
2. **Symbol.toStringTag**: ES6+ 要求检测此属性用于 toString
3. **null 和 undefined**: Object 方法需要先调用 ToObject
4. **Proxy 劫持**: Object 方法需要检查是否有 Proxy handler
5. **toString 中的类型检测**: 必须检查 is_array 和 is_function 标志
6. **属性顺序**: Object.keys/values/entries 必须按插入顺序枚举
