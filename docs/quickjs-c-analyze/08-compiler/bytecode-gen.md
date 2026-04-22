# 字节码生成（Compiler）

## 1. 编译入口（行 36496-36537）

QuickJS 使用 **Single-Pass 编译**：Parser 在递归下降过程中直接 emit 字节码，无需 AST 中间表示。

### `js_parse_program()` 流程

```c
static __exception int js_parse_program(JSParseState *s)
{
    // 1. 词法分析第一个 token
    if (next_token(s)) return -1;

    // 2. 解析 "use strict" 等指令
    if (js_parse_directives(s)) return -1;

    // 3. 确定变量作用域规则
    fd->is_global_var = (fd->eval_type == JS_EVAL_TYPE_GLOBAL) ||
                        (fd->eval_type == JS_EVAL_TYPE_MODULE) ||
                        !(fd->js_mode & JS_MODE_STRICT);

    // 4. 添加隐藏返回值变量（脚本模式）
    if (!s->is_module) {
        fd->eval_ret_idx = add_var(s->ctx, fd, JS_ATOM__ret_);
    }

    // 5. 循环解析顶层语句
    while (s->token.val != TOK_EOF) {
        if (js_parse_source_element(s)) return -1;
    }

    // 6. 生成 return 语句
    emit_return(s, TRUE);
    return 0;
}
```

---

## 2. 字节码缓冲区

### 2.1 `JSFunctionDef` 中的字节码字段（行 21530-21537）

```c
typedef struct JSFunctionDef {
    // ...
    DynBuf byte_code;              // 字节码动态缓冲区
    int last_opcode_pos;           // 最后一个 opcode 的位置（用于 lvalue 分析）
    const uint8_t *last_opcode_source_ptr;  // 源码位置（行号）
    BOOL use_short_opcodes;        // 是否使用短 opcode 编码
    // ...
} JSFunctionDef;
```

### 2.2 `DynBuf` 动态缓冲区

```c
typedef struct DynBuf {
    uint8_t *buf;     // 缓冲区起始
    size_t size;      // 已使用大小
    size_t allocated; // 已分配大小
    BOOL error;       // 错误标志
    void * opaque;    // 底层分配器
} DynBuf;
```

---

## 3. Emit 函数

### 3.1 基本 Emit 函数（行 23265-23320）

```c
// 写入 1 字节
static void emit_u8(JSParseState *s, uint8_t val)
{
    dbuf_putc(&s->cur_func->byte_code, val);
}

// 写入 2 字节（小端序）
static void emit_u16(JSParseState *s, uint16_t val)
{
    dbuf_put_u16(&s->cur_func->byte_code, val);
}

// 写入 4 字节（小端序）
static void emit_u32(JSParseState *s, uint32_t val)
{
    dbuf_put_u32(&s->cur_func->byte_code, val);
}

// 写入 Opcode
static void emit_op(JSParseState *s, uint8_t val)
{
    fd = s->cur_func;
    fd->last_opcode_pos = bc->size;  // 更新位置
    dbuf_putc(&fd->byte_code, val);
}

// 写入 Atom（4 字节索引）
static void emit_atom(JSParseState *s, JSAtom name)
{
    dbuf_put_u32(&s->cur_func->byte_code, JS_DupAtom(s->ctx, name));
}

// 写入源码位置（行号信息）
static void emit_source_pos(JSParseState *s, const uint8_t *source_ptr)
{
    if (fd->last_opcode_source_ptr != source_ptr) {
        dbuf_putc(bc, OP_line_num);
        dbuf_put_u32(bc, source_ptr - s->buf_start);
        fd->last_opcode_source_ptr = source_ptr;
    }
}
```

### 3.2 标签（Label）系统（行 23335-23400）

```c
typedef struct {
    int ref_count;  // 引用计数
    int pos;        // 标签位置（回填用）
    int pos2;       // 辅助位置
    int addr;       // 绝对地址
    JumpSlot *first_reloc;
} LabelSlot;

// 创建新标签
static int new_label(JSParseState *s)
{
    label = fd->label_count++;
    ls = &fd->label_slots[label];
    ls->ref_count = 0;
    ls->pos = -1;
    return label;
}

// 写入标签
static void emit_label_raw(JSParseState *s, int label)
{
    emit_u8(s, OP_label);
    emit_u32(s, label);
    s->cur_func->label_slots[label].pos = bc->size;
}

// emit_goto — 生成跳转指令
static int emit_goto(JSParseState *s, int opcode, int label)
{
    if (!js_is_live_code(s)) return -1;  // 死代码检测
    if (label < 0) label = new_label(s);
    emit_op(s, opcode);
    emit_u32(s, label);
    s->cur_func->label_slots[label].ref_count++;
    return label;
}
```

