# 06 - Runtime 模块

本模块分析 QuickJS 的运行时系统。

## 文件列表

| 文件 | 内容 |
|------|------|
| [00-overview.md](00-overview.md) | 运行时系统概览 |
| [context.md](context.md) | JSContext 设计详解 |
| [function-call.md](function-call.md) | 函数调用机制 |
| [scope-chain.md](scope-chain.md) | 作用域链实现 |

## 核心概念

### 1. 双层架构

```
JSRuntime (进程级)
    └── JSContext (每个执行环境)
            ├── 全局对象
            ├── 内置对象
            └── 模块系统
```

### 2. 内存管理集成

Runtime 直接管理:
- GC 堆
- 原子表 (Atoms)
- 类定义数组
- 形状哈希表

### 3. 中断机制

通过 `interrupt_counter` 实现:
- 执行时间限制
- 调试支持
- 协作式多任务

## 关键数据结构

### JSRuntime (行 239-326)

```c
struct JSRuntime {
    JSMallocFunctions mf;           // 内存分配器
    JSMallocState malloc_state;    // 内存统计
    
    // 原子管理
    int atom_hash_size;
    JSAtomStruct **atom_array;
    
    // 类定义
    int class_count;
    JSClass *class_array;
    
    // GC
    struct list_head gc_obj_list;
    struct list_head gc_zero_ref_count_list;
    
    // 栈管理
    uintptr_t stack_size;
    uintptr_t stack_limit;
    
    // 执行状态
    JSValue current_exception;
    struct JSStackFrame *current_stack_frame;
    
    // 作业队列 (Promise)
    struct list_head job_list;
    
    // 模块系统
    JSModuleNormalizeFunc *module_normalize_func;
};
```

### JSContext (行 448-493)

```c
struct JSContext {
    JSGCObjectHeader header;
    JSRuntime *rt;
    
    // 形状缓存
    JSShape *array_shape;
    JSShape *arguments_shape;
    
    // 内置构造函数
    JSValue *class_proto;
    JSValue function_proto;
    JSValue array_ctor;
    
    // 全局对象
    JSValue global_obj;
    JSValue global_var_obj;
    
    // 执行控制
    int interrupt_counter;
    
    // 模块
    struct list_head loaded_modules;
};
```

## 初始化流程

```
1. JS_NewRuntime()
   ├── JS_InitAtoms()           // 初始化原子表
   └── JS_InitBuiltinClasses()  // 注册内置类

2. JS_NewContext(Runtime)
   ├── JS_NewContextRaw()
   │   └── JS_AddIntrinsicBasicObjects()
   │       ├── Object.prototype
   │       ├── Function.prototype
   │       └── Global Object
   └── JS_AddIntrinsicBaseObjects()
       ├── Object, Function
       ├── Iterator
       └── Number, String, Boolean
```

## Go 移植要点

```go
type Runtime struct {
    // 内存管理
    malloc MallocFunctions
    
    // 原子系统
    atoms     map[string]*Atom
    atomArray []*Atom
    
    // 类系统
    classDef []ClassDefinition
    
    // GC
    gcObjects     []*GCObject
    gcZeroRefList []*GCObject
    
    // 执行
    stackLimit      uintptr
    currentException Value
    currentFrame    *StackFrame
    
    // Promise 作业
    jobQueue []Job
    
    // 模块
    moduleLoader ModuleLoader
}

type Context struct {
    rt *Runtime
    
    // 形状缓存
    arrayShape *Shape
    
    // 构造函数
    prototypes map[ClassID]*Object
    globals    map[string]Value
    
    // 执行控制
    interruptCounter int
    
    // 全局对象
    GlobalObject *Object
}
```

## 参考位置

| 功能 | quickjs.c 行号 |
|------|---------------|
| JSRuntime 结构 | 239 |
| JSContext 结构 | 448 |
| JS_NewRuntime | ~1640 |
| JS_NewContext | 2176-2229 |
| JS_CallInternal | 17372 |
| JS_GetPropertyInternal | 7805 |
| JS_SetPropertyInternal | 9258 |
