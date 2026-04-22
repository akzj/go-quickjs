# Generator 实现分析

## 概述

QuickJS 的 Generator 基于协程机制实现，核心是 `JSAsyncFunctionState` 共享栈帧 + 状态机控制。

## 关键数据结构

### JSGeneratorData (quickjs.c:20458-20461)
```c
typedef enum JSGeneratorStateEnum {
    JS_GENERATOR_STATE_SUSPENDED_START,   // 初始挂起
    JS_GENERATOR_STATE_SUSPENDED_YIELD,   // yield 后挂起
    JS_GENERATOR_STATE_SUSPENDED_YIELD_STAR, // yield* 后挂起
    JS_GENERATOR_STATE_EXECUTING,          // 执行中
    JS_GENERATOR_STATE_COMPLETED,          // 已完成
} JSGeneratorStateEnum;

typedef struct JSGeneratorData {
    JSGeneratorStateEnum state;
    JSAsyncFunctionState *func_state;  // 共享栈帧
} JSGeneratorData;
```

### JSAsyncFunctionState (quickjs.c:720-729)
```c
typedef struct JSAsyncFunctionState {
    JSGCObjectHeader header;
    JSValue this_val;
    int argc;
    BOOL throw_flag;          // 用于 throw 异常
    BOOL is_completed;        // 函数是否完成
    JSValue resolving_funcs[2]; // 仅用于 async functions
    JSStackFrame frame;        // 栈帧
} JSAsyncFunctionState;
```

## 核心机制

### 1. 函数调用链

```
js_generator_function_call()      // 创建 generator 对象
    └─> async_func_init()        // 初始化 JSAsyncFunctionState
    └─> js_async_function_resume() // 恢复执行

js_generator_next()              // 调用 .next()
    └─> async_func_resume()      // 恢复栈帧执行
    └─> 返回 yield 值或完成
```

### 2. async_func_resume (quickjs.c:20379-20414)

```c
static JSValue async_func_resume(JSContext *ctx, JSAsyncFunctionState *s)
{
    // 关键：用 JS_TAG_INT 存储指针，通过 JS_CallInternal 调用
    func_obj = JS_MKPTR(JS_TAG_INT, s);
    ret = JS_CallInternal(ctx, func_obj, ..., JS_CALL_FLAG_GENERATOR);
    
    // 结束时标记完成，关闭闭包变量
    if (JS_IsException(ret) || JS_IsUndefined(ret)) {
        s->is_completed = TRUE;
        close_var_refs(rt, b, sf);
        async_func_free_frame(rt, s);
    }
    return ret;
}
```

### 3. VM 层 opcodes (quickjs.c:20024-20038)

```c
CASE(OP_await):
    ret_val = JS_NewInt32(ctx, FUNC_RET_AWAIT);
    goto done_generator;
CASE(OP_yield):
    ret_val = JS_NewInt32(ctx, FUNC_RET_YIELD);
    goto done_generator;
CASE(OP_yield_star):
CASE(OP_async_yield_star):
    ret_val = JS_NewInt32(ctx, FUNC_RET_YIELD_STAR);
    goto done_generator;
CASE(OP_initial_yield):
    ret_val = JS_NewInt32(ctx, FUNC_RET_INITIAL_YIELD);
    goto done_generator;
```

### 4. FUNC_RET 返回码 (quickjs.c:17366-17369)

```c
#define FUNC_RET_AWAIT         0
#define FUNC_RET_YIELD         1
#define FUNC_RET_YIELD_STAR    2
#define FUNC_RET_INITIAL_YIELD 3
```

## 状态转换图

```
                    调用 generator()
                         │
                         ▼
          ┌──────────────────────────────┐
          │ JS_GENERATOR_STATE_SUSPENDED_START │
          └──────────────────────────────┘
                         │
               ┌─────────┴─────────┐
               ▼                   ▼
          .next()              .return/.throw()
               │                   │
               ▼                   ▼
          设置 cur_sp[-1]       free_generator_stack()
          = 传入值               → COMPLETED
               │
               ▼
          async_func_resume()
               │
         ┌─────┴─────┐
         ▼           ▼
      有 yield    无 yield
         │           │
         ▼           ▼
    返回 (value, false)   返回最终值
    状态→SUSPENDED_YIELD    → COMPLETED
```

## Go 实现建议

### 1. Generator 结构

```go
type Generator struct {
    ctx     *Context
    state   GeneratorState
    // 核心：栈帧状态
    frame   *StackFrame
    // 保存的寄存器/栈
    savedPC  int
    savedSP  int
}

type GeneratorState int
const (
    GEN_STATE_SUSPENDED_START  GeneratorState = iota
    GEN_STATE_SUSPENDED_YIELD
    GEN_STATE_SUSPENDED_YIELD_STAR
    GEN_STATE_EXECUTING
    GEN_STATE_COMPLETED
)
```

### 2. 关键实现点

- **不要复制 C 的 setjmp/longjmp** — Go 用 panic/recover 或通道通信
- **栈帧序列化** — Generator 挂起时保存整个栈帧状态（PC、SP、局部变量）
- **闭包变量** — close_var_refs 需要遍历所有 JSVarRef 并解包

### 3. 陷阱

1. **栈深度问题**: 生成器可能嵌套，需要完整保存栈帧
2. **异常传播**: throw 时设置 throw_flag，恢复后抛出异常
3. **yield* 委托**: 需要递归处理迭代器

## 相关 opcodes (quickjs-opcode.h:212-216)

```c
DEF(initial_yield, 1, 0, 0, none)    // async 生成器初始 yield
DEF(yield,        1, 1, 2, none)    // yield value
DEF(yield_star,   1, 1, 2, none)    // yield* 委托
DEF(async_yield_star, 1, 1, 2, none) // async yield*
```