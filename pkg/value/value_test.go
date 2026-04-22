package value

import "testing"

func TestIntValue(t *testing.T) {
	v := NewInt(42)
	if v.Tag() != JS_TAG_INT {
		t.Errorf("expected tag %v, got %v", JS_TAG_INT, v.Tag())
	}
}

func TestBoolValue(t *testing.T) {
	trueVal := True()
	if trueVal.Tag() != JS_TAG_BOOL {
		t.Errorf("True() tag = %v, want %v", trueVal.Tag(), JS_TAG_BOOL)
	}

	falseVal := False()
	if falseVal.Tag() != JS_TAG_BOOL {
		t.Errorf("False() tag = %v, want %v", falseVal.Tag(), JS_TAG_BOOL)
	}
}

func TestUndefined(t *testing.T) {
	v := Undefined()
	if v.Tag() != JS_TAG_UNDEFINED {
		t.Errorf("Undefined() tag = %v, want %v", v.Tag(), JS_TAG_UNDEFINED)
	}
}

func TestNull(t *testing.T) {
	v := Null()
	if v.Tag() != JS_TAG_NULL {
		t.Errorf("Null() tag = %v, want %v", v.Tag(), JS_TAG_NULL)
	}
}

func TestFloatValue(t *testing.T) {
	v := NewFloat(3.14)
	if v.Tag() != JS_TAG_FLOAT64 {
		t.Errorf("float tag = %v, want %v", v.Tag(), JS_TAG_FLOAT64)
	}
}

func TestTagHelpers(t *testing.T) {
	tests := []struct {
		tag    Tag
		isHeap bool
		isInline bool
	}{
		{JS_TAG_INT, false, true},
		{JS_TAG_BOOL, false, true},
		{JS_TAG_NULL, false, true},
		{JS_TAG_UNDEFINED, false, true},
		{JS_TAG_OBJECT, true, false},
		{JS_TAG_STRING, true, false},
	}

	for _, tt := range tests {
		if tt.tag.IsHeapTag() != tt.isHeap {
			t.Errorf("Tag(%d).IsHeapTag() = %v, want %v", tt.tag, tt.tag.IsHeapTag(), tt.isHeap)
		}
		if tt.tag.IsInlineTag() != tt.isInline {
			t.Errorf("Tag(%d).IsInlineTag() = %v, want %v", tt.tag, tt.tag.IsInlineTag(), tt.isInline)
		}
	}
}

func TestConversion(t *testing.T) {
	tests := []struct {
		val       JSValue
		wantFloat float64
		wantBool  bool
	}{
		{NewInt(0), 0, false},
		{NewInt(1), 1, true},
		{NewInt(-5), -5, true},
		{True(), 1, true},
		{False(), 0, false},
		{Undefined(), 0, false},
		{Null(), 0, false},
	}

	for _, tt := range tests {
		got := ToFloat64(tt.val)
		if got != tt.wantFloat {
			t.Errorf("ToFloat64(%v) = %v, want %v", tt.val, got, tt.wantFloat)
		}
		gotBool := ToBool(tt.val)
		if gotBool != tt.wantBool {
			t.Errorf("ToBool(%v) = %v, want %v", tt.val, gotBool, tt.wantBool)
		}
	}
}

func TestAdd(t *testing.T) {
	a := NewInt(1)
	b := NewInt(1)
	result := Add(a, b)
	if n, ok := result.(IntValue); !ok || int64(n) != 2 {
		t.Errorf("Add(1, 1) = %v, want 2", result)
	}
}

func TestSub(t *testing.T) {
	a := NewInt(5)
	b := NewInt(3)
	result := Sub(a, b)
	if n, ok := result.(IntValue); !ok || int64(n) != 2 {
		t.Errorf("Sub(5, 3) = %v, want 2", result)
	}
}

func TestMul(t *testing.T) {
	a := NewInt(4)
	b := NewInt(2)
	result := Mul(a, b)
	if n, ok := result.(IntValue); !ok || int64(n) != 8 {
		t.Errorf("Mul(4, 2) = %v, want 8", result)
	}
}

func TestDiv(t *testing.T) {
	a := NewInt(10)
	b := NewInt(3)
	result := Div(a, b)
	// Float division: 10/3 = 3.333...
	if f, ok := result.(Float64Value); !ok || f.v < 3.3 || f.v > 3.4 {
		t.Errorf("Div(10, 3) = %v, want ~3.333", result)
	}
}

func TestMod(t *testing.T) {
	a := NewInt(10)
	b := NewInt(3)
	result := Mod(a, b)
	if n, ok := result.(IntValue); !ok || int64(n) != 1 {
		t.Errorf("Mod(10, 3) = %v, want 1", result)
	}
}

func TestLt(t *testing.T) {
	if !Lt(NewInt(1), NewInt(2)) {
		t.Error("Lt(1, 2) = false, want true")
	}
	if Lt(NewInt(3), NewInt(2)) {
		t.Error("Lt(3, 2) = true, want false")
	}
}

func TestLte(t *testing.T) {
	if !Lte(NewInt(2), NewInt(2)) {
		t.Error("Lte(2, 2) = false, want true")
	}
}

func TestGt(t *testing.T) {
	if !Gt(NewInt(2), NewInt(1)) {
		t.Error("Gt(2, 1) = false, want true")
	}
	if Gt(NewInt(1), NewInt(2)) {
		t.Error("Gt(1, 2) = true, want false")
	}
}

func TestGte(t *testing.T) {
	if !Gte(NewInt(2), NewInt(2)) {
		t.Error("Gte(2, 2) = false, want true")
	}
}

func TestStrictEq(t *testing.T) {
	// Same type, same value
	if !StrictEq(NewInt(1), NewInt(1)) {
		t.Error("StrictEq(1, 1) = false, want true")
	}
	// Same type, different value
	if StrictEq(NewInt(1), NewInt(2)) {
		t.Error("StrictEq(1, 2) = true, want false")
	}
	// Different type (1 vs 1.5 - different enough to be different types)
	if StrictEq(NewInt(1), NewFloat(1.5)) {
		t.Error("StrictEq(1, 1.5) = true, want false (different types)")
	}
}