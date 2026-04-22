package compiler

import (
	"github.com/akzj/go-quickjs/pkg/lexer"
	"github.com/akzj/go-quickjs/pkg/opcode"
	"github.com/akzj/go-quickjs/pkg/value"
)

// Bytecode represents compiled JavaScript code
type Bytecode struct {
	Code     []byte          // instruction bytes
	Pool     []value.JSValue // constant pool
	VarCount int             // number of local variables
	VarNames []string        // variable names (index -> name)
	ArgCount int             // number of arguments
}

// SimpleCompile compiles JavaScript source to bytecode.
// This is a convenience function that combines lexing and compiling.
func SimpleCompile(source string) *Bytecode {
	tokens := lexer.TokenizeSimple(source)
	if tokens == nil || len(tokens) == 0 {
		return nil
	}

	c := NewCompiler(tokens)
	return c.Compile()
}

// Compiler transforms tokens into bytecode
type Compiler struct {
	tokens []lexer.Token
	pos    int
	bc     *Bytecode
}

// NewCompiler creates a new compiler from tokens
func NewCompiler(tokens []lexer.Token) *Compiler {
	return &Compiler{
		tokens: tokens,
		pos:    0,
		bc: &Bytecode{
			Code:     make([]byte, 0, 64),
			Pool:     make([]value.JSValue, 0, 8),
			VarNames: make([]string, 0),
		},
	}
}

// Compile compiles the tokens and returns bytecode
func (c *Compiler) Compile() *Bytecode {
	c.parseProgram()

	// Ensure return at end
	if len(c.bc.Code) == 0 || c.bc.Code[len(c.bc.Code)-1] != byte(opcode.OP_return) {
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_return))
	}

	c.bc.VarCount = len(c.bc.VarNames)
	return c.bc
}

// peek returns the current token without consuming
func (c *Compiler) peek() lexer.Token {
	if c.pos < len(c.tokens) {
		return c.tokens[c.pos]
	}
	return lexer.Token{Type: lexer.TokenEof}
}

// next consumes and returns the next token
func (c *Compiler) next() lexer.Token {
	tok := c.peek()
	c.pos++
	return tok
}

// expect consumes a token of expected type, returns true if matched
func (c *Compiler) expect(typ lexer.TokenType) bool {
	if c.peek().Type == typ {
		c.pos++
		return true
	}
	return false
}

// parseProgram parses the entire program
func (c *Compiler) parseProgram() {
	for c.peek().Type != lexer.TokenEof {
		c.parseStatement()
	}
}

// parseStatement parses a statement
func (c *Compiler) parseStatement() {
	tok := c.peek()
	switch tok.Type {
	case lexer.TokenVar, lexer.TokenLet:
		c.parseVarDecl(tok.Type == lexer.TokenLet, true) // as statement, drop result
	case lexer.TokenIf:
		c.parseIf()
	case lexer.TokenWhile:
		c.parseWhile()
	case lexer.TokenLeftBrace:
		c.parseBlock()
	case lexer.TokenSemicolon:
		c.next() // consume empty statement
	default:
		c.parseExpression()
		// Optional semicolon
		if c.peek().Type == lexer.TokenSemicolon {
			c.next()
		}
	}
}

// parseBlock parses a block: { statement* }
func (c *Compiler) parseBlock() {
	if !c.expect(lexer.TokenLeftBrace) {
		return
	}
	for {
		if c.peek().Type == lexer.TokenRightBrace || c.peek().Type == lexer.TokenEof {
			break
		}
		c.parseStatement()
	}
	if !c.expect(lexer.TokenRightBrace) {
		// Error - missing closing brace
	}
}

// registerVar registers a variable and returns its index
func (c *Compiler) registerVar(name string) int {
	// Check if already registered
	for i, n := range c.bc.VarNames {
		if n == name {
			return i
		}
	}
	idx := len(c.bc.VarNames)
	c.bc.VarNames = append(c.bc.VarNames, name)
	return idx
}

