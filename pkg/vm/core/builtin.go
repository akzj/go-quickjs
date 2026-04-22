package core

import (
	"github.com/akzj/go-quickjs/pkg/lexer"
	"github.com/akzj/go-quickjs/pkg/opcode"
	"github.com/akzj/go-quickjs/pkg/value"
)

// tokenType is an alias for lexer.TokenType for backward compatibility
type tokenType = lexer.TokenType

// token is an alias for lexer.Token for backward compatibility
type token = lexer.Token

// CompileAndRun compiles JavaScript source and executes it
func (ctx *JSContext) CompileAndRun(source string) value.JSValue {
	bc := simpleCompile(source, ctx)
	if bc == nil {
		return value.Undefined()
	}
	return ctx.RunBytecode(bc)
}

// simpleCompile compiles JavaScript source to bytecode
// Supports: var/let declarations, identifiers, arithmetic, if/else, while
func simpleCompile(source string, ctx *JSContext) *Bytecode {
	tokens := tokenize(source)
	if tokens == nil || len(tokens) == 0 {
		return nil
	}

	bc := &Bytecode{
		Code:     make([]byte, 0, 64),
		Pool:     make([]value.JSValue, 0, 8),
		VarNames: make([]string, 0),
	}

	// Parse and compile
	p := &parser{
		tokens: tokens,
		pos:    0,
		bc:     bc,
	}
	p.parse()

	// Ensure return at end
	if len(bc.Code) == 0 || bc.Code[len(bc.Code)-1] != byte(opcode.OP_return) {
		bc.Code = append(bc.Code, byte(opcode.OP_return))
	}

	bc.VarCount = len(bc.VarNames)
	return bc
}

// parser handles parsing and bytecode generation
type parser struct {
	tokens []token
	pos    int
	bc     *Bytecode
}

// peek returns the current token without consuming
func (p *parser) peek() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{Type: lexer.TokenEof}
}

// next consumes and returns the next token
func (p *parser) next() token {
	tok := p.peek()
	p.pos++
	return tok
}

// expect consumes a token of expected type, returns true if matched
func (p *parser) expect(typ tokenType) bool {
	if p.peek().Type == typ {
		p.pos++
		return true
	}
	return false
}

// parse parses the entire program
func (p *parser) parse() {
	for p.peek().Type != lexer.TokenEof {
		p.parseStatement()
	}
}

// parseStatement parses a statement
func (p *parser) parseStatement() {
	tok := p.peek()
	switch tok.Type {
	case lexer.TokenVar, lexer.TokenLet:
		p.parseVarDecl(tok.Type == lexer.TokenLet, true) // as statement, drop result
	case lexer.TokenIf:
		p.parseIf()
	case lexer.TokenWhile:
		p.parseWhile()
	case lexer.TokenLeftBrace:
		p.parseBlock()
	case lexer.TokenSemicolon:
		p.next() // consume empty statement
	default:
		p.parseExpression()
		// Optional semicolon
		if p.peek().Type == lexer.TokenSemicolon {
			p.next()
		}
	}
}

// parseBlock parses a block: { statement* }
func (p *parser) parseBlock() {
	if !p.expect(lexer.TokenLeftBrace) {
		return
	}
	for {
		if p.peek().Type == lexer.TokenRightBrace || p.peek().Type == lexer.TokenEof {
			break
		}
		p.parseStatement()
	}
	if !p.expect(lexer.TokenRightBrace) {
		// Error - missing closing brace
	}
}

// registerVar registers a variable and returns its index
func (p *parser) registerVar(name string) int {
	// Check if already registered
	for i, n := range p.bc.VarNames {
		if n == name {
			return i
		}
	}
	idx := len(p.bc.VarNames)
	p.bc.VarNames = append(p.bc.VarNames, name)
	return idx
}

// parseVarDecl parses var/let declaration: "var x = expr;" or "let y = expr;"
// dropResult: if true, emit OP_drop after the declaration (for statement context)
//             if false, keep the value (for expression context like: var x = 5, y = x)
func (p *parser) parseVarDecl(isLet bool, dropResult bool) {
	if isLet {
		p.next() // consume 'let'
	} else {
		p.next() // consume 'var'
	}

	for {
		nameTok := p.next()
		if nameTok.Type != lexer.TokenIdent {
			// Error: expected identifier
			return
		}
		name := nameTok.Str

		// Register variable
		idx := p.registerVar(name)

		// Check for initializer: "= expr"
		if p.peek().Type == lexer.TokenAssign {
			p.next() // consume '='
			p.parseExpression()
		} else {
			// Default value is undefined
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_undefined))
		}

		// put_var_init or put_var based on declaration type
		// NOTE: put_var/pop_var_init already pops the value from the stack
		if isLet {
			p.emitU16(opcode.OP_put_var_init, uint16(idx))
		} else {
			p.emitU16(opcode.OP_put_var, uint16(idx))
		}

		// Check for comma (multiple declarations)
		if p.peek().Type == lexer.TokenComma {
			p.next()
			continue
		}
		break
	}

	// Consume semicolon
	if p.peek().Type == lexer.TokenSemicolon {
		p.next()
	}
}

