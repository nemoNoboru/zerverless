package volunteer

import (
	"log"
	"sync"
)

type Stats struct {
	Connected int
	Idle      int
	Busy      int
}

type Manager struct {
	mu         sync.RWMutex
	volunteers map[string]*Volunteer
}

func NewManager() *Manager {
	return &Manager{
		volunteers: make(map[string]*Volunteer),
	}
}

func (m *Manager) Add(v *Volunteer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.volunteers[v.ID] = v
	log.Printf("Volunteer connected: %s (total: %d)", v.ID, len(m.volunteers))
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.volunteers, id)
	log.Printf("Volunteer disconnected: %s (total: %d)", id, len(m.volunteers))
}

func (m *Manager) Get(id string) (*Volunteer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.volunteers[id]
	return v, ok
}

func (m *Manager) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var idle, busy int
	for _, v := range m.volunteers {
		switch v.Status {
		case StatusIdle:
			idle++
		case StatusBusy:
			busy++
		}
	}

	return Stats{
		Connected: len(m.volunteers),
		Idle:      idle,
		Busy:      busy,
	}
}

func (m *Manager) GetIdle() *Volunteer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, v := range m.volunteers {
		if v.Status == StatusIdle {
			return v
		}
	}
	return nil
}

// GetIdleFor returns an idle volunteer that supports the given job type
func (m *Manager) GetIdleFor(jobType string) *Volunteer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, v := range m.volunteers {
		if v.Status == StatusIdle && v.Capabilities.Supports(jobType) {
			return v
		}
	}
	return nil
}
