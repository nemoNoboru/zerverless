# Docker Container Support for Zerverless

## Overview

Adding Docker container support would allow Zerverless to execute arbitrary containerized workloads, significantly expanding the platform's capabilities beyond the current sandboxed runtimes (Wasm, Python, Lua, JS).

## Feasibility Analysis

### ✅ Technically Feasible

Yes, Docker-in-Docker (DinD) or Docker socket mounting is possible, but with important considerations:

1. **Docker-in-Docker (DinD)**
   - Worker runs inside a Docker container with Docker daemon
   - Requires privileged mode or Docker socket mounting
   - Higher resource overhead

2. **Docker Socket Mounting** (Recommended)
   - Worker runs on host with Docker socket access
   - More efficient, better resource utilization
   - Requires careful security configuration

3. **Rootless Docker**
   - Better security isolation
   - Some limitations on features
   - Good middle ground

## Architecture Changes Required

### 1. Worker Capabilities

Add Docker capability to volunteer capabilities:

```go
type Capabilities struct {
    Wasm        bool `json:"wasm"`
    Python      bool `json:"python"`
    Lua         bool `json:"lua"`
    JS          bool `json:"js"`
    Docker      bool `json:"docker"`  // NEW
    MaxMemoryMB int  `json:"max_memory_mb"`
}
```

### 2. Job Type Extension

Extend job types to support Docker:

```go
// JobMessage would need:
type JobMessage struct {
    // ... existing fields
    JobType        string `json:"job_type"` // "docker" for containers
    DockerImage    string `json:"docker_image,omitempty"`
    DockerCommand  []string `json:"docker_command,omitempty"`
    DockerEnv      map[string]string `json:"docker_env,omitempty"`
    DockerVolumes  map[string]string `json:"docker_volumes,omitempty"`
}
```

### 3. Worker Implementation

Add Docker execution handler:

```go
func (w *Worker) executeDocker(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
    // 1. Pull image (if needed)
    // 2. Create container with resource limits
    // 3. Inject input data (via env vars or volume)
    // 4. Execute container
    // 5. Capture stdout/stderr
    // 6. Cleanup container
    // 7. Return result
}
```

## Security Considerations

### Critical Security Requirements

1. **Resource Limits**
   - CPU limits (cgroups)
   - Memory limits
   - Disk I/O limits
   - Network restrictions

2. **Isolation**
   - Read-only root filesystem
   - No privileged mode
   - Restricted capabilities
   - Network namespace isolation
   - User namespace (rootless)

3. **Image Trust**
   - Image verification/signing
   - Allowed registries whitelist
   - Image scanning for vulnerabilities
   - Size limits

4. **Input/Output Sanitization**
   - Validate input data
   - Limit output size
   - Timeout enforcement

### Recommended Docker Run Configuration

```go
containerConfig := &container.Config{
    Image: job.DockerImage,
    Cmd:   job.DockerCommand,
    Env:   buildEnvVars(job.InputData),
    // Security
    User:  "nobody", // Non-root user
    ReadOnlyRootfs: true,
}

hostConfig := &container.HostConfig{
    // Resource limits
    Memory:     int64(job.MaxMemoryMB) * 1024 * 1024,
    MemorySwap: -1, // Disable swap
    CPUQuota:   int64(job.CPUQuota * 100000), // microseconds
    CPUPeriod:  100000,
    
    // Security
    Privileged: false,
    CapDrop:    []string{"ALL"},
    CapAdd:     []string{}, // Minimal capabilities
    
    // Network
    NetworkMode: "none", // Or isolated network
    
    // Volumes (read-only input, writable temp)
    Binds: []string{
        "/tmp/input:/input:ro",
        "/tmp/output:/output:rw",
    },
    
    // Auto-remove
    AutoRemove: true,
    
    // Timeout
    StopTimeout: int(timeout.Seconds()),
}
```

## Implementation Approach

### Phase 1: Basic Docker Support

1. **Worker Changes**
   - Add Docker client initialization
   - Implement `executeDocker()` method
   - Add Docker capability detection

2. **Orchestrator Changes**
   - Extend job submission API
   - Add Docker job type validation
   - Update capability matching

3. **API Changes**
   ```json
   POST /api/jobs
   {
     "job_type": "docker",
     "docker_image": "alpine:latest",
     "docker_command": ["echo", "Hello"],
     "input_data": {"key": "value"},
     "timeout_seconds": 30
   }
   ```

### Phase 2: Advanced Features

