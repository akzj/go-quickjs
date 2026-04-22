# Go 重写陷阱汇总

## 概述

本节汇总 QuickJS Go 重写中的关键陷阱，按模块组织。

## 通用陷阱

### 1. 不要复制 C 的内存管理

**陷阱**: 直接翻译 `malloc/free`、`js_malloc/js_free`

**问题**: Go 有 GC，不需要手动管理内存

**正确做法**:
```go
// 错误
func js_malloc(ctx *Context, size int) unsafe.Pointer {
    return C.malloc(C.size_t(size))
}

// 正确
func (ctx *Context) alloc(size int) []byte {
    return make([]byte, size)
}
```

### 2. 不要使用 setjmp/longjmp

**陷阱**: 试图在 Go 中模拟 C 的非局部跳转

**问题**: Go 没有直接等价物

**正确做法**: 使用 panic/recover + defer，或状态机模式

```go
// 错误 - 不要尝试
// import "github.com/m不放setjmp"

defer func() {
    if r := recover(); r != nil {
        // 处理异常
    }
}()
```

### 3. 不要使用 C 的 static 变量

**陷阱**: 把 C 的 `static int counter` 变成 Go 包级变量

**问题**: 违反 Go 包设计原则

**正确做法**:
```go
// 错误
var counter int

// 正确
type Runtime struct {
    counter int
}

func (rt *Runtime) increment() int {
    rt.counter++
    return rt.counter
}
```

## 协程模块陷阱

### 4. Generator 帧状态完整性

**陷阱**: 只保存部分帧状态

**问题**: 恢复时上下文丢失

**必须保存**:
- PC (程序计数器)
- SP (栈指针)
- 局部变量
- VarRefs (闭包引用)

### 5. close_var_refs 遗漏

**陷阱**: Generator 完成时忘记关闭闭包变量

**问题**: 闭包中的变量仍然引用栈位置

**正确实现**:
```go
func closeVarRefs(rt *Runtime, frame *Frame) {
    for i, ref := range frame.varRefs {
        if ref != nil {
            // 解包变量引用
            ref.value = frame.locals[ref.index]
        }
    }
}
```

### 6. async_func_resume 的 tag 使用

**陷阱**: 不理解 `JS_MKPTR(JS_TAG_INT, s)` 用法

**问题**: 直接存储指针会丢失

**C 代码**:
```c
// 使用 JS_TAG_INT tag 存储指针
func_obj = JS_MKPTR(JS_TAG_INT, s);
ret = JS_CallInternal(ctx, func_obj, ..., JS_CALL_FLAG_GENERATOR);
```

**Go 实现**:
```go
// 用特殊标记值存储指针
const generatorTag = -1 // 或其他特殊值
ptr := unsafe.Pointer(s)
// 需要在 GC 中正确追踪
```

### 7. OP_initial_yield 遗漏

**陷阱**: 忽略 async generator 的初始 yield

**问题**: async generator 第一次 `.next()` 返回 `{value: undefined, done: false}`

**正确流程**:
```go
func (ag *AsyncGenerator) Next(v Value) Value {
    if ag.isFirstCall {
        ag.isFirstCall = false
        return iteratorResult(undefined, false)
    }
    // 正常执行
}
```

### 8. AsyncGenerator 请求队列重入

**陷阱**: 连续调用 `.next()` 时的竞态

**正确实现**:
```go
func (ag *AsyncGenerator) EnqueueRequest(req *Request) {
    ag.queue = append(ag.queue, req)
    if ag.state != STATE_EXECUTING {
        ag.processQueue()
    }
}

func (ag *AsyncGenerator) processQueue() {
    for len(ag.queue) > 0 {
        req := ag.queue[0]
        ag.state = STATE_EXECUTING
        ag.executeRequest(req)
        if ag.state == STATE_EXECUTING {
            // 等待 await
            return
        }
    }
}
```

### 9. Promise 循环引用

**陷阱**: 处理自引用的 Promise

```javascript
let p = new Promise((resolve) => resolve(p));
```

**正确实现**:
```go
func (p *Promise) resolve(v Value) {
    if p.isResolving {
        // 检测到循环，终止
        return
    }
    p.isResolving = true
    // 继续解析
}
```

### 10. throw_flag 传播

**陷阱**: async function 中 throw 后没有正确传播

**C 代码**:
```c
if (magic == GEN_MAGIC_THROW && s->state == SUSPENDED_YIELD) {
    JS_Throw(ctx, ret);
    s->func_state->throw_flag = TRUE;
}
```

**Go 实现**:
```go
if magic == THROW && gen.state == SUSPENDED_YIELD {
    ctx.throw(gen.receivedValue)
    gen.frame.throwFlag = true
}
```

## 词法/语法分析陷阱

### 11. UTF-8 处理

