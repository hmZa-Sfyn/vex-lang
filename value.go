package main

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
)

// ValueType enum
type ValueType int

const (
	VAL_NUMBER ValueType = iota
	VAL_STRING
	VAL_BOOL
	VAL_NULL
	VAL_ARRAY
	VAL_OBJECT
	VAL_FUNCTION
	VAL_BUILTIN
	VAL_CONN    // TCP connection
	VAL_LISTENER // TCP listener
	VAL_RESPONSE // HTTP response
	VAL_REQUEST  // HTTP request context
	VAL_CHANNEL  // async channel
	VAL_ERROR
)

// Value is a vex runtime value
type Value struct {
	Type     ValueType
	Num      float64
	Str      string
	Bool     bool
	Array    []*Value
	Object   map[string]*Value
	Fn       *FnLit
	FnDecl   *FnDecl
	FnEnv    *Env
	Builtin  func(args []*Value, callsite *CallInfo) (*Value, error)
	Conn     net.Conn
	Listener net.Listener
	Resp     *http.Response
	Req      *http.Request
	RespWriter http.ResponseWriter
	Chan     chan *Value
	ErrMsg   string
	// for object iteration order
	Keys     []string
}

type CallInfo struct {
	File  string
	Line  int
	Col   int
	Lines []string
}

func NumberVal(n float64) *Value   { return &Value{Type: VAL_NUMBER, Num: n} }
func StringVal(s string) *Value    { return &Value{Type: VAL_STRING, Str: s} }
func BoolVal(b bool) *Value        { return &Value{Type: VAL_BOOL, Bool: b} }
func NullVal() *Value              { return &Value{Type: VAL_NULL} }
func ErrVal(msg string) *Value     { return &Value{Type: VAL_ERROR, ErrMsg: msg} }
func ArrayVal(a []*Value) *Value   { return &Value{Type: VAL_ARRAY, Array: a} }
func ObjectVal(m map[string]*Value, keys []string) *Value {
	return &Value{Type: VAL_OBJECT, Object: m, Keys: keys}
}
func ChanVal(ch chan *Value) *Value { return &Value{Type: VAL_CHANNEL, Chan: ch} }

func (v *Value) IsTruthy() bool {
	switch v.Type {
	case VAL_BOOL:
		return v.Bool
	case VAL_NULL:
		return false
	case VAL_NUMBER:
		return v.Num != 0 && !math.IsNaN(v.Num)
	case VAL_STRING:
		return v.Str != ""
	case VAL_ERROR:
		return false
	}
	return true
}

func (v *Value) IsNull() bool { return v.Type == VAL_NULL }
func (v *Value) IsError() bool { return v.Type == VAL_ERROR }

func (v *Value) String() string {
	return v.Format(false)
}

func (v *Value) Format(pretty bool) string {
	return v.formatIndent(0, pretty)
}

func (v *Value) formatIndent(depth int, pretty bool) string {
	indent := strings.Repeat("  ", depth)
	inner  := strings.Repeat("  ", depth+1)

	switch v.Type {
	case VAL_NUMBER:
		if math.Trunc(v.Num) == v.Num && !math.IsInf(v.Num, 0) {
			return fmt.Sprintf("%g", v.Num)
		}
		return fmt.Sprintf("%g", v.Num)
	case VAL_STRING:
		return v.Str
	case VAL_BOOL:
		if v.Bool { return "true" }
		return "false"
	case VAL_NULL:
		return "null"
	case VAL_ERROR:
		return fmt.Sprintf("<error: %s>", v.ErrMsg)
	case VAL_ARRAY:
		if len(v.Array) == 0 {
			return "[]"
		}
		if !pretty {
			parts := make([]string, len(v.Array))
			for i, el := range v.Array {
				parts[i] = el.formatIndent(depth, false)
			}
			return "[" + strings.Join(parts, ", ") + "]"
		}
		var sb strings.Builder
		sb.WriteString("[\n")
		for _, el := range v.Array {
			sb.WriteString(inner + el.formatIndent(depth+1, pretty) + ",\n")
		}
		sb.WriteString(indent + "]")
		return sb.String()
	case VAL_OBJECT:
		if len(v.Object) == 0 {
			return "{}"
		}
		keys := v.Keys
		if len(keys) == 0 {
			for k := range v.Object {
				keys = append(keys, k)
			}
		}
		if !pretty {
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				if val, ok := v.Object[k]; ok {
					parts = append(parts, fmt.Sprintf("%s: %s", k, val.formatIndent(depth, false)))
				}
			}
			return "{ " + strings.Join(parts, ", ") + " }"
		}
		var sb strings.Builder
		sb.WriteString("{\n")
		for _, k := range keys {
			if val, ok := v.Object[k]; ok {
				sb.WriteString(fmt.Sprintf("%s%s: %s,\n", inner, k, val.formatIndent(depth+1, pretty)))
			}
		}
		sb.WriteString(indent + "}")
		return sb.String()
	case VAL_FUNCTION, VAL_BUILTIN:
		return "<fn>"
	case VAL_CONN:
		if v.Conn != nil {
			return fmt.Sprintf("<conn %s>", v.Conn.RemoteAddr())
		}
		return "<conn closed>"
	case VAL_LISTENER:
		if v.Listener != nil {
			return fmt.Sprintf("<listener %s>", v.Listener.Addr())
		}
		return "<listener closed>"
	case VAL_RESPONSE:
		if v.Resp != nil {
			return fmt.Sprintf("<response %d>", v.Resp.StatusCode)
		}
		return "<response>"
	case VAL_CHANNEL:
		return "<channel>"
	}
	return "null"
}

