# GitOps Deployment System for Zerverless

## Overview

Simple GitOps system that watches a Git repository and automatically deploys functions and static files based on a single YAML configuration file.

## Core Concept

1. **Point Zerverless to a Git repository**
2. **Define deployments in `zerverless.yaml`**
3. **Auto-sync on changes** (polling every 5 minutes)
4. **Sync static files** from a directory in the repo

## YAML Schema

### Application Manifest (`zerverless.yaml`)

```yaml
apiVersion: zerverless.io/v1
kind: Application
metadata:
  name: my-app
  namespace: myuser
spec:
  source:
    repoURL: https://github.com/user/repo
    branch: main
  
  # Function deployments
  functions:
    - path: /api/hello
      runtime: lua
      code: |
        function handle(req)
          return {status=200, body="Hello from GitOps!"}
        end
    
    - path: /api/python
      runtime: python
      codeFile: handlers/python.py
    
    - path: /api/docker
      runtime: docker
      dockerfile: ./services/api/Dockerfile
      context: ./services/api
  
  # Static files directory (optional)
  static:
    dir: ./static  # Directory in repo to sync
```

## Architecture

### Components

1. **Git Watcher** - Polls repository every 5 minutes, clones/pulls on changes
2. **YAML Parser** - Parses `zerverless.yaml`, validates schema
3. **Sync Engine** - Deploys functions and syncs static files

### Data Flow

```
Git Repository
    ↓ (poll every 5min)
Git Watcher (clone/pull)
    ↓
Local Cache
    ↓ (parse zerverless.yaml)
YAML Parser
    ↓
Sync Engine
    ├─→ Deploy Functions → Deployment Store
    └─→ Sync Static Files → Storage Store
```

## Implementation

### File Structure

```
internal/gitops/
  ├── application.go       # Application CRD parsing
  ├── watcher.go           # Git polling and cloning
  ├── syncer.go            # Sync functions and static files
  └── application_test.go  # Tests
```

### API Endpoints

```bash
# Register Git repository
POST /api/gitops/applications
{
  "name": "my-app",
  "namespace": "myuser",
  "source": {
    "repoURL": "https://github.com/user/repo",
    "branch": "main"
  }
}

# List applications
GET /api/gitops/applications

# Get application status
GET /api/gitops/applications/{name}

# Manual sync
POST /api/gitops/applications/{name}/sync

# Delete application
DELETE /api/gitops/applications/{name}
```

## Example Repository Structure

```
my-repo/
├── zerverless.yaml          # Application manifest
├── handlers/
│   ├── hello.lua            # Function code
│   └── api.py               # Function code
├── docker/
│   ├── Dockerfile           # Docker deployment
│   └── app.py
└── static/                  # Static files directory
    ├── index.html
    ├── css/
    │   └── style.css
    └── js/
        └── app.js
```

### Example `zerverless.yaml`

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
    # Inline code
    - path: /hello
      runtime: lua
      code: |
        function handle(req)
          return {status=200, body="Hello!"}
        end
    
    # Reference to file
    - path: /api
      runtime: python
      codeFile: handlers/api.py
    
    # Docker deployment
    - path: /docker
      runtime: docker
      dockerfile: ./docker/Dockerfile
      context: ./docker
  
  static:
    dir: ./static
```

## Sync Process

1. **Poll Repository** (every 5 minutes)
   - Check latest commit SHA
   - If changed, proceed to sync

2. **Fetch Changes**
   - Clone (first time) or pull (updates)
   - Checkout branch

3. **Parse Manifest**
   - Load `zerverless.yaml`
   - Load referenced code files
   - Validate schema

4. **Sync Functions**
   - Deploy/update each function via deployment API
   - For Docker: build image from dockerfile, then deploy
   - Delete functions not in manifest

5. **Sync Static Files**
   - Copy files from `static.dir` to storage store
   - Delete files not in Git directory

## Security Considerations

1. **Authentication** - SSH keys or tokens for private repos
2. **Authorization** - Validate namespace matches user
3. **Code Validation** - Existing job timeout and resource limits apply

## Status

```json
{
  "name": "my-app",
  "namespace": "myuser",
  "source": {
    "repoURL": "https://github.com/user/repo",
    "branch": "main",
    "commit": "abc123..."
  },
  "lastSync": "2024-01-15T10:30:00Z",
  "status": "synced"
}
```

## Example Workflow

1. **Create `zerverless.yaml` in repo**
   ```yaml
   apiVersion: zerverless.io/v1
   kind: Application
   metadata:
     name: my-app
     namespace: myuser
   spec:
     source:
       repoURL: https://github.com/myuser/my-repo
       branch: main
     functions:
       - path: /api/hello
         runtime: lua
         code: |
           function handle(req)
             return {status=200, body="Hello!"}
           end
       - path: /api/docker
         runtime: docker
         dockerfile: ./docker/Dockerfile
         context: ./docker
     static:
       dir: ./static
   ```

2. **Register with Zerverless**
   ```bash
   curl -X POST http://localhost:8000/api/gitops/applications \
     -H "Content-Type: application/json" \
     -d '{
       "name": "my-app",
       "namespace": "myuser",
       "source": {
         "repoURL": "https://github.com/myuser/my-repo",
         "branch": "main"
       }
     }'
   ```

3. **Auto-sync**
   - Zerverless polls every 5 minutes
   - Detects changes and syncs automatically
   - Functions available at `/myuser/api/hello`
   - Static files at `/static/myuser/*`

4. **Update and deploy**
   ```bash
   # Edit zerverless.yaml or code files
   git commit -am "Update function"
   git push
   # Auto-syncs within 5 minutes
   ```

