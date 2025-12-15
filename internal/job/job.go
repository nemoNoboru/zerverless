package job

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

type Job struct {
	ID             string         `json:"id"`
	JobType        string         `json:"job_type"`
	Code           string         `json:"code,omitempty"`
	WasmCID        string         `json:"wasm_cid,omitempty"`
	InputData      map[string]any `json:"input_data,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds"`
	Status         Status         `json:"status"`
	Result         any            `json:"result,omitempty"`
	Error          string         `json:"error,omitempty"`
	VolunteerID    string         `json:"volunteer_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
}

func New(jobType, code string, input map[string]any, timeout int) *Job {
	return &Job{
		ID:             uuid.NewString(),
		JobType:        jobType,
		Code:           code,
		InputData:      input,
		TimeoutSeconds: timeout,
		Status:         StatusPending,
		CreatedAt:      time.Now().UTC(),
	}
}

type Store struct {
	mu    sync.RWMutex
	jobs  map[string]*Job
	order []string // FIFO order
}

func NewStore() *Store {
	return &Store{
		jobs:  make(map[string]*Job),
		order: make([]string, 0),
	}
}

func (s *Store) Add(j *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
	s.order = append(s.order, j.ID)
}

func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	return j, ok
}

func (s *Store) NextPending() *Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range s.order {
		if j := s.jobs[id]; j.Status == StatusPending {
			return j
		}
	}
	return nil
}

func (s *Store) List(limit, offset int, status string) ([]*Job, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*Job
	for _, id := range s.order {
		j := s.jobs[id]
		if status == "" || string(j.Status) == status {
			filtered = append(filtered, j)
		}
	}

	total := len(filtered)
	if offset >= total {
		return []*Job{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return filtered[offset:end], total
}

func (s *Store) SetStatus(id string, status Status, volunteerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.VolunteerID = volunteerID
	}
}

func (s *Store) Complete(id string, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = StatusCompleted
		j.Result = result
		now := time.Now().UTC()
		j.CompletedAt = &now
	}
}

func (s *Store) Fail(id string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = StatusFailed
		j.Error = errMsg
		now := time.Now().UTC()
		j.CompletedAt = &now
	}
}

func (s *Store) Stats() (pending, running, completed, failed int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, j := range s.jobs {
		switch j.Status {
		case StatusPending:
			pending++
		case StatusRunning:
			running++
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		}
	}
	return
}
