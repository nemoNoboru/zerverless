# Decoupled GitOps Architecture with Persistent Storage

## Overview

This document describes a decoupled, modular architecture where:
1. **GitOps creates jobs** instead of directly building/deploying
2. **Jobs and deployments persist** in a "zerverless" namespace database
3. **Workers handle Docker build/deploy jobs** asynchronously
4. **Data survives restarts** via BadgerDB persistence

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Orchestrator Node                        │
│                                                             │
│  ┌──────────────┐      ┌──────────────┐                   │
│  │ GitOps       │      │ Job Store    │                   │
│  │ Watcher      │─────▶│ (Persistent) │                   │
│  │              │      │              │                   │
│  └──────────────┘      └──────────────┘                   │
│         │                      │                            │
│         │ Creates jobs         │                            │
│         │                      ▼                            │
│         │              ┌──────────────┐                     │
│         │              │ Dispatcher   │                     │
│         │              │              │                     │
│         │              └──────────────┘                     │
│         │                      │                            │
│         │                      ▼                            │
│         │              ┌──────────────┐                     │
│         │              │ Deployment   │                     │
│         │              │ Store        │                     │
│         │              │ (Persistent) │                     │
│         │              └──────────────┘                     │
│         │                      │                            │
│         └──────────────────────┼────────────────────────────┘
│                                 │                            │
│                                 ▼                            │
│                    ┌────────────────────┐                   │
│                    │ BadgerDB            │                   │
│                    │ namespace:         │                   │
│                    │ "zerverless"       │                   │
│                    │                    │                   │
│                    │ - jobs/            │                   │
│                    │ - deployments/     │                   │
│                    │ - gitops/          │                   │
│                    └────────────────────┘                   │
└─────────────────────────────────────────────────────────────┘
                    │
                    │ (WebSocket)
                    ▼
         ┌──────────────────────┐
         │  Workers              │
         │  - docker-build       │
         │  - docker-deploy      │
         │  - docker-run         │
         └──────────────────────┘
```

## Persistent Storage Design

### "zerverless" Namespace Database

All system data stored in BadgerDB under `zerverless` namespace:

```
zerverless/
├── jobs/
│   ├── {job-id} → Job JSON
│   └── index/ → List of job IDs
├── deployments/
│   ├── {user}/{path} → Deployment JSON
│   └── index/{user} → List of paths
└── gitops/
    ├── applications/{name} → Application JSON
    └── sync-state/{name} → Last sync commit SHA
```

## Job Types

### 1. `docker-build` Job

Builds a Docker image from a Dockerfile in a Git repository.

```go
type DockerBuildJob struct {
    JobID          string            `json:"job_id"`
    JobType        string            `json:"job_type"` // "docker-build"
    Namespace      string            `json:"namespace"` // "zerverless"
    RepoPath       string            `json:"repo_path"` // Local path to cloned repo
    DockerfilePath string            `json:"dockerfile_path"` // Relative to repo
    ContextPath    string            `json:"context_path"` // Relative to repo
    ImageTag       string            `json:"image_tag"` // Output image tag
    BuildArgs      map[string]string `json:"build_args,omitempty"`
    TimeoutSeconds int               `json:"timeout_seconds"`
    Status         string            `json:"status"`
    Result         *BuildResult       `json:"result,omitempty"`
    Error          string            `json:"error,omitempty"`
    CreatedAt      time.Time         `json:"created_at"`
}

