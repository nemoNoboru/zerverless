package job

import (
	"fmt"
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
	Namespace      string         `json:"namespace,omitempty"` // User namespace this job belongs to
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
	return NewWithNamespace(jobType, code, input, timeout, "")
}

func NewWithNamespace(jobType, code string, input map[string]any, timeout int, namespace string) *Job {
	return &Job{
		ID:             uuid.NewString(),
		JobType:        jobType,
		Namespace:      namespace,
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

func (s *Store) Add(j *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
	s.order = append(s.order, j.ID)
	return nil
}

func (s *Store) Get(id string) (*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	return j, nil
}

func (s *Store) Update(j *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[j.ID]; !ok {
		return fmt.Errorf("job not found: %s", j.ID)
	}
	s.jobs[j.ID] = j
	return nil
}

func (s *Store) ListPending() ([]*Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var pending []*Job
	for _, id := range s.order {
		if j := s.jobs[id]; j.Status == StatusPending {
			pending = append(pending, j)
		}
	}
	return pending, nil
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

func (s *Store) SetStatus(id string, status Status, volunteerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = status
		j.VolunteerID = volunteerID
		return nil
	}
	return fmt.Errorf("job not found: %s", id)
}

func (s *Store) Complete(id string, result any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = StatusCompleted
		j.Result = result
		now := time.Now().UTC()
		j.CompletedAt = &now
		return nil
	}
	return fmt.Errorf("job not found: %s", id)
}

func (s *Store) Fail(id string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok {
		j.Status = StatusFailed
		j.Error = errMsg
		now := time.Now().UTC()
		j.CompletedAt = &now
		return nil
	}
	return fmt.Errorf("job not found: %s", id)
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
