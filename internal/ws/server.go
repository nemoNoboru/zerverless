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

	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/docker"
	"github.com/zerverless/orchestrator/internal/job"
	"github.com/zerverless/orchestrator/internal/volunteer"
)

// SyncResponse is the result of a synchronous execution
type SyncResponse struct {
	Result any
	Error  string
}

type Server struct {
	vm           *volunteer.Manager
	store        job.JobStore
	deployStore  deploy.DeployStore
	containerMgr *docker.ContainerManager
	dispatcher   *job.Dispatcher
	connsMu      sync.RWMutex
	conns        map[string]*websocket.Conn
	// For synchronous request-response
	pendingMu sync.RWMutex
	pending   map[string]chan SyncResponse
}

func NewServer(vm *volunteer.Manager, store job.JobStore) *Server {
	return NewServerWithDeploy(vm, store, nil)
}

func NewServerWithDeploy(vm *volunteer.Manager, store job.JobStore, deployStore deploy.DeployStore) *Server {
	s := &Server{
		vm:          vm,
		store:       store,
		deployStore: deployStore,
		conns:       make(map[string]*websocket.Conn),
		pending:     make(map[string]chan SyncResponse),
	}
	s.dispatcher = job.NewDispatcher(store, s.sendJobToVolunteer)
	return s
}

func (s *Server) SetDeployStore(deployStore deploy.DeployStore) {
	s.deployStore = deployStore
}

func (s *Server) SetContainerManager(containerMgr *docker.ContainerManager) {
	s.containerMgr = containerMgr
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
	// Try to dispatch all pending jobs until we can't find matches
	maxAttempts := 10 // Prevent infinite loops
	for i := 0; i < maxAttempts; i++ {
		// Get the next pending job to check its type
		j := s.store.NextPending()
		if j == nil {
			return
		}

		// Find an idle volunteer that can handle this job type and namespace
		idle := s.vm.GetIdleForNamespace(j.JobType, j.Namespace)
		if idle != nil {
			// Use TryDispatchJob to dispatch the specific job we found
			s.dispatcher.TryDispatchJob(j, idle.ID)
			// Continue to try dispatching more jobs
		} else {
			log.Printf("DispatchToIdle: no idle volunteer found for job type %s, namespace %s", j.JobType, j.Namespace)
			// No volunteer available for this job, stop trying
			return
		}
	}
}

// ExecuteSync sends code to a worker and waits for the response synchronously
func (s *Server) ExecuteSync(ctx context.Context, runtime, code string, input map[string]any, timeout time.Duration) (*SyncResponse, error) {
	return s.ExecuteSyncWithNamespace(ctx, runtime, code, input, timeout, "")
}

