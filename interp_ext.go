package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// =========================================================
//  SHELL EXECUTION
// =========================================================

// VexProcess wraps an os/exec process for bg process management
type VexProcess struct {
	Cmd     *exec.Cmd
	Stdout  *bytes.Buffer
	Stderr  *bytes.Buffer
	Done    chan struct{}
	ExitErr error
	mu      sync.Mutex
}

func (p *VexProcess) Wait() *Value {
	<-p.Done
	p.mu.Lock()
	defer p.mu.Unlock()
	obj := map[string]*Value{
		"stdout":   StringVal(p.Stdout.String()),
		"stderr":   StringVal(p.Stderr.String()),
		"success":  BoolVal(p.ExitErr == nil),
		"exit_code": NumberVal(0),
	}
	if p.ExitErr != nil {
		if exitErr, ok := p.ExitErr.(*exec.ExitError); ok {
			obj["exit_code"] = NumberVal(float64(exitErr.ExitCode()))
		}
		obj["error"] = StringVal(p.ExitErr.Error())
	}
	keys := []string{"stdout", "stderr", "success", "exit_code"}
	return ObjectVal(obj, keys)
}

// execShell runs a shell command (with template interpolation already done)
func execShell(cmd string, bg bool, env *Env, interp *Interpreter) *Value {
	// Choose shell
	shell := "/bin/sh"
	shellArg := "-c"
	if s := os.Getenv("SHELL"); s != "" {
		shell = s
	}
	if runtime.GOOS == "windows" {
		shell = "cmd"
		shellArg = "/C"
	}

	// Interpolate ${...} in command
	interpolated := interpolateTemplate(cmd, env, interp)

	var stdoutBuf, stderrBuf bytes.Buffer
	c := exec.Command(shell, shellArg, interpolated)
	c.Stdout = &stdoutBuf
	c.Stderr = &stderrBuf
	c.Env = os.Environ()

	if bg {
		done := make(chan struct{})
		proc := &VexProcess{Cmd: c, Stdout: &stdoutBuf, Stderr: &stderrBuf, Done: done}
		if err := c.Start(); err != nil {
			return ErrVal(fmt.Sprintf("process start failed: %s", err))
		}
		go func() {
			proc.ExitErr = c.Wait()
			close(done)
		}()

		// Return a process object
		procRef := proc
		waitFn := &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			return procRef.Wait(), nil
		}}
		killFn := &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if c.Process != nil { c.Process.Kill() }
			return NullVal(), nil
		}}
		obj := map[string]*Value{
			"pid":  NumberVal(float64(c.Process.Pid)),
			"wait": waitFn,
			"kill": killFn,
		}
		return ObjectVal(obj, []string{"pid", "wait", "kill"})
	}

	// Foreground: run and return result
	err := c.Run()
	result := map[string]*Value{
		"stdout":    StringVal(strings.TrimRight(stdoutBuf.String(), "\n")),
		"stderr":    StringVal(stderrBuf.String()),
		"success":   BoolVal(err == nil),
		"exit_code": NumberVal(0),
		"output":    StringVal(strings.TrimRight(stdoutBuf.String(), "\n")),
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = NumberVal(float64(exitErr.ExitCode()))
		}
		result["error"] = StringVal(err.Error())
	}
	return ObjectVal(result, []string{"stdout", "stderr", "success", "exit_code", "output"})
}

// =========================================================
//  STRUCT / ENUM RUNTIME
// =========================================================

// StructDef holds a struct definition
type StructDef struct {
	Name    string
	Fields  []StructField
	Methods map[string]*Value // from impl blocks
}

// EnumDef holds an enum definition
type EnumDef struct {
	Name     string
	Variants []EnumVariant
	Values   map[string]*Value // variant name -> value
}

// =========================================================
//  INTERPRETER EXEC EXTENSIONS
// =========================================================

