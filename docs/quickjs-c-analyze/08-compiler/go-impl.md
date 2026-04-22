# Go Compiler 实现指南

## 1. 架构设计

### 1.1 三层架构

```
┌─────────────────────────────────────────────┐
│              Go Parser                       │
│  (AST → AST 节点)                            │
├─────────────────────────────────────────────┤
│           AST Visitor                         │
│  (遍历 AST，生成字节码)                       │
├─────────────────────────────────────────────┤
│         Bytecode Writer                      │
│  (Emit 字节码到缓冲区)                        │
└─────────────────────────────────────────────┘
```

### 1.2 核心接口

```go
// 字节码写入器
type BytecodeWriter interface {
    EmitOp(op Opcode)
    EmitU8(v uint8)
    EmitU16(v uint16)
    EmitU32(v uint32)
    EmitAtom(atom Atom)
    EmitLabel() int          // 返回标签 ID
    BindLabel(id int)        // 绑定标签到当前位置
}

// 编译器接口
type Compiler interface {
    CompileProgram(ast *Program) (*FunctionDef, error)
    CompileFunc(fn *FuncDecl) (*FunctionDef, error)
}

// AST 到字节码的遍历器
type ASTCompiler struct {
    writer BytecodeWriter
    ctx    *Context
    fd     *FunctionDef     // 当前函数
}
```

---

## 2. FunctionDef 数据结构

### 2.1 Go 定义

```go
type FunctionDef struct {
    // 上下文
    Context *Context
    
    // 函数信息
    Name      Atom
    IsEval    bool
    IsModule  bool
    FuncKind  FuncKind      // JS_FUNC_NORMAL, JS_FUNC_ASYNC, etc.
    FuncType  FuncType      // JS_PARSE_FUNC_STATEMENT, EXPR, ARROW, etc.
    
    // 参数
    Args      []*VarDef
    ArgCount  int
    
    // 变量
    Vars      []*VarDef
    VarCount  int
    
    // 作用域
    ScopeLevel int
    ScopeFirst int
    
    // 字节码
    Bytecode *bytes.Buffer
    
    // 常量池
    ConstPool []Value
    CPoolCount int
    
    // 闭包变量
    ClosureVars []*ClosureVar
    
    // 标签
    Labels     []*LabelSlot
    LabelCount int
    
    // 嵌套函数
    Children []*FunctionDef
    
    // 调试信息
    Source []byte
    Pc2Line []byte
}
```

### 2.2 变量定义

```go
type VarDef struct {
    Name      Atom
    ScopeLevel int
    Kind      VarKind    // VAR, LET, CONST, ARG
    Index     int        // 栈帧偏移或作用域索引
    IsLocal   bool
}

type VarKind uint8

const (
    VAR_VAR VarKind = iota
    VAR_LET
    VAR_CONST
    VAR_ARG
)
```

---

## 3. 字节码生成实现

### 3.1 基础 Emit 函数

```go
type SimpleWriter struct {
    buf *bytes.Buffer
}

func (w *SimpleWriter) EmitOp(op Opcode) {
    w.buf.WriteByte(byte(op))
}

func (w *SimpleWriter) EmitU8(v uint8) {
    w.buf.WriteByte(v)
}

func (w *SimpleWriter) EmitU16(v uint16) {
    binary.LittleEndian.PutUint16(w.buf.Write(make([]byte, 2)), v)
}

func (w *SimpleWriter) EmitU32(v uint32) {
    binary.LittleEndian.PutUint32(w.buf.Write(make([]byte, 4)), v)
}

func (w *SimpleWriter) EmitAtom(atom Atom) {
    w.EmitU32(uint32(atom))
}
```

### 3.2 标签系统

