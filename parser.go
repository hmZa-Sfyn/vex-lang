package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser builds an AST from tokens
type Parser struct {
	tokens []Token
	pos    int
	file   string
	lines  []string
	errors []*VexError
}

func NewParser(tokens []Token, file string, src string) *Parser {
	return &Parser{
		tokens: tokens,
		pos:    0,
		file:   file,
		lines:  strings.Split(src, "\n"),
	}
}

func (p *Parser) peek() Token {
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		if t.Type == TOKEN_NEWLINE || t.Type == TOKEN_SEMICOLON {
			return t
		}
		return t
	}
	return Token{Type: TOKEN_EOF}
}

func (p *Parser) peekSkipNL() Token {
	i := p.pos
	for i < len(p.tokens) {
		t := p.tokens[i]
		if t.Type != TOKEN_NEWLINE && t.Type != TOKEN_SEMICOLON {
			return t
		}
		i++
	}
	return Token{Type: TOKEN_EOF}
}

func (p *Parser) advance() Token {
	t := p.tokens[p.pos]
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}

func (p *Parser) skipNL() {
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		if t.Type == TOKEN_NEWLINE || t.Type == TOKEN_SEMICOLON {
			p.pos++
		} else {
			break
		}
	}
}

func (p *Parser) expect(tt TokenType) (Token, bool) {
	p.skipNL()
	t := p.peek()
	if t.Type != tt {
		p.errorf(t, "expected '%s', got '%s'",
			tt.String(), t.Literal)
		return t, false
	}
	return p.advance(), true
}

func (p *Parser) check(tt TokenType) bool {
	return p.peek().Type == tt
}

func (p *Parser) checkSkipNL(tt TokenType) bool {
	return p.peekSkipNL().Type == tt
}

func (p *Parser) match(types ...TokenType) bool {
	t := p.peek()
	for _, tt := range types {
		if t.Type == tt {
			return true
		}
	}
	return false
}

func (p *Parser) eat(tt TokenType) bool {
	if p.check(tt) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) eatSkipNL(tt TokenType) bool {
	if p.checkSkipNL(tt) {
		p.skipNL()
		p.advance()
		return true
	}
	return false
}

func (p *Parser) errorf(t Token, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	err := NewVexError(ERR_SYNTAX, msg, p.file, t.Line, t.Col, p.lines, "", "")
	p.errors = append(p.errors, err)
}

func (p *Parser) errorHint(t Token, msg, hint, fix string) {
	err := NewVexError(ERR_SYNTAX, msg, p.file, t.Line, t.Col, p.lines, hint, fix)
	p.errors = append(p.errors, err)
}

func (p *Parser) pos2() position {
	t := p.peek()
	return position{line: t.Line, col: t.Col, file: p.file}
}

// ===== PARSE ENTRY =====

func (p *Parser) Parse() (*Program, []*VexError) {
	prog := &Program{File: p.file, Lines: p.lines}
	p.skipNL()
	for !p.check(TOKEN_EOF) {
		stmt := p.parseStmt()
		if stmt != nil {
			prog.Stmts = append(prog.Stmts, stmt)
		}
		p.skipNL()
	}
	return prog, p.errors
}

// ===== STATEMENTS =====

func (p *Parser) parseStmt() Stmt {
	p.skipNL()
	t := p.peek()

	switch t.Type {
	case TOKEN_LET, TOKEN_CONST:
		return p.parseLetStmt()
	case TOKEN_FN:
		return p.parseFnDecl()
	case TOKEN_RETURN:
		return p.parseReturn()
	case TOKEN_IF:
		return p.parseIf()
	case TOKEN_WHILE:
		return p.parseWhile()
	case TOKEN_FOR:
		return p.parseFor()
	case TOKEN_BREAK:
		p.advance()
		return &BreakStmt{position: position{t.Line, t.Col, p.file}}
	case TOKEN_CONTINUE:
		p.advance()
		return &ContinueStmt{position: position{t.Line, t.Col, p.file}}
	case TOKEN_IMPORT:
		return p.parseImport()
	case TOKEN_MATCH:
		return p.parseMatch()
	case TOKEN_SERVE:
		return p.parseServe()
	case TOKEN_EOF:
		return nil
	default:
		return p.parseExprStmt()
	}
}

