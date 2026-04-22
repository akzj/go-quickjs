# 12 - Go 实现指导模块分析索引

## 模块概述

Go 实现指导模块汇总了 QuickJS 重写中的陷阱、注意事项和实现建议。

## 文件清单

| 文件 | 描述 |
|------|------|
| `traps.md` | Go 重写陷阱汇总（26 个陷阱） |
| `README.md` | 本模块索引 |

## 陷阱分类

### 通用陷阱 (1-3)
- 内存管理翻译
- setjmp/longjmp 等价
- static 变量处理

### 协程陷阱 (4-10)
- Generator 帧状态完整性
- close_var_refs 遗漏
- async_func_resume tag 使用
- OP_initial_yield 遗漏
- AsyncGenerator 请求队列重入
- Promise 循环引用
- throw_flag 传播

### 词法/语法陷阱 (11-13)
- UTF-8 处理
- 贪心/非贪心匹配
- ASI 规则

### 编译/VM 陷阱 (14-17)
- 字节顺序
- 整数溢出
- 浮点 NaN 比较
- 运算符优先级

### 内置对象陷阱 (18-20)
- Array.length 行为
- Object.create(null) 原型
- this 绑定规则

### 调试陷阱 (21-22)
- 调试输出格式
- 错误消息差异

### 测试陷阱 (23-24)
- 测试覆盖遗漏
- 异步测试

### 性能陷阱 (25-26)
- 过度内存分配
- 字符串拼接

## 参考

- quickjs.c 各相关实现
- quickjs-opcode.h opcode 定义
- METHODOLOGY.md 设计原则