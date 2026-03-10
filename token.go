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

	// New control flow
	TOKEN_TRY
	TOKEN_CATCH
	TOKEN_FINALLY
	TOKEN_THROW
	TOKEN_DO
	TOKEN_UNLESS  // unless cond { } (inverted if)
	TOKEN_UNTIL   // until cond { } (inverted while)
	TOKEN_LOOP    // infinite loop { }
	TOKEN_DEFER   // defer fn call

	// Type system
	TOKEN_STRUCT
	TOKEN_ENUM
	TOKEN_TYPE
	TOKEN_IMPL   // impl StructName { fn ... }
	TOKEN_SELF   // self reference inside impl
	TOKEN_NEW    // new StructName { ... }
	TOKEN_IS     // type check: x is TypeName
	TOKEN_AS     // type cast: x as string

	// Shell / process
	TOKEN_SHELL  // shell "cmd"  or  $ backtick
	TOKEN_BG     // bg { ... }  background block
	TOKEN_PROC   // proc keyword

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
	TOKEN_DOUBLE_STAR // **  (power)
	TOKEN_AMP         // &   (bitwise and / address-of)
	TOKEN_CARET       // ^   (bitwise xor)
	TOKEN_TILDE       // ~   (bitwise not)
	TOKEN_LSHIFT      // <<
	TOKEN_RSHIFT      // >>
	TOKEN_RANGE       // ..  (range operator)
	TOKEN_HASH        // #!  shell line / shebang
	TOKEN_AT          // @   decorator

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
	TOKEN_TRY:        "try",
	TOKEN_CATCH:      "catch",
	TOKEN_FINALLY:    "finally",
	TOKEN_THROW:      "throw",
	TOKEN_DO:         "do",
	TOKEN_UNLESS:     "unless",
	TOKEN_UNTIL:      "until",
	TOKEN_LOOP:       "loop",
	TOKEN_DEFER:      "defer",
	TOKEN_STRUCT:     "struct",
	TOKEN_ENUM:       "enum",
	TOKEN_TYPE:       "type",
	TOKEN_IMPL:       "impl",
	TOKEN_SELF:       "self",
	TOKEN_NEW:        "new",
	TOKEN_IS:         "is",
	TOKEN_AS:         "as",
	TOKEN_SHELL:      "shell",
	TOKEN_BG:         "bg",
	TOKEN_PROC:       "proc",
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
	TOKEN_DOUBLE_STAR: "**",
	TOKEN_AMP:        "&",
	TOKEN_CARET:      "^",
	TOKEN_TILDE:      "~",
	TOKEN_LSHIFT:     "<<",
	TOKEN_RSHIFT:     ">>",
	TOKEN_RANGE:      "..",
	TOKEN_AT:         "@",
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
	"try":      TOKEN_TRY,
	"catch":    TOKEN_CATCH,
	"finally":  TOKEN_FINALLY,
	"throw":    TOKEN_THROW,
	"do":       TOKEN_DO,
	"unless":   TOKEN_UNLESS,
	"until":    TOKEN_UNTIL,
	"loop":     TOKEN_LOOP,
	"defer":    TOKEN_DEFER,
	"struct":   TOKEN_STRUCT,
	"enum":     TOKEN_ENUM,
	"type":     TOKEN_TYPE,
	"impl":     TOKEN_IMPL,
	"self":     TOKEN_SELF,
	"new":      TOKEN_NEW,
	"is":       TOKEN_IS,
	"as":       TOKEN_AS,
	"shell":    TOKEN_SHELL,
	"bg":       TOKEN_BG,
	"proc":     TOKEN_PROC,
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
