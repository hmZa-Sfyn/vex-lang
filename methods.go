package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func stringMethod(v *Value, key string) *Value {
	s := v.Str
	switch key {
	case "len":
		return NumberVal(float64(len([]rune(s))))
	case "upper":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			return StringVal(strings.ToUpper(s)), nil
		}}
	case "lower":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			return StringVal(strings.ToLower(s)), nil
		}}
	case "trim":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			return StringVal(strings.TrimSpace(s)), nil
		}}
	case "split":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			sep := ""
			if len(args) > 0 {
				sep = args[0].Str
			}
			parts := strings.Split(s, sep)
			arr := make([]*Value, len(parts))
			for i, p := range parts {
				arr[i] = StringVal(p)
			}
			return ArrayVal(arr), nil
		}}
	case "contains":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return BoolVal(false), nil
			}
			return BoolVal(strings.Contains(s, args[0].Str)), nil
		}}
	case "starts_with":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return BoolVal(false), nil
			}
			return BoolVal(strings.HasPrefix(s, args[0].Str)), nil
		}}
	case "ends_with":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return BoolVal(false), nil
			}
			return BoolVal(strings.HasSuffix(s, args[0].Str)), nil
		}}
	case "replace":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) < 2 {
				return StringVal(s), nil
			}
			return StringVal(strings.ReplaceAll(s, args[0].Str, args[1].Str)), nil
		}}
	case "index":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return NumberVal(-1), nil
			}
			return NumberVal(float64(strings.Index(s, args[0].Str))), nil
		}}
	case "slice":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			runes := []rune(s)
			start, end := 0, len(runes)
			if len(args) > 0 {
				start = int(args[0].Num)
			}
			if len(args) > 1 {
				end = int(args[1].Num)
			}
			if start < 0 {
				start = 0
			}
			if end > len(runes) {
				end = len(runes)
			}
			if start > end {
				start = end
			}
			return StringVal(string(runes[start:end])), nil
		}}
	case "bytes":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			bs := []byte(s)
			arr := make([]*Value, len(bs))
			for i, b := range bs {
				arr[i] = NumberVal(float64(b))
			}
			return ArrayVal(arr), nil
		}}
	}
	return NullVal()
}

func arrayMethod(v *Value, key string) *Value {
	arr := v.Array
	switch key {
	case "len":
		return NumberVal(float64(len(arr)))
	case "push":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			v.Array = append(v.Array, args...)
			return v, nil
		}}
	case "pop":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(v.Array) == 0 {
				return NullVal(), nil
			}
			last := v.Array[len(v.Array)-1]
			v.Array = v.Array[:len(v.Array)-1]
			return last, nil
		}}
	case "shift":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(v.Array) == 0 {
				return NullVal(), nil
			}
			first := v.Array[0]
			v.Array = v.Array[1:]
			return first, nil
		}}
	case "join":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			sep := ""
			if len(args) > 0 {
				sep = args[0].Str
			}
			parts := make([]string, len(arr))
			for i, el := range arr {
				parts[i] = el.String()
			}
			return StringVal(strings.Join(parts, sep)), nil
		}}
	case "map":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return v, nil
			}
			fn := args[0]
			result := make([]*Value, len(arr))
			for i, el := range arr {
				r, err := callValue(fn, []*Value{el, NumberVal(float64(i))}, ci)
				if err != nil {
					return nil, err
				}
				result[i] = r
			}
			return ArrayVal(result), nil
		}}
	case "filter":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return v, nil
			}
			fn := args[0]
			var result []*Value
			for i, el := range arr {
				r, err := callValue(fn, []*Value{el, NumberVal(float64(i))}, ci)
				if err != nil {
					return nil, err
				}
				if r.IsTruthy() {
					result = append(result, el)
				}
			}
			return ArrayVal(result), nil
		}}
	case "reduce":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) < 2 {
				return NullVal(), nil
			}
			fn := args[0]
			acc := args[1]
			for i, el := range arr {
				r, err := callValue(fn, []*Value{acc, el, NumberVal(float64(i))}, ci)
				if err != nil {
					return nil, err
				}
				acc = r
			}
			return acc, nil
		}}
	case "find":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return NullVal(), nil
			}
			fn := args[0]
			for i, el := range arr {
				r, err := callValue(fn, []*Value{el, NumberVal(float64(i))}, ci)
				if err != nil {
					return nil, err
				}
				if r.IsTruthy() {
					return el, nil
				}
			}
			return NullVal(), nil
		}}
	case "includes":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return BoolVal(false), nil
			}
			for _, el := range arr {
				if el.Equals(args[0]) {
					return BoolVal(true), nil
				}
			}
			return BoolVal(false), nil
		}}
	case "slice":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			start, end := 0, len(arr)
			if len(args) > 0 {
				start = int(args[0].Num)
			}
			if len(args) > 1 {
				end = int(args[1].Num)
			}
			if start < 0 {
				start = 0
			}
			if end > len(arr) {
				end = len(arr)
			}
			if start > end {
				start = end
			}
			return ArrayVal(arr[start:end]), nil
		}}
	case "flat":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			var flat []*Value
			for _, el := range arr {
				if el.Type == VAL_ARRAY {
					flat = append(flat, el.Array...)
				} else {
					flat = append(flat, el)
				}
			}
			return ArrayVal(flat), nil
		}}
	case "reverse":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			cp := make([]*Value, len(arr))
			for i, el := range arr {
				cp[len(arr)-1-i] = el
			}
			return ArrayVal(cp), nil
		}}
	case "first":
		if len(arr) == 0 {
			return NullVal()
		}
		return arr[0]
	case "last":
		if len(arr) == 0 {
			return NullVal()
		}
		return arr[len(arr)-1]
	}
	return NullVal()
}

