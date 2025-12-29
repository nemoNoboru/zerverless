# Docker-in-Docker Integration with GitOps

## Overview

This document explains how to integrate Docker-in-Docker (DinD) with the GitOps subsystem to build and run Dockerfiles fetched from Git repositories.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Orchestrator Node                        │
│                                                             │
│  ┌──────────────┐      ┌──────────────┐                    │
│  │ GitOps       │      │ Docker       │                    │
│  │ Watcher      │─────▶│ Builder      │                    │
│  │              │      │ (DinD)       │                    │
│  └──────────────┘      └──────────────┘                    │
│         │                      │                            │
│         │                      │                            │
│         ▼                      ▼                            │
│  ┌──────────────┐      ┌──────────────┐                    │
│  │ Local Git    │      │ Docker       │                    │
│  │ Cache        │      │ Daemon       │                    │
│  │ (repo clone) │      │ (DinD)       │                    │
│  └──────────────┘      └──────────────┘                    │
│                                                             │
│         │                      │                            │
│         └──────────┬───────────┘                            │
│                    │                                        │
│                    ▼                                        │
│            ┌──────────────┐                                │
│            │ Deployment   │                                │
│            │ Store        │                                │
│            └──────────────┘                                │
└─────────────────────────────────────────────────────────────┘
                    │
                    │ (deploy)
                    ▼
         ┌──────────────────────┐
         │  Workers (Native)    │
         │  - Execute Docker    │
         │    containers        │
         └──────────────────────┘
```

## Docker-in-Docker Setup

### Option 1: DinD Container (Recommended for Orchestrator)

Run the orchestrator with a DinD sidecar container:

```yaml
# docker-compose.yml
services:
  orchestrator:
    image: zerverless/orchestrator:latest
    volumes:
      - ./data:/data
    depends_on:
      - docker-daemon
  
  docker-daemon:
    image: docker:dind
    privileged: true
    volumes:
      - docker-storage:/var/lib/docker
    environment:
      - DOCKER_TLS_CERTDIR=/certs
    command: dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2376
```

### Option 2: Docker Socket Mounting (Simpler, Less Secure)

Mount the host Docker socket into the orchestrator:

```yaml
services:
  orchestrator:
    image: zerverless/orchestrator:latest
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/data
```

## Implementation Flow

### 1. GitOps Sync Process

When GitOps detects a Docker function:

```go
// internal/gitops/syncer.go

type Syncer struct {
    watcher    *Watcher
    deployStore *deploy.Store
    dockerBuilder *docker.Builder
    baseDir    string
}

func (s *Syncer) SyncApplication(app *Application, repoPath string) error {
    for _, fn := range app.Spec.Functions {
        if fn.Runtime == "docker" {
            // Build Docker image from Git repository
            imageTag, err := s.buildDockerImage(fn, repoPath, app.Metadata.Namespace)
            if err != nil {
                return fmt.Errorf("build docker image for %s: %w", fn.Path, err)
            }
            
            // Deploy as function with image reference
            deployment := &deploy.Deployment{
                User:    app.Metadata.Namespace,
                Path:    fn.Path,
                Runtime: "docker",
                Code:    imageTag, // Store image tag as "code"
            }
            s.deployStore.Set(deployment)
        } else {
            // Handle other runtimes (lua, python, js)
            // ...
        }
    }
    return nil
}
```

### 2. Docker Builder Implementation

```go
// internal/docker/builder.go

package docker

import (
    "context"
    "fmt"
    "path/filepath"
    "time"
    
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    "github.com/docker/docker/pkg/archive"
)

type Builder struct {
    client *client.Client
    imagePrefix string // e.g., "zerverless"
}

func NewBuilder(dockerHost string) (*Builder, error) {
    cli, err := client.NewClientWithOpts(
        client.WithHost(dockerHost),
        client.WithAPIVersionNegotiation(),
    )
    if err != nil {
        return nil, fmt.Errorf("create docker client: %w", err)
    }
    
    return &Builder{
        client: cli,
        imagePrefix: "zerverless",
    }, nil
}

