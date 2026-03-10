package main

// Node is the base AST interface
type Node interface {
	nodeType() string
	Pos() (line, col int)
}

// Statement nodes
type Stmt interface {
	Node
	stmtNode()
}

// Expression nodes
type Expr interface {
	Node
	exprNode()
}

// ---- Position carrier ----
type position struct {
	line, col int
	file      string
}

func (p position) Pos() (int, int) { return p.line, p.col }

// ======== STATEMENTS ========

type Program struct {
	Stmts []Stmt
	File  string
	Lines []string
}
func (p *Program) nodeType() string     { return "Program" }
func (p *Program) Pos() (int, int)      { return 1, 1 }

type LetStmt struct {
	position
	Name    string
	IsConst bool
	Value   Expr
}
func (s *LetStmt) nodeType() string { return "LetStmt" }
func (s *LetStmt) stmtNode()        {}

type ReturnStmt struct {
	position
	Value Expr
}
func (s *ReturnStmt) nodeType() string { return "ReturnStmt" }
func (s *ReturnStmt) stmtNode()        {}

type ExprStmt struct {
	position
	Expr Expr
}
func (s *ExprStmt) nodeType() string { return "ExprStmt" }
func (s *ExprStmt) stmtNode()        {}

type BlockStmt struct {
	position
	Stmts []Stmt
}
func (s *BlockStmt) nodeType() string { return "BlockStmt" }
func (s *BlockStmt) stmtNode()        {}

type IfStmt struct {
	position
	Cond     Expr
	Then     *BlockStmt
	Else     Stmt // *IfStmt or *BlockStmt
}
func (s *IfStmt) nodeType() string { return "IfStmt" }
func (s *IfStmt) stmtNode()        {}

type WhileStmt struct {
	position
	Cond Expr
	Body *BlockStmt
}
func (s *WhileStmt) nodeType() string { return "WhileStmt" }
func (s *WhileStmt) stmtNode()        {}

type ForInStmt struct {
	position
	Key   string
	Value string
	Iter  Expr
	Body  *BlockStmt
}
func (s *ForInStmt) nodeType() string { return "ForInStmt" }
func (s *ForInStmt) stmtNode()        {}

type BreakStmt struct{ position }
func (s *BreakStmt) nodeType() string { return "BreakStmt" }
func (s *BreakStmt) stmtNode()        {}

type ContinueStmt struct{ position }
func (s *ContinueStmt) nodeType() string { return "ContinueStmt" }
func (s *ContinueStmt) stmtNode()        {}

type ImportStmt struct {
	position
	Names  []string
	Module string
}
func (s *ImportStmt) nodeType() string { return "ImportStmt" }
func (s *ImportStmt) stmtNode()        {}

type MatchStmt struct {
	position
	Subject Expr
	Arms    []MatchArm
}
func (s *MatchStmt) nodeType() string { return "MatchStmt" }
func (s *MatchStmt) stmtNode()        {}

type MatchArm struct {
	Pattern Expr // nil = default (_)
	Body    *BlockStmt
}

// Serve statement: serve "0.0.0.0:8080" { ... }
type ServeStmt struct {
	position
	Addr    Expr
	Routes  []*RouteHandler
	Options map[string]Expr
}
func (s *ServeStmt) nodeType() string { return "ServeStmt" }
func (s *ServeStmt) stmtNode()        {}

type RouteHandler struct {
	position
	Method  string // GET, POST, etc or "*"
	Path    Expr
	Handler Expr // fn literal or ident
}

// ======== EXPRESSIONS ========

type NumberLit struct {
	position
	Value float64
	Raw   string
}
func (e *NumberLit) nodeType() string { return "NumberLit" }
func (e *NumberLit) exprNode()        {}

type StringLit struct {
	position
	Value string
}
func (e *StringLit) nodeType() string { return "StringLit" }
func (e *StringLit) exprNode()        {}

type BoolLit struct {
	position
	Value bool
}
func (e *BoolLit) nodeType() string { return "BoolLit" }
func (e *BoolLit) exprNode()        {}

type NullLit struct{ position }
func (e *NullLit) nodeType() string { return "NullLit" }
func (e *NullLit) exprNode()        {}

type Ident struct {
	position
	Name string
}
func (e *Ident) nodeType() string { return "Ident" }
func (e *Ident) exprNode()        {}

type ArrayLit struct {
	position
	Elements []Expr
}
func (e *ArrayLit) nodeType() string { return "ArrayLit" }
func (e *ArrayLit) exprNode()        {}

type ObjectLit struct {
	position
	Keys   []string
	Values []Expr
}
func (e *ObjectLit) nodeType() string { return "ObjectLit" }
func (e *ObjectLit) exprNode()        {}

type FnLit struct {
	position
	Params  []Param
	Body    *BlockStmt
	IsAsync bool
}
func (e *FnLit) nodeType() string { return "FnLit" }
func (e *FnLit) exprNode()        {}

