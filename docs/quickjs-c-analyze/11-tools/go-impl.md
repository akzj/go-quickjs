# Go 工具实现指导

## 概述

本文档指导如何实现协程模块的对比工具和分析辅助工具。

## 工具架构

```
cmd/
├── bccompare/       // 字节码对比
│   └── main.go
├── tokencompare/    // Token 对比
│   └── main.go
├── vmtrace/         // VM 执行 trace
│   └── main.go
└── testdash/        // 测试看板
    └── main.go
```

## 1. TokenCompare 实现

### 设计目标
- 读取 JavaScript 源文件
- 调用 C QuickJS 和 Go QuickJS 的词法分析
- 对比 token 流输出

### 接口

```go
package main

import (
    "flag"
    "fmt"
    "os"
    "os/exec"
)

func runLexer(binary string, file string) (string, error) {
    cmd := exec.Command(binary, "--lex-only", file)
    out, err := cmd.Output()
    return string(out), err
}

func main() {
    cBinary := flag.String("c", "./qjs", "C QuickJS binary")
    goBinary := flag.String("go", "./go-qjs", "Go QuickJS binary")
    verbose := flag.Bool("v", false, "verbose output")
    flag.Parse()

    for _, file := range flag.Args() {
        cOut, cErr := runLexer(*cBinary, file)
        goOut, goErr := runLexer(*goBinary, file)

        if cErr != nil || goErr != nil {
            fmt.Printf("❌ %s: error\n", file)
            if cErr != nil {
                fmt.Printf("   C error: %v\n", cErr)
            }
            if goErr != nil {
                fmt.Printf("   Go error: %v\n", goErr)
            }
            continue
        }

        if cOut != goOut {
            fmt.Printf("❌ %s: MISMATCH\n", file)
            if *verbose {
                printDiff(cOut, goOut)
            }
        } else {
            fmt.Printf("✓ %s: OK\n", file)
        }
    }
}

func printDiff(a, b string) {
    linesA := strings.Split(a, "\n")
    linesB := strings.Split(b, "\n")
    
    for i := 0; i < max(len(linesA), len(linesB)); i++ {
        lineA := ""
        lineB := ""
        if i < len(linesA) {
            lineA = linesA[i]
        }
        if i < len(linesB) {
            lineB = linesB[i]
        }
        
        if lineA != lineB {
            fmt.Printf("  [%3d] C: %s\n", i+1, lineA)
            fmt.Printf("  [%3d] Go: %s\n", i+1, lineB)
        }
    }
}
```

### C QuickJS 修改需求

需要为 C QuickJS 添加 `--lex-only` 选项：

```c
// quickjs.c 中的 main 函数或添加新选项
if (argc > 1 && strcmp(argv[1], "--lex-only") == 0) {
    // 输出 token 流
    JS_Context *ctx = JS_NewContext(rt);
    while (1) {
        JS_Token tok = js_parse_token(ctx, &buf);
        printf("%s %s\n", token_type_string(tok), token_value_string(tok));
        if (tok == TOKEN_EOF) break;
    }
    exit(0);
}
```

## 2. BytecodeCompare 实现

### 设计目标
- 输出字节码格式兼容
- 支持符号信息（行号、变量名）
- 允许 minor diff（变量重命名等）

### 接口