1. **Image Management**
   - Image caching
   - Pre-pull common images
   - Image size limits

2. **Volume Management**
   - Temporary volume creation
   - Input/output volume mounting
   - Automatic cleanup

3. **Network Isolation**
   - Per-container networks
   - Network policies
   - Firewall rules

### Phase 3: Production Hardening

1. **Security**
   - Image signing verification
   - Registry authentication
   - Security scanning integration

2. **Monitoring**
   - Container metrics
   - Resource usage tracking
   - Execution logs

3. **Optimization**
   - Container reuse pools
   - Image layer caching
   - Faster startup times

## Code Structure

### New Files Needed

```
internal/docker/
  ├── runtime.go          # Docker runtime implementation
  ├── runtime_test.go     # Tests
  ├── container.go        # Container management
  └── security.go         # Security policies
```

### Example Implementation

```go
package docker

import (
    "context"
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
)

type Runtime struct {
    client *client.Client
}

func NewRuntime() (*Runtime, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return nil, err
    }
    return &Runtime{client: cli}, nil
}

func (r *Runtime) Execute(ctx context.Context, config *ContainerConfig, timeout time.Duration) (*ExecutionResult, error) {
    // Pull image if needed
    // Create container with security settings
    // Start container
    // Wait for completion or timeout
    // Capture logs
    // Remove container
    // Return result
}
```

## Trade-offs

### Advantages

✅ **Universal Runtime Support**
- Run any language/framework
- Use existing container images
- Leverage container ecosystem

✅ **Isolation**
- Strong process isolation
- Resource limits via cgroups
- Network isolation

✅ **Reproducibility**
- Consistent execution environment
- Versioned images
- Dependency management

### Disadvantages

❌ **Security Complexity**
- Larger attack surface
- Requires careful configuration
- Ongoing security maintenance

❌ **Resource Overhead**
- Container startup time (100-500ms)
- Image pull time (seconds to minutes)
- Higher memory usage

❌ **Operational Complexity**
- Docker daemon dependency
- Image management
- Storage requirements

❌ **Browser Volunteers**
- Cannot run Docker in browser
- Only native workers can support
- Limits volunteer pool

## Comparison: Current vs Docker

| Aspect | Current (Wasm/Lua/JS) | Docker |
|--------|----------------------|--------|
| **Startup Time** | <10ms | 100-500ms |
| **Security** | Language-level sandbox | OS-level isolation |
| **Resource Overhead** | Low | Medium-High |
| **Browser Support** | ✅ Yes (Wasm/JS) | ❌ No |
| **Language Support** | Limited | Universal |
| **Image Size** | N/A | 10MB-1GB+ |
| **Cold Start** | Instant | Image pull + container start |

## Recommendations

### Option 1: Hybrid Approach (Recommended)

Keep current runtimes for:
- Fast, lightweight jobs
- Browser volunteers
- Simple code execution

Add Docker for:
- Complex workloads
- Specific language requirements
- Legacy applications
- Native workers only

### Option 2: Docker-Only

Replace all runtimes with Docker:
- More consistent
- Universal support
- Lose browser volunteers
- Higher overhead

### Option 3: Docker as Optional

- Workers advertise Docker capability
- Orchestrator routes Docker jobs only to capable workers
- Maintains backward compatibility
- Best of both worlds

## Implementation Priority

1. **High Priority**
   - Basic Docker execution
   - Resource limits
   - Security hardening

2. **Medium Priority**
   - Image caching
   - Volume management
   - Network isolation

3. **Low Priority**
   - Container pools
   - Advanced monitoring
   - Image signing

## Example Usage

### Job Submission

```bash
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "docker",
    "docker_image": "python:3.11-slim",
    "docker_command": ["python", "-c", "print(\"Hello from Docker!\")"],
    "timeout_seconds": 60
  }'
```

### Deployment

```bash
curl -X POST http://localhost:8000/api/deploy/user/my-service \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "docker",
    "docker_image": "myapp:latest",
    "docker_command": ["/app/entrypoint.sh"]
  }'
```

## Docker Image Building

### Feasibility: ✅ Yes, but with significant complexity

Building Docker images inside Zerverless is **technically feasible** but introduces additional security, resource, and operational challenges beyond just running containers.

### Architecture for Image Building

#### Job Type Extension

