package main

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Interpreter executes a vex AST
type Interpreter struct {
	globals *Env
	file    string
	lines   []string
	mu      sync.Mutex
}

func NewInterpreter(parent *Env) *Interpreter {
	interp := &Interpreter{}
	if parent == nil {
		interp.globals = NewEnv(nil)
		interp.registerBuiltins()
	} else {
		interp.globals = parent
	}
	return interp
}

func (interp *Interpreter) Run(prog *Program) error {
	interp.file = prog.File
	interp.lines = prog.Lines
	res := interp.execBlock(&BlockStmt{Stmts: prog.Stmts}, interp.globals)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

// ===== BLOCK / STATEMENT EXECUTION =====

func (interp *Interpreter) execBlock(block *BlockStmt, env *Env) Result {
	if block == nil {
		return ok(NullVal())
	}
	for _, stmt := range block.Stmts {
		res := interp.execStmt(stmt, env)
		if res.Err != nil || res.Signal != SIG_NONE {
			return res
		}
	}
	return ok(NullVal())
}

func (interp *Interpreter) execStmt(stmt Stmt, env *Env) Result {
	switch s := stmt.(type) {
	case *LetStmt:
		return interp.execLet(s, env)
	case *FnDecl:
		return interp.execFnDecl(s, env)
	case *ReturnStmt:
		return interp.execReturn(s, env)
	case *IfStmt:
		return interp.execIf(s, env)
	case *WhileStmt:
		return interp.execWhile(s, env)
	case *ForInStmt:
		return interp.execForIn(s, env)
	case *BreakStmt:
		return brk()
	case *ContinueStmt:
		return cont()
	case *ImportStmt:
		return interp.execImport(s, env)
	case *MatchStmt:
		return interp.execMatch(s, env)
	case *ServeStmt:
		return interp.execServe(s, env)
	case *TryCatchStmt:
		return interp.execTryCatch(s, env)
	case *ThrowStmt:
		return interp.execThrow(s, env)
	case *DoWhileStmt:
		return interp.execDoWhile(s, env)
	case *UnlessStmt:
		return interp.execUnless(s, env)
	case *UntilStmt:
		return interp.execUntil(s, env)
	case *LoopStmt:
		return interp.execLoop(s, env)
	case *DeferStmt:
		// Defers are collected and run at end of function — for simplicity we run at block end
		// Full defer support requires scope-level tracking; here we just run them at end
		res := interp.evalExpr(s.Call, env)
		return res
	case *StructDecl:
		return interp.execStructDecl(s, env)
	case *EnumDecl:
		return interp.execEnumDecl(s, env)
	case *ImplBlock:
		return interp.execImplBlock(s, env)
	case *TypeAlias:
		return interp.execTypeAlias(s, env)
	case *BgStmt:
		return interp.execBg(s, env)
	case *BlockStmt:
		childEnv := NewEnv(env)
		return interp.execBlock(s, childEnv)
	case *ExprStmt:
		res := interp.evalExpr(s.Expr, env)
		return res
	}
	return ok(NullVal())
}

func (interp *Interpreter) execLet(s *LetStmt, env *Env) Result {
	var val *Value = NullVal()
	if s.Value != nil {
		res := interp.evalExpr(s.Value, env)
		if res.Err != nil {
			return res
		}
		val = res.Value
	}
	env.Def(s.Name, val, s.IsConst)
	return ok(val)
}

func (interp *Interpreter) execFnDecl(s *FnDecl, env *Env) Result {
	fn := &Value{
		Type:   VAL_FUNCTION,
		FnDecl: s,
		FnEnv:  env,
	}
	env.Def(s.Name, fn, false)
	return ok(fn)
}

func (interp *Interpreter) execReturn(s *ReturnStmt, env *Env) Result {
	if s.Value == nil {
		return ret(NullVal())
	}
	res := interp.evalExpr(s.Value, env)
	if res.Err != nil {
		return res
	}
	return ret(res.Value)
}

func (interp *Interpreter) execIf(s *IfStmt, env *Env) Result {
	condRes := interp.evalExpr(s.Cond, env)
	if condRes.Err != nil {
		return condRes
	}
	if condRes.Value.IsTruthy() {
		childEnv := NewEnv(env)
		return interp.execBlock(s.Then, childEnv)
	} else if s.Else != nil {
		childEnv := NewEnv(env)
		return interp.execStmt(s.Else, childEnv)
	}
	return ok(NullVal())
}

func (interp *Interpreter) execWhile(s *WhileStmt, env *Env) Result {
	for {
		condRes := interp.evalExpr(s.Cond, env)
		if condRes.Err != nil {
			return condRes
		}
		if !condRes.Value.IsTruthy() {
			break
		}
		childEnv := NewEnv(env)
		res := interp.execBlock(s.Body, childEnv)
		if res.Err != nil {
			return res
		}
		if res.Signal == SIG_BREAK {
			break
		}
		if res.Signal == SIG_RETURN {
			return res
		}
	}
	return ok(NullVal())
}

func (interp *Interpreter) execForIn(s *ForInStmt, env *Env) Result {
	iterRes := interp.evalExpr(s.Iter, env)
	if iterRes.Err != nil {
		return iterRes
	}

	iter := iterRes.Value
	loop := func(key, val *Value) Result {
		childEnv := NewEnv(env)
		if s.Key != "" {
			childEnv.Def(s.Key, key, false)
		}
		childEnv.Def(s.Value, val, false)
		res := interp.execBlock(s.Body, childEnv)
		return res
	}

	switch iter.Type {
	case VAL_ARRAY:
		for i, el := range iter.Array {
			res := loop(NumberVal(float64(i)), el)
			if res.Err != nil { return res }
			if res.Signal == SIG_BREAK { break }
			if res.Signal == SIG_RETURN { return res }
		}
	case VAL_OBJECT:
		keys := iter.Keys
		if len(keys) == 0 {
			for k := range iter.Object { keys = append(keys, k) }
		}
		for _, k := range keys {
			v := iter.Object[k]
			res := loop(StringVal(k), v)
			if res.Err != nil { return res }
			if res.Signal == SIG_BREAK { break }
			if res.Signal == SIG_RETURN { return res }
		}
	case VAL_STRING:
		for i, ch := range iter.Str {
			res := loop(NumberVal(float64(i)), StringVal(string(ch)))
			if res.Err != nil { return res }
			if res.Signal == SIG_BREAK { break }
			if res.Signal == SIG_RETURN { return res }
		}
	}
	return ok(NullVal())
}

func (interp *Interpreter) execImport(s *ImportStmt, env *Env) Result {
	// Built-in module system
	switch s.Module {
	case "net", "http", "tcp", "udp", "ws":
		// Already built-in
	default:
		return errResult(fmt.Errorf("module '%s' not found", s.Module))
	}
	return ok(NullVal())
}

func (interp *Interpreter) execMatch(s *MatchStmt, env *Env) Result {
	subjRes := interp.evalExpr(s.Subject, env)
	if subjRes.Err != nil { return subjRes }
	subj := subjRes.Value

	for _, arm := range s.Arms {
		if arm.Pattern == nil {
			// Default arm
			childEnv := NewEnv(env)
			return interp.execBlock(arm.Body, childEnv)
		}
		patRes := interp.evalExpr(arm.Pattern, env)
		if patRes.Err != nil { return patRes }
		if subj.Equals(patRes.Value) {
			childEnv := NewEnv(env)
			return interp.execBlock(arm.Body, childEnv)
		}
	}
	return ok(NullVal())
}

func (interp *Interpreter) execServe(s *ServeStmt, env *Env) Result {
	addrRes := interp.evalExpr(s.Addr, env)
	if addrRes.Err != nil { return addrRes }
	addr := addrRes.Value.String()

	mux := http.NewServeMux()

	// Register routes
	for _, route := range s.Routes {
		r := route
		pathRes := interp.evalExpr(r.Path, env)
		if pathRes.Err != nil { return pathRes }
		path := pathRes.Value.String()

		handlerRes := interp.evalExpr(r.Handler, env)
		if handlerRes.Err != nil { return handlerRes }
		handler := handlerRes.Value

		capturedEnv := env
		mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
			if r.Method != "*" && req.Method != r.Method {
				http.Error(w, "Method Not Allowed", 405)
				return
			}
			reqVal := &Value{Type: VAL_REQUEST, Req: req}
			respVal := &Value{Type: VAL_OBJECT, Object: map[string]*Value{}, Keys: []string{}}
			// Build res object with methods
			for _, key := range []string{"send", "json", "html", "status", "header"} {
				k := key
				respVal.Object[k] = respWriterMethod(w, k)
				respVal.Keys = append(respVal.Keys, k)
			}
			ci := &CallInfo{File: interp.file, Line: r.line, Col: r.col, Lines: interp.lines}
			_, err := callValue(handler, []*Value{reqVal, respVal}, ci)
			if err != nil {
				interp.mu.Lock()
				fmt.Fprintf(os.Stderr, "handler error: %v\n", err)
				interp.mu.Unlock()
				http.Error(w, "Internal Server Error", 500)
			}
			_ = capturedEnv
		})
	}

	// Pretty startup message
	fmt.Printf("%s%s🚀 vex server running on http://%s%s\n",
		colorBold, colorGreen, addr, colorReset)
	for _, r := range s.Routes {
		pathRes, _ := interp.evalExpr(r.Path, env), r
		_ = pathRes
	}
	fmt.Printf("%s   press Ctrl+C to stop%s\n", colorDim, colorReset)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return errResult(fmt.Errorf("server error: %v", err))
	}
	return ok(NullVal())
}