```go
type LabelManager struct {
    labels   []*LabelSlot      // 已定义的标签
    pending  []*PendingLabel   // 待绑定的跳转
    buf      *bytes.Buffer
}

type LabelSlot struct {
    Pos    int    // 标签位置（-1 = 未绑定）
    Refs   []int   // 引用此标签的偏移量
}

type PendingLabel struct {
    Pos    int     // 跳转指令位置
    Label  int     // 目标标签 ID
}

func (w *LabelManager) EmitGoto(op Opcode, target int) {
    // 记录跳转位置
    w.pending = append(w.pending, &PendingLabel{
        Pos:   w.buf.Len(),
        Label: target,
    })
    // 写入占位符
    w.EmitOp(op)
    w.EmitU32(0)  // 占位
}

func (w *LabelManager) NewLabel() int {
    id := len(w.labels)
    w.labels = append(w.labels, &LabelSlot{Pos: -1})
    return id
}

func (w *LabelManager) BindLabel(id int) {
    slot := w.labels[id]
    slot.Pos = w.buf.Len()
    
    // 回填所有待定的引用
    for _, p := range w.pending {
        if p.Label == id {
            binary.LittleEndian.PutUint32(
                w.buf.Bytes()[p.Pos+1:p.Pos+5],
                uint32(slot.Pos),
            )
        }
    }
}
```

---

## 4. 表达式编译

### 4.1 二元表达式

```go
func (c *ASTCompiler) visitBinaryExpr(e *BinaryExpr) {
    c.visitExpr(e.Left)
    c.visitExpr(e.Right)
    
    switch e.Op {
    case '+':
        c.EmitOp(OP_add)
    case '-':
        c.EmitOp(OP_sub)
    case '*':
        c.EmitOp(OP_mul)
    case '/':
        c.EmitOp(OP_div)
    case '%':
        c.EmitOp(OP_mod)
    case '<':
        c.EmitOp(OP_lt)
    case TOK_SHL:
        c.EmitOp(OP_shl)
    case TOK_SAR:
        c.EmitOp(OP_sar)
    case TOK_SHR:
        c.EmitOp(OP_shr)
    case TOK_EQ:
        c.EmitOp(OP_eq)
    case TOK_STRICT_EQ:
        c.EmitOp(OP_strict_eq)
    // ...
    }
}
```

### 4.2 赋值表达式

```go
func (c *ASTCompiler) visitAssignExpr(e *AssignExpr) {
    // 解析 lvalue
    lvalue := c.emitLvalue(e.LHS)
    
    // 解析 rvalue
    c.visitExpr(e.RHS)
    
    // 处理复合赋值
    if e.Op != '=' {
        // 读取原值
        c.emitGetLvalue(lvalue)
        // 计算新值（栈上已有 op rhs）
        c.visitBinaryExpr(&BinaryExpr{Op: e.Op, Left: ..., Right: ...})
    }
    
    // 写入 lvalue
    c.emitPutLvalue(lvalue)
}
```

### 4.3 条件表达式

```go
func (c *ASTCompiler) visitTernaryExpr(e *TernaryExpr) {
    end := c.NewLabel()
    else_ := c.NewLabel()
    
    // 条件
    c.visitExpr(e.Cond)
    c.EmitGoto(OP_if_false, else_)  // 跳转到 else
    
    // then 分支
    c.visitExpr(e.Then)
    c.EmitGoto(OP_goto, end)        // 跳过 else
    c.BindLabel(else_)
    
    // else 分支
    c.visitExpr(e.Else)
    c.BindLabel(end)
}
```

### 4.4 逻辑表达式

```go
// a && b → a ? b : a
func (c *ASTCompiler) visitLogicalAnd(e *LogicalExpr) {
    end := c.NewLabel()
    
    c.visitExpr(e.Left)
    c.EmitOp(OP_dup)
    c.EmitGoto(OP_if_false, end)    // left 为假，直接返回
    c.EmitOp(OP_drop)               // 丢弃 left
    c.visitExpr(e.Right)
    c.BindLabel(end)
}

// a || b → a ? a : b
func (c *ASTCompiler) visitLogicalOr(e *LogicalExpr) {
    end := c.NewLabel()
    
    c.visitExpr(e.Left)
    c.EmitOp(OP_dup)
    c.EmitGoto(OP_if_true, end)     // left 为真，直接返回
    c.EmitOp(OP_drop)               // 丢弃 left
    c.visitExpr(e.Right)
    c.BindLabel(end)
}
```