type BuildResult struct {
    ImageTag string `json:"image_tag"`
    BuildLog string `json:"build_log,omitempty"`
    Size     int64  `json:"size"` // Image size in bytes
}
```

### 2. `docker-deploy` Job

Deploys a built Docker image as a function endpoint.

```go
type DockerDeployJob struct {
    JobID          string    `json:"job_id"`
    JobType        string    `json:"job_type"` // "docker-deploy"
    Namespace      string    `json:"namespace"` // User namespace
    ImageTag       string    `json:"image_tag"` // Built image to deploy
    User           string    `json:"user"` // Deployment user
    Path           string    `json:"path"` // Function path
    Command        []string  `json:"command,omitempty"` // Override CMD
    Env            map[string]string `json:"env,omitempty"`
    TimeoutSeconds int       `json:"timeout_seconds"`
    Status         string    `json:"status"`
    Result         *DeployResult `json:"result,omitempty"`
    Error          string    `json:"error,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
}

type DeployResult struct {
    DeploymentKey string `json:"deployment_key"` // user+path
    ImageTag       string `json:"image_tag"`
}
```

## Implementation

### 1. Persistent Job Store

```go
// internal/job/persistent_store.go

package job

import (
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/zerverless/orchestrator/internal/db"
)

const SystemNamespace = "zerverless"

type PersistentStore struct {
    dbStore *db.Store
}

func NewPersistentStore(dbStore *db.Store) *PersistentStore {
    return &PersistentStore{dbStore: dbStore}
}

func (s *PersistentStore) Add(j *Job) error {
    data, err := json.Marshal(j)
    if err != nil {
        return fmt.Errorf("marshal job: %w", err)
    }
    
    key := fmt.Sprintf("jobs/%s", j.ID)
    if err := s.dbStore.Set(SystemNamespace, key, data); err != nil {
        return fmt.Errorf("store job: %w", err)
    }
    
    // Add to index
    indexKey := fmt.Sprintf("jobs/index/%s", j.ID)
    if err := s.dbStore.Set(SystemNamespace, indexKey, []byte(j.ID)); err != nil {
        return fmt.Errorf("add to index: %w", err)
    }
    
    return nil
}

func (s *PersistentStore) Get(id string) (*Job, error) {
    key := fmt.Sprintf("jobs/%s", id)
    data, err := s.dbStore.Get(SystemNamespace, key)
    if err != nil {
        return nil, fmt.Errorf("get job: %w", err)
    }
    
    var j Job
    if err := json.Unmarshal(data, &j); err != nil {
        return nil, fmt.Errorf("unmarshal job: %w", err)
    }
    
    return &j, nil
}

func (s *PersistentStore) Update(j *Job) error {
    return s.Add(j) // Same as Add for updates
}

func (s *PersistentStore) ListPending() ([]*Job, error) {
    // List all jobs, filter pending
    keys, err := s.dbStore.List(SystemNamespace, "jobs/", 0)
    if err != nil {
        return nil, err
    }
    
    var pending []*Job
    for _, key := range keys {
        if key == "jobs/index/" {
            continue
        }
        jobID := key[len("jobs/"):]
        j, err := s.Get(jobID)
        if err != nil {
            continue
        }
        if j.Status == StatusPending {
            pending = append(pending, j)
        }
    }
    
    return pending, nil
}

func (s *PersistentStore) NextPending() *Job {
    pending, err := s.ListPending()
    if err != nil || len(pending) == 0 {
        return nil
    }
    return pending[0] // FIFO - first in list
}
```

### 2. Persistent Deployment Store

```go
// internal/deploy/persistent_store.go

package deploy

import (
    "encoding/json"
    "fmt"
    
    "github.com/zerverless/orchestrator/internal/db"
)

const SystemNamespace = "zerverless"

type PersistentStore struct {
    dbStore *db.Store
}

func NewPersistentStore(dbStore *db.Store) *PersistentStore {
    return &PersistentStore{dbStore: dbStore}
}

func (s *PersistentStore) Set(d *Deployment) error {
    data, err := json.Marshal(d)
    if err != nil {
        return fmt.Errorf("marshal deployment: %w", err)
    }
    
    key := fmt.Sprintf("deployments/%s/%s", d.User, d.Path)
    if err := s.dbStore.Set(SystemNamespace, key, data); err != nil {
        return fmt.Errorf("store deployment: %w", err)
    }
    
    // Add to user index
    indexKey := fmt.Sprintf("deployments/index/%s/%s", d.User, d.Path)
    if err := s.dbStore.Set(SystemNamespace, indexKey, []byte(d.Path)); err != nil {
        return fmt.Errorf("add to index: %w", err)
    }
    
    return nil
}

func (s *PersistentStore) Get(user, path string) (*Deployment, error) {
    key := fmt.Sprintf("deployments/%s/%s", user, path)
    data, err := s.dbStore.Get(SystemNamespace, key)
    if err != nil {
        return nil, fmt.Errorf("get deployment: %w", err)
    }
    
    var d Deployment
    if err := json.Unmarshal(data, &d); err != nil {
        return nil, fmt.Errorf("unmarshal deployment: %w", err)
    }
    
    return &d, nil
}

func (s *PersistentStore) Delete(user, path string) error {
    key := fmt.Sprintf("deployments/%s/%s", user, path)
    if err := s.dbStore.Delete(SystemNamespace, key); err != nil {
        return fmt.Errorf("delete deployment: %w", err)
    }
    
    // Remove from index
    indexKey := fmt.Sprintf("deployments/index/%s/%s", user, path)
    s.dbStore.Delete(SystemNamespace, indexKey)
    
    return nil
}

func (s *PersistentStore) ListByUser(user string) ([]*Deployment, error) {
    prefix := fmt.Sprintf("deployments/%s/", user)
    keys, err := s.dbStore.List(SystemNamespace, prefix, 0)
    if err != nil {
        return nil, err
    }
    
    var deployments []*Deployment
    for _, key := range keys {
        if key == prefix+"index/" {
            continue
        }
        path := key[len(prefix):]
        d, err := s.Get(user, path)
        if err != nil {
            continue
        }
        deployments = append(deployments, d)
    }
    
    return deployments, nil
}

func (s *PersistentStore) ListAll() ([]*Deployment, error) {
    prefix := "deployments/"
    keys, err := s.dbStore.List(SystemNamespace, prefix, 0)
    if err != nil {
        return nil, err
    }
    
    var deployments []*Deployment
    seen := make(map[string]bool)
    
    for _, key := range keys {
        // Skip index entries
        if len(key) > len(prefix) && key[len(prefix):len(prefix)+6] == "index/" {
            continue
        }
        
        // Extract user and path
        rest := key[len(prefix):]
        parts := splitPath(rest)
        if len(parts) < 2 {
            continue
        }
        
        user := parts[0]
        path := parts[1]
        deploymentKey := user + path
        
        if seen[deploymentKey] {
            continue
        }
        seen[deploymentKey] = true
        
        d, err := s.Get(user, path)
        if err != nil {
            continue
        }
        deployments = append(deployments, d)
    }
    
    return deployments, nil
}
```

### 3. GitOps Job Creator

```go
// internal/gitops/syncer.go

package gitops

import (
    "fmt"
    "path/filepath"
    "time"
    
    "github.com/zerverless/orchestrator/internal/job"
)

type Syncer struct {
    watcher    *Watcher
    jobStore   *job.PersistentStore
    deployStore *deploy.PersistentStore
    baseDir    string
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
            // Deploy directly (no build step needed)
            deployment := &deploy.Deployment{
                User:    app.Metadata.Namespace,
                Path:    fn.Path,
                Runtime: fn.Runtime,
                Code:    fn.Code, // or read from CodeFile
            }
            s.deployStore.Set(deployment)
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
    
    // Resolve paths
    dockerfilePath := filepath.Join(repoPath, fn.Dockerfile)
    contextPath := filepath.Join(repoPath, fn.Context)
    
    return &job.Job{
        ID:             uuid.NewString(),
        JobType:        "docker-build",
        Namespace:      "zerverless", // System namespace
        Code:           "", // Not used for build jobs
        InputData: map[string]any{
            "repo_path":       repoPath,
            "dockerfile_path":  fn.Dockerfile,
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
        Code:           "", // Will be set from build job result
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
```

### 4. Worker Docker Job Handlers

```go
// internal/worker/worker.go (extended)

func (w *Worker) executeJob(ctx context.Context, conn *websocket.Conn, job JobMessage) {
    switch job.JobType {
    case "docker-build":
        w.executeDockerBuild(ctx, conn, job)
    case "docker-deploy":
        w.executeDockerDeploy(ctx, conn, job)
    case "docker", "docker-run":
        w.executeDockerRun(ctx, conn, job)
    // ... existing handlers
    }
}

func (w *Worker) executeDockerBuild(ctx context.Context, conn *websocket.Conn, job JobMessage) {
    // Extract build parameters
    repoPath := job.InputData["repo_path"].(string)
    dockerfilePath := job.InputData["dockerfile_path"].(string)
    contextPath := job.InputData["context_path"].(string)
    imageTag := job.InputData["image_tag"].(string)
    
    // Build image using Docker client
    builder := docker.NewBuilder(w.dockerClient)
    result, err := builder.BuildImage(ctx, repoPath, dockerfilePath, contextPath, imageTag)
    
    if err != nil {
        w.sendError(ctx, conn, job.JobID, err.Error())
        return
    }
    
    // Send result
    msg := ResultMessage{
        Type:   "result",
        JobID:  job.JobID,
        Result: map[string]any{
            "image_tag": result.ImageTag,
            "build_log": result.BuildLog,
            "size":      result.Size,
        },
    }
    wsjson.Write(ctx, conn, msg)
    w.sendReady(ctx, conn)
}

func (w *Worker) executeDockerDeploy(ctx context.Context, conn *websocket.Conn, job JobMessage) {
    // Get build job result to get image tag
    buildJobID := job.InputData["build_job_id"].(string)
    buildJob, err := w.getJobResult(buildJobID) // Fetch from orchestrator
    if err != nil {
        w.sendError(ctx, conn, job.JobID, fmt.Sprintf("get build job: %v", err))
        return
    }
    
    imageTag := buildJob.Result.(map[string]any)["image_tag"].(string)
    user := job.InputData["user"].(string)
    path := job.InputData["path"].(string)
    
    // Create deployment
    deployment := &deploy.Deployment{
        User:    user,
        Path:    path,
        Runtime: "docker",
        Code:    imageTag, // Store image tag as "code"
    }
    
    // Store deployment (via API or direct store access)
    if err := w.storeDeployment(deployment); err != nil {
        w.sendError(ctx, conn, job.JobID, fmt.Sprintf("store deployment: %v", err))
        return
    }
    
    // Send success
    msg := ResultMessage{
        Type:   "result",
        JobID:  job.JobID,
        Result: map[string]any{
            "deployment_key": deployment.Key(),
            "image_tag":      imageTag,
        },
    }
    wsjson.Write(ctx, conn, msg)
    w.sendReady(ctx, conn)
}
```

### 5. Job Dependency Handling

```go
// internal/job/dispatcher.go (extended)

func (d *Dispatcher) TryDispatch(volunteerID string) {
    // Get next pending job that has no pending dependencies
    j := d.store.NextPendingWithDependencies()
    if j == nil {
        return
    }
    
    // Check if dependencies are complete
    if !d.dependenciesComplete(j) {
        return // Wait for dependencies
    }
    
    // Dispatch as normal
    d.store.SetStatus(j.ID, StatusRunning, volunteerID)
    if !d.assignFn(j, volunteerID) {
        d.store.SetStatus(j.ID, StatusPending, "")
    }
}

func (d *Dispatcher) dependenciesComplete(j *Job) bool {
    // Check if this job has a build_job_id dependency
    if buildJobID, ok := j.InputData["build_job_id"].(string); ok {
        buildJob, err := d.store.Get(buildJobID)
        if err != nil {
            return false
        }
        return buildJob.Status == StatusCompleted
    }
    return true // No dependencies
}
```

## Startup Recovery

On orchestrator startup, load persisted data:

```go
// cmd/orchestrator/main.go

func runOrchestrator() {
    // ... existing setup
    
    // Initialize persistent stores
    dbManager := db.NewManager("./data")
    zerverlessStore, _ := dbManager.GetStore("zerverless")
    
    jobStore := job.NewPersistentStore(zerverlessStore)
    deployStore := deploy.NewPersistentStore(zerverlessStore)
    
    // Load pending jobs
    pending, _ := jobStore.ListPending()
    log.Printf("Recovered %d pending jobs from storage", len(pending))
    
    // Load deployments
    deployments, _ := deployStore.ListAll()
    log.Printf("Recovered %d deployments from storage", len(deployments))
    
    // ... continue startup
}
```

## Benefits

✅ **Decoupling**: GitOps only creates jobs, doesn't execute
✅ **Modularity**: Each component has single responsibility
✅ **Persistence**: Jobs and deployments survive restarts
✅ **Scalability**: Workers can handle build/deploy jobs independently
✅ **Reliability**: Failed jobs can be retried
✅ **Observability**: All jobs tracked in persistent store

## Migration Path

1. **Phase 1**: Add persistent stores alongside in-memory stores
2. **Phase 2**: Migrate job store to persistent
3. **Phase 3**: Migrate deployment store to persistent
4. **Phase 4**: Update GitOps to create jobs
5. **Phase 5**: Remove in-memory stores

## Example Flow

1. **GitOps detects change** → parses `zerverless.yaml`
2. **Creates docker-build job** → stores in persistent job store
3. **Creates docker-deploy job** → stores in persistent job store (with build_job_id)
4. **Dispatcher finds build job** → routes to worker with Docker capability
5. **Worker builds image** → returns image tag
6. **Dispatcher finds deploy job** → checks build job is complete
7. **Worker deploys** → stores deployment in persistent store
8. **Function available** → HTTP endpoint ready

All data persists across restarts!



