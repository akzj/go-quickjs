# 复杂 C→Go 重写项目研发方法论

> 基于 go-lua（Go 重写 Lua 5.5.1 VM）项目的实战经验总结。
> 该项目从零开始，达到 C Lua 官方测试套件 26/27 通过（96%+），总计 ~15,000 行 Go 代码。
> 本文档面向 AI 工程师（zerofas 及同行），指导如何研发复杂度高的 C→Go 重写项目。

---

## 一、核心原则：为什么重写项目会进入"无底洞"

重写项目失败的根本原因不是 bug 多，而是 **没有可观测的进度指标**。

```
失败模式：
  写代码 → 跑不通 → 修 bug → 引入新 bug → 修新 bug → 不知道离目标多远 → 放弃

成功模式：
  定义可观测指标 → 写最小代码 → 跑测试 → 看到数字变化 → 有方向感 → 持续推进
```

**核心原则：一切以官方测试套件为锚点。不是"实现功能"，而是"通过测试"。**

---

## 二、方法论框架：五阶段推进法

### 阶段 0：项目评估（做不做？）

在写任何代码之前，回答这些问题：

| 问题 | go-lua 的答案 | go-quickjs 的现实 |
|------|-------------|-----------------|
| C 源码总行数？ | ~15,000 行（核心） | **88,000 行**（quickjs.c 一个文件 60,000 行）|
| 官方测试套件？ | 34 个 .lua 测试文件 | 12 个 .js 测试文件 + test262（50,000+ 用例）|
| 模块耦合度？ | 低（lex/parse/vm/gc 分离） | **极高**（全在一个 .c 文件里）|
| 语言复杂度？ | Lua 语法简单（~50 关键字） | **JS 语法极复杂**（ES2023，闭包/原型链/async/generator/proxy/...）|
| 预估工作量？ | 3-6 人月 | **12-24 人月**（保守估计）|

**关键决策点**：如果 C 源码 > 30,000 行，必须采用 **分层切片** 策略（见阶段 1）。
绝对不能试图"从头到尾翻译"。

### 阶段 1：分层切片 — 找到最小可测试子集

**这是最关键的一步。90% 的重写项目失败在这里。**

原则：**不是按模块分，而是按"能跑通的最小测试"分。**

#### go-lua 的成功路径：

```
第 1 刀：lexer + parser + compiler → 能编译 Lua 代码为字节码
  验证：对比 C Lua 的 luac 输出，字节码 100% 一致
  工具：bccompare（字节码对比工具）

第 2 刀：VM 核心指令（算术/比较/跳转/表操作）→ 能执行简单 Lua 脚本
  验证：math.lua, sort.lua, strings.lua 通过
  里程碑：第一个官方测试通过 ✅

第 3 刀：函数调用 + 闭包 + 元方法 → 能执行复杂 Lua 脚本
  验证：calls.lua, closure.lua, events.lua 通过

第 4 刀：协程 + 错误处理 → 能执行所有控制流
  验证：coroutine.lua, errors.lua 通过

第 5 刀：GC + 调试库 → 完整运行时
  验证：gc.lua, db.lua 通过
```

#### go-quickjs 应该怎么切：

```
第 1 刀：Lexer + Parser → 能解析 JS 代码为 AST
  验证：解析 test_language.js 中的每个语句不报错
  工具：写一个 AST dump 对比工具（对比 C QuickJS 的 dump 输出）

第 2 刀：Bytecode compiler → AST → 字节码
  验证：对比 qjsc -d 的字节码输出
  工具：bytecode 对比工具（你已经有 debug_bytecode.go）

第 3 刀：VM 核心（数值/字符串/基本对象）→ 能执行 1+1
  验证：test_language.js 前 100 行通过

第 4 刀：对象系统（原型链/属性描述符）→ 能执行 OOP
  验证：test_builtin.js 部分通过

第 5 刀：闭包 + 作用域 + this 绑定
  验证：test_closure.js 通过

第 6 刀：正则 + Unicode（你已经在做）
  验证：test_builtin.js 正则部分通过

第 7 刀：异常处理 + Generator + Async
  验证：test_language.js 完整通过
```

**关键**：每一刀都有明确的验证标准。不是"实现了 XX 功能"，而是"通过了 XX 测试"。