```go
type JobMessage struct {
    // ... existing fields
    
    // For running pre-built images
    DockerImage    string   `json:"docker_image,omitempty"`
    DockerCommand  []string `json:"docker_command,omitempty"`
    
    // For building images
    Dockerfile     string            `json:"dockerfile,omitempty"`      // Dockerfile content
    BuildContext   map[string]string `json:"build_context,omitempty"`   // Files for build
    BuildArgs      map[string]string `json:"build_args,omitempty"`      // Build arguments
    ImageTag       string            `json:"image_tag,omitempty"`       // Output image name
    BuildOnly      bool              `json:"build_only,omitempty"`    // Build but don't run
}
```

#### Build Workflow

1. **Build Phase** (if Dockerfile provided)
   - Create temporary build directory
   - Write Dockerfile
   - Write build context files
   - Execute `docker build`
   - Capture build logs
   - Tag resulting image
   - Cleanup build directory

2. **Run Phase** (if not build-only)
   - Use built image or pre-built image
   - Execute container as normal

### Implementation Example

```go
func (w *Worker) executeDocker(ctx context.Context, job JobMessage, timeout time.Duration) (*wasm.ExecutionResult, error) {
    var imageName string
    
    // Phase 1: Build image if Dockerfile provided
    if job.Dockerfile != "" {
        buildResult, err := w.buildDockerImage(ctx, job, timeout/2) // Use half timeout for build
        if err != nil {
            return nil, fmt.Errorf("build failed: %w", err)
        }
        imageName = buildResult.ImageTag
    } else {
        imageName = job.DockerImage
    }
    
    // Phase 2: Run container
    if !job.BuildOnly {
        return w.runDockerContainer(ctx, imageName, job, timeout/2)
    }
    
    return &wasm.ExecutionResult{
        Output: fmt.Sprintf("Image built: %s", imageName),
    }, nil
}

func (w *Worker) buildDockerImage(ctx context.Context, job JobMessage, timeout time.Duration) (*BuildResult, error) {
    // Create temporary build directory
    buildDir, err := os.MkdirTemp("", "zerverless-build-*")
    if err != nil {
        return nil, err
    }
    defer os.RemoveAll(buildDir)
    
    // Write Dockerfile
    dockerfilePath := filepath.Join(buildDir, "Dockerfile")
    if err := os.WriteFile(dockerfilePath, []byte(job.Dockerfile), 0644); err != nil {
        return nil, err
    }
    
    // Write build context files
    for path, content := range job.BuildContext {
        fullPath := filepath.Join(buildDir, path)
        os.MkdirAll(filepath.Dir(fullPath), 0755)
        if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
            return nil, err
        }
    }
    
    // Generate image tag
    imageTag := job.ImageTag
    if imageTag == "" {
        imageTag = fmt.Sprintf("zerverless/%s:%s", job.JobID[:8], time.Now().Format("20060102150405"))
    }
    
    // Build image with resource limits
    buildCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    
    buildOptions := types.ImageBuildOptions{
        Dockerfile:  "Dockerfile",
        Tags:        []string{imageTag},
        BuildArgs:   job.BuildArgs,
        Remove:      true, // Remove intermediate containers
        NoCache:     false, // Allow caching
        PullParent:  true, // Pull base images
    }
    
    tar, err := archive.TarWithOptions(buildDir, &archive.TarOptions{})
    if err != nil {
        return nil, err
    }
    
    buildResponse, err := w.dockerClient.ImageBuild(buildCtx, tar, buildOptions)
    if err != nil {
        return nil, err
    }
    defer buildResponse.Body.Close()
    
    // Stream build output
    var buildLog strings.Builder
    scanner := bufio.NewScanner(buildResponse.Body)
    for scanner.Scan() {
        line := scanner.Text()
        buildLog.WriteString(line + "\n")
        // Parse Docker build output for errors
    }
    
    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("build output error: %w", err)
    }
    
    return &BuildResult{
        ImageTag: imageTag,
        BuildLog: buildLog.String(),
    }, nil
}
```

### Security Considerations for Building

#### Critical Security Requirements

1. **Build Time Limits**
   - Maximum build time (e.g., 10 minutes)
   - Separate timeout from execution timeout
   - Kill build process on timeout

2. **Resource Limits During Build**
   - CPU limits for build process
   - Memory limits (prevent OOM)
   - Disk space limits (build context size)
   - Network restrictions (allowed registries only)

3. **Build Context Validation**
   - Maximum build context size (e.g., 100MB)
   - File count limits
   - Path traversal prevention
   - Forbidden file patterns

