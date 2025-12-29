package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/zerverless/orchestrator/internal/deploy"
	"github.com/zerverless/orchestrator/internal/job"
)

type Syncer struct {
	watcher     *Watcher
	jobStore    job.JobStore
	deployStore deploy.DeployStore
	baseDir     string
}

func NewSyncer(watcher *Watcher, jobStore job.JobStore, deployStore deploy.DeployStore, baseDir string) *Syncer {
	return &Syncer{
		watcher:     watcher,
		jobStore:    jobStore,
		deployStore: deployStore,
		baseDir:     baseDir,
	}
}

func (s *Syncer) SyncApplication(app *Application, repoPath string) error {
	for _, fn := range app.Spec.Functions {
		if fn.Runtime == "docker" {
			// Create docker-build job
			buildJob := s.createDockerBuildJob(fn, repoPath, app.Metadata.Namespace)
			if err := s.jobStore.Add(buildJob); err != nil {
				return fmt.Errorf("create build job: %w", err)
			}

			// Create docker-deploy job (depends on build)
			deployJob := s.createDockerDeployJob(fn, app.Metadata.Namespace, buildJob.ID)
			if err := s.jobStore.Add(deployJob); err != nil {
				return fmt.Errorf("create deploy job: %w", err)
			}
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
			if err := s.deployStore.Set(deployment); err != nil {
				return fmt.Errorf("deploy function %s: %w", fn.Path, err)
			}
		}
	}
	return nil
}

func (s *Syncer) createDockerBuildJob(
	fn FunctionDef,
	repoPath string,
	namespace string,
) *job.Job {
	// Generate image tag
	imageTag := fmt.Sprintf("zerverless/%s%s:%s",
		namespace,
		fn.Path,
		time.Now().Format("20060102150405"),
	)

	return &job.Job{
		ID:             uuid.NewString(),
		JobType:        "docker-build",
		Namespace:      "zerverless", // System namespace
		Code:           "",           // Not used for build jobs
		InputData: map[string]any{
			"repo_path":      repoPath,
			"dockerfile_path": fn.Dockerfile,
			"context_path":    fn.Context,
			"image_tag":       imageTag,
			"namespace":       namespace,
			"function_path":   fn.Path,
		},
		TimeoutSeconds: 600, // 10 minutes for builds
		Status:         job.StatusPending,
		CreatedAt:      time.Now().UTC(),
	}
}

func (s *Syncer) createDockerDeployJob(
	fn FunctionDef,
	namespace string,
	buildJobID string,
) *job.Job {
	return &job.Job{
		ID:             uuid.NewString(),
		JobType:        "docker-deploy",
		Namespace:      "zerverless", // System namespace
		Code:           "",            // Will be set from build job result
		InputData: map[string]any{
			"build_job_id": buildJobID, // Reference to build job
			"user":         namespace,
			"path":         fn.Path,
			"command":      []string{}, // Optional override
		},
		TimeoutSeconds: 60,
		Status:         job.StatusPending,
		CreatedAt:      time.Now().UTC(),
	}
}


