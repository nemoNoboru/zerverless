package storage

import (
	"os"
	"testing"
)

func TestStore_PutGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	namespace := "alice"
	path := "static/index.html"
	content := []byte("<html><body>Hello</body></html>")

	// Put file
	if err := store.Put(namespace, path, content); err != nil {
		t.Fatalf("put file: %v", err)
	}

	// Get file
	got, err := store.Get(namespace, path)
	if err != nil {
		t.Fatalf("get file: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("expected %s, got %s", content, got)
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	namespace := "alice"
	path := "static/test.txt"
	content := []byte("test content")

	// Put file
	if err := store.Put(namespace, path, content); err != nil {
		t.Fatalf("put file: %v", err)
	}

	// Delete file
	if err := store.Delete(namespace, path); err != nil {
		t.Fatalf("delete file: %v", err)
	}

	// Get should fail
	_, err = store.Get(namespace, path)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestStore_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	namespace := "alice"
	
	// Put multiple files
	store.Put(namespace, "static/file1.txt", []byte("content1"))
	store.Put(namespace, "static/file2.txt", []byte("content2"))
	store.Put(namespace, "static/sub/file3.txt", []byte("content3"))

	// List files
	files, err := store.List(namespace, "static/")
	if err != nil {
		t.Fatalf("list files: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

