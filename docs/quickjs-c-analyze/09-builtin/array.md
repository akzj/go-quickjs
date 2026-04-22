# Array 内置对象实现

## 类定义 (行 1535)

```c
#define JS_CLASS_ARRAY 2
```

## Array 对象结构

### JSArray (行 1518-1525)

```c
typedef struct JSArray {
    JSGCObjectHeader header;
    JSObject obj;
    uint32_t max_size;  // 最大索引 + 1
    uint32_t size;     // 当前长度
} JSArray;
```

### Fast Array 优化

```c
typedef struct JSPropertyEnum {
    int atom_index;
    uint32_t hash;
    uint8_t is_enumerable;
    uint8_t is_writable;
    uint8_t is_configurable;
} JSPropertyEnum;

typedef struct {
    int size;  // 数组大小
    int count; // 元素数量
    union {
        JSValue *values;     // 快速数组
        JSPropertyEnum *tab; // 属性表
    } u;
} JSArrayEnum;
```

## Array 构造函数 (行 39570-39640)

```c
static JSValue js_array_constructor(JSContext *ctx, JSValueConst new_target,
                                    int argc, JSValueConst *argv)
{
    JSValue obj;
    uint32_t len;
    
    // new Array(length) 或 new Array(...items)
    if (argc == 0) {
        // Array() -> []
        obj = JS_NewArray(ctx);
    } else if (argc == 1) {
        // Array(len) -> 空数组，len 是长度
        if (JS_IsNumber(argv[0])) {
            double d = JS_ToFloat64(ctx, argv[0]);
            if (d != d || d < 0 || d > 4294967295) {
                JS_ThrowRangeError(ctx, "Invalid array length");
                return JS_EXCEPTION;
            }
            len = (uint32_t)d;
            obj = JS_NewArray(ctx);
            if (JS_IsException(obj))
                return JS_EXCEPTION;
            // 预分配但不初始化
            if (len > 0)
                JS_SetPropertyInternal(ctx, obj, JS_ATOM_length, 
                                       JS_NewUint32(ctx, len), ...);
        } else {
            // Array(item) -> [item]
            obj = JS_NewArray(ctx);
            if (JS_IsException(obj))
                return JS_EXCEPTION;
            JS_SetPropertyUint32(ctx, obj, 0, JS_DupValue(ctx, argv[0]));
        }
    } else {
        // Array(...items) -> [items...]
        obj = JS_NewArray(ctx);
        if (JS_IsException(obj))
            return JS_EXCEPTION;
        for(uint32_t i = 0; i < argc; i++)
            JS_SetPropertyUint32(ctx, obj, i, JS_DupValue(ctx, argv[i]));
    }
    
    return obj;
}
```

## Array.prototype 方法 (行 42097-44150)

### push/pop/shift/unshift (行 42097-42160)

```c
// push 使用 unshift = 0, pop 使用 magic = 1
static JSValue js_array_push(JSContext *ctx, JSValueConst this_val,
                             int argc, JSValueConst *argv, int magic)
{
    JSValue *sp;
    uint32_t len;
    
    if (JS_IsException(JS_GetLength(ctx, this_val, &len)))
        return JS_EXCEPTION;
    
    // 转换 this 为数组
    this_val = JS_ToObject(ctx, this_val);
    
    if (magic == 0) {
        // push
        if (check_exception_free(
            sp = js_enter_match_value(ctx, this_val, len, TRUE, TRUE)))
            return JS_EXCEPTION;
        if (sp) {
            // 快速路径：直接追加
            for(int i = 0; i < argc; i++)
                sp[i] = JS_DupValue(ctx, argv[i]);
            JS_SetLength(ctx, this_val, len + argc);
            return JS_NewUint32(ctx, len + argc);
        }
    } else {
        // pop: 减少长度
    }
    
    // 慢速路径：逐个设置
    for(int i = 0; i < argc; i++) {
        if (JS_SetPropertyUint32(ctx, this_val, len + i, JS_DupValue(ctx, argv[i])) < 0)
            return JS_EXCEPTION;
    }
    
    JS_SetLength(ctx, this_val, len + argc);
    return JS_NewUint32(ctx, len + argc);
}
```

### map (行 42850-42920)

```c
static JSValue js_array_map(JSContext *ctx, JSValueConst this_val,
                            int argc, JSValueConst *argv)
{
    JSContext *c = ctx;
    JSValue T, A, k, v, result;
    uint32_t len;
    JSValue *arrp;
    uint32_t count;
    BOOL hole;
    
    // 1. 获取长度
    if (JS_IsException(js_get_length(ctx, &len, this_val)))
        return JS_EXCEPTION;
    
    // 2. 创建结果数组
    A = JS_NewArrayInternal(ctx);
    JS_SetLength(ctx, A, len);
    
    // 3. 遍历
    for(uint32_t i = 0; i < len; i++) {
        // 检查元素是否存在
        if (js_get_array_element(ctx, this_val, i, &v, &hole) < 0)
            return JS_EXCEPTION;
        
        if (!hole) {
            // 调用回调
            JSValue item_args[3] = { v, JS_NewInt32(ctx, i), JS_DupValue(ctx, this_val) };
            JSValue mapped = JS_Call(ctx, argv[0], T, 3, item_args);
            JS_FreeValue(ctx, item_args[2]);
            
            if (JS_IsException(mapped))
                return JS_EXCEPTION;
            
            // 设置到结果数组
            JS_SetPropertyUint32(ctx, A, i, mapped);
        }
    }
    
    return A;
}
```

### reduce/reduceRight (行 42920-43020)