func (interp *Interpreter) execTryCatch(s *TryCatchStmt, env *Env) Result {
	// Collect deferred calls
	var deferred []Expr

	childEnv := NewEnv(env)
	// inject defer collector
	childEnv.Def("__deferred__", NullVal(), false)

	res := interp.execBlock(s.Body, childEnv)

	// Run finally before returning regardless
	runFinally := func() {
		// Run any deferred calls (collected during block)
		for i := len(deferred) - 1; i >= 0; i-- {
			interp.evalExpr(deferred[i], childEnv)
		}
		if s.Finally != nil {
			interp.execBlock(s.Finally, NewEnv(env))
		}
	}

	if res.Err != nil {
		// Error occurred — run catch
		runFinally()
		if s.Catch != nil {
			catchEnv := NewEnv(env)
			if s.CatchVar != "" {
				errMsg := res.Err.Error()
				catchEnv.Def(s.CatchVar, StringVal(errMsg), false)
				// Also provide structured error
				catchEnv.Def(s.CatchVar+"_obj", ObjectVal(map[string]*Value{
					"message": StringVal(errMsg),
					"type":    StringVal("RuntimeError"),
				}, []string{"message", "type"}), false)
			}
			catchRes := interp.execBlock(s.Catch, catchEnv)
			if catchRes.Signal == SIG_RETURN { return catchRes }
			return ok(NullVal())
		}
		return ok(ErrVal(res.Err.Error()))
	}

	runFinally()
	if res.Signal == SIG_RETURN { return res }
	return ok(NullVal())
}

func (interp *Interpreter) execThrow(s *ThrowStmt, env *Env) Result {
	res := interp.evalExpr(s.Value, env)
	if res.Err != nil { return res }
	msg := res.Value.String()
	return errResult(fmt.Errorf("%s", msg))
}

func (interp *Interpreter) execDoWhile(s *DoWhileStmt, env *Env) Result {
	for {
		childEnv := NewEnv(env)
		res := interp.execBlock(s.Body, childEnv)
		if res.Err != nil { return res }
		if res.Signal == SIG_BREAK { break }
		if res.Signal == SIG_RETURN { return res }

		condRes := interp.evalExpr(s.Cond, env)
		if condRes.Err != nil { return condRes }
		if !condRes.Value.IsTruthy() { break }
	}
	return ok(NullVal())
}

func (interp *Interpreter) execUnless(s *UnlessStmt, env *Env) Result {
	condRes := interp.evalExpr(s.Cond, env)
	if condRes.Err != nil { return condRes }
	if !condRes.Value.IsTruthy() {
		childEnv := NewEnv(env)
		return interp.execBlock(s.Body, childEnv)
	} else if s.Else != nil {
		childEnv := NewEnv(env)
		return interp.execStmt(s.Else, childEnv)
	}
	return ok(NullVal())
}

func (interp *Interpreter) execUntil(s *UntilStmt, env *Env) Result {
	for {
		condRes := interp.evalExpr(s.Cond, env)
		if condRes.Err != nil { return condRes }
		if condRes.Value.IsTruthy() { break }
		childEnv := NewEnv(env)
		res := interp.execBlock(s.Body, childEnv)
		if res.Err != nil { return res }
		if res.Signal == SIG_BREAK { break }
		if res.Signal == SIG_RETURN { return res }
	}
	return ok(NullVal())
}

func (interp *Interpreter) execLoop(s *LoopStmt, env *Env) Result {
	for {
		childEnv := NewEnv(env)
		res := interp.execBlock(s.Body, childEnv)
		if res.Err != nil { return res }
		if res.Signal == SIG_BREAK { break }
		if res.Signal == SIG_RETURN { return res }
	}
	return ok(NullVal())
}

func (interp *Interpreter) execBg(s *BgStmt, env *Env) Result {
	go func() {
		childEnv := NewEnv(env)
		res := interp.execBlock(s.Body, childEnv)
		if res.Err != nil {
			interp.mu.Lock()
			fmt.Fprintf(os.Stderr, "%s[bg error]%s %v\n", colorRed+colorBold, colorReset, res.Err)
			interp.mu.Unlock()
		}
	}()
	return ok(NullVal())
}

