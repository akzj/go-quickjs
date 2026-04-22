# 作用域管理

## 1. 核心数据结构

### 1.1 `JSFunctionDef` 中的作用域字段（行 21440-21480）

```c
typedef struct JSFunctionDef {
    // ...
    int scope_level;           // 当前作用域层级
    int scope_first;          // 当前作用域第一个变量的索引
    int scope_size;           // scopes 数组分配大小
    int scope_count;          // scopes 数组已使用大小
    JSVarScope *scopes;       // 作用域数组
    JSVarScope def_scope_array[4];  // 内联数组（避免小分配）

    // 变量定义
    JSVarDef *vars;           // 变量定义数组
    int var_size;
    int var_count;
    JSVarDef *args;           // 参数定义数组
    int arg_size;
    int arg_count;
    int defined_arg_count;

    // 特殊变量索引
    int var_ref_count;        // 局部/参数变量引用计数
    int var_object_idx;       // arguments 对象索引（-1 = 无）
    int this_var_idx;         // this 绑定索引
    int new_target_var_idx;   // new.target 索引
    // ...
} JSFunctionDef;
```

### 1.2 `JSVarScope` 作用域描述符（行 21458）

```c
typedef struct JSVarScope {
    int first;    // 作用域中第一个变量的 vd[] 索引
    int parent;   // 父作用域的 scope_level（-1 = 无父）
} JSVarScope;
```

### 1.3 `JSVarDef` 变量定义（行 21400）

```c
typedef struct JSVarDef {
    JSAtom var_name;      // 变量名
    int scope_level;      // 定义时的作用域层级
    int scope_next;       // 同一作用域的下一个变量（链表）
    uint8_t var_kind;     // 变量种类
    BOOL is_local;        // 是否为局部变量
    int idx;              // 变量索引（栈帧偏移或作用域偏移）
} JSVarDef;
```

### 1.4 变量种类（`JSVarKind`）

```c
typedef enum {
    JS_VAR_FUNCTION,      // 函数声明（var）
    JS_VAR_LET,           // let 声明
    JS_VAR_CONST,         // const 声明
    JS_VAR_ARG,           // 函数参数
    JS_VAR_CATCH,         // catch 绑定
} JSVarKind;
```

---

## 2. 作用域层级体系

### 2.1 层级划分

| scope_level | 含义 | 包含 |
|-------------|------|------|
| 0 | 函数作用域（var 参数/变量） | var 参数、`var` 声明 |
| 1+ | 块作用域（let/const） | `let`/`const` 块、for 循环、catch |
| -1 | 全局作用域 | 全局 `var` 声明 |

### 2.2 作用域初始化（行 31535-31550）

```c
fd->scopes = fd->def_scope_array;
fd->scope_size = countof(fd->def_scope_array);
fd->scope_count = 1;
fd->scopes[0].first = -1;      // 无变量
fd->scopes[0].parent = -1;     // 无父作用域
fd->scope_level = 0;           // 从函数作用域开始
fd->scope_first = -1;
```

### 2.3 块作用域创建 `push_scope()`（行 23740）

```c
static void push_scope(JSParseState *s)
{
    JSFunctionDef *fd = s->cur_func;

    // 扩展作用域数组（如果需要）
    if (fd->scope_count >= fd->scope_size) {
        js_resize_array(s->ctx, (void **)&fd->scopes, ...);
    }

    // 创建新作用域
    fd->scopes[fd->scope_count].first = fd->scope_first;  // 继承父作用域变量
    fd->scopes[fd->scope_count].parent = fd->scope_level;  // 记录父作用域
    fd->scope_first = -1;    // 新作用域暂无数组
    fd->scope_level++;       // 进入更深层级
}
```

### 2.4 块作用域销毁 `pop_scope()`

```c
static void pop_scope(JSParseState *s)
{
    JSFunctionDef *fd = s->cur_func;

    // 将当前作用域的变量链接回父作用域
    JSVarScope *cur = &fd->scopes[fd->scope_level];
    if (cur->first >= 0) {
        int i = cur->first;
        while (fd->vars[i].scope_next >= 0)
            i = fd->vars[i].scope_next;
        fd->vars[i].scope_next = fd->scope_first;
    }
    fd->scope_first = cur->first;

    // 恢复父作用域
    fd->scope_level = fd->scopes[fd->scope_level].parent;
}
```

---

## 3. 变量定义

### 3.1 `js_define_var()`（行 23640）

```c
static int js_define_var(JSParseState *s, JSAtom name, int tok)
{
    JSFunctionDef *fd = s->cur_func;

    // 1. 检查重复定义
    for (i = fd->scope_first; i >= 0; i = fd->vars[i].scope_next) {
        if (fd->vars[i].var_name == name) {
            if (fd->vars[i].scope_level == fd->scope_level) {
                // 同一作用域重复定义
                return js_parse_error(s, "invalid redefinition");
            }
            // 允许外层作用域同名（遮蔽）
        }
    }

    // 2. 添加变量定义
    if (fd->var_count >= fd->var_size) {
        js_resize_array(s->ctx, (void **)&fd->vars, ...);
    }
    vd = &fd->vars[fd->var_count++];
    vd->var_name = name;
    vd->scope_level = fd->scope_level;
    vd->scope_next = fd->scope_first;
    vd->var_kind = (tok == TOK_VAR) ? JS_VAR_FUNCTION : JS_VAR_LET;
    vd->is_local = TRUE;

    // 3. 分配索引
    if (tok == TOK_VAR && fd->scope_level == 0) {
        vd->idx = fd->var_count - 1;  // var 在函数作用域用索引
    } else {
        vd->idx = -1;  // let/const 索引在字节码生成时确定
    }

    fd->scope_first = fd->var_count - 1;
    return 0;
}
```

