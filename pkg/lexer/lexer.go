package lexer

import (
	"strings"
)

// Lexer performs lexical analysis on JavaScript source code
type Lexer struct {
	source string
	pos    int
	line   int
	column int
}

// NewLexer creates a new lexer for the given source
func NewLexer(source string) *Lexer {
	return &Lexer{
		source: source,
		pos:    0,
		line:   1,
		column: 1,
	}
}

// NextToken returns the next token from the source
func (l *Lexer) NextToken() Token {
	// Skip whitespace
	for l.pos < len(l.source) && isWhitespace(l.source[l.pos]) {
		l.advance()
	}

	if l.pos >= len(l.source) {
		return Token{Type: TokenEof, Line: l.line, Column: l.column}
	}

	c := l.source[l.pos]
	startLine := l.line
	startColumn := l.column

	// Numbers
	if c >= '0' && c <= '9' {
		return l.readNumber(startLine, startColumn)
	}

	// Identifiers and keywords
	if isIdentStart(c) {
		return l.readIdent(startLine, startColumn)
	}

	// String literals
	if c == '"' || c == '\'' {
		return l.readString(startLine, startColumn)
	}

	// Operators and delimiters
	return l.readOperator(startLine, startColumn)
}

func (l *Lexer) advance() {
	if l.pos < len(l.source) {
		if l.source[l.pos] == '\n' {
			l.line++
			l.column = 1
		} else {
			l.column++
		}
		l.pos++
	}
}

func (l *Lexer) peek() byte {
	if l.pos < len(l.source) {
		return l.source[l.pos]
	}
	return 0
}

func (l *Lexer) peekNext() byte {
	if l.pos+1 < len(l.source) {
		return l.source[l.pos+1]
	}
	return 0
}

// isWhitespace checks if c is whitespace
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isIdentStart checks if c can start an identifier
func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '$'
}

// isIdentPart checks if c can be part of an identifier
func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func (l *Lexer) readNumber(line, column int) Token {
	start := l.pos
	var n int32
	for l.pos < len(l.source) && l.source[l.pos] >= '0' && l.source[l.pos] <= '9' {
		l.advance()
	}
	numStr := l.source[start:l.pos]
	for _, d := range numStr {
		n = n*10 + int32(d-'0')
	}
	return Token{Type: TokenNum, Value: n, Line: line, Column: column}
}

func (l *Lexer) readIdent(line, column int) Token {
	start := l.pos
	for l.pos < len(l.source) && isIdentPart(l.source[l.pos]) {
		l.advance()
	}
	ident := l.source[start:l.pos]

	// Keyword lookup
	tt := lookupKeyword(ident)
	if tt != TokenError {
		return Token{Type: tt, Line: line, Column: column}
	}

	return Token{Type: TokenIdent, Str: ident, Line: line, Column: column}
}

func (l *Lexer) readString(line, column int) Token {
	quote := l.source[l.pos]
	l.advance() // skip opening quote

	start := l.pos
	for l.pos < len(l.source) && l.source[l.pos] != quote {
		if l.source[l.pos] == '\\' && l.pos+1 < len(l.source) {
			l.advance() // skip escape
		}
		l.advance()
	}

	strVal := l.source[start:l.pos]
	if l.pos < len(l.source) {
		l.advance() // skip closing quote
	}

	return Token{Type: TokenString, Str: strVal, Line: line, Column: column}
}

