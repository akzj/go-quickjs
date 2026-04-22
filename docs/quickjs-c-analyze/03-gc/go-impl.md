# QuickJS GC Go 实现方案

## 1. 核心设计决策

### 1.1 不要翻译 C 的 GC

**关键洞察**: Go 有自己的垃圾回收器。翻译 C 的引用计数 + 标记清除是**错误的方向**。

| C GC 机制 | Go 对应 |
|-----------|---------|
| 引用计数 | Go runtime GC (通常足够) |
| 标记-清除 | Go runtime GC (自动) |
| GC 阶段枚举 | 不需要 |
| 手动 free | Go runtime 自动回收 |
| GC 对象链表 | Go runtime 追踪 |

### 1.2 真正需要实现的是什么?

1. **JS 语义层面的引用计数** — 用于 JavaScript 对象之间的引用关系
2. **弱引用 (WeakRef)** — JavaScript WeakMap/WeakSet/WeakRef
3. **FinalizationRegistry** — JS 特有的清理机制
4. **循环检测** — 即使 Go GC 回收内存，也需要检测循环以便正确调用 finalizer

## 2. 混合 GC 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Go Runtime GC                            │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  管理所有 Go 堆内存: *Object, *String, *BigInt 等        │    │
│  │  - 自动追踪可达性                                        │    │
│  │  - 自动回收无引用对象                                     │    │
│  └─────────────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────────────┤
│                     JS Semantics Layer                          │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  JS 引用计数 (用于 Finalizer 和 WeakRef)                │    │
│  │  - FinalizerRegistry callbacks                          │    │
│  │  - WeakRef/WeakMap/WeakSet 清理                         │    │
│  │  - 循环引用检测 (用于 finalizer 触发)                    │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

## 3. Go GC 对象基类

### 3.1 基础结构

```go
package quickjs

import (
    "runtime"
    "sync"
    "sync/atomic"
)

// GCObject is embedded in all JavaScript heap objects
type GCObject struct {
    // JS 引用计数 (用于 finalizer 触发)
    // Go runtime GC 会自动管理内存，但这个计数用于 JS 语义
    jsRefCount int32
    
    // 对象类型，用于 finalize 判断
    gcType GCType
    
    // 链表链接 (用于 GC 扫描)
    next, prev *GCObject
    
    // 标记位
    marked bool
    
    // 锁 (用于并发安全)
    mu sync.Mutex
}

type GCType int

const (
    GCTypeObject GCType = iota
    GCTypeFunctionBytecode
    GCTypeShape
    GCTypeVarRef
    GCTypeAsyncFunction
    GCTypeModule
)
```

### 3.2 引用计数操作

```go
// AddRef 增加 JS 引用计数
func (o *GCObject) AddRef() {
    atomic.AddInt32(&o.jsRefCount, 1)
}

// Release 减少 JS 引用计数，返回是否应该触发清理检查
func (o *GCObject) Release(rt *Runtime) bool {
    refCount := atomic.AddInt32(&o.jsRefCount, -1)
    
    if refCount == 0 {
        // 触发 finalizer 检查
        rt.scheduleFinalization(o)
        return true
    }
    return false
}

// RefCount 返回当前引用计数 (调试用)
func (o *GCObject) RefCount() int32 {
    return atomic.LoadInt32(&o.jsRefCount)
}
```

## 4. JSValue 与引用计数

### 4.1 Go 语义下的引用计数

**关键理解**: 在 Go 中，值传递不会复制堆对象。只有指针传递会共享引用。

```go
// Go 的值传递是安全的
func example(ctx *Context) {
    obj := ctx.NewObject()
    // obj 是 Value 的副本，但底层的 *Object 指针是共享的
    
    // 不需要 JS_DupValue!
    // obj2 := obj  // 这是副本，包含相同的指针
    
    // 但我们需要追踪 JS 语义上的引用
    // 在 JS: let a = obj; let b = a;  // 两个引用
}
```

### 4.2 Value 的 AddRef/Release

