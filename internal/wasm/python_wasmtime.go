package wasm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v39"
)

// WasmtimePythonRuntime executes Python code using wasmtime-go (handles large WASM)
type WasmtimePythonRuntime struct {
	engine    *wasmtime.Engine
	module    *wasmtime.Module
	stdlibDir string // Path to Python stdlib
}

// NewWasmtimePythonRuntime creates a Python runtime using wasmtime-go
// stdlibDir should point to the Python lib directory (containing encodings/, etc)
func NewWasmtimePythonRuntime(pythonWasmPath, stdlibDir string) (*WasmtimePythonRuntime, error) {
	wasmBytes, err := os.ReadFile(pythonWasmPath)
	if err != nil {
		return nil, fmt.Errorf("read python.wasm: %w", err)
	}

	// Verify stdlib exists
	if _, err := os.Stat(filepath.Join(stdlibDir, "encodings")); os.IsNotExist(err) {
		return nil, fmt.Errorf("stdlib not found at %s (missing encodings/)", stdlibDir)
	}

	engine := wasmtime.NewEngine()

	module, err := wasmtime.NewModule(engine, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile python module: %w", err)
	}

	return &WasmtimePythonRuntime{
		engine:    engine,
		module:    module,
		stdlibDir: stdlibDir,
	}, nil
}

func (p *WasmtimePythonRuntime) Close(ctx context.Context) error {
	return nil
}

// Execute runs Python code and returns stdout
func (p *WasmtimePythonRuntime) Execute(ctx context.Context, code string, timeout time.Duration) (*ExecutionResult, error) {
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

	// Create stdout/stderr capture files
	stdoutPath := filepath.Join(tmpDir, "stdout")
	stderrPath := filepath.Join(tmpDir, "stderr")

	// Create WASI config
	wasiConfig := wasmtime.NewWasiConfig()
	wasiConfig.SetArgv([]string{"python", "/tmp/script.py"})
	wasiConfig.SetEnv(
		[]string{"PYTHONDONTWRITEBYTECODE", "PYTHONUNBUFFERED", "PYTHONHOME", "PYTHONPATH"},
		[]string{"1", "1", "/usr", "/usr/lib"},
	)

	// Mount temp dir for script
	if err := wasiConfig.PreopenDir(tmpDir, "/tmp", wasmtime.DIR_READ|wasmtime.DIR_WRITE, wasmtime.FILE_READ|wasmtime.FILE_WRITE); err != nil {
		return nil, fmt.Errorf("preopen tmp: %w", err)
	}

	// Mount stdlib at /usr/lib (standard Python location)
	if err := wasiConfig.PreopenDir(p.stdlibDir, "/usr/lib", wasmtime.DIR_READ, wasmtime.FILE_READ); err != nil {
		return nil, fmt.Errorf("preopen stdlib: %w", err)
	}

	wasiConfig.SetStdoutFile(stdoutPath)
	wasiConfig.SetStderrFile(stderrPath)

	// Create store with WASI
	store := wasmtime.NewStore(p.engine)
	store.SetWasi(wasiConfig)

	// Create linker and add WASI
	linker := wasmtime.NewLinker(p.engine)
	if err := linker.DefineWasi(); err != nil {
		return nil, fmt.Errorf("define wasi: %w", err)
	}

	// Instantiate and run
	instance, err := linker.Instantiate(store, p.module)
	if err != nil {
		return nil, fmt.Errorf("instantiate: %w", err)
	}

	// Get _start function (WASI entry point)
	start := instance.GetFunc(store, "_start")
	if start == nil {
		return nil, fmt.Errorf("no _start function found")
	}

	// Run with timeout
	done := make(chan error, 1)
	go func() {
		_, err := start.Call(store)
		done <- err
	}()

	select {
	case <-time.After(timeout):
		return &ExecutionResult{Error: "timeout"}, nil
	case runErr := <-done:
		// Read stdout/stderr
		stdout, _ := os.ReadFile(stdoutPath)
		stderr, _ := os.ReadFile(stderrPath)

		if runErr != nil {
			// Check if we got output anyway
			if len(stdout) > 0 || len(stderr) > 0 {
				if len(stderr) > 0 {
					return &ExecutionResult{
						Output: string(stdout),
						Error:  string(stderr),
					}, nil
				}
				return &ExecutionResult{Output: string(stdout)}, nil
			}
			return nil, fmt.Errorf("run: %w", runErr)
		}

		if len(stderr) > 0 {
			return &ExecutionResult{
				Output: string(stdout),
				Error:  string(stderr),
			}, nil
		}

		return &ExecutionResult{Output: string(stdout)}, nil
	}
}

