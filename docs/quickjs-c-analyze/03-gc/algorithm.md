# QuickJS 垃圾回收算法分析

## 1. GC 架构概览

QuickJS 使用**引用计数 + 标记-清除**混合算法:

```
┌─────────────────────────────────────────────────────────────────┐
│                        GC 架构                                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   引用计数 (Reference Counting)                                 │
│   ├── 处理: 大部分单对象释放                                     │
│   ├── 优点: 即时释放，无停顿                                     │
│   └── 缺点: 无法处理循环引用                                     │
│                                                                 │
│   标记-清除 (Mark-Sweep)                                        │
│   ├── 处理: 循环引用检测                                         │
│   ├── 触发: 内存阈值 / 手动触发                                  │
│   └── 优点: 能检测跨对象循环                                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 2. 核心数据结构

### 2.1 JSGCObjectHeader

```c
// quickjs.c:355-362
struct JSGCObjectHeader {
    int ref_count;              /* 引用计数，必须是第一个字段 */
    JSGCObjectTypeEnum gc_obj_type : 4;  /* 对象类型 */
    uint8_t mark : 1;           /* GC 标记位 */
    uint8_t dummy0 : 3;
    struct list_head link;      /* 链表链接 */
};
```

### 2.2 GC 对象类型

```c
// quickjs.c:344-350
typedef enum {
    JS_GC_OBJ_TYPE_JS_OBJECT,          // JavaScript 对象
    JS_GC_OBJ_TYPE_FUNCTION_BYTECODE,   // 函数字节码
    JS_GC_OBJ_TYPE_SHAPE,               // 对象形状
    JS_GC_OBJ_TYPE_VAR_REF,             // 变量引用
    JS_GC_OBJ_TYPE_ASYNC_FUNCTION,      // 异步函数
    JS_GC_OBJ_TYPE_JS_CONTEXT,          // JS 上下文
    JS_GC_OBJ_TYPE_MODULE,              // 模块
} JSGCObjectTypeEnum;
```

### 2.3 GC 阶段

```c
// quickjs.c:232-235
typedef enum {
    JS_GC_PHASE_NONE,        // 正常执行
    JS_GC_PHASE_DECREF,      // 递减引用计数阶段
    JS_GC_PHASE_REMOVE_CYCLES,  // 清除循环阶段
} JSGCPhaseEnum;
```

### 2.4 Runtime 中的 GC 相关字段

```c
// quickjs.c:258-263
struct JSRuntime {
    // ...
    struct list_head gc_obj_list;           // 所有 GC 对象链表
    struct list_head gc_zero_ref_count_list; // 零引用对象链表
    struct list_head tmp_obj_list;           // 临时对象链表
    JSGCPhaseEnum gc_phase : 8;              // 当前 GC 阶段
    size_t malloc_gc_threshold;             // GC 触发阈值
    // ...
};
```

## 3. 引用计数机制

### 3.1 引用计数增加

```c
// quickjs.h:698-706
static inline JSValue JS_DupValue(JSContext *ctx, JSValueConst v)
{
    if (JS_VALUE_HAS_REF_COUNT(v)) {
        JSGCObjectHeader *p = JS_VALUE_GET_PTR(v);
        p->ref_count++;
    }
    return (JSValue)v;
}

static inline JSValue JS_DupValueRT(JSRuntime *rt, JSValueConst v)
{
    if (JS_VALUE_HAS_REF_COUNT(v)) {
        JSGCObjectHeader *p = JS_VALUE_GET_PTR(v);
        p->ref_count++;
    }
    return (JSValue)v;
}
```

**宏定义** (`quickjs.h:284`):
```c
#define JS_VALUE_HAS_REF_COUNT(v) \
    ((unsigned)JS_VALUE_GET_TAG(v) >= (unsigned)JS_TAG_FIRST)
