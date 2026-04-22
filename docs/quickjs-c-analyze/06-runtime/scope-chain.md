# 作用域链机制

## 概述

QuickJS 的作用域链用于变量解析，采用以下机制：
- **词法作用域**: 基于源代码结构，不依赖运行时调用栈
- **闭包**: 通过 JSVarRef 实现自由变量捕获
- **动态作用域**: eval 和 with 语句引入的动态作用域

## 闭包变量引用 (行 530-580)

### JSClosureVar 结构

```c
typedef struct JSClosureVar {
    JSClosureTypeEnum closure_type : 3;  // 闭包类型
    uint8_t is_lexical : 1;              // 是否词法变量
    uint8_t is_const : 1;                // 是否 const
    uint8_t var_kind : 4;               // 变量种类
    uint16_t var_idx;                    // 变量索引
    JSAtom var_name;                     // 变量名 (用于调试)
} JSClosureVar;

typedef enum {
    JS_CLOSURE_LOCAL,    // 父函数的局部变量
    JS_CLOSURE_ARG,      // 父函数的参数
    JS_CLOSURE_REF,      // 父函数的闭包变量
    JS_CLOSURE_GLOBAL_REF,   // 全局变量引用
    JS_CLOSURE_GLOBAL_DECL,  // eval 代码的全局声明
    JS_CLOSURE_GLOBAL,       // eval 代码的全局变量
    JS_CLOSURE_MODULE_DECL,   // 模块变量定义
    JS_CLOSURE_MODULE_IMPORT, // 模块导入
} JSClosureTypeEnum;
```

### JSVarRef 结构 (行 530-545)

```c
typedef struct JSVarRef {
    union {
        JSGCObjectHeader header;
        struct {
            int __gc_ref_count;
            uint8_t __gc_mark;
            uint8_t is_detached;
            uint8_t is_lexical;
            uint8_t is_const;
        };
    };
    JSValue *pvalue;    // 指向实际值的指针
    // ...
} JSVarRef;
```

## 闭包捕获机制

### 1. 编译时确定

函数字节码包含 `closure_var` 数组，记录所有需要捕获的变量：

```c
typedef struct JSFunctionBytecode {
    // ...
    JSClosureVar *closure_var;  // 闭包变量列表
    uint16_t var_ref_count;      // 变量引用计数
    // ...
} JSFunctionBytecode;
```

### 2. 运行时链接

当创建闭包时：

```c
// 在 JS_NewFromFunc bytecode 时
for(i = 0; i < b->closure_var_count; i++) {
    JSVarRef *var_ref;
    JSClosureVar *cv = &b->closure_var[i];
    
    // 根据变量类型获取引用
    switch(cv->closure_type) {
    case JS_CLOSURE_LOCAL:
    case JS_CLOSURE_ARG:
        // 获取父栈帧中的变量指针
        var_ref = js_new_var_ref(ctx, parent_var_buf + cv->var_idx);
        break;
    case JS_CLOSURE_REF:
        // 沿闭包链查找
        var_ref = find_var_ref(parent, cv->var_idx);
        break;
    }
    
    // 创建持久的引用对象
    func_obj->u.func.var_refs[i] = var_ref;
    var_ref->header.ref_count++;
}
```

### 3. 变量访问

闭包变量的访问通过 `JSVarRef` 间接访问：

```c
// 字节码: OP_get_var_ref
CASE(OP_get_var_ref):
{
    int var_idx = get_u16(pc);
    *sp++ = JS_DupValue(ctx, *var_refs[var_idx]->pvalue);
    pc += 2;
    BREAK;
}

// 字节码: OP_set_var_ref
CASE(OP_set_var_ref):
{
    int var_idx = get_u16(pc);
    JS_FreeValue(ctx, *var_refs[var_idx]->pvalue);
    *var_refs[var_idx]->pvalue = *--sp;
    pc += 2;
    BREAK;
}
```

## 作用域链构建

### 静态作用域 (词法作用域)

作用域在编译时确定，基于源代码结构：