// ===== EXPRESSION EVALUATION =====

func (interp *Interpreter) evalExpr(expr Expr, env *Env) Result {
	if expr == nil {
		return ok(NullVal())
	}
	switch e := expr.(type) {
	case *NumberLit:
		return ok(NumberVal(e.Value))
	case *StringLit:
		// Handle template interpolation
		if strings.Contains(e.Value, "${") {
			return ok(StringVal(interpolateTemplate(e.Value, env, interp)))
		}
		return ok(StringVal(e.Value))
	case *BoolLit:
		return ok(BoolVal(e.Value))
	case *NullLit:
		return ok(NullVal())
	case *Ident:
		return interp.evalIdent(e, env)
	case *ArrayLit:
		return interp.evalArray(e, env)
	case *ObjectLit:
		return interp.evalObject(e, env)
	case *FnLit:
		return ok(&Value{Type: VAL_FUNCTION, Fn: e, FnEnv: env})
	case *BinaryExpr:
		return interp.evalBinary(e, env)
	case *UnaryExpr:
		return interp.evalUnary(e, env)
	case *AssignExpr:
		return interp.evalAssign(e, env)
	case *CallExpr:
		return interp.evalCall(e, env)
	case *MemberExpr:
		return interp.evalMember(e, env)
	case *IndexExpr:
		return interp.evalIndex(e, env)
	case *PipeExpr:
		return interp.evalPipe(e, env)
	case *TernaryExpr:
		return interp.evalTernary(e, env)
	case *SpawnExpr:
		return interp.evalSpawn(e, env)
	case *AwaitExpr:
		return interp.evalAwait(e, env)
	case *FetchExpr:
		return interp.evalFetch(e, env)
	case *ConnectExpr:
		return interp.evalConnect(e, env)
	case *ListenExpr:
		return interp.evalListen(e, env)
	case *SendExpr:
		return interp.evalSend(e, env)
	case *RecvExpr:
		return interp.evalRecv(e, env)
	case *ShellExpr:
		return interp.evalShell(e, env)
	case *NewExpr:
		return interp.evalNew(e, env)
	case *IsExpr:
		return interp.evalIs(e, env)
	case *AsExpr:
		return interp.evalAs(e, env)
	case *RangeExpr:
		return interp.evalRange(e, env)
	case *SelfExpr:
		if v, found := env.Get("self"); found { return ok(v) }
		return ok(NullVal())
	}
	return ok(NullVal())
}

