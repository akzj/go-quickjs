# 02-value-model / 值模型分析

## 概述

本模块分析 QuickJS 的值模型设计，包括 JSValue 类型系统、类型转换机制。

## 文件索引

| 文件 | 内容 |
|------|------|
| [jsvalue.md](jsvalue.md) | JSValue 核心类型设计、NaN Boxing、Tag 系统 |
| [conversion.md](conversion.md) | ToNumber/ToString/ToBool/ToPrimitive 类型转换 |
| [go-impl.md](go-impl.md) | Go 语言实现建议 |

## 核心要点

### 1. JSValue 设计

QuickJS 使用 64 位整数存储所有 JavaScript 值，通过 tag 区分类型。

**NaN Boxing**:
- Float64 通过 NaN 编码存储在 JSValue 中
- 其他类型通过 tag + payload 组合

**Tag 分类**:
- 负数 tag: Heap 对象引用 (需要 GC)
- 0-8: 立即数 (无堆分配)
- JS_TAG_FLOAT64: 堆分配浮点数

### 2. 类型转换

完整的 ES 规范类型转换实现：
- ToPrimitive: 调用 valueOf/toString
- ToNumber/ToInteger/ToInt32/ToUint32/ToFloat64
- ToString: 特殊值处理 (NaN, Infinity, -0)
- ToBoolean: 对象总是 true

### 3. Go 实现建议

- 推荐使用 Tagged Union 而非 NaN Boxing
- 利用 Go 的 interface{} 或自定义类型
- 避免 unsafe.Pointer 操作浮点位
- Float64 优化: 小数用 int 表示

## 关键代码位置

| 功能 | quickjs.h | quickjs.c |
|------|----------|----------|
| Tag 定义 | 77-93 | - |
| JSValue (NaN) | 146-158 | - |
| Float64 编码 | 152-182 | - |
| 工厂函数 | 550-606 | - |
| 类型判断 | 611-661 | - |
| 引用计数宏 | 284 | - |