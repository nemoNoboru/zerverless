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

	dockerrt "github.com/zerverless/orchestrator/internal/docker"
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
	dockerRuntime *dockerrt.Runtime
	dockerEnabled bool
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

	// Try to initialize Docker runtime
	dockerRT, err := dockerrt.NewRuntime()
	if err != nil {
		log.Printf("Warning: Docker runtime not available: %v", err)
	} else {
		w.dockerRuntime = dockerRT
		w.dockerEnabled = true
		log.Printf("Docker runtime enabled")
	}

	return w
}

func (w *Worker) Run(ctx context.Context) error {
	defer w.runtime.Close(ctx)
	if w.pythonRuntime != nil {
		defer w.pythonRuntime.Close(ctx)
	}
	if w.dockerRuntime != nil {
		defer w.dockerRuntime.Close()
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
	case "docker-build":
		result, err = w.executeDockerBuild(ctx, job, timeout)
	case "docker-build-deploy":
		result, err = w.executeDockerBuildDeploy(ctx, job, timeout)
	case "docker-deploy":
		result, err = w.executeDockerDeploy(ctx, conn, job, timeout)
	case "docker", "docker-run":
		result, err = w.executeDockerRun(ctx, job, timeout)
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

func (w *Worker) executeDockerBuild(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
	if !w.dockerEnabled {
		return nil, fmt.Errorf("docker runtime not available")
	}

	// Extract build parameters
	repoPath, ok := job.InputData["repo_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo_path in input_data")
	}
	dockerfilePath, ok := job.InputData["dockerfile_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing dockerfile_path in input_data")
	}
	contextPath, ok := job.InputData["context_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing context_path in input_data")
	}
	imageTag, ok := job.InputData["image_tag"].(string)
	if !ok {
		return nil, fmt.Errorf("missing image_tag in input_data")
	}

	log.Printf("Building Docker image: %s", imageTag)

	// Build image
	buildResult, err := w.dockerRuntime.BuildImage(ctx, repoPath, dockerfilePath, contextPath, imageTag, timeout)
	if err != nil {
		return nil, fmt.Errorf("build image: %w", err)
	}

	// Return result as JSON
	resultJSON, _ := json.Marshal(buildResult)
	return &wasm.ExecutionResult{
		Output: string(resultJSON),
	}, nil
}

func (w *Worker) executeDockerBuildDeploy(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
	if !w.dockerEnabled {
		return nil, fmt.Errorf("docker runtime not available")
	}

	// Extract build parameters
	repoPath, ok := job.InputData["repo_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing repo_path in input_data")
	}
	dockerfilePath, ok := job.InputData["dockerfile_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing dockerfile_path in input_data")
	}
	contextPath, ok := job.InputData["context_path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing context_path in input_data")
	}
	imageTag, ok := job.InputData["image_tag"].(string)
	if !ok {
		return nil, fmt.Errorf("missing image_tag in input_data")
	}
	user, ok := job.InputData["user"].(string)
	if !ok {
		return nil, fmt.Errorf("missing user in input_data")
	}
	path, ok := job.InputData["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing path in input_data")
	}

	log.Printf("Building and deploying Docker image: %s", imageTag)

	// Build image (use 80% of timeout for build, 20% for container start)
	buildTimeout := time.Duration(float64(timeout) * 0.8)
	buildResult, err := w.dockerRuntime.BuildImage(ctx, repoPath, dockerfilePath, contextPath, imageTag, buildTimeout)
	if err != nil {
		return nil, fmt.Errorf("build image: %w", err)
	}

	// Start container for deployment (worker handles this)
	deploymentKey := fmt.Sprintf("%s%s", user, path)
	containerPort := 80 // Default port for web servers
	containerInfo, err := w.dockerRuntime.StartContainerForDeployment(ctx, deploymentKey, imageTag, containerPort)
	if err != nil {
		log.Printf("Warning: Failed to start container, deployment will be created but container not running: %v", err)
		// Still return success, orchestrator can start container later
		result := map[string]any{
			"image_tag": buildResult.ImageTag,
			"user":      user,
			"path":      path,
			"status":    "built",
			"build_log": buildResult.BuildLog,
			"error":     fmt.Sprintf("container start failed: %v", err),
		}
		resultJSON, _ := json.Marshal(result)
		return &wasm.ExecutionResult{
			Output: string(resultJSON),
		}, nil
	}

	log.Printf("Container started: %s on port %d", containerInfo.ContainerID[:12], containerInfo.HostPort)

	// Return result with deployment and container info
	result := map[string]any{
		"image_tag":      buildResult.ImageTag,
		"user":           user,
		"path":           path,
		"status":         "ready",
		"build_log":      buildResult.BuildLog,
		"container_id":   containerInfo.ContainerID,
		"host_port":      containerInfo.HostPort,
		"container_port": containerInfo.ContainerPort,
	}

	resultJSON, _ := json.Marshal(result)
	return &wasm.ExecutionResult{
		Output: string(resultJSON),
	}, nil
}

func (w *Worker) executeDockerDeploy(ctx context.Context, conn *websocket.Conn, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
	// Image tag should be in job.Code (set by orchestrator after build completes)
	imageTag := job.Code
	if imageTag == "" {
		return nil, fmt.Errorf("missing image tag in job code (build job may not have completed)")
	}

	user, ok := job.InputData["user"].(string)
	if !ok {
		return nil, fmt.Errorf("missing user in input_data")
	}
	path, ok := job.InputData["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing path in input_data")
	}

	// docker-deploy is handled by the orchestrator, not the worker
	// The worker just needs to return success so the orchestrator can create the deployment
	// In a full implementation, we'd create the deployment here, but for now
	// we return the deployment info so the orchestrator can handle it

	result := map[string]any{
		"image_tag": imageTag,
		"user":      user,
		"path":      path,
		"status":    "ready",
	}

	resultJSON, _ := json.Marshal(result)
	return &wasm.ExecutionResult{
		Output: string(resultJSON),
	}, nil
}

func (w *Worker) executeDockerRun(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
	if !w.dockerEnabled {
		return nil, fmt.Errorf("docker runtime not available")
	}

	// Image tag is stored in job.Code for Docker runtime
	imageTag := job.Code
	if imageTag == "" {
		return nil, fmt.Errorf("no image tag provided")
	}

	// Extract command and env from input data
	var command []string
	if cmd, ok := job.InputData["command"].([]any); ok {
		for _, c := range cmd {
			if s, ok := c.(string); ok {
				command = append(command, s)
			}
		}
	}

	env := make(map[string]string)
	if envMap, ok := job.InputData["env"].(map[string]any); ok {
		for k, v := range envMap {
			if s, ok := v.(string); ok {
				env[k] = s
			}
		}
	}

	// Add INPUT data as JSON env var
	if job.InputData != nil {
		inputJSON, _ := json.Marshal(job.InputData)
		env["INPUT"] = string(inputJSON)
	}

	log.Printf("Running Docker container: %s", imageTag)

	// Run container
	runResult, err := w.dockerRuntime.RunContainer(ctx, imageTag, command, env, timeout)
	if err != nil {
		return nil, fmt.Errorf("run container: %w", err)
	}

	// Return result
	if runResult.Error != "" {
		return &wasm.ExecutionResult{
			Output: runResult.Output,
			Error:  runResult.Error,
		}, nil
	}

	return &wasm.ExecutionResult{
		Output: runResult.Output,
	}, nil
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
			Docker:      w.dockerEnabled,
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
	Docker      bool `json:"docker"`
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