```
- 只有负数 tag (heap 对象) 才有引用计数
- 立即数 (int, bool, null, undefined) 无需引用计数

### 3.2 引用计数减少与释放

```c
// quickjs.c:6027-6098
void __JS_FreeValueRT(JSRuntime *rt, JSValue v)
{
    uint32_t tag = JS_VALUE_GET_TAG(v);
    
    switch(tag) {
    case JS_TAG_STRING:
        {
            JSString *p = JS_VALUE_GET_STRING(v);
            if (--p->header.ref_count <= 0) {
                js_free_rt(rt, p);
            }
        }
        break;
        
    case JS_TAG_STRING_ROPE:
        // 递归释放左右子树
        JS_FreeValueRT(rt, p->left);
        JS_FreeValueRT(rt, p->right);
        js_free_rt(rt, p);
        break;
        
    case JS_TAG_OBJECT:
    case JS_TAG_FUNCTION_BYTECODE:
    case JS_TAG_MODULE:
        {
            JSGCObjectHeader *p = JS_VALUE_GET_PTR(v);
            if (rt->gc_phase != JS_GC_PHASE_REMOVE_CYCLES) {
                list_del(&p->link);              // 从 gc_obj_list 移除
                list_add(&p->link, &rt->gc_zero_ref_count_list);
                p->mark = 1;                      // 标记为即将释放
                if (rt->gc_phase == JS_GC_PHASE_NONE) {
                    free_zero_refcount(rt);      // 立即处理
                }
            }
        }
        break;
        
    case JS_TAG_BIG_INT:
        // BigInt 直接释放 (无子引用)
        js_free_rt(rt, JS_VALUE_GET_PTR(v));
        break;
        
    case JS_TAG_SYMBOL:
        // Symbol 释放 (可能是 atom)
        JS_FreeAtomStruct(rt, JS_VALUE_GET_PTR(v));
        break;
    }
}
```

### 3.3 零引用对象处理

```c
// quickjs.c:6010-6024
static void free_zero_refcount(JSRuntime *rt)
{
    JSGCObjectHeader *p;
    
    rt->gc_phase = JS_GC_PHASE_DECREF;
    
    while (!list_empty(&rt->gc_zero_ref_count_list)) {
        p = list_entry(rt->gc_zero_ref_count_list.next, 
                       JSGCObjectHeader, link);
        free_gc_object(rt, p);  // 释放对象及其子引用
    }
    
    rt->gc_phase = JS_GC_PHASE_NONE;
}
```

## 4. 标记-清除算法

### 4.1 触发条件

```c
// quickjs.c:1360-1372
void JS_RunGC(JSRuntime *rt)
{
    JS_RunGCInternal(rt, TRUE);
}

// 内存阈值检查 (在内存分配时)
static void *js_malloc_internal(...)
{
    if (rt->malloc_state.malloc_size > rt->malloc_gc_threshold) {
        JS_RunGC(rt);  // 触发 GC
        rt->malloc_gc_threshold = rt->malloc_state.malloc_size + 
                                   256 * 1024;
    }
}
```

### 4.2 GC 算法流程

```
JS_RunGC()
    │
    ├─► gc_remove_weak_objects()     // 处理弱引用
    │
    ├─► gc_decref()                   // 第一阶段: 递减所有子对象引用
    │      for each obj in gc_obj_list:
    │          mark_children(obj, gc_decref_child)
    │          obj.mark = 1
    │          if obj.ref_count == 0:
    │              move to tmp_obj_list
    │
    ├─► gc_scan()                      // 第二阶段: 扫描保留存活对象
    │      for each obj in tmp_obj_list:
    │          if obj.ref_count > 0:
    │              obj.ref_count++     // 恢复引用
    │              mark_children()     // 递归标记
    │              move to gc_obj_list
    │
    ├─► gc_free_cycles()               // 第三阶段: 释放循环垃圾
    │      for each obj in tmp_obj_list:
    │          if obj.gc_obj_type is collectable:
    │              free_gc_object()   // 释放循环中的对象
    │
    └─► free_zero_refcount()          // 第四阶段: 处理新零引用
```

### 4.3 第一阶段: gc_decref

```c
// quickjs.c:6282-6312
static void gc_decref_child(JSRuntime *rt, JSGCObjectHeader *p)
{
    assert(p->ref_count > 0);
    p->ref_count--;
    if (p->ref_count == 0 && p->mark == 1) {
        list_del(&p->link);
        list_add_tail(&p->link, &rt->tmp_obj_list);
    }
}

static void gc_decref(JSRuntime *rt)
{
    JSGCObjectHeader *p;
    
    init_list_head(&rt->tmp_obj_list);
    
    list_for_each_safe(el, el1, &rt->gc_obj_list) {
        p = list_entry(el, JSGCObjectHeader, link);
        assert(p->mark == 0);
        mark_children(rt, p, gc_decref_child);  // 递归递减
        p->mark = 1;                              // 标记已处理
        if (p->ref_count == 0) {
            list_del(&p->link);
            list_add_tail(&p->link, &rt->tmp_obj_list);
        }
    }
}
```

### 4.4 第二阶段: gc_scan

```c
// quickjs.c:6315-6350
static void gc_scan_incref_child(JSRuntime *rt, JSGCObjectHeader *p)
{
    p->ref_count++;
    if (p->ref_count == 1) {
        // ref_count 从 0 变为 1: 从 tmp_obj_list 移到 gc_obj_list
        list_del(&p->link);
        list_add_tail(&p->link, &rt->gc_obj_list);
        p->mark = 0;  // 重置标记
    }
}

static void gc_scan_incref_child2(JSRuntime *rt, JSGCObjectHeader *p)
{
    p->ref_count++;  // 简单递增
}

