package compiler

import (
	"testing"

	"github.com/akzj/go-quickjs/pkg/lexer"
)

func TestSimpleCompileNumber(t *testing.T) {
	bc := SimpleCompile("42")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// First byte should be OP_push_i32
	if bc.Code[0] != 1 {
		t.Errorf("Expected OP_push_i32 (1), got %d", bc.Code[0])
	}
	// Last byte should be OP_return (24) for complete program
	if bc.Code[len(bc.Code)-1] != 24 {
		t.Errorf("Expected OP_return (24), got %d", bc.Code[len(bc.Code)-1])
	}
}

func TestSimpleCompileArithmetic(t *testing.T) {
	bc := SimpleCompile("3 + 4")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should have: push_i32(3), push_i32(4), add, return
	if len(bc.Code) < 6 {
		t.Errorf("Expected at least 6 bytes, got %d", len(bc.Code))
	}
}

func TestSimpleCompileVarDecl(t *testing.T) {
	bc := SimpleCompile("var x = 5")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should register variable x
	if len(bc.VarNames) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(bc.VarNames))
	}
	if bc.VarNames[0] != "x" {
		t.Errorf("Expected variable 'x', got '%s'", bc.VarNames[0])
	}
}

func TestSimpleCompileVarDeclMultiple(t *testing.T) {
	bc := SimpleCompile("var x = 1, y = 2")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if len(bc.VarNames) != 2 {
		t.Errorf("Expected 2 variables, got %d", len(bc.VarNames))
	}
	if bc.VarNames[0] != "x" || bc.VarNames[1] != "y" {
		t.Errorf("Expected variables [x, y], got %v", bc.VarNames)
	}
}

func TestSimpleCompileLetDecl(t *testing.T) {
	bc := SimpleCompile("let a = 10")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if len(bc.VarNames) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(bc.VarNames))
	}
	if bc.VarNames[0] != "a" {
		t.Errorf("Expected variable 'a', got '%s'", bc.VarNames[0])
	}
}

func TestSimpleCompileIdentifier(t *testing.T) {
	bc := SimpleCompile("myVar")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should register myVar and push it
	if len(bc.VarNames) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(bc.VarNames))
	}
}

func TestSimpleCompileAssignment(t *testing.T) {
	bc := SimpleCompile("x = 42")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should register x, push 42, put_var
	if len(bc.VarNames) != 1 {
		t.Errorf("Expected 1 variable, got %d", len(bc.VarNames))
	}
}

func TestSimpleCompileEmpty(t *testing.T) {
	bc := SimpleCompile("")
	// Empty input may produce bytecode with just return, or nil
	if bc != nil && len(bc.Code) >= 1 && bc.Code[len(bc.Code)-1] == 14 {
		// Just a return - acceptable
		return
	}
}

func TestSimpleCompileBoolean(t *testing.T) {
	bc := SimpleCompile("true")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should compile without error
	if len(bc.Code) < 2 {
		t.Errorf("Expected at least 2 bytes, got %d", len(bc.Code))
	}
}

func TestSimpleCompileNull(t *testing.T) {
	bc := SimpleCompile("null")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if len(bc.Code) < 2 {
		t.Errorf("Expected at least 2 bytes, got %d", len(bc.Code))
	}
}

func TestSimpleCompileUndefined(t *testing.T) {
	bc := SimpleCompile("undefined")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if len(bc.Code) < 2 {
		t.Errorf("Expected at least 2 bytes, got %d", len(bc.Code))
	}
}

func TestNewCompiler(t *testing.T) {
	tokens := []lexer.Token{
		{Type: lexer.TokenNum, Value: 42},
		{Type: lexer.TokenEof},
	}
	c := NewCompiler(tokens)
	if c == nil {
		t.Fatal("NewCompiler returned nil")
	}
	bc := c.Compile()
	if bc == nil {
		t.Fatal("Compile returned nil")
	}
}