func (interp *Interpreter) evalIdent(e *Ident, env *Env) Result {
	val, ok2 := env.Get(e.Name)
	if !ok2 {
		err := NewVexError(
			ERR_UNDEFINED,
			fmt.Sprintf("'%s' is not defined", e.Name),
			interp.file, e.line, e.col, interp.lines,
			"variable not found in scope",
			fmt.Sprintf("declare it with: let %s = ...", e.Name),
		)
		return errResult(err)
	}
	return ok(val)
}

func (interp *Interpreter) evalArray(e *ArrayLit, env *Env) Result {
	arr := make([]*Value, 0, len(e.Elements))
	for _, el := range e.Elements {
		if unary, isUnary := el.(*UnaryExpr); isUnary && unary.Op == "..." {
			res := interp.evalExpr(unary.Operand, env)
			if res.Err != nil { return res }
			if res.Value.Type == VAL_ARRAY {
				arr = append(arr, res.Value.Array...)
			} else {
				arr = append(arr, res.Value)
			}
		} else {
			res := interp.evalExpr(el, env)
			if res.Err != nil { return res }
			arr = append(arr, res.Value)
		}
	}
	return ok(ArrayVal(arr))
}

func (interp *Interpreter) evalObject(e *ObjectLit, env *Env) Result {
	obj := make(map[string]*Value)
	keys := make([]string, 0, len(e.Keys))
	for i, key := range e.Keys {
		res := interp.evalExpr(e.Values[i], env)
		if res.Err != nil { return res }
		obj[key] = res.Value
		keys = append(keys, key)
	}
	return ok(ObjectVal(obj, keys))
}

func (interp *Interpreter) evalBinary(e *BinaryExpr, env *Env) Result {
	leftRes := interp.evalExpr(e.Left, env)
	if leftRes.Err != nil { return leftRes }

	// Short-circuit for && and ||
	if e.Op == "&&" {
		if !leftRes.Value.IsTruthy() { return ok(leftRes.Value) }
		return interp.evalExpr(e.Right, env)
	}
	if e.Op == "||" {
		if leftRes.Value.IsTruthy() { return ok(leftRes.Value) }
		return interp.evalExpr(e.Right, env)
	}
	if e.Op == "??" {
		if !leftRes.Value.IsNull() && !leftRes.Value.IsError() { return ok(leftRes.Value) }
		return interp.evalExpr(e.Right, env)
	}

	rightRes := interp.evalExpr(e.Right, env)
	if rightRes.Err != nil { return rightRes }

	l, r := leftRes.Value, rightRes.Value

	switch e.Op {
	case "+":
		if l.Type == VAL_STRING || r.Type == VAL_STRING {
			return ok(StringVal(l.String() + r.String()))
		}
		if l.Type == VAL_ARRAY && r.Type == VAL_ARRAY {
			merged := append(l.Array, r.Array...)
			return ok(ArrayVal(merged))
		}
		return ok(NumberVal(l.Num + r.Num))
	case "-": return ok(NumberVal(l.Num - r.Num))
	case "*":
		if l.Type == VAL_STRING && r.Type == VAL_NUMBER {
			return ok(StringVal(strings.Repeat(l.Str, int(r.Num))))
		}
		return ok(NumberVal(l.Num * r.Num))
	case "**": return ok(NumberVal(math.Pow(l.Num, r.Num)))
	case "/":
		if r.Num == 0 {
			return ok(NumberVal(math.NaN()))
		}
		return ok(NumberVal(l.Num / r.Num))
	case "%": return ok(NumberVal(math.Mod(l.Num, r.Num)))
	case "==": return ok(BoolVal(l.Equals(r)))
	case "!=": return ok(BoolVal(!l.Equals(r)))
	case "<":  return ok(BoolVal(l.Num < r.Num))
	case "<=": return ok(BoolVal(l.Num <= r.Num))
	case ">":  return ok(BoolVal(l.Num > r.Num))
	case ">=": return ok(BoolVal(l.Num >= r.Num))
	case "&":  return ok(NumberVal(float64(int64(l.Num) & int64(r.Num))))
	case "^":  return ok(NumberVal(float64(int64(l.Num) ^ int64(r.Num))))
	case "<<": return ok(NumberVal(float64(int64(l.Num) << uint(r.Num))))
	case ">>": return ok(NumberVal(float64(int64(l.Num) >> uint(r.Num))))
	}
	return ok(NullVal())
}

