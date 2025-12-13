package ws

import (
	"time"

	"github.com/zerverless/orchestrator/internal/volunteer"
)

type BaseMessage struct {
	Type string `json:"type"`
}

// Volunteer → Orchestrator

type ReadyMessage struct {
	Type         string                  `json:"type"`
	Capabilities *volunteer.Capabilities `json:"capabilities,omitempty"`
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
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// Orchestrator → Volunteer

type AckMessage struct {
	Type        string `json:"type"`
	VolunteerID string `json:"volunteer_id"`
	Message     string `json:"message"`
}

type JobMessage struct {
	Type           string         `json:"type"`
	JobID          string         `json:"job_id"`
	WasmCID        string         `json:"wasm_cid"`
	WasmURL        string         `json:"wasm_url"`
	InputData      map[string]any `json:"input_data"`
	TimeoutSeconds int            `json:"timeout_seconds"`
}

type CancelMessage struct {
	Type  string `json:"type"`
	JobID string `json:"job_id"`
}

