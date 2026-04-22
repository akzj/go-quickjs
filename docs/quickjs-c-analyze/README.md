# QuickJS C 源码分析文档

> 本目录包含对 QuickJS C 源码的完整分析，为 Go 重写提供技术指导。

## 📊 文档统计

| 指标 | 数值 |
|------|------|
| 文档文件数 | 45 个 |
| 总行数 | ~17,000 行 |
| 分析模块 | 12 个 |

## 📁 模块索引

### 核心引擎

| 模块 | 目录 | 文件数 | 核心内容 |
|------|------|--------|----------|
| **架构设计** | 01-architecture/ | 3 | 模块边界、数据流、Go package 映射 |
| **值模型** | 02-value-model/ | 4 | JSValue NaN-boxing、类型转换 |
| **GC** | 03-gc/ | 3 | 引用计数+标记清除算法 |
| **指令集** | 04-opcodes/ | 3 | ~100 opcode 枚举和语义 |
| **VM** | 05-vm/ | 4 | 解释器、栈管理、函数调用 |
| **运行时** | 06-runtime/ | 5 | JSContext、作用域链、函数调用 |

### 编译前端

| 模块 | 目录 | 文件数 | 核心内容 |
|------|------|--------|----------|
| **Parser** | 07-parser/ | 4 | Token、词法分析、递归下降解析 |
| **Compiler** | 08-compiler/ | 4 | 字节码生成、作用域管理 |

### 标准库和高级特性

| 模块 | 目录 | 文件数 | 核心内容 |
|------|------|--------|----------|
| **内置对象** | 09-builtin/ | 6 | Object/Array/String 等实现 |
| **协程** | 10-coroutine/ | 4 | Generator、Async/Promise、Job Queue |

### 工程实践

| 模块 | 目录 | 文件数 | 核心内容 |
|------|------|--------|----------|
| **工具链** | 11-tools/ | 3 | 对比工具设计 (bccompare, tokencompare) |
| **Go 实现指导** | 12-go-impl/ | 2 | 26 个陷阱、注意事项 |

## 🎯 Go 重写研发路线图

```
阶段 1: 基础层 (2-3 周)
├── pkg/value — JSValue 类型、类型转换
├── pkg/opcode — Opcode 枚举定义
└── pkg/vm/core — 最小 VM (解释器循环)

阶段 2: 执行引擎 (2-3 周)
├── pkg/vm/stack — 栈帧管理
├── pkg/vm/runtime — JSContext、JSRuntime
└── pkg/vm/builtin — 最小内置对象

阶段 3: 编译器 (4-6 周)
├── pkg/lexer — 词法分析
├── pkg/parser — 递归下降解析
└── pkg/compiler — 字节码生成

阶段 4: 标准库 (4-6 周)
├── pkg/builtin/object
├── pkg/builtin/array
├── pkg/builtin/string
└── pkg/builtin/function

阶段 5: 高级特性 (4-8 周)
├── pkg/coroutine/generator
├── pkg/coroutine/promise
└── pkg/coroutine/async
```

## 🔧 工具链

### 必需工具 (开发驱动)
1. **tokencompare** — 对比 C/Go 词法输出
2. **bccompare** — 对比 C/Go 字节码输出
3. **vmtrace** — 运行时 trace 对比

### 可选工具 (效率提升)
4. **testdash** — 测试进度看板
5. **disasm** — 字节码反汇编

## ⚠️ 重要原则

1. **模块隔离** — 每个模块独立研发、测试
2. **正确性优先** — 不是消灭 bug，是正确实现
3. **对比驱动** — 用工具对比 C/Go 输出，有方向感
4. **不翻译 GC** — 信任 Go 的 GC

## 📖 阅读顺序建议

1. **新手入门**: 01-architecture → 02-value-model → 04-opcodes
2. **VM 研发**: 05-vm → 06-runtime → 04-opcodes
3. **Parser 研发**: 07-parser → 08-compiler → 04-opcodes
4. **高级特性**: 10-coroutine → 09-builtin → 12-go-impl

---

*分析版本: QuickJS 2024-03-24*
*文档版本: v1.0*
