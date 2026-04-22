# Token 类型与词法规则

## 1. Token 枚举 (行 21219-21277)

QuickJS 将 Token 分为三类，按数值范围划分：

| 范围 | Token | 含义 |
|------|-------|------|
| -128~-1 | `TOK_NUMBER`, `TOK_STRING`, `TOK_TEMPLATE`, `TOK_IDENT`, `TOK_REGEXP` | 字面量/标识符 |
| -128~-65 | `TOK_MUL_ASSIGN` ~ `TOK_ERROR` | 运算符/分隔符 |
| -64~-1 | `TOK_NULL` ~ `TOK_AWAIT` | 关键字 |

**关键设计点**：
- 关键字顺序与 `JSAtom` 枚举顺序严格对应（代码注释："WARNING: same order as atoms"）
- `TOK_AWAIT` 必须是最后一个关键字（用于 `is_strict_future_keyword` 判断）

### 运算符 Token（优先级影响顺序）

```c
// 行 21219-21254
/* warning: order matters (see js_parse_assign_expr) */
TOK_MUL_ASSIGN,   // *=
TOK_DIV_ASSIGN,   // /=
TOK_MOD_ASSIGN,   // %=
TOK_PLUS_ASSIGN,  // +=
TOK_MINUS_ASSIGN, // -=
TOK_SHL_ASSIGN,   // <<=
TOK_SAR_ASSIGN,   // >>=
TOK_SHR_ASSIGN,   // >>>=
TOK_AND_ASSIGN,   // &=
TOK_XOR_ASSIGN,   // ^=
TOK_OR_ASSIGN,    // |=
TOK_POW_ASSIGN,   // **=
TOK_LAND_ASSIGN,  // &&=
TOK_LOR_ASSIGN,   // ||=
TOK_DOUBLE_QUESTION_MARK_ASSIGN, // ??=
TOK_DEC,          // --
TOK_INC,          // ++
TOK_SHL,          // <<
TOK_SAR,          // >>
TOK_SHR,          // >>>
TOK_LT,           // <
TOK_LTE,          // <=
TOK_GT,           // >
TOK_GTE,          // >=
TOK_EQ,           // ==
TOK_STRICT_EQ,    // ===
TOK_NEQ,          // !=
TOK_STRICT_NEQ,   // !==
TOK_LAND,         // &&
TOK_LOR,          // ||
TOK_POW,          // **
TOK_ARROW,        // =>
TOK_ELLIPSIS,     // ...
TOK_DOUBLE_QUESTION_MARK, // ??
TOK_QUESTION_MARK_DOT, // ?.
TOK_ERROR,
TOK_PRIVATE_NAME,
TOK_EOF,
```

### 关键字 Token

```c
// 行 21264-21277
TOK_NULL,     /* must be first */
TOK_FALSE,
TOK_TRUE,
TOK_IF,
TOK_ELSE,
TOK_RETURN,
TOK_VAR,
TOK_THIS,
TOK_DELETE,
TOK_VOID,
TOK_TYPEOF,
TOK_NEW,
TOK_IN,
TOK_INSTANCEOF,
TOK_DO,
TOK_WHILE,
TOK_FOR,
TOK_BREAK,
TOK_CONTINUE,
TOK_SWITCH,
TOK_CASE,
TOK_DEFAULT,
TOK_THROW,
TOK_TRY,
TOK_CATCH,
TOK_FINALLY,
TOK_FUNCTION,
TOK_DEBUGGER,
TOK_WITH,
/* FutureReservedWord */
TOK_CLASS,
TOK_CONST,
TOK_ENUM,
TOK_EXPORT,
TOK_EXTENDS,
TOK_IMPORT,
TOK_SUPER,
/* FutureReservedWords when parsing strict mode code */
TOK_IMPLEMENT,
TOK_INTERFACE,
TOK_LET,
TOK_PACKAGE,
TOK_PRIVATE,
TOK_PROTECTED,
TOK_PUBLIC,
TOK_STATIC,
TOK_YIELD,
TOK_AWAIT, /* must be last */
```

---

## 2. Token 数据结构（行 21539-21560）

