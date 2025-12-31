package job

import (
	"os"
	"testing"

	"github.com/zerverless/orchestrator/internal/db"
)

func TestPersistentStore_AddAndGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbStore, err := db.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create db store: %v", err)
	}
	defer dbStore.Close()

	store := NewPersistentStore(dbStore)
	j := New("python", "print('hello')", nil, 30)

	if err := store.Add(j); err != nil {
		t.Fatalf("add job: %v", err)
	}

	got, err := store.Get(j.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}

	if got.ID != j.ID {
		t.Errorf("expected %s, got %s", j.ID, got.ID)
	}
	if got.JobType != j.JobType {
		t.Errorf("expected %s, got %s", j.JobType, got.JobType)
	}
}

func TestPersistentStore_NextPending(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbStore, err := db.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create db store: %v", err)
	}
	defer dbStore.Close()

	store := NewPersistentStore(dbStore)
	j1 := New("python", "print(1)", nil, 30)
	j2 := New("python", "print(2)", nil, 30)

	store.Add(j1)
	store.Add(j2)

	next := store.NextPending()
	if next == nil {
		t.Fatal("expected pending job")
	}
	if next.ID != j1.ID {
		t.Errorf("expected %s, got %s", j1.ID, next.ID)
	}
}

