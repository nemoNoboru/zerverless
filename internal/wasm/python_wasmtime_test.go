package wasm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func getPythonPaths() (wasmPath, stdlibPath string) {
	// Get wasm path - try env var first, then common locations
	if p := os.Getenv("MICROPYTHON_WASM"); p != "" {
		// Convert to absolute if relative
		if !filepath.IsAbs(p) {
			if abs, err := filepath.Abs(p); err == nil {
				wasmPath = abs
			} else {
				wasmPath = p
			}
		} else {
			wasmPath = p
		}
	} else {
		paths := []string{"../../python.wasm", "./python.wasm"}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				wasmPath, _ = filepath.Abs(p)
				break
			}
		}
	}

	// Get stdlib path
	if p := os.Getenv("PYTHON_STDLIB"); p != "" {
		if !filepath.IsAbs(p) {
			if abs, err := filepath.Abs(p); err == nil {
				stdlibPath = abs
			} else {
				stdlibPath = p
			}
		} else {
			stdlibPath = p
		}
	} else {
		paths := []string{"../../lib", "./lib"}
		for _, p := range paths {
			if _, err := os.Stat(filepath.Join(p, "encodings")); err == nil {
				stdlibPath, _ = filepath.Abs(p)
				break
			}
		}
	}

	return
}

func TestWasmtimePythonRuntime_ExecuteInline(t *testing.T) {
	wasmPath, stdlibPath := getPythonPaths()
	if wasmPath == "" {
		t.Skip("python.wasm not found")
	}
	if stdlibPath == "" {
		t.Skip("python stdlib not found")
	}

	rt, err := NewWasmtimePythonRuntime(wasmPath, stdlibPath)
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}
	defer rt.Close(context.Background())

	result, err := rt.ExecuteInline(context.Background(), `print("hello")`, 30*time.Second)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Error != "" {
		t.Logf("stderr: %s", result.Error)
	}

	output := strings.TrimSpace(result.Output.(string))
	if output != "hello" {
		t.Errorf("expected 'hello', got %q", output)
	}
}

func TestWasmtimePythonRuntime_Execute(t *testing.T) {
	wasmPath, stdlibPath := getPythonPaths()
	if wasmPath == "" {
		t.Skip("python.wasm not found")
	}
	if stdlibPath == "" {
		t.Skip("python stdlib not found")
	}

	rt, err := NewWasmtimePythonRuntime(wasmPath, stdlibPath)
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
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
			wantOut: "hello",
		},
		{
			name:    "arithmetic",
			code:    `print(2 + 3)`,
			wantOut: "5",
		},
		{
			name:    "loop",
			code:    "for i in range(3):\n    print(i)",
			wantOut: "0\n1\n2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rt.Execute(context.Background(), tt.code, 30*time.Second)
			if err != nil {
				t.Fatalf("execute error: %v", err)
			}

			if result.Error != "" {
				t.Logf("stderr: %s", result.Error)
			}

			output := strings.TrimSpace(result.Output.(string))
			want := strings.TrimSpace(tt.wantOut)
			if output != want {
				t.Errorf("got output %q, want %q", output, want)
			}
		})
	}
}

func TestWasmtimePythonRuntime_ExecuteWithInput(t *testing.T) {
	wasmPath, stdlibPath := getPythonPaths()
	if wasmPath == "" {
		t.Skip("python.wasm not found")
	}
	if stdlibPath == "" {
		t.Skip("python stdlib not found")
	}

	rt, err := NewWasmtimePythonRuntime(wasmPath, stdlibPath)
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}
	defer rt.Close(context.Background())

	code := `print(INPUT["a"] + INPUT["b"])`
	inputJSON := `{"a": 5, "b": 3}`

	result, err := rt.ExecuteWithInput(context.Background(), code, inputJSON, 30*time.Second)
	if err != nil {
		t.Fatalf("execute error: %v", err)
	}

	if result.Error != "" {
		t.Logf("stderr: %s", result.Error)
	}

	output := strings.TrimSpace(result.Output.(string))
	if output != "8" {
		t.Errorf("got output %q, want %q", output, "8")
	}
}