```go
// Value 的引用操作
type Value interface {
    AddRef()
    Release()
    
    // ... other methods
}

// ObjectValue with reference counting
type ObjectValue struct {
    v *Object
}

func (o ObjectValue) AddRef() {
    if o.v != nil {
        o.v.AddRef()
    }
}

func (o ObjectValue) Release(ctx *Context) {
    if o.v != nil {
        o.v.Release(ctx.Runtime())
    }
}
```

### 4.3 Context 管理引用

```go
// Context 提供安全的值操作
type Context struct {
    rt *Runtime
}

func (ctx *Context) NewObject() Value {
    obj := ctx.rt.allocObject()
    obj.AddRef()  // 初始引用
    return ObjectValue{v: obj}
}

func (ctx *Context) FreeValue(v Value) {
    v.Release(ctx)
}

// DupValue 在 Go 中简化，因为 Go 已经是引用语义
// 但 JS 语义需要显式追踪
func (ctx *Context) DupValue(v Value) Value {
    v.AddRef()
    return v
}
```

## 5. FinalizationRegistry

### 5.1 设计

```go
// FinalizationRegistry 允许注册在对象被 GC 时调用的回调
type FinalizationRegistry struct {
    GCObject
    
    // 注册的清理回调
    // key: 被追踪对象的指针
    // value: cleanup callback
    registrations map[unsafe.Pointer]Value
    mu           sync.Mutex
}

func (rt *Runtime) NewFinalizationRegistry(cleanupCallback Value) *FinalizationRegistry {
    fr := &FinalizationRegistry{
        registrations: make(map[unsafe.Pointer]Value),
    }
    fr.AddRef()
    return fr
}

// Register 注册一个 finalization
func (fr *FinalizationRegistry) Register(target Value, heldValue Value) {
    // 获取 target 的底层指针
    ptr := fr.getTargetPointer(target)
    
    fr.mu.Lock()
    fr.registrations[ptr] = heldValue
    fr.mu.Unlock()
    
    // 注册 Go runtime finalizer
    runtime.SetFinalizer(target, func(obj interface{}) {
        fr.runCleanup(obj)
    })
}

// runCleanup 在对象被 GC 时调用
func (fr *FinalizationRegistry) runCleanup(obj interface{}) {
    fr.mu.Lock()
    heldValue, ok := fr.registrations[ptr]
    if ok {
        delete(fr.registrations, ptr)
        fr.mu.Unlock()
        
        // 调用清理回调
        callback := fr.cleanupCallback
        callback(ValueOf(heldValue))
    } else {
        fr.mu.Unlock()
    }
}
```

### 5.2 使用 runtime.SetFinalizer

```go
// 关键: 使用 Go 的 finalizer 来触发 JS finalizer

type Object struct {
    GCObject
    // ... fields
    finalizer func(*Object)
}

// Free 被调用时 (refCount 到达 0)
func (o *Object) Free(ctx *Context) {
    // 调用 JS finalizer
    if o.finalizer != nil {
        o.finalizer(o)
    }
    
    // 清理子引用
    o.freeChildren(ctx)
    
    // 放回对象池 (可选优化)
    ctx.rt.freeObjectPool.Put(o)
}
```

## 6. 弱引用 (WeakRef)

### 6.1 WeakRef 对象

```go
// WeakRef 允许持有对象的引用而不阻止 GC
type WeakRef struct {
    GCObject
    target unsafe.Pointer  // 原始对象指针
}

func (rt *Runtime) NewWeakRef(target Value) Value {
    wr := &WeakRef{
        target: extractPointer(target),
    }
    wr.AddRef()
    return WeakRefValue{v: wr}
}

// deref 返回 target 或 null (如果已被 GC)
func (wr *WeakRef) Deref() Value {
    ptr := atomic.LoadPointer(&wr.target)
    if ptr == nil {
        return Null  // 对象已被 GC
    }
    
    obj := wr.resolve(ptr)
    if obj == nil || !obj.IsLive() {
        return Null
    }
    return ObjectValue{v: obj}
}
```

### 6.2 WeakMap / WeakSet

