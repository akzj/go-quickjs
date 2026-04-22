package compiler

import (
	"github.com/akzj/go-quickjs/pkg/lexer"
	"github.com/akzj/go-quickjs/pkg/opcode"
	"github.com/akzj/go-quickjs/pkg/value"
)

// Bytecode represents compiled JavaScript code
type Bytecode struct {
	Code     []byte          // instruction bytes
	Pool     []value.JSValue // constant pool (stores function bytecode)
	VarCount int             // number of local variables
	VarNames []string        // variable names (index -> name)
	ArgCount int             // number of arguments
}

// FunctionInfo stores information about a compiled function
type FunctionInfo struct {
	Bytecode *Bytecode // nested function bytecode
}

// SimpleCompile compiles JavaScript source to bytecode.
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
	if len(c.bc.Code) == 0 || c.bc.Code[len(c.bc.Code)-1] != byte(opcode.OP_return) {
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_return))
	}
	c.bc.VarCount = len(c.bc.VarNames)
	return c.bc
}

func (c *Compiler) peek() lexer.Token {
	if c.pos < len(c.tokens) {
		return c.tokens[c.pos]
	}
	return lexer.Token{Type: lexer.TokenEof}
}

func (c *Compiler) next() lexer.Token {
	tok := c.peek()
	c.pos++
	return tok
}

func (c *Compiler) expect(typ lexer.TokenType) bool {
	if c.peek().Type == typ {
		c.pos++
		return true
	}
	return false
}

func (c *Compiler) parseProgram() {
	for c.peek().Type != lexer.TokenEof {
		c.parseStatement()
	}
}

func (c *Compiler) parseStatement() {
	tok := c.peek()
	switch tok.Type {
	case lexer.TokenVar, lexer.TokenLet:
		c.parseVarDecl(tok.Type == lexer.TokenLet, true)
	case lexer.TokenFunction:
		c.parseFunction()
	case lexer.TokenReturn:
		c.parseReturn()
	case lexer.TokenIf:
		c.parseIf()
	case lexer.TokenWhile:
		c.parseWhile()
	case lexer.TokenLeftBrace:
		c.parseBlock()
	case lexer.TokenSemicolon:
		c.next()
	default:
		c.parseExpression()
		if c.peek().Type == lexer.TokenSemicolon {
			c.next()
		}
	}
}

func (c *Compiler) parseFunction() {
	c.next() // consume 'function'
	nameTok := c.next()
	if nameTok.Type != lexer.TokenIdent {
		return
	}
	funcName := nameTok.Str

	if !c.expect(lexer.TokenLeftParen) {
		return
	}
	if !c.expect(lexer.TokenRightParen) {
		return
	}
	if !c.expect(lexer.TokenLeftBrace) {
		return
	}

	// Create nested bytecode for function body
	nestedBC := &Bytecode{
		Code:     make([]byte, 0, 32),
		Pool:     make([]value.JSValue, 0, 4),
		VarNames: make([]string, 0),
	}

	// Swap bytecode context
	oldBC := c.bc
	c.bc = nestedBC

	// Parse function body statements until '}'
	for c.peek().Type != lexer.TokenRightBrace && c.peek().Type != lexer.TokenEof {
		c.parseStatement()
	}
	c.expect(lexer.TokenRightBrace) // consume '}'

	if len(c.bc.Code) == 0 || c.bc.Code[len(c.bc.Code)-1] != byte(opcode.OP_return) {
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_return))
	}
	nestedBC.VarCount = len(nestedBC.VarNames)
	c.bc = oldBC

	// Store function bytecode in constant pool
	funcInfo := &FunctionInfo{Bytecode: nestedBC}
	c.bc.Pool = append(c.bc.Pool, value.MakeFunction(funcInfo))
	funcIdx := len(c.bc.Pool) - 1

	// Register function as variable and store it
	funcVarIdx := c.registerVar(funcName)
	c.emitPushConst(uint32(funcIdx))
	c.emitU16(opcode.OP_put_var, uint16(funcVarIdx))
}

func (c *Compiler) parseReturn() {
	c.next() // consume 'return'
	if c.peek().Type != lexer.TokenRightBrace && c.peek().Type != lexer.TokenSemicolon && c.peek().Type != lexer.TokenEof {
		c.parseExpression()
	} else {
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_undefined))
	}
	c.bc.Code = append(c.bc.Code, byte(opcode.OP_ret))
}

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
	c.expect(lexer.TokenRightBrace)
}

func (c *Compiler) registerVar(name string) int {
	for i, n := range c.bc.VarNames {
		if n == name {
			return i
		}
	}
	idx := len(c.bc.VarNames)
	c.bc.VarNames = append(c.bc.VarNames, name)
	return idx
}

