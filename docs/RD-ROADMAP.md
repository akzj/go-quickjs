# Go-QuickJS 研发路线图

> 基于 QuickJS C 源码分析，制定的 Go 重写研发计划。

## 📋 项目概述

| 项目 | 描述 |
|------|------|
| 目标 | Go 重写 QuickJS JavaScript 引擎 |
| 参考实现 | quickjs.c (~60,000 行) |
| 分析文档 | docs/quickjs-c-analyze/ (45 个文件, ~17,000 行) |
| 预估工期 | 12-24 人月 |

## 🎯 研发原则

### 核心原则
1. **模块隔离** — 每个模块独立研发、测试、集成
2. **正确性优先** — 不是消灭 bug，是正确实现
3. **对比驱动** — 用工具对比 C/Go 输出，有方向感
4. **不翻译 GC** — 信任 Go 的 GC

### 禁止事项
- ❌ 不要翻译 C 的内存管理 (malloc/free)
- ❌ 不要翻译 C 的 GC (setjmp/longjmp)
- ❌ 不要一次性实现所有功能
- ❌ 不要在底层模块未测试前进行集成

---

## 📦 阶段 1: 基础层 (2-3 周)

### 1.1 pkg/value — JSValue 类型系统

**目标**: 定义 Go 版本的 JS 值类型

**设计决策**:
```
推荐方案: Tagged Interface (idiomatic Go)
─────────────────────────────────────────
type JSValue interface {
    Tag() Tag
}

type IntValue int64
type FloatValue float64
type StringValue *JSString
type ObjectValue *JSObject
type UndefinedValue struct{}
type NullValue struct{}
type BoolValue bool
```

**对比工具**: 值类型转换测试

**验收标准**:
- [ ] JSValue 可以表示所有 JS 类型
- [ ] 类型断言安全
- [ ] ToNumber/ToString/ToBool 实现正确

---

### 1.2 pkg/opcode — Opcode 定义

**目标**: 定义所有 VM opcode 枚举

**文件**: `pkg/opcode/opcode.go`

**内容**:
```go
const (
    OP_INVALID Opcode = iota
    OP_push
    OP_pop
    OP_put_x
    // ... ~100 opcodes
)
```

**验收标准**:
- [ ] opcode 数量与 quickjs-opcode.h 一致
- [ ] 操作数大小正确
- [ ] 栈变化效应记录

---

### 1.3 pkg/vm/core — 最小 VM

**目标**: 能执行最简单字节码的 VM

**最小测试**: `eval("1+1") === 2`

**设计**:
```go
type VM struct {
    ctx     *Context
    frames  *StackFrame  // linked list
    pc      int
    sp      int
    stack   []JSValue
}

func (vm *VM) Run(bytecode *Bytecode) JSValue {
    for {
        op := bytecode.Code[vm.pc]
        vm.pc++
        switch op {
        case OP_push:
            vm.push(vm.readConstant())
        case OP_add:
            rhs := vm.pop()
            lhs := vm.pop()
            vm.push(add(lhs, rhs))
        case OP_return:
            return vm.pop()
        // ...
        }
    }
}
```

**验收标准**:
- [ ] VM 可以加载字节码
- [ ] OP_push, OP_add, OP_return 工作正常
- [ ] `eval("1+1")` 返回 2

---

## 📦 阶段 2: 执行引擎 (2-3 周)

### 2.1 pkg/vm/stack — 栈帧管理

**目标**: 实现函数调用栈

**关键结构**:
```go
type StackFrame struct {
    Prev       *StackFrame
    Function   JSValue
    PC         int
    SP         int
    Locals     []JSValue    // 局部变量
    Args       []JSValue    // 参数
    VarRefs    []*JSVarRef  // 闭包变量引用
}
```

**实现要点**:
- 帧的分配和释放
- 调用约定 (参数传递、返回值)
- 局部变量槽管理

**验收标准**:
- [ ] 函数调用/返回正确
- [ ] 参数正确传递
- [ ] 局部变量正确存取

---

### 2.2 pkg/vm/runtime — 运行时环境

**目标**: JSContext 和 JSRuntime

