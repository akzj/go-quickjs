# Go Parser 实现指南

## 1. 设计原则

### 1.1 Single-Pass 编译

QuickJS 采用 **无 AST 的 Single-Pass 编译**：
- Parser 在递归下降过程中直接 emit 字节码
- 优点：内存效率高、无中间表示转换开销
- 缺点：错误恢复困难、增量编译不友好

**Go 实现建议**：
- 初期可以构建 AST（便于调试和错误报告）
- 后期优化为 Single-Pass（如果性能需要）
- 或者保持 AST，设计 BytecodeEmitter 接口层

### 1.2 类型安全

C 语言的 `int` Token 值在 Go 中需要类型安全包装：

```go
// Token 类型
type TokenType int32

const (
    TOK_EOF TokenType = iota
    TOK_ERROR
    // ... 运算符 ...
    TOK_MUL_ASSIGN
    // ... 关键字 ...
    TOK_FUNCTION
    TOK_VAR
    TOK_RETURN
    // ...
    TOK_IDENT
    TOK_NUMBER
    TOK_STRING
)

// Token 结构
type Token struct {
    Type  TokenType
    Pos   SourcePos      // 源码位置
    Lit   string         // 字面量（标识符名、字符串内容）
    Num   Value          // 数字值（复用 JSValue）
    Atom  Atom           // Atom（标识符转换后）
}
```

---

## 2. 词法分析器实现

### 2.1 接口设计

```go
type Lexer interface {
    NextToken() (Token, error)
    PeekToken() (Token, error)      // 预览下一个 Token（不消耗）
    SetPos(pos SourcePos)            // 设置位置（用于回退）
    Position() SourcePos             // 获取当前位置
}

type SourcePos struct {
    Offset int    // 字节偏移
    Line   int    // 行号
    Col    int    // 列号
}
```

### 2.2 核心实现

```go
type lexer struct {
    src    []byte
    pos    int
    line   int
    col    int
    token  Token     // 当前 Token
    hasPeek bool     // 是否有预览的 Token
}

func (l *lexer) NextToken() (Token, error) {
    if l.hasPeek {
        l.hasPeek = false
        return l.token, nil
    }
    return l.scanToken()
}

func (l *lexer) PeekToken() (Token, error) {
    if !l.hasPeek {
        tok, err := l.scanToken()
        if err != nil {
            return Token{}, err
        }
        l.token = tok
        l.hasPeek = true
    }
    return l.token, nil
}

func (l *lexer) scanToken() (Token, error) {
    l.skipWhitespace()
    
    if l.pos >= len(l.src) {
        return Token{Type: TOK_EOF, Pos: l.pos()}, nil
    }
    
    ch := l.src[l.pos]
    switch {
    case isLetter(ch) || ch >= utf8.RuneSelf:
        return l.scanIdent()
    case isDigit(ch):
        return l.scanNumber()
    case ch == '"' || ch == '\'':
        return l.scanString(ch)
    case ch == '`':
        return l.scanTemplate()
    case ch == '/':
        if l.peek() == '/' || l.peek() == '*' {
            return l.scanComment()
        }
        // 除法或正则（上下文相关）
        l.pos++
        return Token{Type: '/', Pos: l.pos}, nil
    default:
        return l.scanOperator()
    }
}
```

### 2.3 标识符/关键字识别

```go
var keywords = map[string]TokenType{
    "if":       TOK_IF,
    "else":     TOK_ELSE,
    "function": TOK_FUNCTION,
    "return":   TOK_RETURN,
    "var":      TOK_VAR,
    "let":      TOK_LET,
    "const":    TOK_CONST,
    // ... 其他关键字
}

