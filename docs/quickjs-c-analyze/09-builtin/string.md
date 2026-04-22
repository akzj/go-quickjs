# String 内置对象实现

## 类定义 (行 1536)

```c
#define JS_CLASS_STRING 3
```

## JSString 结构 (行 530-545)

```c
struct JSString {
    JSRefCountHeader header;  // 引用计数，必须为首
    uint32_t len : 31;       // 字符串长度
    uint8_t is_wide_char : 1; // 0=8位，1=16位
    uint32_t hash : 30;       // 哈希值
    uint8_t atom_type : 2;    // 原子类型
    uint32_t hash_next;      // 原子表链接
    union {
        uint8_t str8[0];      // 8位字符串
        uint16_t str16[0];    // 16位字符串 (UTF-16)
    } u;
};
```

## String 值表示

### 内部值 vs 原始对象

```c
// JS_TAG_STRING (行 104)
// 字符串值直接存储在 JSValue 中
static inline JSValue JS_MKVAL(int tag, uint32_t val) {
    return (val << 32) | tag;
}

// 原始字符串值
static inline uint32_t JS_VALUE_GET_STRING(JSValue v) {
    return v >> 32;
}

// 字符串对象 (JS_CLASS_STRING)
// 值存储在对象的 u.object_data 中
typedef struct {
    JSGCObjectHeader header;
    JSClassID class_id : 16;
    // ...
    union {
        struct {
            JSString *str;  // 字符串数据
        } string;
    } u;
} JSObject;
```

## String 构造函数 (行 44170-44250)

```c
static JSValue js_string_constructor(JSContext *ctx, JSValueConst new_target,
                                     int argc, JSValueConst *argv)
{
    JSValue ret;
    
    if (argc == 0) {
        // String() -> ""
        ret = JS_NewString(ctx, "");
    } else if (JS_IsNull(argv[0]) || !JS_IsSymbol(argv[0])) {
        // String(value) -> ToString(value)
        ret = JS_ToStringInternal(ctx, argv[0], 0);
    } else {
        // Symbol 需要特殊处理
        JS_ThrowTypeError(ctx, "cannot convert symbol to string");
        ret = JS_EXCEPTION;
    }
    
    return ret;
}
```

## 字符串属性访问 (行 44650-44800)

### 字符访问

```c
static JSValue js_string_charAt(JSContext *ctx, JSValueConst this_val,
                                 int argc, JSValueConst *argv)
{
    JSValue idx;
    int64_t idx64;
    
    this_val = JS_ToString(ctx, this_val);  // 自动转字符串
    idx = argv[0];
    
    if (JS_ToInt64Sat(ctx, &idx64, idx))
        return JS_EXCEPTION;
    
    JSString *p = JS_VALUE_GET_STRING(this_val);
    if (p->is_wide_char) {
        if (idx64 >= 0 && idx64 < p->len)
            return js_new_string16(ctx, p->u.str16 + idx64, 1);
    } else {
        if (idx64 >= 0 && idx64 < p->len)
            return js_new_string8(ctx, p->u.str8 + idx64, 1);
    }
    
    return JS_NewString(ctx, "");
}

static JSValue js_string_charCodeAt(JSContext *ctx, JSValueConst this_val,
                                     int argc, JSValueConst *argv)
{
    JSValue idx;
    int64_t idx64;
    double d;
    
    this_val = JS_ToString(ctx, this_val);
    idx = argv[0];
    
    if (JS_ToInt64Sat(ctx, &idx64, idx))
        return JS_EXCEPTION;
    
    JSString *p = JS_VALUE_GET_STRING(this_val);
    if (idx64 < 0 || idx64 >= p->len) {
        d = NAN;
    } else if (p->is_wide_char) {
        d = p->u.str16[idx64];
    } else {
        d = p->u.str8[idx64];
    }
    
    return __JS_NewFloat64(ctx, d);
}
```

### 字符串搜索

```c
static JSValue js_string_indexOf(JSContext *ctx, JSValueConst this_val,
                                  int argc, JSValueConst *argv, int magic)
{
    JSValueConst search_string, pos;
    int64_t pos_int, start, search_len, this_len;
    const char *search_buf;
    
    this_val = JS_ToString(ctx, this_val);
    if (JS_IsException(this_val))
        return JS_EXCEPTION;
    
    if (JS_IsException(search_string = JS_ToString(ctx, argv[0])))
        return JS_EXCEPTION;
    
    this_len = js_get_strlen(this_val);
    
    // 解析位置参数
    if (argc > 1 && !JS_IsUndefined(argv[1])) {
        if (JS_ToInt64Sat(ctx, &pos_int, argv[1]))
            return JS_EXCEPTION;
    } else {
        pos_int = 0;
    }
    
    // 边界处理
    if (pos_int < 0) pos_int = 0;
    if (pos_int > this_len) pos_int = this_len;
    
    // 搜索
    search_len = js_get_strlen(search_string);
    start = js_string_indexOf_vec(this_val, pos_int, search_string);
    
    if (magic == 0) // indexOf
        return JS_NewInt64(ctx, start);
    else             // lastIndexOf
        return JS_NewInt64(ctx, js_string_lastIndexOf_vec(this_val, pos_int, search_string));
}
```

## 字符串操作方法

### slice/substring/substr