func (interp *Interpreter) evalUnary(e *UnaryExpr, env *Env) Result {
	res := interp.evalExpr(e.Operand, env)
	if res.Err != nil { return res }
	switch e.Op {
	case "!": return ok(BoolVal(!res.Value.IsTruthy()))
	case "-": return ok(NumberVal(-res.Value.Num))
	}
	return res
}

func (interp *Interpreter) evalAssign(e *AssignExpr, env *Env) Result {
	valRes := interp.evalExpr(e.Value, env)
	if valRes.Err != nil { return valRes }
	val := valRes.Value

	switch target := e.Target.(type) {
	case *Ident:
		existing, exists := env.Get(target.Name)
		if e.Op != "=" && exists {
			switch e.Op {
			case "+=":
				if existing.Type == VAL_STRING { val = StringVal(existing.Str + val.String()) } else { val = NumberVal(existing.Num + val.Num) }
			case "-=": val = NumberVal(existing.Num - val.Num)
			case "*=": val = NumberVal(existing.Num * val.Num)
			case "/=": val = NumberVal(existing.Num / val.Num)
			}
		}
		if err := env.Set(target.Name, val); err != nil {
			return errResult(NewVexError(ERR_RUNTIME, err.Error(), interp.file, target.line, target.col, interp.lines, "cannot reassign a constant", "use 'let' instead of 'const'"))
		}
	case *MemberExpr:
		objRes := interp.evalExpr(target.Object, env)
		if objRes.Err != nil { return objRes }
		obj := objRes.Value
		if obj.Type == VAL_OBJECT {
			if obj.Object == nil { obj.Object = make(map[string]*Value) }
			// Add key if new
			if _, exists := obj.Object[target.Key]; !exists {
				obj.Keys = append(obj.Keys, target.Key)
			}
			obj.Object[target.Key] = val
		}
	case *IndexExpr:
		objRes := interp.evalExpr(target.Object, env)
		if objRes.Err != nil { return objRes }
		idxRes := interp.evalExpr(target.Index, env)
		if idxRes.Err != nil { return idxRes }
		obj := objRes.Value
		if obj.Type == VAL_ARRAY {
			idx := int(idxRes.Value.Num)
			if idx >= 0 && idx < len(obj.Array) {
				obj.Array[idx] = val
			}
		} else if obj.Type == VAL_OBJECT {
			key := idxRes.Value.String()
			if _, exists := obj.Object[key]; !exists {
				obj.Keys = append(obj.Keys, key)
			}
			obj.Object[key] = val
		}
	}
	return ok(val)
}

func (interp *Interpreter) evalCall(e *CallExpr, env *Env) Result {
	calleeRes := interp.evalExpr(e.Callee, env)
	if calleeRes.Err != nil { return calleeRes }
	fn := calleeRes.Value

	args := make([]*Value, 0, len(e.Args))
	for _, arg := range e.Args {
		if unary, isSpread := arg.(*UnaryExpr); isSpread && unary.Op == "..." {
			res := interp.evalExpr(unary.Operand, env)
			if res.Err != nil { return res }
			if res.Value.Type == VAL_ARRAY { args = append(args, res.Value.Array...) } else { args = append(args, res.Value) }
		} else {
			res := interp.evalExpr(arg, env)
			if res.Err != nil { return res }
			args = append(args, res.Value)
		}
	}

	ci := &CallInfo{File: interp.file, Line: e.line, Col: e.col, Lines: interp.lines}

	switch fn.Type {
	case VAL_BUILTIN:
		val, err := fn.Builtin(args, ci)
		if err != nil { return errResult(err) }
		return ok(val)
	case VAL_FUNCTION:
		childEnv := NewEnv(fn.FnEnv)
		var params []Param
		if fn.FnDecl != nil {
			params = fn.FnDecl.Params
		} else if fn.Fn != nil {
			params = fn.Fn.Params
		}
		for i, param := range params {
			if i < len(args) {
				childEnv.Def(param.Name, args[i], false)
			} else if param.Default != nil {
				res := interp.evalExpr(param.Default, childEnv)
				if res.Err != nil { return res }
				childEnv.Def(param.Name, res.Value, false)
			} else {
				childEnv.Def(param.Name, NullVal(), false)
			}
		}
		// args array
		childEnv.Def("args", ArrayVal(args), false)
		var body *BlockStmt
		if fn.Fn != nil { body = fn.Fn.Body }
		if fn.FnDecl != nil { body = fn.FnDecl.Body }
		res := interp.execBlock(body, childEnv)
		if res.Err != nil { return res }
		if res.Signal == SIG_RETURN {
			if res.Value != nil { return ok(res.Value) }
			return ok(NullVal())
		}
		return ok(NullVal())
	}

	return errResult(NewVexError(ERR_TYPE,
		fmt.Sprintf("'%s' is not a function", fn.String()),
		interp.file, e.line, e.col, interp.lines,
		"only functions can be called with ()",
		"make sure the value is declared as fn",
	))
}