### 阶段 2：字节码对比驱动开发（BDD — Bytecode-Driven Development）

这是 go-lua 项目最成功的方法论创新。

```
传统方法：读 C 代码 → 理解语义 → 用 Go 重写 → 跑测试 → 不通过 → 猜哪里错了

BDD 方法：
  1. 用 C 版本编译一段代码，dump 字节码
  2. 用 Go 版本编译同一段代码，dump 字节码
  3. diff → 精确定位第一个不同的字节码
  4. 只修那一个字节码的生成逻辑
  5. 重复直到 100% 一致
```

#### 实战工具：

go-lua 项目建了两个对比工具：
- `tools/bccompare` — 字节码对比
- `tools/vmcompare` — 运行时行为对比（逐指令执行，对比寄存器状态）

**go-quickjs 必须建类似工具**：
```bash
# 用 C QuickJS 编译并 dump
qjsc -d test.js > expected.bytecode

# 用 Go QuickJS 编译并 dump
go-quickjs -dump test.js > actual.bytecode

# diff
diff expected.bytecode actual.bytecode
```

**没有这个工具，你就是在黑暗中摸索。有了它，每个 bug 都有精确坐标。**

### 阶段 3：测试套件渐进式启用

**不要试图一次性通过所有测试。**

go-lua 的策略：

```
阶段 A：直接运行，记录失败点（26 个文件，最初只有 3 个通过）
阶段 B：对每个失败文件，找到第一个失败的 assert，只修那一个
阶段 C：修完后重跑，找下一个失败的 assert
阶段 D：当一个文件全部通过，标记为 ✅，永远不能回退

进度看板：
  strings.lua  ✅ (第 1 周)
  math.lua     ✅ (第 1 周)
  sort.lua     ✅ (第 2 周)
  ...
  gc.lua       ✅ (第 8 周)
  coroutine.lua ⏳ (line 775/1100)
```

**关键指标**：
- ✅ 文件数 / 总文件数（go-lua: 26/27 = 96%）
- 每个 ⏳ 文件的进度行号（coroutine.lua: 775/1100 = 70%）

**go-quickjs 的测试启用顺序建议**：
```
1. test_language.js  — 语言核心（最重要）
2. test_builtin.js   — 内置对象
3. test_closure.js   — 闭包
4. test_bigint.js    — BigInt
5. test_loop.js      — 循环优化
6. test262           — 标准合规性（最后）
```

### 阶段 4：Patch 分类法 — 区分"不能修"和"不想修"

当测试失败时，不是所有失败都需要修。分三类：

| 类别 | 含义 | 处理 | go-lua 示例 |
|------|------|------|------------|
| **FIXABLE** | Go 能实现，只是还没做 | 排入计划 | coroutine.lua pcallk |
| **PLATFORM** | Go 平台限制，无法实现 | 永久 patch，注释说明 | T.totalmem（Go 无法控制内存上限）|
| **DEFERRED** | 能修但 ROI 低 | 记录，不修 | GC generational auto-switch |

**每个 patch 必须有注释说明属于哪一类。** 这样任何时候都能回答"还差多少"。

### 阶段 5：持续集成 — 绿灯永不回退

```
规则 1：每次 commit 前必须 go test ./... 全绿
规则 2：已通过的测试文件永远不能变成 FAIL
规则 3：每个 PR 的标题格式：feat: XXX (test_language.js 120/500 → 180/500)
```

---

## 三、QuickJS 特有的挑战与对策

### 挑战 1：quickjs.c 是 60,000 行的单文件

**对策**：不要试图理解整个文件。用 `grep` 定位功能边界。

```bash
# QuickJS 的核心函数入口
grep -n '^static.*JS_' quickjs.c | head -50   # 所有 static JS_ 函数
grep -n 'case OP_' quickjs.c                   # VM 指令分发
grep -n 'js_parse_' quickjs.c                  # parser 函数
grep -n 'JS_CallInternal' quickjs.c            # 函数调用核心
```

**建议**：先用工具提取函数列表和调用图，画出模块边界，再决定 Go 的 package 划分。

### 挑战 2：JS 语法比 Lua 复杂 10 倍

**对策**：不要自己写 parser。

