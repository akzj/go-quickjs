# JSContext 设计详解

## 结构定义 (行 448-493)

```c
struct JSContext {
    JSGCObjectHeader header;     // GC 对象头，必须为首
    JSRuntime *rt;               // 指向所属 Runtime
    struct list_head link;       // 链表节点

    // 预计算的 Shape 缓存
    JSShape *array_shape;        // Array 对象初始形状
    JSShape *arguments_shape;   // arguments 对象形状
    JSShape *mapped_arguments_shape;  // mapped arguments 形状
    JSShape *regexp_shape;       // RegExp 对象形状
    JSShape *regexp_result_shape;      // RegExp 结果形状

    // 内置构造函数
    JSValue *class_proto;        // 类原型数组
    JSValue function_proto;      // Function.prototype
    JSValue function_ctor;       // Function 构造函数
    JSValue array_ctor;          // Array 构造函数
    JSValue regexp_ctor;         // RegExp 构造函数
    JSValue promise_ctor;        // Promise 构造函数
    JSValue iterator_ctor;       // Iterator 构造函数
    JSValue async_iterator_proto;
    JSValue array_proto_values;
    
    // 错误类型原型
    JSValue native_error_proto[JS_NATIVE_ERROR_COUNT];
    
    // 全局对象
    JSValue global_obj;          // 全局对象 (window/globalThis)
    JSValue global_var_obj;      // let/const 全局变量容器
    
    // 运行时状态
    uint64_t random_state;      // 随机数状态
    int interrupt_counter;       // 中断计数器
    
    // 模块系统
    struct list_head loaded_modules;  // 已加载模块列表
    
    // 可定制钩子
    JSValue (*compile_regexp)(...);    // 正则编译回调
    JSValue (*eval_internal)(...);   // eval 实现回调
    void *user_opaque;               // 用户扩展数据
};
```

## 核心设计特点

### 1. 预计算 Shape 缓存

Shape 是 QuickJS 对象属性布局的压缩表示：
- **array_shape**: Array 对象创建时复用
- **arguments_shape**: 非映射 arguments 对象
- **mapped_arguments_shape**: 映射到实参的 arguments 对象

```c
// 创建预计算 Shape (行 55745-55760)
ctx->array_shape = js_new_shape2(ctx, get_proto_obj(ctx->class_proto[JS_CLASS_ARRAY]),
                                 JS_PROP_INITIAL_HASH_SIZE, 1);
add_shape_property(ctx, &ctx->array_shape, NULL, JS_ATOM_length, 
                   JS_PROP_WRITABLE | JS_PROP_LENGTH);
```

### 2. 内置构造函数缓存

避免重复查找，提高性能：

```c
ctx->function_proto = JS_NewCFunction3(ctx, js_function_proto, "", 0, ...);
ctx->array_ctor = JS_NewCConstructor(ctx, JS_CLASS_ARRAY, "Array", ...);
```

### 3. 全局对象双层结构

```
global_obj
    ├── 全局函数 (parseInt, eval, ...)
    ├── 全局类 (Object, Array, Function, ...)
    └── 动态添加的属性
    
global_var_obj  
    └── let/const 声明的全局变量
```

这种分离设计支持：
- eval 代码的特殊作用域处理
- 更好的 let/const 封装

### 4. 中断机制

```c
int interrupt_counter;  // 每次调用递减，归零时触发中断
```

用于：
- 执行时间限制
- 调试断点
- 协作式多任务

## Context 创建流程 (行 2176-2229)

```c
JSContext *JS_NewContextRaw(JSRuntime *rt)
{
    // 1. 分配 Context 结构
    ctx = js_mallocz_rt(rt, sizeof(JSContext));
    
    // 2. 添加到 GC 对象列表
    add_gc_object(rt, &ctx->header, JS_GC_OBJ_TYPE_JS_CONTEXT);
    
    // 3. 分配类原型数组
    ctx->class_proto = js_malloc_rt(rt, sizeof(ctx->class_proto[0]) * rt->class_count);
    
    // 4. 加入 Runtime 的 Context 列表
    list_add_tail(&ctx->link, &rt->context_list);
    
    // 5. 初始化基本对象
    JS_AddIntrinsicBasicObjects(ctx);
    
    return ctx;
}

JSContext *JS_NewContext(JSRuntime *rt)
{
    ctx = JS_NewContextRaw(rt);
    // 添加所有内置对象
    JS_AddIntrinsicBaseObjects(ctx);
    JS_AddIntrinsicDate(ctx);
    JS_AddIntrinsicEval(ctx);
    // ... 更多内置对象
    return ctx;
}
```

## Go 实现建议

### 1. Context 结构

```go
type Context struct {
    rt *Runtime
    
    // Shape 缓存
    arrayShape        *Shape
    argumentsShape    *Shape
    
    // 构造函数缓存
    constructors map[ClassID]*Object
    prototypes   map[ClassID]*Object
    
    // 全局对象
    GlobalObject   *Object
    GlobalVarObj   *Object  // let/const
    
    // 运行时状态
    randomState uint64
    interruptCounter int
    
    // 模块系统
    loadedModules []*Module
    
    // 用户扩展
    userOpaque interface{}
}
```

### 2. 关键实现点

```go
func NewContext(rt *Runtime) (*Context, error) {
    ctx := &Context{
        rt:            rt,
        constructors:  make(map[ClassID]*Object),
        prototypes:    make(map[ClassID]*Object),
        randomState:   uint64(time.Now().UnixNano()),
        interruptCounter: 100000, // 默认值
    }
    
    // 1. 添加到 Runtime
    rt.addContext(ctx)
    
    // 2. 初始化基本对象
    if err := ctx.initBasicObjects(); err != nil {
        return nil, err
    }
    
    // 3. 初始化内置对象
    if err := ctx.initBaseObjects(); err != nil {
        return nil, err
    }
    
    return ctx, nil
}
```

### 3. 中断机制

```go
func (ctx *Context) pollInterrupts() bool {
    ctx.interruptCounter--
    if ctx.interruptCounter <= 0 {
        if ctx.rt.interruptHandler != nil {
            if ctx.rt.interruptHandler(ctx.rt, ctx.rt.interruptOpaque) {
                return true // 中断请求
            }
        }
        ctx.interruptCounter = 100000 // 重置
    }
    return false
}
```

## 陷阱规避

1. **不要直接操作 class_proto 数组** - 通过 JS_SetClassProto/JS_GetClassProto
2. **interrupt_counter 是关键限流器** - 不要设置为 0
3. **global_var_obj 是 let/const 的宿主** - 与 global_obj 分离是设计要求
4. **Context 属于 GC 对象** - 引用计数管理必须正确