func (p *Parser) parseLetStmt() *LetStmt {
	t := p.advance()
	isConst := t.Type == TOKEN_CONST
	pos := position{t.Line, t.Col, p.file}

	nameT, ok := p.expect(TOKEN_IDENT)
	if !ok {
		return nil
	}

	var val Expr
	if p.eat(TOKEN_ASSIGN) {
		val = p.parseExpr()
	}

	return &LetStmt{position: pos, Name: nameT.Literal, IsConst: isConst, Value: val}
}

func (p *Parser) parseFnDecl() *FnDecl {
	t := p.advance() // eat 'fn'
	pos := position{t.Line, t.Col, p.file}

	isAsync := false
	if p.peek().Literal == "async" {
		p.advance()
		isAsync = true
	}

	nameT, ok := p.expect(TOKEN_IDENT)
	if !ok {
		return nil
	}

	params := p.parseParams()
	body := p.parseBlock()

	return &FnDecl{position: pos, Name: nameT.Literal, Params: params, Body: body, IsAsync: isAsync}
}

func (p *Parser) parseReturn() *ReturnStmt {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}

	var val Expr
	if !p.check(TOKEN_NEWLINE) && !p.check(TOKEN_SEMICOLON) && !p.check(TOKEN_RBRACE) && !p.check(TOKEN_EOF) {
		val = p.parseExpr()
	}
	return &ReturnStmt{position: pos, Value: val}
}

func (p *Parser) parseIf() *IfStmt {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}

	cond := p.parseExpr()
	then := p.parseBlock()

	var els Stmt
	p.skipNL()
	if p.check(TOKEN_ELSE) {
		p.advance()
		p.skipNL()
		if p.check(TOKEN_IF) {
			els = p.parseIf()
		} else {
			els = p.parseBlock()
		}
	}

	return &IfStmt{position: pos, Cond: cond, Then: then, Else: els}
}

func (p *Parser) parseWhile() *WhileStmt {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}
	cond := p.parseExpr()
	body := p.parseBlock()
	return &WhileStmt{position: pos, Cond: cond, Body: body}
}

func (p *Parser) parseFor() *ForInStmt {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}

	// for val in iter  OR  for key, val in iter
	key := ""
	valT, _ := p.expect(TOKEN_IDENT)
	valName := valT.Literal

	if p.eat(TOKEN_COMMA) {
		key = valName
		v2, _ := p.expect(TOKEN_IDENT)
		valName = v2.Literal
	}

	if _, ok := p.expect(TOKEN_IN); !ok {
		p.errorHint(p.peek(), "expected 'in' in for loop", "for loops use 'in' keyword", "use: for item in collection { ... }")
	}

	iter := p.parseExpr()
	body := p.parseBlock()
	return &ForInStmt{position: pos, Key: key, Value: valName, Iter: iter, Body: body}
}

func (p *Parser) parseImport() *ImportStmt {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}

	var names []string
	// import { a, b } from "mod" OR import "mod"
	if p.eat(TOKEN_LBRACE) {
		for !p.check(TOKEN_RBRACE) && !p.check(TOKEN_EOF) {
			nameT, _ := p.expect(TOKEN_IDENT)
			names = append(names, nameT.Literal)
			if !p.eat(TOKEN_COMMA) {
				break
			}
		}
		p.expect(TOKEN_RBRACE)
		p.expect(TOKEN_FROM)
	}

	modT, _ := p.expect(TOKEN_STRING)
	return &ImportStmt{position: pos, Names: names, Module: modT.Literal}
}