选项 A：直接翻译 QuickJS 的 parser（C→Go 逐行翻译）
选项 B：用现成的 Go JS parser（如 esprima-go, goja 的 parser）
选项 C：先用 C QuickJS 编译为字节码，Go 只实现 VM

**推荐选项 C**（至少作为第一阶段）：
```
C QuickJS (parser+compiler) → bytecode → Go QuickJS VM (执行)
```
这样你可以先验证 VM 的正确性，再回头做 parser。
go-lua 项目也是先确保 VM 正确（用 C Lua 编译的字节码测试），再修 parser bug。

### 挑战 3：JS 对象模型远比 Lua 表复杂

Lua 只有 table（数组+哈希）。JS 有：
- 原型链（prototype chain）
- 属性描述符（configurable/enumerable/writable）
- Proxy/Reflect
- Symbol
- WeakMap/WeakSet
- 类（class）语法糖

**对策**：分层实现，每层有测试验证。

```
层 1：plain object + array（能跑 {a:1}.a === 1）
层 2：prototype chain（能跑 new Foo()）
层 3：property descriptor（能跑 Object.defineProperty）
层 4：Symbol + Iterator
层 5：Proxy/Reflect
层 6：Class 语法糖
```

### 挑战 4：test262 有 50,000+ 用例

**对策**：不要一开始就跑 test262。先用 QuickJS 自带的 12 个测试文件。
test262 是最终验证，不是开发驱动。

---

## 四、工具链建设（第一周必须完成）

在写任何业务代码之前，先建这些工具：

### 1. 字节码对比工具

```go
// cmd/bccompare/main.go
// 输入：一段 JS 代码
// 输出：C QuickJS 字节码 vs Go QuickJS 字节码 的 diff
```

### 2. 运行时对比工具

```go
// cmd/vmcompare/main.go  
// 输入：一段 JS 代码
// 输出：逐指令执行 trace（C vs Go），找到第一个分歧点
```

### 3. 测试进度看板

```go
// cmd/testdash/main.go
// 输入：运行所有 test_*.js
// 输出：
//   test_language.js   ✅ 450/500 assertions passed
//   test_builtin.js    ⏳ 120/300 assertions passed  
//   test_closure.js    ❌ 0/80 assertions passed
```

### 4. C QuickJS 参考运行器

```bash
# 编译 C QuickJS，用于参考对比
cd quickjs-master && make
# 运行测试
./qjs tests/test_language.js
# dump 字节码
./qjsc -d test.js
```

---

## 五、每日工作流

```
早上：
  1. git pull && go test ./...  （确认绿灯）
  2. 查看测试看板，找到当前 ⏳ 文件的下一个失败 assert
  3. 在 C QuickJS 中运行同一段代码，观察正确行为

工作中：
  4. 用字节码对比工具定位差异
  5. 修复差异（通常是一个函数/一个 opcode）
  6. go test ./... 确认不回退
  7. commit（标题包含进度数字）

下班前：
  8. 更新测试看板
  9. 如果某个 bug 修了 2 小时还没解决 → 记录为 TODO，跳过，继续下一个
```

**规则**：单个 bug 最多投入 2 小时。超过就跳过。
go-lua 的经验：80% 的 bug 在 30 分钟内修复。剩下 20% 的 bug 占 80% 的时间。
先跳过难的，把简单的都修完，回头再看难的（往往因为上下文更多了，变简单了）。

---

## 六、go-lua 的关键教训

### 教训 1：不要翻译 C 的内存管理

C 的 malloc/free/realloc 在 Go 中没有意义。Go 有 GC。

```
错误：翻译 C 的引用计数 / 手动内存管理
正确：用 Go 的 GC，只实现 Lua/JS 层面的 GC 语义（finalizer、weak reference）
```

go-lua 的 GC 不管理真正的内存。它管理的是 Lua 对象链表（mark/sweep），
真正的内存回收由 Go runtime 完成。这是一个关键的架构决策。

### 教训 2：C 的 longjmp 在 Go 中用 panic/recover

C Lua 的错误处理用 `setjmp/longjmp`。Go 没有这个。

```
错误：试图用 goroutine 模拟 longjmp
正确：用 panic(LuaError{}) + recover 模拟，在 PCall 边界 recover
```

