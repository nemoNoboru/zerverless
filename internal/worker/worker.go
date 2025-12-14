package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	jsrt "github.com/zerverless/orchestrator/internal/js"
	luart "github.com/zerverless/orchestrator/internal/lua"
	"github.com/zerverless/orchestrator/internal/wasm"
)

type Worker struct {
	url           string
	id            string
	jobsCompleted int
	jobsFailed    int
	runtime       *wasm.Runtime
	pythonRuntime *wasm.WasmtimePythonRuntime
	pythonEnabled bool
	luaRuntime    *luart.Runtime
	jsRuntime     *jsrt.Runtime
}

type Options struct {
	PythonWasmPath string
	PythonStdlib   string
}

func New(url string) *Worker {
	return NewWithOptions(url, Options{})
}

func NewWithOptions(url string, opts Options) *Worker {
	w := &Worker{
		url:        url,
		runtime:    wasm.NewRuntime(),
		luaRuntime: luart.NewRuntime(),
		jsRuntime:  jsrt.NewRuntime(),
	}

	// Try to load Python runtime if paths provided or defaults exist
	pythonPath := opts.PythonWasmPath
	stdlibPath := opts.PythonStdlib

	if pythonPath == "" {
		pythonPath = "./python.wasm"
	}
	if stdlibPath == "" {
		stdlibPath = "./lib"
	}

	if _, err := os.Stat(pythonPath); err == nil {
		pythonRT, err := wasm.NewWasmtimePythonRuntime(pythonPath, stdlibPath)
		if err != nil {
			log.Printf("Warning: failed to load Python runtime: %v", err)
		} else {
			w.pythonRuntime = pythonRT
			w.pythonEnabled = true
			log.Printf("CPython WASI runtime enabled")
		}
	}

	return w
}

func (w *Worker) Run(ctx context.Context) error {
	defer w.runtime.Close(ctx)
	if w.pythonRuntime != nil {
		defer w.pythonRuntime.Close(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := w.connect(ctx); err != nil {
				log.Printf("Connection error: %v, reconnecting in 5s...", err)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
			}
		}
	}
}

func (w *Worker) connect(ctx context.Context) error {
	log.Printf("Connecting to %s...", w.url)

	conn, _, err := websocket.Dial(ctx, w.url, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "goodbye")

	// Wait for ack
	var ack AckMessage
	if err := wsjson.Read(ctx, conn, &ack); err != nil {
		return fmt.Errorf("read ack: %w", err)
	}

	w.id = ack.VolunteerID
	log.Printf("Connected! ID: %s", w.id)

	// Send ready
	if err := w.sendReady(ctx, conn); err != nil {
		return fmt.Errorf("send ready: %w", err)
	}

	// Start heartbeat
	go w.heartbeat(ctx, conn)

	// Message loop
	return w.messageLoop(ctx, conn)
}

func (w *Worker) messageLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		var base BaseMessage
		if err := json.Unmarshal(data, &base); err != nil {
			log.Printf("Invalid message: %v", err)
			continue
		}

		switch base.Type {
		case "job":
			var job JobMessage
			json.Unmarshal(data, &job)
			w.executeJob(ctx, conn, job)

		case "cancel":
			log.Printf("Job cancelled")

		case "heartbeat":
			// Server acknowledged

		default:
			log.Printf("Unknown message type: %s", base.Type)
		}
	}
}