static void gc_scan(JSRuntime *rt)
{
    JSGCObjectHeader *p;
    
    list_for_each_safe(el, el1, &rt->tmp_obj_list) {
        p = list_entry(el, JSGCObjectHeader, link);
        p->ref_count++;
        mark_children(rt, p, gc_scan_incref_child);
        // 如果 ref_count > 1，对象存活
        // 如果 ref_count == 1，对象可能被循环引用持有
    }
}
```

### 4.5 第三阶段: gc_free_cycles

```c
// quickjs.c:6352-6408
static void gc_free_cycles(JSRuntime *rt)
{
    rt->gc_phase = JS_GC_PHASE_REMOVE_CYCLES;
    
    // 释放 tmp_obj_list 中剩余的对象
    for (;;) {
        el = rt->tmp_obj_list.next;
        if (el == &rt->tmp_obj_list) break;
        p = list_entry(el, JSGCObjectHeader, link);
        
        switch(p->gc_obj_type) {
        case JS_GC_OBJ_TYPE_JS_OBJECT:
        case JS_GC_OBJ_TYPE_FUNCTION_BYTECODE:
        case JS_GC_OBJ_TYPE_ASYNC_FUNCTION:
        case JS_GC_OBJ_TYPE_MODULE:
            free_gc_object(rt, p);  // 强制释放
            break;
        default:
            // 非循环垃圾对象移到 zero_ref_count_list
            list_del(&p->link);
            list_add_tail(&p->link, &rt->gc_zero_ref_count_list);
            break;
        }
    }
    
    // 处理 weakref 引用
    list_for_each_safe(el, el1, &rt->gc_zero_ref_count_list) {
        p = list_entry(el, JSGCObjectHeader, link);
        // 检查 weakref 引用
        if (p->gc_obj_type == JS_GC_OBJ_TYPE_JS_OBJECT &&
            ((JSObject*)p)->weakref_count != 0) {
            p->mark = 0;  // 保留有 weakref 引用的对象
        } else {
            js_free_rt(rt, p);  // 真正释放
        }
    }
}
```

## 5. 子对象标记 (mark_children)

```c
// quickjs.c:6163-6278
static void mark_children(JSRuntime *rt, JSGCObjectHeader *gp,
                          JS_MarkFunc *mark_func)
{
    switch(gp->gc_obj_type) {
    case JS_GC_OBJ_TYPE_JS_OBJECT:
        {
            JSObject *p = (JSObject *)gp;
            // 标记 shape
            mark_func(rt, &p->shape->header);
            
            // 标记所有属性
            for (i = 0; i < p->shape->prop_count; i++) {
                if (prs->flags & JS_PROP_TMASK) {
                    if (JS_PROP_GETSET) mark_func(getter);
                    if (JS_PROP_VARREF) mark_func(var_ref);
                    // ...
                } else {
                    JS_MarkValue(rt, p->prop[i].u.value);
                }
            }
            
            // 调用 class 的 gc_mark 回调
            if (p->class_id != JS_CLASS_OBJECT) {
                gc_mark = rt->class_array[p->class_id].gc_mark;
                if (gc_mark) gc_mark(rt, obj);
            }
        }
        break;
        
    case JS_GC_OBJ_TYPE_FUNCTION_BYTECODE:
        {
            JSFunctionBytecode *b = (JSFunctionBytecode *)gp;
            // 标记常量池
            for (i = 0; i < b->cpool_count; i++)
                JS_MarkValue(rt, b->cpool[i]);
            // 标记 realm
            mark_func(rt, &b->realm->header);
        }
        break;
        
    case JS_GC_OBJ_TYPE_VAR_REF:
        // 标记栈帧或变量
        break;
        
    case JS_GC_OBJ_TYPE_ASYNC_FUNCTION:
        // 标记栈帧、参数、闭包
        break;
        
    case JS_GC_OBJ_TYPE_SHAPE:
        // 标记 prototype
        break;
        
    // ...
    }
}
```

## 6. 对象类型与 GC 策略

| 对象类型 | 引用关系 | GC 策略 |
|---------|---------|--------|
| JSObject | ref_count + 属性引用 | 正常 RC |
| FunctionBytecode | ref_count + 常量池 | 正常 RC |
| Shape | ref_count + prototype | 正常 RC，共享可能 |
| VarRef | ref_count + 栈帧 | 特殊处理 |
| AsyncFunction | ref_count + 栈 | 循环检测 |
| Module | ref_count + 导出/导入 | 循环检测 |
| String | ref_count | 正常 RC |
| BigInt | 无子引用 | 直接释放 |
| Symbol | 可能为 atom | atom 特殊处理 |

## 7. 代码位置索引

| 功能 | 文件位置 |
|------|---------|
| JSGCObjectHeader | `quickjs.c:355-362` |
| GC 对象类型 | `quickjs.c:344-350` |
| GC 阶段枚举 | `quickjs.c:232-235` |
| JS_DupValue | `quickjs.h:698-706` |
| __JS_FreeValueRT | `quickjs.c:6027-6098` |
| free_zero_refcount | `quickjs.c:6010-6024` |
| JS_RunGC | `quickjs.c:6410-6432` |
| gc_decref | `quickjs.c:6282-6312` |
| gc_scan | `quickjs.c:6315-6350` |
| gc_free_cycles | `quickjs.c:6352-6408` |
| mark_children | `quickjs.c:6163-6278` |
| JS_MarkValue | `quickjs.c` |