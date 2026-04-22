package core

import (
	"github.com/akzj/go-quickjs/pkg/opcode"
	"github.com/akzj/go-quickjs/pkg/value"
)

// CompileAndRun compiles JavaScript source and executes it
func (ctx *JSContext) CompileAndRun(source string) value.JSValue {
	bc := simpleCompile(source, ctx)
	if bc == nil {
		return value.Undefined()
	}
	return ctx.RunBytecode(bc)
}

// simpleCompile compiles numeric expressions to bytecode
// Uses a two-phase approach:
// 1. Parse tokens into operand/operator list
// 2. Emit bytecode: for each operand, push; for each operator, emit op
//
// "1 + 1" -> [num(1), op(+), num(1)] -> [push(1), push(1), add] -> return
func simpleCompile(source string, ctx *JSContext) *Bytecode {
	tokens := tokenize(source)
	if tokens == nil || len(tokens) == 0 {
		return nil
	}

	bc := &Bytecode{
		Code:    make([]byte, 0, 64),
		Pool:    make([]value.JSValue, 0, 8),
		VarCount: 0,
	}

	// Separate numbers and operators
	var operands []int32
	var operators []tokenType

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch tok.typ {
		case tokenNum:
			operands = append(operands, tok.value)
		case tokenPlus, tokenMinus, tokenMul, tokenDiv, tokenMod:
			operators = append(operators, tok.typ)
		case tokenTrue, tokenFalse, tokenUndefined, tokenNull:
			// Literals directly emit to bytecode
			bc.Code = append(bc.Code, byte(tok.typ.toOpcode()))
		case tokenLeftParen, tokenRightParen:
			// Skip for now
		case tokenEOF:
			// Done
		}
	}

	// Emit bytecode:
	// 1. Push all operands first (left to right)
	// 2. Then emit all operators (left to right)
	// For "1 + 2 + 3":
	//   operands = [1, 2, 3], operators = [+, +]
	//   bytecode: push(1), push(2), push(3), add, add
	//   trace: [1] -> [1,2] -> [1,2,3] -> [3] -> [6] ✓
	
	// First: push all operands
	for _, v := range operands {
		emitPushI32(bc, v)
	}
	
	// Then: emit all operators
	for _, op := range operators {
		bc.Code = append(bc.Code, byte(op.toOpcode()))
	}

	// Add return
	bc.Code = append(bc.Code, byte(opcode.OP_return))

	return bc
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

// tokenTypeToOpcode converts token type to opcode
func (t tokenType) toOpcode() opcode.Opcode {
	switch t {
	case tokenPlus:
		return opcode.OP_add
	case tokenMinus:
		return opcode.OP_sub
	case tokenMul:
		return opcode.OP_mul
	case tokenDiv:
		return opcode.OP_div
	case tokenMod:
		return opcode.OP_mod
	case tokenTrue:
		return opcode.OP_push_true
	case tokenFalse:
		return opcode.OP_push_false
	case tokenUndefined:
		return opcode.OP_undefined
	case tokenNull:
		return opcode.OP_null
	default:
		return opcode.OP_invalid
	}
}

// Token types
type tokenType int

const (
	tokenNum tokenType = iota
	tokenPlus
	tokenMinus
	tokenMul
	tokenDiv
	tokenMod
	tokenTrue
	tokenFalse
	tokenUndefined
	tokenNull
	tokenLeftParen
	tokenRightParen
	tokenEOF
)

type token struct {
	typ   tokenType
	value int32
}

// tokenize parses source into tokens
func tokenize(source string) []token {
	tokens := make([]token, 0, len(source))

	i := 0
	for i < len(source) {
		c := source[i]

		// Skip whitespace
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			i++
			continue
		}

		// Numbers
		if c >= '0' && c <= '9' {
			start := i
			for i < len(source) && source[i] >= '0' && source[i] <= '9' {
				i++
			}
			numStr := source[start:i]
			var n int32
			for _, d := range numStr {
				n = n*10 + int32(d-'0')
			}
			tokens = append(tokens, token{typ: tokenNum, value: n})
			continue
		}

		// Single char operators
		switch c {
		case '+':
			tokens = append(tokens, token{typ: tokenPlus})
		case '-':
			tokens = append(tokens, token{typ: tokenMinus})
		case '*':
			tokens = append(tokens, token{typ: tokenMul})
		case '/':
			tokens = append(tokens, token{typ: tokenDiv})
		case '%':
			tokens = append(tokens, token{typ: tokenMod})
		case '(':
			tokens = append(tokens, token{typ: tokenLeftParen})
		case ')':
			tokens = append(tokens, token{typ: tokenRightParen})
		default:
			// Keywords
			remaining := source[i:]
			if len(remaining) >= 4 && remaining[:4] == "true" {
				tokens = append(tokens, token{typ: tokenTrue})
				i += 4
				continue
			}
			if len(remaining) >= 5 && remaining[:5] == "false" {
				tokens = append(tokens, token{typ: tokenFalse})
				i += 5
				continue
			}
			if len(remaining) >= 9 && remaining[:9] == "undefined" {
				tokens = append(tokens, token{typ: tokenUndefined})
				i += 9
				continue
			}
			if len(remaining) >= 4 && remaining[:4] == "null" {
				tokens = append(tokens, token{typ: tokenNull})
				i += 4
				continue
			}
		}
		i++
	}

	tokens = append(tokens, token{typ: tokenEOF})
	return tokens
}