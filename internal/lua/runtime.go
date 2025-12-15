package lua

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

type Result struct {
	Output string
}

type HTTPRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type HTTPResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type Runtime struct{}

func NewRuntime() *Runtime {
	return &Runtime{}
}

func (r *Runtime) Execute(ctx context.Context, code string, input map[string]any, timeout time.Duration) (*Result, error) {
	L := lua.NewState()
	defer L.Close()

	// Capture stdout
	var output strings.Builder
	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		n := L.GetTop()
		for i := 1; i <= n; i++ {
			if i > 1 {
				output.WriteString("\t")
			}
			output.WriteString(L.ToStringMeta(L.Get(i)).String())
		}
		output.WriteString("\n")
		return 0
	}))

	// Inject INPUT table
	if input != nil {
		inputTable := L.NewTable()
		for k, v := range input {
			L.SetField(inputTable, k, goToLua(L, v))
		}
		L.SetGlobal("INPUT", inputTable)
	}

	// Setup timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	L.SetContext(ctx)

	// Execute
	if err := L.DoString(code); err != nil {
		return nil, err
	}

	// Get return value if any
	ret := L.Get(-1)
	if ret != lua.LNil {
		output.WriteString(luaToString(ret))
	}

	return &Result{Output: output.String()}, nil
}

// HandleRequest executes a handler function with an HTTP request and returns an HTTP response
func (r *Runtime) HandleRequest(ctx context.Context, code string, req *HTTPRequest, timeout time.Duration) (*HTTPResponse, error) {
	L := lua.NewState()
	defer L.Close()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	L.SetContext(ctx)

	// Execute code to define handle function
	if err := L.DoString(code); err != nil {
		return nil, err
	}

	// Check handle function exists
	handleFn := L.GetGlobal("handle")
	if handleFn == lua.LNil {
		return nil, fmt.Errorf("handle function not defined")
	}

	// Build request table
	reqTable := L.NewTable()
	L.SetField(reqTable, "method", lua.LString(req.Method))
	L.SetField(reqTable, "path", lua.LString(req.Path))
	L.SetField(reqTable, "body", lua.LString(req.Body))

	queryTable := L.NewTable()
	for k, v := range req.Query {
		L.SetField(queryTable, k, lua.LString(v))
	}
	L.SetField(reqTable, "query", queryTable)

	headersTable := L.NewTable()
	for k, v := range req.Headers {
		L.SetField(headersTable, k, lua.LString(v))
	}
	L.SetField(reqTable, "headers", headersTable)

	// Call handle(request)
	L.Push(handleFn)
	L.Push(reqTable)
	if err := L.PCall(1, 1, nil); err != nil {
		return nil, err
	}

	// Parse response table
	respVal := L.Get(-1)
	respTable, ok := respVal.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("handle must return a table")
	}

	resp := &HTTPResponse{
		Status:  200,
		Headers: make(map[string]string),
	}

	if status := respTable.RawGetString("status"); status != lua.LNil {
		resp.Status = int(status.(lua.LNumber))
	}
	if body := respTable.RawGetString("body"); body != lua.LNil {
		resp.Body = body.String()
	}
	if headers := respTable.RawGetString("headers"); headers != lua.LNil {
		if ht, ok := headers.(*lua.LTable); ok {
			ht.ForEach(func(k, v lua.LValue) {
				resp.Headers[k.String()] = v.String()
			})
		}
	}

	return resp, nil
}

func goToLua(L *lua.LState, v any) lua.LValue {
	switch val := v.(type) {
	case float64:
		return lua.LNumber(val)
	case int:
		return lua.LNumber(float64(val))
	case string:
		return lua.LString(val)
	case bool:
		return lua.LBool(val)
	case map[string]any:
		t := L.NewTable()
		for k, v := range val {
			L.SetField(t, k, goToLua(L, v))
		}
		return t
	case []any:
		t := L.NewTable()
		for i, v := range val {
			L.SetTable(t, lua.LNumber(i+1), goToLua(L, v))
		}
		return t
	default:
		return lua.LNil
	}
}

func luaToString(v lua.LValue) string {
	switch val := v.(type) {
	case lua.LNumber:
		n := float64(val)
		if n == float64(int(n)) {
			return fmt.Sprintf("%d", int(n))
		}
		return fmt.Sprintf("%g", n)
	case lua.LString:
		return string(val)
	case lua.LBool:
		return fmt.Sprintf("%t", bool(val))
	case *lua.LTable:
		return tableToJSON(val)
	default:
		return ""
	}
}

func tableToJSON(t *lua.LTable) string {
	m := make(map[string]any)
	t.ForEach(func(k, v lua.LValue) {
		key := k.String()
		m[key] = luaToGo(v)
	})
	b, _ := json.Marshal(m)
	return string(b)
}

func luaToGo(v lua.LValue) any {
	switch val := v.(type) {
	case lua.LNumber:
		return float64(val)
	case lua.LString:
		return string(val)
	case lua.LBool:
		return bool(val)
	case *lua.LTable:
		m := make(map[string]any)
		val.ForEach(func(k, v lua.LValue) {
			m[k.String()] = luaToGo(v)
		})
		return m
	default:
		return nil
	}
}
