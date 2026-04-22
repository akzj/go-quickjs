# QuickJS 类型转换分析

## 1. 转换函数总览

QuickJS 提供了完整的类型转换 API：

| 目标类型 | 函数 | 异常处理 |
|---------|------|---------|
| bool | `JS_ToBool()` / `JS_ToBoolFree()` | 返回 -1 表示异常 |
| int32 | `JS_ToInt32()` / `JS_ToInt32Free()` | 同上 |
| int64 | `JS_ToInt64()` / `JS_ToInt64Ext()` | 同上 |
| uint32 | `JS_ToUint32()` | 同上 |
| float64 | `JS_ToFloat64()` / `JS_ToFloat64Free()` | 同上 |
| bigint | `JS_ToBigInt64()` | 同上 |
| string | `JS_ToString()` / `JS_ToStringFree()` | 永不失败 |
| property key | `JS_ToPropertyKey()` | 同上 |

## 2. ToBool 转换

### 2.1 源码位置

`quickjs.c:10755-10820`

### 2.2 转换规则

```c
static int JS_ToBoolFree(JSContext *ctx, JSValue val)
{
    switch(JS_VALUE_GET_TAG(val)) {
    case JS_TAG_BOOL:
        return JS_VALUE_GET_BOOL(val);
    case JS_TAG_INT:
        return JS_VALUE_GET_INT(val) != 0;
    case JS_TAG_UNDEFINED:
    case JS_TAG_NULL:
        return FALSE;
    case JS_TAG_EXCEPTION:
        return -1;
    case JS_TAG_OBJECT:
        // ToBoolean: 所有对象都是 true
        // 但需要调用 valueOf/toString 检查自定义类型
        return JS_ToBoolFree(ctx, JS_ToPrimitive(val, JS_FALSE));
    // ... float, bigint, string, symbol ...
    }
}
```

### 2.3 JavaScript 语义

| 输入类型 | 结果 |
|---------|------|
| undefined | false |
| null | false |
| boolean | 原值 |
| number | `NaN` 和 `0` 为 false, 其他 true |
| string | 空字符串为 false, 其他 true |
| object | **永远 true** (即使 valueOf 返回 false) |

**注意**: 对象会先调用 `ToPrimitive` 转换。

## 3. ToNumber 转换

### 3.1 ToInt32

`quickjs.c` 中的关键实现：

```c
// quickjs.c:9540-9620 (大致位置)
static int JS_ToInt32Free(JSContext *ctx, int32_t *pres, JSValue val)
{
    // 1. 已经是 int: 直接返回
    if (JS_VALUE_GET_TAG(val) == JS_TAG_INT) {
        *pres = JS_VALUE_GET_INT(val);
        return 0;
    }
    
    // 2. 浮点数: trunc 并范围检查
    if (JS_TAG_IS_FLOAT64(tag)) {
        double d = JS_VALUE_GET_FLOAT64(val);
        if (d != d || d >= 2147483648.0 || d < -2147483648.0)
            return JS_ThrowRangeError(...);
        *pres = (int32_t)d;
        return 0;
    }
    
    // 3. BigInt: 直接转换 (可能有精度损失)
    if (JS_TAG_IS_BIG_INT(tag)) {
        // BigInt to int32
    }
    
    // 4. 其他类型: 先转 number 再转 int32
    val = JS_ToNumberFree(ctx, val);
    // ...
}
```

### 3.2 ToFloat64

```c
static int JS_ToFloat64Free(JSContext *ctx, double *pres, JSValue val)
{
    int tag = JS_VALUE_GET_TAG(val);
    
    switch(tag) {
    case JS_TAG_INT:
        *pres = JS_VALUE_GET_INT(val);
        return 0;
    case JS_TAG_FLOAT64:
        *pres = JS_VALUE_GET_FLOAT64(val);
        return 0;
    case JS_TAG_BIG_INT:
    case JS_TAG_SHORT_BIG_INT:
        // BigInt → float (可能有精度损失)
        return JS_ToBigInt64Free(ctx, &i) ? -1 : (*pres = i, 0);
    case JS_TAG_UNDEFINED:
        *pres = JS_NAN;
        return 0;
    case JS_TAG_NULL:
        *pres = 0;
        return 0;
    case JS_TAG_BOOL:
        *pres = JS_VALUE_GET_BOOL(val);
        return 0;
    case JS_TAG_STRING:
        // 解析字符串为数字
        return js_strtod(ctx, str, pres);
    case JS_TAG_OBJECT:
        // ToNumber(obj) = ToNumber(ToPrimitive(obj))
        val = JS_ToNumberFree(ctx, JS_ToPrimitive(val, JS_FALSE));
        // ...
    }
}
```