func (interp *Interpreter) execStructDecl(s *StructDecl, env *Env) Result {
	def := &StructDef{
		Name:    s.Name,
		Fields:  s.Fields,
		Methods: make(map[string]*Value),
	}

	// Build constructor function
	constructor := &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		fields := make(map[string]*Value)
		keys := make([]string, 0, len(def.Fields))

		// If called with object arg, use it
		if len(args) == 1 && args[0].Type == VAL_OBJECT {
			for _, f := range def.Fields {
				keys = append(keys, f.Name)
				if v, ok := args[0].Object[f.Name]; ok {
					fields[f.Name] = v
				} else if f.Default != nil {
					r := interp.evalExpr(f.Default, env)
					if r.Err != nil { return nil, r.Err }
					fields[f.Name] = r.Value
				} else {
					fields[f.Name] = NullVal()
				}
			}
		} else {
			// Positional args
			for i, f := range def.Fields {
				keys = append(keys, f.Name)
				if i < len(args) {
					fields[f.Name] = args[i]
				} else if f.Default != nil {
					r := interp.evalExpr(f.Default, env)
					if r.Err != nil { return nil, r.Err }
					fields[f.Name] = r.Value
				} else {
					fields[f.Name] = NullVal()
				}
			}
		}

		// Attach methods
		for k, v := range def.Methods {
			fields[k] = v
			keys = append(keys, k)
		}

		v := ObjectVal(fields, keys)
		v.Str = "__struct__" + def.Name
		return v, nil
	}}

	// Store def for impl lookups
	interp.globals.Def("__struct_def_"+s.Name, NullVal(), false)
	structDefs[s.Name] = def

	env.Def(s.Name, constructor, false)
	return ok(constructor)
}

func (interp *Interpreter) execEnumDecl(s *EnumDecl, env *Env) Result {
	obj := make(map[string]*Value)
	keys := make([]string, 0, len(s.Variants)+1)

	for i, v := range s.Variants {
		var val *Value
		if v.Value != nil {
			res := interp.evalExpr(v.Value, env)
			if res.Err != nil { return res }
			val = res.Value
		} else {
			val = NumberVal(float64(i))
		}
		obj[v.Name] = val
		keys = append(keys, v.Name)
	}
	obj["_name"] = StringVal(s.Name)
	keys = append(keys, "_name")

	enumVal := ObjectVal(obj, keys)
	enumDefs[s.Name] = s
	env.Def(s.Name, enumVal, true)
	return ok(enumVal)
}

func (interp *Interpreter) execImplBlock(s *ImplBlock, env *Env) Result {
	def, ok2 := structDefs[s.Target]
	if !ok2 {
		return errResult(NewVexError(ERR_RUNTIME,
			fmt.Sprintf("cannot impl '%s': struct not found", s.Target),
			interp.file, s.line, s.col, interp.lines,
			"declare the struct before impl block",
			fmt.Sprintf("add 'struct %s { ... }' before this impl block", s.Target),
		))
	}

	// Also get the constructor from env so we can update it
	ctorVal, _ := env.Get(s.Target)

	for _, method := range s.Methods {
		m := method
		fnVal := &Value{
			Type:   VAL_FUNCTION,
			FnDecl: m,
			FnEnv:  env,
		}
		def.Methods[m.Name] = fnVal
		if ctorVal != nil {
			// patch: nothing to do, constructor reads def.Methods at call time
		}
	}
	_ = ctorVal
	return ok(NullVal())
}

func (interp *Interpreter) execTypeAlias(s *TypeAlias, env *Env) Result {
	// Just a declaration, no runtime effect
	env.Def(s.Name, StringVal(s.Value), false)
	return ok(NullVal())
}

// =========================================================
//  EXPRESSION EVAL EXTENSIONS
// =========================================================

func (interp *Interpreter) evalShell(e *ShellExpr, env *Env) Result {
	result := execShell(e.Command, e.Bg, env, interp)
	return ok(result)
}