---

## 4. 常量池

### 4.1 `cpool_add()`（行 23389）

```c
static int cpool_add(JSParseState *s, JSValue val)
{
    fd = s->cur_func;
    // 动态扩展 cpool 数组
    if (js_resize_array(s->ctx, &fd->cpool, sizeof(fd->cpool[0]),
                        &fd->cpool_size, fd->cpool_count + 1)) {
        JS_FreeValue(s->ctx, val);
        return -1;
    }
    fd->cpool[fd->cpool_count++] = val;
    return fd->cpool_count - 1;
}
```

### 4.2 `emit_push_const()`（行 23402）

```c
static __exception int emit_push_const(JSParseState *s, JSValueConst val, BOOL as_atom)
{
    // 如果是字符串且 as_atom=TRUE，尝试作为 Atom
    if (JS_VALUE_GET_TAG(val) == JS_TAG_STRING && as_atom) {
        atom = JS_NewAtomStr(s->ctx, JS_VALUE_GET_STRING(val));
        if (atom != JS_ATOM_NULL && !__JS_AtomIsTaggedInt(atom)) {
            emit_op(s, OP_push_atom_value);
            emit_u32(s, atom);
            return 0;
        }
    }
    // 否则添加到常量池
    idx = cpool_add(s, JS_DupValue(s->ctx, val));
    if (idx < 0) return -1;
    emit_op(s, OP_push_const);
    emit_u32(s, idx);
    return 0;
}
```

---

## 5. 变量操作字节码

### 5.1 变量读取

```c
// 作用域变量读取
emit_op(s, OP_scope_get_var);
emit_atom(s, name);       // 变量名 Atom
emit_u16(s, scope_level); // 作用域层级

// 局部变量（let/const/arg）— 基于栈帧偏移
emit_op(s, OP_get_loc);
emit_u16(s, var_index);

// 临时值
emit_op(s, OP_push_i32);
emit_u32(s, value);

// 常量
emit_op(s, OP_push_const);
emit_u32(s, cpool_index);
```

### 5.2 变量写入

```c
// 作用域变量写入
emit_op(s, OP_scope_put_var);
emit_atom(s, name);
emit_u16(s, scope_level);

// 局部变量写入
emit_op(s, OP_set_loc);
emit_u16(s, var_index);

// 不关心原值
emit_op(s, OP_set_orth);
emit_u16(s, var_index);
```

### 5.3 LValue 分析（行 25361-25502）

```c
// get_lvalue 分析上一个 opcode，确定赋值目标类型
static __exception int get_lvalue(JSParseState *s, int *popcode, int *pscope,
                                  JSAtom *pname, int *plabel, int *pdepth,
                                  BOOL keep, int tok)
{
    fd = s->cur_func;
    opcode = get_prev_opcode(fd);

    switch(opcode) {
    case OP_scope_get_var:      // 变量名 + 作用域层级
        name = get_u32(...);
        scope = get_u16(...);
        depth = (has_with_scope) ? 2 : 0;
        break;
    case OP_get_field:          // obj.prop
        name = get_u32(...);
        depth = 1;
        break;
    case OP_scope_get_private_field:  // obj.#private
        name = get_u32(...);
        scope = get_u16(...);
        depth = 1;
        break;
    case OP_get_array_el:       // obj[key]
        depth = 2;
        break;
    case OP_get_super_value:    // super.prop
        depth = 3;
        break;
    default:
        // 非法赋值目标
        return js_parse_error(s, "invalid assignment left-hand side");
    }

    // 删除上一个 opcode（因为我们要重新 emit 写入）
    fd->byte_code.size = fd->last_opcode_pos;
}
```

---

## 6. 常见模式的字节码生成

### 6.1 变量声明 `var x = 1;`

