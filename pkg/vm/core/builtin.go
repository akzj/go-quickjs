package core

import (
	"github.com/akzj/go-quickjs/pkg/compiler"
	"github.com/akzj/go-quickjs/pkg/lexer"
	"github.com/akzj/go-quickjs/pkg/value"
)

// tokenType is an alias for lexer.TokenType for backward compatibility
type tokenType = lexer.TokenType

// token is an alias for lexer.Token for backward compatibility
type token = lexer.Token

// CompileAndRun compiles JavaScript source and executes it
// Delegates to the compiler package
func (ctx *JSContext) CompileAndRun(source string) value.JSValue {
	cb := compiler.SimpleCompile(source)
	if cb == nil {
		return value.Undefined()
	}
	// Convert compiler.Bytecode to core.Bytecode for VM execution
	// Convert Pool entries (compiler.FunctionInfo -> core.FunctionInfo)
	pool := make([]value.JSValue, len(cb.Pool))
	for i, v := range cb.Pool {
		if fv, ok := v.(value.FunctionValue); ok {
			info := fv.Info()
			if cfi, ok := info.(*compiler.FunctionInfo); ok {
				pool[i] = value.MakeFunction(FromCompiler(cfi))
			} else {
				pool[i] = v
			}
		} else {
			pool[i] = v
		}
	}
	bc := &Bytecode{
		Code:     cb.Code,
		Pool:     pool,
		VarCount: cb.VarCount,
		VarNames: cb.VarNames,
		ArgCount: cb.ArgCount,
	}
	return ctx.RunBytecode(bc)
}

// tokenize is kept for backward compatibility - delegates to lexer package
func tokenize(source string) []token {
	return lexer.TokenizeSimple(source)
}