4. **Dockerfile Security**
   - Validate Dockerfile syntax
   - Restrict dangerous instructions:
     - `RUN` with network access
     - `COPY` from external sources
     - `FROM` only from whitelisted registries
   - Scan for malicious patterns
   - Maximum layer count

5. **Base Image Restrictions**
   - Only allow base images from trusted registries
   - Whitelist of allowed base images
   - Scan base images for vulnerabilities
   - Size limits on base images

6. **Build Isolation**
   - Build in isolated network namespace
   - No access to host filesystem (except temp dir)
   - Restricted Docker socket access
   - Build in separate container (DinD)

#### Recommended Build Configuration

```go
buildOptions := types.ImageBuildOptions{
    Dockerfile: "Dockerfile",
    Tags:       []string{imageTag},
    BuildArgs:  job.BuildArgs,
    
    // Security
    Remove:     true,  // Clean up intermediate containers
    NoCache:    false, // Allow caching for efficiency
    PullParent: true, // Always pull latest base
    
    // Resource limits (via Docker daemon config)
    // These would need to be set at daemon level
}

// Build in isolated container (DinD approach)
containerConfig := &container.Config{
    Image: "docker:dind", // Docker-in-Docker
    Cmd:   []string{"dockerd", "--host=unix:///var/run/docker.sock"},
    // ... security settings
}
```

### Build Context Management

#### Option 1: Inline Build Context (Simple)

```json
{
  "job_type": "docker",
  "dockerfile": "FROM python:3.11\nCOPY app.py /app/\nCMD [\"python\", \"/app/app.py\"]",
  "build_context": {
    "app.py": "print('Hello from built image!')"
  },
  "image_tag": "myapp:latest"
}
```

**Pros:**
- Simple to implement
- No external storage needed
- Works with current architecture

**Cons:**
- Limited by message size
- Not efficient for large contexts
- WebSocket message size limits

#### Option 2: Build Context Upload (Recommended)

```json
POST /api/jobs/build-context
{
  "job_id": "job-123",
  "files": {
    "Dockerfile": "...",
    "app.py": "...",
    "requirements.txt": "..."
  }
}
```

Then reference in job:

```json
{
  "job_type": "docker",
  "build_context_id": "job-123",
  "dockerfile": "Dockerfile",
  "image_tag": "myapp:latest"
}
```

**Pros:**
- Handles large build contexts
- Can reuse contexts
- Better for multi-file projects

**Cons:**
- Requires storage system
- More complex implementation
- Context cleanup needed

#### Option 3: Git Repository (Advanced)

```json
{
  "job_type": "docker",
  "build_from_git": {
    "url": "https://github.com/user/repo",
    "ref": "main",
    "path": "."
  },
  "dockerfile": "Dockerfile",
  "image_tag": "myapp:latest"
}
```

**Pros:**
- Natural workflow for developers
- Version control integration
- Large projects supported

**Cons:**
- Requires git client
- Network access needed
- Security (SSH keys, tokens)

### Build Caching Strategy

1. **Layer Caching**
   - Docker automatically caches layers
   - Reuse unchanged layers
   - Significant time savings

2. **Base Image Caching**
   - Pre-pull common base images
   - Cache on worker nodes
   - Reduce build time

3. **Build Context Caching**
   - Hash build context
   - Reuse if unchanged
   - Skip rebuild if identical

### Example API Usage

#### Build and Run

```bash
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "docker",
    "dockerfile": "FROM python:3.11-slim\nWORKDIR /app\nCOPY main.py .\nCMD [\"python\", \"main.py\"]",
    "build_context": {
      "main.py": "import sys\nprint(f\"Hello from {sys.version}\")"
    },
    "docker_command": ["python", "main.py"],
    "input_data": {},
    "timeout_seconds": 300
  }'
```

#### Build Only (for later use)

```bash
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "docker",
    "dockerfile": "FROM node:18\nWORKDIR /app\nCOPY package.json .\nRUN npm install\nCOPY . .",
    "build_context": {
      "package.json": "{\"dependencies\": {\"express\": \"^4.18.0\"}}",
      "index.js": "const express = require(\"express\"); const app = express(); app.get(\"/\", (req, res) => res.send(\"Hello\")); app.listen(3000);"
    },
    "image_tag": "myapp:v1.0.0",
    "build_only": true,
    "timeout_seconds": 600
  }'
```

#### Build with Arguments

