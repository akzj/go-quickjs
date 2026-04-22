# 语法分析器设计

## 1. 解析器入口（行 36496-36537）

### `js_parse_program()` — 程序解析

```c
static __exception int js_parse_program(JSParseState *s)
{
    JSFunctionDef *fd = s->cur_func;

    if (next_token(s)) return -1;              // 获取第一个 Token
    if (js_parse_directives(s)) return -1;     // 解析 "use strict" 等指令

    fd->is_global_var = ...;                    // 确定变量作用域规则

    if (!s->is_module) {
        // 添加隐藏的返回值变量
        fd->eval_ret_idx = add_var(s->ctx, fd, JS_ATOM__ret_);
    }

    // 解析所有源元素
    while (s->token.val != TOK_EOF) {
        if (js_parse_source_element(s)) return -1;
    }

    // 生成 return 语句
    emit_return(s, TRUE);
    return 0;
}
```

### 解析流程图

```
输入: 源代码字符串
  ↓
js_parse_init() — 初始化 Parser State
  ↓
next_token() — 获取第一个 Token
  ↓
js_parse_directives() — 解析 "use strict" 等指令
  ↓
while (token != EOF):
  js_parse_source_element() — 解析一个顶层语句/声明
      ├── TOK_FUNCTION → js_parse_function_decl()
      ├── async function → js_parse_function_decl()
      ├── TOK_EXPORT → js_parse_export()
      ├── TOK_IMPORT → js_parse_import()
      └── 其他 → js_parse_statement_or_decl()
  ↓
emit_return() — 生成隐式 return 语句
  ↓
输出: JSFunctionDef (字节码 + 元信息)
```

---

## 2. 表达式解析（优先级递进解析）

### 2.1 解析层次（从低到高）

| 层级 | 函数 | 运算符 |
|------|------|--------|
| 0 | `js_parse_unary()` | `+`, `-`, `!`, `~`, `typeof`, `delete`, `await`, `++`, `--` |
| 1 | `js_parse_expr_binary()` level=1 | `*`, `/`, `%` |
| 2 | level=2 | `+`, `-` |
| 3 | level=3 | `<<`, `>>`, `>>>` |
| 4 | level=4 | `<`, `>`, `<=`, `>=`, `instanceof`, `in`, private field |
| 5 | level=5 | `==`, `!=`, `===`, `!==` |
| 6 | level=6 | `&` |
| 7 | level=7 | `^` |
| 8 | level=8 | `\|` |
| - | `js_parse_expr_binary()` | 循环处理相同优先级运算符 |
| - | `js_parse_logical_and_or()` | `&&`, `\|\|` |
| - | `js_parse_coalesce_expr()` | `??` |
| - | `js_parse_cond_expr()` | `? :` |
| - | `js_parse_assign_expr2()` | `=`, `+=`, ... |

### 2.2 `js_parse_expr()` — 顶层表达式（行 23877）

```c
static __exception int js_parse_expr(JSParseState *s);
// 等价于
js_parse_assign_expr(s);  // 行 27713
```

### 2.3 `js_parse_assign_expr()` — 赋值表达式（行 27713）

```c
static __exception int js_parse_assign_expr(JSParseState *s)
{
    return js_parse_assign_expr2(s, PF_IN_ACCEPTED);
}
```

### 2.4 `js_parse_assign_expr2()` — 赋值表达式实现（行 27427）

```c
static __exception int js_parse_assign_expr2(JSParseState *s, int parse_flags)
{
    // 1. 处理 yield/generator
    if (s->token.val == TOK_YIELD) {
        // 处理 yield 表达式（可能带 *)
    }

    // 2. Arrow Function 检测
    if (s->token.val == '(' && peek_token(...) == TOK_ARROW) {
        return js_parse_function_decl(s, JS_PARSE_FUNC_ARROW, ...);
    }

    // 3. async function 检测
    if (token_is_pseudo_keyword(s, JS_ATOM_async)) {
        // 处理 async 标识符
    }

    // 4. 解构赋值
    if (s->token.val == '{' || s->token.val == '[') {
        return js_parse_destructuring_element(...);
    }

    // 5. 普通赋值
    js_parse_cond_expr(s, parse_flags);  // 解析右侧表达式

    if (op == '=' || (op >= TOK_MUL_ASSIGN && op <= TOK_POW_ASSIGN)) {
        // 获取左侧 lvalue
        get_lvalue(s, &opcode, &scope, &name, &label, NULL, (op != '='), op);
        // 解析右侧值
        js_parse_assign_expr2(s, parse_flags);
        // 设置属性名（如有）
        set_object_name(s, name);
        // 写入 lvalue
        put_lvalue(s, opcode, scope, name, label, PUT_LVALUE_NOKEEP, is_let);
    }
}
```