// BuildImage builds a Docker image from a Dockerfile in the Git repository
func (b *Builder) BuildImage(
    ctx context.Context,
    repoPath string,
    dockerfilePath string,
    contextPath string,
    namespace string,
    functionPath string,
) (string, error) {
    // Generate unique image tag
    imageTag := fmt.Sprintf("%s/%s%s:%s",
        b.imagePrefix,
        namespace,
        functionPath,
        time.Now().Format("20060102150405"),
    )
    
    // Resolve paths relative to repository root
    fullContextPath := filepath.Join(repoPath, contextPath)
    fullDockerfilePath := filepath.Join(repoPath, dockerfilePath)
    
    // Create build context tar
    buildCtx, err := archive.TarWithOptions(fullContextPath, &archive.TarOptions{
        ExcludePatterns: []string{".git"},
    })
    if err != nil {
        return "", fmt.Errorf("create build context: %w", err)
    }
    defer buildCtx.Close()
    
    // Build options
    dockerfileRelPath, err := filepath.Rel(fullContextPath, fullDockerfilePath)
    if err != nil {
        return "", fmt.Errorf("resolve dockerfile path: %w", err)
    }
    
    buildOptions := types.ImageBuildOptions{
        Dockerfile:  dockerfileRelPath,
        Tags:        []string{imageTag},
        Remove:      true,  // Remove intermediate containers
        NoCache:     false, // Allow caching
        PullParent:  true,  // Pull base images
    }
    
    // Build image
    buildResponse, err := b.client.ImageBuild(ctx, buildCtx, buildOptions)
    if err != nil {
        return "", fmt.Errorf("docker build: %w", err)
    }
    defer buildResponse.Body.Close()
    
    // Stream build output (optional, for logging)
    // Could parse buildResponse.Body for progress/errors
    
    return imageTag, nil
}

// CleanupOldImages removes old image versions to save space
func (b *Builder) CleanupOldImages(ctx context.Context, namespace, functionPath string, keepLatest int) error {
    // List images with prefix
    images, err := b.client.ImageList(ctx, types.ImageListOptions{
        Filters: filters.NewArgs(
            filters.Arg("reference", fmt.Sprintf("%s/%s%s:*", b.imagePrefix, namespace, functionPath)),
        ),
    })
    if err != nil {
        return err
    }
    
    // Sort by creation time, keep latest N, delete rest
    // Implementation details...
    
    return nil
}
```

### 3. Integration with GitOps Watcher

```go
// internal/gitops/syncer.go (continued)

func (s *Syncer) buildDockerImage(
    fn FunctionDef,
    repoPath string,
    namespace string,
) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
    defer cancel()
    
    // Build image
    imageTag, err := s.dockerBuilder.BuildImage(
        ctx,
        repoPath,
        fn.Dockerfile,
        fn.Context,
        namespace,
        fn.Path,
    )
    if err != nil {
        return "", fmt.Errorf("build failed: %w", err)
    }
    
    return imageTag, nil
}
```

### 4. Deployment Store Extension

Extend the deployment structure to support Docker:

```go
// internal/deploy/deploy.go

type Deployment struct {
    User      string    `json:"user"`
    Path      string    `json:"path"`
    Runtime   string    `json:"runtime"`
    Code      string    `json:"code"`      // For lua/python/js: code content
    ImageTag  string    `json:"image_tag,omitempty"` // For docker: image tag
    CreatedAt time.Time `json:"created_at"`
}

// For Docker deployments, Code field contains image tag
// For other runtimes, Code field contains source code
```

### 5. Function Invocation (Worker Side)

When a Docker function is invoked, the worker needs to:

```go
// internal/worker/worker.go

func (w *Worker) executeDocker(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
    // Image tag is stored in job.Code for Docker runtime
    imageTag := job.Code
    
    // Create container config
    containerConfig := &container.Config{
        Image: imageTag,
        Cmd:   []string{}, // Use CMD from Dockerfile, or override
        Env:   buildEnvVars(job.InputData),
    }
    
    hostConfig := &container.HostConfig{
        Memory:     int64(256) * 1024 * 1024, // 256MB default
        MemorySwap: -1,
        AutoRemove: true,
        NetworkMode: "none", // Isolated network
    }
    
    // Create and start container
    resp, err := w.dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
    if err != nil {
        return nil, fmt.Errorf("create container: %w", err)
    }
    
    // Start container
    if err := w.dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
        return nil, fmt.Errorf("start container: %w", err)
    }
    
    // Wait for completion or timeout
    statusCh, errCh := w.dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
    
    select {
    case err := <-errCh:
        return nil, err
    case status := <-statusCh:
        // Get logs
        logs, _ := w.dockerClient.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
            ShowStdout: true,
            ShowStderr: true,
        })
        
        return &wasm.ExecutionResult{
            Output: readLogs(logs),
            Error:  status.Error,
        }, nil
    case <-time.After(timeout):
        // Timeout - stop container
        w.dockerClient.ContainerStop(ctx, resp.ID, container.StopOptions{})
        return nil, fmt.Errorf("execution timeout")
    }
}
```

## Complete Workflow Example

### 1. Git Repository Structure

```
my-repo/
├── zerverless.yaml
└── services/
    └── api/
        ├── Dockerfile
        └── app.py
```

### 2. zerverless.yaml

```yaml
apiVersion: zerverless.io/v1
kind: Application
metadata:
  name: my-app
  namespace: alice