func (c *Compiler) parseVarDecl(isLet bool, dropResult bool) {
	if isLet {
		c.next()
	} else {
		c.next()
	}
	for {
		nameTok := c.next()
		if nameTok.Type != lexer.TokenIdent {
			return
		}
		name := nameTok.Str
		idx := c.registerVar(name)

		if c.peek().Type == lexer.TokenAssign {
			c.next()
			c.parseExpression()
		} else {
			c.bc.Code = append(c.bc.Code, byte(opcode.OP_undefined))
		}

		if isLet {
			c.emitU16(opcode.OP_put_var_init, uint16(idx))
		} else {
			c.emitU16(opcode.OP_put_var, uint16(idx))
		}

		if c.peek().Type == lexer.TokenComma {
			c.next()
			continue
		}
		break
	}
	if c.peek().Type == lexer.TokenSemicolon {
		c.next()
	}
}

func (c *Compiler) parseIf() {
	c.next()
	if !c.expect(lexer.TokenLeftParen) {
		return
	}
	c.parseExpression()
	if !c.expect(lexer.TokenRightParen) {
		return
	}
	ifFalsePos := len(c.bc.Code)
	c.emitLabel(opcode.OP_if_false, 0)
	c.parseStatement()
	thenBranchEnd := len(c.bc.Code) + 5
	c.patchLabel(ifFalsePos, thenBranchEnd-(ifFalsePos+5))
	if c.peek().Type == lexer.TokenElse {
		c.next()
		gotoEndPos := len(c.bc.Code)
		c.emitLabel(opcode.OP_goto, 0)
		c.parseStatement()
		elseEnd := len(c.bc.Code)
		c.patchLabel(gotoEndPos, elseEnd-(gotoEndPos+5))
	}
}

func (c *Compiler) parseWhile() {
	c.next()
	loopStart := len(c.bc.Code)
	if !c.expect(lexer.TokenLeftParen) {
		return
	}
	c.parseExpression()
	if !c.expect(lexer.TokenRightParen) {
		return
	}
	exitPos := len(c.bc.Code)
	c.emitLabel(opcode.OP_if_false, 0)
	c.parseStatement()
	gotoPos := len(c.bc.Code)
	c.emitLabel(opcode.OP_goto, 0)
	c.patchLabel(gotoPos, loopStart-gotoPos-5)
	gotoTarget := len(c.bc.Code)
	c.patchLabel(exitPos, gotoTarget-(exitPos+5))
}

func (c *Compiler) parseExpression() {
	c.parseAssignment()
}

func (c *Compiler) parseAssignment() {
	tok := c.peek()
	if tok.Type == lexer.TokenIdent {
		savedPos := c.pos
		c.next()
		nextTok := c.peek()
		if nextTok.Type == lexer.TokenAssign {
			c.next()
			name := tok.Str
			idx := c.registerVar(name)
			c.parseAssignment()
			c.emitU16(opcode.OP_put_var, uint16(idx))
			return
		}
		// Restore position for function call or other cases
		c.pos = savedPos
	}
	c.parseComparison()
}

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

func (c *Compiler) parseUnary() {
	c.parsePrimary()
}

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
		if c.peek().Type == lexer.TokenLeftParen {
			c.parseCall(idx)
		}
	case lexer.TokenLeftParen:
		c.next()
		c.parseExpression()
		c.expect(lexer.TokenRightParen)
	default:
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_undefined))
	}
}

func (c *Compiler) parseCall(funcVarIdx int) {
	c.next() // consume '('
	argCount := 0
	for c.peek().Type != lexer.TokenRightParen && c.peek().Type != lexer.TokenEof {
		c.parseExpression()
		argCount++
		if c.peek().Type == lexer.TokenComma {
			c.next()
		} else {
			break
		}
	}
	c.expect(lexer.TokenRightParen)

	if argCount == 0 {
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_call0))
	} else if argCount == 1 {
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_call1))
	} else if argCount == 2 {
		c.bc.Code = append(c.bc.Code, byte(opcode.OP_call2))
	} else {
		c.emitU16(opcode.OP_call, uint16(argCount))
	}
}

func (c *Compiler) emitPushI32(v int32) {
	c.bc.Code = append(c.bc.Code, byte(opcode.OP_push_i32))
	c.bc.Code = append(c.bc.Code,
		byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
	)
}

func (c *Compiler) emitPushConst(v uint32) {
	c.bc.Code = append(c.bc.Code, byte(opcode.OP_push_const))
	c.bc.Code = append(c.bc.Code,
		byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
	)
}

func (c *Compiler) emitU16(op opcode.Opcode, v uint16) {
	c.bc.Code = append(c.bc.Code, byte(op))
	c.bc.Code = append(c.bc.Code, byte(v), byte(v>>8))
}

func (c *Compiler) emitLabel(op opcode.Opcode, offset int32) {
	c.bc.Code = append(c.bc.Code, byte(op))
	c.bc.Code = append(c.bc.Code,
		byte(offset), byte(offset>>8), byte(offset>>16), byte(offset>>24),
	)
}

func (c *Compiler) patchLabel(pos int, offset int) {
	if pos+1 >= len(c.bc.Code) {
		return
	}
	c.bc.Code[pos+1] = byte(offset)
	c.bc.Code[pos+2] = byte(offset >> 8)
	c.bc.Code[pos+3] = byte(offset >> 16)
	c.bc.Code[pos+4] = byte(offset >> 24)
}
