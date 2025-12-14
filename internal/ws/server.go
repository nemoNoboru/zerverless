package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/volunteer"
)

// SyncResponse is the result of a synchronous execution
type SyncResponse struct {
	Result any
	Error  string
}

type Server struct {
	vm         *volunteer.Manager
	store      *job.Store
	dispatcher *job.Dispatcher
	connsMu    sync.RWMutex
	conns      map[string]*websocket.Conn
	// For synchronous request-response
	pendingMu sync.RWMutex
	pending   map[string]chan SyncResponse
}

func NewServer(vm *volunteer.Manager, store *job.Store) *Server {
	s := &Server{
		vm:      vm,
		store:   store,
		conns:   make(map[string]*websocket.Conn),
		pending: make(map[string]chan SyncResponse),
	}
	s.dispatcher = job.NewDispatcher(store, s.sendJobToVolunteer)
	return s
}

func (s *Server) sendJobToVolunteer(j *job.Job, volunteerID string) bool {
	s.connsMu.RLock()
	conn, ok := s.conns[volunteerID]
	s.connsMu.RUnlock()
	if !ok {
		return false
	}

	// Check volunteer exists and can handle this job type
	v, ok := s.vm.Get(volunteerID)
	if !ok {
		return false
	}
	if !v.Capabilities.Supports(j.JobType) {
		log.Printf("Volunteer %s doesn't support job type %s", volunteerID[:8], j.JobType)
		return false
	}

	v.SetBusy(j.ID)

	msg := JobMessage{
		Type:           "job",
		JobID:          j.ID,
		JobType:        j.JobType,
		Code:           j.Code,
		WasmCID:        j.WasmCID,
		InputData:      j.InputData,
		TimeoutSeconds: j.TimeoutSeconds,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := wsjson.Write(ctx, conn, msg); err != nil {
		log.Printf("Failed to send job to %s: %v", volunteerID, err)
		v.SetIdle() // Revert on failure
		return false
	}

	log.Printf("Dispatched job %s to volunteer %s", j.ID, volunteerID)
	return true
}

// DispatchToIdle dispatches pending jobs to matching idle volunteers
func (s *Server) DispatchToIdle() {
	// Get the next pending job to check its type
	j := s.store.NextPending()
	if j == nil {
		return
	}

	// Find an idle volunteer that can handle this job type
	idle := s.vm.GetIdleFor(j.JobType)
	if idle != nil {
		s.dispatcher.TryDispatch(idle.ID)
	}
}

// ExecuteSync sends code to a worker and waits for the response synchronously
func (s *Server) ExecuteSync(ctx context.Context, runtime, code string, input map[string]any, timeout time.Duration) (*SyncResponse, error) {
	// Find a capable idle volunteer
	v := s.vm.GetIdleFor(runtime)
	if v == nil {
		return nil, nil // No worker available
	}

	s.connsMu.RLock()
	conn, ok := s.conns[v.ID]
	s.connsMu.RUnlock()
	if !ok {
		return nil, nil
	}

	// Create job ID and response channel
	jobID := "sync-" + v.ID[:8] + "-" + time.Now().Format("150405.000")
	respChan := make(chan SyncResponse, 1)

	s.pendingMu.Lock()
	s.pending[jobID] = respChan
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, jobID)
		s.pendingMu.Unlock()
	}()

	// Mark volunteer as busy
	v.SetBusy(jobID)

	// Send job
	msg := JobMessage{
		Type:           "job",
		JobID:          jobID,
		JobType:        runtime,
		Code:           code,
		InputData:      input,
		TimeoutSeconds: int(timeout.Seconds()),
	}

	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := wsjson.Write(sendCtx, conn, msg); err != nil {
		v.SetIdle()
		return nil, err
	}

	// Wait for response
	select {
	case resp := <-respChan:
		return &resp, nil
	case <-time.After(timeout):
		v.SetIdle()
		return &SyncResponse{Error: "timeout"}, nil
	case <-ctx.Done():
		v.SetIdle()
		return &SyncResponse{Error: "cancelled"}, nil
	}
}

func (s *Server) HandleVolunteer(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // Allow all origins
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "goodbye")

	newVolunteer := volunteer.New()
	newVolunteer.UserAgent = r.UserAgent()
	s.vm.Add(newVolunteer)

	// Track connection for job dispatch
	s.connsMu.Lock()
	s.conns[newVolunteer.ID] = conn
	s.connsMu.Unlock()

	defer func() {
		s.connsMu.Lock()
		delete(s.conns, newVolunteer.ID)
		s.connsMu.Unlock()
		s.vm.Remove(newVolunteer.ID)
	}()

	// Send ack
	ack := AckMessage{
		Type:        "ack",
		VolunteerID: newVolunteer.ID,
		Message:     "Welcome!",
	}
	if err := wsjson.Write(r.Context(), conn, ack); err != nil {
		log.Printf("Failed to send ack: %v", err)
		return
	}

	s.handleMessages(r.Context(), conn, newVolunteer)
}

func (s *Server) handleMessages(ctx context.Context, conn *websocket.Conn, v *volunteer.Volunteer) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		var msg BaseMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		switch msg.Type {
		case "ready":
			var ready ReadyMessage
			if err := json.Unmarshal(data, &ready); err != nil {
				log.Printf("Invalid ready message: %v", err)
				continue
			}
			if ready.Capabilities != nil {
				v.Capabilities = *ready.Capabilities
			}
			v.SetIdle()
			log.Printf("Volunteer %s ready", v.ID)
			// Try to dispatch a pending job
			s.dispatcher.TryDispatch(v.ID)

		case "heartbeat":
			v.UpdateHeartbeat()
			hb := HeartbeatMessage{Type: "heartbeat", Timestamp: time.Now().UTC()}
			wsjson.Write(ctx, conn, hb)

		case "result":
			var result ResultMessage
			json.Unmarshal(data, &result)
			log.Printf("Volunteer %s completed job %s", v.ID, result.JobID)

			// Check if it's a sync request
			s.pendingMu.RLock()
			respChan, isSync := s.pending[result.JobID]
			s.pendingMu.RUnlock()

			if isSync {
				respChan <- SyncResponse{Result: result.Result}
			} else {
				s.store.Complete(result.JobID, result.Result)
			}

			v.JobsCompleted++
			v.SetIdle()
			// Try to dispatch next job
			s.dispatcher.TryDispatch(v.ID)

		case "quit":
			log.Printf("Volunteer %s quit", v.ID)
			return

		case "error":
			var errMsg ErrorMessage
			json.Unmarshal(data, &errMsg)
			log.Printf("Volunteer %s failed job %s: %s", v.ID, errMsg.JobID, errMsg.Error)

			// Check if it's a sync request
			s.pendingMu.RLock()
			respChan, isSync := s.pending[errMsg.JobID]
			s.pendingMu.RUnlock()

			if isSync {
				respChan <- SyncResponse{Error: errMsg.Error}
			} else {
				s.store.Fail(errMsg.JobID, errMsg.Error)
			}

			v.JobsFailed++
			v.SetIdle()
			// Try to dispatch next job
			s.dispatcher.TryDispatch(v.ID)

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}