func (p *Parser) parseMatch() *MatchStmt {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}

	subject := p.parseExpr()
	p.skipNL()
	p.expect(TOKEN_LBRACE)

	var arms []MatchArm
	p.skipNL()
	for !p.check(TOKEN_RBRACE) && !p.check(TOKEN_EOF) {
		var pattern Expr
		if p.peek().Literal == "_" {
			p.advance()
		} else {
			pattern = p.parseExpr()
		}
		p.skipNL()
		p.expect(TOKEN_FATARROW)
		body := p.parseBlock()
		arms = append(arms, MatchArm{Pattern: pattern, Body: body})
		p.skipNL()
	}
	p.expect(TOKEN_RBRACE)

	return &MatchStmt{position: pos, Subject: subject, Arms: arms}
}

func (p *Parser) parseServe() *ServeStmt {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}

	addr := p.parseExpr()
	p.skipNL()
	p.expect(TOKEN_LBRACE)

	var routes []*RouteHandler
	opts := map[string]Expr{}

	p.skipNL()
	for !p.check(TOKEN_RBRACE) && !p.check(TOKEN_EOF) {
		routeT := p.peek()
		method := strings.ToUpper(routeT.Literal)
		validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "OPTIONS": true, "HEAD": true, "*": true}

		if validMethods[method] || routeT.Literal == "*" {
			p.advance()
			rhPos := position{routeT.Line, routeT.Col, p.file}
			path := p.parseExpr()
			p.skipNL()
			p.expect(TOKEN_FATARROW)
			handler := p.parseExpr()
			routes = append(routes, &RouteHandler{position: rhPos, Method: method, Path: path, Handler: handler})
		} else if routeT.Type == TOKEN_IDENT {
			// option key: value
			p.advance()
			p.expect(TOKEN_COLON)
			val := p.parseExpr()
			opts[routeT.Literal] = val
		} else {
			p.errorf(routeT, "expected HTTP method or option in serve block")
			p.advance()
		}
		p.skipNL()
	}
	p.expect(TOKEN_RBRACE)

	return &ServeStmt{position: pos, Addr: addr, Routes: routes, Options: opts}
}

func (p *Parser) parseExprStmt() *ExprStmt {
	pos := p.pos2()
	expr := p.parseExpr()
	if expr == nil {
		t := p.peek()
		p.errorf(t, "unexpected token '%s'", t.Literal)
		p.advance() // skip bad token
		return nil
	}
	return &ExprStmt{position: pos, Expr: expr}
}

func (p *Parser) parseBlock() *BlockStmt {
	p.skipNL()
	t, ok := p.expect(TOKEN_LBRACE)
	if !ok {
		p.errorHint(p.peek(), "expected '{' to open block", "blocks must be wrapped in braces", "add '{' here")
		return &BlockStmt{position: position{t.Line, t.Col, p.file}}
	}
	pos := position{t.Line, t.Col, p.file}
	var stmts []Stmt
	p.skipNL()
	for !p.check(TOKEN_RBRACE) && !p.check(TOKEN_EOF) {
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		p.skipNL()
	}
	p.expect(TOKEN_RBRACE)
	return &BlockStmt{position: pos, Stmts: stmts}
}

func (p *Parser) parseParams() []Param {
	p.expect(TOKEN_LPAREN)
	var params []Param
	for !p.check(TOKEN_RPAREN) && !p.check(TOKEN_EOF) {
		nameT, _ := p.expect(TOKEN_IDENT)
		param := Param{Name: nameT.Literal}
		if p.eat(TOKEN_ASSIGN) {
			param.Default = p.parseExpr()
		}
		params = append(params, param)
		if !p.eat(TOKEN_COMMA) {
			break
		}
	}
	p.expect(TOKEN_RPAREN)
	return params
}

// ===== EXPRESSIONS (Pratt parser) =====

type prefixParseFn func() Expr
type infixParseFn func(left Expr) Expr

