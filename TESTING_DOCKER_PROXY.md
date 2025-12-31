# Testing Docker HTTP Proxying

## Prerequisites

1. **Restart the orchestrator** to load the new container manager code:
   ```bash
   # Stop current orchestrator
   pkill -f orchestrator
   
   # Rebuild and start
   make build
   ./bin/zerverless
   ```

2. **Ensure Docker is running** and accessible

3. **Ensure a worker with Docker capability** is connected

## Test Steps

1. **Run the test script**:
   ```bash
   ./test_docker_proxy.sh
   ```

2. **Or test manually**:

   ```bash
   # Build a simple nginx image
   mkdir test-html && cd test-html
   echo '<h1>Hello from Docker!</h1>' > index.html
   cat > Dockerfile << 'EOF'
   FROM nginx:alpine
   COPY index.html /usr/share/nginx/html/
   EXPOSE 80
   EOF
   
   # Build image (via zerverless)
   curl -X POST http://localhost:8000/api/jobs \
     -H "Content-Type: application/json" \
     -d '{
       "job_type": "docker-build",
       "input_data": {
         "repo_path": "/path/to/test-html",
         "dockerfile_path": "Dockerfile",
         "context_path": ".",
         "image_tag": "test-html:latest"
       }
     }'
   
   # Deploy
   curl -X POST http://localhost:8000/api/deploy/test/html \
     -H "Content-Type: application/json" \
     -d '{
       "runtime": "docker",
       "code": "test-html:latest"
     }'
   
   # Test
   curl http://localhost:8000/test/html
   ```

## Expected Behavior

1. **Deployment**: Container starts automatically on a host port (9000-9999)
2. **Invocation**: HTTP requests are proxied to the container
3. **Response**: HTML/content from container is returned

## Troubleshooting

- **"Container manager not available"**: Restart orchestrator
- **"Container exited with code 1"**: Check container logs: `docker logs <container-name>`
- **Port conflicts**: Check if ports 9000-9999 are available
- **Container not starting**: Check Docker daemon is running and accessible



