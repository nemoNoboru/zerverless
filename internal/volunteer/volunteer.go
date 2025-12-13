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
	Wasm        bool `json:"wasm"`
	MaxMemoryMB int  `json:"max_memory_mb"`
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