### 3.3 JavaScript ToNumber 语义

| 输入 | 结果 |
|------|------|
| undefined | NaN |
| null | +0 |
| boolean | 1 (true) 或 0 (false) |
| number | 原值 |
| string | 解析数字，空字符串 → 0 |
| object | ToNumber(ToPrimitive(obj)) |
| BigInt | 原值 (后续可能出错) |

## 4. ToString 转换

### 4.1 源码位置

`quickjs.c` 中的字符串转换函数。

### 4.2 转换规则

```c
JSValue JS_ToString(JSContext *ctx, JSValueConst val)
{
    switch(JS_VALUE_GET_TAG(val)) {
    case JS_TAG_STRING:
    case JS_TAG_STRING_ROPE:
        // 已经是字符串，增加引用计数返回
        return JS_DupValue(ctx, val);
        
    case JS_TAG_INT:
        // 整数 → 字符串
        return JS_NewString(ctx, int_to_str(JS_VALUE_GET_INT(val)));
        
    case JS_TAG_FLOAT64:
        // 浮点 → 字符串 (特殊处理 NaN, Infinity)
        return js_FloatToString(ctx, JS_VALUE_GET_FLOAT64(val));
        
    case JS_TAG_BIG_INT:
    case JS_TAG_SHORT_BIG_INT:
        return js_BigIntToString(ctx, val);
        
    case JS_TAG_BOOL:
        return JS_NewAtomString(ctx, 
            JS_VALUE_GET_BOOL(val) ? "true" : "false");
        
    case JS_TAG_NULL:
        return JS_NewAtomString(ctx, "null");
        
    case JS_TAG_UNDEFINED:
        return JS_NewAtomString(ctx, "undefined");
        
    case JS_TAG_EXCEPTION:
        return JS_EXCEPTION;
        
    case JS_TAG_OBJECT:
        // ToString(obj) = ToString(ToPrimitive(obj, hint string))
        // 注意: hint 是 "string"，不是 "number"
        return JS_ToStringFree(ctx, JS_ToPrimitive(val, JS_TRUE));
    }
}
```

### 4.3 JavaScript ToString 语义

| 输入 | 结果 |
|------|------|
| undefined | "undefined" |
| null | "null" |
| boolean | "true" / "false" |
| number | 数字字符串 (特殊值见下) |
| string | 原值 |
| object | ToString(ToPrimitive(obj, "string")) |
| BigInt | 数字字符串 + "n" |
| Symbol | **抛出 TypeError** |

**Number ToString 特殊情况**:
- `NaN` → "NaN"
- `+0` → "0"
- `-0` → "0"
- `Infinity` → "Infinity"
- `-Infinity` → "-Infinity"

## 5. ToPrimitive 转换

### 5.1 源码位置

`quickjs.c` 中的 `JS_ToPrimitive()` 函数。

### 5.2 算法

```c
// quickjs.c (大致)
JSValue JS_ToPrimitive(JSContext *ctx, JSValueConst obj, JS_BOOL hint_is_string)
{
    // hint: JS_FALSE = "number", JS_TRUE = "string"
    
    // 1. 检查 @@toPrimitive 方法
    JSValue exotic_to_prim = JS_GetMethodRaw(ctx, obj, JS_ATOM_Symbol_toPrimitive);
    if (!JS_IsUndefined(exotic_to_prim)) {
        // 调用 obj[Symbol.toPrimitive](hint)
        return JS_Call(ctx, exotic_to_prim, obj, 1, &hint_arg);
    }
    
    // 2. 普通对象
    if (hint_is_string) {
        // 先尝试 valueOf, 再尝试 toString
        method = JS_GetProperty(ctx, obj, JS_ATOM_valueOf);
        if (JS_IsException(method)) return JS_EXCEPTION;
        if (JS_IsFunction(ctx, method)) {
            result = JS_Call(ctx, method, obj, 0, NULL);
            if (JS_IsException(result)) return result;
            if (JS_IsObject(result)) {
                // 返回值仍是对象，抛出 TypeError
                return JS_ThrowTypeError(ctx, "cannot convert to primitive");
            }
            return result;  // 或继续 toString
        }
        // 再试 toString ...
    } else {
        // hint = "number": 先 valueOf 再 toString
    }
}
```

