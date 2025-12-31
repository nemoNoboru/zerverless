package job

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zerverless/orchestrator/internal/db"
)

const SystemNamespace = "zerverless"

type PersistentStore struct {
	dbStore *db.Store
}

func NewPersistentStore(dbStore *db.Store) *PersistentStore {
	return &PersistentStore{dbStore: dbStore}
}

func (s *PersistentStore) Add(j *Job) error {
	data, err := json.Marshal(j)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	key := fmt.Sprintf("jobs/%s", j.ID)
	if err := s.dbStore.Set(SystemNamespace, key, data); err != nil {
		return fmt.Errorf("store job: %w", err)
	}

	return nil
}

func (s *PersistentStore) Get(id string) (*Job, error) {
	key := fmt.Sprintf("jobs/%s", id)
	data, err := s.dbStore.Get(SystemNamespace, key)
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}

	var j Job
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}

	return &j, nil
}

func (s *PersistentStore) Update(j *Job) error {
	return s.Add(j)
}

func (s *PersistentStore) ListPending() ([]*Job, error) {
	keys, err := s.dbStore.List(SystemNamespace, "jobs/", 0)
	if err != nil {
		return nil, err
	}

	var pending []*Job
	for _, key := range keys {
		if !strings.HasPrefix(key, "jobs/") {
			continue
		}
		jobID := key[len("jobs/"):]
		if jobID == "" {
			continue
		}
		j, err := s.Get(jobID)
		if err != nil {
			continue
		}
		if j.Status == StatusPending {
			pending = append(pending, j)
		}
	}

	// Sort by CreatedAt for FIFO order
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].CreatedAt.Before(pending[j].CreatedAt)
	})

	return pending, nil
}

func (s *PersistentStore) NextPending() *Job {
	pending, err := s.ListPending()
	if err != nil || len(pending) == 0 {
		return nil
	}
	return pending[0]
}

func (s *PersistentStore) List(limit, offset int, status string) ([]*Job, int) {
	keys, err := s.dbStore.List(SystemNamespace, "jobs/", 0)
	if err != nil {
		return []*Job{}, 0
	}

	var all []*Job
	for _, key := range keys {
		if !strings.HasPrefix(key, "jobs/") {
			continue
		}
		jobID := key[len("jobs/"):]
		if jobID == "" {
			continue
		}
		j, err := s.Get(jobID)
		if err != nil {
			continue
		}
		if status == "" || string(j.Status) == status {
			all = append(all, j)
		}
	}

	// Sort by CreatedAt
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt) // Most recent first
	})

	total := len(all)
	if offset >= total {
		return []*Job{}, total
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return all[offset:end], total
}

func (s *PersistentStore) Stats() (pending, running, completed, failed int) {
	keys, err := s.dbStore.List(SystemNamespace, "jobs/", 0)
	if err != nil {
		return 0, 0, 0, 0
	}

	for _, key := range keys {
		if !strings.HasPrefix(key, "jobs/") {
			continue
		}
		jobID := key[len("jobs/"):]
		if jobID == "" {
			continue
		}
		j, err := s.Get(jobID)
		if err != nil {
			continue
		}
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

func (s *PersistentStore) SetStatus(id string, status Status, volunteerID string) error {
	j, err := s.Get(id)
	if err != nil {
		return err
	}
	j.Status = status
	j.VolunteerID = volunteerID
	return s.Update(j)
}

func (s *PersistentStore) Complete(id string, result any) error {
	j, err := s.Get(id)
	if err != nil {
		return err
	}
	j.Status = StatusCompleted
	j.Result = result
	now := time.Now().UTC()
	j.CompletedAt = &now
	return s.Update(j)
}

func (s *PersistentStore) Fail(id string, errMsg string) error {
	j, err := s.Get(id)
	if err != nil {
		return err
	}
	j.Status = StatusFailed
	j.Error = errMsg
	now := time.Now().UTC()
	j.CompletedAt = &now
	return s.Update(j)
}

