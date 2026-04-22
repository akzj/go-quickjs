# 09 - Builtin 内置对象模块

本模块分析 QuickJS 内置 JavaScript 对象的实现。

## 文件列表

| 文件 | 内容 |
|------|------|
| [overview.md](overview.md) | 内置对象分类和初始化 |
| [object.md](object.md) | Object 实现详解 |
| [array.md](array.md) | Array 实现详解 |
| [string.md](string.md) | String 实现详解 |
| [go-impl.md](go-impl.md) | Go 实现指南 |

## 内置对象分类

### 原始值包装器 (Primitive Wrappers)

| 类 | JS_CLASS_* | 内部表示 |
|----|------------|----------|
| Number | JS_CLASS_NUMBER | `double` |
| String | JS_CLASS_STRING | `JSString*` |
| Boolean | JS_CLASS_BOOLEAN | `BOOL` |
| Symbol | JS_CLASS_SYMBOL | `JSAtom` |
| BigInt | JS_CLASS_BIG_INT | `JSBigInt*` |

### 复合对象

| 类 | JS_CLASS_* | 特点 |
|----|------------|------|
| Object | JS_CLASS_OBJECT | 属性哈希表 |
| Array | JS_CLASS_ARRAY | 连续存储优化 |
| Function | JS_CLASS_BYTECODE_FUNCTION | 字节码容器 |
| Date | JS_CLASS_DATE | 双精度时间戳 |
| RegExp | JS_CLASS_REGEXP | 正则表达式 |

### 集合类型

| 类 | JS_CLASS_* |
|----|------------|
| Map | JS_CLASS_MAP |
| Set | JS_CLASS_SET |
| WeakMap | JS_CLASS_WEAK_MAP |
| WeakSet | JS_CLASS_WEAK_SET |
| WeakRef | JS_CLASS_WEAK_REF |

### 类型化数组

| 类 | JS_CLASS_* |
|----|------------|
| Int8Array | JS_CLASS_INT8_ARRAY |
| Uint8Array | JS_CLASS_UINT8_ARRAY |
| Int16Array | JS_CLASS_INT16_ARRAY |
| Uint16Array | JS_CLASS_UINT16_ARRAY |
| Int32Array | JS_CLASS_INT32_ARRAY |
| Uint32Array | JS_CLASS_UINT32_ARRAY |
| Float32Array | JS_CLASS_FLOAT32_ARRAY |
| Float64Array | JS_CLASS_FLOAT64_ARRAY |
| BigInt64Array | JS_CLASS_BIG_INT64_ARRAY |
| BigUint64Array | JS_CLASS_BIG_UINT64_ARRAY |

## 初始化顺序

```
1. JS_AddIntrinsicBasicObjects() (行 55685)
   ├── Object.prototype
   ├── Function.prototype
   ├── Error
   ├── Array
   └── Global Object

2. JS_AddIntrinsicBaseObjects() (行 55817)
   ├── Iterator
   ├── Number
   ├── Boolean
   ├── String
   ├── Symbol
   ├── BigInt
   └── Math

3. 其他内置对象
   ├── JS_AddIntrinsicDate()
   ├── JS_AddIntrinsicRegExp()
   ├── JS_AddIntrinsicJSON()
   ├── JS_AddIntrinsicProxy()
   ├── JS_AddIntrinsicMapSet()
   ├── JS_AddIntrinsicTypedArrays()
   ├── JS_AddIntrinsicPromise()
   └── JS_AddIntrinsicWeakRef()
```

## 对象结构

### JSObject (行 1500-1532)

```c
struct JSObject {
    JSGCObjectHeader header;
    JSClassID class_id : 16;
    JSType type : 8;
    
    union {
        struct {
            JSShape *shape;     // 属性布局
            JSValue *prop;      // 连续属性
            int prop_size;
        } object;
        
        JSFunctionBytecode *func;
        struct {
            JSValue target;
            JSValue handler;
        } proxy;
        
        // 数组、类型化数组等...
    } u;
};
```

## 关键实现位置

| 内置对象 | 构造函数行号 | 原型方法行号 |
|----------|-------------|-------------|
| Object | 39516 | 40424 |
| Function | 17193 | 17100 |
| Array | 39570 | 42097 |
| String | 44170 | 44650 |
| Number | 44320 | 44450 |
| Boolean | 44250 | 44570 |
| Date | 54400 | 54500 |
| RegExp | 48600 | 48700 |
| Map | 52600 | 52650 |
| Set | 52700 | 52750 |
| Promise | 52800 | 52900 |

## Go 移植架构

```go
// builtin.go

type BuiltinRegistry struct {
    classes    map[ClassID]*ClassDefinition
    prototypes map[ClassID]*Object
    constructors map[ClassID]*Object
}

func (ctx *Context) initBuiltinObjects() error {
    // 1. 基础对象 (必须首先初始化)
    if err := ctx.initObject(); err != nil {
        return err
    }
    if err := ctx.initFunction(); err != nil {
        return err
    }
    
    // 2. 原始值包装器
    if err := ctx.initNumber(); err != nil {
        return err
    }
    if err := ctx.initString(); err != nil {
        return err
    }
    if err := ctx.initBoolean(); err != nil {
        return err
    }
    if err := ctx.initSymbol(); err != nil {
        return err
    }
    
    // 3. 复合对象
    if err := ctx.initArray(); err != nil {
        return err
    }
    if err := ctx.initDate(); err != nil {
        return err
    }
    if err := ctx.initRegExp(); err != nil {
        return err
    }
    
    // 4. 集合和高级类型
    if err := ctx.initMapSet(); err != nil {
        return err
    }
    if err := ctx.initTypedArrays(); err != nil {
        return err
    }
    if err := ctx.initPromise(); err != nil {
        return err
    }
    if err := ctx.initProxy(); err != nil {
        return err
    }
    
    return nil
}
```

## 陷阱规避

1. **初始化顺序**: Object.prototype 必须首先初始化
2. **constructor 循环引用**: 原型必须指向构造函数
3. **Symbol.toStringTag**: ES6+ 要求
4. **Fast Array 优化**: 稀疏数组不能使用优化
5. **原型方法中的 ToObject**: 所有方法必须先转换 this
6. **NaN 比较**: 必须使用 `math.IsNaN()`
7. **Infinity 处理**: 溢出检查
8. **Promise 微任务**: 必须异步执行