### 5.3 关键点

1. **Symbol.toPrimitive 优先**: 如果对象有这个方法，优先调用
2. **hint 参数**: 
   - `JS_FALSE` ("number"): Date 除外，都先 valueOf
   - `JS_TRUE` ("string"): 都先 toString
3. **Date 特殊处理**: Date 对象先 toString 再 valueOf
4. **类型检查**: 如果返回值是对象，抛出 TypeError

## 6. ToPropertyKey 转换

### 6.1 源码位置

`quickjs.c` 中的 `JS_ToPropertyKey()` 函数。

### 6.2 算法

```c
JSValue JS_ToPropertyKey(JSContext *ctx, JSValueConst val)
{
    // 等价于: ToPrimitive(val, "string")
    
    if (JS_IsString(ctx, val))
        return JS_DupValue(ctx, val);
    
    if (JS_IsSymbol(ctx, val))
        return JS_DupValue(ctx, val);
    
    // 其他类型: 先 ToPrimitive 再 ToString
    val = JS_ToPrimitive(ctx, val, TRUE);
    if (JS_IsException(val))
        return val;
    
    // Symbol 会抛出异常 (Symbol 不能转 string)
    return JS_ToString(ctx, val);
}
```

## 7. 对象到原始值的隐式转换

### 7.1 加法运算符 `+`

```c
// quickjs.c 操作符处理
case OP_add: {
    // 1. ToPrimitive(x) + ToPrimitive(y)
    v1 = JS_ToPrimitive(ctx, op1, FALSE);
    v2 = JS_ToPrimitive(ctx, op2, FALSE);
    
    if (JS_IsString(ctx, v1) || JS_IsString(ctx, v2)) {
        // 任一是字符串: ToString
        s1 = JS_ToString(ctx, v1);
        s2 = JS_ToString(ctx, v2);
        // 字符串连接
    } else {
        // 都是数字: ToNumber
        n1 = JS_ToNumberFree(ctx, v1);
        n2 = JS_ToNumberFree(ctx, v2);
        // 数值加法
    }
}
```

### 7.2 比较运算符

```c
// OP_lt, OP_le 等
case OP_lt: {
    // 1. 如果是字符串: 字典序比较
    // 2. 否则: ToNumber
    n1 = JS_ToNumberFree(ctx, op1);
    n2 = JS_ToNumberFree(ctx, op2);
    // 数值比较
}
```

## 8. 代码位置索引

| 功能 | 文件位置 |
|------|---------|
| JS_ToBool | `quickjs.c:10755-10820` |
| JS_ToInt32 | `quickjs.c:9590-9610` |
| JS_ToFloat64 | `quickjs.c:9625-9645` |
| JS_ToString | `quickjs.c` 字符串转换函数 |
| JS_ToPrimitive | `quickjs.c` |
| JS_ToPropertyKey | `quickjs.c` |
| 加法运算符 | `quickjs.c` `OP_add` 处理 |

## 9. 错误处理模式

所有转换函数返回 `int`:
- `0`: 成功
- `-1`: 异常 (`JS_IsException(result)`)

通常提供两个版本:
- `JS_ToType()`: 不释放输入值
- `JS_ToTypeFree()`: 释放输入值

```c
int JS_ToBool(JSContext *ctx, JSValueConst val);
int JS_ToBoolFree(JSContext *ctx, JSValue val);

int JS_ToInt32(JSContext *ctx, int32_t *pres, JSValueConst val);
int JS_ToInt32Free(JSContext *ctx, int32_t *pres, JSValue val);
```