package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Mock orchestrator for testing
type mockOrchestrator struct {
	t             *testing.T
	server        *httptest.Server
	connections   []*websocket.Conn
	mu            sync.Mutex
	jobResults    chan ResultMessage
	jobErrors     chan ErrorMessage
	readyReceived chan ReadyMessage
}

func newMockOrchestrator(t *testing.T) *mockOrchestrator {
	m := &mockOrchestrator{
		t:             t,
		jobResults:    make(chan ResultMessage, 10),
		jobErrors:     make(chan ErrorMessage, 10),
		readyReceived: make(chan ReadyMessage, 10),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/volunteer", m.handleVolunteer)
	m.server = httptest.NewServer(mux)

	return m
}

func (m *mockOrchestrator) handleVolunteer(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		m.t.Logf("accept error: %v", err)
		return
	}

	m.mu.Lock()
	m.connections = append(m.connections, conn)
	m.mu.Unlock()

	// Send ack
	ack := AckMessage{
		Type:        "ack",
		VolunteerID: "test-volunteer-123",
		Message:     "Welcome!",
	}
	wsjson.Write(r.Context(), conn, ack)

	// Read messages
	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}

		var base BaseMessage
		json.Unmarshal(data, &base)

		switch base.Type {
		case "ready":
			var msg ReadyMessage
			json.Unmarshal(data, &msg)
			m.readyReceived <- msg
		case "result":
			var msg ResultMessage
			json.Unmarshal(data, &msg)
			m.jobResults <- msg
		case "error":
			var msg ErrorMessage
			json.Unmarshal(data, &msg)
			m.jobErrors <- msg
		}
	}
}

func (m *mockOrchestrator) sendJob(job JobMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, conn := range m.connections {
		wsjson.Write(context.Background(), conn, job)
	}
}

func (m *mockOrchestrator) wsURL() string {
	return "ws" + strings.TrimPrefix(m.server.URL, "http") + "/ws/volunteer"
}

func (m *mockOrchestrator) close() {
	m.mu.Lock()
	for _, conn := range m.connections {
		conn.Close(websocket.StatusNormalClosure, "")
	}
	m.mu.Unlock()
	m.server.Close()
}

func getPythonTestPaths() (wasmPath, stdlibPath string, ok bool) {
	// Try env vars first
	wasmPath = os.Getenv("MICROPYTHON_WASM")
	stdlibPath = os.Getenv("PYTHON_STDLIB")

	// Try common locations
	if wasmPath == "" {
		paths := []string{"../../python.wasm", "./python.wasm"}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				wasmPath, _ = filepath.Abs(p)
				break
			}
		}
	}

	if stdlibPath == "" {
		paths := []string{"../../lib", "./lib"}
		for _, p := range paths {
			if _, err := os.Stat(filepath.Join(p, "encodings")); err == nil {
				stdlibPath, _ = filepath.Abs(p)
				break
			}
		}
	}

	ok = wasmPath != "" && stdlibPath != ""
	return
}

