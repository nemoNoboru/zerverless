# GitOps Implementation Guide

## Current Status

✅ **Phase 1 Complete**: Basic YAML parsing
- Application manifest structure
- Function definition parsing
- Test coverage

✅ **Phase 2 Complete**: Git Integration
- Git repository cloning (go-git)
- Polling support (HasChanges method)
- Authentication (SSH keys, tokens)
- Branch checkout
- Sync method (clone/pull/checkout combined)

## Next Steps

### Phase 3: Sync Engine
- [ ] Deploy/update functions via deployment API
- [ ] Build Docker images from dockerfile (if runtime=docker)
- [ ] Sync static files to storage store
- [ ] Delete removed functions/files
- [ ] Status tracking

### Phase 4: API Integration
- [ ] Register Git repository endpoint
- [ ] List applications endpoint
- [ ] Manual sync trigger

## Usage Example

### 1. Create `zerverless.yaml` in your repo

```yaml
apiVersion: zerverless.io/v1
kind: Application
metadata:
  name: my-services
  namespace: myuser
spec:
  source:
    repoURL: https://github.com/myuser/my-repo
    branch: main
  functions:
    - path: /hello
      runtime: lua
      code: |
        function handle(req)
          return {status=200, body="Hello from GitOps!"}
        end
    - path: /docker
      runtime: docker
      dockerfile: ./docker/Dockerfile
      context: ./docker
  static:
    dir: ./static
```

### 2. Register with Zerverless

```bash
curl -X POST http://localhost:8000/api/gitops/applications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-services",
    "namespace": "myuser",
    "source": {
      "repoURL": "https://github.com/myuser/my-repo",
      "branch": "main"
    }
  }'
```

### 3. Auto-sync
- Zerverless polls every 5 minutes
- Detects changes
- Syncs functions and static files automatically

## Architecture

```
┌─────────────────┐
│  Git Repository │
│  (GitHub/GitLab)│
└────────┬────────┘
         │
         │ (poll/webhook)
         ▼
┌─────────────────┐
│  Git Watcher     │
│  - Clone/Pull    │
│  - Checkout      │
└────────┬────────┘
         │
         │ (parse)
         ▼
┌─────────────────┐
│  YAML Parser    │
│  - Parse manifest│
│  - Validate      │
└────────┬────────┘
         │
         │ (sync)
         ▼
┌─────────────────┐
│  Sync Engine     │
│  - Diff          │
│  - Apply         │
└────────┬────────┘
         │
         │ (deploy)
         ▼
┌─────────────────┐
│ Deployment Store│
│  - Create/Update│
│  - Delete        │
└─────────────────┘
```

## File Structure

```
internal/gitops/
  ├── application.go       ✅ Application CRD parsing
  ├── application_test.go  ✅ Tests
  ├── watcher.go           ✅ Git polling and cloning
  ├── watcher_test.go      ✅ Watcher tests
  └── syncer.go            ⏳ Sync functions and static files
```

## Testing

Run tests:
```bash
go test ./internal/gitops/... -v
```

## Dependencies

- `gopkg.in/yaml.v3` - YAML parsing
- `github.com/go-git/go-git/v5` - Git operations ✅

## Docker Support

When `runtime: docker` is specified:
1. Build Docker image from `dockerfile` in `context` directory
2. Deploy as function at specified path
3. Rebuild on changes to dockerfile or context files

## Static File Hosting

When `static.dir` is specified in the manifest:
1. Sync all files from that directory to storage store
2. Files accessible at `/static/{namespace}/*`
3. Delete files not present in Git directory
4. Preserve directory structure