func (interp *Interpreter) evalMember(e *MemberExpr, env *Env) Result {
	objRes := interp.evalExpr(e.Object, env)
	if objRes.Err != nil { return objRes }
	obj := objRes.Value

	// Request object special handling
	if obj.Type == VAL_REQUEST {
		return ok(requestMethod(obj, e.Key))
	}

	prop := obj.GetProp(e.Key)
	return ok(prop)
}

func (interp *Interpreter) evalIndex(e *IndexExpr, env *Env) Result {
	objRes := interp.evalExpr(e.Object, env)
	if objRes.Err != nil { return objRes }
	idxRes := interp.evalExpr(e.Index, env)
	if idxRes.Err != nil { return idxRes }

	obj := objRes.Value
	idx := idxRes.Value

	switch obj.Type {
	case VAL_ARRAY:
		i := int(idx.Num)
		if i < 0 { i = len(obj.Array) + i }
		if i >= 0 && i < len(obj.Array) { return ok(obj.Array[i]) }
		return ok(NullVal())
	case VAL_OBJECT:
		key := idx.String()
		if val, exists := obj.Object[key]; exists { return ok(val) }
		return ok(NullVal())
	case VAL_STRING:
		i := int(idx.Num)
		runes := []rune(obj.Str)
		if i < 0 { i = len(runes) + i }
		if i >= 0 && i < len(runes) { return ok(StringVal(string(runes[i]))) }
		return ok(NullVal())
	}
	return ok(NullVal())
}

func (interp *Interpreter) evalPipe(e *PipeExpr, env *Env) Result {
	leftRes := interp.evalExpr(e.Left, env)
	if leftRes.Err != nil { return leftRes }

	rightRes := interp.evalExpr(e.Right, env)
	if rightRes.Err != nil { return rightRes }

	fn := rightRes.Value
	ci := &CallInfo{File: interp.file, Line: e.line, Col: e.col, Lines: interp.lines}
	val, err := callValue(fn, []*Value{leftRes.Value}, ci)
	if err != nil { return errResult(err) }
	return ok(val)
}

func (interp *Interpreter) evalTernary(e *TernaryExpr, env *Env) Result {
	condRes := interp.evalExpr(e.Cond, env)
	if condRes.Err != nil { return condRes }
	if condRes.Value.IsTruthy() {
		return interp.evalExpr(e.Then, env)
	}
	return interp.evalExpr(e.Else, env)
}

func (interp *Interpreter) evalSpawn(e *SpawnExpr, env *Env) Result {
	ch := make(chan *Value, 1)
	go func() {
		res := interp.evalExpr(e.Call, env)
		if res.Value != nil {
			ch <- res.Value
		} else {
			ch <- NullVal()
		}
		close(ch)
	}()
	return ok(ChanVal(ch))
}

func (interp *Interpreter) evalAwait(e *AwaitExpr, env *Env) Result {
	res := interp.evalExpr(e.Value, env)
	if res.Err != nil { return res }
	if res.Value.Type == VAL_CHANNEL {
		val := <-res.Value.Chan
		if val == nil { return ok(NullVal()) }
		return ok(val)
	}
	return res
}

func (interp *Interpreter) evalFetch(e *FetchExpr, env *Env) Result {
	urlRes := interp.evalExpr(e.URL, env)
	if urlRes.Err != nil { return urlRes }
	urlStr := urlRes.Value.String()

	method := "GET"
	var body strings.Builder
	headers := map[string]string{}

	for k, v := range e.Options {
		res := interp.evalExpr(v, env)
		if res.Err != nil { return res }
		switch k {
		case "method":
			method = strings.ToUpper(res.Value.String())
		case "body":
			if res.Value.Type == VAL_OBJECT || res.Value.Type == VAL_ARRAY {
				body.WriteString(toJSON(res.Value))
				if _, ok2 := headers["Content-Type"]; !ok2 {
					headers["Content-Type"] = "application/json"
				}
			} else {
				body.WriteString(res.Value.String())
			}
		case "headers":
			if res.Value.Type == VAL_OBJECT {
				for hk, hv := range res.Value.Object {
					headers[hk] = hv.String()
				}
			}
		}
	}

	var bodyReader *strings.Reader
	if body.Len() > 0 {
		bodyReader = strings.NewReader(body.String())
	}

	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequest(method, urlStr, bodyReader)
	} else {
		req, err = http.NewRequest(method, urlStr, nil)
	}
	if err != nil {
		return ok(ErrVal(fmt.Sprintf("fetch error: %s", err.Error())))
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if _, ok2 := headers["User-Agent"]; !ok2 {
		req.Header.Set("User-Agent", "vex/1.0")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ok(ErrVal(fmt.Sprintf("network error: %s", err.Error())))
	}

	return ok(&Value{Type: VAL_RESPONSE, Resp: resp})
}

