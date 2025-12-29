package deploy

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zerverless/orchestrator/internal/db"
)

const SystemNamespace = "zerverless"

type PersistentStore struct {
	dbStore *db.Store
}

func NewPersistentStore(dbStore *db.Store) *PersistentStore {
	return &PersistentStore{dbStore: dbStore}
}

func (s *PersistentStore) Set(d *Deployment) error {
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal deployment: %w", err)
	}

	key := fmt.Sprintf("deployments/%s/%s", d.User, d.Path)
	if err := s.dbStore.Set(SystemNamespace, key, data); err != nil {
		return fmt.Errorf("store deployment: %w", err)
	}

	return nil
}

func (s *PersistentStore) Get(user, path string) (*Deployment, error) {
	key := fmt.Sprintf("deployments/%s/%s", user, path)
	data, err := s.dbStore.Get(SystemNamespace, key)
	if err != nil {
		return nil, fmt.Errorf("get deployment: %w", err)
	}

	var d Deployment
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("unmarshal deployment: %w", err)
	}

	return &d, nil
}

func (s *PersistentStore) Delete(user, path string) error {
	key := fmt.Sprintf("deployments/%s/%s", user, path)
	if err := s.dbStore.Delete(SystemNamespace, key); err != nil {
		return fmt.Errorf("delete deployment: %w", err)
	}
	return nil
}

func (s *PersistentStore) ListByUser(user string) ([]*Deployment, error) {
	prefix := fmt.Sprintf("deployments/%s/", user)
	keys, err := s.dbStore.List(SystemNamespace, prefix, 0)
	if err != nil {
		return nil, err
	}

	var deployments []*Deployment
	for _, key := range keys {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		path := key[len(prefix):]
		if path == "" {
			continue
		}
		d, err := s.Get(user, path)
		if err != nil {
			continue
		}
		deployments = append(deployments, d)
	}

	return deployments, nil
}

func (s *PersistentStore) ListAll() ([]*Deployment, error) {
	prefix := "deployments/"
	keys, err := s.dbStore.List(SystemNamespace, prefix, 0)
	if err != nil {
		return nil, err
	}

	var deployments []*Deployment
	seen := make(map[string]bool)

	for _, key := range keys {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		rest := key[len(prefix):]
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) < 2 {
			continue
		}

		user := parts[0]
		path := parts[1]
		deploymentKey := user + path

		if seen[deploymentKey] {
			continue
		}
		seen[deploymentKey] = true

		d, err := s.Get(user, path)
		if err != nil {
			continue
		}
		deployments = append(deployments, d)
	}

	return deployments, nil
}

func (s *PersistentStore) List() ([]*Deployment, error) {
	return s.ListAll()
}