func (l *lexer) scanIdent() (Token, error) {
    start := l.pos
    for l.pos < len(l.src) && isIdentChar(l.src[l.pos]) {
        l.pos++
    }
    lit := string(l.src[start:l.pos])
    
    if tok, ok := keywords[lit]; ok {
        return Token{Type: tok, Pos: start, Lit: lit}, nil
    }
    
    // 标识符 → Atom
    atom := ctx.Intern(lit)
    return Token{Type: TOK_IDENT, Pos: start, Lit: lit, Atom: atom}, nil
}
```

### 2.4 数字解析

```go
func (l *lexer) scanNumber() (Token, error) {
    start := l.pos
    
    // 进制检测
    base := 10
    if l.src[l.pos] == '0' && l.pos+1 < len(l.src) {
        switch l.src[l.pos+1] {
        case 'x', 'X': base = 16; l.pos += 2
        case 'o', 'O': base = 8; l.pos += 2
        case 'b', 'B': base = 2; l.pos += 2
        }
    }
    
    // 解析数字
    hasDecimal := false
    for l.pos < len(l.src) {
        ch := l.src[l.pos]
        if ch == '.' && !hasDecimal {
            hasDecimal = true
            l.pos++
            continue
        }
        if !isDigit(ch, base) {
            break
        }
        l.pos++
    }
    
    // BigInt 后缀
    if l.pos < len(l.src) && l.src[l.pos] == 'n' {
        l.pos++
        return Token{Type: TOK_BIGINT, Pos: start, Lit: string(l.src[start:l.pos])}, nil
    }
    
    // 解析数值
    lit := string(l.src[start:l.pos])
    val, err := parseNumber(lit, base)
    if err != nil {
        return Token{}, err
    }
    
    return Token{Type: TOK_NUMBER, Pos: start, Lit: lit, Num: val}, nil
}
```

---

## 3. 语法分析器实现

### 3.1 Parser 接口

```go
type Parser interface {
    ParseProgram(ctx *Context) (*FunctionDef, error)
    ParseExpression() (Node, error)
}

type parser struct {
    lex  Lexer
    ctx  *Context
    cur  Token     // 当前 Token
    prev Token     // 上一个 Token
}

func NewParser(src []byte, ctx *Context) Parser {
    p := &parser{
        lex: newLexer(src),
        ctx: ctx,
    }
    p.next()  // 预读第一个 Token
    return p
}
```

### 3.2 辅助方法

```go
func (p *parser) next() {
    p.prev = p.cur
    p.cur, _ = p.lex.NextToken()
}

func (p *parser) expect(t TokenType) error {
    if p.cur.Type != t {
        return p.error("expected %v, got %v", t, p.cur.Type)
    }
    p.next()
    return nil
}

func (p *parser) match(t TokenType) bool {
    if p.cur.Type == t {
        p.next()
        return true
    }
    return false
}

func (p *parser) error(format string, args ...interface{}) error {
    return &ParseError{
        Pos: p.cur.Pos,
        Msg: fmt.Sprintf(format, args...),
    }
}
```

### 3.3 表达式解析（优先级递进）

```go
// 赋值表达式
func (p *parser) parseAssignExpr() (Node, error) {
    node, err := p.parseTernaryExpr()
    if err != nil {
        return nil, err
    }
    
    if isAssignOp(p.cur.Type) {
        op := p.cur.Type
        p.next()
        rhs, err := p.parseAssignExpr()
        if err != nil {
            return nil, err
        }
        return &AssignExpr{Op: op, LHS: node, RHS: rhs}, nil
    }
    
    return node, nil
}

// 三元表达式
func (p *parser) parseTernaryExpr() (Node, error) {
    node, err := p.parseOrExpr()
    if err != nil {
        return nil, err
    }
    
    if p.match('?') {
        then, err := p.parseAssignExpr()
        if err != nil {
            return nil, err
        }
        p.expect(':')
        else_, err := p.parseAssignExpr()
        if err != nil {
            return nil, err
        }
        return &TernaryExpr{Cond: node, Then: then, Else: else_}, nil
    }
    
    return node, nil
}

// 逻辑或
func (p *parser) parseOrExpr() (Node, error) {
    node, err := p.parseAndExpr()
    if err != nil {
        return nil, err
    }
    
    for p.match(TOK_LOR) {
        rhs, err := p.parseAndExpr()
        if err != nil {
            return nil, err
        }
        node = &BinaryExpr{Op: TOK_LOR, Left: node, Right: rhs}
    }
    return node, nil
}