func (interp *Interpreter) evalConnect(e *ConnectExpr, env *Env) Result {
	addrRes := interp.evalExpr(e.Addr, env)
	if addrRes.Err != nil { return addrRes }
	addr := addrRes.Value.String()

	proto := e.Protocol
	if p, ok2 := e.Options["protocol"]; ok2 {
		res := interp.evalExpr(p, env)
		if res.Err != nil { return res }
		proto = res.Value.String()
	}
	if proto == "" { proto = "tcp" }

	conn, err := net.DialTimeout(proto, addr, 10*time.Second)
	if err != nil {
		return ok(ErrVal(fmt.Sprintf("connection failed: %s", err.Error())))
	}
	return ok(&Value{Type: VAL_CONN, Conn: conn})
}

func (interp *Interpreter) evalListen(e *ListenExpr, env *Env) Result {
	addrRes := interp.evalExpr(e.Addr, env)
	if addrRes.Err != nil { return addrRes }
	addr := addrRes.Value.String()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return ok(ErrVal(fmt.Sprintf("listen failed: %s", err.Error())))
	}
	fmt.Printf("%s%s📡 TCP listening on %s%s\n", colorBold, colorCyan, addr, colorReset)
	return ok(&Value{Type: VAL_LISTENER, Listener: ln})
}

func (interp *Interpreter) evalSend(e *SendExpr, env *Env) Result {
	connRes := interp.evalExpr(e.Conn, env)
	if connRes.Err != nil { return connRes }
	dataRes := interp.evalExpr(e.Data, env)
	if dataRes.Err != nil { return dataRes }

	conn := connRes.Value
	data := dataRes.Value.String()

	if conn.Type == VAL_CONN && conn.Conn != nil {
		_, err := fmt.Fprint(conn.Conn, data)
		if err != nil { return ok(ErrVal(err.Error())) }
	}
	return ok(NullVal())
}

func (interp *Interpreter) evalRecv(e *RecvExpr, env *Env) Result {
	connRes := interp.evalExpr(e.Conn, env)
	if connRes.Err != nil { return connRes }
	conn := connRes.Value

	if conn.Type == VAL_CONN && conn.Conn != nil {
		buf := make([]byte, 4096)
		n, err := conn.Conn.Read(buf)
		if err != nil { return ok(ErrVal(err.Error())) }
		return ok(StringVal(string(buf[:n])))
	}
	return ok(NullVal())
}

// ===== BUILT-IN FUNCTIONS =====

