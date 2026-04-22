# 函数调用机制

## 概述

QuickJS 的函数调用通过 `JS_CallInternal` (行 17372) 实现，支持多种函数类型：
- C 函数 (JS_CLASS_C_FUNCTION)
- 字节码函数 (JS_CLASS_BYTECODE_FUNCTION)
- 绑定函数 (JS_CLASS_BOUND_FUNCTION)
- Generator 函数
- Async 函数

## 调用入口 (行 20145)

```c
JSValue JS_Call(JSContext *ctx, JSValueConst func_obj, JSValueConst this_obj,
                int argc, JSValueConst *argv)
{
    return JS_CallInternal(ctx, func_obj, this_obj, JS_UNDEFINED, argc, argv, 0);
}
```

## JS_CallInternal 核心逻辑 (行 17372-18750)

### 1. 前置检查

```c
// 中断检查
if (js_poll_interrupts(caller_ctx))
    return JS_EXCEPTION;

// 函数对象类型检查
if (unlikely(JS_VALUE_GET_TAG(func_obj) != JS_TAG_OBJECT))
    goto not_a_function;

// 获取对象指针
p = JS_VALUE_GET_OBJ(func_obj);

// 非字节码函数调用类方法
if (unlikely(p->class_id != JS_CLASS_BYTECODE_FUNCTION)) {
    JSClassCall *call_func = rt->class_array[p->class_id].call;
    if (!call_func)
        goto not_a_function;
    return call_func(caller_ctx, func_obj, this_obj, argc, argv, flags);
}
```

### 2. 参数处理

```c
// 参数数量适配
if (unlikely(argc < b->arg_count || (flags & JS_CALL_FLAG_COPY_ARGV))) {
    arg_allocated_size = b->arg_count;
} else {
    arg_allocated_size = 0;
}

// 栈空间计算
alloca_size = sizeof(JSValue) * (arg_allocated_size + b->var_count + b->stack_size) +
              sizeof(JSVarRef *) * b->var_ref_count;

// 栈溢出检查
if (js_check_stack_overflow(rt, alloca_size))
    return JS_ThrowStackOverflow(caller_ctx);
```

### 3. 栈帧分配

```c
// 栈布局:
// [参数] [局部变量] [操作数栈] [变量引用指针数组]

arg_buf = argv;
sf->arg_count = argc;
sf->cur_func = (JSValue)func_obj;

local_buf = alloca(alloca_size);

// 参数复制/填充
if (unlikely(arg_allocated_size)) {
    int n = min_int(argc, b->arg_count);
    arg_buf = local_buf;
    for(i = 0; i < n; i++)
        arg_buf[i] = JS_DupValue(caller_ctx, argv[i]);
    for(; i < b->arg_count; i++)
        arg_buf[i] = JS_UNDEFINED;
}

var_buf = local_buf + arg_allocated_size;
sf->var_buf = var_buf;
sf->arg_buf = arg_buf;

// 初始化局部变量
for(i = 0; i < b->var_count; i++)
    var_buf[i] = JS_UNDEFINED;

stack_buf = var_buf + b->var_count;
sp = stack_buf;
```

### 4. PC 和帧链更新

```c
pc = b->byte_code_buf;
sf->prev_frame = rt->current_stack_frame;
rt->current_stack_frame = sf;
ctx = b->realm;  // 设置当前 realm
```

## C 函数调用 (行 17193)

```c
static JSValue js_call_c_function(JSContext *ctx, JSValueConst func_obj,
                                  int argc, JSValueConst *argv, int magic)
{
    JSObject *p = JS_VALUE_GET_OBJ(func_obj);
    JSCFunction *func;
    JSCFunctionMagic *func_magic;
    
    // C 函数存储在 u.c_handler
    if (p->class_id == JS_CLASS_C_FUNCTION) {
        func = p->u.c_function.generic;
        return func(ctx, this_val, argc, argv);
    } else {
        // C 函数带 magic
        func_magic = p->u.c_function.generic_magic;
        return func_magic(ctx, this_val, argc, argv, magic);
    }
}
```

## 构造函数调用 (行 20237)

```c
static JSValue JS_CallConstructorInternal(JSContext *ctx, JSValueConst func_obj,
                                         JSValueConst new_target, int argc, 
                                         JSValueConst *argv, int flags)
{
    // 1. 调用函数获取返回值
    ret = JS_CallInternal(ctx, func_obj, JS_UNDEFINED, new_target, argc, argv, flags);
    
    // 2. 如果返回值为对象，直接返回
    if (JS_IsObject(ret))
        return ret;
    
    // 3. 否则返回 new_target 创建的对象
    obj = JS_NewObjectFromProto(ctx, JS_GetPrototype(ctx, new_target));
    JS_FreeValue(ctx, ret);
    return obj;
}
```

