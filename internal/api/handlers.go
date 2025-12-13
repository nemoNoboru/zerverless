package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/zerverless/orchestrator/internal/config"
	"github.com/zerverless/orchestrator/internal/volunteer"
)

var startTime = time.Now()

type Handlers struct {
	cfg *config.Config
	vm  *volunteer.Manager
}

func NewHandlers(cfg *config.Config, vm *volunteer.Manager) *Handlers {
	return &Handlers{cfg: cfg, vm: vm}
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
	stats := h.vm.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"node_id":        h.cfg.NodeID,
		"uptime_seconds": int(time.Since(startTime).Seconds()),
		"volunteers": map[string]int{
			"connected": stats.Connected,
			"idle":      stats.Idle,
			"busy":      stats.Busy,
		},
		"jobs": map[string]int{
			"pending":         0,
			"running":         0,
			"completed_total": 0,
			"failed_total":    0,
		},
		"peers": map[string]int{
			"connected": 0,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

