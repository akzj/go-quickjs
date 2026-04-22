package lexer

import (
	"testing"
)

func TestNumbers(t *testing.T) {
	tests := []struct {
		input    string
		expected int32
	}{
		{"123", 123},
		{"0", 0},
		{"42", 42},
		{"999", 999},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		tok := l.NextToken()
		if tok.Type != TokenNum {
			t.Errorf("Expected TokenNum, got %v", tok.Type)
		}
		if tok.Value != tt.expected {
			t.Errorf("Expected %d, got %d", tt.expected, tok.Value)
		}
	}
}

func TestKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"var", TokenVar},
		{"let", TokenLet},
		{"const", TokenConst},
		{"if", TokenIf},
		{"else", TokenElse},
		{"while", TokenWhile},
		{"for", TokenFor},
		{"function", TokenFunction},
		{"return", TokenReturn},
		{"true", TokenTrue},
		{"false", TokenFalse},
		{"null", TokenNull},
		{"undefined", TokenUndefined},
		{"break", TokenBreak},
		{"continue", TokenContinue},
		{"switch", TokenSwitch},
		{"case", TokenCase},
		{"default", TokenDefault},
		{"throw", TokenThrow},
		{"try", TokenTry},
		{"catch", TokenCatch},
		{"finally", TokenFinally},
		{"new", TokenNew},
		{"delete", TokenDelete},
		{"typeof", TokenTypeOf},
		{"void", TokenVoid},
		{"this", TokenThis},
		{"in", TokenIn},
		{"instanceof", TokenInstanceOf},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Type != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, tok.Type)
			}
		})
	}
}

func TestIdentifiers(t *testing.T) {
	l := NewLexer("foo bar baz")

	tok := l.NextToken()
	if tok.Type != TokenIdent || tok.Str != "foo" {
		t.Errorf("Expected 'foo', got '%s' type=%v", tok.Str, tok.Type)
	}

	tok = l.NextToken()
	if tok.Type != TokenIdent || tok.Str != "bar" {
		t.Errorf("Expected 'bar', got '%s'", tok.Str)
	}

	tok = l.NextToken()
	if tok.Type != TokenIdent || tok.Str != "baz" {
		t.Errorf("Expected 'baz', got '%s'", tok.Str)
	}
}

func TestEOF(t *testing.T) {
	l := NewLexer("")
	tok := l.NextToken()
	if tok.Type != TokenEof {
		t.Errorf("Expected EOF, got %v", tok.Type)
	}
}

func TestWhitespace(t *testing.T) {
	l := NewLexer("   42   ")
	tok := l.NextToken()
	if tok.Type != TokenNum || tok.Value != 42 {
		t.Errorf("Expected 42, got %d type=%v", tok.Value, tok.Type)
	}
	tok = l.NextToken()
	if tok.Type != TokenEof {
		t.Errorf("Expected EOF, got %v", tok.Type)
	}
}

func TestOperators(t *testing.T) {
	tests := []struct {
		input    string
		expected TokenType
	}{
		{"+", TokenPlus},
		{"-", TokenMinus},
		{"*", TokenMul},
		{"/", TokenDiv},
		{"%", TokenMod},
		{"(", TokenLeftParen},
		{")", TokenRightParen},
		{"{", TokenLeftBrace},
		{"}", TokenRightBrace},
		{"[", TokenLeftBracket},
		{"]", TokenRightBracket},
		{";", TokenSemicolon},
		{",", TokenComma},
		{".", TokenDot},
		{":", TokenColon},
		{"!", TokenBang},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Type != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, tok.Type)
			}
		})
	}
}

func TestCompoundOperators(t *testing.T) {
	tests := []struct {
		input       string
		expected    TokenType
		consumedOne bool // does it consume 2 chars or 1?
	}{
		{"==", TokenEq, true},
		{"!=", TokenNeq, true},
		{"<=", TokenLte, true},
		{">=", TokenGte, true},
		{"&&", TokenAnd, true},
		{"||", TokenOr, true},
		{"++", TokenPlusPlus, true},
		{"--", TokenMinusMinus, true},
		{"===", TokenStrictEq, true},
		{"!==", TokenStrictNeq, true},
		{"=>", TokenArrow, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Type != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, tok.Type)
			}
			tok = l.NextToken()
			if tok.Type != TokenEof {
				t.Errorf("Expected EOF after compound operator, got %v", tok.Type)
			}
		})
	}
}

func TestStringLiteral(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`"hello"`, "hello"},
		{"'world'", "world"},
		{`""`, ""},
		{`"test123"`, "test123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := NewLexer(tt.input)
			tok := l.NextToken()
			if tok.Type != TokenString {
				t.Errorf("Expected TokenString, got %v", tok.Type)
			}
			if tok.Str != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, tok.Str)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tokens := Tokenize("var x = 42;")
	expected := []TokenType{TokenVar, TokenIdent, TokenAssign, TokenNum, TokenSemicolon, TokenEof}
	if len(tokens) != len(expected) {
		t.Fatalf("Expected %d tokens, got %d", len(expected), len(tokens))
	}
	for i, exp := range expected {
		if tokens[i].Type != exp {
			t.Errorf("Token %d: expected %v, got %v", i, exp, tokens[i].Type)
		}
	}
}

func TestPositionTracking(t *testing.T) {
	l := NewLexer("a\nb")
	// skip 'a' and newline
	l.NextToken() // 'a'
	tok := l.NextToken() // '\n' + 'b' = skip whitespace
	if tok.Type != TokenIdent || tok.Str != "b" {
		t.Errorf("Expected 'b', got type=%v str=%s", tok.Type, tok.Str)
	}
	if tok.Line != 2 {
		t.Errorf("Expected line 2, got %d", tok.Line)
	}
}

func TestBackwardsCompatibility(t *testing.T) {
	// Ensure TokenizeSimple produces tokens compatible with old format
	source := "var x = 42;"
	tokens := TokenizeSimple(source)

	// Old format: token{typ, value, name}
	// Should have: var, ident(x), =, 42, ;, EOF
	if len(tokens) < 5 {
		t.Errorf("Expected at least 5 tokens, got %d", len(tokens))
	}

	if tokens[0].Type != TokenVar {
		t.Errorf("Expected TokenVar, got %v", tokens[0].Type)
	}
	if tokens[1].Type != TokenIdent || tokens[1].Str != "x" {
		t.Errorf("Expected ident 'x', got %v", tokens[1].Str)
	}
	if tokens[2].Type != TokenAssign {
		t.Errorf("Expected TokenAssign, got %v", tokens[2].Type)
	}
	if tokens[3].Type != TokenNum || tokens[3].Value != 42 {
		t.Errorf("Expected 42, got %d", tokens[3].Value)
	}
}

func BenchmarkTokenize(b *testing.B) {
	source := "var x = 42; while (x > 0) { x = x - 1; }"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Tokenize(source)
	}
}

func BenchmarkNewLexer(b *testing.B) {
	source := "var x = 42; while (x > 0) { x = x - 1; }"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewLexer(source)
	}
}
