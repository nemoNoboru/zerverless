package lua

import (
	"context"
	"testing"
	"time"
)

func TestRuntime_SimpleReturn(t *testing.T) {
	rt := NewRuntime()

	result, err := rt.Execute(context.Background(), "return 42", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "42" {
		t.Errorf("expected 42, got %s", result.Output)
	}
}

func TestRuntime_WithInput(t *testing.T) {
	rt := NewRuntime()

	code := `return INPUT.x + INPUT.y`
	input := map[string]any{"x": 10.0, "y": 5.0}

	result, err := rt.Execute(context.Background(), code, input, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "15" {
		t.Errorf("expected 15, got %s", result.Output)
	}
}

func TestRuntime_Print(t *testing.T) {
	rt := NewRuntime()

	code := `print("hello world")`

	result, err := rt.Execute(context.Background(), code, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", result.Output)
	}
}

func TestRuntime_SyntaxError(t *testing.T) {
	rt := NewRuntime()

	_, err := rt.Execute(context.Background(), "this is not valid lua!", nil, 5*time.Second)
	if err == nil {
		t.Error("expected syntax error")
	}
}

func TestRuntime_Timeout(t *testing.T) {
	rt := NewRuntime()

	code := `while true do end` // infinite loop

	_, err := rt.Execute(context.Background(), code, nil, 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRuntime_TableReturn(t *testing.T) {
	rt := NewRuntime()

	code := `return {result = 42, message = "ok"}`

	result, err := rt.Execute(context.Background(), code, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Table should be JSON-encoded
	if result.Output != `{"message":"ok","result":42}` {
		t.Errorf("expected JSON table, got %s", result.Output)
	}
}

func TestRuntime_HandleRequest(t *testing.T) {
	rt := NewRuntime()

	code := `
function handle(request)
  return {
    status = 200,
    headers = {["Content-Type"] = "text/plain"},
    body = "Hello " .. (request.query.name or "world")
  }
end
`
	req := &HTTPRequest{
		Method: "GET",
		Path:   "/hello",
		Query:  map[string]string{"name": "Lua"},
	}

	resp, err := rt.HandleRequest(context.Background(), code, req, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("expected 200, got %d", resp.Status)
	}
	if resp.Body != "Hello Lua" {
		t.Errorf("expected 'Hello Lua', got %s", resp.Body)
	}
}

func TestRuntime_HandleRequest_NoHandler(t *testing.T) {
	rt := NewRuntime()

	code := `-- no handle function`
	req := &HTTPRequest{Method: "GET", Path: "/"}

	_, err := rt.HandleRequest(context.Background(), code, req, 5*time.Second)
	if err == nil {
		t.Error("expected error for missing handle function")
	}
}