```c
typedef struct JSToken {
    int val;                              // Token 值（枚举值）
    const uint8_t *ptr;                    // 源码位置指针
    union {
        struct {
            JSValue str;                  // 字符串字面量值
            int sep;                      // 分隔符（' 或 "）
        } str;
        struct {
            JSValue val;                  // 数字字面量值
        } num;
        struct {
            JSAtom atom;                  // 标识符/关键字对应的 Atom
            BOOL has_escape;              // 是否有 Unicode 转义
            BOOL is_reserved;             // 是否为保留字
        } ident;
        struct {
            JSValue body;                 // RegExp 正则表达式体
            JSValue flags;                // RegExp 标志
        } regexp;
    } u;
} JSToken;
```

**设计要点**：
- Token 是**值语义**（复制），不持有指针
- 字符串/数字/RegExp 使用 `JSValue`，支持自动 GC 引用计数
- 标识符使用 `JSAtom`（原子化字符串），避免重复存储

---

## 3. 词法分析器（行 22257-23226）

### 3.1 主循环 `next_token()`（行 22257）

```c
static __exception int next_token(JSParseState *s)
{
    free_token(s, &s->token);  // 释放前一个 Token 的动态资源
    p = s->last_ptr = s->buf_ptr;
 redo:
    s->token.ptr = p;
    c = *p;
    switch(c) {
    case 0:      // 行 22276 - 字符串结束
    case '`':    // 行 22286 - 模板字符串
    case '\'':
    case '"':   // 行 22288 - 字符串字面量
    case '\r':
    case '\n':  // 行 22291 - 换行符
    case '\f':
    case '\v':
    case ' ':
    case '\t':  // 行 22303 - 空白符
        p++; goto redo;
    case '/':   // 行 22308 - 注释或除法
        if (p[1] == '*') { ... /* 块注释 */ }
        else if (p[1] == '/') { ... /* 行注释 */ }
        else if (p[1] == '=') { ... /* /= */ }
        else { ... /* 除法 */ }
    default:    // 行 22380 - 标识符、关键字、数字
        ...
    }
}
```

### 3.2 标识符/关键字解析（行 22380-22475）

```
1. 解析 UTF-8 字符序列
2. 查表判断是否为关键字
3. 如果是关键字 → TOK_IDENT + is_reserved=true
4. 否则 → atom = JS_NewAtomStr(string)
```

**关键字查表**：QuickJS 使用编译时生成的跳转表（quickjs-atom.h），通过字符串比较识别。

### 3.3 数字解析（行 22477-22679）

支持格式：
- 十进制整数/浮点数
- 十六进制 (0x/0X)
- 八进制 (0o/0O) / 旧式八进制
- 二进制 (0b/0B)
- BigInt 后缀 (n)
- 科学计数法

**优化**：小整数 (INT32 范围) 使用 `JS_TAG_INT` 标签，避免堆分配。

### 3.4 字符串解析 `js_parse_string()`（行 21889）

```
1. 解析分隔符
2. 扫描直到匹配的分隔符
3. 处理转义序列：
   - \n, \t, \r, \", \', \\
   - \xNN (2位十六进制)
   - \uNNNN / \u{NNNN} (Unicode)
   - \0 (但不是字符串结束)
