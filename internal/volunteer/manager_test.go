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

func TestCapabilities_Supports(t *testing.T) {
	tests := []struct {
		caps    Capabilities
		jobType string
		want    bool
	}{
		{Capabilities{JS: true}, "js", true},
		{Capabilities{JS: true}, "javascript", true},
		{Capabilities{JS: false}, "js", false},
		{Capabilities{Lua: true}, "lua", true},
		{Capabilities{Lua: false}, "lua", false},
		{Capabilities{Python: true}, "python", true},
		{Capabilities{Python: true}, "py", true},
		{Capabilities{Wasm: true}, "wasm", true},
		{Capabilities{Wasm: true}, "", true},
		{Capabilities{}, "unknown", false},
	}

	for _, tt := range tests {
		got := tt.caps.Supports(tt.jobType)
		if got != tt.want {
			t.Errorf("Supports(%q) with %+v = %v, want %v", tt.jobType, tt.caps, got, tt.want)
		}
	}
}

func TestManager_GetIdleFor(t *testing.T) {
	m := NewManager()

	// v1: only supports Lua
	v1 := New()
	v1.Capabilities = Capabilities{Lua: true}

	// v2: only supports JS
	v2 := New()
	v2.Capabilities = Capabilities{JS: true}

	m.Add(v1)
	m.Add(v2)

	// Should find v2 for JS job
	jsVolunteer := m.GetIdleFor("js")
	if jsVolunteer == nil {
		t.Fatal("expected to find JS volunteer")
	}
	if !jsVolunteer.Capabilities.JS {
		t.Error("expected JS capability")
	}

	// Should find v1 for Lua job
	luaVolunteer := m.GetIdleFor("lua")
	if luaVolunteer == nil {
		t.Fatal("expected to find Lua volunteer")
	}
	if !luaVolunteer.Capabilities.Lua {
		t.Error("expected Lua capability")
	}

	// Should not find anyone for Python job
	pyVolunteer := m.GetIdleFor("python")
	if pyVolunteer != nil {
		t.Error("expected no Python volunteer")
	}
}