const (
	PREC_NONE      = 0
	PREC_ASSIGN    = 1
	PREC_COALESCE  = 2
	PREC_OR        = 3
	PREC_AND       = 4
	PREC_EQUAL     = 5
	PREC_COMPARE   = 6
	PREC_ADD       = 7
	PREC_MUL       = 8
	PREC_UNARY     = 9
	PREC_CALL      = 10
	PREC_MEMBER    = 11
	PREC_PIPE      = 12
)

func (p *Parser) getPrec(t Token) int {
	switch t.Type {
	case TOKEN_ASSIGN, TOKEN_PLUS_ASSIGN, TOKEN_MINUS_ASSIGN, TOKEN_STAR_ASSIGN, TOKEN_SLASH_ASSIGN:
		return PREC_ASSIGN
	case TOKEN_COALESCE:
		return PREC_COALESCE
	case TOKEN_OR:
		return PREC_OR
	case TOKEN_AND:
		return PREC_AND
	case TOKEN_EQ, TOKEN_NEQ:
		return PREC_EQUAL
	case TOKEN_LT, TOKEN_LTE, TOKEN_GT, TOKEN_GTE:
		return PREC_COMPARE
	case TOKEN_PLUS, TOKEN_MINUS:
		return PREC_ADD
	case TOKEN_STAR, TOKEN_SLASH, TOKEN_PERCENT:
		return PREC_MUL
	case TOKEN_LPAREN:
		return PREC_CALL
	case TOKEN_DOT, TOKEN_LBRACKET:
		return PREC_MEMBER
	case TOKEN_PIPE:
		return PREC_PIPE
	case TOKEN_QUESTION:
		return PREC_ASSIGN // ternary
	}
	return PREC_NONE
}

func (p *Parser) parseExpr() Expr {
	return p.parsePrecedence(PREC_NONE + 1)
}

func (p *Parser) parsePrecedence(minPrec int) Expr {
	left := p.parseUnary()
	if left == nil {
		return nil
	}

	for {
		t := p.peek()
		prec := p.getPrec(t)
		if prec < minPrec {
			break
		}

		switch t.Type {
		case TOKEN_ASSIGN, TOKEN_PLUS_ASSIGN, TOKEN_MINUS_ASSIGN, TOKEN_STAR_ASSIGN, TOKEN_SLASH_ASSIGN:
			p.advance()
			val := p.parsePrecedence(PREC_ASSIGN)
			left = &AssignExpr{position: position{t.Line, t.Col, p.file}, Target: left, Op: t.Literal, Value: val}
		case TOKEN_PIPE:
			p.advance()
			right := p.parsePrecedence(prec + 1)
			left = &PipeExpr{position: position{t.Line, t.Col, p.file}, Left: left, Right: right}
		case TOKEN_QUESTION:
			p.advance()
			then := p.parseExpr()
			p.expect(TOKEN_COLON)
			els := p.parseExpr()
			left = &TernaryExpr{position: position{t.Line, t.Col, p.file}, Cond: left, Then: then, Else: els}
		case TOKEN_COALESCE:
			p.advance()
			right := p.parsePrecedence(PREC_COALESCE + 1)
			left = &BinaryExpr{position: position{t.Line, t.Col, p.file}, Op: "??", Left: left, Right: right}
		case TOKEN_OR, TOKEN_AND,
			TOKEN_EQ, TOKEN_NEQ,
			TOKEN_LT, TOKEN_LTE, TOKEN_GT, TOKEN_GTE,
			TOKEN_PLUS, TOKEN_MINUS,
			TOKEN_STAR, TOKEN_SLASH, TOKEN_PERCENT:
			p.advance()
			right := p.parsePrecedence(prec + 1)
			left = &BinaryExpr{position: position{t.Line, t.Col, p.file}, Op: t.Literal, Left: left, Right: right}
		case TOKEN_DOT:
			p.advance()
			keyT, _ := p.expect(TOKEN_IDENT)
			left = &MemberExpr{position: position{t.Line, t.Col, p.file}, Object: left, Key: keyT.Literal}
		case TOKEN_LBRACKET:
			p.advance()
			idx := p.parseExpr()
			p.expect(TOKEN_RBRACKET)
			left = &IndexExpr{position: position{t.Line, t.Col, p.file}, Object: left, Index: idx}
		case TOKEN_LPAREN:
			left = p.parseCall(left)
		default:
			return left
		}
	}
	return left
}

