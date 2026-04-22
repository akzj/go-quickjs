package lexer

// TokenType represents a JavaScript token type
type TokenType int

const (
	// Literals
	TokenNum TokenType = iota
	TokenString
	TokenTemplate

	// Identifiers
	TokenIdent

	// Keywords
	TokenTrue
	TokenFalse
	TokenNull
	TokenUndefined
	TokenVar
	TokenLet
	TokenConst
	TokenIf
	TokenElse
	TokenWhile
	TokenFor
	TokenFunction
	TokenReturn
	TokenBreak
	TokenContinue
	TokenSwitch
	TokenCase
	TokenDefault
	TokenThrow
	TokenTry
	TokenCatch
	TokenFinally
	TokenIn
	TokenInstanceOf
	TokenNew
	TokenDelete
	TokenTypeOf
	TokenVoid
	TokenThis

	// Operators
	TokenPlus
	TokenMinus
	TokenMul
	TokenDiv
	TokenMod
	TokenAssign         // =
	TokenEq             // ==
	TokenNeq            // !=
	TokenStrictEq       // ===
	TokenStrictNeq      // !==
	TokenLt             // <
	TokenLte            // <=
	TokenGt             // >
	TokenGte            // >=
	TokenBang           // !
	TokenAnd            // &&
	TokenOr             // ||
	TokenPlusPlus       // ++
	TokenMinusMinus     // --

	// Delimiters
	TokenLeftParen      // (
	TokenRightParen     // )
	TokenLeftBrace      // {
	TokenRightBrace     // }
	TokenLeftBracket    // [
	TokenRightBracket   // ]
	TokenSemicolon      // ;
	TokenColon          // :
	TokenComma          // ,
	TokenDot            // .
	TokenArrow          // =>
	TokenEllipsis       // ...

	// Special
	TokenEof
	TokenError
)

// Token represents a JavaScript token
type Token struct {
	Type   TokenType
	Value  int32 // for numbers
	Str    string // for identifiers, strings
	Line   int    // source line
	Column int    // source column
}
