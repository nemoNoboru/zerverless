package gitops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zerverless/orchestrator/internal/db"
	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/job"
)

func TestSyncer_SyncApplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gitops-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbStore, err := db.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create db store: %v", err)
	}
	defer dbStore.Close()

	jobStore := job.NewPersistentStore(dbStore)
	deployStore := deploy.NewPersistentStore(dbStore)
	watcher := NewWatcher(tmpDir, 5*time.Minute)
	syncer := NewSyncer(watcher, jobStore, deployStore, tmpDir)

	app := &Application{
		APIVersion: "zerverless.io/v1",
		Kind:       "Application",
		Metadata: ApplicationMeta{
			Name:      "test-app",
			Namespace: "alice",
		},
		Spec: ApplicationSpec{
			Functions: []FunctionDef{
				{
					Path:   "/hello",
					Runtime: "lua",
					Code:   "function handle() return {status=200} end",
				},
			},
		},
	}

	// Create a temporary repo directory
	repoPath := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("create repo dir: %v", err)
	}

	if err := syncer.SyncApplication(app, repoPath); err != nil {
		t.Fatalf("sync application: %v", err)
	}

	// Verify deployment was created
	d, err := deployStore.Get("alice", "/hello")
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if d.Runtime != "lua" {
		t.Errorf("expected lua, got %s", d.Runtime)
	}
}