```c
static JSValue js_string_slice(JSContext *ctx, JSValueConst this_val,
                               int argc, JSValueConst *argv)
{
    int64_t len = js_get_strlen(this_val);
    int64_t start, end, span;
    
    // 解析 start
    if (JS_ToInt64Sat(ctx, &start, argv[0]))
        return JS_EXCEPTION;
    if (start < 0) start = max_int64(len + start, 0);
    if (start > len) start = len;
    
    // 解析 end
    if (argc > 1 && !JS_IsUndefined(argv[1])) {
        if (JS_ToInt64Sat(ctx, &end, argv[1]))
            return JS_EXCEPTION;
        if (end < 0) end = max_int64(len + end, 0);
        if (end > len) end = len;
    } else {
        end = len;
    }
    
    span = end - start;
    if (span < 0) span = 0;
    
    return js_substring(ctx, this_val, start, start + span);
}
```

### split

```c
static JSValue js_string_split(JSContext *ctx, JSValueConst this_val,
                               int argc, JSValueConst *argv)
{
    JSValue splitter, limit_val;
    uint32_t limit = 0;
    JSValue *tab;
    uint32_t i, j, split_len;
    
    this_val = JS_ToString(ctx, this_val);
    
    // limit 处理
    if (argc > 1 && !JS_IsUndefined(argv[1])) {
        double d = JS_ToFloat64(ctx, argv[1]);
        limit = (uint32_t)min_uint64(d, 4294967295);
    }
    
    // 空字符串分隔符：每个字符分隔
    if (JS_IsUndefined(splitter) || JS_IsNull(splitter)) {
        // ...
    } else if (js_string_is_empty(splitter)) {
        // 空字符串：返回包含原始字符串的数组
        result = JS_NewArray(ctx);
        JS_SetPropertyUint32(ctx, result, 0, JS_DupValue(ctx, this_val));
    } else {
        // 正常分隔：找到所有匹配位置
        // 使用手动循环避免递归
    }
    
    return result;
}
```

## Go 实现建议

### 1. String 结构

```go
type String struct {
    length    int
    isUTF16   bool
    data      []uint16  // UTF-16 存储
}

func NewString(s string) *String {
    runes := []rune(s)
    return &String{
        length:  len(runes),
        isUTF16: true,
        data:    utf16.Encode(runes),
    }
}

func (s *String) Len() int {
    return s.length
}

func (s *String) CharAt(idx int) rune {
    if idx < 0 || idx >= s.length {
        return 0
    }
    return rune(s.data[idx])
}
```

### 2. String 值包装

```go
type StringObject struct {
    Object
    str *String
}

func (ctx *Context) ToString(val Value) Value {
    switch val.Tag {
    case TagString:
        return val  // 原始字符串直接返回
    case TagInt:
        return NewStringValue(strconv.FormatInt(val.Int64(), 10))
    case TagFloat:
        return NewStringValue(strconv.FormatFloat(val.Float64(), 'g', -1, 64))
    case TagObject:
        if obj, ok := val.(*Object); ok && obj.ClassID == ClassString {
            return val
        }
        // 调用 valueOf 或 toString
        return obj.Get(ctx, "valueOf").Call(ctx, obj, nil)
    }
    return NewStringValue("undefined")
}
```

### 3. 字符串方法

```go
func StringIndexOf(ctx *Context, this Value, args []Value) Value {
    s := ctx.ToString(this).String()
    search := ctx.ToString(args[0]).String()
    
    pos := 0
    if len(args) > 1 && !args[1].IsUndefined() {
        pos = int(args[1].ToInteger())
        if pos < 0 {
            pos = 0
        }
    }
    
    if pos > len(s) {
        pos = len(s)
    }
    
    idx := strings.Index(s[pos:], search)
    if idx < 0 {
        return NewIntValue(-1)
    }
    return NewIntValue(int64(pos + idx))
}

func StringSlice(ctx *Context, this Value, args []Value) Value {
    s := ctx.ToString(this).String()
    l := len(s)
    
    start := 0
    end := l
    
    if len(args) > 0 {
        start = int(args[0].ToInteger())
        if start < 0 {
            start = l + start
            if start < 0 {
                start = 0
            }
        }
    }
    
    if len(args) > 1 && !args[1].IsUndefined() {
        end = int(args[1].ToInteger())
        if end < 0 {
            end = l + end
            if end < 0 {
                end = 0
            }
        }
    }
    
    if start > l {
        start = l
    }
    if end > l {
        end = l
    }
    if start > end {
        start = end
    }
    
    return NewStringValue(s[start:end])
}

func StringSplit(ctx *Context, this Value, args []Value) Value {
    s := ctx.ToString(this).String()
    separator := ""
    if len(args) > 0 && !args[0].IsUndefined() {
        separator = ctx.ToString(args[0]).String()
    }
    
    limit := math.MaxInt
    if len(args) > 1 && !args[1].IsUndefined() {
        limit = int(args[1].ToUint32())
    }
    
    result := NewArray(ctx)
    if separator == "" {
        // 每个字符分隔
        runes := []rune(s)
        for i := 0; i < len(runes) && result.Len() < limit; i++ {
            result.Push(NewStringValue(string(runes[i])))
        }
    } else {
        parts := strings.SplitN(s, separator, limit)
        for _, p := range parts {
            result.Push(NewStringValue(p))
        }
    }
    
    return ValueOf(result)
}
```

## 陷阱规避

1. **UTF-16 编码**: JavaScript 字符串使用 UTF-16，Go 使用 UTF-8
2. **代理对 (Surrogate Pairs)**: UTF-16 的 2-byte 字符需要正确处理
3. **Symbol.toPrimitive**: 字符串转换时调用此方法
4. **null/undefined**: String(null) -> "null"，String(undefined) -> "undefined"
5. **String.raw**: 模板字符串的原始字符串
6. **normalize**: Unicode 正规化
7. **padStart/padEnd**: 需要正确处理多字节字符
8. **trimEnd/trimStart**: ES2019 引入，trimRight/trimLeft 已废弃