```javascript
var x = 1;
```
```
OP_push_i32  1        ; push 1
OP_set_orth  x         ; set x (不关心原值)
OP_drop                ; pop 栈
```

### 6.2 条件表达式 `a ? b : c`

```javascript
a ? b : c
```
```
push a                 ; condition
OP_if_false  L1        ; if !a goto L1
push b                 ; then branch
OP_goto     L2         ; skip else
OP_label   L1:
push c                 ; else branch
OP_label   L2:
```

### 6.3 逻辑与 `a && b`

```javascript
a && b
```
```
push a
OP_dup               ; dup a (条件为真时保留)
push b
OP_if_false L1       ; if !a goto L1 (跳过 b)
OP_drop              ; pop dup 的值
OP_label L1:
; 栈: false 时为 a，true 时为 b
```

### 6.4 逻辑或 `a || b`

```javascript
a || b
```
```
push a
OP_dup
push b
OP_if_true L1        ; if a goto L1
OP_drop
OP_label L1:
```

### 6.5 函数调用 `f(a, b)`

```javascript
f(a, b)
```
```
push f               ; function reference
push a               ; argument 1
push b               ; argument 2
OP_call      2       ; call with 2 arguments
```

### 6.6 属性访问 `obj.prop`

```javascript
obj.prop
```
```
push obj
OP_get_field  "prop" ; get property "prop"
```

### 6.7 函数声明

```javascript
function foo() { return 1; }
```
```
OP_closure    cpool_idx  ; 创建闭包（字节码来自 cpool）
OP_set_loc    foo        ; 存入 foo
OP_drop
```

---

## 7. Return 语句生成（行 27820-27894）

```c
static void emit_return(JSParseState *s, BOOL hasval)
{
    BlockEnv *top = s->cur_func->top_break;

    // 1. 处理 finally 块
    while (top) {
        if (top->has_iterator || top->label_finally != -1) {
            if (!hasval) {
                emit_op(s, OP_undefined);
                hasval = TRUE;
            }
            emit_op(s, OP_nip_catch);  // 清理栈

            if (top->has_iterator) {
                // for-of/for-in 迭代器关闭
                emit_op(s, OP_iterator_close);
            } else {
                // 执行 finally
                emit_goto(s, OP_gosub, top->label_finally);
            }
        }
        top = top->prev;
    }

    // 2. 派生类构造函数检查
    if (s->cur_func->is_derived_class_constructor) {
        emit_op(s, OP_check_ctor_return);
        label = emit_goto(s, OP_if_false, -1);
        emit_op(s, OP_drop);  // 丢弃返回值
    }

    // 3. 生成 return opcode
    emit_source_pos(s, ...);
    emit_op(s, OP_return);
    if (!hasval)
        emit_op(s, OP_return_undef);
}
```

---

## 8. Short Opcodes（字节优化）

QuickJS 支持将 1 字节 opcode 压缩为更短的编码：

```c
#if SHORT_OPCODES
#define short_opcode_info(op) \
    opcode_info[(op) >= OP_TEMP_START ? \
                (op) + (OP_TEMP_END - OP_TEMP_START) : (op)]
#else
#define short_opcode_info(op) opcode_info[op]
#endif
```

在最终编译阶段（`js_compile_function()`），会将常规 opcode 替换为短 opcode。

---

## 9. 闭包和常量池索引

### 9.1 嵌套函数

```javascript
function outer() {
    let x = 1;
    function inner() {
        return x;  // 引用外层变量
    }
    return inner;
}
```
```
; outer 函数
OP_push_i32  1
OP_set_loc   x          ; x = 1
OP_closure   inner_cpool_idx  ; 创建 inner 闭包
; 闭包字节码中包含 x 的引用
OP_return
```

### 9.2 闭包变量捕获

```c
typedef struct {
    int cpool_idx;    // 常量池索引（父函数的 JSFunctionDef）
    uint8_t is_arg;
    uint8_t is_const;
    uint8_t is_lexical;
    uint8_t scope_level;
    int var_index;
} JSClosureVar;
```

---

## 10. 未检查项

- `js_compile_function()` 完整实现
- Short opcode 压缩算法
- Bytecode 序列化/反序列化
- 字节码验证（security check）
- pc2line 表的生成
- 函数重载参数处理
- 迭代器闭包优化