```
Global Scope
    └── Function A
        └── Function B (closure capturing A's variables)
```

### 动态作用域 (eval/with)

```c
// eval 代码可以访问调用者的局部变量
CASE(OP_scope_eval):
{
    int scope_idx = get_u16(pc);
    // 创建 eval 专用作用域
    scope = js_clone_scope(ctx, current_scope, scope_idx);
    // 执行 eval 代码
    ret = eval_internal(scope, ...);
    BREAK;
}

// with 语句扩展作用域
CASE(OP_with):
{
    JSValue obj = *--sp;
    // 创建一个临时的 with 作用域
    with_scope = js_new_with_scope(ctx, current_scope, obj);
    // 在 with_scope 中执行后续代码
    execute_in_scope(with_scope, ...);
    BREAK;
}
```

## Go 实现建议

### 1. 闭包引用结构

```go
type VarRef struct {
    refCount int
    isDetached bool
    isLexical  bool
    isConst    bool
    value      *Value  // 指向实际值的指针
}

type Closure struct {
    Func   *BytecodeFunction
    VarRefs []*VarRef  // 与字节码的 closure_var 对应
}
```

### 2. 作用域链表示

```go
type Scope struct {
    Parent *Scope
    This   Value
    
    // 变量存储
    Vars map[string]Value
    
    // 特殊作用域
    WithObject Value  // with 语句的对象
    EvalScope  bool   // 是否为 eval 作用域
}

func (ctx *Context) resolveVariable(name string) (Value, error) {
    // 1. 先检查当前作用域
    for scope := ctx.currentScope; scope != nil; scope = scope.Parent {
        if val, ok := scope.Vars[name]; ok {
            return val, nil
        }
        // with 作用域的特殊处理
        if scope.WithObject != nil {
            if val, err := scope.WithObject.Get(ctx, name); err == nil {
                return val, nil
            }
        }
    }
    return UndefinedValue, ctx.ThrowReferenceError(name + " is not defined")
}
```

### 3. 闭包创建

```go
func (ctx *Context) newClosure(funcObj *Object, parentFrame *StackFrame, bc *FunctionBytecode) error {
    funcObj.VarRefs = make([]*VarRef, len(bc.ClosureVars))
    
    for i, cv := range bc.ClosureVars {
        var ref *VarRef
        
        switch cv.Type {
        case ClosureTypeLocal:
            // 从父栈帧获取变量
            ref = parentFrame.newVarRef(cv.Index)
            
        case ClosureTypeArg:
            // 从父栈帧获取参数
            ref = parentFrame.newVarRef(cv.Index)
            
        case ClosureTypeRef:
            // 沿闭包链向上查找
            ref = findVarRef(parentFrame, cv.Index)
            
        case ClosureTypeGlobalRef:
            // 获取全局变量引用
            ref = ctx.getGlobalVarRef(cv.Index)
        }
        
        funcObj.VarRefs[i] = ref
        ref.AddRef()
    }
    
    return nil
}
```

### 4. 变量访问操作

```go
func (frame *StackFrame) GetVarRef(idx int) Value {
    ref := frame.VarRefs[idx]
    if ref == nil {
        return UndefinedValue
    }
    return ref.value.Dup()
}

func (frame *StackFrame) SetVarRef(idx int, val Value) {
    ref := frame.VarRefs[idx]
    if ref == nil {
        return
    }
    if ref.isConst {
        // const 变量不能重新赋值
        return
    }
    ref.value.Free()
    ref.value = val.Dup()
}
```

## 陷阱规避

1. **闭包内存泄漏**: VarRef 必须正确管理引用计数，否则闭包持有的变量无法被 GC
2. **const 变量保护**: 闭包捕获后必须阻止对 const 变量的重新赋值
3. **with 作用域优先级**: with 对象的属性优先于普通作用域
4. **eval 的动态作用域**: eval 可以访问调用者的局部变量，这是设计要求
5. **模块作用域隔离**: 模块的 import 必须创建只读的绑定
6. **TDZ (Temporal Dead Zone)**: let/const 变量在声明前不能访问