QuickJS 也大量使用 longjmp。同样的策略适用。

### 教训 3：先跑通，再优化

go-lua 最初的 GC 就是一个简单的 FullGC（stop-the-world）。
增量 GC、分代 GC 都是后来加的。但从第一天起，gc.lua 就能通过。

```
错误：一开始就实现增量 GC
正确：先实现最简单的 mark-sweep，通过测试，再迭代优化
```

### 教训 4：T 库（测试辅助库）是关键

C Lua 的测试套件依赖一个 `T` 库（C API 测试接口）。
go-lua 花了大量时间实现 T 库（testlib.go, ~1600 行），但这是值得的——
它让我们能直接运行官方测试，而不是自己写测试。

QuickJS 的测试不依赖 T 库（纯 JS），这是一个优势。

### 教训 5：Patch 数量是技术债务的精确度量

```
go-lua 的 patch 历史：
  第 1 周：15 个 patch（大量功能缺失）
  第 4 周：8 个 patch（核心功能完成）
  第 8 周：6 个 patch（4 个 PLATFORM 不可修，2 个 DEFERRED）

patch 数量的下降曲线 = 项目健康度
如果 patch 数量不下降或上升 → 架构有问题，需要重新评估
```

---

## 七、go-quickjs 的重启建议

基于以上方法论，go-quickjs 应该这样重启：

### 第 1 周：工具链 + 最小 VM

```
1. 编译 C QuickJS（make），确认能运行所有测试
2. 建字节码对比工具
3. 用 C QuickJS 编译 "1+1" → 字节码 → Go VM 执行 → 输出 2
4. 里程碑：第一个表达式求值正确
```

### 第 2-3 周：VM 核心指令

```
1. 实现 30 个核心 opcode（算术/比较/跳转/加载常量/函数调用）
2. 验证：能执行 test_language.js 前 50 行
3. 每天 commit，标题带进度数字
```

### 第 4-6 周：对象系统

```
1. 实现 JSValue / JSObject / prototype chain
2. 验证：test_builtin.js 前 100 行
3. 这是最难的部分，预留充足时间
```

### 第 7-8 周：闭包 + 作用域

```
1. 实现 closure / scope chain / this binding
2. 验证：test_closure.js 通过
```

### 第 9-12 周：标准库 + 异常处理

```
1. 实现 try/catch/finally, Promise, Generator
2. 验证：test_language.js 完整通过
```

### 第 13+ 周：test262

```
1. 开始跑 test262 子集
2. 按章节逐步启用
```

---

## 八、给 AI 工程师的特别建议

如果你是 AI agent 在执行这个项目：

### 1. 永远不要"理解了再写"

```
错误：读完 60,000 行 quickjs.c 再开始写 Go
正确：找到一个 10 行的 C 函数，翻译成 Go，测试，提交，下一个
```

### 2. 用 C 版本作为 Oracle

任何时候不确定行为是否正确，直接在 C QuickJS 中运行同一段代码。
C 版本的输出就是 ground truth。不要猜。

### 3. 一个 commit 解决一个问题

```
好的 commit：fix: OP_ADD handles string concatenation (test_language.js 50→52)
坏的 commit：implement various fixes and improvements
```

### 4. 遇到 2 小时修不了的 bug → 跳过

记录在 TODO.md，继续下一个。回头再看。
go-lua 项目中，很多"难 bug"在实现了更多功能后自然消失了。

### 5. 测试是唯一的进度指标

```
不要说：我实现了原型链
要说：test_builtin.js 从 120/300 → 180/300
```

---

## 附录：go-lua 项目统计

| 指标 | 数值 |
|------|------|
| C Lua 源码 | ~15,000 行 |
| Go 实现 | ~15,000 行 |
| 官方测试通过 | 26/27 (96.3%) |
| Patches（不可修） | 4 个 |
| Patches（延迟） | 2 个 |
| 总 commits | ~80+ |
| 核心模块 | lexer, parser, compiler, VM, GC, stdlib (10个), debug, API |
| 关键工具 | bccompare, vmcompare, testlib (T库) |

---

*方法论版本 1.0 — 基于 go-lua 实战经验，2025 年*