type Param struct {
	Name    string
	Default Expr
}

type CallExpr struct {
	position
	Callee Expr
	Args   []Expr
}
func (e *CallExpr) nodeType() string { return "CallExpr" }
func (e *CallExpr) exprNode()        {}

type IndexExpr struct {
	position
	Object Expr
	Index  Expr
}
func (e *IndexExpr) nodeType() string { return "IndexExpr" }
func (e *IndexExpr) exprNode()        {}

type MemberExpr struct {
	position
	Object Expr
	Key    string
}
func (e *MemberExpr) nodeType() string { return "MemberExpr" }
func (e *MemberExpr) exprNode()        {}

type BinaryExpr struct {
	position
	Op    string
	Left  Expr
	Right Expr
}
func (e *BinaryExpr) nodeType() string { return "BinaryExpr" }
func (e *BinaryExpr) exprNode()        {}

type UnaryExpr struct {
	position
	Op      string
	Operand Expr
}
func (e *UnaryExpr) nodeType() string { return "UnaryExpr" }
func (e *UnaryExpr) exprNode()        {}

type AssignExpr struct {
	position
	Target Expr
	Op     string
	Value  Expr
}
func (e *AssignExpr) nodeType() string { return "AssignExpr" }
func (e *AssignExpr) exprNode()        {}

type PipeExpr struct {
	position
	Left  Expr
	Right Expr // must resolve to callable
}
func (e *PipeExpr) nodeType() string { return "PipeExpr" }
func (e *PipeExpr) exprNode()        {}

type SpawnExpr struct {
	position
	Call Expr
}
func (e *SpawnExpr) nodeType() string { return "SpawnExpr" }
func (e *SpawnExpr) exprNode()        {}

type AwaitExpr struct {
	position
	Value Expr
}
func (e *AwaitExpr) nodeType() string { return "AwaitExpr" }
func (e *AwaitExpr) exprNode()        {}

// Fetch expression: fetch "https://..." { method: "POST", body: data }
type FetchExpr struct {
	position
	URL     Expr
	Options map[string]Expr
}
func (e *FetchExpr) nodeType() string { return "FetchExpr" }
func (e *FetchExpr) exprNode()        {}

// Connect expression: connect "tcp://host:port"
type ConnectExpr struct {
	position
	Protocol string
	Addr     Expr
	Options  map[string]Expr
}
func (e *ConnectExpr) nodeType() string { return "ConnectExpr" }
func (e *ConnectExpr) exprNode()        {}

// Listen expression: listen "tcp://0.0.0.0:9000"
type ListenExpr struct {
	position
	Protocol string
	Addr     Expr
}
func (e *ListenExpr) nodeType() string { return "ListenExpr" }
func (e *ListenExpr) exprNode()        {}

// Send expression: send conn "data"
type SendExpr struct {
	position
	Conn Expr
	Data Expr
}
func (e *SendExpr) nodeType() string { return "SendExpr" }
func (e *SendExpr) exprNode()        {}

// Recv expression: recv conn
type RecvExpr struct {
	position
	Conn Expr
}
func (e *RecvExpr) nodeType() string { return "RecvExpr" }
func (e *RecvExpr) exprNode()        {}

// Ternary: cond ? then : else
type TernaryExpr struct {
	position
	Cond Expr
	Then Expr
	Else Expr
}
func (e *TernaryExpr) nodeType() string { return "TernaryExpr" }
func (e *TernaryExpr) exprNode()        {}

// Try expression: try expr ?? fallback
type TryExpr struct {
	position
	Value    Expr
	Fallback Expr
}
func (e *TryExpr) nodeType() string { return "TryExpr" }
func (e *TryExpr) exprNode()        {}

// Template string: `hello ${name}`
type TemplateLit struct {
	position
	Parts []TemplatePart
}
func (e *TemplateLit) nodeType() string { return "TemplateLit" }
func (e *TemplateLit) exprNode()        {}

type TemplatePart struct {
	IsExpr bool
	Text   string
	Expr   Expr
}

// FnDecl (named function at stmt level)
type FnDecl struct {
	position
	Name    string
	Params  []Param
	Body    *BlockStmt
	IsAsync bool
}
func (s *FnDecl) nodeType() string { return "FnDecl" }
func (s *FnDecl) stmtNode()        {}

// ======== NEW STATEMENTS ========

// TryCatchStmt: try { } catch e { } finally { }
type TryCatchStmt struct {
	position
	Body    *BlockStmt
	CatchVar string
	Catch   *BlockStmt  // nil if no catch
	Finally *BlockStmt  // nil if no finally
}
func (s *TryCatchStmt) nodeType() string { return "TryCatchStmt" }
func (s *TryCatchStmt) stmtNode()        {}