### 2.5 `js_parse_unary()` — 一元运算符（行 27021）

```c
static __exception int js_parse_unary(JSParseState *s, int parse_flags)
{
    switch(s->token.val) {
    case '+':   emit_op(s, OP_plus); break;
    case '-':   emit_op(s, OP_neg); break;
    case '!':   emit_op(s, OP_lnot); break;
    case '~':   emit_op(s, OP_not); break;
    case TOK_VOID: emit_op(s, OP_drop); emit_op(s, OP_undefined); break;
    case TOK_DEC:
    case TOK_INC:
        // ++x / --x: 获取 lvalue，emit OP_inc/OP_dec，put_lvalue
        break;
    case TOK_TYPEOF:
        // 特殊处理：typeof 未定义变量不报错
        if (prev_op == OP_scope_get_var)
            byte_code[last_pos] = OP_scope_get_var_undef;
        break;
    case TOK_DELETE:
        return js_parse_delete(s);
    case TOK_AWAIT:
        // async 函数中的 await
        emit_op(s, OP_await);
        break;
    default:
        // 后缀表达式
        if (js_parse_postfix_expr(s, PF_POSTFIX_CALL)) return -1;
        // ASI 检查
        if (!s->got_lf && (s->token.val == TOK_INC || s->token.val == TOK_DEC))
            emit_op(s, OP_dec + s->token.val - TOK_INC);  // x++ / x--
    }
}
```

### 2.6 `js_parse_postfix_expr()` — 后缀表达式（行 26260）

处理：
- 字面量（数字、字符串、模板、RegExp）
- 标识符/关键字
- 分组表达式 `(...)`
- 对象/数组字面量
- 函数表达式
- 类表达式
- `new` 表达式
- 属性访问 `obj.prop`
- 索引访问 `obj[key]`
- 函数调用 `func()`
- 可选链 `obj?.prop`

```c
static __exception int js_parse_postfix_expr(JSParseState *s, int parse_flags)
{
    // 1. 处理主表达式（primary expression）
    switch(s->token.val) {
    case TOK_NUMBER:    emit_push_const(...); break;
    case TOK_STRING:    emit_push_const(...); break;
    case TOK_TEMPLATE:  js_parse_template(...); break;
    case '/':           js_parse_regexp(...); break;
    case TOK_IDENT:     emit_scope_get_var(...); break;
    case '(':
        if (peek_token(...) == TOK_ARROW) {
            // Arrow function
            return js_parse_function_decl(s, JS_PARSE_FUNC_ARROW, ...);
        }
        // 分组表达式
        js_parse_expr_paren(s);
        break;
    case '{':  js_parse_object_literal(s); break;
    case '[':  js_parse_array_literal(s); break;
    case TOK_FUNCTION:  js_parse_function_decl(s, JS_PARSE_FUNC_EXPR, ...); break;
    case TOK_CLASS:     js_parse_class(s, TRUE, ...); break;
    case TOK_NEW:
        // new target[, ...args]
        break;
    }

    // 2. 后缀运算符循环
    for (;;) {
        switch(s->token.val) {
        case '(':              // 函数调用
            js_parse_arguments(s);
            emit_op(s, is_opt ? OP_call_opt : OP_call);
            break;
        case '.':              // 属性访问
            next_token(s);
            emit_op(s, OP_get_field);
            emit_atom(s, atom);
            break;
        case '[':              // 索引访问
            js_parse_expr(s);
            js_parse_expect(s, ']');
            emit_op(s, OP_get_array_el);
            break;
        case TOK_QUESTION_MARK_DOT:  // 可选链
            // OP_is_undefined_or_null + 条件跳转
            break;
        case TOK_INC:
        case TOK_DEC:
            // 后缀 ++/--
            break;
        case TOK_TEMPLATE:
            // 模板字符串继续部分
            js_parse_template(s, 1, &argc);
            break;
        default:
            return 0;
        }
    }
}
```