func (p *Parser) parseCall(callee Expr) *CallExpr {
	t := p.advance() // eat '('
	pos := position{t.Line, t.Col, p.file}
	var args []Expr
	p.skipNL()
	for !p.check(TOKEN_RPAREN) && !p.check(TOKEN_EOF) {
		args = append(args, p.parseExpr())
		if !p.eat(TOKEN_COMMA) {
			break
		}
		p.skipNL()
	}
	p.expect(TOKEN_RPAREN)
	return &CallExpr{position: pos, Callee: callee, Args: args}
}

func (p *Parser) parseUnary() Expr {
	t := p.peek()

	switch t.Type {
	case TOKEN_NOT, TOKEN_MINUS:
		p.advance()
		operand := p.parseUnary()
		return &UnaryExpr{position: position{t.Line, t.Col, p.file}, Op: t.Literal, Operand: operand}
	case TOKEN_SPAWN:
		p.advance()
		call := p.parseExpr()
		return &SpawnExpr{position: position{t.Line, t.Col, p.file}, Call: call}
	case TOKEN_AWAIT:
		p.advance()
		val := p.parseExpr()
		return &AwaitExpr{position: position{t.Line, t.Col, p.file}, Value: val}
	case TOKEN_FETCH:
		return p.parseFetch()
	case TOKEN_CONNECT:
		return p.parseConnect()
	case TOKEN_LISTEN:
		return p.parseListen()
	case TOKEN_SEND:
		return p.parseSend()
	case TOKEN_RECV:
		return p.parseRecv()
	}

	return p.parsePrimary()
}

func (p *Parser) parseFetch() *FetchExpr {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}
	url := p.parsePrimary()
	opts := map[string]Expr{}

	if p.checkSkipNL(TOKEN_LBRACE) {
		p.skipNL()
		p.advance() // eat {
		p.skipNL()
		for !p.check(TOKEN_RBRACE) && !p.check(TOKEN_EOF) {
			keyT, _ := p.expect(TOKEN_IDENT)
			p.expect(TOKEN_COLON)
			val := p.parseExpr()
			opts[keyT.Literal] = val
			p.eat(TOKEN_COMMA)
			p.skipNL()
		}
		p.expect(TOKEN_RBRACE)
	}
	return &FetchExpr{position: pos, URL: url, Options: opts}
}

func (p *Parser) parseConnect() *ConnectExpr {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}
	addr := p.parsePrimary()
	opts := map[string]Expr{}
	proto := "tcp"

	if p.checkSkipNL(TOKEN_LBRACE) {
		p.skipNL()
		p.advance()
		p.skipNL()
		for !p.check(TOKEN_RBRACE) && !p.check(TOKEN_EOF) {
			keyT, _ := p.expect(TOKEN_IDENT)
			p.expect(TOKEN_COLON)
			val := p.parseExpr()
			opts[keyT.Literal] = val
			p.eat(TOKEN_COMMA)
			p.skipNL()
		}
		p.expect(TOKEN_RBRACE)
	}
	return &ConnectExpr{position: pos, Protocol: proto, Addr: addr, Options: opts}
}

func (p *Parser) parseListen() *ListenExpr {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}
	addr := p.parsePrimary()
	return &ListenExpr{position: pos, Addr: addr}
}

func (p *Parser) parseSend() *SendExpr {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}
	conn := p.parsePrimary()
	data := p.parseExpr()
	return &SendExpr{position: pos, Conn: conn, Data: data}
}