func TestCompilerVarCount(t *testing.T) {
	bc := SimpleCompile("var a = 1; var b = 2; var c = 3")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if bc.VarCount != 3 {
		t.Errorf("Expected VarCount=3, got %d", bc.VarCount)
	}
}

func TestSimpleCompileComparison(t *testing.T) {
	bc := SimpleCompile("3 < 5")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should have: push_i32(3), push_i32(5), OP_lt, return
	if len(bc.Code) < 6 {
		t.Errorf("Expected at least 6 bytes, got %d", len(bc.Code))
	}
}

func TestSimpleCompileWhile(t *testing.T) {
	// Simple while loop: while(true) {}
	bc := SimpleCompile("while(true) {}")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should have: push_true, if_false(goto exit), [empty block], goto(back), [exit]
	// That's at least 5 + 1 + 5 = 11 bytes minimum
	if len(bc.Code) < 10 {
		t.Errorf("Expected at least 10 bytes for while, got %d", len(bc.Code))
	}
}

func TestSimpleCompileIf(t *testing.T) {
	bc := SimpleCompile("if(true) x")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should have: push_true, if_false(skip), get_var(x), [end]
	if len(bc.Code) < 5 {
		t.Errorf("Expected at least 5 bytes for if, got %d", len(bc.Code))
	}
}

func TestSimpleCompileIfElse(t *testing.T) {
	bc := SimpleCompile("if(false) a; else b;")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should have goto instructions for the branches
	if len(bc.Code) < 10 {
		t.Errorf("Expected at least 10 bytes for if-else, got %d", len(bc.Code))
	}
}

func TestSimpleCompileBlock(t *testing.T) {
	bc := SimpleCompile("{ x; y; }")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Block with two statements
	if len(bc.VarNames) != 2 {
		t.Errorf("Expected 2 variables (x, y), got %d", len(bc.VarNames))
	}
}

func TestSimpleCompileExpressionStatement(t *testing.T) {
	bc := SimpleCompile("1 + 2;")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Expression statement with semicolon
	if len(bc.Code) < 6 {
		t.Errorf("Expected at least 6 bytes, got %d", len(bc.Code))
	}
}

func TestSimpleCompileNestedExpressions(t *testing.T) {
	bc := SimpleCompile("(1 + 2) * 3")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should handle parentheses correctly
	if len(bc.Code) < 8 {
		t.Errorf("Expected at least 8 bytes, got %d", len(bc.Code))
	}
}

func TestSimpleCompileFalse(t *testing.T) {
	bc := SimpleCompile("false")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if len(bc.Code) < 2 {
		t.Errorf("Expected at least 2 bytes, got %d", len(bc.Code))
	}
}

func TestSimpleCompileMultipleStatements(t *testing.T) {
	bc := SimpleCompile("var x = 1; var y = 2; x + y")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should have two var declarations and an expression
	if len(bc.VarNames) != 2 {
		t.Errorf("Expected 2 variables, got %d", len(bc.VarNames))
	}
}

func TestSimpleCompileAddition(t *testing.T) {
	bc := SimpleCompile("10 + 20")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	// Should compile addition
	if len(bc.Code) < 10 {
		t.Errorf("Expected at least 10 bytes for addition, got %d", len(bc.Code))
	}
}

func TestSimpleCompileSubtraction(t *testing.T) {
	bc := SimpleCompile("10 - 5")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if len(bc.Code) < 10 {
		t.Errorf("Expected at least 10 bytes for subtraction, got %d", len(bc.Code))
	}
}

func TestSimpleCompileMultiplication(t *testing.T) {
	bc := SimpleCompile("3 * 4")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if len(bc.Code) < 10 {
		t.Errorf("Expected at least 10 bytes for multiplication, got %d", len(bc.Code))
	}
}

func TestSimpleCompileDivision(t *testing.T) {
	bc := SimpleCompile("10 / 2")
	if bc == nil {
		t.Fatal("Expected bytecode, got nil")
	}
	if len(bc.Code) < 10 {
		t.Errorf("Expected at least 10 bytes for division, got %d", len(bc.Code))
	}
}