### 2.7 `js_parse_cond_expr()` — 条件表达式（行 27414）

```c
static __exception int js_parse_cond_expr(JSParseState *s, int parse_flags)
{
    js_parse_coalesce_expr(s, parse_flags);
    if (s->token.val == '?') {
        next_token(s);
        label1 = emit_goto(s, OP_if_false, -1);  // 跳过 then 分支
        js_parse_assign_expr(s);                  // then 分支
        js_parse_expect(s, ':');
        label2 = emit_goto(s, OP_goto, -1);       // 跳过 else 分支
        emit_label(s, label1);
        js_parse_assign_expr2(s, parse_flags);   // else 分支
        emit_label(s, label2);
    }
}
```

---

## 3. 语句解析

### 3.1 `js_parse_source_element()` — 源元素（行 31457）

```c
static __exception int js_parse_source_element(JSParseState *s)
{
    if (s->token.val == TOK_FUNCTION ||
        (is_async && peek_token(...) == TOK_FUNCTION)) {
        return js_parse_function_decl(s, JS_PARSE_FUNC_STATEMENT, ...);
    }
    if (s->token.val == TOK_EXPORT && s->is_module) {
        return js_parse_export(s);
    }
    if (s->token.val == TOK_IMPORT && s->is_module &&
        peek_token(...) != '(' && peek_token(...) != '.') {
        return js_parse_import(s);
    }
    return js_parse_statement_or_decl(s, DECL_MASK_ALL);
}
```

### 3.2 `js_parse_statement_or_decl()` — 语句/声明（行 28325）

```c
static __exception int js_parse_statement_or_decl(JSParseState *s, int decl_mask)
{
    switch(s->token.val) {
    case '{':           return js_parse_block(s);
    case TOK_VAR:       return js_parse_var(s, parse_flags, tok, FALSE);
    case TOK_IF:        return js_parse_if(s);
    case TOK_WHILE:     return js_parse_while(s);
    case TOK_FOR:       return js_parse_for(s);
    case TOK_RETURN:    return js_parse_return(s);
    case TOK_SWITCH:    return js_parse_switch(s);
    case TOK_BREAK:
    case TOK_CONTINUE:  return js_parse_break_cont(s);
    case TOK_THROW:     return js_parse_throw(s);
    case TOK_TRY:       return js_parse_try(s);
    case TOK_DEBUGGER:  return js_parse_debugger(s);
    case ';':           next_token(s); return 0;
    case TOK_WITH:      return js_parse_with(s);
    default:
        if (is_function_decl) return js_parse_function_decl(...);
        return js_parse_expr_or_destructuring(s, decl_mask);
    }
}
```

### 3.3 块语句 `js_parse_block()`（行 27930）

```c
static __exception int js_parse_block(JSParseState *s)
{
    js_parse_expect(s, '{');
    if (s->token.val != '}') {
        push_scope(s);
        while (s->token.val != '}') {
            js_parse_statement_or_decl(s, DECL_MASK_ALL);
        }
        pop_scope(s);
    }
    js_parse_expect(s, '}');
}
```

### 3.4 变量声明 `js_parse_var()`（行 27943）

```c
static __exception int js_parse_var(JSParseState *s, int parse_flags, int tok, BOOL export_flag)
{
    for (;;) {
        if (s->token.val == TOK_IDENT) {
            name = js_parse_destructuring_var(s, tok, is_arg);
            js_define_var(s, name, tok);

            if (s->token.val == '=') {
                next_token(s);
                if (need_var_reference(s, tok)) {
                    // 需要引用（with 语义或全局变量）
                    emit_scope_get_var(...);
                    get_lvalue(...);
                    js_parse_assign_expr2(...);
                    put_lvalue(...);
                } else {
                    js_parse_assign_expr2(...);
                    emit_op(s, tok == TOK_CONST ? OP_set_loc : OP_set_orth);
                }
                set_object_name(s, name);
            }
        } else if (s->token.val == '{' || s->token.val == '[') {
            // 解构赋值
            js_parse_destructuring_element(...);
        }
        if (s->token.val != ',') break;
        next_token(s);
    }
    js_parse_expect_semi(s);
}
```