func (v *Value) Repr() string {
	switch v.Type {
	case VAL_STRING:
		return fmt.Sprintf("%q", v.Str)
	default:
		return v.String()
	}
}

func (v *Value) Equals(other *Value) bool {
	if v.Type != other.Type {
		return false
	}
	switch v.Type {
	case VAL_NUMBER:
		return v.Num == other.Num
	case VAL_STRING:
		return v.Str == other.Str
	case VAL_BOOL:
		return v.Bool == other.Bool
	case VAL_NULL:
		return true
	}
	return false
}

// GetProp retrieves a property from a value (including built-in methods)
func (v *Value) GetProp(key string) *Value {
	switch v.Type {
	case VAL_OBJECT:
		if val, ok := v.Object[key]; ok {
			return val
		}
	case VAL_ARRAY:
		return arrayMethod(v, key)
	case VAL_STRING:
		return stringMethod(v, key)
	case VAL_RESPONSE:
		return responseMethod(v, key)
	case VAL_REQUEST:
		return requestMethod(v, key)
	}
	return NullVal()
}

// ====== Environment (scope) ======

type Env struct {
	vars   map[string]*Value
	consts map[string]bool
	parent *Env
}

func NewEnv(parent *Env) *Env {
	return &Env{
		vars:   make(map[string]*Value),
		consts: make(map[string]bool),
		parent: parent,
	}
}

func (e *Env) Get(name string) (*Value, bool) {
	if v, ok := e.vars[name]; ok {
		return v, true
	}
	if e.parent != nil {
		return e.parent.Get(name)
	}
	return nil, false
}

func (e *Env) Set(name string, val *Value) error {
	// Check if const in any scope
	env := e
	for env != nil {
		if env.consts[name] {
			return fmt.Errorf("cannot reassign constant '%s'", name)
		}
		if _, ok := env.vars[name]; ok {
			env.vars[name] = val
			return nil
		}
		env = env.parent
	}
	// Not found in any scope, create in current
	e.vars[name] = val
	return nil
}

func (e *Env) Def(name string, val *Value, isConst bool) {
	e.vars[name] = val
	if isConst {
		e.consts[name] = true
	}
}

// ====== Control flow signals ======

type Signal int

const (
	SIG_NONE Signal = iota
	SIG_RETURN
	SIG_BREAK
	SIG_CONTINUE
)

type Result struct {
	Value  *Value
	Signal Signal
	Err    error
}

func ok(v *Value) Result             { return Result{Value: v} }
func ret(v *Value) Result            { return Result{Value: v, Signal: SIG_RETURN} }
func brk() Result                    { return Result{Signal: SIG_BREAK} }
func cont() Result                   { return Result{Signal: SIG_CONTINUE} }
func errResult(e error) Result       { return Result{Err: e} }
func valErr(msg string) Result       { return Result{Value: ErrVal(msg)} }
