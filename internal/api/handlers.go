package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/volunteer"
	"github.com/zerverless/orchestrator/internal/ws"
)

var startTime = time.Now()

// DispatchFunc is called after job submission to trigger dispatch
type DispatchFunc func()

type Handlers struct {
	cfg         *config.Config
	vm          *volunteer.Manager
	store       job.JobStore
	deployStore deploy.DeployStore
	wsServer    *ws.Server
	onDispatch  DispatchFunc
}

func NewHandlers(cfg *config.Config, vm *volunteer.Manager, store job.JobStore) *Handlers {
	return &Handlers{cfg: cfg, vm: vm, store: store}
}

func (h *Handlers) SetDispatchFunc(fn DispatchFunc) {
	h.onDispatch = fn
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (h *Handlers) Info(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"node_id":        h.cfg.NodeID,
		"version":        "0.1.0",
		"uptime_seconds": int(time.Since(startTime).Seconds()),
	})
}

func (h *Handlers) Stats(w http.ResponseWriter, r *http.Request) {
	vStats := h.vm.Stats()
	pending, running, completed, failed := h.store.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"node_id":        h.cfg.NodeID,
		"uptime_seconds": int(time.Since(startTime).Seconds()),
		"volunteers": map[string]int{
			"connected": vStats.Connected,
			"idle":      vStats.Idle,
			"busy":      vStats.Busy,
		},
		"jobs": map[string]int{
			"pending":         pending,
			"running":         running,
			"completed_total": completed,
			"failed_total":    failed,
		},
		"peers": map[string]int{
			"connected": 0,
		},
	})
}

type JobRequest struct {
	JobType        string         `json:"job_type"`
	Namespace      string         `json:"namespace,omitempty"` // User namespace
	Code           string         `json:"code,omitempty"`
	WasmCID        string         `json:"wasm_cid,omitempty"`
	InputData      map[string]any `json:"input_data,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds"`
}

func (h *Handlers) SubmitJob(w http.ResponseWriter, r *http.Request) {
	var req JobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	timeout := req.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}

	j := job.NewWithNamespace(req.JobType, req.Code, req.InputData, timeout, req.Namespace)
	if req.WasmCID != "" {
		j.WasmCID = req.WasmCID
	}
	h.store.Add(j)

	// Trigger dispatch to idle volunteers
	if h.onDispatch != nil {
		h.onDispatch()
	}

	writeJSON(w, http.StatusCreated, j)
}

func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	j, err := h.store.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	writeJSON(w, http.StatusOK, j)
}

func (h *Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	status := r.URL.Query().Get("status")

	if limit <= 0 {
		limit = 20
	}

	jobs, total := h.store.List(limit, offset, status)
	writeJSON(w, http.StatusOK, map[string]any{
		"jobs":   jobs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type DeployRequest struct {
	Runtime string `json:"runtime"`
	Code    string `json:"code"`
}

func (h *Handlers) Deploy(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	path := "/" + strings.TrimPrefix(chi.URLParam(r, "*"), "/")

	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "code is required"})
		return
	}

	if req.Runtime == "" {
		req.Runtime = "lua" // default
	}

	d := deploy.New(user, path, req.Runtime, req.Code)
	if err := h.deployStore.Set(d); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, d)
}

func (h *Handlers) ListDeployments(w http.ResponseWriter, r *http.Request) {
	deployments, err := h.deployStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deployments": deployments,
		"total":       len(deployments),
	})
}

func (h *Handlers) DeleteDeployment(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	path := "/" + strings.TrimPrefix(chi.URLParam(r, "*"), "/")

	if err := h.deployStore.Delete(user, path); err != nil {
		http.NotFound(w, r)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) Invoke(w http.ResponseWriter, r *http.Request) {
	user := chi.URLParam(r, "user")
	path := "/" + strings.TrimPrefix(chi.URLParam(r, "*"), "/")

	// Find deployment
	d, err := h.deployStore.Get(user, path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Build request input for the handler
	query := make(map[string]string)
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			query[k] = v[0]
		}
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	body, _ := io.ReadAll(r.Body)

	input := map[string]any{
		"method":  r.Method,
		"path":    path,
		"query":   query,
		"headers": headers,
		"body":    string(body),
	}

	// Wrap code to call handle(INPUT)
	var wrappedCode string
	switch d.Runtime {
	case "lua":
		wrappedCode = d.Code + "\nreturn handle(INPUT)"
	case "js", "javascript":
		wrappedCode = d.Code + "\nhandle(INPUT)"
	case "docker":
		// For Docker, Code contains the image tag
		// Pass input as env vars, use default CMD from image
		wrappedCode = d.Code // Image tag
	default:
		wrappedCode = d.Code
	}

	// Execute synchronously with namespace awareness (user = namespace for deployments)
	// For Docker, ExecuteSyncWithNamespace will use docker runtime and pass input as env
	resp, err := h.wsServer.ExecuteSyncWithNamespace(r.Context(), d.Runtime, wrappedCode, input, 30*time.Second, user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if resp == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no workers available"})
		return
	}
	if resp.Error != "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": resp.Error})
		return
	}

	// Parse the response from the handler
	// Expected format: {status: int, headers: map, body: string}
	if respMap, ok := resp.Result.(map[string]any); ok {
		status := http.StatusOK
		if s, ok := respMap["status"].(float64); ok {
			status = int(s)
		}

		// Set headers
		if hdrs, ok := respMap["headers"].(map[string]any); ok {
			for k, v := range hdrs {
				if vs, ok := v.(string); ok {
					w.Header().Set(k, vs)
				}
			}
		}

		w.WriteHeader(status)

		// Write body
		if bodyStr, ok := respMap["body"].(string); ok {
			w.Write([]byte(bodyStr))
		}
		return
	}

	// Fallback: return result as-is
	writeJSON(w, http.StatusOK, resp.Result)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