### 3.2 `add_var()` — 添加变量（用于 eval/全局）

```c
static int add_var(JSContext *ctx, JSFunctionDef *fd, JSAtom name)
{
    // 类似于 js_define_var，但不检查重复
    // 用于全局变量、eval 变量等
}
```

---

## 4. 变量查找

### 4.1 `find_var()` — 查找变量（行 23438）

```c
static int find_var(JSContext *ctx, JSFunctionDef *fd, JSAtom name)
{
    // 1. 在当前作用域链中查找
    int scope = fd->scope_level;
    while (scope >= 0) {
        int idx = find_var_in_scope(ctx, fd, name, scope);
        if (idx >= 0) return idx;
        scope = fd->scopes[scope].parent;
    }

    // 2. 全局查找（eval 或全局代码）
    if (fd->is_eval || fd->is_global_var) {
        // 返回 -1 表示需要全局查找
    }

    return -1;
}
```

### 4.2 `find_var_in_scope()` — 在特定作用域查找（行 23451）

```c
static int find_var_in_scope(JSContext *ctx, JSFunctionDef *fd,
                             JSAtom name, int scope_level)
{
    // 遍历同一作用域的所有变量
    for (scope_idx = fd->scopes[scope_level].first; scope_idx >= 0;
         scope_idx = fd->vars[scope_idx].scope_next) {
        if (fd->vars[scope_idx].scope_level != scope_level) break;
        if (fd->vars[scope_idx].var_name == name)
            return scope_idx;
    }
    return -1;
}
```

### 4.3 `find_arg()` — 查找参数（行 23425）

```c
static int find_arg(JSContext *ctx, JSFunctionDef *fd, JSAtom name)
{
    for (i = fd->arg_count; i-- > 0;) {
        if (fd->args[i].var_name == name)
            return i | ARGUMENT_VAR_OFFSET;  // 标记为参数
    }
    return -1;
}
```

---

## 5. 闭包变量捕获

### 5.1 闭包变量结构（行 21508）

```c
typedef struct JSClosureVar {
    int cpool_idx;        // 父函数的常量池索引
    uint8_t is_arg;       // 是否为参数
    uint8_t is_const;     // 是否为 const
    uint8_t is_lexical;   // 是否为词法作用域
    uint8_t scope_level;  // 作用域层级
    int var_index;        // 变量索引
} JSClosureVar;
```

### 5.2 闭包创建过程

1. **解析时**：记录对外部变量的引用
2. **编译时**：`js_close_function_def()` 收集闭包变量
3. **运行时**：创建闭包时复制捕获的变量

```c
static void js_close_function_def(JSParseState *s, JSFunctionDef *fd)
{
    // 1. 收集被引用的外部变量
    for (i = 0; i < fd->var_count; i++) {
        JSVarDef *vd = &fd->vars[i];
        if (vd->scope_level > 0) {  // 非函数作用域变量
            if (is_referenced(fd, vd)) {
                // 添加到闭包变量数组
                closure_var[closure_var_count++] = ...;
            }
        }
    }

    // 2. 分配栈帧空间
    // ...
}
```

---

## 6. 作用域与字节码生成

### 6.1 变量读取字节码

```c
// 作用域变量（let/const/外层 var）
emit_op(s, OP_scope_get_var);
emit_atom(s, name);           // 变量名 Atom
emit_u16(s, scope_level);     // 作用域层级

// 局部变量（当前作用域内的 let/const）
emit_op(s, OP_get_loc);
emit_u16(s, var_index);
```

### 6.2 变量写入字节码

```c
// 作用域变量
emit_op(s, OP_scope_put_var);
emit_atom(s, name);
emit_u16(s, scope_level);

// 局部变量
emit_op(s, OP_set_loc);
emit_u16(s, var_index);

// 不关心原值（用于初始化）
emit_op(s, OP_set_orth);
emit_u16(s, var_index);
```

### 6.3 `with` 语句处理（特殊作用域）

`with` 语句会创建动态作用域，需要特殊处理：

```c
// 检测 with 作用域
if (has_with_scope(fd, scope)) {
    depth = 2;  // OP_scope_get_var → OP_get_ref_value
}
```

---

## 7. 特殊变量索引

| 索引 | 含义 | 用途 |
|------|------|------|
| `var_object_idx` | arguments 对象 | 访问 `arguments` |
| `eval_ret_idx` | eval 返回值 | `$ret` |
| `this_var_idx` | this 绑定 | `this` |
| `new_target_var_idx` | new.target | `new.target` |
| `arguments_arg_idx` | arguments 参数 | 参数作用域中的 arguments |
| `func_var_idx` | 函数自身 | 递归调用 |

---

## 8. 严格模式

### 8.1 严格模式检测

```c
// js_parse_directives() 中
if (!strcmp(str, "use strict")) {
    fd->has_use_strict = TRUE;
    fd->js_mode |= JS_MODE_STRICT;
}
```

### 8.2 严格模式限制

```c
// 删除变量
if (name == JS_ATOM_eval || name == JS_ATOM_arguments) {
    if (fd->js_mode & JS_MODE_STRICT)
        return js_parse_error(s, "invalid lvalue in strict mode");
}

// with 语句
if (fd->js_mode & JS_MODE_STRICT)
    return js_parse_error(s, "with statements not allowed in strict mode");
```

---

## 9. 未检查项

- `is_child_scope()` 闭包分析
- `find_var_in_child_scope()` 实现
- 全局变量 hoisting 机制
- 模块作用域处理
- 迭代器变量（for-of 中的迭代变量）
- catch 作用域的变量处理