// parseVarDecl parses var/let declaration: "var x = expr;" or "let y = expr;"
// dropResult: if true, emit OP_drop after the declaration (for statement context)
//             if false, keep the value (for expression context like: var x = 5, y = x)
func (c *Compiler) parseVarDecl(isLet bool, dropResult bool) {
	if isLet {
		c.next() // consume 'let'
	} else {
		c.next() // consume 'var'
	}

	for {
		nameTok := c.next()
		if nameTok.Type != lexer.TokenIdent {
			// Error: expected identifier
			return
		}
		name := nameTok.Str

		// Register variable
		idx := c.registerVar(name)

		// Check for initializer: "= expr"
		if c.peek().Type == lexer.TokenAssign {
			c.next() // consume '='
			c.parseExpression()
		} else {
			// Default value is undefined
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_undefined))
		}

		// put_var_init or put_var based on declaration type
		// NOTE: put_var/pop_var_init already pops the value from the stack
		if isLet {
			c.emitU16(opcode.OP_put_var_init, uint16(idx))
		} else {
			c.emitU16(opcode.OP_put_var, uint16(idx))
		}

		// Check for comma (multiple declarations)
		if c.peek().Type == lexer.TokenComma {
			c.next()
			continue
		}
		break
	}

	// Consume semicolon
	if c.peek().Type == lexer.TokenSemicolon {
		c.next()
	}
}

// parseIf parses: if (cond) stmt [else stmt]
func (c *Compiler) parseIf() {
	c.next() // consume 'if'

	// Expect '('
	if !c.expect(lexer.TokenLeftParen) {
		return
	}

	// Parse condition
	c.parseExpression()

	// Expect ')'
	if !c.expect(lexer.TokenRightParen) {
		return
	}

	// Condition is on stack - emit if_false to skip then-branch
	ifFalsePos := len(c.bc.Code)
	c.emitLabel(opcode.OP_if_false, 0) // placeholder (5 bytes)

	// Parse then-branch
	c.parseStatement()

	// thenBranchEnd must include the goto that comes BEFORE else branch
	// The goto is 5 bytes, so we add 5 to account for it
	thenBranchEnd := len(c.bc.Code) + 5

	// Patch if_false: offset from AFTER this instruction (ifFalsePos+5) to thenBranchEnd
	c.patchLabel(ifFalsePos, thenBranchEnd-(ifFalsePos+5))

	// Check for else
	if c.peek().Type == lexer.TokenElse {
		c.next() // consume 'else'

		// Emit goto to skip else when then is done (this goes BEFORE else branch)
		gotoEndPos := len(c.bc.Code)
		c.emitLabel(opcode.OP_goto, 0) // placeholder

		// Parse else-branch
		c.parseStatement()

		// elseEnd is right after else-branch (no +5 needed, this is final destination)
		elseEnd := len(c.bc.Code)

		// Patch end goto: offset from AFTER goto (gotoEndPos+5) to elseEnd
		c.patchLabel(gotoEndPos, elseEnd-(gotoEndPos+5))
	}
}

// parseWhile parses: while (cond) stmt
func (c *Compiler) parseWhile() {
	c.next() // consume 'while'

	// Loop start position (for backpatching)
	loopStart := len(c.bc.Code)

	// Expect '('
	if !c.expect(lexer.TokenLeftParen) {
		return
	}

	// Parse condition
	c.parseExpression()

	// Expect ')'
	if !c.expect(lexer.TokenRightParen) {
		return
	}

	// Condition on stack - emit if_false to exit loop
	exitPos := len(c.bc.Code)
	c.emitLabel(opcode.OP_if_false, 0) // placeholder

	// Parse body
	c.parseStatement()

	// Emit goto back to condition
	// Fix: capture gotoPos BEFORE emitLabel, then patch
	gotoPos := len(c.bc.Code)         // position BEFORE emit (after body)
	c.emitLabel(opcode.OP_goto, 0)    // emit placeholder
	// After emit: len = gotoPos + 5
	// We want: gotoPos + 5 + offset = loopStart
	// So: offset = loopStart - (gotoPos + 5)
	c.patchLabel(gotoPos, loopStart-gotoPos-5)

	// After goto is emitted, current position is AFTER the goto (gotoPos + 5)
	// This is also where if_false should jump to (exit the loop)
	gotoTarget := len(c.bc.Code) // position AFTER goto

	// Patch exit jump: if_false should jump to gotoTarget
	// if_false at position exitPos, after reading offset: PC = exitPos + 5
	// We want: exitPos + 5 + offset = gotoTarget
	// So: offset = gotoTarget - (exitPos + 5)
	c.patchLabel(exitPos, gotoTarget-(exitPos+5))
}

// parseExpression parses an expression (supports assignment)
func (c *Compiler) parseExpression() {
	c.parseAssignment()
}