// parseIf parses: if (cond) stmt [else stmt]
func (p *parser) parseIf() {
	p.next() // consume 'if'

	// Expect '('
	if !p.expect(lexer.TokenLeftParen) {
		return
	}

	// Parse condition
	p.parseExpression()

	// Expect ')'
	if !p.expect(lexer.TokenRightParen) {
		return
	}

	// Condition is on stack - emit if_false to skip then-branch
	ifFalsePos := len(p.bc.Code)
	p.emitLabel(opcode.OP_if_false, 0) // placeholder (5 bytes)

	// Parse then-branch
	p.parseStatement()

	// thenBranchEnd must include the goto that comes BEFORE else branch
	// The goto is 5 bytes, so we add 5 to account for it
	thenBranchEnd := len(p.bc.Code) + 5

	// Patch if_false: offset from AFTER this instruction (ifFalsePos+5) to thenBranchEnd
	p.patchLabel(ifFalsePos, thenBranchEnd-(ifFalsePos+5))

	// Check for else
	if p.peek().Type == lexer.TokenElse {
		p.next() // consume 'else'

		// Emit goto to skip else when then is done (this goes BEFORE else branch)
		gotoEndPos := len(p.bc.Code)
		p.emitLabel(opcode.OP_goto, 0) // placeholder

		// Parse else-branch
		p.parseStatement()

		// elseEnd is right after else-branch (no +5 needed, this is final destination)
		elseEnd := len(p.bc.Code)

		// Patch end goto: offset from AFTER goto (gotoEndPos+5) to elseEnd
		p.patchLabel(gotoEndPos, elseEnd-(gotoEndPos+5))
	}
}

// parseWhile parses: while (cond) stmt
func (p *parser) parseWhile() {
	p.next() // consume 'while'

	// Loop start position (for backpatching)
	loopStart := len(p.bc.Code)

	// Expect '('
	if !p.expect(lexer.TokenLeftParen) {
		return
	}

	// Parse condition
	p.parseExpression()

	// Expect ')'
	if !p.expect(lexer.TokenRightParen) {
		return
	}

	// Condition on stack - emit if_false to exit loop
	exitPos := len(p.bc.Code)
	p.emitLabel(opcode.OP_if_false, 0) // placeholder

	// Parse body
	p.parseStatement()

	// Emit goto back to condition
	// Fix: capture gotoPos BEFORE emitLabel, then patch
	gotoPos := len(p.bc.Code)         // position BEFORE emit (after body)
	p.emitLabel(opcode.OP_goto, 0)    // emit placeholder
	// After emit: len = gotoPos + 5
	// We want: gotoPos + 5 + offset = loopStart
	// So: offset = loopStart - (gotoPos + 5)
	p.patchLabel(gotoPos, loopStart-gotoPos-5)

	// After goto is emitted, current position is AFTER the goto (gotoPos + 5)
	// This is also where if_false should jump to (exit the loop)
	gotoTarget := len(p.bc.Code) // position AFTER goto

	// Patch exit jump: if_false should jump to gotoTarget
	// if_false at position exitPos, after reading offset: PC = exitPos + 5
	// We want: exitPos + 5 + offset = gotoTarget
	// So: offset = gotoTarget - (exitPos + 5)
	p.patchLabel(exitPos, gotoTarget-(exitPos+5))
}

// parseExpression parses an expression (supports assignment)
func (p *parser) parseExpression() {
	p.parseAssignment()
}

// parseAssignment handles = (lowest precedence)
func (p *parser) parseAssignment() {
	// Check if this is an assignment: ident = expr
	tok := p.peek()
	if tok.Type == lexer.TokenIdent {
		// Look ahead to check if next is =
		savedPos := p.pos
		p.next() // consume identifier
		if p.peek().Type == lexer.TokenAssign {
			p.next() // consume =
			// It's an assignment: ident = expr
			name := tok.Str
			idx := p.registerVar(name)
			p.parseAssignment() // parse right-hand side
			// Value is on stack, now store it
			p.emitU16(opcode.OP_put_var, uint16(idx))
			return
		}
		// Not an assignment, restore position
		p.pos = savedPos
	}
	// Not an assignment, parse comparison
	p.parseComparison()
}