### 3.5 For 循环 `js_parse_for()`（行 28600）

支持多种形式：
- `for (var/let/const x = 0; i < n; i++)`
- `for (x in obj)`
- `for (x of iterable)`
- `for (await x of iterable)`

```c
static __exception int js_parse_for(JSParseState *s)
{
    // 检测 ForAwait/ForIn/ForOf
    if (s->token.val == TOK_AWAIT) { ... }
    else if (peek_token(...) == TOK_IN || peek_token(...) == TOK_OF) { ... }

    // 生成 For 循环初始化字节码
    if (init_type == FOR_IN) {
        emit_op(s, OP_for_in);    // 迭代器初始化
        emit_op(s, OP_for_of_start);
    } else if (init_type == FOR_OF) {
        emit_op(s, OP_for_of_start);
    } else {
        js_parse_expr(s);         // 普通 for 循环
    }

    // 循环体
    js_parse_statement_or_decl(s, DECL_MASK_FOR);

    // 迭代后语句
    emit_op(s, OP_for_in_next);
}
```

---

## 4. 函数解析

### 4.1 函数声明入口（行 36486）

```c
static __exception int js_parse_function_decl(JSParseState *s,
                                              JSParseFunctionEnum func_type,
                                              JSFunctionKindEnum func_kind,
                                              JSAtom func_name,
                                              const uint8_t *ptr)
{
    return js_parse_function_decl2(s, func_type, func_kind, func_name, ptr,
                                   JS_PARSE_EXPORT_NONE, NULL);
}
```

### 4.2 函数声明实现 `js_parse_function_decl2()`（行 23883）

```c
static __exception int js_parse_function_decl2(...)
{
    // 1. 创建 JSFunctionDef
    fd = js_new_function_def(ctx, s->cur_func, is_eval, is_func_expr, ...);

    // 2. 设置函数属性
    fd->func_type = func_type;
    fd->func_kind = func_kind;
    fd->func_name = func_name;

    // 3. 解析函数参数
    if (js_parse_function_params(s, fd, func_type, &has_simple_params, ...))
        goto fail;

    // 4. 处理 "use strict" 指令
    js_parse_directives(s);

    // 5. 解析函数体
    s->cur_func = fd;
    fd->in_function_body = TRUE;
    while (s->token.val != '}') {
        js_parse_statement_or_decl(s, DECL_MASK_ALL);
    }
    next_token(s);  // 消耗 '}'

    // 6. 添加闭包引用
    js_close_function_def(s, fd);

    // 7. 生成闭包创建字节码
    emit_op(s, OP_closure);
    emit_u32(s, cpool_idx);
}
```

### 4.3 Arrow Function 检测（行 27543-27603）

```c
// 在 js_parse_assign_expr2 中
if (s->token.val == '(' && js_parse_skip_parens_token(...) == TOK_ARROW) {
    return js_parse_function_decl(s, JS_PARSE_FUNC_ARROW, JS_FUNC_NORMAL, ...);
}

if (token_is_pseudo_keyword(s, JS_ATOM_async) &&
    ((s->token.val == '(' && ...) ||
     (s->token.val == TOK_IDENT && peek_token(...) == TOK_ARROW))) {
    return js_parse_function_decl(s, JS_PARSE_FUNC_ARROW, JS_FUNC_ASYNC, ...);
}

if (s->token.val == TOK_IDENT && peek_token(...) == TOK_ARROW) {
    return js_parse_function_decl(s, JS_PARSE_FUNC_ARROW, JS_FUNC_NORMAL, ...);
}
```

---

## 5. 对象/数组字面量

### 5.1 对象字面量 `js_parse_object_literal()`（行 24395）

```c
static __exception int js_parse_object_literal(JSParseState *s)
{
    emit_op(s, OP_object);  // 创建空对象
    while (s->token.val != '}') {
        prop_type = js_parse_property_name(s, &name, ...);

        if (prop_type == PROP_TYPE_VAR) {
            // 简写属性 {x} → {x: x}
            emit_scope_get_var(name, scope_level);
            emit_define_field(name);
        } else if (s->token.val == '(') {
            // 方法 {foo() {}}
            js_parse_function_decl(s, JS_PARSE_FUNC_METHOD, ...);
            emit_define_method(name, flags);
        } else {
            // 属性 {x: expr}
            js_parse_assign_expr(s);
            emit_define_field(name);
        }
    }
    next_token(s);  // 消耗 '}'
}
```