func (interp *Interpreter) evalNew(e *NewExpr, env *Env) Result {
	def, found := structDefs[e.TypeName]
	if !found {
		// Try calling the constructor
		ctorVal, ok2 := env.Get(e.TypeName)
		if !ok2 {
			return errResult(NewVexError(ERR_UNDEFINED,
				fmt.Sprintf("struct '%s' is not defined", e.TypeName),
				interp.file, e.line, e.col, interp.lines,
				"struct must be declared before use",
				fmt.Sprintf("add 'struct %s { ... }' before this line", e.TypeName),
			))
		}
		// Build single object arg
		obj := make(map[string]*Value)
		for k, v := range e.Fields {
			res := interp.evalExpr(v, env)
			if res.Err != nil { return res }
			obj[k] = res.Value
		}
		ci := &CallInfo{File: interp.file, Line: e.line, Col: e.col, Lines: interp.lines}
		result, err := callValue(ctorVal, []*Value{ObjectVal(obj, e.Keys)}, ci)
		if err != nil { return errResult(err) }
		return ok(result)
	}

	fields := make(map[string]*Value)
	keys := make([]string, 0, len(def.Fields))
	for _, f := range def.Fields {
		keys = append(keys, f.Name)
		if fieldExpr, ok2 := e.Fields[f.Name]; ok2 {
			res := interp.evalExpr(fieldExpr, env)
			if res.Err != nil { return res }
			fields[f.Name] = res.Value
		} else if f.Default != nil {
			res := interp.evalExpr(f.Default, env)
			if res.Err != nil { return res }
			fields[f.Name] = res.Value
		} else {
			fields[f.Name] = NullVal()
		}
	}

	// Attach methods
	for k, v := range def.Methods {
		mCopy := v
		// Wrap method to inject self
		selfRef := ObjectVal(fields, keys)
		selfRef.Str = "__struct__" + def.Name
		fields[k] = wrapMethodSelf(mCopy, selfRef)
		keys = append(keys, k)
	}

	v := ObjectVal(fields, keys)
	v.Str = "__struct__" + def.Name
	return ok(v)
}

func wrapMethodSelf(fn *Value, self *Value) *Value {
	return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		// Inject self as first param if method uses it
		allArgs := append([]*Value{self}, args...)
		return callValue(fn, allArgs, ci)
	}}
}

func (interp *Interpreter) evalIs(e *IsExpr, env *Env) Result {
	res := interp.evalExpr(e.Value, env)
	if res.Err != nil { return res }
	v := res.Value

	switch e.TypeName {
	case "number":  return ok(BoolVal(v.Type == VAL_NUMBER))
	case "string":  return ok(BoolVal(v.Type == VAL_STRING))
	case "bool":    return ok(BoolVal(v.Type == VAL_BOOL))
	case "null":    return ok(BoolVal(v.Type == VAL_NULL))
	case "array":   return ok(BoolVal(v.Type == VAL_ARRAY))
	case "object":  return ok(BoolVal(v.Type == VAL_OBJECT))
	case "fn", "function": return ok(BoolVal(v.Type == VAL_FUNCTION || v.Type == VAL_BUILTIN))
	case "error":   return ok(BoolVal(v.Type == VAL_ERROR))
	case "conn":    return ok(BoolVal(v.Type == VAL_CONN))
	case "channel": return ok(BoolVal(v.Type == VAL_CHANNEL))
	default:
		// Check struct type tag
		if v.Type == VAL_OBJECT && v.Str == "__struct__"+e.TypeName {
			return ok(BoolVal(true))
		}
		return ok(BoolVal(false))
	}
}

func (interp *Interpreter) evalAs(e *AsExpr, env *Env) Result {
	res := interp.evalExpr(e.Value, env)
	if res.Err != nil { return res }
	v := res.Value

	switch e.TypeName {
	case "string": return ok(StringVal(v.String()))
	case "number":
		switch v.Type {
		case VAL_NUMBER: return ok(v)
		case VAL_STRING:
			var n float64
			fmt.Sscanf(v.Str, "%f", &n)
			return ok(NumberVal(n))
		case VAL_BOOL:
			if v.Bool { return ok(NumberVal(1)) }
			return ok(NumberVal(0))
		}
		return ok(NumberVal(0))
	case "bool": return ok(BoolVal(v.IsTruthy()))
	case "int": return ok(NumberVal(math.Trunc(v.Num)))
	case "float": return ok(NumberVal(v.Num))
	case "array":
		if v.Type == VAL_ARRAY { return ok(v) }
		return ok(ArrayVal([]*Value{v}))
	}
	return res
}

func (interp *Interpreter) evalRange(e *RangeExpr, env *Env) Result {
	startRes := interp.evalExpr(e.Start, env)
	if startRes.Err != nil { return startRes }
	endRes := interp.evalExpr(e.End, env)
	if endRes.Err != nil { return endRes }

	start := int(startRes.Value.Num)
	end   := int(endRes.Value.Num)
	if e.Inclusive { end++ }

	arr := make([]*Value, 0, end-start)
	for i := start; i < end; i++ {
		arr = append(arr, NumberVal(float64(i)))
	}
	return ok(ArrayVal(arr))
}

