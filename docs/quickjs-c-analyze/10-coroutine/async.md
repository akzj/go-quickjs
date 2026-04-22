# Async/Promise 实现分析

## 概述

QuickJS 的 async/await 基于 Generator + Promise 实现。async function 自动插入 await 调用。

## Promise 数据结构

### JSPromiseData (quickjs.c:52640-52647)
```c
typedef struct JSPromiseData {
    JSPromiseStateEnum promise_state;  // PENDING/FULFILLED/REJECTED
    struct list_head promise_reactions[2];  // [fulfill, reject] 回调链
    BOOL is_handled;                    // 是否被处理（用于 rejection tracking）
    JSValue promise_result;             // resolve/reject 的值
} JSPromiseData;

typedef enum JSPromiseStateEnum {
    JS_PROMISE_PENDING = 0,
    JS_PROMISE_FULFILLED = 1,
    JS_PROMISE_REJECTED = 2,
} JSPromiseStateEnum;
```

### JSPromiseReactionData (quickjs.c:52659-52663)
```c
typedef struct JSPromiseReactionData {
    struct list_head link;
    JSValue resolving_funcs[2];  // resolve, reject 函数
    JSValue handler;            // then 传入的回调
} JSPromiseReactionData;
```

### JSPromiseFunctionData (quickjs.c:52649-52658)
```c
typedef struct JSPromiseFunctionDataResolved {
    int ref_count;
    BOOL already_resolved;
} JSPromiseFunctionDataResolved;

typedef struct JSPromiseFunctionData {
    JSValue promise;           // 关联的 promise
    JSPromiseFunctionDataResolved *presolved;
} JSPromiseFunctionData;
```

## Microtask Queue / Job Queue

### JS_EnqueueJob (quickjs.c:1870-1873)
```c
int JS_EnqueueJob(JSContext *ctx, JSJobFunc *job_func,
                  int argc, JSValueConst *argv)
{
    return JS_EnqueueJob2(ctx, job_func, argc, argv, FALSE);
}
```

Job 存储在 `JSRuntime.job_list` 链表，通过 `JS_ExecutePendingJob` 执行。

### promise_reaction_job (quickjs.c:52692-52728)
```c
static JSValue promise_reaction_job(JSContext *ctx, int argc,
                                    JSValueConst *argv)
{
    // argv[0] = resolve_func
    // argv[1] = reject_func
    // argv[2] = handler
    // argv[3] = is_reject
    // argv[4] = value
    
    // 调用 handler(value)
    // 然后调用 resolve_func 或 reject_func
}
```

### fulfill_or_reject_promise (quickjs.c:52745-52778)
```c
static void fulfill_or_reject_promise(JSContext *ctx, JSValueConst promise,
                                      JSValueConst value, BOOL is_reject)
{
    JSPromiseData *s = JS_GetOpaque(promise, JS_CLASS_PROMISE);
    // 设置 promise_state
    // 遍历 promise_reactions[is_reject] 并 enqueue jobs
    // 清空 promise_reactions[1-is_reject]
}
```

## Async Function 实现

### JSAsyncFunctionState (quickjs.c:720-729)
```c
typedef struct JSAsyncFunctionState {
    JSGCObjectHeader header;
    JSValue this_val;
    int argc;
    BOOL throw_flag;          // 控制是否抛出异常
    BOOL is_completed;        // 函数是否完成
    JSValue resolving_funcs[2]; // resolve/reject callbacks
    JSStackFrame frame;
} JSAsyncFunctionState;
```

### js_async_function_call (quickjs.c:20747-20770)
```c
static JSValue js_async_function_call(JSContext *ctx, JSValueConst func_obj,
                                      JSValueConst this_obj, int argc, ...) {
    JSAsyncFunctionState *s = async_func_init(ctx, func_obj, this_obj, argc, argv);
    
    // 创建返回的 Promise
    promise = JS_NewPromiseCapability(ctx, resolving_funcs);
    s->resolving_funcs[0] = resolving_funcs[0];  // resolve
    s->resolving_funcs[1] = resolving_funcs[1];  // reject
    
    // 开始执行
    js_async_function_resume(ctx, s);
    
    async_func_free(ctx->rt, s);
    return promise;
}
```

### js_async_function_resume (quickjs.c:20666-20702)
```c
static void js_async_function_resume(JSContext *ctx, JSAsyncFunctionState *s)
{
    // 调用 async_func_resume 执行
    ret = async_func_resume(ctx, s);
    
    // 如果是 await 触发的恢复，创建 resolving functions
    if (JS_IsException(ret) || JS_IsUndefined(ret)) {
        // 函数完成，resolve/reject promise
        if (js_async_function_resolve_create(ctx, s, resolving_funcs)) {
            // error
        }
        // 调用 resolve 或 reject
    }
}
```

## 状态转换图

