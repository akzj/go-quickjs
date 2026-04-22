# 03-gc / 垃圾回收分析

## 概述

本模块分析 QuickJS 的垃圾回收机制，包括引用计数、标记-清除、循环检测。

## 文件索引

| 文件 | 内容 |
|------|------|
| [algorithm.md](algorithm.md) | GC 算法详细分析、代码追踪 |
| [go-impl.md](go-impl.md) | Go 语言 GC 对应方案 |

## 核心要点

### 1. 混合 GC 架构

QuickJS 使用 **引用计数 + 标记-清除** 混合算法：

```
引用计数 (即时释放)     标记-清除 (循环检测)
       │                      │
       ▼                      ▼
   大部分对象            循环引用对象
   的即时释放             的检测释放
```

### 2. 引用计数机制

- `JS_VALUE_HAS_REF_COUNT()` 宏判断是否需要 RC
- 只有负数 tag (heap 对象) 才需要引用计数
- 立即数 (int, bool, null, undefined) 无 RC 开销

### 3. GC 三阶段

1. **gc_decref**: 递减所有子对象引用计数
2. **gc_scan**: 扫描并恢复可达对象的计数
3. **gc_free_cycles**: 释放循环垃圾对象

### 4. GC 对象类型

| 类型 | 说明 |
|------|------|
| JS_GC_OBJ_TYPE_JS_OBJECT | JavaScript 对象 |
| JS_GC_OBJ_TYPE_FUNCTION_BYTECODE | 函数字节码 |
| JS_GC_OBJ_TYPE_SHAPE | 对象形状 |
| JS_GC_OBJ_TYPE_VAR_REF | 变量引用 |
| JS_GC_OBJ_TYPE_ASYNC_FUNCTION | 异步函数 |
| JS_GC_OBJ_TYPE_JS_CONTEXT | JS 上下文 |
| JS_GC_OBJ_TYPE_MODULE | 模块 |

### 5. Go 实现关键洞察

**不要翻译 C 的 GC！** Go 有自己的垃圾回收器。

正确思路：
- Go runtime 管理内存
- 实现 JS 语义层面的引用计数 (用于 FinalizerRegistry 和 WeakRef)
- 使用 runtime.SetFinalizer 触发 JS finalizer
- 实现循环检测以正确调用 JS 层 finalizer

## 关键代码位置

| 功能 | quickjs.c |
|------|----------|
| JSGCObjectHeader | 355-362 |
| GC 对象类型 | 344-350 |
| GC 阶段枚举 | 232-235 |
| JS_DupValue | quickjs.h:698-706 |
| __JS_FreeValueRT | 6027-6098 |
| JS_RunGC | 6410-6432 |
| gc_decref | 6282-6312 |
| gc_scan | 6315-6350 |
| gc_free_cycles | 6352-6408 |
| mark_children | 6163-6278 |