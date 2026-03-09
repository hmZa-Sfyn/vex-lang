package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseJSON converts a JSON string to a vex Value
func parseJSON(s string) (*Value, error) {
	var raw interface{}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, err
	}
	return jsonToValue(raw), nil
}

func jsonToValue(v interface{}) *Value {
	if v == nil {
		return NullVal()
	}
	switch val := v.(type) {
	case bool:
		return BoolVal(val)
	case float64:
		return NumberVal(val)
	case string:
		return StringVal(val)
	case []interface{}:
		arr := make([]*Value, len(val))
		for i, el := range val {
			arr[i] = jsonToValue(el)
		}
		return ArrayVal(arr)
	case map[string]interface{}:
		obj := make(map[string]*Value)
		var keys []string
		for k, el := range val {
			obj[k] = jsonToValue(el)
			keys = append(keys, k)
		}
		return ObjectVal(obj, keys)
	}
	return NullVal()
}

// toJSON converts a vex Value to a JSON string
func toJSON(v *Value) string {
	b, err := json.Marshal(valueToJSON(v))
	if err != nil {
		return "null"
	}
	return string(b)
}

// toPrettyJSON converts a vex Value to pretty-printed JSON
func toPrettyJSON(v *Value) string {
	b, err := json.MarshalIndent(valueToJSON(v), "", "  ")
	if err != nil {
		return "null"
	}
	return string(b)
}

func valueToJSON(v *Value) interface{} {
	switch v.Type {
	case VAL_NULL:
		return nil
	case VAL_BOOL:
		return v.Bool
	case VAL_NUMBER:
		return v.Num
	case VAL_STRING:
		return v.Str
	case VAL_ARRAY:
		arr := make([]interface{}, len(v.Array))
		for i, el := range v.Array {
			arr[i] = valueToJSON(el)
		}
		return arr
	case VAL_OBJECT:
		obj := make(map[string]interface{})
		for k, el := range v.Object {
			obj[k] = valueToJSON(el)
		}
		return obj
	default:
		return v.String()
	}
}

// Template string interpolation
func interpolateTemplate(template string, env *Env, interp *Interpreter) string {
	var sb strings.Builder
	i := 0
	runes := []rune(template)
	for i < len(runes) {
		if runes[i] == '$' && i+1 < len(runes) && runes[i+1] == '{' {
			// Find closing }
			i += 2
			start := i
			depth := 1
			for i < len(runes) && depth > 0 {
				if runes[i] == '{' {
					depth++
				} else if runes[i] == '}' {
					depth--
				}
				if depth > 0 {
					i++
				}
			}
			expr := string(runes[start:i])
			i++ // skip }

			// Tokenize and parse the expression
			lexer := NewLexer(expr, "<template>")
			tokens, _ := lexer.Tokenize()
			parser := NewParser(tokens, "<template>", expr)
			prog, _ := parser.Parse()

			if len(prog.Stmts) > 0 {
				if es, ok := prog.Stmts[0].(*ExprStmt); ok {
					res := interp.evalExpr(es.Expr, env)
					if res.Value != nil {
						sb.WriteString(res.Value.String())
					}
				}
			}
		} else {
			sb.WriteRune(runes[i])
			i++
		}
	}
	return sb.String()
}

// urlEncodeValues encodes a map as URL query string
func urlEncodeValues(obj map[string]*Value) string {
	var parts []string
	for k, v := range obj {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v.String()))
	}
	return strings.Join(parts, "&")
}
