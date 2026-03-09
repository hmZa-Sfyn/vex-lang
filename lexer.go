package main

import (
	"fmt"
	"strings"
	"unicode"
)

// Lexer tokenizes vex source code
type Lexer struct {
	src      []rune
	pos      int
	line     int
	col      int
	file     string
	tokens   []Token
	errors   []*VexError
	lines    []string
}

func NewLexer(src, file string) *Lexer {
	lines := strings.Split(src, "\n")
	return &Lexer{
		src:   []rune(src),
		pos:   0,
		line:  1,
		col:   1,
		file:  file,
		lines: lines,
	}
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) peekAt(offset int) rune {
	p := l.pos + offset
	if p >= len(l.src) {
		return 0
	}
	return l.src[p]
}

func (l *Lexer) advance() rune {
	ch := l.src[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) makeToken(t TokenType, lit string, line, col int) Token {
	return Token{Type: t, Literal: lit, Line: line, Col: col, File: l.file}
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.src) {
		ch := l.peek()
		if ch == ' ' || ch == '\t' || ch == '\r' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) readString(quote rune, line, col int) Token {
	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.advance()
		if ch == quote {
			return l.makeToken(TOKEN_STRING, sb.String(), line, col)
		}
		if ch == '\\' {
			if l.pos >= len(l.src) {
				break
			}
			esc := l.advance()
			switch esc {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '\\':
				sb.WriteRune('\\')
			case '"':
				sb.WriteRune('"')
			case '\'':
				sb.WriteRune('\'')
			case '`':
				sb.WriteRune('`')
			default:
				sb.WriteRune('\\')
				sb.WriteRune(esc)
			}
		} else if ch == '\n' && quote != '`' {
			l.errors = append(l.errors, NewVexError(
				ERR_SYNTAX, "unterminated string literal",
				l.file, line, col, l.lines,
				"strings must be closed on the same line",
				fmt.Sprintf("add closing %c before the newline", quote),
			))
			return l.makeToken(TOKEN_STRING, sb.String(), line, col)
		} else {
			sb.WriteRune(ch)
		}
	}
	l.errors = append(l.errors, NewVexError(
		ERR_SYNTAX, "unterminated string literal",
		l.file, line, col, l.lines,
		"reached end of file without closing quote",
		fmt.Sprintf("add a closing %c to end the string", quote),
	))
	return l.makeToken(TOKEN_STRING, sb.String(), line, col)
}

func (l *Lexer) readNumber(line, col int) Token {
	var sb strings.Builder
	isFloat := false
	for l.pos < len(l.src) {
		ch := l.peek()
		if unicode.IsDigit(ch) || ch == '_' {
			if ch != '_' {
				sb.WriteRune(ch)
			}
			l.advance()
		} else if ch == '.' && !isFloat && unicode.IsDigit(l.peekAt(1)) {
			isFloat = true
			sb.WriteRune(ch)
			l.advance()
		} else if (ch == 'e' || ch == 'E') && !isFloat {
			isFloat = true
			sb.WriteRune(ch)
			l.advance()
			if l.peek() == '+' || l.peek() == '-' {
				sb.WriteRune(l.advance())
			}
		} else {
			break
		}
	}
	return l.makeToken(TOKEN_NUMBER, sb.String(), line, col)
}

func (l *Lexer) readIdent(line, col int) Token {
	var sb strings.Builder
	for l.pos < len(l.src) {
		ch := l.peek()
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' {
			sb.WriteRune(ch)
			l.advance()
		} else {
			break
		}
	}
	lit := sb.String()
	if tt, ok := keywords[lit]; ok {
		return l.makeToken(tt, lit, line, col)
	}
	return l.makeToken(TOKEN_IDENT, lit, line, col)
}

func (l *Lexer) readLineComment() {
	for l.pos < len(l.src) && l.peek() != '\n' {
		l.advance()
	}
}

func (l *Lexer) readBlockComment(line, col int) {
	depth := 1
	for l.pos < len(l.src) && depth > 0 {
		if l.peek() == '/' && l.peekAt(1) == '*' {
			l.advance(); l.advance()
			depth++
		} else if l.peek() == '*' && l.peekAt(1) == '/' {
			l.advance(); l.advance()
			depth--
		} else {
			l.advance()
		}
	}
	if depth != 0 {
		l.errors = append(l.errors, NewVexError(
			ERR_SYNTAX, "unclosed block comment",
			l.file, line, col, l.lines,
			"block comments must be closed with */",
			"add */ to close the comment",
		))
	}
}