---

## 5. 语句编译

### 5.1 变量声明

```go
func (c *ASTCompiler) visitVarDecl(d *VarDecl) {
    for _, decl := range d.Declarations {
        // 添加变量定义
        idx := c.addVar(decl.Name, d.Kind)
        
        // 初始化
        if decl.Init != nil {
            c.visitExpr(decl.Init)
        } else {
            c.EmitOp(OP_undefined)
        }
        
        // 存储
        switch d.Kind {
        case VAR_VAR:
            c.EmitOp(OP_set_orth)
            c.EmitU16(uint16(idx))
        case VAR_LET, VAR_CONST:
            c.EmitOp(OP_set_loc)
            c.EmitU16(uint16(idx))
        case VAR_ARG:
            // 参数已经在栈上
        }
    }
}
```

### 5.2 If 语句

```go
func (c *ASTCompiler) visitIfStmt(s *IfStmt) {
    else_ := c.NewLabel()
    end := c.NewLabel()
    
    c.visitExpr(s.Cond)
    c.EmitGoto(OP_if_false, else_)
    
    c.visitStmt(s.Then)
    c.EmitGoto(OP_goto, end)
    
    c.BindLabel(else_)
    if s.Else != nil {
        c.visitStmt(s.Else)
    }
    
    c.BindLabel(end)
}
```

### 5.3 While 循环

```go
func (c *ASTCompiler) visitWhileStmt(s *WhileStmt) {
    loop := c.NewLabel()
    end := c.NewLabel()
    
    c.PushBreakTarget(end)
    c.PushContinueTarget(loop)
    
    c.BindLabel(loop)
    c.visitExpr(s.Cond)
    c.EmitGoto(OP_if_false, end)
    
    c.visitStmt(s.Body)
    c.EmitGoto(OP_goto, loop)
    
    c.BindLabel(end)
    c.PopBreakTarget()
    c.PopContinueTarget()
}
```

### 5.4 For 循环

```go
func (c *ASTCompiler) visitForStmt(s *ForStmt) {
    end := c.NewLabel()
    c.PushBreakTarget(end)
    
    // 初始化
    if s.Init != nil {
        c.visitStmt(s.Init)
    }
    
    loop := c.NewLabel()
    c.BindLabel(loop)
    
    // 条件
    if s.Cond != nil {
        c.visitExpr(s.Cond)
        c.EmitGoto(OP_if_false, end)
    }
    
    // 循环体
    c.visitStmt(s.Body)
    
    // 迭代表达式
    if s.Incr != nil {
        c.visitExpr(s.Incr)
        c.EmitOp(OP_drop)  // 丢弃迭代值
    }
    
    c.EmitGoto(OP_goto, loop)
    c.BindLabel(end)
    c.PopBreakTarget()
}
```

### 5.5 Return 语句

```go
func (c *ASTCompiler) visitReturnStmt(s *ReturnStmt) {
    // 处理 finally
    c.emitFinally(RETURN)
    
    if s.Value != nil {
        c.visitExpr(s.Value)
        c.EmitOp(OP_return)
    } else {
        c.EmitOp(OP_return_undef)
    }
}
```

---

## 6. 函数编译

### 6.1 函数定义

