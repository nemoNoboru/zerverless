package volunteer

import "testing"

func TestManager_AddRemove(t *testing.T) {
	m := NewManager()
	v := New()

	m.Add(v)

	stats := m.Stats()
	if stats.Connected != 1 {
		t.Errorf("expected 1 connected, got %d", stats.Connected)
	}

	m.Remove(v.ID)

	stats = m.Stats()
	if stats.Connected != 0 {
		t.Errorf("expected 0 connected, got %d", stats.Connected)
	}
}

func TestManager_Get(t *testing.T) {
	m := NewManager()
	v := New()

	m.Add(v)

	got, ok := m.Get(v.ID)
	if !ok {
		t.Error("expected to find volunteer")
	}
	if got.ID != v.ID {
		t.Errorf("expected %s, got %s", v.ID, got.ID)
	}

	_, ok = m.Get("nonexistent")
	if ok {
		t.Error("expected not to find nonexistent volunteer")
	}
}

func TestManager_Stats(t *testing.T) {
	m := NewManager()

	v1 := New()
	v2 := New()
	v2.SetBusy("job-1")

	m.Add(v1)
	m.Add(v2)

	stats := m.Stats()

	if stats.Connected != 2 {
		t.Errorf("expected 2 connected, got %d", stats.Connected)
	}
	if stats.Idle != 1 {
		t.Errorf("expected 1 idle, got %d", stats.Idle)
	}
	if stats.Busy != 1 {
		t.Errorf("expected 1 busy, got %d", stats.Busy)
	}
}

func TestManager_GetIdle(t *testing.T) {
	m := NewManager()

	v1 := New()
	v1.SetBusy("job-1")
	v2 := New()

	m.Add(v1)
	m.Add(v2)

	idle := m.GetIdle()
	if idle == nil {
		t.Error("expected to find idle volunteer")
	}
	if idle.Status != StatusIdle {
		t.Errorf("expected idle status, got %s", idle.Status)
	}
}

