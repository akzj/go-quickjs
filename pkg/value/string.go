package value

// StringValue represents a JavaScript string
type StringValue struct {
	str string
}

// NewString creates a new StringValue
func NewString(s string) JSValue {
	return StringValue{str: s}
}

// Tag returns JS_TAG_STRING
func (v StringValue) Tag() Tag {
	return JS_TAG_STRING
}

// String returns the string value
func (v StringValue) String() string {
	return v.str
}