// =========================================================
//  GLOBAL REGISTRIES
// =========================================================

var structDefs = map[string]*StructDef{}
var enumDefs   = map[string]*EnumDecl{}

// =========================================================
//  EXTENDED BUILT-INS (registered in registerBuiltins)
// =========================================================

func (interp *Interpreter) registerExtendedBuiltins() {
	env := interp.globals

	// proc — run a shell command and return result
	env.Def("proc", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ErrVal("proc requires a command"), nil }
		cmd := args[0].String()
		bg := false
		if len(args) > 1 { bg = args[1].Bool }
		return execShell(cmd, bg, interp.globals, interp), nil
	}}, true)

	// sh — alias for proc (shorthand)
	env.Def("sh", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ErrVal("sh requires a command"), nil }
		result := execShell(args[0].String(), false, interp.globals, interp)
		// sh() returns stdout string directly for convenience
		if result.Type == VAL_OBJECT {
			if out, ok := result.Object["stdout"]; ok { return out, nil }
		}
		return result, nil
	}}, true)

	// exec — run with args array: exec("ls", ["-la", "/tmp"])
	env.Def("exec", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ErrVal("exec requires a command"), nil }
		cmdName := args[0].String()
		var cmdArgs []string
		if len(args) > 1 && args[1].Type == VAL_ARRAY {
			for _, a := range args[1].Array { cmdArgs = append(cmdArgs, a.String()) }
		}
		var stdoutBuf, stderrBuf bytes.Buffer
		c := exec.Command(cmdName, cmdArgs...)
		c.Stdout = &stdoutBuf
		c.Stderr = &stderrBuf
		err := c.Run()
		result := map[string]*Value{
			"stdout":    StringVal(strings.TrimRight(stdoutBuf.String(), "\n")),
			"stderr":    StringVal(stderrBuf.String()),
			"success":   BoolVal(err == nil),
			"exit_code": NumberVal(0),
		}
		if err != nil {
			if exitErr, ok2 := err.(*exec.ExitError); ok2 {
				result["exit_code"] = NumberVal(float64(exitErr.ExitCode()))
			}
			result["error"] = StringVal(err.Error())
		}
		return ObjectVal(result, []string{"stdout", "stderr", "success", "exit_code"}), nil
	}}, true)

	// pipe — pipe commands: pipe("echo hello", "grep h")
	env.Def("pipe_cmd", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return ErrVal("pipe_cmd requires at least 2 commands"), nil }

		cmds := make([]*exec.Cmd, len(args))
		for i, arg := range args {
			parts := strings.Fields(interpolateTemplate(arg.String(), interp.globals, interp))
			if len(parts) == 0 { continue }
			cmds[i] = exec.Command(parts[0], parts[1:]...)
		}

		// Chain pipes
		for i := 0; i < len(cmds)-1; i++ {
			if cmds[i] == nil || cmds[i+1] == nil { continue }
			pr, pw := io.Pipe()
			cmds[i].Stdout = pw
			cmds[i+1].Stdin = pr
			go func(pw *io.PipeWriter, c *exec.Cmd) {
				c.Wait()
				pw.Close()
			}(pw, cmds[i])
		}

		var finalOut bytes.Buffer
		if cmds[len(cmds)-1] != nil {
			cmds[len(cmds)-1].Stdout = &finalOut
		}

		for _, c := range cmds {
			if c != nil { c.Start() }
		}
		if cmds[len(cmds)-1] != nil {
			cmds[len(cmds)-1].Wait()
		}

		return StringVal(strings.TrimRight(finalOut.String(), "\n")), nil
	}}, true)

	// env_set — set environment variable for child processes
	env.Def("env_set", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return NullVal(), nil }
		os.Setenv(args[0].String(), args[1].String())
		return NullVal(), nil
	}}, true)

	// ── Process management ──
	env.Def("process", ObjectVal(map[string]*Value{
		"pid":  NumberVal(float64(os.Getpid())),
		"cwd": &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			dir, _ := os.Getwd()
			return StringVal(dir), nil
		}},
		"chdir": &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 { return ErrVal("chdir requires a path"), nil }
			return BoolVal(os.Chdir(args[0].String()) == nil), nil
		}},
		"env": &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			obj := make(map[string]*Value)
			var keys []string
			for _, e := range os.Environ() {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					obj[parts[0]] = StringVal(parts[1])
					keys = append(keys, parts[0])
				}
			}
			return ObjectVal(obj, keys), nil
		}},
		"exit": &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			code := 0
			if len(args) > 0 { code = int(args[0].Num) }
			os.Exit(code)
			return NullVal(), nil
		}},
	}, []string{"pid", "cwd", "chdir", "env", "exit"}), true)

	// ── Timer utilities ──
	env.Def("set_timeout", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return NullVal(), nil }
		fn := args[0]
		ms := args[1].Num
		go func() {
			time.Sleep(time.Duration(ms) * time.Millisecond)
			callValue(fn, nil, ci)
		}()
		return NullVal(), nil
	}}, true)

	env.Def("set_interval", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return NullVal(), nil }
		fn := args[0]
		ms := args[1].Num
		stop := make(chan bool, 1)
		go func() {
			ticker := time.NewTicker(time.Duration(ms) * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					callValue(fn, nil, ci)
				case <-stop:
					return
				}
			}
		}()
		stopFn := &Value{Type: VAL_BUILTIN, Builtin: func(a []*Value, ci2 *CallInfo) (*Value, error) {
			stop <- true
			return NullVal(), nil
		}}
		return stopFn, nil
	}}, true)

	// ── String builder / formatting ──
	env.Def("pad_start", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return NullVal(), nil }
		s := args[0].String()
		n := int(args[1].Num)
		pad := " "
		if len(args) > 2 { pad = args[2].String() }
		for len([]rune(s)) < n { s = pad + s }
		return StringVal(s), nil
	}}, true)

	env.Def("pad_end", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return NullVal(), nil }
		s := args[0].String()
		n := int(args[1].Num)
		pad := " "
		if len(args) > 2 { pad = args[2].String() }
		for len([]rune(s)) < n { s = s + pad }
		return StringVal(s), nil
	}}, true)

	// ── Pipeline utilities ──
	env.Def("tap", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return NullVal(), nil }
		val := args[0]
		fn := args[1]
		callValue(fn, []*Value{val}, ci)
		return val, nil // returns original unchanged
	}}, true)

	env.Def("compose", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		fns := args
		return &Value{Type: VAL_BUILTIN, Builtin: func(inner []*Value, ci2 *CallInfo) (*Value, error) {
			var cur *Value
			if len(inner) > 0 { cur = inner[0] }
			for _, fn := range fns {
				r, err := callValue(fn, []*Value{cur}, ci2)
				if err != nil { return nil, err }
				cur = r
			}
			return cur, nil
		}}, nil
	}}, true)

	env.Def("partial", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NullVal(), nil }
		fn := args[0]
		bound := args[1:]
		return &Value{Type: VAL_BUILTIN, Builtin: func(rest []*Value, ci2 *CallInfo) (*Value, error) {
			all := append(bound, rest...)
			return callValue(fn, all, ci2)
		}}, nil
	}}, true)

	env.Def("memoize", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NullVal(), nil }
		fn := args[0]
		cache := make(map[string]*Value)
		return &Value{Type: VAL_BUILTIN, Builtin: func(inner []*Value, ci2 *CallInfo) (*Value, error) {
			var keyParts []string
			for _, a := range inner { keyParts = append(keyParts, a.Repr()) }
			key := strings.Join(keyParts, ",")
			if v, ok := cache[key]; ok { return v, nil }
			r, err := callValue(fn, inner, ci2)
			if err != nil { return nil, err }
			cache[key] = r
			return r, nil
		}}, nil
	}}, true)

	// ── Event emitter ──
	env.Def("emitter", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		listeners := make(map[string][]*Value)
		var mu sync.Mutex

		on := &Value{Type: VAL_BUILTIN, Builtin: func(a []*Value, ci2 *CallInfo) (*Value, error) {
			if len(a) < 2 { return NullVal(), nil }
			event := a[0].String()
			mu.Lock()
			listeners[event] = append(listeners[event], a[1])
			mu.Unlock()
			return NullVal(), nil
		}}
		emit := &Value{Type: VAL_BUILTIN, Builtin: func(a []*Value, ci2 *CallInfo) (*Value, error) {
			if len(a) == 0 { return NullVal(), nil }
			event := a[0].String()
			payload := a[1:]
			mu.Lock()
			fns := listeners[event]
			mu.Unlock()
			for _, fn := range fns {
				callValue(fn, payload, ci2)
			}
			return NullVal(), nil
		}}
		off := &Value{Type: VAL_BUILTIN, Builtin: func(a []*Value, ci2 *CallInfo) (*Value, error) {
			if len(a) == 0 { return NullVal(), nil }
			event := a[0].String()
			mu.Lock()
			delete(listeners, event)
			mu.Unlock()
			return NullVal(), nil
		}}

		return ObjectVal(map[string]*Value{"on": on, "emit": emit, "off": off},
			[]string{"on", "emit", "off"}), nil
	}}, true)

	// ── Buffer (byte operations) ──
	env.Def("buffer", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		var buf bytes.Buffer
		if len(args) > 0 { buf.WriteString(args[0].String()) }

		write := &Value{Type: VAL_BUILTIN, Builtin: func(a []*Value, ci2 *CallInfo) (*Value, error) {
			if len(a) > 0 { buf.WriteString(a[0].String()) }
			return NullVal(), nil
		}}
		toString := &Value{Type: VAL_BUILTIN, Builtin: func(a []*Value, ci2 *CallInfo) (*Value, error) {
			return StringVal(buf.String()), nil
		}}
		reset := &Value{Type: VAL_BUILTIN, Builtin: func(a []*Value, ci2 *CallInfo) (*Value, error) {
			buf.Reset()
			return NullVal(), nil
		}}
		reader := &Value{Type: VAL_BUILTIN, Builtin: func(a []*Value, ci2 *CallInfo) (*Value, error) {
			return &Value{Type: VAL_BUILTIN, Builtin: func(inner []*Value, ci3 *CallInfo) (*Value, error) {
				line, _ := bufio.NewReader(&buf).ReadString('\n')
				return StringVal(line), nil
			}}, nil
		}}

		return ObjectVal(map[string]*Value{
			"write": write, "string": toString, "reset": reset, "reader": reader,
		}, []string{"write", "string", "reset", "reader"}), nil
	}}, true)

	// ── Regex (simple built-in) ──
	env.Def("regex", &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NullVal(), nil }
		pattern := args[0].String()
		return makeRegexObj(pattern), nil
	}}, true)
}