// parseAssignment handles = (lowest precedence)
func (c *Compiler) parseAssignment() {
	// Check if this is an assignment: ident = expr
	tok := c.peek()
	if tok.Type == lexer.TokenIdent {
		// Look ahead to check if next is =
		savedPos := c.pos
		c.next() // consume identifier
		if c.peek().Type == lexer.TokenAssign {
			c.next() // consume =
			// It's an assignment: ident = expr
			name := tok.Str
			idx := c.registerVar(name)
			c.parseAssignment() // parse right-hand side
			// Value is on stack, now store it
			c.emitU16(opcode.OP_put_var, uint16(idx))
			return
		}
		// Not an assignment, restore position
		c.pos = savedPos
	}
	// Not an assignment, parse comparison
	c.parseComparison()
}

// parseAdditive handles + and - (lower precedence than */%)
func (c *Compiler) parseAdditive() {
	c.parseMultiplicative()

	for {
		switch c.peek().Type {
		case lexer.TokenPlus:
			c.next()
			c.parseMultiplicative()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_add))
		case lexer.TokenMinus:
			c.next()
			c.parseMultiplicative()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_sub))
		default:
			return
		}
	}
}

// parseComparison handles relational operators
func (c *Compiler) parseComparison() {
	c.parseAdditive()

	for {
		switch c.peek().Type {
		case lexer.TokenLt:
			c.next()
			c.parseAdditive()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_lt))
		case lexer.TokenLte:
			c.next()
			c.parseAdditive()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_lte))
		case lexer.TokenGt:
			c.next()
			c.parseAdditive()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_gt))
		case lexer.TokenGte:
			c.next()
			c.parseAdditive()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_gte))
		case lexer.TokenEq:
			c.next()
			c.parseAdditive()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_eq))
		case lexer.TokenNeq:
			c.next()
			c.parseAdditive()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_neq))
		default:
			return
		}
	}
}

// parseMultiplicative handles *, /, % (higher precedence than +-)
func (c *Compiler) parseMultiplicative() {
	c.parseUnary()

	for {
		switch c.peek().Type {
		case lexer.TokenMul:
			c.next()
			c.parseUnary()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_mul))
		case lexer.TokenDiv:
			c.next()
			c.parseUnary()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_div))
		case lexer.TokenMod:
			c.next()
			c.parseUnary()
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_mod))
		default:
			return
		}
	}
}

// parseUnary handles unary operators (not yet implemented)
func (c *Compiler) parseUnary() {
	c.parsePrimary()
}

// parsePrimary handles primary expressions
func (c *Compiler) parsePrimary() {
	tok := c.peek()

	switch tok.Type {
	case lexer.TokenNum:
		c.next()
		c.emitPushI32(tok.Value)

	case lexer.TokenTrue:
		c.next()
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_push_true))

	case lexer.TokenFalse:
		c.next()
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_push_false))

	case lexer.TokenUndefined:
		c.next()
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_undefined))

	case lexer.TokenNull:
		c.next()
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_null))

	case lexer.TokenIdent:
		c.next()
		name := tok.Str
		idx := c.registerVar(name)
		c.emitU16(opcode.OP_get_var_undef, uint16(idx))

	case lexer.TokenLeftParen:
		c.next()
		c.parseExpression()
		if !c.expect(lexer.TokenRightParen) {
			// Error - missing closing paren
		}

	default:
		// Unexpected token - push undefined
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_undefined))
	}
}

// emitPushI32 emits push_i32 instruction
func (c *Compiler) emitPushI32(v int32) {
	c.bc.Code = append(c.bc.Code, byte(opcode.OP_push_i32))
	c.bc.Code = append(c.bc.Code,
		byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
	)
}

// emitU16 emits opcode + u16 operand
func (c *Compiler) emitU16(op opcode.Opcode, v uint16) {
	c.bc.Code = append(c.bc.Code, byte(op))
	c.bc.Code = append(c.bc.Code, byte(v), byte(v>>8))
}

// emitLabel emits jump instruction with placeholder offset
func (c *Compiler) emitLabel(op opcode.Opcode, offset int32) {
	c.bc.Code = append(c.bc.Code, byte(op))
	c.bc.Code = append(c.bc.Code,
		byte(offset), byte(offset>>8), byte(offset>>16), byte(offset>>24),
	)
}

// patchLabel patches the offset at position
func (c *Compiler) patchLabel(pos int, offset int) {
	if pos+1 >= len(c.bc.Code) {
		return
	}
	// pos is where the opcode starts, offset starts at pos+1 (4 bytes)
	c.bc.Code[pos+1] = byte(offset)
	c.bc.Code[pos+2] = byte(offset >> 8)
	c.bc.Code[pos+3] = byte(offset >> 16)
	c.bc.Code[pos+4] = byte(offset >> 24)
}