**关键结构**:
```go
type Runtime struct {
    GCObjects    list.List      // GC 对象链表
    Atoms        AtomTable      // 原子表
    Classes      []ClassDef     // 类定义表
}

type Context struct {
    Runtime      *Runtime
    GlobalObject *JSObject
    Vars         map[string]JSValue  // 全局变量
}
```

**实现要点**:
- Global 对象创建
- 内置对象初始化
- Atoms 管理

**验收标准**:
- [ ] 可以创建 Runtime 和 Context
- [ ] 全局对象存在
- [ ] eval() 可以访问全局变量

---

### 2.3 pkg/vm/builtin — 最小内置对象

**目标**: 实现最基本内置对象

**优先级**:
1. Object.prototype
2. Function.prototype
3. Number, Boolean, String (最小实现)
4. Array.prototype (最小实现)

**验收标准**:
- [ ] `new Object()` 工作
- [ ] `({}).toString()` 返回 "[object Object]"
- [ ] 基本类型转换正确

---

## 📦 阶段 3: 编译器 (4-6 周)

### 3.1 pkg/lexer — 词法分析

**目标**: JS 词法分析器

**关键类型**:
```go
type TokenType int

const (
    TOKEN_EOF TokenType = iota
    TOKEN_IDENT
    TOKEN_NUMBER
    TOKEN_STRING
    // 关键字...
)

type Token struct {
    Type    TokenType
    Value   string
    Line    int
    Column  int
}

type Lexer struct {
    Source   string
    Pos      int
    Line     int
    Column   int
}
```

**对比工具**: `tokencompare`
```bash
# C QuickJS (需要添加 --lex-only 标志)
./qjsc --lex-only test.js > /tmp/c_tokens.txt

# Go QuickJS
go run . -lex-only test.js > /tmp/go_tokens.txt

# diff
diff /tmp/c_tokens.txt /tmp/go_tokens.txt
```

**验收标准**:
- [ ] Token 数量一致
- [ ] Token 类型一致
- [ ] 位置信息一致
- [ ] ASI (自动分号插入) 正确

---

### 3.2 pkg/parser — 语法分析

**目标**: 递归下降解析器

**关键函数**:
```go
func (p *Parser) ParseProgram() *FunctionDef
func (p *Parser) ParseFunction() *FunctionDef
func (p *Parser) ParseStatement() Stmt
func (p *Parser) ParseExpression() Expr
```

**设计原则**:
- Single-pass: 直接 emit 字节码，不生成 AST
- 错误恢复: panic + recover 模式

**验收标准**:
- [ ] 所有 test_language.js 的语句可以解析
- [ ] 错误消息友好
- [ ] 位置信息准确

---

### 3.3 pkg/compiler — 字节码生成

**目标**: JS → 字节码编译器

**关键类型**:
```go
type BytecodeWriter struct {
    Code      []byte
    Constants []JSValue
    Functions []*Bytecode
}

type FunctionDef struct {
    Vars      []string
    Bytecode  *BytecodeWriter
}
```

**对比工具**: `bccompare`
```bash
# C QuickJS
./qjsc -d test.js > /tmp/c_bytecode.txt

# Go QuickJS
go run . -dump test.js > /tmp/go_bytecode.txt

# diff
diff /tmp/c_bytecode.txt /tmp/go_bytecode.txt
```

**验收标准**:
- [ ] 字节码逐字节一致 (最终目标)
- [ ] 运行时行为一致 (最低目标)

---

## 📦 阶段 4: 标准库 (4-6 周)

### 4.1 pkg/builtin/object

**目标**: Object 内置对象完整实现

**关键方法**:
- Object.create()
- Object.keys()
- Object.values()
- Object.entries()
- hasOwnProperty()
- toString()
- valueOf()

**验收标准**:
- [ ] test_builtin.js Object 部分通过

---

### 4.2 pkg/builtin/array

**目标**: Array 内置对象完整实现

**关键方法**:
- push, pop, shift, unshift
- map, filter, reduce
- indexOf, includes
- splice, slice

**验收标准**:
- [ ] test_builtin.js Array 部分通过

---

### 4.3 其他内置对象

