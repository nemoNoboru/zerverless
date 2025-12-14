package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/volunteer"
)

var startTime = time.Now()

// DispatchFunc is called after job submission to trigger dispatch
type DispatchFunc func()

type Handlers struct {
	cfg        *config.Config
	vm         *volunteer.Manager
	store      *job.Store
	onDispatch DispatchFunc
}

func NewHandlers(cfg *config.Config, vm *volunteer.Manager, store *job.Store) *Handlers {
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

	j := job.New(req.JobType, req.Code, req.InputData, timeout)
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
	j, ok := h.store.Get(id)
	if !ok {
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
