package volunteer

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusIdle         Status = "idle"
	StatusBusy         Status = "busy"
	StatusDisconnected Status = "disconnected"
)

type Capabilities struct {
	Wasm        bool     `json:"wasm"`
	Python      bool     `json:"python"`
	Lua         bool     `json:"lua"`
	JS          bool     `json:"js"`
	Docker      bool     `json:"docker"`
	MaxMemoryMB int      `json:"max_memory_mb"`
	Namespaces  []string `json:"namespaces,omitempty"` // Whitelist of namespaces this volunteer can work on
}

// Supports checks if the volunteer can handle a given job type
func (c *Capabilities) Supports(jobType string) bool {
	switch jobType {
	case "js", "javascript":
		return c.JS
	case "lua":
		return c.Lua
	case "python", "py":
		return c.Python
	case "wasm", "":
		return c.Wasm
	case "docker", "docker-build", "docker-build-deploy", "docker-deploy", "docker-run":
		return c.Docker
	default:
		return false
	}
}

// SupportsNamespace checks if the volunteer can work on the given namespace
// Empty whitelist means all namespaces are supported
// "zerverless" namespace is always supported (system namespace)
func (c *Capabilities) SupportsNamespace(namespace string) bool {
	// System namespace is always supported
	if namespace == "zerverless" {
		return true
	}
	if len(c.Namespaces) == 0 {
		return true // No whitelist = support all
	}
	for _, ns := range c.Namespaces {
		if ns == namespace {
			return true
		}
	}
	return false
}

type Volunteer struct {
	ID            string       `json:"id"`
	ConnectedAt   time.Time    `json:"connected_at"`
	LastHeartbeat time.Time    `json:"last_heartbeat"`
	Status        Status       `json:"status"`
	CurrentJobID  string       `json:"current_job_id,omitempty"`
	JobsCompleted int          `json:"jobs_completed"`
	JobsFailed    int          `json:"jobs_failed"`
	Capabilities  Capabilities `json:"capabilities"`
	UserAgent     string       `json:"user_agent,omitempty"`
}

func New() *Volunteer {
	now := time.Now().UTC()
	return &Volunteer{
		ID:            uuid.NewString(),
		ConnectedAt:   now,
		LastHeartbeat: now,
		Status:        StatusIdle,
		Capabilities:  Capabilities{Wasm: true, MaxMemoryMB: 128},
	}
}

func (v *Volunteer) UpdateHeartbeat() {
	v.LastHeartbeat = time.Now().UTC()
}

func (v *Volunteer) SetBusy(jobID string) {
	v.Status = StatusBusy
	v.CurrentJobID = jobID
}

func (v *Volunteer) SetIdle() {
	v.Status = StatusIdle
	v.CurrentJobID = ""
}