func TestWorker_ConnectsAndSendsReady(t *testing.T) {
	mock := newMockOrchestrator(t)
	defer mock.close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	w := New(mock.wsURL())

	// Run worker in background
	go w.Run(ctx)

	// Wait for ready message
	select {
	case ready := <-mock.readyReceived:
		if !ready.Capabilities.Wasm {
			t.Error("expected wasm capability")
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for ready message")
	}
}

func TestWorker_ExecutesPythonJob(t *testing.T) {
	wasmPath, stdlibPath, ok := getPythonTestPaths()
	if !ok {
		t.Skip("python.wasm or stdlib not found")
	}

	mock := newMockOrchestrator(t)
	defer mock.close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	w := NewWithOptions(mock.wsURL(), Options{
		PythonWasmPath: wasmPath,
		PythonStdlib:   stdlibPath,
	})

	// Run worker in background
	go w.Run(ctx)

	// Wait for ready
	select {
	case ready := <-mock.readyReceived:
		if !ready.Capabilities.Python {
			t.Fatal("expected python capability")
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for ready")
	}

	// Send Python job
	job := JobMessage{
		Type:           "job",
		JobID:          "test-job-001",
		JobType:        "python",
		Code:           `print(INPUT["a"] + INPUT["b"])`,
		InputData:      map[string]any{"a": 5, "b": 3},
		TimeoutSeconds: 10,
	}
	mock.sendJob(job)

	// Wait for result
	select {
	case result := <-mock.jobResults:
		if result.JobID != "test-job-001" {
			t.Errorf("wrong job id: %s", result.JobID)
		}
		output, ok := result.Result.(string)
		if !ok {
			t.Fatalf("expected string result, got %T", result.Result)
		}
		if strings.TrimSpace(output) != "8" {
			t.Errorf("expected '8', got %q", output)
		}
	case err := <-mock.jobErrors:
		t.Fatalf("job failed: %s", err.Error)
	case <-ctx.Done():
		t.Fatal("timeout waiting for result")
	}
}

func TestWorker_PythonWithComplexInput(t *testing.T) {
	wasmPath, stdlibPath, ok := getPythonTestPaths()
	if !ok {
		t.Skip("python.wasm or stdlib not found")
	}

	mock := newMockOrchestrator(t)
	defer mock.close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	w := NewWithOptions(mock.wsURL(), Options{
		PythonWasmPath: wasmPath,
		PythonStdlib:   stdlibPath,
	})
	go w.Run(ctx)

	// Wait for ready
	<-mock.readyReceived

	// Fibonacci job
	job := JobMessage{
		Type:    "job",
		JobID:   "fib-job-001",
		JobType: "python",
		Code: `
def fib(n):
    if n <= 1:
        return n
    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b
    return b

n = INPUT.get("n", 10)
print(fib(n))
`,
		InputData:      map[string]any{"n": 10},
		TimeoutSeconds: 10,
	}
	mock.sendJob(job)

	select {
	case result := <-mock.jobResults:
		output := strings.TrimSpace(result.Result.(string))
		if output != "55" {
			t.Errorf("fib(10) should be 55, got %s", output)
		}
	case err := <-mock.jobErrors:
		t.Fatalf("job failed: %s", err.Error)
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}

func TestWorker_PythonSyntaxError(t *testing.T) {
	wasmPath, stdlibPath, ok := getPythonTestPaths()
	if !ok {
		t.Skip("python.wasm or stdlib not found")
	}

	mock := newMockOrchestrator(t)
	defer mock.close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	w := NewWithOptions(mock.wsURL(), Options{
		PythonWasmPath: wasmPath,
		PythonStdlib:   stdlibPath,
	})
	go w.Run(ctx)

	<-mock.readyReceived

	// Invalid Python syntax
	job := JobMessage{
		Type:           "job",
		JobID:          "bad-job-001",
		JobType:        "python",
		Code:           `def broken(`,
		InputData:      nil,
		TimeoutSeconds: 10,
	}
	mock.sendJob(job)

	select {
	case <-mock.jobResults:
		// Might still get result with error in stderr
	case err := <-mock.jobErrors:
		if err.Error == "" {
			t.Error("expected error message")
		}
		t.Logf("Got expected error: %s", err.Error)
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}

func TestWorker_PythonNotEnabled(t *testing.T) {
	mock := newMockOrchestrator(t)
	defer mock.close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create worker without valid paths
	w := NewWithOptions(mock.wsURL(), Options{
		PythonWasmPath: "/nonexistent/path.wasm",
		PythonStdlib:   "/nonexistent/lib",
	})
	go w.Run(ctx)

	// Wait for ready - should not have python capability
	select {
	case ready := <-mock.readyReceived:
		if ready.Capabilities.Python {
			t.Error("should not have python capability without python.wasm")
		}
	case <-ctx.Done():
		t.Fatal("timeout")
	}

	// Send Python job - should fail
	job := JobMessage{
		Type:           "job",
		JobID:          "py-job-001",
		JobType:        "python",
		Code:           `print("hello")`,
		TimeoutSeconds: 5,
	}
	mock.sendJob(job)

	select {
	case <-mock.jobResults:
		t.Error("should not succeed without python runtime")
	case err := <-mock.jobErrors:
		if !strings.Contains(err.Error, "not available") {
			t.Errorf("expected 'not available' error, got: %s", err.Error)
		}
	case <-ctx.Done():
		t.Fatal("timeout")
	}
}