**陷阱**: C QuickJS 使用自定义字符编码

**问题**: Go 默认 UTF-8，可能行为不同

**正确做法**:
```go
// 使用 rune 而非 byte
func lexChar(s []rune, i int) (rune, int) {
    // 正确处理 Unicode
}
```

### 12. 贪心 vs 非贪心匹配

**陷阱**: regex/词法分析的贪心行为不一致

**测试用例**:
```javascript
var a = /a+/;
var b = a.exec("aaaa");
```

必须确保与 C 版本匹配。

### 13. 自动分号插入 (ASI)

**陷阱**: ASI 规则复杂，易出错

**C 代码**: 检查换行符位置

**Go 实现**:
```go
func shouldInsertSemicolon(prev, curr Token) bool {
    if prev.Line != curr.Line {
        return true
    }
    // 检查特殊规则
    // ...
}
```

## 编译/VM 陷阱

### 14. 字节顺序

**陷阱**: 字节码格式与大端/小端相关

**正确做法**: 统一使用小端，或在文件头标记字节序

### 15. 整数溢出

**陷阱**: Go int 在 64 位平台是 64 位，C int 通常 32 位

**正确做法**:
```go
type Int32 int32
type Uint32 uint32

func addInt32(a, b Int32) Int32 {
    result := int64(a) + int64(b)
    return Int32(result) // 溢出时 wrap
}
```

### 16. 浮点 NaN 比较

**陷阱**: NaN != NaN 在 Go 中成立

```go
math.NaN() == math.NaN() // false
```

**正确做法**:
```go
func isNaN(f float64) bool {
    return f != f // 利用 NaN != NaN 的特性
}
```

### 17. 运算符优先级

**陷阱**: 编译时计算的表达式优先级

**C 代码**: 使用显式括号树

**Go 实现**:
```go
func evalBinOp(left, op string, right Value) Value {
    switch op {
    case "+":
        if isString(left) || isString(right) {
            return concatString(left, right)
        }
        return addNumber(left, right)
    // ...
    }
}
```

## 内置对象陷阱

### 18. Array.length 行为

```javascript
var a = [1, 2, 3];
a.length = 1; // 裁剪数组
a.length = 5; // 扩展但不创建元素
```

### 19. Object.create(null) 原型

**陷阱**: Object.create(null) 没有原型

```go
obj := &Object{
    properties: make(map[string]Value),
    prototype: nil, // 无原型
}
```

### 20. this 绑定规则

**陷阱**: 严格模式和非严格模式的 this 不同

```go
func callFunction(fn Value, this Value, args []Value) Value {
    if strictMode {
        // this 可以是 undefined
    } else {
        // this 默认 global
    }
}
```

## 调试陷阱

### 21. 调试输出格式不一致

**问题**: 两个实现输出格式不同导致 diff 失败

**解决**: 定义明确的输出格式规范

```go
type TokenOutput struct {
    Type  string
    Value string
    Line  int
    Col   int
}

func (t TokenOutput) String() string {
    return fmt.Sprintf("%s %q %d:%d", t.Type, t.Value, t.Line, t.Col)
}
```

### 22. 错误消息差异

**陷阱**: 同样的错误，C 和 Go 产生不同消息

**解决**: 参考 C 的错误消息格式

```go
func throwTypeError(ctx *Context, msg string) Value {
    // 参考 C: "TypeError: %s"
    return ctx.throwError("TypeError", msg)
}
```

## 测试陷阱

### 23. 测试覆盖遗漏

**陷阱**: 只测试常见路径

**解决**: 包含边界情况

```javascript
// 边界测试
void 0;                    // undefined
"".charAt(-1);            // 越界访问
Math.pow(2, 1024);        // 溢出
try {} catch (e) {}       // 空 try
```

### 24. 异步测试

**陷阱**: 异步代码测试困难

**解决**: 使用 Promise 控制顺序

```go
func TestAsync(t *testing.T) {
    ctx := NewContext()
    promise := ctx.EvalPromise(`
        async function test() {
            await Promise.resolve(1);
            return 2;
        }
        test();
    `)
    
    result := promise.Await()
    if result != 2 {
        t.Errorf("expected 2, got %v", result)
    }
}
```

## 性能陷阱

### 25. 过度内存分配

**陷阱**: 每个操作都分配新对象

**解决**: 使用对象池

```go
var valuePool = sync.Pool{
    New: func() interface{} {
        return &Value{}
    },
}

func (ctx *Context) allocValue() *Value {
    return valuePool.Get().(*Value)
}
```

### 26. 字符串拼接

**陷阱**: 大量字符串拼接

**解决**: 使用 strings.Builder 或 runes

```go
var b strings.Builder
for _, r := range chars {
    b.WriteRune(r)
}
result := b.String()
```