package job

import (
	"testing"
)

func TestDispatcher_AssignsPendingToIdle(t *testing.T) {
	store := NewStore()
	j := New("python", "print(1)", nil, 30)
	store.Add(j)

	// Mock volunteer assignment function
	var assignedJob *Job
	var assignedTo string
	assignFn := func(job *Job, volunteerID string) bool {
		assignedJob = job
		assignedTo = volunteerID
		return true
	}

	d := NewDispatcher(store, assignFn)
	d.TryDispatch("vol-1")

	if assignedJob == nil {
		t.Fatal("expected job to be assigned")
	}
	if assignedJob.ID != j.ID {
		t.Errorf("expected %s, got %s", j.ID, assignedJob.ID)
	}
	if assignedTo != "vol-1" {
		t.Errorf("expected vol-1, got %s", assignedTo)
	}
	if assignedJob.Status != StatusRunning {
		t.Errorf("expected running, got %s", assignedJob.Status)
	}
}

func TestDispatcher_NoPendingJobs(t *testing.T) {
	store := NewStore()
	called := false
	assignFn := func(job *Job, volunteerID string) bool {
		called = true
		return true
	}

	d := NewDispatcher(store, assignFn)
	d.TryDispatch("vol-1")

	if called {
		t.Error("expected assign not to be called")
	}
}

func TestDispatcher_AssignmentFails(t *testing.T) {
	store := NewStore()
	j := New("python", "print(1)", nil, 30)
	store.Add(j)

	// Assignment fails (volunteer disconnected)
	assignFn := func(job *Job, volunteerID string) bool {
		return false
	}

	d := NewDispatcher(store, assignFn)
	d.TryDispatch("vol-1")

	// Job should remain pending
	got, _ := store.Get(j.ID)
	if got.Status != StatusPending {
		t.Errorf("expected pending, got %s", got.Status)
	}
}