func (l *Lexer) readOperator(line, column int) Token {
	c := l.source[l.pos]
	l.advance()

	switch c {
	case '+':
		if l.peek() == '+' {
			l.advance()
			return Token{Type: TokenPlusPlus, Line: line, Column: column}
		}
		return Token{Type: TokenPlus, Line: line, Column: column}
	case '-':
		if l.peek() == '-' {
			l.advance()
			return Token{Type: TokenMinusMinus, Line: line, Column: column}
		}
		return Token{Type: TokenMinus, Line: line, Column: column}
	case '*':
		return Token{Type: TokenMul, Line: line, Column: column}
	case '/':
		return Token{Type: TokenDiv, Line: line, Column: column}
	case '%':
		return Token{Type: TokenMod, Line: line, Column: column}
	case '(':
		return Token{Type: TokenLeftParen, Line: line, Column: column}
	case ')':
		return Token{Type: TokenRightParen, Line: line, Column: column}
	case '{':
		return Token{Type: TokenLeftBrace, Line: line, Column: column}
	case '}':
		return Token{Type: TokenRightBrace, Line: line, Column: column}
	case '[':
		return Token{Type: TokenLeftBracket, Line: line, Column: column}
	case ']':
		return Token{Type: TokenRightBracket, Line: line, Column: column}
	case ';':
		return Token{Type: TokenSemicolon, Line: line, Column: column}
	case ',':
		return Token{Type: TokenComma, Line: line, Column: column}
	case '.':
		if l.peek() == '.' && l.peekNext() == '.' {
			l.advance()
			l.advance()
			return Token{Type: TokenEllipsis, Line: line, Column: column}
		}
		return Token{Type: TokenDot, Line: line, Column: column}
	case ':':
		return Token{Type: TokenColon, Line: line, Column: column}
	case '=':
		if l.peek() == '=' {
			l.advance()
			if l.peek() == '=' {
				l.advance()
				return Token{Type: TokenStrictEq, Line: line, Column: column}
			}
			return Token{Type: TokenEq, Line: line, Column: column}
		}
		if l.peek() == '>' {
			l.advance()
			return Token{Type: TokenArrow, Line: line, Column: column}
		}
		return Token{Type: TokenAssign, Line: line, Column: column}
	case '!':
		if l.peek() == '=' {
			l.advance()
			if l.peek() == '=' {
				l.advance()
				return Token{Type: TokenStrictNeq, Line: line, Column: column}
			}
			return Token{Type: TokenNeq, Line: line, Column: column}
		}
		return Token{Type: TokenBang, Line: line, Column: column}
	case '<':
		if l.peek() == '=' {
			l.advance()
			return Token{Type: TokenLte, Line: line, Column: column}
		}
		return Token{Type: TokenLt, Line: line, Column: column}
	case '>':
		if l.peek() == '=' {
			l.advance()
			return Token{Type: TokenGte, Line: line, Column: column}
		}
		return Token{Type: TokenGt, Line: line, Column: column}
	case '&':
		if l.peek() == '&' {
			l.advance()
			return Token{Type: TokenAnd, Line: line, Column: column}
		}
		return Token{Type: TokenError, Line: line, Column: column}
	case '|':
		if l.peek() == '|' {
			l.advance()
			return Token{Type: TokenOr, Line: line, Column: column}
		}
		return Token{Type: TokenError, Line: line, Column: column}
	}

	return Token{Type: TokenError, Line: line, Column: column}
}

// lookupKeyword returns the token type for a keyword, or TokenError if not a keyword
func lookupKeyword(s string) TokenType {
	switch s {
	case "true":
		return TokenTrue
	case "false":
		return TokenFalse
	case "null":
		return TokenNull
	case "undefined":
		return TokenUndefined
	case "var":
		return TokenVar
	case "let":
		return TokenLet
	case "const":
		return TokenConst
	case "if":
		return TokenIf
	case "else":
		return TokenElse
	case "while":
		return TokenWhile
	case "for":
		return TokenFor
	case "function":
		return TokenFunction
	case "return":
		return TokenReturn
	case "break":
		return TokenBreak
	case "continue":
		return TokenContinue
	case "switch":
		return TokenSwitch
	case "case":
		return TokenCase
	case "default":
		return TokenDefault
	case "throw":
		return TokenThrow
	case "try":
		return TokenTry
	case "catch":
		return TokenCatch
	case "finally":
		return TokenFinally
	case "in":
		return TokenIn
	case "instanceof":
		return TokenInstanceOf
	case "new":
		return TokenNew
	case "delete":
		return TokenDelete
	case "typeof":
		return TokenTypeOf
	case "void":
		return TokenVoid
	case "this":
		return TokenThis
	default:
		return TokenError
	}
}

// Tokenize is a convenience function that returns all tokens from source
func Tokenize(source string) []Token {
	l := NewLexer(source)
	tokens := make([]Token, 0)
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TokenEof || tok.Type == TokenError {
			break
		}
	}
	return tokens
}

// TokenizeStrings is a helper to tokenize multiple sources (for backwards compat)
func TokenizeStrings(sources ...string) [][]Token {
	result := make([][]Token, len(sources))
	for i, s := range sources {
		result[i] = Tokenize(s)
	}
	return result
}

// TokenizeSimple is the old-style tokenizer compatible with builtin.go's tokenize
// It returns a slice of tokens with minimal info (no line/column)
func TokenizeSimple(source string) []Token {
	l := NewLexer(source)
	tokens := make([]Token, 0)
	for {
		tok := l.NextToken()
		// Return without line/column for compatibility
		tokens = append(tokens, tok)
		if tok.Type == TokenEof {
			break
		}
	}
	return tokens
}