// parseAdditive handles + and - (lower precedence than */%)
func (p *parser) parseAdditive() {
	p.parseMultiplicative()

	for {
		switch p.peek().Type {
		case lexer.TokenPlus:
			p.next()
			p.parseMultiplicative()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_add))
		case lexer.TokenMinus:
			p.next()
			p.parseMultiplicative()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_sub))
		default:
			return
		}
	}
}

// parseComparison handles relational operators
func (p *parser) parseComparison() {
	p.parseAdditive()

	for {
		switch p.peek().Type {
		case lexer.TokenLt:
			p.next()
			p.parseAdditive()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_lt))
		case lexer.TokenLte:
			p.next()
			p.parseAdditive()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_lte))
		case lexer.TokenGt:
			p.next()
			p.parseAdditive()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_gt))
		case lexer.TokenGte:
			p.next()
			p.parseAdditive()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_gte))
		case lexer.TokenEq:
			p.next()
			p.parseAdditive()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_eq))
		case lexer.TokenNeq:
			p.next()
			p.parseAdditive()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_neq))
		default:
			return
		}
	}
}

// parseMultiplicative handles *, /, % (higher precedence than +-)
func (p *parser) parseMultiplicative() {
	p.parseUnary()

	for {
		switch p.peek().Type {
		case lexer.TokenMul:
			p.next()
			p.parseUnary()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_mul))
		case lexer.TokenDiv:
			p.next()
			p.parseUnary()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_div))
		case lexer.TokenMod:
			p.next()
			p.parseUnary()
			p.bc.Code = append(p.bc.Code, byte(opcode.OP_mod))
		default:
			return
		}
	}
}

// parseUnary handles unary operators (not yet implemented)
func (p *parser) parseUnary() {
	p.parsePrimary()
}

// parsePrimary handles primary expressions
func (p *parser) parsePrimary() {
	tok := p.peek()

	switch tok.Type {
	case lexer.TokenNum:
		p.next()
		p.emitPushI32(tok.Value)

	case lexer.TokenTrue:
		p.next()
		p.bc.Code = append(p.bc.Code, byte(opcode.OP_push_true))

	case lexer.TokenFalse:
		p.next()
		p.bc.Code = append(p.bc.Code, byte(opcode.OP_push_false))

	case lexer.TokenUndefined:
		p.next()
		p.bc.Code = append(p.bc.Code, byte(opcode.OP_undefined))

	case lexer.TokenNull:
		p.next()
		p.bc.Code = append(p.bc.Code, byte(opcode.OP_null))

	case lexer.TokenIdent:
		p.next()
		name := tok.Str
		idx := p.registerVar(name)
		p.emitU16(opcode.OP_get_var_undef, uint16(idx))

	case lexer.TokenLeftParen:
		p.next()
		p.parseExpression()
		if !p.expect(lexer.TokenRightParen) {
			// Error - missing closing paren
		}

	default:
		// Unexpected token - push undefined
		p.bc.Code = append(p.bc.Code, byte(opcode.OP_undefined))
	}
}

// emitPushI32 emits push_i32 instruction
func (p *parser) emitPushI32(v int32) {
	p.bc.Code = append(p.bc.Code, byte(opcode.OP_push_i32))
	p.bc.Code = append(p.bc.Code,
		byte(v), byte(v>>8), byte(v>>16), byte(v>>24),
	)
}

// emitU16 emits opcode + u16 operand
func (p *parser) emitU16(op opcode.Opcode, v uint16) {
	p.bc.Code = append(p.bc.Code, byte(op))
	p.bc.Code = append(p.bc.Code, byte(v), byte(v>>8))
}

// emitLabel emits jump instruction with placeholder offset
func (p *parser) emitLabel(op opcode.Opcode, offset int32) {
	p.bc.Code = append(p.bc.Code, byte(op))
	p.bc.Code = append(p.bc.Code,
		byte(offset), byte(offset>>8), byte(offset>>16), byte(offset>>24),
	)
}

// patchLabel patches the offset at position
func (p *parser) patchLabel(pos int, offset int) {
	if pos+1 >= len(p.bc.Code) {
		return
	}
	// pos is where the opcode starts, offset starts at pos+1 (4 bytes)
	p.bc.Code[pos+1] = byte(offset)
	p.bc.Code[pos+2] = byte(offset >> 8)
	p.bc.Code[pos+3] = byte(offset >> 16)
	p.bc.Code[pos+4] = byte(offset >> 24)
}

// emitPushI32 emits a push_i32 instruction with the given value
func emitPushI32(bc *Bytecode, v int32) {
	bc.Code = append(bc.Code, byte(opcode.OP_push_i32))
	bc.Code = append(bc.Code,
		byte(v),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24),
	)
}

// Tokenize is now an alias to the lexer package
// This maintains backward compatibility with the old tokenize() signature
func tokenize(source string) []token {
	return lexer.TokenizeSimple(source)
}