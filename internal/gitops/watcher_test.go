package gitops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_Clone(t *testing.T) {
	// Use a public test repo
	repoURL := "https://github.com/octocat/Hello-World.git"
	branch := "master"
	
	tmpDir := t.TempDir()
	auth := &Auth{}
	
	watcher := NewWatcher(tmpDir, 5*time.Minute)
	
	repoPath, err := watcher.Clone(repoURL, branch, auth)
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	
	if repoPath == "" {
		t.Fatal("expected repo path")
	}
	
	// Verify .git directory exists
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("expected .git directory")
	}
}

func TestWatcher_GetLatestCommit(t *testing.T) {
	repoURL := "https://github.com/octocat/Hello-World.git"
	branch := "master"
	
	tmpDir := t.TempDir()
	auth := &Auth{}
	
	watcher := NewWatcher(tmpDir, 5*time.Minute)
	
	repoPath, err := watcher.Clone(repoURL, branch, auth)
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	
	commit, err := watcher.GetLatestCommit(repoPath, branch)
	if err != nil {
		t.Fatalf("get latest commit failed: %v", err)
	}
	
	if commit == "" {
		t.Error("expected commit SHA")
	}
}

func TestWatcher_Pull(t *testing.T) {
	repoURL := "https://github.com/octocat/Hello-World.git"
	branch := "master"
	
	tmpDir := t.TempDir()
	auth := &Auth{}
	
	watcher := NewWatcher(tmpDir, 5*time.Minute)
	
	repoPath, err := watcher.Clone(repoURL, branch, auth)
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	
	err = watcher.Pull(repoPath, branch, auth)
	if err != nil {
		t.Fatalf("pull failed: %v", err)
	}
}

func TestWatcher_CheckoutBranch(t *testing.T) {
	repoURL := "https://github.com/octocat/Hello-World.git"
	branch := "master"
	
	tmpDir := t.TempDir()
	auth := &Auth{}
	
	watcher := NewWatcher(tmpDir, 5*time.Minute)
	
	repoPath, err := watcher.Clone(repoURL, branch, auth)
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	
	// Checkout a different branch (if it exists) or same branch
	err = watcher.CheckoutBranch(repoPath, branch)
	if err != nil {
		t.Fatalf("checkout branch failed: %v", err)
	}
}

func TestWatcher_HasChanges(t *testing.T) {
	repoURL := "https://github.com/octocat/Hello-World.git"
	branch := "master"
	
	tmpDir := t.TempDir()
	auth := &Auth{}
	
	watcher := NewWatcher(tmpDir, 5*time.Minute)
	
	repoPath, err := watcher.Clone(repoURL, branch, auth)
	if err != nil {
		t.Fatalf("clone failed: %v", err)
	}
	
	// First check - should have no changes (already up to date)
	hasChanges, err := watcher.HasChanges(repoPath, branch, auth)
	if err != nil {
		t.Fatalf("has changes check failed: %v", err)
	}
	
	// After initial clone, should be up to date
	if hasChanges {
		t.Error("expected no changes after clone")
	}
}

func TestWatcher_Sync(t *testing.T) {
	repoURL := "https://github.com/octocat/Hello-World.git"
	branch := "master"
	
	tmpDir := t.TempDir()
	auth := &Auth{}
	
	watcher := NewWatcher(tmpDir, 5*time.Minute)
	
	// First sync - should clone
	repoPath, err := watcher.Sync(repoURL, branch, auth)
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	
	if repoPath == "" {
		t.Fatal("expected repo path")
	}
	
	// Second sync - should pull
	repoPath2, err := watcher.Sync(repoURL, branch, auth)
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	
	if repoPath != repoPath2 {
		t.Errorf("expected same path, got %s and %s", repoPath, repoPath2)
	}
}

