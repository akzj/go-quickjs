# 08-compiler — QuickJS Compiler 分析

## 概述

QuickJS 的 Compiler 采用 **Single-Pass 设计**：Parser 在递归下降过程中直接 emit 字节码，无需 AST 中间表示。

## 文件列表

| 文件 | 描述 |
|------|------|
| `bytecode-gen.md` | 字节码生成器设计、Emit 函数、标签系统 |
| `scope.md` | 作用域管理、变量查找、闭包变量捕获 |
| `go-impl.md` | Go Compiler 实现指南 |

## 核心设计

### Bytecode 生成流程

```
js_parse_program()
  ├── next_token()
  ├── js_parse_directives()
  └── while (token != EOF)
        js_parse_source_element()
              ├── emit_op(OP_closure)  // 函数声明
              ├── emit_scope_put_var() // 变量声明
              ├── emit_bin_op()       // 表达式
              └── emit_return()
```

### 字节码结构

```c
typedef struct JSFunctionDef {
    DynBuf byte_code;              // 字节码缓冲区
    JSValue *cpool;               // 常量池
    int cpool_count;
    JSClosureVar *closure_var;     // 闭包变量
    int closure_var_count;
    LabelSlot *label_slots;       // 标签槽
    int label_count;
} JSFunctionDef;
```

### Emit 函数体系

| 函数 | 用途 |
|------|------|
| `emit_u8()` | 写入 1 字节 |
| `emit_u16()` | 写入 2 字节（小端） |
| `emit_u32()` | 写入 4 字节（小端） |
| `emit_op()` | 写入 Opcode |
| `emit_atom()` | 写入 Atom（4 字节） |
| `emit_source_pos()` | 写入行号信息 |
| `emit_label()` | 写入标签 |
| `emit_goto()` | 生成跳转指令 |

### 作用域层级

| Level | 含义 | 字节码 |
|-------|------|--------|
| 0 | 函数作用域 | `OP_scope_get_var`/`OP_scope_put_var` |
| 1+ | 块作用域 | `OP_get_loc`/`OP_set_loc` |
| -1 | 全局 | `OP_scope_get_var` |

### 关键代码位置

| 功能 | 文件:行 |
|------|---------|
| Bytecode 缓冲区 | `quickjs.c:21530-21537` |
| Emit 函数 | `quickjs.c:23265-23320` |
| 标签系统 | `quickjs.c:23335-23400` |
| 常量池 | `quickjs.c:23389-23420` |
| 变量查找 | `quickjs.c:23425-23460` |
| LValue 分析 | `quickjs.c:25361-25502` |
| 作用域定义 | `quickjs.c:21440-21480` |
| Return 生成 | `quickjs.c:27820-27894` |

## Short Opcodes

QuickJS 支持将 1 字节 Opcode 压缩为更短的编码：

```c
#if SHORT_OPCODES
#define short_opcode_info(op) \
    opcode_info[(op) >= OP_TEMP_START ? \
                (op) + (OP_TEMP_END - OP_TEMP_START) : (op)]
#endif
```

## 未分析项

- `js_compile_function()` 完整实现
- Bytecode 序列化/反序列化
- pc2line 表生成算法
- Short opcode 压缩算法
- 函数重载参数处理
- 字节码安全验证
