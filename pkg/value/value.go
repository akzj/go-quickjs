package value

import "fmt"

// JSValue is the interface for all JavaScript values
// This is a tagged interface pattern - each concrete type represents a JS value
type JSValue interface {
	Tag() Tag
	String() string
}

// --- Inline Value Types ---

// IntValue represents a JavaScript integer (JS_TAG_INT)
type IntValue int64

func (v IntValue) Tag() Tag { return JS_TAG_INT }
func (v IntValue) String() string { return formatNumber(float64(v)) }

// BoolValue represents a JavaScript boolean (JS_TAG_BOOL)
type BoolValue bool

func (v BoolValue) Tag() Tag { return JS_TAG_BOOL }
func (v BoolValue) String() string {
	if v {
		return "true"
	}
	return "false"
}

// UndefinedValue represents JavaScript undefined (JS_TAG_UNDEFINED)
type UndefinedValue struct{}

var undefinedInstance = UndefinedValue{}

func Undefined() JSValue { return undefinedInstance }
func (v UndefinedValue) Tag() Tag { return JS_TAG_UNDEFINED }
func (v UndefinedValue) String() string { return "undefined" }

// NullValue represents JavaScript null (JS_TAG_NULL)
type NullValue struct{}

var nullInstance = NullValue{}

func Null() JSValue { return nullInstance }
func (v NullValue) Tag() Tag { return JS_TAG_NULL }
func (v NullValue) String() string { return "null" }

// --- Constructor Functions ---

func NewInt(v int64) JSValue { return IntValue(v) }

// NewFloat converts a float64 to an appropriate JSValue
// Integers in safe range are represented as IntValue
func NewFloat(v float64) JSValue {
	// Check if it's a safe integer
	if v == float64(int64(v)) && v >= -9007199254740992 && v <= 9007199254740992 {
		return IntValue(int64(v))
	}
	return Float64Value{v}
}

// Float64Value represents a heap-allocated 64-bit float (JS_TAG_FLOAT64)
type Float64Value struct{ v float64 }

func NewFloat64(v float64) JSValue { return Float64Value{v} }
func (v Float64Value) Tag() Tag { return JS_TAG_FLOAT64 }
func (v Float64Value) String() string { return formatNumber(v.v) }
func (v Float64Value) Float() float64 { return v.v }

// Predicate helpers
func True() JSValue  { return BoolValue(true) }
func False() JSValue { return BoolValue(false) }

// --- Helper Functions ---

func formatNumber(f float64) string {
	if f != f { // NaN check
		return "NaN"
	}
	if f == 1e300 || f == -1e300 { // Infinity
		if f > 0 {
			return "Infinity"
		}
		return "-Infinity"
	}
	if f == float64(int64(f)) {
		return formatInt(int64(f))
	}
	return fmt.Sprintf("%g", f)
}

func formatInt(i int64) string {
	if i < 0 {
		return "-" + formatUint(uint64(-i))
	}
	return formatUint(uint64(i))
}

func formatUint(u uint64) string {
	if u == 0 {
		return "0"
	}
	digits := "0123456789"
	var buf [32]byte
	i := len(buf)
	for u > 0 {
		i--
		buf[i] = digits[u%10]
		u /= 10
	}
	return string(buf[i:])
}