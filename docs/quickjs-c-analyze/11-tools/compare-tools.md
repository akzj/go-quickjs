# 对比工具设计

## 概述

根据 METHODOLOGY.md 的"对比驱动开发 (BDD)"原则，每个模块必须有对比工具。这些工具用于验证 Go 重写与 C QuickJS 的行为一致性。

## 工具清单

### 1. TokenCompare — 词法分析对比

**目的**: 验证词法分析器输出完全一致

**输入**: JavaScript 源代码文件

**输出对比**:
```bash
# C QuickJS 词法输出（需修改 quickjs.c 添加调试输出）
$ ./qjs --lex-only test.js > c_tokens.txt

# Go QuickJS 词法输出
$ go run . -lex test.js > go_tokens.txt

# diff
$ diff c_tokens.txt go_tokens.txt
```

**Go 实现**:

```go
// cmd/tokencompare/main.go
package main

import (
    "flag"
    "fmt"
    "os"
    "os/exec"
    "strings"
)

func main() {
    cBinary := flag.String("c", "./qjs", "C QuickJS binary")
    goBinary := flag.String("go", "./go-qjs", "Go QuickJS binary")
    flag.Parse()
    
    files := flag.Args()
    if len(files) == 0 {
        flag.Usage()
        os.Exit(1)
    }
    
    for _, file := range files {
        cOut, cErr := exec.Command(*cBinary, "--lex-only", file).Output()
        goOut, goErr := exec.Command(*goBinary, "-lex", file).Output()
        
        if cErr != nil || goErr != nil {
            fmt.Printf("FAIL: %s\n", file)
            if cErr != nil {
                fmt.Printf("  C error: %v\n", cErr)
            }
            if goErr != nil {
                fmt.Printf("  Go error: %v\n", goErr)
            }
            continue
        }
        
        if string(cOut) != string(goOut) {
            fmt.Printf("FAIL: %s\n", file)
            fmt.Printf("Diff:\n%s\n", diff(string(cOut), string(goOut)))
        } else {
            fmt.Printf("PASS: %s\n", file)
        }
    }
}
```

**需要 C QuickJS 修改**:
- 添加 `--lex-only` 选项输出 tokens
- 输出格式应与 Go 版本完全一致

### 2. BytecodeCompare — 字节码对比

**目的**: 验证字节码编译结果一致

**输入**: JavaScript 源代码文件

**输出对比**:
```bash
# C QuickJS 字节码输出（带调试信息）
$ ./qjs -d test.js > c_bc.txt

# Go QuickJS 字节码输出
$ go run . -dump test.js > go_bc.txt

# diff
$ diff c_bc.txt go_bc.txt
```

**字节码格式设计**:

```
// 每条指令格式
<pc>: <opcode> [args...]
```

示例:
```
0000: call 1          ; 调用 1 个参数
0003: push 42         ; 压入常数 42
0006: add             ; 加法
0007: return          ; 返回
```

**Go 实现**:

```go
// cmd/bccompare/main.go
type BCInstruction struct {
    PC   int
    Op   string
    Args []string
    Comment string
}

func (i *BCInstruction) String() string {
    s := fmt.Sprintf("%04d: %s", i.PC, i.Op)
    if len(i.Args) > 0 {
        s += " " + strings.Join(i.Args, ", ")
    }
    if i.Comment != "" {
        s += " ; " + i.Comment
    }
    return s
}
```

### 3. VMTrace — 运行时 Trace

**目的**: 逐指令对比 VM 执行过程

**功能**:
- 记录每条指令执行后的状态
- 包括: PC, SP, 栈内容, 寄存器
- 对比两个实现的结果差异

**输出格式**:
```
[pc=0003] push 42
  sp=2 -> [undefined, undefined, 42]
  accumulator=42

[pc=0006] add
  sp=1 -> [undefined, 44]
  accumulator=44
```

**Go 实现**:

