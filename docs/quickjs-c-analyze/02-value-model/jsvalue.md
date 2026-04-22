# QuickJS 值模型分析

## 1. JSValue 核心设计

### 1.1 NaN Boxing 实现 (默认配置)

QuickJS 使用 64 位整数存储所有 JavaScript 值，支持两种模式：

**NaN Boxing 模式** (默认, `JS_NAN_BOXING` 定义):
```
┌──────────────────────────────────────────────────────────────────┐
│                         64 bits                                  │
├──────────────────────────────────────────────────────────────────┤
│                          JSValue (uint64_t)                      │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │ high 32 bits          │  low 32 bits                        │ │
│  │ ┌──────────────────┐  │  ┌────────────────────────────────┐ │ │
│  │ │ tag (32 bits)    │  │  │ payload (32 bits)              │ │ │
│  │ │ -9 ~ 8           │  │  │ int value / pointer bits       │ │ │
│  │ └──────────────────┘  │  └────────────────────────────────┘ │ │
│  └─────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────┘
```

**关键宏定义** (`quickjs.h:146-159`):
```c
typedef uint64_t JSValue;

#define JS_VALUE_GET_TAG(v) (int)((v) >> 32)
#define JS_VALUE_GET_INT(v) (int)(v)
#define JS_MKVAL(tag, val) (((uint64_t)(tag) << 32) | (uint32_t)(val))
#define JS_MKPTR(tag, ptr) (((uint64_t)(tag) << 32) | (uintptr_t)(ptr))
```

### 1.2 非 NaN Boxing 模式 (备选)

当 `JS_NAN_BOXING` 未定义时，使用结构体+tag 字段:

```c
typedef union JSValueUnion {
    int32_t int32;
    double float64;
    void *ptr;
    int64_t short_big_int;
} JSValueUnion;

typedef struct JSValue {
    JSValueUnion u;
    int64_t tag;
} JSValue;
```

## 2. Tag 系统详解

### 2.1 Tag 枚举值 (`quickjs.h:77-93`)

```c
typedef enum {
    JS_TAG_FIRST       = -9,  /* first negative tag */
    JS_TAG_BIG_INT     = -9,  /* 大整数 (heap allocated) */
    JS_TAG_SYMBOL      = -8,  /* Symbol */
    JS_TAG_STRING      = -7,  /* 普通字符串 */
    JS_TAG_STRING_ROPE = -6,  /* Rope 字符串 (连接优化) */
    JS_TAG_MODULE      = -3,  /* 内部使用: 模块 */
    JS_TAG_FUNCTION_BYTECODE = -2, /* 内部使用: 函数字节码 */
    JS_TAG_OBJECT      = -1,  /* 对象 (heap pointer) */
    
    JS_TAG_INT         = 0,   /* 32位整数 (内联) */
    JS_TAG_BOOL        = 1,  /* 布尔值 */
    JS_TAG_NULL        = 2,  /* null */
    JS_TAG_UNDEFINED   = 3,  /* undefined */
    JS_TAG_UNINITIALIZED = 4, /* 未初始化 */
    JS_TAG_CATCH_OFFSET = 5, /* catch 偏移量 */
    JS_TAG_EXCEPTION   = 6,  /* 异常标记 */
    JS_TAG_SHORT_BIG_INT = 7, /* 短大整数 (内联, 32-63 bits) */
    JS_TAG_FLOAT64     = 8,  /* 64位浮点 (heap allocated) */
    
    /* any larger tag is FLOAT64 if JS_NAN_BOXING */
} JSTag;
```

### 2.2 Tag 分类

| Tag 范围 | 类型 | 存储方式 | 示例 |
|---------|------|---------|------|
| `JS_TAG_FIRST` ~ `-1` | Heap 对象引用 | 指针 (intptr_t) | 对象、字符串、Symbol、BigInt |
| `0` | 内联整数 | 值复制 | `JS_TAG_INT`, `JS_TAG_BOOL` |
| `>= 0 && < JS_TAG_FLOAT64` | 特殊内联值 | 值复制 | null, undefined, exception |
| `JS_TAG_FLOAT64` | 堆分配浮点 | 指针 | NaN boxing 编码的 double |

### 2.3 特殊值定义 (`quickjs.h:287-290`)

```c
#define JS_NULL           JS_MKVAL(JS_TAG_NULL, 0)
#define JS_UNDEFINED      JS_MKVAL(JS_TAG_UNDEFINED, 0)
#define JS_FALSE          JS_MKVAL(JS_TAG_BOOL, 0)
#define JS_TRUE           JS_MKVAL(JS_TAG_BOOL, 1)
#define JS_EXCEPTION      JS_MKVAL(JS_TAG_EXCEPTION, 0)
#define JS_UNINITIALIZED  JS_MKVAL(JS_TAG_UNINITIALIZED, 0)
```

## 3. Float64 处理 (NaN Boxing 核心)

### 3.1 编码原理

NaN Boxing 利用 IEEE 754 双精度浮点:
- NaN 的范围: `0x7FF8000000000000` ~ `0x7FFFFFFFFFFFFFFF` 或 `0xFFF8000000000000` ~ `0xFFFFFFFFFFFFFFFF`
- QuickJS 将 tag 编码到 NaN 的高位