```go
// WeakMap 键被 GC 时自动删除条目
type WeakMap struct {
    GCObject
    entries map[*Object]*Value
    mu     sync.RWMutex
}

func (wm *WeakMap) Delete(key Value) {
    if obj, ok := key.(ObjectValue); ok {
        wm.mu.Lock()
        delete(wm.entries, obj.v)
        wm.mu.Unlock()
    }
}

// cleanup 在对象被 GC 时调用
func (wm *WeakMap) cleanup(ptr unsafe.Pointer) {
    wm.mu.Lock()
    delete(wm.entries, (*Object)(ptr))
    wm.mu.Unlock()
}
```

## 7. 循环引用检测

### 7.1 什么时候需要检测循环?

```
场景: 对象 A 引用 B，B 引用 A，但外部没有其他引用

C 中:
  - A.refCount = 1, B.refCount = 1
  - 外部引用消失后，两者都变为 0
  - 但由于循环，两者都无法被立即释放
  - 需要 GC 扫描来检测和释放

Go 中:
  - Go runtime GC 会回收 A 和 B 的内存
  - 但我们需要知道何时调用 JS finalizer
```

### 7.2 JS 引用图

```go
// Runtime 维护 JS 对象之间的引用关系
type Runtime struct {
    // JS 引用追踪 (用于 finalizer 触发)
    jsRefs map[*GCObject][]*GCObject  // object -> objects it references
    
    mu     sync.RWMutex
}

// 记录引用
func (rt *Runtime) addJSRef(from, to *GCObject) {
    if from == nil || to == nil {
        return
    }
    rt.mu.Lock()
    rt.jsRefs[from] = append(rt.jsRefs[from], to)
    rt.mu.Unlock()
}

// 移除引用
func (rt *Runtime) removeJSRef(from, to *GCObject) {
    if from == nil || to == nil {
        return
    }
    rt.mu.Lock()
    refs := rt.jsRefs[from]
    for i, r := range refs {
        if r == to {
            // 删除 refs[i]
            break
        }
    }
    rt.mu.Unlock()
}
```

### 7.3 检测不可达循环

```go
// 在 Go runtime GC 完成后，检查是否有 JS 对象成为垃圾
func (rt *Runtime) checkJSCycles() {
    rt.mu.Lock()
    
    // 找到所有 refCount == 0 但仍在 jsRefs 图中的对象
    var cycles []*GCObject
    for obj := range rt.jsRefs {
        if obj.RefCount() == 0 && !rt.isExternallyReachable(obj) {
            cycles = append(cycles, obj)
        }
    }
    
    // 对每个循环对象调用 finalizer 并清理引用
    for _, obj := range cycles {
        rt.runFinalizers(obj)
        rt.cleanupJSEdges(obj)
        delete(rt.jsRefs, obj)
    }
    
    rt.mu.Unlock()
}
```

## 8. GC 触发机制

### 8.1 内存阈值

```go
type Runtime struct {
    gcThreshold      int64  // 触发 GC 的阈值
    lastGCCheck     int64  // 上次检查时的内存使用
}

const defaultGCThreshold = 256 * 1024  // 256KB

// alloc 分配内存时检查
func (rt *Runtime) alloc(size int) unsafe.Pointer {
    ptr := rt.malloc(size)
    
    // 检查是否需要 GC
    newSize := rt.memoryUsage()
    if newSize > rt.gcThreshold {
        rt.triggerGC()
        rt.gcThreshold = newSize + defaultGCThreshold
    }
    
    return ptr
}

func (rt *Runtime) triggerGC() {
    // 建议 Go runtime 进行 GC
    runtime.GC()
    
    // 然后运行 JS 层的清理
    rt.checkJSCycles()
}
```

### 8.2 手动触发

```go
// JavaScript: JS_RunGC()
func (rt *Runtime) RunGC() {
    runtime.GC()
    rt.checkJSCycles()
}
```

## 9. 内存管理 vs C 原文对照