```go
func (c *ASTCompiler) visitFuncDecl(fn *FuncDecl) *FunctionDef {
    // 创建子函数定义
    child := &FunctionDef{
        Context: c.ctx,
        Name:    fn.Name,
        FuncKind: fn.Kind,
        FuncType: fn.Type,
        Parent: c.fd,
    }
    
    // 编译参数
    for _, param := range fn.Params {
        child.addVar(param.Name, VAR_ARG)
    }
    
    // 编译函数体
    oldFd := c.fd
    c.fd = child
    for _, stmt := range fn.Body {
        c.visitStmt(stmt)
    }
    c.fd = oldFd
    
    // 添加到常量池
    idx := c.AddToPool(Value{tag: JS_TAG_FUNCTION_BYTECODE, ptr: child})
    
    // emit closure 创建
    c.EmitOp(OP_closure)
    c.EmitU32(uint32(idx))
    
    return child
}
```

### 6.2 函数调用

```go
func (c *ASTCompiler) visitCallExpr(call *CallExpr) {
    // 可选链处理
    isOptional := call.Optional
    
    // callee
    c.visitExpr(call.Callee)
    
    // arguments
    for _, arg := range call.Args {
        c.visitExpr(arg)
    }
    
    if isOptional {
        c.EmitOp(OP_call_opt)
    } else {
        c.EmitOp(OP_call)
    }
    c.EmitU16(uint16(len(call.Args)))
}
```

---

## 7. 闭包和作用域

### 7.1 变量查找

```go
func (c *Compiler) findVar(name Atom) *VarDef {
    // 在当前作用域链中查找
    for scope := c.fd.ScopeLevel; scope >= 0; scope-- {
        if v := c.fd.findVarInScope(name, scope); v != nil {
            return v
        }
    }
    // 全局查找
    return c.ctx.GlobalEnv.GetVar(name)
}

func (c *Compiler) emitGetVar(name Atom) {
    v := c.findVar(name)
    if v.ScopeLevel == c.fd.ScopeLevel {
        // 局部变量
        c.EmitOp(OP_get_loc)
        c.EmitU16(uint16(v.Index))
    } else {
        // 作用域变量
        c.EmitOp(OP_scope_get_var)
        c.EmitAtom(name)
        c.EmitU16(uint16(v.ScopeLevel))
    }
}
```

### 7.2 闭包变量捕获

```go
func (c *Compiler) closeFunction(fd *FunctionDef) {
    // 收集被嵌套函数引用的变量
    for _, child := range fd.Children {
        for _, ref := range child.ClosureRefs {
            if ref.ScopeLevel < fd.ScopeLevel {
                // 添加到闭包变量数组
                fd.addClosureVar(ref)
            }
        }
    }
    
    // 生成闭包字节码
    // ...
}
```

---

## 8. 常量池

### 8.1 实现

```go
type ConstPool struct {
    items []Value
    index map[Value]int  // 去重
}

func (c *ConstPool) Add(v Value) int {
    // 查找已有
    if idx, ok := c.index[v]; ok {
        return idx
    }
    
    // 添加新项
    idx := len(c.items)
    c.items = append(c.items, v)
    c.index[v] = idx
    return idx
}

func (c *Compiler) emitPushConst(v Value) {
    idx := c.pool.Add(v)
    c.EmitOp(OP_push_const)
    c.EmitU32(uint32(idx))
}

func (c *Compiler) emitPushAtom(atom Atom) {
    c.EmitOp(OP_push_atom_value)
    c.EmitAtom(atom)
}
```

### 8.2 字符串 interning

```go
func (c *Compiler) emitInternedString(s string) {
    atom := c.ctx.Intern(s)
    c.emitPushAtom(atom)
}
```

---

## 9. 行号信息

### 9.1 源码位置映射

```go
func (c *Compiler) emitSourcePos(pos SourcePos) {
    if c.lastSourcePos != pos.Offset {
        c.EmitOp(OP_line_num)
        c.EmitU32(uint32(pos.Offset))
        c.lastSourcePos = pos.Offset
    }
}
```

### 9.2 pc2line 表

```go
type Pc2LineBuilder struct {
    entries []pc2lineEntry
}

type pc2lineEntry struct {
    pc   int
    line int
}

// 每条指令后检查是否需要记录
func (b *Pc2LineBuilder) MaybeAddEntry(pc, line int) {
    if len(b.entries) == 0 || b.entries[len(b.entries)-1].line != line {
        b.entries = append(b.entries, pc2lineEntry{pc, line})
    }
}
```

