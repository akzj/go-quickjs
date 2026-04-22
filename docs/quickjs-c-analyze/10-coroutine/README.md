# 10 - 协程模块分析索引

## 模块概述

协程模块包括 Generator、Async Function、Promise 和事件循环。

## 文件清单

| 文件 | 描述 |
|------|------|
| `generator.md` | Generator 实现机制 |
| `async.md` | Async/Promise 实现机制 |
| `go-impl.md` | Go 协程实现建议 |

## 关键发现

### Generator
- 基于 `JSAsyncFunctionState` + 状态机
- 无 setjmp/longjmp，使用返回码机制
- 关键 opcode: `OP_yield`, `OP_yield_star`, `OP_initial_yield`

### Async
- 底层使用 Generator + Promise
- AsyncFunction 调用时返回 Promise
- Job Queue 用于 microtask 执行

### 关键数据结构
- `JSAsyncFunctionState`: 共享栈帧状态
- `JSGeneratorData`: Generator 特有状态
- `JSPromiseData`: Promise 状态和回调链
- `JSAsyncGeneratorData`: AsyncGenerator + 请求队列

### 关键 API
- `JS_EnqueueJob()`: 入队 microtask
- `JS_ExecutePendingJob()`: 执行 pending jobs
- `async_func_resume()`: 恢复协程执行

## 参考

- quickjs.c:720-729 (JSAsyncFunctionState)
- quickjs.c:20379-20414 (async_func_resume)
- quickjs.c:20450-20600 (Generator)
- quickjs.c:52640-52778 (Promise)
- quickjs-opcode.h:212-216 (yield/await opcodes)