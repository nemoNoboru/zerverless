package deploy

import (
	"sync"
	"time"
)

type Deployment struct {
	User      string    `json:"user"`
	Path      string    `json:"path"`
	Runtime   string    `json:"runtime"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
}

func New(user, path, runtime, code string) *Deployment {
	return &Deployment{
		User:      user,
		Path:      path,
		Runtime:   runtime,
		Code:      code,
		CreatedAt: time.Now().UTC(),
	}
}

// Key returns the unique identifier for a deployment
func (d *Deployment) Key() string {
	return d.User + d.Path
}

type Store struct {
	mu          sync.RWMutex
	deployments map[string]*Deployment
}

func NewStore() *Store {
	return &Store{
		deployments: make(map[string]*Deployment),
	}
}

func (s *Store) Set(d *Deployment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deployments[d.Key()] = d
}

func (s *Store) Get(user, path string) (*Deployment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.deployments[user+path]
	return d, ok
}

func (s *Store) Delete(user, path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := user + path
	if _, ok := s.deployments[key]; !ok {
		return false
	}
	delete(s.deployments, key)
	return true
}

func (s *Store) List() []*Deployment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Deployment, 0, len(s.deployments))
	for _, d := range s.deployments {
		result = append(result, d)
	}
	return result
}
