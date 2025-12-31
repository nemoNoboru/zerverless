package db

import (
	"os"
	"testing"
)

func TestStore_GetSet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	namespace := "user/alice/"
	key := "test-key"
	value := []byte("test-value")

	// Set value
	if err := store.Set(namespace, key, value); err != nil {
		t.Fatalf("set value: %v", err)
	}

	// Get value
	got, err := store.Get(namespace, key)
	if err != nil {
		t.Fatalf("get value: %v", err)
	}

	if string(got) != string(value) {
		t.Errorf("expected %s, got %s", value, got)
	}
}

func TestStore_GetNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	_, err = store.Get("user/alice/", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	namespace := "user/alice/"
	key := "test-key"
	value := []byte("test-value")

	// Set value
	if err := store.Set(namespace, key, value); err != nil {
		t.Fatalf("set value: %v", err)
	}

	// Delete value
	if err := store.Delete(namespace, key); err != nil {
		t.Fatalf("delete value: %v", err)
	}

	// Get should fail
	_, err = store.Get(namespace, key)
	if err == nil {
		t.Error("expected error after delete")
	}
}



