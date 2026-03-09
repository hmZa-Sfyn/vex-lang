package main

// TokenType represents the type of a lexical token
type TokenType int

const (
	// Literals
	TOKEN_NUMBER TokenType = iota
	TOKEN_STRING
	TOKEN_BOOL
	TOKEN_NULL
	TOKEN_IDENT

	// Keywords
	TOKEN_LET
	TOKEN_CONST
	TOKEN_FN
	TOKEN_RETURN
	TOKEN_IF
	TOKEN_ELSE
	TOKEN_WHILE
	TOKEN_FOR
	TOKEN_IN
	TOKEN_BREAK
	TOKEN_CONTINUE
	TOKEN_IMPORT
	TOKEN_FROM
	TOKEN_MATCH
	TOKEN_SPAWN // async goroutine-like
	TOKEN_AWAIT
	TOKEN_YIELD

	// Network keywords
	TOKEN_SERVE   // start HTTP server
	TOKEN_CONNECT // TCP/UDP connect
	TOKEN_LISTEN  // TCP listen
	TOKEN_FETCH   // HTTP fetch
	TOKEN_SEND
	TOKEN_RECV
	TOKEN_CLOSE
	TOKEN_SOCKET
	TOKEN_WS // websocket

	// Operators
	TOKEN_PLUS
	TOKEN_MINUS
	TOKEN_STAR
	TOKEN_SLASH
	TOKEN_PERCENT
	TOKEN_EQ
	TOKEN_NEQ
	TOKEN_LT
	TOKEN_LTE
	TOKEN_GT
	TOKEN_GTE
	TOKEN_AND
	TOKEN_OR
	TOKEN_NOT
	TOKEN_ASSIGN
	TOKEN_PLUS_ASSIGN
	TOKEN_MINUS_ASSIGN
	TOKEN_STAR_ASSIGN
	TOKEN_SLASH_ASSIGN
	TOKEN_ARROW   // ->
	TOKEN_FATARROW // =>
	TOKEN_PIPE    // |>
	TOKEN_SPREAD  // ...
	TOKEN_QUESTION // ?
	TOKEN_COALESCE // ??

	// Delimiters
	TOKEN_LPAREN
	TOKEN_RPAREN
	TOKEN_LBRACE
	TOKEN_RBRACE
	TOKEN_LBRACKET
	TOKEN_RBRACKET
	TOKEN_COMMA
	TOKEN_DOT
	TOKEN_COLON
	TOKEN_SEMICOLON
	TOKEN_NEWLINE

	// Special
	TOKEN_EOF
	TOKEN_ILLEGAL
	TOKEN_COMMENT
)

var tokenNames = map[TokenType]string{
	TOKEN_NUMBER:     "number",
	TOKEN_STRING:     "string",
	TOKEN_BOOL:       "bool",
	TOKEN_NULL:       "null",
	TOKEN_IDENT:      "identifier",
	TOKEN_LET:        "let",
	TOKEN_CONST:      "const",
	TOKEN_FN:         "fn",
	TOKEN_RETURN:     "return",
	TOKEN_IF:         "if",
	TOKEN_ELSE:       "else",
	TOKEN_WHILE:      "while",
	TOKEN_FOR:        "for",
	TOKEN_IN:         "in",
	TOKEN_BREAK:      "break",
	TOKEN_CONTINUE:   "continue",
	TOKEN_IMPORT:     "import",
	TOKEN_FROM:       "from",
	TOKEN_MATCH:      "match",
	TOKEN_SPAWN:      "spawn",
	TOKEN_AWAIT:      "await",
	TOKEN_YIELD:      "yield",
	TOKEN_SERVE:      "serve",
	TOKEN_CONNECT:    "connect",
	TOKEN_LISTEN:     "listen",
	TOKEN_FETCH:      "fetch",
	TOKEN_SEND:       "send",
	TOKEN_RECV:       "recv",
	TOKEN_CLOSE:      "close",
	TOKEN_SOCKET:     "socket",
	TOKEN_WS:         "ws",
	TOKEN_PLUS:       "+",
	TOKEN_MINUS:      "-",
	TOKEN_STAR:       "*",
	TOKEN_SLASH:      "/",
	TOKEN_PERCENT:    "%",
	TOKEN_EQ:         "==",
	TOKEN_NEQ:        "!=",
	TOKEN_LT:         "<",
	TOKEN_LTE:        "<=",
	TOKEN_GT:         ">",
	TOKEN_GTE:        ">=",
	TOKEN_AND:        "&&",
	TOKEN_OR:         "||",
	TOKEN_NOT:        "!",
	TOKEN_ASSIGN:     "=",
	TOKEN_PLUS_ASSIGN:  "+=",
	TOKEN_MINUS_ASSIGN: "-=",
	TOKEN_STAR_ASSIGN:  "*=",
	TOKEN_SLASH_ASSIGN: "/=",
	TOKEN_ARROW:      "->",
	TOKEN_FATARROW:   "=>",
	TOKEN_PIPE:       "|>",
	TOKEN_SPREAD:     "...",
	TOKEN_QUESTION:   "?",
	TOKEN_COALESCE:   "??",
	TOKEN_LPAREN:     "(",
	TOKEN_RPAREN:     ")",
	TOKEN_LBRACE:     "{",
	TOKEN_RBRACE:     "}",
	TOKEN_LBRACKET:   "[",
	TOKEN_RBRACKET:   "]",
	TOKEN_COMMA:      ",",
	TOKEN_DOT:        ".",
	TOKEN_COLON:      ":",
	TOKEN_SEMICOLON:  ";",
	TOKEN_NEWLINE:    "newline",
	TOKEN_EOF:        "EOF",
	TOKEN_ILLEGAL:    "ILLEGAL",
}

func (t TokenType) String() string {
	if s, ok := tokenNames[t]; ok {
		return s
	}
	return "unknown"
}

// Token represents a single lexical token with position info
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
	File    string
}

var keywords = map[string]TokenType{
	"let":      TOKEN_LET,
	"const":    TOKEN_CONST,
	"fn":       TOKEN_FN,
	"return":   TOKEN_RETURN,
	"if":       TOKEN_IF,
	"else":     TOKEN_ELSE,
	"while":    TOKEN_WHILE,
	"for":      TOKEN_FOR,
	"in":       TOKEN_IN,
	"break":    TOKEN_BREAK,
	"continue": TOKEN_CONTINUE,
	"import":   TOKEN_IMPORT,
	"from":     TOKEN_FROM,
	"match":    TOKEN_MATCH,
	"spawn":    TOKEN_SPAWN,
	"await":    TOKEN_AWAIT,
	"yield":    TOKEN_YIELD,
	"true":     TOKEN_BOOL,
	"false":    TOKEN_BOOL,
	"null":     TOKEN_NULL,
	"serve":    TOKEN_SERVE,
	"connect":  TOKEN_CONNECT,
	"listen":   TOKEN_LISTEN,
	"fetch":    TOKEN_FETCH,
	"send":     TOKEN_SEND,
	"recv":     TOKEN_RECV,
	"close":    TOKEN_CLOSE,
	"socket":   TOKEN_SOCKET,
	"ws":       TOKEN_WS,
}