func (interp *Interpreter) registerBuiltins() {
	env := interp.globals

	// I/O
	env.Def("print", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		parts := make([]string, len(args))
		for i, a := range args { parts[i] = a.String() }
		fmt.Println(strings.Join(parts, " "))
		return NullVal(), nil
	}), true)

	env.Def("println", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		parts := make([]string, len(args))
		for i, a := range args { parts[i] = a.String() }
		fmt.Println(strings.Join(parts, " "))
		return NullVal(), nil
	}), true)

	env.Def("eprint", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		parts := make([]string, len(args))
		for i, a := range args { parts[i] = a.String() }
		fmt.Fprintln(os.Stderr, strings.Join(parts, " "))
		return NullVal(), nil
	}), true)

	env.Def("input", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) > 0 { fmt.Print(args[0].String()) }
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		return StringVal(scanner.Text()), nil
	}), true)

	env.Def("inspect", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		parts := make([]string, len(args))
		for i, a := range args { parts[i] = a.Repr() }
		fmt.Println(strings.Join(parts, " "))
		return NullVal(), nil
	}), true)

	env.Def("fmt", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return StringVal(""), nil }
		format := args[0].String()
		fmtArgs := make([]interface{}, len(args)-1)
		for i, a := range args[1:] { fmtArgs[i] = a.String() }
		return StringVal(fmt.Sprintf(format, fmtArgs...)), nil
	}), true)

	// Type conversions
	env.Def("str", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return StringVal(""), nil }
		return StringVal(args[0].String()), nil
	}), true)

	env.Def("num", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NumberVal(0), nil }
		n, err := strconv.ParseFloat(args[0].String(), 64)
		if err != nil { return ErrVal("cannot convert to number"), nil }
		return NumberVal(n), nil
	}), true)

	env.Def("bool", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return BoolVal(false), nil }
		return BoolVal(args[0].IsTruthy()), nil
	}), true)

	env.Def("int", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NumberVal(0), nil }
		return NumberVal(math.Trunc(args[0].Num)), nil
	}), true)

	env.Def("type_of", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return StringVal("null"), nil }
		switch args[0].Type {
		case VAL_NUMBER:   return StringVal("number"), nil
		case VAL_STRING:   return StringVal("string"), nil
		case VAL_BOOL:     return StringVal("bool"), nil
		case VAL_NULL:     return StringVal("null"), nil
		case VAL_ARRAY:    return StringVal("array"), nil
		case VAL_OBJECT:   return StringVal("object"), nil
		case VAL_FUNCTION, VAL_BUILTIN: return StringVal("function"), nil
		case VAL_CONN:     return StringVal("connection"), nil
		case VAL_LISTENER: return StringVal("listener"), nil
		case VAL_RESPONSE: return StringVal("response"), nil
		case VAL_CHANNEL:  return StringVal("channel"), nil
		case VAL_ERROR:    return StringVal("error"), nil
		}
		return StringVal("unknown"), nil
	}), true)

	// Math
	env.Def("math", ObjectVal(map[string]*Value{
		"pi":    NumberVal(math.Pi),
		"e":     NumberVal(math.E),
		"inf":   NumberVal(math.Inf(1)),
		"nan":   NumberVal(math.NaN()),
		"floor": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Floor(args[0].Num)), nil }),
		"ceil":  builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Ceil(args[0].Num)), nil }),
		"round": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Round(args[0].Num)), nil }),
		"abs":   builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Abs(args[0].Num)), nil }),
		"sqrt":  builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Sqrt(args[0].Num)), nil }),
		"pow":   builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Pow(args[0].Num, args[1].Num)), nil }),
		"min":   builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Min(args[0].Num, args[1].Num)), nil }),
		"max":   builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Max(args[0].Num, args[1].Num)), nil }),
		"log":   builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Log(args[0].Num)), nil }),
		"sin":   builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Sin(args[0].Num)), nil }),
		"cos":   builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(math.Cos(args[0].Num)), nil }),
		"random": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) { return NumberVal(rand.Float64()), nil }),
		"rand_int": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) < 2 { return NumberVal(0), nil }
			min2, max2 := int(args[0].Num), int(args[1].Num)
			return NumberVal(float64(min2 + rand.Intn(max2-min2))), nil
		}),
	}, []string{"pi", "e", "floor", "ceil", "round", "abs", "sqrt", "pow", "min", "max", "log", "sin", "cos", "random"}), true)

	// JSON
	env.Def("json", ObjectVal(map[string]*Value{
		"parse": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return NullVal(), nil }
			v, err := parseJSON(args[0].String())
			if err != nil { return ErrVal(err.Error()), nil }
			return v, nil
		}),
		"stringify": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return StringVal("null"), nil }
			pretty := len(args) > 1 && args[1].Bool
			if pretty { return StringVal(toPrettyJSON(args[0])), nil }
			return StringVal(toJSON(args[0])), nil
		}),
	}, []string{"parse", "stringify"}), true)

	// Time
	env.Def("time", ObjectVal(map[string]*Value{
		"now": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			return NumberVal(float64(time.Now().UnixMilli())), nil
		}),
		"sleep": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) > 0 {
				time.Sleep(time.Duration(args[0].Num) * time.Millisecond)
			}
			return NullVal(), nil
		}),
		"format": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			t := time.Now()
			if len(args) > 0 {
				ms := int64(args[0].Num)
				t = time.UnixMilli(ms)
			}
			layout := time.RFC3339
			if len(args) > 1 {
				layout = args[1].String()
			}
			return StringVal(t.Format(layout)), nil
		}),
	}, []string{"now", "sleep", "format"}), true)

	// OS
	env.Def("os", ObjectVal(map[string]*Value{
		"args": func() *Value {
			arr := make([]*Value, len(os.Args))
			for i, a := range os.Args { arr[i] = StringVal(a) }
			return ArrayVal(arr)
		}(),
		"env": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return NullVal(), nil }
			return StringVal(os.Getenv(args[0].String())), nil
		}),
		"exit": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			code := 0
			if len(args) > 0 { code = int(args[0].Num) }
			os.Exit(code)
			return NullVal(), nil
		}),
		"read_file": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return ErrVal("read_file requires a path"), nil }
			b, err := os.ReadFile(args[0].String())
			if err != nil { return ErrVal(err.Error()), nil }
			return StringVal(string(b)), nil
		}),
		"write_file": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) < 2 { return ErrVal("write_file requires path and content"), nil }
			err := os.WriteFile(args[0].String(), []byte(args[1].String()), 0644)
			if err != nil { return ErrVal(err.Error()), nil }
			return BoolVal(true), nil
		}),
	}, []string{"args", "env", "exit", "read_file", "write_file"}), true)

	// Error handling
	env.Def("is_err", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return BoolVal(false), nil }
		return BoolVal(args[0].IsError()), nil
	}), true)

	env.Def("err_msg", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return StringVal(""), nil }
		if args[0].IsError() { return StringVal(args[0].ErrMsg), nil }
		return StringVal(""), nil
	}), true)

	env.Def("assert", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NullVal(), nil }
		if !args[0].IsTruthy() {
			msg := "assertion failed"
			if len(args) > 1 { msg = args[1].String() }
			return nil, fmt.Errorf("assertion failed: %s", msg)
		}
		return BoolVal(true), nil
	}), true)

	// Network utils
	env.Def("net", ObjectVal(map[string]*Value{
		"accept": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return ErrVal("accept requires a listener"), nil }
			ln := args[0]
			if ln.Type != VAL_LISTENER || ln.Listener == nil {
				return ErrVal("not a listener"), nil
			}
			conn, err := ln.Listener.Accept()
			if err != nil { return ErrVal(err.Error()), nil }
			return &Value{Type: VAL_CONN, Conn: conn}, nil
		}),
		"close": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return NullVal(), nil }
			v := args[0]
			if v.Type == VAL_CONN && v.Conn != nil { v.Conn.Close() }
			if v.Type == VAL_LISTENER && v.Listener != nil { v.Listener.Close() }
			return NullVal(), nil
		}),
		"send": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) < 2 { return ErrVal("send requires conn and data"), nil }
			conn := args[0]
			if conn.Type != VAL_CONN || conn.Conn == nil { return ErrVal("not a connection"), nil }
			_, err := conn.Conn.Write([]byte(args[1].String()))
			if err != nil { return ErrVal(err.Error()), nil }
			return NullVal(), nil
		}),
		"recv": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return ErrVal("recv requires a connection"), nil }
			conn := args[0]
			if conn.Type != VAL_CONN || conn.Conn == nil { return ErrVal("not a connection"), nil }
			size := 4096
			if len(args) > 1 { size = int(args[1].Num) }
			buf := make([]byte, size)
			n, err := conn.Conn.Read(buf)
			if err != nil { return ErrVal(err.Error()), nil }
			return StringVal(string(buf[:n])), nil
		}),
		"remote_addr": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return NullVal(), nil }
			conn := args[0]
			if conn.Type != VAL_CONN || conn.Conn == nil { return NullVal(), nil }
			return StringVal(conn.Conn.RemoteAddr().String()), nil
		}),
		"local_addr": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return NullVal(), nil }
			conn := args[0]
			if conn.Type != VAL_CONN || conn.Conn == nil { return NullVal(), nil }
			return StringVal(conn.Conn.LocalAddr().String()), nil
		}),
	}, []string{"accept", "close", "send", "recv", "remote_addr", "local_addr"}), true)

	// Channel utilities
	env.Def("chan", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		size := 0
		if len(args) > 0 { size = int(args[0].Num) }
		return ChanVal(make(chan *Value, size)), nil
	}), true)

	env.Def("chan_send", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return NullVal(), nil }
		ch := args[0]
		if ch.Type != VAL_CHANNEL { return ErrVal("not a channel"), nil }
		ch.Chan <- args[1]
		return NullVal(), nil
	}), true)

	env.Def("chan_recv", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NullVal(), nil }
		ch := args[0]
		if ch.Type != VAL_CHANNEL { return ErrVal("not a channel"), nil }
		val, ok2 := <-ch.Chan
		if !ok2 { return NullVal(), nil }
		return val, nil
	}), true)

	// Array/Object utilities at top level
	env.Def("keys", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ArrayVal(nil), nil }
		v := args[0]
		if v.Type == VAL_OBJECT {
			keys := v.Keys
			if len(keys) == 0 {
				for k := range v.Object { keys = append(keys, k) }
			}
			arr := make([]*Value, len(keys))
			for i, k := range keys { arr[i] = StringVal(k) }
			return ArrayVal(arr), nil
		}
		return ArrayVal(nil), nil
	}), true)

	env.Def("values", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ArrayVal(nil), nil }
		v := args[0]
		if v.Type == VAL_OBJECT {
			keys := v.Keys
			if len(keys) == 0 {
				for k := range v.Object { keys = append(keys, k) }
			}
			arr := make([]*Value, 0, len(keys))
			for _, k := range keys {
				if val, ok2 := v.Object[k]; ok2 { arr = append(arr, val) }
			}
			return ArrayVal(arr), nil
		}
		return ArrayVal(nil), nil
	}), true)

	env.Def("entries", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ArrayVal(nil), nil }
		v := args[0]
		if v.Type == VAL_OBJECT {
			keys := v.Keys
			if len(keys) == 0 {
				for k := range v.Object { keys = append(keys, k) }
			}
			arr := make([]*Value, 0, len(keys))
			for _, k := range keys {
				if val, ok2 := v.Object[k]; ok2 {
					entry := ArrayVal([]*Value{StringVal(k), val})
					arr = append(arr, entry)
				}
			}
			return ArrayVal(arr), nil
		}
		return ArrayVal(nil), nil
	}), true)

	env.Def("range", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ArrayVal(nil), nil }
		start, end2, step := 0.0, 0.0, 1.0
		if len(args) == 1 { end2 = args[0].Num } else { start = args[0].Num; end2 = args[1].Num }
		if len(args) >= 3 { step = args[2].Num }
		var arr []*Value
		for i := start; i < end2; i += step {
			arr = append(arr, NumberVal(i))
		}
		return ArrayVal(arr), nil
	}), true)

	env.Def("len", builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NumberVal(0), nil }
		v := args[0]
		switch v.Type {
		case VAL_STRING: return NumberVal(float64(len([]rune(v.Str)))), nil
		case VAL_ARRAY:  return NumberVal(float64(len(v.Array))), nil
		case VAL_OBJECT: return NumberVal(float64(len(v.Object))), nil
		}
		return NumberVal(0), nil
	}), true)

	// String utilities
	env.Def("string", ObjectVal(map[string]*Value{
		"from_bytes": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return StringVal(""), nil }
			arr := args[0].Array
			bs := make([]byte, len(arr))
			for i, b := range arr { bs[i] = byte(b.Num) }
			return StringVal(string(bs)), nil
		}),
		"repeat": builtinFn(func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) < 2 { return StringVal(""), nil }
			return StringVal(strings.Repeat(args[0].String(), int(args[1].Num))), nil
		}),
	}, []string{"from_bytes", "repeat"}), true)

	// Register all extended builtins (shell, process, timers, etc.)
	interp.registerExtendedBuiltins()
}

func builtinFn(fn func(args []*Value, ci *CallInfo) (*Value, error)) *Value {
	return &Value{Type: VAL_BUILTIN, Builtin: fn}
}