```c
static JSValue js_array_reduce(JSContext *ctx, JSValueConst this_val,
                               int argc, JSValueConst *argv, int magic)
{
    JSValue accumulator, kValue;
    uint32_t len;
    BOOL present;
    int64_t index;
    
    // 获取初始值
    if (argc > 1) {
        accumulator = JS_DupValue(ctx, argv[1]);
    } else {
        // 查找第一个元素作为初始值
        for(uint32_t i = 0; i < len; i++) {
            if (js_get_array_element(ctx, this_val, i, &kValue, &present) < 0)
                return JS_EXCEPTION;
            if (present) {
                accumulator = kValue;
                goto has_initial;
            }
        }
        // 没有初始值且数组为空
        return JS_ThrowTypeError(ctx, "reduce of empty array with no initial value");
    }
    
    // 累加
    for(uint32_t i = 0; i < len; i++) {
        if (js_get_array_element(ctx, this_val, i, &kValue, &present) < 0)
            return JS_EXCEPTION;
        if (present) {
            JSValue args[4] = { accumulator, kValue, JS_NewInt32(ctx, i), JS_DupValue(ctx, this_val) };
            accumulator = JS_Call(ctx, argv[0], JS_UNDEFINED, 4, args);
            JS_FreeValue(ctx, args[3]);
            if (JS_IsException(accumulator))
                return JS_EXCEPTION;
        }
    }
    
    return accumulator;
}
```

## Go 实现建议

### 1. Array 结构

```go
type Array struct {
    Object
    length uint32
    values []Value  // 快速数组存储
}

func NewArray(ctx *Context) *Array {
    return &Array{
        Object: Object{
            ClassID:   ClassArray,
            Prototype: ctx.ClassPrototypes[ClassArray],
            Properties: make(map[string]*Property),
        },
        values: make([]Value, 0, 16),
    }
}
```

### 2. 长度属性

```go
func (a *Array) GetLength(ctx *Context) uint32 {
    return a.length
}

func (a *Array) SetLength(ctx *Context, length uint32) {
    if length < a.length {
        // 截断
        for i := length; i < a.length; i++ {
            a.values[i].Free()
        }
        a.values = a.values[:length]
    } else if length > a.length {
        // 扩展
        if uint32(cap(a.values)) < length {
            newCap := length
            if newCap < uint32(cap(a.values))*2 {
                newCap = uint32(cap(a.values)) * 2
            }
            newValues := make([]Value, length, newCap)
            copy(newValues, a.values)
            a.values = newValues
        } else {
            a.values = a.values[:length]
        }
    }
    a.length = length
}
```

### 3. 索引属性操作

```go
func (a *Array) GetIndex(ctx *Context, index uint32) (Value, bool) {
    if index < a.length {
        return a.values[index], true
    }
    return UndefinedValue, false
}

func (a *Array) SetIndex(ctx *Context, index uint32, value Value) {
    if index >= a.length {
        // 需要扩展数组
        a.SetLength(ctx, index+1)
    }
    a.values[index] = value
}
```

### 4. Array.prototype.map

```go
func ArrayMap(ctx *Context, this Value, args []Value) Value {
    if len(args) == 0 {
        return ctx.ThrowTypeError("Array.prototype.map requires callback")
    }
    
    callback := args[0]
    T := UndefinedValue
    if len(args) > 1 {
        T = args[1]
    }
    
    arr := this.ToObject(ctx).(*Array)
    len := arr.GetLength(ctx)
    
    result := NewArray(ctx)
    result.SetLength(ctx, len)
    
    for i := uint32(0); i < len; i++ {
        if val, ok := arr.GetIndex(ctx, i); ok {
            mapped := callback.Call(ctx, T, []Value{val, NewIntValue(int64(i)), this.Dup()})
            result.SetIndex(ctx, i, mapped)
        }
    }
    
    return ValueOf(result)
}
```

### 5. Array.prototype.reduce

```go
func ArrayReduce(ctx *Context, this Value, args []Value, right bool) Value {
    if len(args) == 0 {
        return ctx.ThrowTypeError("Array.prototype.reduce requires callback")
    }
    
    callback := args[0]
    arr := this.ToObject(ctx).(*Array)
    len := arr.GetLength(ctx)
    
    var accumulator Value
    var kStart uint32
    
    if len(args) > 1 {
        accumulator = args[1]
    } else {
        // 查找初始值
        found := false
        for i := uint32(0); i < len; i++ {
            if val, ok := arr.GetIndex(ctx, i); ok {
                accumulator = val
                kStart = i + 1
                found = true
                break
            }
        }
        if !found {
            return ctx.ThrowTypeError("reduce of empty array with no initial value")
        }
    }
    
    iter := kStart
    end := len
    step := uint32(1)
    if right {
        iter = len - 1
        end = kStart - 1
        step = ^uint32(0) // -1 as unsigned
    }
    
    for i := iter; i != end; i += step {
        if val, ok := arr.GetIndex(ctx, i); ok {
            accumulator = callback.Call(ctx, UndefinedValue, 
                []Value{accumulator, val, NewIntValue(int64(i)), this.Dup()})
        }
    }
    
    return accumulator
}
```

## 陷阱规避

1. **稀疏数组**: 索引不连续时不能使用快速数组
2. **length 属性**: 修改 length 可能截断或扩展数组
3. **原型方法中的 ToObject**: 所有方法必须先转换 this 为对象
4. **Hole 跳过**: map/filter 等必须跳过不存在 (hole) 的元素
5. **reduce 的初始值**: 没有初始值时使用第一个实际元素
6. **delete 操作**: 删除数组元素会创建 hole
7. **类数组对象**: arguments, NodeList 等有 length 和索引属性
8. **Fast Array 条件**: 当数组只有索引属性且无访问器时启用优化