| C 函数 | Go 实现 | 说明 |
|--------|---------|------|
| `JS_NewRuntime()` | `NewRuntime()` | 创建运行时 |
| `JS_FreeRuntime()` | `runtime GC` | Go 自动回收 |
| `JS_NewContext()` | `rt.NewContext()` | 创建上下文 |
| `JS_FreeContext()` | `runtime GC` | Go 自动回收 |
| `JS_DupValue()` | `v.AddRef()` | 增加引用 |
| `JS_FreeValue()` | `v.Release()` | 减少引用 |
| `JS_MarkValue()` | 不需要 | Go GC 自动追踪 |
| `JS_RunGC()` | `runtime.GC()` + `checkJSCycles()` | 触发清理 |
| `gc_decref()` | `runtime GC` | 自动 |
| `gc_scan()` | `runtime GC` | 自动 |
| `free_gc_object()` | `runtime GC` | 自动 |

## 10. 简化策略

### 10.1 最小实现

如果你只需要基本的 JS 引擎功能:

```go
type Runtime struct {
    // 所有 GC 追踪由 Go runtime 完成
}

// 只需实现 JS 语义层面的引用计数用于 finalizer
type Object struct {
    GCObject
    // ...
}

// 弱引用使用 Go 的 WeakRef (Go 1.21+)
// 或者使用 sync.Map + manual cleanup
```

### 10.2 完全实现

如果需要完整的 WeakRef/FinalizationRegistry:

```go
// 完整的实现需要:
// 1. JS 引用图追踪
// 2. 循环检测算法
// 3. FinalizationRegistry 回调
// 4. WeakRef/WeakMap/WeakSet 清理
```

## 11. 测试策略

### 11.1 Finalizer 测试

```go
func TestFinalizationRegistry(t *testing.T) {
    rt := NewRuntime()
    defer rt.Free()
    ctx := rt.NewContext()
    defer ctx.Free()
    
    var finalized int
    registry := rt.NewFinalizationRegistry(ctx.Undefined())
    
    obj := ctx.NewObject()
    registry.Register(obj, Null)
    
    // 删除唯一引用
    obj = nil
    
    // 强制 GC
    runtime.GC()
    
    // 等待 finalizer 执行
    time.Sleep(100 * time.Millisecond)
    
    require.Equal(t, 1, finalized)
}
```

### 11.2 弱引用测试

```go
func TestWeakRef(t *testing.T) {
    rt := NewRuntime()
    ctx := rt.NewContext()
    
    obj := ctx.NewObject()
    weak := ctx.NewWeakRef(obj)
    
    // 解除强引用
    obj = nil
    
    // 强制 GC
    runtime.GC()
    
    // WeakRef.deref() 应该返回 null
    require.True(t, weak.Deref().IsNull())
}
```

## 12. 常见陷阱

### 12.1 循环引用导致内存泄漏

```go
// 问题: A 引用 B，B 引用 A，且没有外部引用
// Go GC 会回收内存，但需要正确处理 finalizer

type A struct {
    b *B
}
type B struct {
    a *A
}

// 在 Go 中这不会泄漏，因为 Go GC 追踪所有引用
// 但 finalizer 可能不会被调用，如果对象被直接回收
```

### 12.2 SetFinalizer 陷阱

```go
// SetFinalizer 只在对象被 GC 回收时调用
// 如果对象被重新引用，finalizer 可能不会调用

obj := &Object{}
runtime.SetFinalizer(obj, func(o *Object) {
    // 这个可能会调用
})
obj = nil  // 删除引用
runtime.GC()

// 但如果:
obj = newObject
runtime.SetFinalizer(obj, ...)
obj = anotherObject  // 旧对象重新可达，finalizer 不会调用
```

### 12.3 跨 goroutine 问题

```go
// Finalizer 可能在任何 goroutine 运行
// 必须使用锁或其他同步机制

type Object struct {
    GCObject
    mu    sync.Mutex
    data  []byte
}

func (o *Object) Finalize() {
    o.mu.Lock()
    defer o.mu.Unlock()
    // 安全访问 o.data
}
```

## 13. 代码结构建议

```
gc/
├── gc_object.go      # GCObject 基类
├── refcount.go       # 引用计数操作
├── finalizer.go      # FinalizationRegistry
├── weakref.go        # WeakRef, WeakMap, WeakSet
├── cycle.go          # 循环检测
├── memory.go         # 内存阈值管理
└── gc_test.go        # 测试
```