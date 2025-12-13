package wasm

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestPythonRuntime_Execute(t *testing.T) {
	// Skip if micropython.wasm not available
	micropythonPath := os.Getenv("MICROPYTHON_WASM")
	if micropythonPath == "" {
		micropythonPath = "../../bin/micropython.wasm"
	}

	if _, err := os.Stat(micropythonPath); os.IsNotExist(err) {
		t.Skip("micropython.wasm not found, skipping Python tests")
	}

	rt, err := NewPythonRuntime(micropythonPath)
	if err != nil {
		t.Fatalf("failed to create python runtime: %v", err)
	}
	defer rt.Close(context.Background())

	tests := []struct {
		name    string
		code    string
		wantOut string
	}{
		{
			name:    "hello world",
			code:    `print("hello")`,
			wantOut: "hello\n",
		},
		{
			name:    "arithmetic",
			code:    `print(2 + 3)`,
			wantOut: "5\n",
		},
		{
			name:    "loop",
			code:    `for i in range(3): print(i)`,
			wantOut: "0\n1\n2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rt.Execute(context.Background(), tt.code, 5*time.Second)
			if err != nil {
				t.Fatalf("execute error: %v", err)
			}

			if result.Error != "" {
				t.Errorf("execution error: %s", result.Error)
			}

			if result.Output != tt.wantOut {
				t.Errorf("got output %q, want %q", result.Output, tt.wantOut)
			}
		})
	}
}

func TestPythonRuntime_ExecuteWithInput(t *testing.T) {
	micropythonPath := os.Getenv("MICROPYTHON_WASM")
	if micropythonPath == "" {
		micropythonPath = "../../bin/micropython.wasm"
	}

	if _, err := os.Stat(micropythonPath); os.IsNotExist(err) {
		t.Skip("micropython.wasm not found, skipping Python tests")
	}

	rt, err := NewPythonRuntime(micropythonPath)
	if err != nil {
		t.Fatalf("failed to create python runtime: %v", err)
	}
	defer rt.Close(context.Background())

	code := `print(INPUT["a"] + INPUT["b"])`
	inputJSON := `{"a": 5, "b": 3}`

	result, err := rt.ExecuteWithInput(context.Background(), code, inputJSON, 5*time.Second)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("execution error: %s", result.Error)
	}

	if result.Output != "8\n" {
		t.Errorf("got output %q, want %q", result.Output, "8\n")
	}
}

