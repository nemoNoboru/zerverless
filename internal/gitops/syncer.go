package gitops

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/job"
)

type Syncer struct {
	watcher      *Watcher
	jobStore     job.JobStore
	deployStore  deploy.DeployStore
	storageStore StorageStore
	baseDir      string
}

type StorageStore interface {
	Put(namespace, path string, content []byte) error
}

func NewSyncer(watcher *Watcher, jobStore job.JobStore, deployStore deploy.DeployStore, storageStore StorageStore, baseDir string) *Syncer {
	return &Syncer{
		watcher:      watcher,
		jobStore:     jobStore,
		deployStore:  deployStore,
		storageStore: storageStore,
		baseDir:      baseDir,
	}
}

func (s *Syncer) SyncApplication(app *Application, repoPath string) error {
	log.Printf("Syncing application %s (namespace: %s)", app.Metadata.Name, app.Metadata.Namespace)

	for _, fn := range app.Spec.Functions {
		log.Printf("Processing function: path=%s, runtime=%s", fn.Path, fn.Runtime)

		if fn.Runtime == "docker" {
			// Create combined docker-build-deploy job
			buildDeployJob := s.createDockerBuildDeployJob(fn, repoPath, app.Metadata.Namespace)
			log.Printf("Created docker-build-deploy job %s for path %s (image: %s)", buildDeployJob.ID[:8], fn.Path, buildDeployJob.InputData["image_tag"])
			if err := s.jobStore.Add(buildDeployJob); err != nil {
				log.Printf("Failed to add build-deploy job: %v", err)
				return fmt.Errorf("create build-deploy job: %w", err)
			}
			log.Printf("Queued docker-build-deploy job %s", buildDeployJob.ID[:8])
		} else {
			// Handle other runtimes (lua, python, js)
			// Read code from file if CodeFile is specified
			code := fn.Code
			if fn.CodeFile != "" {
				codePath := filepath.Join(repoPath, fn.CodeFile)
				fileCode, err := os.ReadFile(codePath)
				if err != nil {
					return fmt.Errorf("read code file %s: %w", fn.CodeFile, err)
				}
				code = string(fileCode)
			}

			if code == "" {
				return fmt.Errorf("no code provided for function %s", fn.Path)
			}

			// Deploy directly (no build step needed)
			deployment := &deploy.Deployment{
				User:    app.Metadata.Namespace,
				Path:    fn.Path,
				Runtime: fn.Runtime,
				Code:    code,
			}
			log.Printf("Deploying function %s/%s (runtime: %s)", app.Metadata.Namespace, fn.Path, fn.Runtime)
			if err := s.deployStore.Set(deployment); err != nil {
				log.Printf("Failed to deploy function: %v", err)
				return fmt.Errorf("deploy function %s: %w", fn.Path, err)
			}
			log.Printf("Deployed function %s/%s", app.Metadata.Namespace, fn.Path)
		}
	}

	// Sync static files if configured
	if app.Spec.Static != nil && app.Spec.Static.Dir != "" {
		log.Printf("Syncing static files from %s", app.Spec.Static.Dir)
		if err := s.syncStaticFiles(app.Metadata.Namespace, repoPath, app.Spec.Static.Dir); err != nil {
			log.Printf("Failed to sync static files: %v", err)
			return fmt.Errorf("sync static files: %w", err)
		}
		log.Printf("Static files synced for namespace %s", app.Metadata.Namespace)
	}

	log.Printf("Application %s sync completed", app.Metadata.Name)
	return nil
}

func (s *Syncer) syncStaticFiles(namespace, repoPath, staticDir string) error {
	if s.storageStore == nil {
		log.Printf("Storage store not available, skipping static file sync")
		return nil
	}

	// repoPath from handler is relative to current working directory
	// (e.g., "gitops/zerverless.git/example" or "gitops/zerverless/example")
	// Make it absolute to resolve correctly
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	// Join with static directory (clean up any ./ prefix)
	staticDir = filepath.Clean(staticDir)
	staticPath := filepath.Join(absRepoPath, staticDir)

	log.Printf("Syncing static files from %s to namespace %s (repoPath=%s, staticDir=%s)", staticPath, namespace, repoPath, staticDir)

	// Walk the static directory and copy files to storage
	err = filepath.Walk(staticPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path from static directory
		relPath, err := filepath.Rel(staticPath, path)
		if err != nil {
			return err
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file %s: %w", path, err)
		}

		// Store in storage with namespace (no "static" prefix since route already has /static/)
		storagePath := relPath
		if err := s.storageStore.Put(namespace, storagePath, content); err != nil {
			return fmt.Errorf("store file %s: %w", storagePath, err)
		}

		log.Printf("Synced static file: %s -> %s/%s", relPath, namespace, storagePath)
		return nil
	})

	return err
}

func (s *Syncer) createDockerBuildDeployJob(
	fn FunctionDef,
	repoPath string,
	namespace string,
) *job.Job {
	// repoPath is already relative to current working directory (from handler)
	// Make it absolute to resolve correctly
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		// Fallback: join with baseDir if Abs fails
		absRepoPath = filepath.Join(s.baseDir, repoPath)
	}

	log.Printf("Docker build-deploy job - repoPath: %s, absRepoPath: %s", repoPath, absRepoPath)

	// Generate image tag
	imageTag := fmt.Sprintf("zerverless/%s%s:%s",
		namespace,
		fn.Path,
		time.Now().Format("20060102150405"),
	)

	return &job.Job{
		ID:        uuid.NewString(),
		JobType:   "docker-build-deploy",
		Namespace: "zerverless", // System namespace
		Code:      "",           // Not used for build-deploy jobs
		InputData: map[string]any{
			"repo_path":       absRepoPath,
			"dockerfile_path": fn.Dockerfile,
			"context_path":    fn.Context,
			"image_tag":       imageTag,
			"user":            namespace,
			"path":            fn.Path,
			"command":         []string{}, // Optional override
		},
		TimeoutSeconds: 660, // 11 minutes (10 for build + 1 for deploy)
		Status:         job.StatusPending,
		CreatedAt:      time.Now().UTC(),
	}
}
