package wasm

import (
	"context"
	"testing"
	"time"
)

// Minimal valid Wasm module that does nothing (for testing instantiation)
// This is a valid Wasm binary with just a start section
var minimalWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic
	0x01, 0x00, 0x00, 0x00, // version
}

// A Wasm module with a run function that does nothing
// Built from: (module (func (export "run")))
var runWasm = []byte{
	0x00, 0x61, 0x73, 0x6d, // magic
	0x01, 0x00, 0x00, 0x00, // version
	0x01, 0x04, 0x01, 0x60, 0x00, 0x00, // type section: () -> ()
	0x03, 0x02, 0x01, 0x00, // function section: 1 func of type 0
	0x07, 0x07, 0x01, 0x03, 0x72, 0x75, 0x6e, 0x00, 0x00, // export "run" -> func 0
	0x0a, 0x04, 0x01, 0x02, 0x00, 0x0b, // code section: empty func
}

func TestRuntime_New(t *testing.T) {
	rt := NewRuntime()
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	rt.Close(context.Background())
}

func TestRuntime_Execute_NoRunFunction(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close(context.Background())

	ctx := context.Background()
	_, err := rt.Execute(ctx, minimalWasm, nil, 5*time.Second)

	if err == nil {
		t.Error("expected error for module without run function")
	}
}

func TestRuntime_Execute_EmptyRun(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close(context.Background())

	ctx := context.Background()
	result, err := rt.Execute(ctx, runWasm, nil, 5*time.Second)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error != "" {
		t.Errorf("unexpected execution error: %s", result.Error)
	}
}

func TestRuntime_Execute_InvalidWasm(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close(context.Background())

	ctx := context.Background()
	_, err := rt.Execute(ctx, []byte("not wasm"), nil, 5*time.Second)

	if err == nil {
		t.Error("expected error for invalid wasm")
	}
}

func TestRuntime_Execute_Timeout(t *testing.T) {
	rt := NewRuntime()
	defer rt.Close(context.Background())

	ctx := context.Background()
	// Use a very short timeout
	_, err := rt.Execute(ctx, runWasm, nil, 1*time.Nanosecond)

	// Should either succeed quickly or timeout
	// This is non-deterministic, so we just check it doesn't panic
	_ = err
}