// Tokenize runs the full lexer and returns all tokens
func (l *Lexer) Tokenize() ([]Token, []*VexError) {
	var tokens []Token

	for l.pos < len(l.src) {
		l.skipWhitespace()
		if l.pos >= len(l.src) {
			break
		}

		ch := l.peek()
		line, col := l.line, l.col

		// Newline - significant
		if ch == '\n' {
			l.advance()
			// Only emit newline if last token was meaningful
			if len(tokens) > 0 {
				last := tokens[len(tokens)-1].Type
				switch last {
				case TOKEN_RPAREN, TOKEN_RBRACE, TOKEN_RBRACKET,
					TOKEN_IDENT, TOKEN_NUMBER, TOKEN_STRING, TOKEN_BOOL, TOKEN_NULL,
					TOKEN_BREAK, TOKEN_CONTINUE, TOKEN_RETURN:
					tokens = append(tokens, l.makeToken(TOKEN_NEWLINE, "\\n", line, col))
				}
			}
			continue
		}

		// Comments
		if ch == '/' && l.peekAt(1) == '/' {
			l.advance(); l.advance()
			l.readLineComment()
			continue
		}
		if ch == '/' && l.peekAt(1) == '*' {
			l.advance(); l.advance()
			l.readBlockComment(line, col)
			continue
		}

		// String literals
		if ch == '"' || ch == '\'' || ch == '`' {
			l.advance()
			tokens = append(tokens, l.readString(ch, line, col))
			continue
		}

		// Numbers
		if unicode.IsDigit(ch) || (ch == '.' && unicode.IsDigit(l.peekAt(1))) {
			tokens = append(tokens, l.readNumber(line, col))
			continue
		}

		// Identifiers & keywords
		if unicode.IsLetter(ch) || ch == '_' {
			tokens = append(tokens, l.readIdent(line, col))
			continue
		}

		// Operators & delimiters
		l.advance()
		switch ch {
		case '+':
			if l.peek() == '=' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_PLUS_ASSIGN, "+=", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_PLUS, "+", line, col)) }
		case '-':
			if l.peek() == '>' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_ARROW, "->", line, col)) } else if l.peek() == '=' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_MINUS_ASSIGN, "-=", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_MINUS, "-", line, col)) }
		case '*':
			if l.peek() == '=' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_STAR_ASSIGN, "*=", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_STAR, "*", line, col)) }
		case '/':
			if l.peek() == '=' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_SLASH_ASSIGN, "/=", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_SLASH, "/", line, col)) }
		case '%':
			tokens = append(tokens, l.makeToken(TOKEN_PERCENT, "%", line, col))
		case '=':
			if l.peek() == '=' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_EQ, "==", line, col)) } else if l.peek() == '>' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_FATARROW, "=>", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_ASSIGN, "=", line, col)) }
		case '!':
			if l.peek() == '=' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_NEQ, "!=", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_NOT, "!", line, col)) }
		case '<':
			if l.peek() == '=' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_LTE, "<=", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_LT, "<", line, col)) }
		case '>':
			if l.peek() == '=' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_GTE, ">=", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_GT, ">", line, col)) }
		case '&':
			if l.peek() == '&' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_AND, "&&", line, col)) } else {
				l.errors = append(l.errors, NewVexError(ERR_SYNTAX, "unexpected character '&'", l.file, line, col, l.lines, "single '&' is not valid in vex", "did you mean '&&' for logical and?"))
			}
		case '|':
			if l.peek() == '|' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_OR, "||", line, col)) } else if l.peek() == '>' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_PIPE, "|>", line, col)) } else {
				l.errors = append(l.errors, NewVexError(ERR_SYNTAX, "unexpected character '|'", l.file, line, col, l.lines, "single '|' is not valid", "use '||' for logical or, or '|>' for pipe"))
			}
		case '?':
			if l.peek() == '?' { l.advance(); tokens = append(tokens, l.makeToken(TOKEN_COALESCE, "??", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_QUESTION, "?", line, col)) }
		case '.':
			if l.peek() == '.' && l.peekAt(1) == '.' { l.advance(); l.advance(); tokens = append(tokens, l.makeToken(TOKEN_SPREAD, "...", line, col)) } else { tokens = append(tokens, l.makeToken(TOKEN_DOT, ".", line, col)) }
		case '(':  tokens = append(tokens, l.makeToken(TOKEN_LPAREN, "(", line, col))
		case ')':  tokens = append(tokens, l.makeToken(TOKEN_RPAREN, ")", line, col))
		case '{':  tokens = append(tokens, l.makeToken(TOKEN_LBRACE, "{", line, col))
		case '}':  tokens = append(tokens, l.makeToken(TOKEN_RBRACE, "}", line, col))
		case '[':  tokens = append(tokens, l.makeToken(TOKEN_LBRACKET, "[", line, col))
		case ']':  tokens = append(tokens, l.makeToken(TOKEN_RBRACKET, "]", line, col))
		case ',':  tokens = append(tokens, l.makeToken(TOKEN_COMMA, ",", line, col))
		case ':':  tokens = append(tokens, l.makeToken(TOKEN_COLON, ":", line, col))
		case ';':  tokens = append(tokens, l.makeToken(TOKEN_SEMICOLON, ";", line, col))
		default:
			l.errors = append(l.errors, NewVexError(
				ERR_SYNTAX,
				fmt.Sprintf("unexpected character '%c'", ch),
				l.file, line, col, l.lines,
				"this character is not valid in vex",
				"remove or replace this character",
			))
			tokens = append(tokens, l.makeToken(TOKEN_ILLEGAL, string(ch), line, col))
		}
	}

	tokens = append(tokens, l.makeToken(TOKEN_EOF, "", l.line, l.col))
	l.tokens = tokens
	return tokens, l.errors
}