### 5.2 数组字面量 `js_parse_array_literal()`（行 25214）

```c
static __exception int js_parse_array_literal(JSParseState *s)
{
    // 小数组（<32 元素，无 hole）栈上创建
    if (elements < 32 && no_holes) {
        while (s->token.val != ']') {
            js_parse_assign_expr(s);
            idx++;
            if (s->token.val == ',') next_token(s);
        }
        emit_op(s, OP_array_from);
        emit_u16(s, idx);
    } else {
        // 大数组或 hole 使用动态创建
        emit_op(s, OP_object);  // 创建数组对象
        // ...
    }
}
```

---

## 6. 作用域管理

### 6.1 作用域结构（行 21460-21480）

```c
typedef struct {
    int first;    // 作用域中第一个变量的索引
    int parent;   // 父作用域
} JSVarScope;

typedef struct {
    JSAtom var_name;      // 变量名
    int scope_level;      // 作用域层级
    int scope_next;       // 同一作用域的下一个变量
    uint8_t var_kind;     // VAR_kind: VAR, LET, CONST, ARG
    uint8_t is_local;     // 是否为局部变量
    int idx;              // 变量索引
} JSVarDef;
```

### 6.2 作用域层级

| Level | 含义 |
|-------|------|
| 0 | 函数参数 + var 变量作用域 |
| 1+ | let/const 块作用域 |
| -1 | 全局作用域 |

### 6.3 `push_scope()` / `pop_scope()`（行 23740-23800）

```c
static void push_scope(JSParseState *s)
{
    JSFunctionDef *fd = s->cur_func;
    if (fd->scope_count >= fd->scope_size) {
        js_resize_array(...);  // 扩展作用域数组
    }
    fd->scopes[fd->scope_count].first = fd->scope_first;
    fd->scopes[fd->scope_count].parent = fd->scope_level;
    fd->scope_first = -1;
    fd->scope_level++;
}
```

---

## 7. 指令（Directives）解析（行 35745）

```c
static __exception int js_parse_directives(JSParseState *s)
{
    if (s->token.val != TOK_STRING) return 0;

    while (s->token.val == TOK_STRING) {
        // 检查字符串内容
        if (!strcmp(str, "use strict")) {
            s->cur_func->has_use_strict = TRUE;
            s->cur_func->js_mode |= JS_MODE_STRICT;
        }
        // ASI 检查
        if (has_semi) break;
    }
    // 回退到 directive 之前的位置
    js_parse_seek_token(s, &pos);
}
```

---

## 8. 关键设计：LValue 处理

### 8.1 `get_lvalue()`（行 25361）

分析上一个 opcode，确定赋值目标的类型：

| 上一个 Opcode | LValue 类型 | Depth |
|--------------|------------|-------|
| `OP_scope_get_var` | 局部/全局变量 | 0 |
| `OP_get_field` | 对象属性 | 1 |
| `OP_scope_get_private_field` | 私有属性 | 1 |
| `OP_get_array_el` | 数组元素 | 2 |
| `OP_get_ref_value` | 闭包引用 | 2 |
| `OP_get_super_value` | super 属性 | 3 |

### 8.2 `put_lvalue()`（行 25505）

根据 lvalue 类型和特殊标志，生成对应的写入 opcode：

| 标志 | 含义 |
|------|------|
| `PUT_LVALUE_KEEP_TOP` | 保留栈顶值（如 `a = b = c`） |
| `PUT_LVALUE_KEEP_SECOND` | 保留第二个值（如 `++a`） |
| `PUT_LVALUE_NOKEEP` | 不保留（普通赋值） |

---

## 9. 未检查项

- `js_parse_delete()` 的完整实现
- `js_parse_with()` 的完整实现
- `js_parse_class()` 的完整实现（行 24702）
- `js_parse_import()` / `js_parse_export()` 模块解析
- `js_parse_destructuring_element()` 完整逻辑
- 错误恢复机制
- JSON 解析模式 (`ext_json`)