func (w *Worker) executeJob(ctx context.Context, conn *websocket.Conn, job JobMessage) {
	log.Printf("Executing job: %s (type: %s)", job.JobID[:8], job.JobType)

	timeout := time.Duration(job.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var result *wasm.ExecutionResult
	var err error

	switch job.JobType {
	case "js", "javascript":
		result, err = w.executeJS(ctx, job, timeout)
	case "lua":
		result, err = w.executeLua(ctx, job, timeout)
	case "python", "py":
		result, err = w.executePython(ctx, job, timeout)
	case "wasm", "":
		result, err = w.executeWasm(ctx, job, timeout)
	default:
		w.sendError(ctx, conn, job.JobID, fmt.Sprintf("unknown job type: %s", job.JobType))
		return
	}

	if err != nil {
		w.sendError(ctx, conn, job.JobID, err.Error())
		return
	}

	if result.Error != "" {
		w.sendError(ctx, conn, job.JobID, result.Error)
		return
	}

	// Send result
	msg := ResultMessage{
		Type:   "result",
		JobID:  job.JobID,
		Result: result.Output,
	}

	if err := wsjson.Write(ctx, conn, msg); err != nil {
		log.Printf("Failed to send result: %v", err)
		w.jobsFailed++
		return
	}

	w.jobsCompleted++
	log.Printf("Job completed: %s (total: %d)", job.JobID[:8], w.jobsCompleted)

	// Ready for next job
	w.sendReady(ctx, conn)
}

func (w *Worker) executeWasm(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
	// Fetch Wasm module
	wasmBytes, err := wasm.FetchWasm(ctx, job.WasmURL)
	if err != nil {
		return nil, fmt.Errorf("fetch wasm: %w", err)
	}

	log.Printf("Fetched Wasm module: %d bytes", len(wasmBytes))

	return w.runtime.Execute(ctx, wasmBytes, job.InputData, timeout)
}

func (w *Worker) executePython(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
	if !w.pythonEnabled {
		return nil, fmt.Errorf("python runtime not available")
	}

	if job.Code == "" {
		return nil, fmt.Errorf("no python code provided")
	}

	log.Printf("Executing Python code (%d bytes)", len(job.Code))

	// Convert input to JSON
	inputJSON := "{}"
	if job.InputData != nil {
		b, _ := json.Marshal(job.InputData)
		inputJSON = string(b)
	}

	return w.pythonRuntime.ExecuteWithInput(ctx, job.Code, inputJSON, timeout)
}

func (w *Worker) executeLua(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
	if job.Code == "" {
		return nil, fmt.Errorf("no lua code provided")
	}

	log.Printf("Executing Lua code (%d bytes)", len(job.Code))

	result, err := w.luaRuntime.Execute(ctx, job.Code, job.InputData, timeout)
	if err != nil {
		return &wasm.ExecutionResult{Error: err.Error()}, nil
	}

	return &wasm.ExecutionResult{Output: result.Output}, nil
}

func (w *Worker) executeJS(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
	if job.Code == "" {
		return nil, fmt.Errorf("no javascript code provided")
	}

	log.Printf("Executing JavaScript code (%d bytes)", len(job.Code))

	result, err := w.jsRuntime.Execute(ctx, job.Code, job.InputData, timeout)
	if err != nil {
		return &wasm.ExecutionResult{Error: err.Error()}, nil
	}

	return &wasm.ExecutionResult{Output: result.Output}, nil
}

func (w *Worker) sendError(ctx context.Context, conn *websocket.Conn, jobID, errMsg string) {
	log.Printf("Job %s failed: %s", jobID[:8], errMsg)

	msg := ErrorMessage{
		Type:  "error",
		JobID: jobID,
		Error: errMsg,
	}
	wsjson.Write(ctx, conn, msg)
	w.jobsFailed++
	w.sendReady(ctx, conn)
}

func (w *Worker) sendReady(ctx context.Context, conn *websocket.Conn) error {
	msg := ReadyMessage{
		Type: "ready",
		Capabilities: &Capabilities{
			Wasm:        true,
			Python:      w.pythonEnabled,
			Lua:         true, // Always available (pure Go)
			JS:          true, // Always available (pure Go)
			MaxMemoryMB: 256,
		},
	}
	return wsjson.Write(ctx, conn, msg)
}

func (w *Worker) heartbeat(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msg := HeartbeatMessage{Type: "heartbeat"}
			if err := wsjson.Write(ctx, conn, msg); err != nil {
				return
			}
		}
	}
}

// Message types

type BaseMessage struct {
	Type string `json:"type"`
}

type Capabilities struct {
	Wasm        bool `json:"wasm"`
	Python      bool `json:"python"`
	Lua         bool `json:"lua"`
	JS          bool `json:"js"`
	MaxMemoryMB int  `json:"max_memory_mb"`
}

type ReadyMessage struct {
	Type         string        `json:"type"`
	Capabilities *Capabilities `json:"capabilities,omitempty"`
}

type AckMessage struct {
	Type        string `json:"type"`
	VolunteerID string `json:"volunteer_id"`
	Message     string `json:"message"`
}

type JobMessage struct {
	Type           string         `json:"type"`
	JobID          string         `json:"job_id"`
	JobType        string         `json:"job_type"` // "wasm" or "python"
	WasmCID        string         `json:"wasm_cid,omitempty"`
	WasmURL        string         `json:"wasm_url,omitempty"`
	Code           string         `json:"code,omitempty"` // Python code
	InputData      map[string]any `json:"input_data"`
	TimeoutSeconds int            `json:"timeout_seconds"`
}

type ResultMessage struct {
	Type   string `json:"type"`
	JobID  string `json:"job_id"`
	Result any    `json:"result"`
}

type ErrorMessage struct {
	Type  string `json:"type"`
	JobID string `json:"job_id"`
	Error string `json:"error"`
}

type HeartbeatMessage struct {
	Type string `json:"type"`
}
