# Docker Testing Guide

This guide shows how to test Docker functionality in Zerverless.

## Prerequisites

1. **Docker daemon running**: Make sure Docker is running on your system
2. **Orchestrator running**: Start the orchestrator with `./orchestrator`
3. **Worker with Docker**: Start a worker that has Docker capability

## Test 1: Busybox Echo Job

Run a simple Docker job using busybox to echo "hello world":

```bash
./test_docker.sh
```

Or manually:

```bash
# Submit job
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "docker",
    "code": "busybox:latest",
    "input_data": {
      "command": ["echo", "hello world"]
    },
    "timeout_seconds": 30
  }'

# Get job ID from response, then check status:
curl http://localhost:8000/api/jobs/JOB_ID_HERE
```

## Test 2: Hello-World Image Job

Test with the official hello-world Docker image:

```bash
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "docker",
    "code": "hello-world:latest",
    "input_data": {},
    "timeout_seconds": 30
  }'
```

## Test 3: Deploy and Invoke Docker Function

Deploy a Docker image as a serverless function and invoke it:

```bash
# Deploy hello-world as a function
curl -X POST http://localhost:8000/api/deploy/test/hello-world \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "docker",
    "code": "hello-world:latest"
  }'

# Invoke the function
curl http://localhost:8000/test/hello-world
```

## Test 4: Deploy Busybox Echo Function

Deploy busybox with a custom command:

```bash
# Deploy
curl -X POST http://localhost:8000/api/deploy/test/echo \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "docker",
    "code": "busybox:latest"
  }'

# Note: This will use the default CMD from busybox
# For custom commands, you'd need to build a custom image
curl http://localhost:8000/test/echo
```

## Expected Results

### Busybox Echo Job
- **Status**: `completed`
- **Result**: Should contain "hello world" in the output

### Hello-World Job
- **Status**: `completed`
- **Result**: Should contain the hello-world message

### Docker Function Invocation
- **Status**: `200 OK`
- **Body**: Container output (stdout/stderr)

## Troubleshooting

1. **No Docker capability**: Make sure your worker reports Docker capability
   - Check worker logs for "Docker runtime enabled"
   - Check `/api/stats` to see volunteer capabilities

2. **Image not found**: Pull images first:
   ```bash
   docker pull busybox:latest
   docker pull hello-world:latest
   ```

3. **Job stuck in pending**: Check if there are volunteers with Docker capability:
   ```bash
   curl http://localhost:8000/api/stats
   ```

4. **Timeout errors**: Increase timeout_seconds in job request

## Advanced: Custom Docker Images

To use custom Docker images:

1. Build your image:
   ```bash
   docker build -t my-function:latest .
   ```

2. Make sure the image is available to the worker (same Docker daemon or push to registry)

3. Deploy or run as job using the image tag:
   ```bash
   curl -X POST http://localhost:8000/api/jobs \
     -H "Content-Type: application/json" \
     -d '{
       "job_type": "docker",
       "code": "my-function:latest",
       "input_data": {
         "command": ["/app/entrypoint.sh"],
         "env": {"MY_VAR": "value"}
       },
       "timeout_seconds": 60
     }'
   ```