func (p *Parser) parseRecv() *RecvExpr {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}
	conn := p.parsePrimary()
	return &RecvExpr{position: pos, Conn: conn}
}

func (p *Parser) parsePrimary() Expr {
	t := p.peek()

	switch t.Type {
	case TOKEN_NUMBER:
		p.advance()
		n, err := strconv.ParseFloat(t.Literal, 64)
		if err != nil {
			p.errorf(t, "invalid number '%s'", t.Literal)
			n = 0
		}
		return &NumberLit{position: position{t.Line, t.Col, p.file}, Value: n, Raw: t.Literal}

	case TOKEN_STRING:
		p.advance()
		// Handle template strings (backtick was lexed as regular string but may contain ${})
		return p.parseTemplateOrString(t)

	case TOKEN_BOOL:
		p.advance()
		return &BoolLit{position: position{t.Line, t.Col, p.file}, Value: t.Literal == "true"}

	case TOKEN_NULL:
		p.advance()
		return &NullLit{position: position{t.Line, t.Col, p.file}}

	case TOKEN_IDENT:
		p.advance()
		return &Ident{position: position{t.Line, t.Col, p.file}, Name: t.Literal}

	case TOKEN_FN:
		return p.parseFnLit()

	case TOKEN_LBRACKET:
		return p.parseArrayLit()

	case TOKEN_LBRACE:
		return p.parseObjectLit()

	case TOKEN_LPAREN:
		p.advance()
		p.skipNL()
		expr := p.parseExpr()
		p.skipNL()
		p.expect(TOKEN_RPAREN)
		return expr

	case TOKEN_SPREAD:
		p.advance()
		val := p.parsePrimary()
		return &UnaryExpr{position: position{t.Line, t.Col, p.file}, Op: "...", Operand: val}
	}

	return nil
}

func (p *Parser) parseTemplateOrString(t Token) Expr {
	// Simple string — no template interpolation in basic strings
	return &StringLit{position: position{t.Line, t.Col, p.file}, Value: t.Literal}
}

func (p *Parser) parseFnLit() *FnLit {
	t := p.advance()
	pos := position{t.Line, t.Col, p.file}
	isAsync := false
	if p.peek().Literal == "async" {
		p.advance()
		isAsync = true
	}
	params := p.parseParams()
	body := p.parseBlock()
	return &FnLit{position: pos, Params: params, Body: body, IsAsync: isAsync}
}

func (p *Parser) parseArrayLit() *ArrayLit {
	t := p.advance() // eat [
	pos := position{t.Line, t.Col, p.file}
	var elems []Expr
	p.skipNL()
	for !p.check(TOKEN_RBRACKET) && !p.check(TOKEN_EOF) {
		elems = append(elems, p.parseExpr())
		if !p.eat(TOKEN_COMMA) {
			break
		}
		p.skipNL()
	}
	p.expect(TOKEN_RBRACKET)
	return &ArrayLit{position: pos, Elements: elems}
}

func (p *Parser) parseObjectLit() *ObjectLit {
	t := p.advance() // eat {
	pos := position{t.Line, t.Col, p.file}
	var keys []string
	var vals []Expr
	p.skipNL()
	for !p.check(TOKEN_RBRACE) && !p.check(TOKEN_EOF) {
		keyT := p.peek()
		if keyT.Type != TOKEN_IDENT && keyT.Type != TOKEN_STRING {
			p.errorf(keyT, "expected object key, got '%s'", keyT.Literal)
			p.advance()
			break
		}
		p.advance()
		keys = append(keys, keyT.Literal)
		p.expect(TOKEN_COLON)
		val := p.parseExpr()
		vals = append(vals, val)
		if !p.eat(TOKEN_COMMA) {
			p.skipNL()
			if p.check(TOKEN_RBRACE) {
				break
			}
		}
		p.skipNL()
	}
	p.expect(TOKEN_RBRACE)
	return &ObjectLit{position: pos, Keys: keys, Values: vals}
}
