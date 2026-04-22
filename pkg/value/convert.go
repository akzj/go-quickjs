package value

// Type conversion functions for JavaScript values

// ToFloat64 converts a JSValue to float64
func ToFloat64(v JSValue) float64 {
	switch val := v.(type) {
	case IntValue:
		return float64(val)
	case Float64Value:
		return val.v
	case BoolValue:
		if val {
			return 1
		}
		return 0
	case UndefinedValue, NullValue:
		return 0 // NaN for null, 0 for undefined in numeric context
	default:
		return 0
	}
}

// ToInt32 converts a JSValue to int32 (truncating towards zero)
func ToInt32(v JSValue) int32 {
	f := ToFloat64(v)
	return int32(f)
}

// ToInt64 converts a JSValue to int64 (truncating towards zero)
func ToInt64(v JSValue) int64 {
	f := ToFloat64(v)
	return int64(f)
}

// ToBool converts a JSValue to boolean (JavaScript semantics)
func ToBool(v JSValue) bool {
	switch val := v.(type) {
	case IntValue:
		return val != 0
	case Float64Value:
		return val.v != 0 && val.v == val.v // not zero and not NaN
	case BoolValue:
		return bool(val)
	case UndefinedValue:
		return false
	case NullValue:
		return false
	default:
		// Objects, strings, etc. are truthy
		return true
	}
}

// ToNumber converts a JSValue to a number (returns new value)
func ToNumber(v JSValue) JSValue {
	return NewFloat(ToFloat64(v))
}

// Binary arithmetic operations

// Add performs JavaScript addition (also string concatenation)
func Add(a, b JSValue) JSValue {
	// Check if either is string
	// For now, just do numeric addition
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return NewFloat(af + bf)
}

// Sub performs JavaScript subtraction
func Sub(a, b JSValue) JSValue {
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return NewFloat(af - bf)
}

// Mul performs JavaScript multiplication
func Mul(a, b JSValue) JSValue {
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return NewFloat(af * bf)
}

// Div performs JavaScript division
func Div(a, b JSValue) JSValue {
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return NewFloat(af / bf)
}

// Mod performs JavaScript modulo
func Mod(a, b JSValue) JSValue {
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return NewFloat(float64(int64(af) % int64(bf)))
}

// Comparison operations

// Lt performs JavaScript less-than comparison
func Lt(a, b JSValue) bool {
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return af < bf
}

// Lte performs JavaScript less-than-or-equal comparison
func Lte(a, b JSValue) bool {
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return af <= bf
}

// Gt performs JavaScript greater-than comparison
func Gt(a, b JSValue) bool {
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return af > bf
}

// Gte performs JavaScript greater-than-or-equal comparison
func Gte(a, b JSValue) bool {
	af := ToFloat64(a)
	bf := ToFloat64(b)
	return af >= bf
}

// StrictEq performs strict equality comparison (===)
func StrictEq(a, b JSValue) bool {
	if a.Tag() != b.Tag() {
		return false
	}
	switch val := a.(type) {
	case IntValue:
		return val == b.(IntValue)
	case Float64Value:
		return val.v == b.(Float64Value).v
	case BoolValue:
		return val == b.(BoolValue)
	}
	return false // Different types or objects default to false
}

// Eq performs loose equality comparison (==)
func Eq(a, b JSValue) bool {
	// Simplified: convert to numbers and compare
	return ToFloat64(a) == ToFloat64(b)
}