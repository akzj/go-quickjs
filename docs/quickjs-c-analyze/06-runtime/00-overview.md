# QuickJS Runtime 机制分析

## 概述

QuickJS 的运行时系统由两层核心结构组成：
- **JSRuntime**: 整个引擎的全局状态容器
- **JSContext**: 每个 JavaScript 执行环境的独立上下文

## 核心设计原则

### 1. 单文件架构
- 所有运行时逻辑在 `quickjs.c` 的 60,000 行代码中
- 通过条件编译支持不同平台优化

### 2. 内存管理集成
- Runtime 直接管理 GC 堆
- Context 共享 Runtime 的内存分配器

### 3. 多上下文支持
- 单个 Runtime 可以有多个 Context
- Context 之间通过 Module 系统交互

## 关键文件位置

| 组件 | 行号范围 | 功能 |
|------|----------|------|
| JSRuntime 结构 | 239-326 | 全局状态管理 |
| JSContext 结构 | 448-493 | 执行上下文 |
| JS_NewRuntime | ~1640 | Runtime 创建 |
| JS_NewContext | 2176-2229 | Context 创建 |
| JS_CallInternal | 17372-18750 | 函数调用核心 |

## 初始化流程

```
JS_NewRuntime()
    ↓
JS_InitAtoms()        // 行 2661 - 初始化原子表
JS_InitBuiltinClasses()  // 行 ~1680 - 初始化内置类
    ↓
JS_NewContextRaw(rt)  // 行 2176
    ↓
JS_AddIntrinsicBasicObjects()   // 行 55685 - 基本对象
JS_AddIntrinsicBaseObjects()   // 行 55817 - 基础对象
JS_AddIntrinsicDate()         // Date 对象
JS_AddIntrinsicEval()         // eval 支持
JS_AddIntrinsicRegExp()       // 正则表达式
JS_AddIntrinsicJSON()         // JSON 对象
JS_AddIntrinsicProxy()        // Proxy 对象
JS_AddIntrinsicMapSet()       // Map/Set
JS_AddIntrinsicTypedArrays()  // 类型化数组
JS_AddIntrinsicPromise()      // Promise
JS_AddIntrinsicWeakRef()      // WeakRef
```

## 下一步

- [Context 设计详解](context.md)
- [函数调用机制](function-call.md)
- [作用域链](scope-chain.md)
