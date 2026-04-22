# 11 - 工具链模块分析索引

## 模块概述

工具链模块包括对比工具、调试辅助和测试基础设施。

## 文件清单

| 文件 | 描述 |
|------|------|
| `compare-tools.md` | 对比工具设计（Token、Bytecode、VM Trace） |
| `go-impl.md` | Go 工具实现指导 |
| `README.md` | 本模块索引 |

## 对比工具清单

| 工具 | 用途 | 优先级 |
|------|------|--------|
| TokenCompare | 词法分析输出对比 | 高 |
| BytecodeCompare | 字节码编译对比 | 高 |
| VMTrace | VM 执行 trace | 中 |
| TestDash | 测试进度可视化 | 低 |

## 关键设计点

### 1. 工具需与 C QuickJS 配合
- C QuickJS 需要添加 `--lex-only`、`-d` 等调试选项
- 输出格式必须与 Go 版本一致

### 2. 测试套件组织
```
tests/
├── lexer/
├── compiler/
├── vm/
└── coroutine/
```

### 3. CI 集成
- GitHub Actions 运行对比测试
- 失败时自动生成详细 diff

## 参考

- `quickjs-master/examples/` — C QuickJS 示例
- METHODOLOGY.md — BDD 原则