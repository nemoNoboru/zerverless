# Docker HTTP Proxy Setup and Testing

## Changes Made

### 1. Added Port Configuration
- Added `Port` field to `Deployment` struct
- Added `Port` field to `DeployRequest` 
- Port can be specified when deploying Docker functions
- Defaults to port 80 if not specified

### 2. Container Name Sanitization
- Fixed container name validation issue
- Container names now sanitized to comply with Docker naming rules
- Replaces `/` with `-` in deployment keys

### 3. Container Security Configuration
- Added tmpfs mounts for nginx writable directories
- Relaxed capability restrictions for web servers
- Containers can now run nginx and other web servers properly

## Testing Instructions

### 1. Restart Orchestrator

The orchestrator needs to be restarted to load the new code:

```bash
# Stop current orchestrator (if running)
pkill -f orchestrator

# Rebuild
make build

# Start orchestrator
./bin/zerverless

# In another terminal, start a worker with Docker support
./bin/zerverless --worker ws://localhost:8000/ws/volunteer
```

### 2. Run the Test

```bash
./test_docker_proxy.sh
```

### 3. Manual Test

```bash
# Build image (if not already built)
# ... build via docker-build job ...

# Deploy with port specification
curl -X POST http://localhost:8000/api/deploy/test/html-server \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "docker",
    "code": "zerverless/test-proxy:latest",
    "port": 80
  }'

# Wait a few seconds for container to start, then test
curl http://localhost:8000/test/html-server
```

## API Usage

### Deploy Docker Function with Port

```json
POST /api/deploy/{user}/{path}
{
  "runtime": "docker",
  "code": "image:tag",
  "port": 80
}
```

**Fields:**
- `runtime`: Must be `"docker"`
- `code`: Docker image tag
- `port`: (Optional) Container port to expose. Defaults to 80 if not specified

### Example Deployments

**Nginx (port 80):**
```json
{
  "runtime": "docker",
  "code": "nginx:alpine",
  "port": 80
}
```

**Node.js app (port 3000):**
```json
{
  "runtime": "docker",
  "code": "node:18",
  "port": 3000
}
```

**Python Flask (port 5000):**
```json
{
  "runtime": "docker",
  "code": "python:3.11",
  "port": 5000
}
```

## How It Works

1. **Deployment**: When you deploy a Docker function with a port:
   - Container starts automatically
   - Container port is mapped to a host port (9000-9999)
   - Container runs continuously

2. **Invocation**: When you request `/{user}/{path}`:
   - Request is proxied to the container's host port
   - Container handles the request
   - Response is proxied back to the client

3. **Port Mapping**: 
   - Container port (specified in deployment) → Host port (auto-allocated 9000-9999)
   - Example: Container port 80 → Host port 9000

## Troubleshooting

- **Container not starting**: Check Docker daemon is running
- **Port conflicts**: Ensure ports 9000-9999 are available
- **Connection refused**: Container may still be starting, wait a few seconds
- **Container restarting**: Check container logs: `docker logs <container-name>`

