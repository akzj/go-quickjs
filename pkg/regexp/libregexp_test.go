package regexp

import (
	"fmt"
	"testing"
)

func TestCompileSimple(t *testing.T) {
	tests := []struct {
		pattern string
		flags   int
		wantErr bool
	}{
		{"abc", 0, false},
		{"", 0, false},
		{"a+b*c", 0, false},
		{"a|b", 0, false},
		{"^test$", 0, false},
		{"(test)", 0, false},
		{"\\d+\\.\\d+", 0, false},
		{"[a-z]+", 0, false},
		{"[\\w]+", 0, false},
		{"a{2,5}", 0, false},
		// Invalid patterns
		{"[", 0, true},    // unterminated
		{"(", 0, true},    // unclosed group
		{"*", 0, true},    // nothing to repeat
	}

	for _, tt := range tests {
		_, err := Compile(tt.pattern, tt.flags, nil)
		if (err != nil) != tt.wantErr {
			t.Errorf("Compile(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
		}
	}
}

func TestMatchSimple(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{"abc", "abc", true},
		{"abc", "ab", false},
		{"abc", "xabc", true},  // non-sticky allows prefix match
		{"^abc$", "abc", true},
		{"^abc$", "xabc", false},
		{"a+b", "aaaab", true},
		{"a*b", "b", true},
		{"a?b", "ab", true},
		{"a?b", "b", true},
		{"test|best", "test", true},
		{"test|best", "best", true},
		{"test|best", "rest", false},
	}

	for _, tt := range tests {
		bc, err := Compile(tt.pattern, 0, nil)
		if err != nil {
			t.Errorf("Compile(%q) failed: %v", tt.pattern, err)
			continue
		}
		capture := make([][]byte, GetAllocCount(bc))
		result := Match(bc, []byte(tt.input), 0, 0, nil, capture)
		matched := result == RetMatch
		if matched != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.input, matched, tt.want)
		}
	}
}

func TestMatchFlags(t *testing.T) {
	tests := []struct {
		pattern string
		flags   int
		input   string
		want    bool
	}{
		{"abc", FlagIgnoreCase, "ABC", true},
		{"ABC", FlagIgnoreCase, "abc", true},
		{"abc", 0, "ABC", false},
		{"^test$", FlagMultiline, "test\ntest", true},
		{"^a", FlagMultiline, "a\na", true},
		{".*", FlagDotAll, "a\nb", true},
	}

	for _, tt := range tests {
		bc, err := Compile(tt.pattern, tt.flags, nil)
		if err != nil {
			t.Errorf("Compile(%q, flags=%d) failed: %v", tt.pattern, tt.flags, err)
			continue
		}
		capture := make([][]byte, GetAllocCount(bc))
		result := Match(bc, []byte(tt.input), 0, 0, nil, capture)
		matched := result == RetMatch
		if matched != tt.want {
			t.Errorf("Match(%q, flags=%d, %q) = %v, want %v", tt.pattern, tt.flags, tt.input, matched, tt.want)
		}
	}
}

func TestMatchCaptures(t *testing.T) {
	bc, err := Compile("a(b)c", 0, nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	capture := make([][]byte, GetAllocCount(bc))
	result := Match(bc, []byte("abc"), 0, 0, nil, capture)

	if result != RetMatch {
		t.Fatalf("Match failed, got %d", result)
	}

	// Check capture[0] - full match (start pointer)
	if string(capture[0]) != "abc" {
		t.Errorf("capture[0] = %q, want %q", string(capture[0]), "abc")
	}

	// capture format is [start0, end0, start1, end1, ...] in pointer format
	// capture[1] is end pointer (currently empty string placeholder)
	// capture[2] is start of group 1
	// capture[3] is end of group 1
}

func TestMatchEscapeSequences(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{`\d+`, "123", true},
		{`\d+`, "abc", false},
		{`\D+`, "abc", true},
		{`\D+`, "123", false},
		{`\s+`, "   ", true},
		{`\S+`, "abc", true},
		{`\w+`, "abc123", true},
		{`\W+`, "   ", true},
	}

	for _, tt := range tests {
		bc, err := Compile(tt.pattern, 0, nil)
		if err != nil {
			t.Errorf("Compile(%q) failed: %v", tt.pattern, err)
			continue
		}
		capture := make([][]byte, GetAllocCount(bc))
		result := Match(bc, []byte(tt.input), 0, 0, nil, capture)
		matched := result == RetMatch
		if matched != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.input, matched, tt.want)
		}
	}
}

func TestMatchGroups(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{"(?:abc)+", "abcabc", true},
		{"(abc)+", "abcabc", true},
	}

	for _, tt := range tests {
		bc, err := Compile(tt.pattern, 0, nil)
		if err != nil {
			t.Errorf("Compile(%q) failed: %v", tt.pattern, err)
			continue
		}
		capture := make([][]byte, GetAllocCount(bc))
		result := Match(bc, []byte(tt.input), 0, 0, nil, capture)
		matched := result == RetMatch
		if matched != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.input, matched, tt.want)
		}
	}
}

func TestBackReference(t *testing.T) {
	bc, err := Compile("(\\w)\\1", 0, nil)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	fmt.Printf("TEST: bc len=%d\n", len(bc))
	// Dump bytecode bytes
	fmt.Printf("TEST bytecode: ")
	for i := 0; i < len(bc); i++ {
		fmt.Printf("[%d]=0x%02x ", i, bc[i])
	}
	fmt.Println()
	capture := make([][]byte, GetAllocCount(bc))
	
	// Should match "aa", "bb", etc.
	// Dump bytecode right before Match
	fmt.Printf("TEST bytecode before Match: ")
	for i := 0; i < len(bc); i++ {
		fmt.Printf("[%d]=0x%02x ", i, bc[i])
	}
	fmt.Println()
	result := Match(bc, []byte("aa"), 0, 0, nil, capture)
	if result != RetMatch {
		t.Errorf("BackReference 'aa' should match, got %d", result)
	}

	// Should not match "ab"
	result = Match(bc, []byte("ab"), 0, 0, nil, capture)
	if result != RetNoMatch {
		t.Errorf("BackReference 'ab' should not match, got %d", result)
	}
}

func TestEmptyPattern(t *testing.T) {
	bc, err := Compile("", 0, nil)
	if err != nil {
		t.Fatalf("Compile empty pattern failed: %v", err)
	}
	capture := make([][]byte, GetAllocCount(bc))
	result := Match(bc, []byte("anything"), 0, 0, nil, capture)
	if result != RetMatch {
		t.Errorf("Empty pattern should match anything, got %d", result)
	}
}