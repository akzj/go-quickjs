# Go 协程实现指导

## 概述

QuickJS 的协程（async/generator）基于状态机 + 栈帧序列化实现。Go 重写时必须理解其核心机制。

## 关键发现：没有 setjmp/longjmp

**重要**: QuickJS 纯用 C 实现，无 setjmp/longjmp。Generator 的"暂停"是通过返回 `FUNC_RET_*` 码让调用者保存状态，下次调用时恢复。

```c
// quickjs.c:17366-17369
#define FUNC_RET_AWAIT         0
#define FUNC_RET_YIELD         1
#define FUNC_RET_YIELD_STAR    2
#define FUNC_RET_INITIAL_YIELD 3
```

## Go 实现方案

### 方案 A: 状态机 + 栈快照

```go
type Generator struct {
    ctx  *Context
    func *Function  // 源函数 bytecode
    
    // 栈帧状态
    pc   int       // 当前指令位置
    sp   int       // 栈指针
    locals []Value  // 局部变量快照
    
    state GeneratorState
}
```

**优点**: 简单，符合 QuickJS 设计
**缺点**: 每次恢复需要重建栈

### 方案 B: Go 协程 + Channel 通信

```go
type Generator struct {
    ctx   *Context
    state GeneratorState
    
    // 用于与 Go 协程通信
    input  chan Value  // .next(value) 传入值
    output chan Value // yield 产出值
    done   chan bool  // 完成信号
}

func (g *Generator) Next(v Value) (Value, bool) {
    g.input <- v
    return <-g.output
}
```

**优点**: 自然利用 Go 协程
**缺点**: 需要把 JS 函数编译为 Go 函数，较复杂

### 方案 C（推荐）: 混合方案

参考 QuickJS 的设计，但用 Go 结构体存储状态：

```go
type Generator struct {
    ctx  *Context
    func *Function
    
    // 核心状态
    state GeneratorState
    
    // 栈帧（作为独立结构）
    frame *GeneratorFrame
    
    // Generator 特有的 yield 缓存
    yieldValue    Value  // 上次 yield 的值
    yieldReceived Value  // 传入 .next() 的值
}

type GeneratorFrame struct {
    pc       int       // bytecode index
    sp       int       // value stack pointer
    locals   []Value   // local variables
    varRefs  []*VarRef // closure variable references
    
    // 父帧（用于嵌套 generator）
    parent *GeneratorFrame
}

// 保存栈帧
func (g *Generator) Suspend() {
    g.frame = &GeneratorFrame{
        pc:     currentPC,
        sp:     currentSP,
        locals: copyLocals(),
        varRefs: copyVarRefs(),
    }
}

// 恢复栈帧
func (g *Generator) Resume(value Value) {
    g.frame.restore()
    // 设置 yieldReceived 到栈顶
}
```

## Async Function 实现

### 核心结构

```go
type AsyncFunction struct {
    ctx  *Context
    func *Function
    
    // Promise 相关
    promise *Promise
    resolveFunc Value
    rejectFunc  Value
    
    // 共享 GeneratorFrame（与 Generator 共用）
    frame *GeneratorFrame
    
    // 状态
    state AsyncState
    
    // 用于 await 时恢复
    throwFlag bool  // 是否抛出异常
}
```

### 执行流程

```go
func AsyncFunctionCall(ctx *Context, funcObj Value, this Value, args []Value) *Promise {
    // 1. 创建 Promise
    promise := NewPromise()
    
    // 2. 初始化 AsyncFunction
    af := &AsyncFunction{
        ctx: ctx,
        func: getFunction(funcObj),
        promise: promise,
        frame: NewGeneratorFrame(),
    }
    
    // 3. 开始执行（不等待）
    go af.resume(nil)
    
    return promise
}

func (af *AsyncFunction) resume(received Value) {
    defer func() {
        if r := recover(); r != nil {
            af.reject(unwrapException(r))
        }
    }()
    
    // 执行字节码，直到遇到 await 或完成
    for {
        ret := af.execute(received)
        
        if isPromise(ret) {
            // await: 等待 Promise 完成
            p := ret.(*Promise)
            p.Then(
                func(v Value) { af.resume(v) },
                func(e Value) { af.reject(e) },
            )
            return
        }
        
        if ret.isReturn() {
            af.resolve(ret.Value())
            return
        }
        
        // ... 其他处理
    }
}
```