// ... 其他优先级 ...
```

### 3.4 语句解析

```go
func (p *parser) parseStatement() (Stmt, error) {
    switch p.cur.Type {
    case TOK_IF:
        return p.parseIfStmt()
    case TOK_WHILE:
        return p.parseWhileStmt()
    case TOK_FOR:
        return p.parseForStmt()
    case TOK_RETURN:
        return p.parseReturnStmt()
    case '{':
        return p.parseBlockStmt()
    case TOK_VAR, TOK_LET, TOK_CONST:
        return p.parseVarDecl()
    default:
        return p.parseExprStmt()
    }
}

func (p *parser) parseIfStmt() (*IfStmt, error) {
    p.expect(TOK_IF)
    p.expect('(')
    cond, err := p.parseExpr()
    if err != nil {
        return nil, err
    }
    p.expect(')')
    then, err := p.parseStatement()
    if err != nil {
        return nil, err
    }
    
    var else_ Stmt
    if p.match(TOK_ELSE) {
        else_, err = p.parseStatement()
        if err != nil {
            return nil, err
        }
    }
    
    return &IfStmt{Cond: cond, Then: then, Else: else_}, nil
}
```

---

## 4. AST 节点定义

### 4.1 表达式节点

```go
type Node interface {
    node()
    Pos() SourcePos
}

type (
    Ident struct {
        Name string
        Atom Atom
    }
    NumberLiteral struct{ Value Value }
    StringLiteral struct{ Value string }
    ArrayLiteral struct{ Elements []Expr }
    ObjectLiteral struct{ Properties []*Property }  // TODO
    FuncExpr struct{ Func *FuncDef }
    ArrowFunc struct{ Func *FuncDef }
    CallExpr struct{ Callee Expr; Args []Expr }
    MemberExpr struct{ Object Expr; Property Expr; Computed bool }
    BinaryExpr struct{ Op TokenType; Left, Right Expr }
    UnaryExpr struct{ Op TokenType; Operand Expr; Postfix bool }
    AssignExpr struct{ Op TokenType; LHS, RHS Expr }
    TernaryExpr struct{ Cond, Then, Else Expr }
)
```

### 4.2 语句节点

```go
type (
    ExprStmt struct{ Expr Expr }
    VarDecl struct{ Kind TokenType; Declarations []*VarDeclarator }
    VarDeclarator struct{ Name *Ident; Init Expr }
    IfStmt struct{ Cond Expr; Then, Else Stmt }
    WhileStmt struct{ Cond Expr; Body Stmt }
    ForStmt struct{ Init Stmt; Cond, Incr Expr; Body Stmt }
    ForInStmt struct{ Target, Source Expr; Body Stmt }
    ForOfStmt struct{ Target, Source Expr; Body Stmt; Await bool }
    ReturnStmt struct{ Value Expr }
    BlockStmt struct{ Body []Stmt }
    FuncStmt struct{ Func *FuncDef }  // 函数声明
)
```

---

## 5. 错误处理

### 5.1 错误恢复

```go
type ErrorHandler func(err *ParseError)

// 跳过到同步点
func (p *parser) skipUntil(tokens ...TokenType) {
    for p.cur.Type != TOK_EOF {
        for _, t := range tokens {
            if p.cur.Type == t {
                return
            }
        }
        p.next()
    }
}
```

### 5.2 详细错误信息

```go
type ParseError struct {
    Pos    SourcePos
    Line   int
    Col    int
    Msg    string
    Source string  // 源码片段
}

func (e *ParseError) Error() string {
    return fmt.Sprintf("%s:%d:%d: %s", e.Pos.Filename, e.Line, e.Col, e.Msg)
}
```

---

## 6. 性能考虑

### 6.1 对象池

```go
var tokenPool = sync.Pool{
    New: func() interface{} {
        return &Token{}
    },
}

func getToken() *Token {
    return tokenPool.Get().(*Token)
}

func putToken(t *Token) {
    tokenPool.Put(t)
}
```

### 6.2 字符串 interning

```go
type Atom = uint32  // 或自定义类型

type AtomTable struct {
    m map[string]Atom
    next Atom
}