// ThrowStmt: throw expr
type ThrowStmt struct {
	position
	Value Expr
}
func (s *ThrowStmt) nodeType() string { return "ThrowStmt" }
func (s *ThrowStmt) stmtNode()        {}

// DoWhileStmt: do { } while cond
type DoWhileStmt struct {
	position
	Body *BlockStmt
	Cond Expr
}
func (s *DoWhileStmt) nodeType() string { return "DoWhileStmt" }
func (s *DoWhileStmt) stmtNode()        {}

// UnlessStmt: unless cond { } (inverted if)
type UnlessStmt struct {
	position
	Cond Expr
	Body *BlockStmt
	Else Stmt
}
func (s *UnlessStmt) nodeType() string { return "UnlessStmt" }
func (s *UnlessStmt) stmtNode()        {}

// UntilStmt: until cond { } (inverted while)
type UntilStmt struct {
	position
	Cond Expr
	Body *BlockStmt
}
func (s *UntilStmt) nodeType() string { return "UntilStmt" }
func (s *UntilStmt) stmtNode()        {}

// LoopStmt: loop { } (infinite loop)
type LoopStmt struct {
	position
	Body *BlockStmt
}
func (s *LoopStmt) nodeType() string { return "LoopStmt" }
func (s *LoopStmt) stmtNode()        {}

// DeferStmt: defer expr (runs at end of scope)
type DeferStmt struct {
	position
	Call Expr
}
func (s *DeferStmt) nodeType() string { return "DeferStmt" }
func (s *DeferStmt) stmtNode()        {}

// ForFinallyStmt: for ... in ... { } finally { }
type ForFinallyStmt struct {
	position
	Key     string
	Value   string
	Iter    Expr
	Body    *BlockStmt
	Finally *BlockStmt
}
func (s *ForFinallyStmt) nodeType() string { return "ForFinallyStmt" }
func (s *ForFinallyStmt) stmtNode()        {}

// StructDecl: struct Name { field: Type, ... }
type StructDecl struct {
	position
	Name   string
	Fields []StructField
}
func (s *StructDecl) nodeType() string { return "StructDecl" }
func (s *StructDecl) stmtNode()        {}

type StructField struct {
	Name    string
	Default Expr   // optional default value
}

// EnumDecl: enum Name { Variant1, Variant2(val), ... }
type EnumDecl struct {
	position
	Name     string
	Variants []EnumVariant
}
func (s *EnumDecl) nodeType() string { return "EnumDecl" }
func (s *EnumDecl) stmtNode()        {}

type EnumVariant struct {
	Name  string
	Value Expr // optional associated value
}

// ImplBlock: impl StructName { fn method(self) { } }
type ImplBlock struct {
	position
	Target  string
	Methods []*FnDecl
}
func (s *ImplBlock) nodeType() string { return "ImplBlock" }
func (s *ImplBlock) stmtNode()        {}

// TypeAlias: type Name = ...
type TypeAlias struct {
	position
	Name  string
	Value string
}
func (s *TypeAlias) nodeType() string { return "TypeAlias" }
func (s *TypeAlias) stmtNode()        {}

// BgStmt: bg { ... } — background block
type BgStmt struct {
	position
	Body *BlockStmt
}
func (s *BgStmt) nodeType() string { return "BgStmt" }
func (s *BgStmt) stmtNode()        {}

// ======== NEW EXPRESSIONS ========

// ShellExpr: $`ls -la ${path}` or shell "cmd"
type ShellExpr struct {
	position
	Command string // raw command template
	Bg      bool   // run in background
}
func (e *ShellExpr) nodeType() string { return "ShellExpr" }
func (e *ShellExpr) exprNode()        {}

// NewExpr: new StructName { field: val }
type NewExpr struct {
	position
	TypeName string
	Fields   map[string]Expr
	Keys     []string
}
func (e *NewExpr) nodeType() string { return "NewExpr" }
func (e *NewExpr) exprNode()        {}

// IsExpr: value is TypeName
type IsExpr struct {
	position
	Value    Expr
	TypeName string
}
func (e *IsExpr) nodeType() string { return "IsExpr" }
func (e *IsExpr) exprNode()        {}

// AsExpr: value as string
type AsExpr struct {
	position
	Value    Expr
	TypeName string
}
func (e *AsExpr) nodeType() string { return "AsExpr" }
func (e *AsExpr) exprNode()        {}

// RangeExpr: start..end
type RangeExpr struct {
	position
	Start     Expr
	End       Expr
	Inclusive bool // true for ..= 
}
func (e *RangeExpr) nodeType() string { return "RangeExpr" }
func (e *RangeExpr) exprNode()        {}

// SelfExpr: self
type SelfExpr struct{ position }
func (e *SelfExpr) nodeType() string { return "SelfExpr" }
func (e *SelfExpr) exprNode()        {}

