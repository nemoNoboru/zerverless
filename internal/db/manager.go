package db

import (
	"fmt"
	"path/filepath"
	"sync"
)

// Manager manages per-namespace database instances
type Manager struct {
	mu        sync.RWMutex
	stores    map[string]*Store
	dataDir   string
}

func NewManager(dataDir string) *Manager {
	return &Manager{
		stores:  make(map[string]*Store),
		dataDir: dataDir,
	}
}

// GetStore returns the database store for a namespace, creating it if needed
func (m *Manager) GetStore(namespace string) (*Store, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if store, ok := m.stores[namespace]; ok {
		return store, nil
	}

	// Create new store for this namespace
	nsDir := filepath.Join(m.dataDir, namespace)
	store, err := NewStore(nsDir)
	if err != nil {
		return nil, fmt.Errorf("create store for namespace %s: %w", namespace, err)
	}

	m.stores[namespace] = store
	return store, nil
}

// Close closes all database stores
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for ns, store := range m.stores {
		if err := store.Close(); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("close store for namespace %s: %w", ns, err)
			}
		}
	}
	return firstErr
}

