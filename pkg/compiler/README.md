# Compiler Package

This package provides JavaScript source code compilation to bytecode.

## Overview

The compiler transforms JavaScript source code (as tokens from `pkg/lexer`) into bytecode that can be executed by the VM in `pkg/vm/core`.

## Usage

```go
import "github.com/akzj/go-quickjs/pkg/compiler"

// Simple compilation
bc := compiler.SimpleCompile("var x = 42; x + 1")

// Or with a Compiler instance
tokens := lexer.TokenizeSimple(source)
c := compiler.NewCompiler(tokens)
bc := c.Compile()
```

## Features

- **Expressions**: Numbers, strings, identifiers, operators (+, -, *, /, %, <, >, ==, !=)
- **Statements**: var/let declarations, if/else, while loops, blocks
- **Variables**: Auto-registration, assignment
- **Control Flow**: if/else statements, while loops

## Architecture

```
Source Code → Lexer → Tokens → Compiler → Bytecode → VM → Result
```

## Package Structure

- `compiler.go` - Main compiler implementation
- `compiler_test.go` - Unit tests

## Bytecode Format

The compiler produces bytecode for the VM defined in `pkg/vm/core/`. The `Bytecode` struct contains:
- `Code` - instruction bytes
- `Pool` - constant pool
- `VarNames` - variable name list
- `VarCount` - number of local variables
- `ArgCount` - number of arguments