package job

import (
	"testing"
)

func TestNewJob(t *testing.T) {
	j := New("python", "print('hello')", map[string]any{"x": 1}, 30)

	if j.ID == "" {
		t.Error("expected job ID")
	}
	if j.Status != StatusPending {
		t.Errorf("expected pending, got %s", j.Status)
	}
	if j.JobType != "python" {
		t.Errorf("expected python, got %s", j.JobType)
	}
	if j.Code != "print('hello')" {
		t.Errorf("expected print('hello'), got %s", j.Code)
	}
	if j.CreatedAt.IsZero() {
		t.Error("expected created_at")
	}
}

func TestStore_AddAndGet(t *testing.T) {
	store := NewStore()
	j := New("python", "print(1)", nil, 30)

	store.Add(j)
	got, ok := store.Get(j.ID)

	if !ok {
		t.Fatal("expected job to exist")
	}
	if got.ID != j.ID {
		t.Errorf("expected %s, got %s", j.ID, got.ID)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	store := NewStore()

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("expected job not found")
	}
}

func TestStore_NextPending(t *testing.T) {
	store := NewStore()
	j1 := New("python", "print(1)", nil, 30)
	j2 := New("python", "print(2)", nil, 30)

	store.Add(j1)
	store.Add(j2)

	// First pending should be j1 (FIFO)
	next := store.NextPending()
	if next == nil {
		t.Fatal("expected pending job")
	}
	if next.ID != j1.ID {
		t.Errorf("expected %s, got %s", j1.ID, next.ID)
	}
}

func TestStore_List(t *testing.T) {
	store := NewStore()
	store.Add(New("python", "1", nil, 30))
	store.Add(New("python", "2", nil, 30))

	jobs, total := store.List(10, 0, "")

	if total != 2 {
		t.Errorf("expected 2 total, got %d", total)
	}
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}
