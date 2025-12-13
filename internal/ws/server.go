package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/zerverless/orchestrator/internal/volunteer"
)

type Server struct {
	vm *volunteer.Manager
}

func NewServer(vm *volunteer.Manager) *Server {
	return &Server{vm: vm}
}

func (s *Server) HandleVolunteer(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("WebSocket accept error: %v", err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "goodbye")

	v := volunteer.New()
	v.UserAgent = r.UserAgent()
	s.vm.Add(v)
	defer s.vm.Remove(v.ID)

	// Send ack
	ack := AckMessage{
		Type:        "ack",
		VolunteerID: v.ID,
		Message:     "Welcome!",
	}
	if err := wsjson.Write(r.Context(), conn, ack); err != nil {
		log.Printf("Failed to send ack: %v", err)
		return
	}

	s.handleMessages(r.Context(), conn, v)
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
			json.Unmarshal(data, &ready)
			if ready.Capabilities != nil {
				v.Capabilities = *ready.Capabilities
			}
			v.SetIdle()
			log.Printf("Volunteer %s ready", v.ID)

		case "heartbeat":
			v.UpdateHeartbeat()
			hb := HeartbeatMessage{Type: "heartbeat", Timestamp: time.Now().UTC()}
			wsjson.Write(ctx, conn, hb)

		case "result":
			var result ResultMessage
			json.Unmarshal(data, &result)
			log.Printf("Volunteer %s completed job %s", v.ID, result.JobID)
			v.JobsCompleted++
			v.SetIdle()

		case "error":
			var errMsg ErrorMessage
			json.Unmarshal(data, &errMsg)
			log.Printf("Volunteer %s failed job %s: %s", v.ID, errMsg.JobID, errMsg.Error)
			v.JobsFailed++
			v.SetIdle()

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

