package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Runtime struct {
	cache wazero.CompilationCache
}

type ExecutionResult struct {
	Output any    `json:"output"`
	Error  string `json:"error,omitempty"`
}

func NewRuntime() *Runtime {
	cache := wazero.NewCompilationCache()
	return &Runtime{cache: cache}
}

func (r *Runtime) Close(ctx context.Context) error {
	return r.cache.Close(ctx)
}

// Execute runs a Wasm module with the given input
func (r *Runtime) Execute(ctx context.Context, wasmBytes []byte, input map[string]any, timeout time.Duration) (*ExecutionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create runtime with cache
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(r.cache))
	defer rt.Close(ctx)

	// Instantiate WASI for basic I/O
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	// Create host functions module for input/output
	inputJSON, _ := json.Marshal(input)
	var outputData []byte

	_, err := rt.NewHostModuleBuilder("env").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) {
			// get_input: copy input JSON to Wasm memory
			mem := m.Memory()
			mem.Write(ptr, inputJSON[:min(int(size), len(inputJSON))])
		}).
		Export("get_input").
		NewFunctionBuilder().
		WithFunc(func() uint32 {
			return uint32(len(inputJSON))
		}).
		Export("get_input_len").
		NewFunctionBuilder().
		WithFunc(func(ctx context.Context, m api.Module, ptr, size uint32) {
			// set_output: read output from Wasm memory
			mem := m.Memory()
			data, _ := mem.Read(ptr, size)
			outputData = make([]byte, len(data))
			copy(outputData, data)
		}).
		Export("set_output").
		Instantiate(ctx)
	if err != nil {
		return nil, fmt.Errorf("host module: %w", err)
	}

	// Compile and instantiate the Wasm module
	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}

	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithStdout(io.Discard).
		WithStderr(io.Discard))
	if err != nil {
		return nil, fmt.Errorf("instantiate: %w", err)
	}
	defer mod.Close(ctx)

	// Call the main/run function
	run := mod.ExportedFunction("run")
	if run == nil {
		run = mod.ExportedFunction("_start")
	}
	if run == nil {
		return nil, fmt.Errorf("no 'run' or '_start' function exported")
	}

	_, err = run.Call(ctx)
	if err != nil {
		return &ExecutionResult{Error: err.Error()}, nil
	}

	// Parse output
	var output any
	if len(outputData) > 0 {
		json.Unmarshal(outputData, &output)
	}

	return &ExecutionResult{Output: output}, nil
}

// FetchWasm downloads a Wasm module from a URL
func FetchWasm(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch: status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