```c
// quickjs.h:152-165
#define JS_FLOAT64_TAG_ADDEND (0x7ff80000 - JS_TAG_FIRST + 1)

static inline double JS_VALUE_GET_FLOAT64(JSValue v)
{
    union { JSValue v; double d; } u;
    u.v = v;
    u.v += (uint64_t)JS_FLOAT64_TAG_ADDEND << 32;
    return u.d;
}

#define JS_NAN (0x7ff8000000000000 - ((uint64_t)JS_FLOAT64_TAG_ADDEND << 32))
```

### 3.2 JS_TAG_IS_FLOAT64 判断

```c
// quickjs.h:125
#define JS_TAG_IS_FLOAT64(tag) ((unsigned)(tag) == JS_TAG_FLOAT64)

// quickjs.h:190 (非 NaN boxing 模式)
#define JS_TAG_IS_FLOAT64(tag) ((unsigned)((tag) - JS_TAG_FIRST) >= (JS_TAG_FLOAT64 - JS_TAG_FIRST))
```

## 4. 值创建函数

### 4.1 内联类型 (无堆分配)

```c
// quickjs.h:552-556
static js_force_inline JSValue JS_NewBool(JSContext *ctx, JS_BOOL val)
{
    return JS_MKVAL(JS_TAG_BOOL, (val != 0));
}

// quickjs.h:557-559
static js_force_inline JSValue JS_NewInt32(JSContext *ctx, int32_t val)
{
    return JS_MKVAL(JS_TAG_INT, val);
}
```

### 4.2 浮点数 (heap allocated)

```c
// quickjs.h:592-606
static js_force_inline JSValue JS_NewFloat64(JSContext *ctx, double d)
{
    int32_t val;
    if (d >= INT32_MIN && d <= INT32_MAX) {
        val = (int32_t)d;
        if ((double)val == d)  // 精确转换测试
            return JS_MKVAL(JS_TAG_INT, val);  // 优化: 小整数用 int
    }
    return __JS_NewFloat64(ctx, d);  // 堆分配
}
```

**优化**: `JS_NewFloat64` 会先测试是否能用 `JS_TAG_INT` 表示，避免不必要的堆分配。

### 4.3 短大整数 (内联, 32-63 bits)

```c
// quickjs.h:139-141
static inline JSValue __JS_NewShortBigInt(JSContext *ctx, int32_t d)
{
    return JS_MKVAL(JS_TAG_SHORT_BIG_INT, d);
}
```

## 5. 类型判断函数

```c
// quickjs.h:611-661
static inline BOOL JS_IsNumber(JSValueConst v) {
    int tag = JS_VALUE_GET_TAG(v);
    return tag == JS_TAG_INT || JS_TAG_IS_FLOAT64(tag);
}

static inline BOOL JS_IsBigInt(JSContext *ctx, JSValueConst v) {
    int tag = JS_VALUE_GET_TAG(v);
    return tag == JS_TAG_BIG_INT || tag == JS_TAG_SHORT_BIG_INT;
}

static inline BOOL JS_IsString(JSValueConst v) {
    int tag = JS_VALUE_GET_TAG(v);
    return tag == JS_TAG_STRING || tag == JS_TAG_STRING_ROPE;
}

static inline BOOL JS_IsObject(JSValueConst v) {
    return JS_VALUE_GET_TAG(v) == JS_TAG_OBJECT;
}
```

## 6. GC 相关宏

### 6.1 引用计数判断

```c
// quickjs.h:284
#define JS_VALUE_HAS_REF_COUNT(v) ((unsigned)JS_VALUE_GET_TAG(v) >= (unsigned)JS_TAG_FIRST)
```

**含义**: 只有负数 tag (heap 对象引用) 才有引用计数。

## 7. 关键设计决策

### 7.1 为什么用负数 tag 表示堆对象?

- 立即数 (int, bool, null, undefined) 用小整数 tag
- 堆对象用负数 tag，与立即数区分
- `JS_VALUE_HAS_REF_COUNT` 利用符号比较实现快速判断

### 7.2 内存效率

- 小整数 (int32): 无堆分配, 8 字节
- 普通浮点数: 8 字节 (float) + 16 字节 (header) = 24 字节
- 字符串: 按实际长度 + header

### 7.3 String Rope 优化

`JS_TAG_STRING_ROPE` 用于延迟字符串连接:
```c
typedef struct JSStringRope {
    JSRefCountHeader header;
    JSValue left;
    JSValue right;
} JSStringRope;
```
直到需要时才执行实际的字符串连接。

## 8. 代码位置索引

| 功能 | 文件位置 |
|------|---------|
| Tag 定义 | `quickjs.h:77-93` |
| JSValue 类型 (NaN boxing) | `quickjs.h:146-158` |
| JSValue 类型 (非 NaN boxing) | `quickjs.h:217-231` |
| Float64 编码 | `quickjs.h:152-182` |
| 特殊值定义 | `quickjs.h:287-290` |
| JS_NewBool/Int32 | `quickjs.h:552-559` |
| JS_NewFloat64 | `quickjs.h:592-606` |
| JS_IsNumber/IsString 等 | `quickjs.h:611-661` |
| JS_VALUE_HAS_REF_COUNT | `quickjs.h:284` |
| JSRefCountHeader | `quickjs.h:98-100` |