func (a *AtomTable) Intern(s string) Atom {
    if atom, ok := a.m[s]; ok {
        return atom
    }
    atom := a.next
    a.next++
    a.m[s] = atom
    return atom
}
```

### 6.3 整数标签优化

```go
// 小整数直接编码在 Value 中
type Value struct {
    tag uint8   // JS_TAG_*
    raw uint64  // 立即数或指针
}

const (
    JS_TAG_INT       = 0
    JS_TAG_STRING    = 2
    JS_TAG_OBJECT    = 4
    // ...
)

func NewIntValue(i int64) Value {
    return Value{tag: JS_TAG_INT, raw: uint64(i)}
}

func (v Value) Int() int64 {
    return int64(v.raw)
}
```

---

## 7. 测试策略

### 7.1 Token 对比测试

```go
func TestLexerTokens(t *testing.T) {
    tests := []struct {
        input    string
        expected []TokenType
    }{
        {"let x = 1;", []TokenType{TOK_LET, TOK_IDENT, '=', TOK_NUMBER, ';'}},
        {"function foo() {}", []TokenType{TOK_FUNCTION, TOK_IDENT, '(', ')', '{', '}'}},
    }
    
    for _, tt := range tests {
        p := NewParser([]byte(tt.input), ctx)
        var got []TokenType
        for {
            tok, _ := p.NextToken()
            got = append(got, tok.Type)
            if tok.Type == TOK_EOF {
                break
            }
        }
        if !reflect.DeepEqual(got, tt.expected) {
            t.Errorf("tokens(%q) = %v, want %v", tt.input, got, tt.expected)
        }
    }
}
```

### 7.2 AST 对比测试

```go
func TestParserAST(t *testing.T) {
    tests := []struct {
        input    string
        expected Node
    }{
        {"let x = 1", &VarDecl{
            Kind: TOK_LET,
            Declarations: []*VarDeclarator{
                {Name: &Ident{Name: "x"}, Init: &NumberLiteral{Value: 1}},
            },
        }},
    }
    
    for _, tt := range tests {
        p := NewParser([]byte(tt.input), ctx)
        got, err := p.ParseExpression()
        if err != nil {
            t.Errorf("parse(%q) error: %v", tt.input, err)
            continue
        }
        if !reflect.DeepEqual(got, tt.expected) {
            t.Errorf("parse(%q) = %v, want %v", tt.input, got, tt.expected)
        }
    }
}
```

### 7.3 字节码对比测试

```go
func TestBytecode(t *testing.T) {
    // 用 QuickJS 编译得到期望字节码
    expected := compileWithQuickJS("let x = 1")
    
    // 用 Go Parser 编译
    got := compileWithGoParser("let x = 1")
    
    // 对比差异
    diffBytecode(t, got, expected)
}
```

---

## 8. 开发路线图

### Phase 1: 基础 Parser
- [ ] Token 定义和词法分析
- [ ] 表达式解析（支持全部优先级）
- [ ] 语句解析（if/while/for/return/var）
- [ ] 基本错误报告

### Phase 2: 完整语法支持
- [ ] 函数声明/表达式
- [ ] Arrow Function
- [ ] Class 语法
- [ ] try/catch/finally
- [ ] 模板字符串
- [ ] 解构赋值

### Phase 3: 字节码生成
- [ ] BytecodeEmitter 接口
- [ ] 字节码生成器
- [ ] 作用域管理
- [ ] 常量池管理
- [ ] 闭包生成

### Phase 4: 集成测试
- [ ] ECMAScript  conformance tests
- [ ] 性能基准测试
- [ ] 与 QuickJS 字节码对比

---

## 9. 已知陷阱

### 9.1 ASI（自动分号插入）

```javascript
// 这些需要正确处理 ASI
return
{ x: 1 }

// 解析为
return;
{ x: 1 }
```

### 9.2 标签和 `yield`

```javascript
// yield 是关键字，在 generator 函数外是标识符
let yield = 1;
```

### 9.3 严格模式保留字

```javascript
"use strict";
let let = 1;  // 语法错误
```

### 9.4 数字字面量解析

```javascript
0x1A  // 十六进制 = 26
1e10  // 科学计数法
0b101 // 二进制
0o17  // 八进制
1n    // BigInt
```