---

## 10. 优化

### 10.1 常量折叠

```go
func (c *Compiler) visitBinaryExpr(e *BinaryExpr) {
    // 检查是否为常量
    if left := c.tryFoldConst(e.Left); left != nil {
        if right := c.tryFoldConst(e.Right); right != nil {
            // 常量折叠
            return c.foldBinOp(e.Op, left, right)
        }
    }
    // 正常编译
    c.visitExpr(e.Left)
    c.visitExpr(e.Right)
    c.emitBinOp(e.Op)
}
```

### 10.2 死代码消除

```go
func (c *Compiler) isDeadCode() bool {
    if len(c.writer.Bytes()) == 0 {
        return false
    }
    last := c.writer.LastOpcode()
    return isTerminalOpcode(last)
}

var terminalOpcodes = map[Opcode]bool{
    OP_return: true,
    OP_return_undef: true,
    OP_throw: true,
    OP_goto: true,
}
```

### 10.3 短 opcode

```go
func (c *Compiler) OptimizeShortOpcodes() {
    // 遍历字节码
    // 将常见 opcode 替换为短编码
    // 例如: OP_push_i32 + 4字节 → OP_push_i32_0~OP_push_i32_8 (1字节)
}
```

---

## 11. 测试策略

### 11.1 字节码对比

```go
func TestBytecodeEquiv(t *testing.T) {
    tests := []struct {
        src string
    }{
        {"let x = 1"},
        {"function f() { return 1 }"},
        {"let f = () => 2"},
        {"class C { method() {} }"},
    }
    
    for _, tt := range tests {
        // QuickJS 字节码
        qjsBc := compileWithQuickJS(tt.src)
        
        // Go 字节码
        goBc := compileWithGo(tt.src)
        
        // 对比（忽略调试信息）
        if !bytes.Equal(goBc.Code(), qjsBc.Code()) {
            t.Errorf("bytecode mismatch for %q", tt.src)
            t.Logf("Go: %x", goBc.Code())
            t.Logf("QJS: %x", qjsBc.Code())
        }
    }
}
```

### 11.2 语义测试

```go
func TestSemantics(t *testing.T) {
    tests := []struct {
        src      string
        expected string  // 期望的 JSON 输出
    }{
        {"let x = 1", "1"},
        {"(function(){return 1})()", "1"},
        {"[1,2,3].map(x => x*2)", "[2,4,6]"},
    }
    
    for _, tt := range tests {
        // Go 执行
        goResult := runGoParser(tt.src)
        
        // QuickJS 执行
        qjsResult := runQuickJS(tt.src)
        
        if goResult != qjsResult {
            t.Errorf("result mismatch for %q", tt.src)
            t.Logf("Go: %s", goResult)
            t.Logf("QJS: %s", qjsResult)
        }
    }
}
```

---

## 12. 已知陷阱

### 12.1 Switch 语句

```go
// switch 需要特殊的穿透逻辑
switch (x) {
    case 1:
        if (y) break;  // 条件穿透
        // 继续执行 case 2
    case 2:
        // ...
}
```

### 12.2 Try-Finally

```go
// finally 需要特殊处理
// return 值必须保存，finally 执行后返回
try {
    return 1;
} finally {
    console.log("finally");
}
```

### 12.3 For-In/Of 迭代器

```go
// 迭代器的创建和清理必须成对
for (let x in obj) { }
// 必须生成:
// 1. 创建迭代器
// 2. 循环体
// 3. 迭代器清理
```

### 12.4 生成器状态机

```go
// generator 函数需要转换为状态机
function* gen() {
    yield 1;
    yield 2;
    return 3;
}

// 转换为:
// switch (state) {
// case 0: yield 1; state++; return;
// case 1: yield 2; state++; return;
// case 2: return 3;
// }
```
