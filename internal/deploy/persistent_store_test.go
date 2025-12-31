package deploy

import (
	"os"
	"testing"

	"github.com/zerverless/orchestrator/internal/db"
)

func TestPersistentStore_SetAndGet(t *testing.T) {
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
	d := New("alice", "/hello", "lua", "function handle() return {status=200} end")

	if err := store.Set(d); err != nil {
		t.Fatalf("set deployment: %v", err)
	}

	got, err := store.Get("alice", "/hello")
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}

	if got.User != d.User {
		t.Errorf("expected %s, got %s", d.User, got.User)
	}
	if got.Path != d.Path {
		t.Errorf("expected %s, got %s", d.Path, got.Path)
	}
}

func TestPersistentStore_Delete(t *testing.T) {
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
	d := New("alice", "/hello", "lua", "code")

	store.Set(d)
	store.Delete("alice", "/hello")

	_, err = store.Get("alice", "/hello")
	if err == nil {
		t.Error("expected error after delete")
	}
}