## Generator 函数处理

Generator 函数特殊处理在行 17412-17440：

```c
if (flags & JS_CALL_FLAG_GENERATOR) {
    JSAsyncFunctionState *s = JS_VALUE_GET_PTR(func_obj);
    sf = &s->frame;  // 使用预分配的帧
    
    // 恢复执行状态
    sp = sf->cur_sp;
    sf->cur_sp = NULL;
    pc = sf->cur_pc;
    goto restart;
}
```

## Go 实现建议

### 1. 函数类型

```go
type FunctionType int

const (
    FunctionTypeC          FunctionType = iota
    FunctionTypeBytecode
    FunctionTypeBound
    FunctionTypeGenerator
    FunctionTypeAsync
)

type Callable interface {
    Call(ctx *Context, this Value, args []Value) (Value, error)
}

type CFunction struct {
    Call    func(ctx *Context, this Value, args []Value) Value
    Magic   int
}

type BytecodeFunction struct {
    Bytecode *FunctionBytecode
    Realm    *Context
}
```

### 2. 调用实现

```go
func (ctx *Context) CallInternal(funcObj, thisObj Value, args []Value, flags int) (Value, error) {
    // 1. 中断检查
    if ctx.pollInterrupts() {
        return UndefinedValue, ctx.ThrowTypeError("interrupted")
    }
    
    // 2. 获取可调用对象
    obj, ok := funcObj.(*Object)
    if !ok {
        return UndefinedValue, ctx.ThrowTypeError("not a function")
    }
    
    // 3. 分派到具体调用逻辑
    switch obj.ClassID {
    case ClassCFunction:
        return ctx.callCFunction(obj, thisObj, args)
    case ClassBytecodeFunction:
        return ctx.callBytecodeFunction(obj, thisObj, args, flags)
    case ClassBoundFunction:
        return ctx.callBoundFunction(obj, thisObj, args, flags)
    default:
        // 检查自定义 call 方法
        if call := obj.getCallMethod(); call != nil {
            return call(ctx, thisObj, args)
        }
        return UndefinedValue, ctx.ThrowTypeError("not a function")
    }
}
```

### 3. 栈帧结构

```go
type StackFrame struct {
    PrevFunc   Value       // 调用者
    CurFunc    Value       // 当前函数
    ArgBuf     []Value     // 参数缓冲区
    VarBuf     []Value     // 局部变量
    VarRefs    []*VarRef   // 闭包引用
    CurPC      []byte      // 当前指令指针
    CurSP      *int        // 当前栈指针
}

func (ctx *Context) callBytecodeFunction(obj *Object, thisObj Value, args []Value, flags int) (Value, error) {
    bc := obj.Bytecode
    
    // 1. 分配栈帧
    frame := &StackFrame{
        CurFunc: ValueOf(obj),
        ArgBuf:  args,
        VarBuf:  make([]Value, bc.VarCount),
        CurPC:   bc.Code,
    }
    
    // 2. 设置闭包引用
    frame.VarRefs = obj.VarRefs
    
    // 3. 初始化局部变量
    for i := range frame.VarBuf {
        frame.VarBuf[i] = UndefinedValue
    }
    
    // 4. 链接栈帧
    frame.PrevFunc = ctx.currentFrame
    ctx.currentFrame = frame
    
    // 5. 执行字节码
    return ctx.execute(frame)
}
```

### 4. 参数处理

```go
func prepareArguments(ctx *Context, bc *FunctionBytecode, args []Value) ([]Value, []Value, error) {
    // 计算所需空间
    need := bc.ArgCount + bc.VarCount + bc.StackSize + bc.VarRefCount
    
    // 分配连续内存
    buf := make([]Value, need)
    
    argBuf := buf[:bc.ArgCount]
    varBuf := buf[bc.ArgCount:bc.ArgCount+bc.VarCount]
    stackBuf := buf[bc.ArgCount+bc.VarCount:bc.ArgCount+bc.VarCount+bc.StackSize]
    
    // 复制/填充参数
    for i := 0; i < bc.ArgCount; i++ {
        if i < len(args) {
            argBuf[i] = args[i].Dup()
        } else {
            argBuf[i] = UndefinedValue
        }
    }
    
    return argBuf, varBuf, nil
}
```

## 陷阱规避

1. **参数窃取 (Argument Stealing)**: arguments 对象可以修改实参，需要分离存储
2. **尾调用优化**: 需要正确识别尾调用模式，释放当前栈帧
3. **Generator 恢复**: 恢复时必须恢复完整的执行状态 (PC, SP, 局部变量)
4. **Realm 切换**: 跨 realm 调用时必须切换 ctx
5. **栈溢出检查**: 必须在分配前检查，否则可能触发不安全的 longjmp