**优先级**:
1. String
2. Number
3. Boolean
4. Date
5. RegExp (复用之前的实现)

---

## 📦 阶段 5: 高级特性 (4-8 周)

### 5.1 pkg/coroutine/generator

**目标**: Generator 函数

**关键实现**:
- Generator 对象状态机
- OP_yield 字节码
- 帧状态保存

**验收标准**:
- [ ] `function* g() { yield 1; yield 2; }` 工作

---

### 5.2 pkg/coroutine/promise

**目标**: Promise 和微任务队列

**关键实现**:
- Promise 构造函数
- then/catch/finally
- Job Queue (js_enqueuJob)
- Microtask 执行

**验收标准**:
- [ ] `new Promise()` 工作
- [ ] async/await 基本工作

---

## 🧪 测试策略

### 测试层次

```
┌─────────────────────────────────────────────┐
│  test262 (标准合规性) — 最终验证             │
├─────────────────────────────────────────────┤
│  test_builtin.js — 内置对象                  │
├─────────────────────────────────────────────┤
│  test_closure.js — 闭包                     │
├─────────────────────────────────────────────┤
│  test_language.js — 语言核心                 │
├─────────────────────────────────────────────┤
│  单元测试 — 每个 package                     │
└─────────────────────────────────────────────┘
```

### 测试启用顺序

```
Week 1-2:   eval("1+1")           → 手动测试
Week 3-4:   变量/赋值/运算         → 单元测试
Week 5-6:   控制流 (if/while/for)  → test_language.js 前半
Week 7-8:   函数调用              → test_language.js 完整
Week 9-10:  对象/数组             → test_builtin.js
Week 11-12: 闭包/原型链           → test_closure.js
Week 13+:  标准库完整             → test_builtin.js 完整
```

---

## 📊 进度指标

### 指标定义

| 指标 | 定义 |
|------|------|
| **Opcode 覆盖率** | 已实现 opcode 数 / 总 opcode 数 (100/100) |
| **测试用例通过率** | 通过用例数 / 总用例数 |
| **字节码一致性** | 与 C QuickJS 字节码逐字节一致的比例 |

### 进度看板格式

```
pkg/value     ✅ 100/100 (opcode 覆盖)  ✅ 100% (类型测试)
pkg/opcode    ✅ 100/100                  ✅ 100% (单元测试)
pkg/vm/core   ⏳ 15/100                    ⏳ 3/10 (基本功能)
pkg/vm/stack  ❌ 0/100                     ❌ 0/10
pkg/lexer     ⏳ 50/100                    ⏳ 5/10
...
```

---

## 🔧 工具链建设

### 必须先完成的工具

```bash
# 1. tokencompare — 词法对比
mkdir -p tools/tokencompare
go mod init github.com/.../tools/tokencompare

# 2. bccompare — 字节码对比
mkdir -p tools/bccompare

# 3. disasm — 字节码反汇编
mkdir -p tools/disasm
```

### 工具使用流程

```
1. 修改 Go 代码
2. 运行 tokencompare (如果改动了 lexer)
3. 运行 bccompare (如果改动了 compiler)
4. 运行单元测试
5. 运行集成测试
6. 更新进度看板
```

---

## ⚠️ 陷阱警示

详见 `docs/quickjs-c-analyze/12-go-impl/traps.md`

### 高危陷阱

1. **longjmp 等价** — C 用 setjmp/longjmp，Go 用 panic/recover
2. **内存管理** — 不要翻译 C malloc/free
3. **闭包变量** — 需要正确实现 JSVarRef
4. **Generator 状态** — 需要保存完整帧状态
5. **Unicode 处理** — Go/JS 使用 UTF-8，C 使用 CESU-8

---

## 📝 每日工作流

```
早上:
  1. git pull && go test ./...  (确认绿灯)
  2. 查看进度看板，决定今天目标
  3. 在 C QuickJS 中验证正确行为

工作中:
  4. 修改代码
  5. 运行对比工具
  6. 运行测试
  7. commit (带进度数字)

下班前:
  8. 更新进度看板
  9. 记录阻塞问题
```

---

*版本: v1.0*
*基于分析文档版本: QuickJS 2024-03-24*
