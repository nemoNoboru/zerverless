package js

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dop251/goja"
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
	vm := goja.New()

	// Capture console output
	var output strings.Builder
	console := vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.String()
		}
		output.WriteString(strings.Join(args, " "))
		output.WriteString("\n")
		return goja.Undefined()
	})
	vm.Set("console", console)

	// Inject INPUT
	if input != nil {
		vm.Set("INPUT", input)
	}

	// Setup timeout via interrupt
	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(timeout):
			vm.Interrupt("timeout")
		case <-ctx.Done():
			vm.Interrupt("cancelled")
		case <-done:
		}
	}()
	defer close(done)

	// Execute
	val, err := vm.RunString(code)
	if err != nil {
		return nil, err
	}

	// Get return value
	if val != nil && !goja.IsUndefined(val) && !goja.IsNull(val) {
		output.WriteString(valueToString(val))
	}

	return &Result{Output: output.String()}, nil
}

// HandleRequest executes a handler function with an HTTP request and returns an HTTP response
func (r *Runtime) HandleRequest(ctx context.Context, code string, req *HTTPRequest, timeout time.Duration) (*HTTPResponse, error) {
	vm := goja.New()

	// Setup timeout
	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(timeout):
			vm.Interrupt("timeout")
		case <-ctx.Done():
			vm.Interrupt("cancelled")
		case <-done:
		}
	}()
	defer close(done)

	// Execute code to define handle function
	if _, err := vm.RunString(code); err != nil {
		return nil, err
	}

	// Get handle function
	handleVal := vm.Get("handle")
	if handleVal == nil || goja.IsUndefined(handleVal) {
		return nil, fmt.Errorf("handle function not defined")
	}

	handleFn, ok := goja.AssertFunction(handleVal)
	if !ok {
		return nil, fmt.Errorf("handle is not a function")
	}

	// Build request object
	reqObj := vm.NewObject()
	reqObj.Set("method", req.Method)
	reqObj.Set("path", req.Path)
	reqObj.Set("body", req.Body)
	reqObj.Set("query", req.Query)
	reqObj.Set("headers", req.Headers)

	// Call handle(request)
	result, err := handleFn(goja.Undefined(), reqObj)
	if err != nil {
		return nil, err
	}

	// Parse response
	respObj := result.Export()
	respMap, ok := respObj.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("handle must return an object")
	}

	resp := &HTTPResponse{
		Status:  200,
		Headers: make(map[string]string),
	}

	if status, ok := respMap["status"].(int64); ok {
		resp.Status = int(status)
	} else if status, ok := respMap["status"].(float64); ok {
		resp.Status = int(status)
	}

	if body, ok := respMap["body"].(string); ok {
		resp.Body = body
	}

	if headers, ok := respMap["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				resp.Headers[k] = vs
			}
		}
	}

	return resp, nil
}

func valueToString(v goja.Value) string {
	exported := v.Export()

	switch val := exported.(type) {
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case string:
		return val
	case bool:
		return fmt.Sprintf("%t", val)
	case map[string]any, []any:
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", val)
	}
}