// ExecuteSyncWithNamespace sends code to a worker with namespace awareness
func (s *Server) ExecuteSyncWithNamespace(ctx context.Context, runtime, code string, input map[string]any, timeout time.Duration, namespace string) (*SyncResponse, error) {
	// Find a capable idle volunteer that supports the namespace
	v := s.vm.GetIdleForNamespace(runtime, namespace)
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

	// For Docker runtime, prepare input data with command and env
	inputData := input
	if runtime == "docker" || runtime == "docker-run" {
		// Docker needs command and env in input_data
		command := []string{} // Use default CMD from image
		env := make(map[string]string)
		
		// Add input as JSON env var
		inputJSON, _ := json.Marshal(input)
		env["INPUT"] = string(inputJSON)
		
		inputData = map[string]any{
			"command": command,
			"env":     env,
		}
	}

	// Send job
	msg := JobMessage{
		Type:           "job",
		JobID:          jobID,
		JobType:        runtime,
		Code:           code,
		InputData:      inputData,
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
				if err := s.store.Complete(result.JobID, result.Result); err != nil {
					log.Printf("Failed to complete job %s: %v", result.JobID, err)
				} else {
					// Handle job dependencies (e.g., docker-build -> docker-deploy)
					s.handleJobDependencies(result.JobID, result.Result)
				}
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
				if err := s.store.Fail(errMsg.JobID, errMsg.Error); err != nil {
					log.Printf("Failed to fail job %s: %v", errMsg.JobID, err)
				}
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

// handleJobDependencies processes job completion and updates dependent jobs
func (s *Server) handleJobDependencies(jobID string, result any) {
	completedJob, err := s.store.Get(jobID)
	if err != nil {
		return
	}

	// Handle docker-build-deploy completion: create deployment directly
	if completedJob.JobType == "docker-build-deploy" {
		s.handleDockerBuildDeployCompletion(jobID, result)
	}

	// Handle docker-build completion: update dependent docker-deploy jobs (legacy)
	if completedJob.JobType == "docker-build" {
		s.handleDockerBuildCompletion(jobID, result)
	}

	// Handle docker-deploy completion: create deployment (legacy)
	if completedJob.JobType == "docker-deploy" {
		s.handleDockerDeployCompletion(jobID, result)
	}
}

func (s *Server) handleDockerBuildCompletion(jobID string, result any) {
	log.Printf("Processing docker-build completion for job %s", jobID[:8])
	
	// Extract image tag from build result
	var imageTag string
	if resultMap, ok := result.(map[string]any); ok {
		if tag, ok := resultMap["image_tag"].(string); ok {
			imageTag = tag
		}
	} else if resultStr, ok := result.(string); ok {
		// Try to parse JSON string
		var buildResult map[string]any
		if err := json.Unmarshal([]byte(resultStr), &buildResult); err == nil {
			if tag, ok := buildResult["image_tag"].(string); ok {
				imageTag = tag
			}
		}
	}

	if imageTag == "" {
		log.Printf("Could not extract image tag from build job %s result", jobID[:8])
		return
	}

	log.Printf("Extracted image tag %s from build job %s", imageTag, jobID[:8])

	// Find pending docker-deploy jobs that depend on this build job
	// We need to search all pending jobs
	pending, err := s.store.ListPending()
	if err != nil {
		log.Printf("Failed to list pending jobs: %v", err)
		return
	}

	log.Printf("Searching %d pending jobs for deploy jobs depending on build job %s", len(pending), jobID[:8])
	
	found := false
	for _, job := range pending {
		if job.JobType == "docker-deploy" {
			if buildJobID, ok := job.InputData["build_job_id"].(string); ok && buildJobID == jobID {
				found = true
				log.Printf("Found deploy job %s depending on build job %s", job.ID[:8], jobID[:8])
				// Update the deploy job with the image tag
				job.Code = imageTag // Store image tag in Code field for docker-deploy
				if err := s.store.Update(job); err != nil {
					log.Printf("Failed to update deploy job %s: %v", job.ID[:8], err)
				} else {
					log.Printf("Updated deploy job %s with image tag %s, attempting dispatch", job.ID[:8], imageTag)
					// Try to dispatch the updated job
					s.dispatcher.TryDispatch("")
				}
			}
		}
	}
	
	if !found {
		log.Printf("No deploy job found depending on build job %s", jobID[:8])
	}
}

func (s *Server) handleDockerBuildDeployCompletion(jobID string, result any) {
	log.Printf("Processing docker-build-deploy completion for job %s", jobID[:8])

	if s.deployStore == nil {
		log.Printf("Deploy store not available, cannot create deployment")
		return
	}

	// Extract deployment info from result
	var imageTag, user, path string
	var containerID string
	var hostPort int
	workerStartedContainer := false
	
	if resultMap, ok := result.(map[string]any); ok {
		if tag, ok := resultMap["image_tag"].(string); ok {
			imageTag = tag
		}
		if u, ok := resultMap["user"].(string); ok {
			user = u
		}
		if p, ok := resultMap["path"].(string); ok {
			path = p
		}
		// Check if worker already started the container
		if cid, ok := resultMap["container_id"].(string); ok && cid != "" {
			containerID = cid
			workerStartedContainer = true
			if hp, ok := resultMap["host_port"].(float64); ok {
				hostPort = int(hp)
			}
		}
	} else if resultStr, ok := result.(string); ok {
		// Try to parse JSON string
		var deployResult map[string]any
		if err := json.Unmarshal([]byte(resultStr), &deployResult); err == nil {
			if tag, ok := deployResult["image_tag"].(string); ok {
				imageTag = tag
			}
			if u, ok := deployResult["user"].(string); ok {
				user = u
			}
			if p, ok := deployResult["path"].(string); ok {
				path = p
			}
			// Check if worker already started the container
			if cid, ok := deployResult["container_id"].(string); ok && cid != "" {
				containerID = cid
				workerStartedContainer = true
				if hp, ok := deployResult["host_port"].(float64); ok {
					hostPort = int(hp)
				}
			}
		}
	}

	if imageTag == "" || user == "" || path == "" {
		log.Printf("Could not extract deployment info from build-deploy job %s result", jobID[:8])
		return
	}

	log.Printf("Creating deployment: user=%s, path=%s, image=%s", user, path, imageTag)

	// Create deployment
	deployment := &deploy.Deployment{
		User:    user,
		Path:    path,
		Runtime: "docker",
		Code:    imageTag, // Store image tag as "code"
		Port:    80,       // Default container port for web servers
	}

	if err := s.deployStore.Set(deployment); err != nil {
		log.Printf("Failed to create deployment: %v", err)
		return
	}

	log.Printf("Deployment created: %s/%s", user, path)

	// Start or track container for the deployment
	if s.containerMgr != nil {
		deploymentKey := deployment.Key()
		containerPort := deployment.Port
		if containerPort == 0 {
			containerPort = 80 // Default to 80 for web servers
		}
		
		if workerStartedContainer {
			// Worker already started the container, just track it
			log.Printf("Worker started container %s on port %d for deployment %s", containerID[:12], hostPort, deploymentKey)
			// Note: ContainerManager will discover the container on first request if needed
			// For now, we rely on the worker's container being accessible
		} else {
			// Worker didn't start container (or failed), orchestrator starts it
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := s.containerMgr.StartContainerForDeployment(ctx, deploymentKey, imageTag, containerPort)
			if err != nil {
				log.Printf("Failed to start container for deployment %s: %v", deploymentKey, err)
				// Don't fail the deployment, container might start later
			} else {
				log.Printf("Orchestrator started container for deployment %s", deploymentKey)
			}
		}
	} else {
		if workerStartedContainer {
			log.Printf("Worker started container %s on port %d, but container manager not available for tracking", containerID[:12], hostPort)
		} else {
			log.Printf("Container manager not available, container will be started on first request")
		}
	}
}

func (s *Server) handleDockerDeployCompletion(jobID string, result any) {
	// Extract deployment info from result
	var imageTag, user, path string
	if resultMap, ok := result.(map[string]any); ok {
		imageTag, _ = resultMap["image_tag"].(string)
		user, _ = resultMap["user"].(string)
		path, _ = resultMap["path"].(string)
	} else if resultStr, ok := result.(string); ok {
		// Try to parse JSON string
		var deployResult map[string]any
		if err := json.Unmarshal([]byte(resultStr), &deployResult); err == nil {
			imageTag, _ = deployResult["image_tag"].(string)
			user, _ = deployResult["user"].(string)
			path, _ = deployResult["path"].(string)
		}
	}

	if imageTag == "" || user == "" || path == "" {
		log.Printf("Could not extract deployment info from deploy job %s result", jobID)
		return
	}

	// Create deployment (we need access to deployStore)
	// For now, log it - in full implementation, we'd need to pass deployStore to Server
	log.Printf("Deployment ready: %s/%s -> %s", user, path, imageTag)
	// TODO: Create deployment in deployStore
}