spec:
  source:
    repoURL: https://github.com/user/my-repo
    branch: main
  functions:
    - path: /api
      runtime: docker
      dockerfile: ./services/api/Dockerfile
      context: ./services/api
```

### 3. Dockerfile

```dockerfile
FROM python:3.11-slim

WORKDIR /app
COPY app.py .

# Function handler interface
ENV PYTHONUNBUFFERED=1

CMD ["python", "app.py"]
```

### 4. Execution Flow

1. **GitOps Watcher** polls repository every 5 minutes
2. **Detects changes** → pulls latest code
3. **Parses zerverless.yaml** → finds Docker function
4. **Docker Builder** builds image:
   - Context: `./services/api`
   - Dockerfile: `./services/api/Dockerfile`
   - Tag: `zerverless/alice/api:20240101120000`
5. **Deployment Store** stores:
   - Path: `/api`
   - Runtime: `docker`
   - ImageTag: `zerverless/alice/api:20240101120000`
6. **Function Invocation**:
   - HTTP request to `/alice/api`
   - Worker pulls image (if needed)
   - Creates container
   - Executes
   - Returns response

## Security Considerations

### 1. Build Isolation

```go
// Build in isolated container (DinD)
buildOptions := types.ImageBuildOptions{
    // ... options
    // Resource limits enforced by DinD container
}
```

### 2. Image Tagging Strategy

- Include namespace to prevent collisions
- Include function path for uniqueness
- Include timestamp for versioning
- Format: `zerverless/{namespace}{path}:{timestamp}`

### 3. Resource Limits

```go
hostConfig := &container.HostConfig{
    Memory:     int64(maxMemoryMB) * 1024 * 1024,
    MemorySwap: -1,
    CPUQuota:   int64(cpuQuota * 100000),
    CPUPeriod:  100000,
}
```

### 4. Image Cleanup

- Keep only latest N versions per function
- Delete old images periodically
- Limit total image storage

## Configuration

### Orchestrator Configuration

```go
// internal/config/config.go

type Config struct {
    // ... existing fields
    
    Docker struct {
        Enabled     bool   `env:"DOCKER_ENABLED" envDefault:"false"`
        Host        string `env:"DOCKER_HOST" envDefault:"unix:///var/run/docker.sock"`
        ImagePrefix string `env:"DOCKER_IMAGE_PREFIX" envDefault:"zerverless"`
        BuildTimeout time.Duration `env:"DOCKER_BUILD_TIMEOUT" envDefault:"10m"`
    }
}
```

### Environment Variables

```bash
# Enable Docker support
DOCKER_ENABLED=true

# Docker daemon connection (DinD or socket)
DOCKER_HOST=unix:///var/run/docker.sock
# OR for DinD:
DOCKER_HOST=tcp://docker-daemon:2376

# Image naming
DOCKER_IMAGE_PREFIX=zerverless

# Build timeout
DOCKER_BUILD_TIMEOUT=10m
```

## Advantages of DinD Approach

✅ **Isolation**: Build process isolated in separate container
✅ **Security**: Can run with restricted privileges
✅ **Portability**: Works in containerized environments
✅ **Consistency**: Same Docker environment everywhere

## Disadvantages

❌ **Resource Overhead**: Additional container for Docker daemon
❌ **Complexity**: More moving parts to manage
❌ **Performance**: Slightly slower than direct socket access

## Alternative: Docker Socket Mounting

For simpler setups, mount host Docker socket:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

**Pros**: Simpler, faster
**Cons**: Less secure, requires host Docker access

## Implementation Checklist

- [ ] Create `internal/docker/builder.go` with DinD client
- [ ] Extend `internal/deploy/deploy.go` with ImageTag field
- [ ] Update `internal/gitops/syncer.go` to build Docker images
- [ ] Add Docker capability to worker
- [ ] Implement `executeDocker` in worker
- [ ] Add image cleanup logic
- [ ] Add configuration options
- [ ] Add tests for Docker builder
- [ ] Update documentation

## Testing

```go
// internal/docker/builder_test.go

func TestBuildImage(t *testing.T) {
    builder, err := NewBuilder("unix:///var/run/docker.sock")
    require.NoError(t, err)
    
    ctx := context.Background()
    imageTag, err := builder.BuildImage(
        ctx,
        "./testdata/repo",
        "./testdata/Dockerfile",
        "./testdata",
        "testuser",
        "/test",
    )
    
    require.NoError(t, err)
    assert.Contains(t, imageTag, "zerverless/testuser/test")
}
```

## Conclusion

Docker-in-Docker integration with GitOps enables:
- **Automatic builds** from Git repositories
- **Versioned deployments** with image tags
- **Isolated execution** in containers
- **GitOps workflow** for containerized functions

The orchestrator builds images when GitOps detects changes, stores image tags in the deployment store, and workers execute containers on demand.


