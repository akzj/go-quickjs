# 07-parser — QuickJS Parser 分析

## 概述

QuickJS 的 Parser 采用 **无 AST 的 Single-Pass 设计**：在递归下降解析过程中直接 emit 字节码，不需要显式的 AST 数据结构。

## 文件列表

| 文件 | 描述 |
|------|------|
| `token.md` | Token 类型定义、词法分析器实现 |
| `parser.md` | 语法分析器设计、表达式/语句解析 |
| `go-impl.md` | Go Parser 实现指南 |

## 核心设计

### Token 系统

- Token 值范围：`-128 ~ -1`
  - `-128 ~ -98`: 字面量和标识符
  - `-97 ~ -65`: 运算符/分隔符
  - `-64 ~ -1`: 关键字
- **关键字顺序与 Atom 枚举顺序严格对应**

### 解析器架构

```
js_parse_program()
  ├── next_token() — 获取第一个 Token
  ├── js_parse_directives() — "use strict"
  ├── js_parse_source_element() — 顶层语句
  │     ├── js_parse_function_decl() — 函数声明
  │     ├── js_parse_export() — export
  │     ├── js_parse_import() — import
  │     └── js_parse_statement_or_decl() — 语句/声明
  └── emit_return() — 生成 return 语句
```

### 表达式解析层次

| 层级 | 函数 | 运算符 |
|------|------|--------|
| 0 | `js_parse_unary()` | `+`, `-`, `!`, `~`, `typeof`, `delete`, `await` |
| 1-8 | `js_parse_expr_binary()` | `*`, `/`, `+`, `-`, `<<`, `>>`, `<`, `>`, `==`, `&`, `^`, `\|` |
| - | `js_parse_logical_and_or()` | `&&`, `\|\|` |
| - | `js_parse_coalesce_expr()` | `??` |
| - | `js_parse_cond_expr()` | `? :` |
| - | `js_parse_assign_expr2()` | `=`, `+=`, `-=`, ... |

### 关键代码位置

| 功能 | 文件:行 |
|------|---------|
| Token 枚举 | `quickjs.c:21219-21277` |
| Token 结构 | `quickjs.c:21539-21560` |
| Parser State | `quickjs.c:21562-21575` |
| 词法分析器 | `quickjs.c:22257-23226` |
| 关键字解析 | `quickjs.c:22380-22475` |
| 数字解析 | `quickjs.c:22477-22679` |
| 字符串解析 | `quickjs.c:21889-22020` |
| 程序入口 | `quickjs.c:36496-36537` |
| 函数解析 | `quickjs.c:36486-36500` |

## LValue 处理

QuickJS 通过分析上一个 emit 的 opcode 来确定赋值目标类型：

| 上一个 Opcode | LValue 类型 | Depth |
|--------------|------------|-------|
| `OP_scope_get_var` | 局部/全局变量 | 0 |
| `OP_get_field` | 对象属性 | 1 |
| `OP_get_array_el` | 数组元素 | 2 |
| `OP_get_super_value` | super 属性 | 3 |

## 未分析项

- JSON 解析模式 (`ext_json`)
- `js_parse_delete()` 完整实现
- `js_parse_class()` 完整实现
- 模块解析 (`js_parse_import/export`)
- 错误恢复机制