// ExecuteInline runs Python code inline using -c flag
func (p *WasmtimePythonRuntime) ExecuteInline(ctx context.Context, code string, timeout time.Duration) (*ExecutionResult, error) {
	// Create temp directory for stdout/stderr capture
	tmpDir, err := os.MkdirTemp("", "zerverless-python-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	stdoutPath := filepath.Join(tmpDir, "stdout")
	stderrPath := filepath.Join(tmpDir, "stderr")

	// Create WASI config with -c flag
	wasiConfig := wasmtime.NewWasiConfig()
	wasiConfig.SetArgv([]string{"python", "-c", code})
	wasiConfig.SetEnv(
		[]string{"PYTHONDONTWRITEBYTECODE", "PYTHONUNBUFFERED", "PYTHONHOME", "PYTHONPATH"},
		[]string{"1", "1", "/usr", "/usr/lib"},
	)

	// Mount stdlib
	if err := wasiConfig.PreopenDir(p.stdlibDir, "/usr/lib", wasmtime.DIR_READ, wasmtime.FILE_READ); err != nil {
		return nil, fmt.Errorf("preopen stdlib: %w", err)
	}

	wasiConfig.SetStdoutFile(stdoutPath)
	wasiConfig.SetStderrFile(stderrPath)

	store := wasmtime.NewStore(p.engine)
	store.SetWasi(wasiConfig)

	linker := wasmtime.NewLinker(p.engine)
	if err := linker.DefineWasi(); err != nil {
		return nil, fmt.Errorf("define wasi: %w", err)
	}

	instance, err := linker.Instantiate(store, p.module)
	if err != nil {
		return nil, fmt.Errorf("instantiate: %w", err)
	}

	start := instance.GetFunc(store, "_start")
	if start == nil {
		return nil, fmt.Errorf("no _start function found")
	}

	done := make(chan error, 1)
	go func() {
		_, err := start.Call(store)
		done <- err
	}()

	select {
	case <-time.After(timeout):
		return &ExecutionResult{Error: "timeout"}, nil
	case runErr := <-done:
		stdout, _ := os.ReadFile(stdoutPath)
		stderr, _ := os.ReadFile(stderrPath)

		if runErr != nil {
			if len(stdout) > 0 || len(stderr) > 0 {
				if len(stderr) > 0 {
					return &ExecutionResult{
						Output: string(stdout),
						Error:  string(stderr),
					}, nil
				}
				return &ExecutionResult{Output: string(stdout)}, nil
			}
			return nil, fmt.Errorf("run: %w", runErr)
		}

		if len(stderr) > 0 {
			return &ExecutionResult{
				Output: string(stdout),
				Error:  string(stderr),
			}, nil
		}

		return &ExecutionResult{Output: string(stdout)}, nil
	}
}

// ExecuteWithInput runs Python code with JSON input available as `INPUT` variable
func (p *WasmtimePythonRuntime) ExecuteWithInput(ctx context.Context, code string, inputJSON string, timeout time.Duration) (*ExecutionResult, error) {
	wrappedCode := fmt.Sprintf(`import json
INPUT = json.loads(%q)
%s`, inputJSON, code)

	return p.Execute(ctx, wrappedCode, timeout)
}