```go
package main

import (
    "bufio"
    "fmt"
    "os"
    "strconv"
    "strings"
)

type BCInstruction struct {
    PC       int
    Op       string
    Args     []string
    LineInfo string
}

func ParseBC(s string) []BCInstruction {
    var insts []BCInstruction
    scanner := bufio.NewScanner(strings.NewReader(s))
    
    for scanner.Scan() {
        line := scanner.Text()
        // 解析格式: "0000: call 1 ; comment"
        inst := parseLine(line)
        if inst != nil {
            insts = append(insts, *inst)
        }
    }
    return insts
}

func parseLine(line string) *BCInstruction {
    // 移除注释
    if i := strings.Index(line, ";"); i >= 0 {
        line = line[:i]
    }
    
    parts := strings.Split(strings.TrimSpace(line), ":")
    if len(parts) != 2 {
        return nil
    }
    
    pc, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
    args := strings.Fields(parts[1])
    
    return &BCInstruction{
        PC:   pc,
        Op:   args[0],
        Args: args[1:],
    }
}

func CompareBC(c, go_ []BCInstruction) []string {
    var diffs []string
    
    // 对比指令
    for i := 0; i < max(len(c), len(go_)); i++ {
        cInst := BCInstruction{}
        goInst := BCInstruction{}
        
        if i < len(c) {
            cInst = c[i]
        }
        if i < len(go_) {
            goInst = go_[i]
        }
        
        if cInst.Op != goInst.Op {
            diffs = append(diffs, 
                fmt.Sprintf("PC %d: C=%s, Go=%s", cInst.PC, cInst.Op, goInst.Op))
        }
    }
    
    return diffs
}
```

### 字节码格式设计

```
<pc> <opcode> [args...] [; comment]
```

示例：
```
0000: call 1                ; 调用函数，1 个参数
0003: pushc 42              ; 压入常数 42
0007: add                  ; 加法
0009: return               ; 返回
```

## 3. VMTrace 实现

### 设计目标
- 逐指令 trace VM 执行
- 显示寄存器、栈、内存状态
- 支持时间/调用深度信息

### 接口

```go
type VMState struct {
    PC    int
    SP    int
    Stack []Value
    Acc   Value  // accumulator
    Env   *Env   // lexical environment
    VarRefs []*Value
}

type Trace struct {
    Inst   *BCInstruction
    Before VMState
    After  VMState
}

func (ctx *Context) ExecuteWithTrace(startPC int) ([]Trace, error) {
    var traces []Trace
    
    for {
        // 记录执行前状态
        before := VMState{
            PC:    ctx.pc,
            SP:    ctx.sp,
            Stack: copyStack(ctx.stack[:ctx.sp]),
            Acc:   ctx.acc,
        }
        
        // 执行指令
        inst := ctx.bytecode[ctx.pc]
        err := ctx.executeInst(inst)
        
        // 记录执行后状态
        after := VMState{
            PC:    ctx.pc,
            SP:    ctx.sp,
            Stack: copyStack(ctx.stack[:ctx.sp]),
            Acc:   ctx.acc,
        }
        
        traces = append(traces, Trace{
            Inst:   inst,
            Before: before,
            After:  after,
        })
        
        if err != nil || ctx.pc >= len(ctx.bytecode) {
            return traces, err
        }
    }
}

func PrintTrace(traces []Trace) string {
    var b strings.Builder
    
    for _, t := range traces {
        fmt.Fprintf(&b, "%04d: %s\n", t.Inst.PC, t.Inst.Op)
        fmt.Fprintf(&b, "  stack: %v\n", formatStack(t.After.Stack))
        if t.After.Acc != nil {
            fmt.Fprintf(&b, "  acc: %v\n", t.After.Acc)
        }
    }
    
    return b.String()
}
```

### 输出格式

```
PC    OP         STACK          ACC
0000: call 1     []             undefined
0003: pushc 42   [42]           42
0007: add        []              42
```

## 4. TestDash 实现

### 设计目标
- 显示测试进度
- 失败用例统计
- 分类清晰

### 接口