```bash
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "docker",
    "dockerfile": "FROM python:3.11\nARG VERSION\nENV APP_VERSION=$VERSION\nRUN echo \"Building version $VERSION\"",
    "build_args": {
      "VERSION": "1.0.0"
    },
    "image_tag": "myapp:1.0.0",
    "build_only": true,
    "timeout_seconds": 300
  }'
```

### Resource Management

#### Build Resource Limits

```go
type BuildLimits struct {
    MaxBuildTime    time.Duration // e.g., 10 minutes
    MaxContextSize  int64         // e.g., 100MB
    MaxFileCount    int           // e.g., 1000 files
    MaxImageSize    int64         // e.g., 1GB
    MaxLayers       int           // e.g., 50 layers
    MaxBuildMemory  int64         // e.g., 2GB
}
```

#### Storage Management

- **Temporary Build Directories**: Auto-cleanup after build
- **Built Images**: 
  - Option 1: Keep on worker (fast, but storage limited)
  - Option 2: Push to registry (persistent, but requires registry)
  - Option 3: Auto-delete after use (saves space, but rebuild needed)

### Challenges and Solutions

#### Challenge 1: Build Time

**Problem**: Docker builds can take minutes to hours

**Solutions**:
- Separate build and execution phases
- Build caching (layer reuse)
- Pre-built base images
- Build time limits with cancellation

#### Challenge 2: Build Context Size

**Problem**: Large projects exceed message size limits

**Solutions**:
- Build context upload endpoint
- Streaming/chunked uploads
- Git repository support
- Compression

#### Challenge 3: Security

**Problem**: Building arbitrary Dockerfiles is dangerous

**Solutions**:
- Dockerfile validation and scanning
- Restricted base images
- Build in isolated container (DinD)
- Resource limits
- Build timeout enforcement

#### Challenge 4: Storage

**Problem**: Built images consume disk space

**Solutions**:
- Auto-cleanup policies
- Image registry integration
- Size limits
- LRU eviction

### Comparison: Build vs Pre-built

| Aspect | Building Images | Pre-built Images |
|--------|----------------|------------------|
| **Flexibility** | ✅ High - any Dockerfile | ❌ Limited to existing images |
| **Build Time** | ❌ 1-30+ minutes | ✅ Instant |
| **Security Risk** | ⚠️ Higher (arbitrary builds) | ✅ Lower (known images) |
| **Resource Usage** | ❌ High (CPU, disk, network) | ✅ Low |
| **Storage** | ❌ Need to store images | ✅ Pull on demand |
| **Use Case** | Custom environments, source builds | Standard runtimes, quick execution |

### Recommendations

#### Option 1: Build Support (Recommended for Advanced Use Cases)

**When to Use:**
- Custom runtime environments
- Source code compilation
- Dependency installation
- Multi-stage builds
- Development workflows

**Implementation Priority:**
1. Basic Dockerfile + inline context
2. Build context upload
3. Build caching
4. Git repository support

#### Option 2: Pre-built Only (Recommended for Production)

**When to Use:**
- Fast execution
- Security-sensitive workloads
- Standard runtimes
- Production deployments

**Implementation Priority:**
1. Image registry integration
2. Image caching
3. Image verification

#### Option 3: Hybrid Approach (Best of Both Worlds)

- Support both building and pre-built images
- Workers advertise build capability
- Orchestrator routes build jobs to capable workers
- Most flexible, but most complex

### Conclusion on Image Building

**Feasibility**: ✅ **Yes, but with significant complexity**

**Recommendation**: 
- **Start with pre-built images only** (Phase 1)
- **Add build support later** if needed (Phase 2)
- **Make building optional** - workers choose to enable it
- **Strict security** - validate, limit, and isolate builds

Building images adds powerful capabilities but introduces:
- Higher security risks
- Resource management complexity
- Longer execution times
- Storage requirements

The value depends on use cases - if users need custom environments or source builds, it's worth the complexity. For standard workloads, pre-built images are simpler and safer.

## Conclusion

Docker support is **feasible and valuable** but should be:
- **Optional** - Workers choose to enable it
- **Secure** - Strict resource limits and isolation
- **Complementary** - Works alongside existing runtimes
- **Native-only** - Browser volunteers cannot support it

The hybrid approach maintains the platform's strengths (fast, browser-friendly) while adding powerful container capabilities for advanced use cases.

**Image Building** adds another dimension:
- **Powerful** - Build from source, custom environments
- **Complex** - Security, resources, storage
- **Optional** - Should be opt-in feature
- **Use-case dependent** - Valuable for some, unnecessary for others