## Promise Microtask 实现

```go
type Context struct {
    // ...
    jobQueue []func(*Context) // 微任务队列
    executing bool            // 防止重入
}

// Job 入队
func (ctx *Context) EnqueueJob(job func(*Context)) {
    ctx.jobQueue = append(ctx.jobQueue, job)
    
    // 如果没有在执行，启动执行循环
    if !ctx.executing {
        ctx.executeJobs()
    }
}

// 执行所有 Job（同步）
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

## AsyncGenerator 请求队列

AsyncGenerator 支持并发调用 `.next()`，需要在内部排队：

```go
type AsyncGeneratorRequest struct {
    magic   int     // NEXT/RETURN/THROW
    arg     Value   // 传入参数
    promise *Promise
    resolve func(Value)
    reject  func(Value)
}

type AsyncGenerator struct {
    // ...
    queue   []*AsyncGeneratorRequest
    state   AsyncGeneratorState
    frame   *GeneratorFrame
}

// 队列处理
func (ag *AsyncGenerator) enqueue(req *AsyncGeneratorRequest) {
    ag.queue = append(ag.queue, req)
    if ag.state != EXECUTING {
        ag.processQueue()
    }
}
```

## 陷阱清单

### 1. Generator 共享 JSAsyncFunctionState

Generator 和 Async Function **共享** `JSAsyncFunctionState` 结构！

- `JSAsyncFunctionState.frame` 被 Generator 和 Async Function 共用
- Go 实现时，Generator 和 AsyncFunction **不要** 各自实现帧，而是共享 frame

### 2. close_var_refs 的复杂性

当 Generator 完成时，需要关闭所有闭包引用：

```go
func closeVarRefs(rt *Runtime, bytecode *FunctionBytecode, frame *Frame) {
    for i := 0; i < bytecode.VarRefCount; i++ {
        ref := frame.varRefs[i]
        if ref != nil {
            // 解包：把栈上的值写入 var_ref 对象
            ref.value = frame.locals[ref.index]
        }
    }
}
```

### 3. async generator 的 AWAITING_RETURN 状态

当调用已完成 async generator 的 `.return()` 时：

1. 先 resolve 返回值（`js_async_generator_completed_return`）
2. 等待该 resolve 的 promise 完成
3. 才真正完成

### 4. OP_initial_yield

Async Generator 启动时自动 `OP_initial_yield` 一次，yield 出未定义值。

### 5. yield* 委托传递

`yield*` 需要处理：
- 返回值通过 IteratorResult 传递
- `.throw()` 需要传播到委托的 generator
- `.return()` 需要传播到委托的 generator

### 6. Promise reaction 循环引用

```javascript
let p = new Promise((resolve) => resolve(p));
```

Promise 解析自己会导致循环。`is_handled` 标志用于追踪。

## 性能考虑

1. **栈帧复制**: 每次 yield 需要复制整个栈帧，开销大
2. **对象池**: 复用 GeneratorFrame 对象避免 GC
3. **延迟求值**: 大型数组 yield 时不要立即复制

## 测试用例

```javascript
// generator
function* gen() {
    yield 1;
    yield 2;
    return 3;
}

// async
async function foo() {
    await Promise.resolve(1);
    return 2;
}

// async generator
async function* agen() {
    yield await Promise.resolve(1);
    yield 2;
}

// yield*
function* gen2() {
    yield* gen();
}
```