package deploy

import (
	"fmt"
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

func (s *Store) Set(d *Deployment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deployments[d.Key()] = d
	return nil
}

func (s *Store) Get(user, path string) (*Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.deployments[user+path]
	if !ok {
		return nil, fmt.Errorf("deployment not found: %s/%s", user, path)
	}
	return d, nil
}

func (s *Store) Delete(user, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := user + path
	if _, ok := s.deployments[key]; !ok {
		return fmt.Errorf("deployment not found: %s/%s", user, path)
	}
	delete(s.deployments, key)
	return nil
}

func (s *Store) List() ([]*Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Deployment, 0, len(s.deployments))
	for _, d := range s.deployments {
		result = append(result, d)
	}
	return result, nil
}