```go
type Module struct {
    Name     string
    Pass     int
    Fail     int
    Skip     int
    Failures []string
}

type Dashboard struct {
    Modules []Module
}

func (d *Dashboard) Render() string {
    var b strings.Builder
    
    // 标题
    fmt.Fprintln(&b, "┌──────────────────────────────────────────┐")
    fmt.Fprintln(&b, "│  QuickJS Go 重写 - 测试进度              │")
    fmt.Fprintln(&b, "├──────────────────────────────────────────┤")
    
    // 模块进度
    totalPass := 0
    totalFail := 0
    total := 0
    
    for _, m := range d.Modules {
        pass := m.Pass
        fail := m.Fail
        skip := m.Skip
        n := pass + fail + skip
        total += n
        totalPass += pass
        totalFail += fail
        
        pct := float64(pass) / float64(n) * 100
        bar := renderBar(pass, n)
        
        fmt.Fprintf(&b, "│  %-12s %s %3d%% (%d/%d)", m.Name, bar, int(pct), pass, n)
        if fail > 0 {
            fmt.Fprintf(&b, " [FAIL: %d]", fail)
        }
        fmt.Fprintln(&b, " │")
    }
    
    // 总进度
    totalPct := float64(totalPass) / float64(total) * 100
    totalBar := renderBar(totalPass, total)
    fmt.Fprintln(&b, "├──────────────────────────────────────────┤")
    fmt.Fprintf(&b, "│  总进度: %s %3d%% (%d/%d)", totalBar, int(totalPct), totalPass, total)
    fmt.Fprintln(&b, "         │")
    fmt.Fprintln(&b, "└──────────────────────────────────────────┘")
    
    return b.String()
}

func renderBar(n, total int) string {
    width := 10
    filled := int(float64(n) / float64(total) * float64(width))
    
    bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
    return bar
}
```

## 5. 集成测试

### 测试套件结构

```
tests/
├── lexer/
│   ├── basic/
│   │   ├── identifiers.js
│   │   ├── keywords.js
│   │   └── operators.js
│   └── expected/
│       └── c_output.txt
├── compiler/
│   ├── basic_expressions.js
│   ├── functions.js
│   ├── closures.js
│   └── expected/
├── vm/
│   ├── arithmetic/
│   ├── control/
│   └── functions/
└── coroutine/
    ├── generators/
    ├── async/
    └── expected/
```

### 测试运行器

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

type TestRunner struct {
    TestDir string
    CBinary string
    GoBinary string
}

func (r *TestRunner) RunAll() (int, int) {
    var passed, failed int
    
    modules := []string{"lexer", "compiler", "vm", "coroutine"}
    
    for _, mod := range modules {
        modDir := filepath.Join(r.TestDir, mod)
        if _, err := os.Stat(modDir); os.IsNotExist(err) {
            continue
        }
        
        p, f := r.runModule(mod, modDir)
        passed += p
        failed += f
        
        fmt.Printf("%s: %d passed, %d failed\n", mod, p, f)
    }
    
    return passed, failed
}

func (r *TestRunner) runModule(name, dir string) (int, int) {
    var passed, failed int
    
    err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        if info.IsDir() || !strings.HasSuffix(path, ".js") {
            return nil
        }
        
        if r.runTest(name, path) {
            passed++
        } else {
            failed++
        }
        
        return nil
    })
    
    if err != nil {
        fmt.Printf("Error walking %s: %v\n", dir, err)
    }
    
    return passed, failed
}

func (r *TestRunner) runTest(module, file string) bool {
    // 对比 C 和 Go 输出
    return true
}
```

## 6. CI 集成

### GitHub Actions

```yaml
name: Compare Tests
on: [push, pull_request]

jobs:
  compare:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Build C QuickJS
        run: |
          cd quickjs-master
          make
      
      - name: Build Go QuickJS
        run: go build -o go-qjs ./...
      
      - name: Run Token Compare
        run: go run ./cmd/tokencompare -c ./qjs -go ./go-qjs ./tests/lexer/
      
      - name: Run Bytecode Compare
        run: go run ./cmd/bccompare -c ./qjs -go ./go-qjs ./tests/compiler/
      
      - name: Run VM Trace
        if: failure()
        run: go run ./cmd/vmtrace -c ./qjs -go ./go-qjs -v ./tests/vm/
```

## 实现优先级

1. **TokenCompare** — 词法分析简单，先实现验证流程
2. **BytecodeCompare** — 最关键，验证编译正确性
3. **TestDash** — 进度可视化，后期实现
4. **VMTrace** — 调试 VM 执行问题，需要时实现