func makeRegexObj(pattern string) *Value {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ErrVal(fmt.Sprintf("invalid regex '%s': %s", pattern, err))
	}
	test := &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return BoolVal(false), nil }
		return BoolVal(re.MatchString(args[0].String())), nil
	}}
	match := &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return NullVal(), nil }
		m := re.FindStringSubmatch(args[0].String())
		if m == nil { return NullVal(), nil }
		arr := make([]*Value, len(m))
		for i, s := range m { arr[i] = StringVal(s) }
		return ArrayVal(arr), nil
	}}
	matchAll := &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ArrayVal(nil), nil }
		matches := re.FindAllString(args[0].String(), -1)
		arr := make([]*Value, len(matches))
		for i, s := range matches { arr[i] = StringVal(s) }
		return ArrayVal(arr), nil
	}}
	replace := &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) < 2 { return NullVal(), nil }
		return StringVal(re.ReplaceAllString(args[0].String(), args[1].String())), nil
	}}
	split := &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
		if len(args) == 0 { return ArrayVal(nil), nil }
		parts := re.Split(args[0].String(), -1)
		arr := make([]*Value, len(parts))
		for i, s := range parts { arr[i] = StringVal(s) }
		return ArrayVal(arr), nil
	}}
	return ObjectVal(map[string]*Value{
		"pattern":  StringVal(pattern),
		"test":     test,
		"match":    match,
		"match_all": matchAll,
		"replace":  replace,
		"split":    split,
	}, []string{"pattern", "test", "match", "match_all", "replace", "split"})
}