```go
// cmd/vmtrace/main.go
type VMState struct {
    PC    int
    SP    int
    Stack []Value
    Acc   Value  // accumulator
}

func (ctx *Context) ExecuteWithTrace(pc int) (Value, []VMState) {
    var trace []VMState
    
    for {
        op := ctx.readOp(pc)
        
        state := VMState{
            PC:    pc,
            SP:    ctx.sp,
            Stack: ctx.stack[:ctx.sp],
            Acc:   ctx.acc,
        }
        trace = append(trace, state)
        
        ret, newPC := ctx.executeOp(pc, op)
        pc = newPC
        
        if ret != nil {
            return ret, trace
        }
    }
}
```

### 4. TestDash — 测试进度看板

**目的**: 可视化显示测试进度

**输出**:
```
┌─────────────────────────────────────────┐
│  QuickJS Go 重写 - 测试进度              │
├─────────────────────────────────────────┤
│  Parser      [████████░░] 80% (40/50)   │
│  Compiler    [██████░░░░] 60% (30/50)   │
│  VM          [████░░░░░░] 40% (20/50)   │
│  Generator   [██░░░░░░░░] 20% (10/50)   │
│  Async       [░░░░░░░░░░]  0% (0/50)    │
├─────────────────────────────────────────┤
│  总进度: [██████░░░░] 40% (100/250)     │
│  失败: 5                                  │
└─────────────────────────────────────────┘
```

**Go 实现**:

```go
// cmd/testdash/main.go
type Module struct {
    Name     string
    Pass     int
    Total    int
    Failures []string
}

func RenderDashboard(modules []Module) string {
    var b strings.Builder
    // 绘制表格...
    return b.String()
}
```

### 5. ASTCompare — AST 对比

**目的**: 对比语法分析结果（用于调试 parser）

```bash
$ ./qjs --ast test.js > c_ast.json
$ go run . -ast test.js > go_ast.json
$ diff c_ast.json go_ast.json
```

## 测试套件组织

```
tests/
├── lexer/
│   ├── basic_tokens.js
│   ├── keywords.js
│   ├── operators.js
│   └── expected/  # C QuickJS 输出
├── parser/
│   ├── expressions.js
│   ├── functions.js
│   └── expected/
├── compiler/
│   ├── bytecode/
│   └── expected/
├── vm/
│   ├── arithmetic/
│   ├── functions/
│   └── expected/
└── coroutine/
    ├── generators/
    ├── async/
    └── expected/
```

## 自动化脚本

```bash
#!/bin/bash
# run_compare.sh

set -e

C_QJS="./quickjs-master/qjs"
GO_QJS="./go-qjs"

echo "=== Token Compare ==="
for f in tests/lexer/*.js; do
    $C_QJS --lex-only "$f" > /tmp/c_tokens.txt
    $GO_QJS -lex "$f" > /tmp/go_tokens.txt
    if ! diff -q /tmp/c_tokens.txt /tmp/go_tokens.txt; then
        echo "FAIL: $f"
    fi
done

echo "=== Bytecode Compare ==="
for f in tests/compiler/*.js; do
    $C_QJS -d "$f" > /tmp/c_bc.txt
    $GO_QJS -dump "$f" > /tmp/go_bc.txt
    if ! diff -q /tmp/c_bc.txt /tmp/go_bc.txt; then
        echo "FAIL: $f"
    fi
done
```

## C QuickJS 调试支持

需要修改 C QuickJS 添加调试选项：

```c
// quickjs.c - 添加选项
typedef struct {
    BOOL lex_debug;
    BOOL parse_debug;
    BOOL bc_debug;
    BOOL exec_debug;
    BOOL vm_trace;
} JSDebugFlags;

// 设置方式
if (ctx->debug_flags.lex_debug) {
    printf("TOKEN: %s\n", token_str);
}
```

## 实现优先级

1. **BytecodeCompare** — 最关键，验证编译正确性
2. **TokenCompare** — 词法分析简单，先实现
3. **VMTrace** — 调试 VM 执行问题
4. **TestDash** — 进度可视化，后期实现
5. **ASTCompare** — 可选，用于详细 parser 调试