func responseMethod(v *Value, key string) *Value {
	resp := v.Resp
	if resp == nil {
		return NullVal()
	}
	switch key {
	case "status":
		return NumberVal(float64(resp.StatusCode))
	case "ok":
		return BoolVal(resp.StatusCode >= 200 && resp.StatusCode < 300)
	case "headers":
		obj := make(map[string]*Value)
		var keys []string
		for k, vs := range resp.Header {
			lower := strings.ToLower(k)
			obj[lower] = StringVal(strings.Join(vs, ", "))
			keys = append(keys, lower)
		}
		return ObjectVal(obj, keys)
	case "text":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return ErrVal(err.Error()), nil
			}
			return StringVal(string(b)), nil
		}}
	case "json":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return ErrVal(err.Error()), nil
			}
			val, err := parseJSON(string(b))
			if err != nil {
				return ErrVal(fmt.Sprintf("json parse error: %s", err.Error())), nil
			}
			return val, nil
		}}
	case "bytes":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				return ErrVal(err.Error()), nil
			}
			arr := make([]*Value, len(b))
			for i, by := range b {
				arr[i] = NumberVal(float64(by))
			}
			return ArrayVal(arr), nil
		}}
	}
	return NullVal()
}

func requestMethod(v *Value, key string) *Value {
	req := v.Req
	if req == nil {
		return NullVal()
	}
	switch key {
	case "method":
		return StringVal(req.Method)
	case "url":
		return StringVal(req.URL.String())
	case "path":
		return StringVal(req.URL.Path)
	case "query":
		obj := make(map[string]*Value)
		var keys []string
		for k, vs := range req.URL.Query() {
			obj[k] = StringVal(strings.Join(vs, ", "))
			keys = append(keys, k)
		}
		return ObjectVal(obj, keys)
	case "headers":
		obj := make(map[string]*Value)
		var keys []string
		for k, vs := range req.Header {
			lower := strings.ToLower(k)
			obj[lower] = StringVal(strings.Join(vs, ", "))
			keys = append(keys, lower)
		}
		return ObjectVal(obj, keys)
	case "body":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			defer req.Body.Close()
			b, err := io.ReadAll(req.Body)
			if err != nil {
				return ErrVal(err.Error()), nil
			}
			return StringVal(string(b)), nil
		}}
	case "json":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			defer req.Body.Close()
			b, err := io.ReadAll(req.Body)
			if err != nil {
				return ErrVal(err.Error()), nil
			}
			val, err := parseJSON(string(b))
			if err != nil {
				return ErrVal(fmt.Sprintf("json parse error: %s", err.Error())), nil
			}
			return val, nil
		}}
	case "param":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) == 0 {
				return NullVal(), nil
			}
			return StringVal(req.URL.Query().Get(args[0].Str)), nil
		}}
	case "remote_addr":
		if req.RemoteAddr != "" {
			h, _, _ := SplitHostPort(req.RemoteAddr)
			return StringVal(h)
		}
		return NullVal()
	}
	return NullVal()
}

