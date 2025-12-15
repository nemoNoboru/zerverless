package js

import (
	"context"
	"testing"
	"time"
)

func TestRuntime_SimpleReturn(t *testing.T) {
	rt := NewRuntime()

	result, err := rt.Execute(context.Background(), "42", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "42" {
		t.Errorf("expected 42, got %s", result.Output)
	}
}

func TestRuntime_WithInput(t *testing.T) {
	rt := NewRuntime()

	code := `INPUT.x + INPUT.y`
	input := map[string]any{"x": 10.0, "y": 5.0}

	result, err := rt.Execute(context.Background(), code, input, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "15" {
		t.Errorf("expected 15, got %s", result.Output)
	}
}

func TestRuntime_ConsoleLog(t *testing.T) {
	rt := NewRuntime()

	code := `console.log("hello world")`

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

	_, err := rt.Execute(context.Background(), "this is not valid js {{{", nil, 5*time.Second)
	if err == nil {
		t.Error("expected syntax error")
	}
}

func TestRuntime_Timeout(t *testing.T) {
	rt := NewRuntime()

	code := `while(true) {}` // infinite loop

	_, err := rt.Execute(context.Background(), code, nil, 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRuntime_ObjectReturn(t *testing.T) {
	rt := NewRuntime()

	code := `({result: 42, message: "ok"})`

	result, err := rt.Execute(context.Background(), code, nil, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Object should be JSON-encoded
	if result.Output != `{"message":"ok","result":42}` {
		t.Errorf("expected JSON object, got %s", result.Output)
	}
}

func TestRuntime_ArrowFunction(t *testing.T) {
	rt := NewRuntime()

	code := `
		const add = (a, b) => a + b;
		add(INPUT.x, INPUT.y)
	`
	input := map[string]any{"x": 3.0, "y": 4.0}

	result, err := rt.Execute(context.Background(), code, input, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output != "7" {
		t.Errorf("expected 7, got %s", result.Output)
	}
}

func TestRuntime_HandleRequest(t *testing.T) {
	rt := NewRuntime()

	code := `
function handle(request) {
  return {
    status: 200,
    headers: {"Content-Type": "text/plain"},
    body: "Hello " + (request.query.name || "world")
  };
}
`
	req := &HTTPRequest{
		Method: "GET",
		Path:   "/hello",
		Query:  map[string]string{"name": "JS"},
	}

	resp, err := rt.HandleRequest(context.Background(), code, req, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != 200 {
		t.Errorf("expected 200, got %d", resp.Status)
	}
	if resp.Body != "Hello JS" {
		t.Errorf("expected 'Hello JS', got %s", resp.Body)
	}
}

func TestRuntime_HandleRequest_NoHandler(t *testing.T) {
	rt := NewRuntime()

	code := `// no handle function`
	req := &HTTPRequest{Method: "GET", Path: "/"}

	_, err := rt.HandleRequest(context.Background(), code, req, 5*time.Second)
	if err == nil {
		t.Error("expected error for missing handle function")
	}
}
