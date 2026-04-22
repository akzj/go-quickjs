package value

// ArrayValue represents a JavaScript array
type ArrayValue struct {
	data []JSValue
}

// NewArray creates a new ArrayValue
func NewArray(size int) *ArrayValue {
	return &ArrayValue{
		data: make([]JSValue, size),
	}
}

// Tag returns JS_TAG_OBJECT (arrays are objects in JS)
func (v *ArrayValue) Tag() Tag {
	return JS_TAG_OBJECT
}

// String returns a string representation
func (v *ArrayValue) String() string {
	return "[object Array]"
}

// Get returns the element at index
func (v *ArrayValue) Get(idx uint32) JSValue {
	if int(idx) < len(v.data) {
		return v.data[idx]
	}
	return Undefined()
}

// Set sets the element at index
func (v *ArrayValue) Set(idx uint32, val JSValue) {
	if int(idx) < len(v.data) {
		v.data[idx] = val
	}
}

// Length returns the array length
func (v *ArrayValue) Length() int {
	return len(v.data)
}

// Push adds elements to the end
func (v *ArrayValue) Push(val JSValue) {
	v.data = append(v.data, val)
}

// Pop removes and returns the last element
func (v *ArrayValue) Pop() JSValue {
	if len(v.data) == 0 {
		return Undefined()
	}
	val := v.data[len(v.data)-1]
	v.data = v.data[:len(v.data)-1]
	return val
}