// TokenToOldFormat converts a lexer.Token to the old token format used by builtin.go
// This is for gradual migration - the old format used token{typ, value, name}
func (t Token) TokenToOldFormat() (TokenType, int32, string) {
	return t.Type, t.Value, t.Str
}

// TokenFromOldFormat creates a lexer.Token from old format
func TokenFromOldFormat(typ TokenType, value int32, name string) Token {
	return Token{Type: typ, Value: value, Str: name}
}

// String returns a string representation of the token type
func (t TokenType) String() string {
	switch t {
	case TokenNum:
		return "NUM"
	case TokenString:
		return "STRING"
	case TokenTemplate:
		return "TEMPLATE"
	case TokenIdent:
		return "IDENT"
	case TokenTrue:
		return "TRUE"
	case TokenFalse:
		return "FALSE"
	case TokenNull:
		return "NULL"
	case TokenUndefined:
		return "UNDEFINED"
	case TokenVar:
		return "VAR"
	case TokenLet:
		return "LET"
	case TokenConst:
		return "CONST"
	case TokenIf:
		return "IF"
	case TokenElse:
		return "ELSE"
	case TokenWhile:
		return "WHILE"
	case TokenFor:
		return "FOR"
	case TokenFunction:
		return "FUNCTION"
	case TokenReturn:
		return "RETURN"
	case TokenBreak:
		return "BREAK"
	case TokenContinue:
		return "CONTINUE"
	case TokenSwitch:
		return "SWITCH"
	case TokenCase:
		return "CASE"
	case TokenDefault:
		return "DEFAULT"
	case TokenThrow:
		return "THROW"
	case TokenTry:
		return "TRY"
	case TokenCatch:
		return "CATCH"
	case TokenFinally:
		return "FINALLY"
	case TokenIn:
		return "IN"
	case TokenInstanceOf:
		return "INSTANCEOF"
	case TokenNew:
		return "NEW"
	case TokenDelete:
		return "DELETE"
	case TokenTypeOf:
		return "TYPEOF"
	case TokenVoid:
		return "VOID"
	case TokenThis:
		return "THIS"
	case TokenPlus:
		return "PLUS"
	case TokenMinus:
		return "MINUS"
	case TokenMul:
		return "MUL"
	case TokenDiv:
		return "DIV"
	case TokenMod:
		return "MOD"
	case TokenAssign:
		return "ASSIGN"
	case TokenEq:
		return "EQ"
	case TokenNeq:
		return "NEQ"
	case TokenStrictEq:
		return "STRICT_EQ"
	case TokenStrictNeq:
		return "STRICT_NEQ"
	case TokenLt:
		return "LT"
	case TokenLte:
		return "LTE"
	case TokenGt:
		return "GT"
	case TokenGte:
		return "GTE"
	case TokenBang:
		return "BANG"
	case TokenAnd:
		return "AND"
	case TokenOr:
		return "OR"
	case TokenPlusPlus:
		return "PLUS_PLUS"
	case TokenMinusMinus:
		return "MINUS_MINUS"
	case TokenLeftParen:
		return "LEFT_PAREN"
	case TokenRightParen:
		return "RIGHT_PAREN"
	case TokenLeftBrace:
		return "LEFT_BRACE"
	case TokenRightBrace:
		return "RIGHT_BRACE"
	case TokenLeftBracket:
		return "LEFT_BRACKET"
	case TokenRightBracket:
		return "RIGHT_BRACKET"
	case TokenSemicolon:
		return "SEMICOLON"
	case TokenColon:
		return "COLON"
	case TokenComma:
		return "COMMA"
	case TokenDot:
		return "DOT"
	case TokenArrow:
		return "ARROW"
	case TokenEllipsis:
		return "ELLIPSIS"
	case TokenEof:
		return "EOF"
	case TokenError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// String implements fmt.Stringer for Token
func (t Token) String() string {
	var b strings.Builder
	b.WriteString(t.Type.String())
	if t.Str != "" {
		b.WriteString("(")
		b.WriteString(t.Str)
		b.WriteString(")")
	} else if t.Type == TokenNum {
		b.WriteString("(")
		b.WriteString(string(rune(t.Value + '0'))) // simplified
		b.WriteString(")")
	}
	return b.String()
}