4. 验证 UTF-8 编码
5. 旧式八进制检测（严格模式下报错）
```

### 3.5 模板字符串 `js_parse_template_part()`（行 21826）

模板字符串特殊处理：
- `${` 触发表达式求值（插入 `TOK_TEMPLATE` + 递归解析表达式）
- 支持 raw strings（`String.raw`）

### 3.6 RegExp 解析 `js_parse_regexp()`（行 22031）

**注意**：RegExp 词法分析在 `next_token()` 中通过将 `/` 回退解析实现：
```c
case '/':
    s->buf_ptr--;  // 回退一个字符
parse_regexp:
    js_parse_regexp(s);
```

---

## 4. 辅助函数

### `peek_token()`（行 23183）
向前查看下一个 Token（不消耗）：
```c
static int peek_token(JSParseState *s, BOOL no_line_terminator)
{
    const uint8_t *p = s->buf_ptr;
    return simple_next_token(&p, no_line_terminator);
}
```

### `simple_next_token()`（行 23087）
无状态的快速 Token 预览（用于 Arrow Function 检测等场景）：
```c
static int simple_next_token(const uint8_t **pp, BOOL no_line_terminator)
```

### `free_token()`（行 21626）
释放 Token 中的动态资源（字符串、数字、Atom）：
```c
static void free_token(JSParseState *s, JSToken *token)
{
    switch(token->val) {
    case TOK_NUMBER:   JS_FreeValue(s->ctx, token->u.num.val); break;
    case TOK_STRING:   JS_FreeValue(s->ctx, token->u.str.str); break;
    case TOK_REGEXP:   JS_FreeValue(s->ctx, token->u.regexp.body); break;
    case TOK_IDENT:    JS_FreeAtom(s->ctx, token->u.ident.atom); break;
    }
}
```

---

## 5. ASI（自动分号插入）

**触发条件**（行 22391-22405）：
- 换行后遇到限制性 Token（`}`, `[`, `/`, `+`, `-`, `)`, `]`, `:`, `,`, `;`）
- 行首遇到 `++`/`--`（后缀自增/自减）
- 遇到 `}` 前没有分号

**`got_lf` 标志**（行 21554）：
```c
BOOL got_lf; /* true if got line feed before the current token */
```
- 在换行/注释后设置为 TRUE
- 用于 ASI 判断

---

## 6. Parser State（行 21562-21575）

```c
typedef struct JSParseState {
    JSContext *ctx;
    const char *filename;
    JSToken token;                    // 当前 Token
    BOOL got_lf;                      // 当前 Token 前是否有换行
    const uint8_t *last_ptr;         // 前一个 Token 结束位置
    const uint8_t *buf_start;       // 源码缓冲区起始
    const uint8_t *buf_ptr;          // 当前扫描位置
    const uint8_t *buf_end;         // 源码缓冲区结束

    JSFunctionDef *cur_func;         // 当前正在解析的函数
    BOOL is_module;                   // 是否解析模块
    BOOL allow_html_comments;        // 是否允许 HTML 注释
    BOOL ext_json;                    // JSON 超集兼容模式
    GetLineColCache get_line_col_cache;
} JSParseState;
```

---

## 7. 关键设计决策

### 7.1 Single-Pass 编译（无 AST）
QuickJS **不构建 AST**。Parser 在递归下降过程中直接 emit 字节码：
- 节省内存分配
- 减少数据结构的转换开销
- 代价：错误恢复和增量编译困难

### 7.2 Token 与 Atom 耦合
关键字 Token 值与 Atom 枚举值一一对应，使得：
- 关键字解析 → 直接得到 Atom
- Atom → 直接得到 Token 值

### 7.3 行号信息嵌入
```c
static void emit_source_pos(JSParseState *s, const uint8_t *source_ptr)
{
    if (fd->last_opcode_source_ptr != source_ptr) {
        dbuf_putc(bc, OP_line_num);
        dbuf_put_u32(bc, source_ptr - s->buf_start);
        fd->last_opcode_source_ptr = source_ptr;
    }
}
```
行号通过 `OP_line_num` 指令嵌入字节码，使用字节偏移而非行列号（节省空间）。

### 7.4 Shebang 支持
```c
static void skip_shebang(const uint8_t **pp, const uint8_t *buf_end)
{
    if (p[0] == '#' && p[1] == '!') { ... }
}
```
支持 `#!` 开头的脚本文件。

---

## 8. 分析方法

| 检查项 | 命令 | 范围 |
|--------|------|------|
| Token 定义 | `grep -n "^.*TOK_" quickjs.c` | 行 21219-21277 |
| Token 结构 | `grep -n "typedef.*JSToken" quickjs.c` | 行 21539 |
| 词法分析 | `grep -n "^static.*next_token\|simple_next_token" quickjs.c` | 行 22257, 23087 |
| 字符串解析 | `grep -n "^static.*js_parse_string" quickjs.c` | 行 21889 |
| RegExp 解析 | `grep -n "^static.*js_parse_regexp" quickjs.c` | 行 22031 |
| 模板字符串 | `grep -n "^static.*js_parse_template_part" quickjs.c` | 行 21826 |
| ASI 判断 | `grep -n "got_lf" quickjs.c` | 全文件 |

---

## 9. 未检查项

- 具体的 Unicode 规范化处理（行 22600-22750）
- JSON 解析模式的词法差异（`json_next_token` 行 22905）
- 正则表达式语法的详细验证
- Hashbang 处理的具体逻辑