### Promise 生命周期
```
new Promise((resolve, reject) => ...)
        │
        ▼
   JS_PROMISE_PENDING
        │
   ┌─────┴─────┐
   ▼           ▼
resolve()    reject()
   │           │
   ▼           ▼
FULFILLED   REJECTED
   │           │
   └─────┬─────┘
         │
         ▼
   enqueue promise_reaction_job
```

### Async Function 生命周期
```
async function foo() {
    await bar();  // OP_await → FUNC_RET_AWAIT → 暂停
}
         │
         ▼
   创建 async function 返回 Promise
         │
         ▼
   async_func_resume() 开始执行
         │
         ▼
   遇到 OP_await → FUNC_RET_AWAIT
         │
         ▼
   暂停，保存栈帧
         │
         ▼
   Promise 完成 → js_async_function_resume()
         │
         ▼
   继续执行或完成
         │
         ▼
   resolve(Promise, returnValue)
```

## AsyncGenerator

### JSAsyncGeneratorData (quickjs.c:20792-20798)
```c
typedef struct JSAsyncGeneratorData {
    JSObject *generator;  // back pointer
    JSAsyncGeneratorStateEnum state;
    JSAsyncFunctionState *func_state;
    struct list_head queue;  // JSAsyncGeneratorRequest 队列
} JSAsyncGeneratorData;
```

### JSAsyncGeneratorRequest (quickjs.c:20771-20779)
```c
typedef struct JSAsyncGeneratorRequest {
    struct list_head link;
    int completion_type;  // GEN_MAGIC_NEXT/RETURN/THROW
    JSValue result;      // 传入的值
    JSValue promise;     // 返回的 promise
    JSValue resolving_funcs[2];
} JSAsyncGeneratorRequest;
```

### 关键机制：请求队列

AsyncGenerator 的核心是请求队列，支持连续调用 `.next()`/`.return()`/`.throw()`：

```c
static void js_async_generator_resume_next(JSContext *ctx,
                                           JSAsyncGeneratorData *s)
{
    for(;;) {
        if (list_empty(&s->queue))
            break;
        next = list_entry(s->queue.next, JSAsyncGeneratorRequest, link);
        
        switch(s->state) {
        case JS_ASYNC_GENERATOR_STATE_EXECUTING:
            // 等待当前执行完成
            goto done;
        case JS_ASYNC_GENERATOR_STATE_SUSPENDED_YIELD:
            // 恢复执行
            ...
        }
    }
done: ;
}
```

## Go 实现建议

### 1. Promise 结构

```go
type Promise struct {
    state    PromiseState
    reactions []Reaction  // 回调链
    result    Value
    handled   bool
}

type Reaction struct {
    resolveFunc Value
    rejectFunc  Value
    handler     Value
}
```

### 2. Microtask 实现

```go
type Context struct {
    // ...
    jobQueue []JobFunc  // 微任务队列
}

func (ctx *Context) EnqueueJob(job JobFunc) {
    ctx.jobQueue = append(ctx.jobQueue, job)
}

func (ctx *Context) ExecutePendingJobs() int {
    count := 0
    for len(ctx.jobQueue) > 0 {
        job := ctx.jobQueue[0]
        ctx.jobQueue = ctx.jobQueue[1:]
        job(ctx)
        count++
    }
    return count
}
```

### 3. 陷阱

1. **循环引用**: Promise reaction 可能形成循环，需要正确标记 is_handled
2. **异步嵌套**: async function 嵌套时需要正确传播 throw_flag
3. **返回 Promise**: async function 必须返回 Promise，不能返回普通值
4. **Generator 共享栈帧**: async_func_state 会在 generator 和 async function 间共享

## 相关 Class IDs

```c
JS_CLASS_PROMISE                    // JSPromiseData
JS_CLASS_PROMISE_RESOLVE_FUNCTION   // JSPromiseFunctionData
JS_CLASS_PROMISE_REJECT_FUNCTION    // JSPromiseFunctionData
JS_CLASS_ASYNC_FUNCTION             // JSAsyncFunctionState
JS_CLASS_ASYNC_FUNCTION_RESOLVE      // JSAsyncFunctionState
JS_CLASS_ASYNC_FUNCTION_REJECT      // JSAsyncFunctionState
JS_CLASS_ASYNC_GENERATOR            // JSAsyncGeneratorData
JS_CLASS_ASYNC_GENERATOR_FUNCTION   // 函数对象
JS_CLASS_ASYNC_FROM_SYNC_ITERATOR   // JSAsyncFromSyncIteratorData
```

## 关键 API

- `JS_PromiseState(ctx, promise)` — 获取 Promise 状态
- `JS_PromiseResult(ctx, promise)` — 获取 Promise 结果
- `JS_EnqueueJob(ctx, job_func, argc, argv)` — 入队 Job
- `JS_IsJobPending(rt)` — 检查是否有待执行 Job
- `JS_ExecutePendingJob(rt, &pctx)` — 执行 Job