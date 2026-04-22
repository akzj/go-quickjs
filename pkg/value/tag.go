package value

// Tag represents the type tag of a JSValue, matching QuickJS JS_TAG_* constants
type Tag int32

// Tag constants - negative tags are heap objects (pointers)
const (
	JS_TAG_FIRST           Tag = -9  // first negative tag
	JS_TAG_BIG_INT         Tag = -9  // BigInt (heap allocated)
	JS_TAG_SYMBOL          Tag = -8  // Symbol
	JS_TAG_STRING          Tag = -7  // String
	JS_TAG_STRING_ROPE     Tag = -6  // Rope string (concatenation optimization)
	JS_TAG_MODULE          Tag = -3  // Internal: module
	JS_TAG_FUNCTION_BYTECODE Tag = -2 // Internal: function bytecode
	JS_TAG_OBJECT          Tag = -1  // Object (heap pointer)

	// Non-negative tags are inline values
	JS_TAG_INT             Tag = 0   // 32-bit integer
	JS_TAG_BOOL            Tag = 1   // Boolean
	JS_TAG_NULL            Tag = 2   // null
	JS_TAG_UNDEFINED       Tag = 3   // undefined
	JS_TAG_UNINITIALIZED   Tag = 4   // uninitialized
	JS_TAG_CATCH_OFFSET    Tag = 5   // catch offset
	JS_TAG_EXCEPTION       Tag = 6   // exception marker
	JS_TAG_SHORT_BIG_INT   Tag = 7   // short BigInt (inline, 32-63 bits)
	JS_TAG_FLOAT64         Tag = 8   // 64-bit float (heap allocated)
)

// IsHeapTag returns true if tag represents a heap-allocated object
func (t Tag) IsHeapTag() bool {
	return t < 0
}

// IsInlineTag returns true if tag represents an inline value (no allocation)
func (t Tag) IsInlineTag() bool {
	return t >= 0
}