package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Worker struct {
	url           string
	id            string
	jobsCompleted int
	jobsFailed    int
}

func New(url string) *Worker {
	return &Worker{url: url}
}

func (w *Worker) Run(ctx context.Context) error {
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
	log.Printf("Executing job: %s", job.JobID[:8])

	// TODO: Actual Wasm execution with wazero
	// For now, simulate execution
	time.Sleep(time.Duration(500+len(job.JobID)%1000) * time.Millisecond)

	result := map[string]any{
		"success": true,
		"output":  42,
	}

	msg := ResultMessage{
		Type:   "result",
		JobID:  job.JobID,
		Result: result,
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

func (w *Worker) sendReady(ctx context.Context, conn *websocket.Conn) error {
	msg := ReadyMessage{
		Type: "ready",
		Capabilities: &Capabilities{
			Wasm:        true,
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
	WasmCID        string         `json:"wasm_cid"`
	WasmURL        string         `json:"wasm_url"`
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

