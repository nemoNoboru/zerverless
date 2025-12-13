package wasm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// PythonRuntime executes Python code using MicroPython WASM
type PythonRuntime struct {
	micropythonWasm []byte
	cache           wazero.CompilationCache
}

// NewPythonRuntime creates a Python runtime
// micropythonPath should point to the micropython.wasm file
func NewPythonRuntime(micropythonPath string) (*PythonRuntime, error) {
	wasmBytes, err := os.ReadFile(micropythonPath)
	if err != nil {
		return nil, fmt.Errorf("read micropython.wasm: %w", err)
	}

	return &PythonRuntime{
		micropythonWasm: wasmBytes,
		cache:           wazero.NewCompilationCache(),
	}, nil
}

// NewPythonRuntimeFromBytes creates a Python runtime from wasm bytes
func NewPythonRuntimeFromBytes(wasmBytes []byte) *PythonRuntime {
	return &PythonRuntime{
		micropythonWasm: wasmBytes,
		cache:           wazero.NewCompilationCache(),
	}
}

func (p *PythonRuntime) Close(ctx context.Context) error {
	return p.cache.Close(ctx)
}

// Execute runs Python code and returns stdout
func (p *PythonRuntime) Execute(ctx context.Context, code string, timeout time.Duration) (*ExecutionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create runtime
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithCompilationCache(p.cache))
	defer rt.Close(ctx)

	// Instantiate WASI
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	// Create temp directory for the script
	tmpDir, err := os.MkdirTemp("", "zerverless-python-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write Python script to temp file
	scriptPath := filepath.Join(tmpDir, "script.py")
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("write script: %w", err)
	}

	// Capture stdout/stderr
	var stdout, stderr bytes.Buffer

	// Compile MicroPython
	compiled, err := rt.CompileModule(ctx, p.micropythonWasm)
	if err != nil {
		return nil, fmt.Errorf("compile micropython: %w", err)
	}

	// Configure WASI with filesystem access and args
	config := wazero.NewModuleConfig().
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithArgs("micropython", "/tmp/script.py").
		WithFSConfig(wazero.NewFSConfig().
			WithDirMount(tmpDir, "/tmp"))

	// Run
	_, err = rt.InstantiateModule(ctx, compiled, config)
	if err != nil {
		// Check if it's just a normal exit
		if ctx.Err() == context.DeadlineExceeded {
			return &ExecutionResult{Error: "timeout"}, nil
		}
		// MicroPython exits with code, which wazero treats as error
		// Check if we got output anyway
		if stdout.Len() > 0 || stderr.Len() > 0 {
			if stderr.Len() > 0 {
				return &ExecutionResult{
					Output: stdout.String(),
					Error:  stderr.String(),
				}, nil
			}
			return &ExecutionResult{Output: stdout.String()}, nil
		}
		return nil, fmt.Errorf("run: %w", err)
	}

	if stderr.Len() > 0 {
		return &ExecutionResult{
			Output: stdout.String(),
			Error:  stderr.String(),
		}, nil
	}

	return &ExecutionResult{Output: stdout.String()}, nil
}

// ExecuteWithInput runs Python code with JSON input available as `INPUT` variable
func (p *PythonRuntime) ExecuteWithInput(ctx context.Context, code string, inputJSON string, timeout time.Duration) (*ExecutionResult, error) {
	// Wrap code to inject input
	wrappedCode := fmt.Sprintf(`import json
INPUT = json.loads(%q)
%s`, inputJSON, code)

	return p.Execute(ctx, wrappedCode, timeout)
}