// SplitHostPort safely splits "host:port" string into host and port
// Returns empty strings and error if format is invalid
func SplitHostPort(addr string) (host, port string, err error) {
	if addr == "" {
		return "", "", nil
	}

	host, port, err = net.SplitHostPort(addr)
	if err == nil {
		return host, port, nil
	}

	// Common fallback cases when net.SplitHostPort fails
	switch {
	case addr[0] == ':':
		// ":8080" style → host is empty
		return "", addr[1:], nil

	case addr[0] == '[':
		// IPv6 with port: "[2001:db8::1]:8080"
		// or just "[2001:db8::1]" without port
		endBracket := len(addr) - 1
		if addr[endBracket] == ']' {
			// No port
			return addr[1:endBracket], "", nil
		}
		// Has port after ]
		if addr[endBracket-1] == ']' && addr[endBracket] == ':' {
			// Actually malformed, but let's be nice
			return addr[1 : endBracket-1], "", nil
		}
	}

	// Most common case: no port at all (just IP or hostname)
	return addr, "", nil
}

// ────────────────────────────────────────────────
//           Convenience wrappers
// ────────────────────────────────────────────────

func HostOnly(addr string) string {
	h, _, _ := SplitHostPort(addr)
	return h
}

func PortOnly(addr string) string {
	_, p, _ := SplitHostPort(addr)
	return p
}

// callValue calls a vex value as a function
func callValue(fn *Value, args []*Value, ci *CallInfo) (*Value, error) {
	switch fn.Type {
	case VAL_BUILTIN:
		return fn.Builtin(args, ci)
	case VAL_FUNCTION:
		interp := NewInterpreter(fn.FnEnv)
		interp.file = ci.File
		interp.lines = ci.Lines
		env := NewEnv(fn.FnEnv)
		params := fn.Fn.Params
		if fn.FnDecl != nil {
			params = fn.FnDecl.Params
		}
		for i, param := range params {
			if i < len(args) {
				env.Def(param.Name, args[i], false)
			} else if param.Default != nil {
				res := interp.evalExpr(param.Default, env)
				if res.Err != nil {
					return nil, res.Err
				}
				env.Def(param.Name, res.Value, false)
			} else {
				env.Def(param.Name, NullVal(), false)
			}
		}
		var body *BlockStmt
		if fn.Fn != nil {
			body = fn.Fn.Body
		}
		if fn.FnDecl != nil {
			body = fn.FnDecl.Body
		}
		res := interp.execBlock(body, env)
		if res.Err != nil {
			return nil, res.Err
		}
		if res.Value != nil {
			return res.Value, nil
		}
		return NullVal(), nil
	}
	return NullVal(), fmt.Errorf("value of type %v is not callable", fn.Type)
}

// responseWriter value methods
func respWriterMethod(w http.ResponseWriter, key string) *Value {
	switch key {
	case "send":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) > 0 {
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprint(w, args[0].String())
			}
			return NullVal(), nil
		}}
	case "json":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) > 0 {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, toJSON(args[0]))
			}
			return NullVal(), nil
		}}
	case "status":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) > 0 {
				w.WriteHeader(int(args[0].Num))
			}
			return NullVal(), nil
		}}
	case "header":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) >= 2 {
				w.Header().Set(args[0].Str, args[1].Str)
			}
			return NullVal(), nil
		}}
	case "html":
		return &Value{Type: VAL_BUILTIN, Builtin: func(args []*Value, ci *CallInfo) (*Value, error) {
			if len(args) > 0 {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, args[0].String())
			}
			return NullVal(), nil
		}}
	}
	return NullVal()
}
