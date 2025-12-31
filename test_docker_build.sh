#!/bin/bash

# Test Docker image building from Dockerfiles

ORCHESTRATOR_URL="http://localhost:8000"
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

echo "=== Testing Docker Image Building ==="
echo ""
echo "Test directory: $TEST_DIR"
echo ""

# Create a simple Dockerfile
cat > "$TEST_DIR/Dockerfile" << 'EOF'
FROM alpine:latest
RUN echo "Building test image" > /build-info.txt
CMD ["cat", "/build-info.txt"]
EOF

# Create a simple app file
cat > "$TEST_DIR/app.txt" << 'EOF'
This is a test file in the build context
EOF

echo "Step 1: Created test Dockerfile and context"
echo "----------------------------------------"
echo "Dockerfile content:"
cat "$TEST_DIR/Dockerfile"
echo ""
echo ""

# Generate unique image tag
IMAGE_TAG="zerverless/test-build:$(date +%s)"

echo "Step 2: Submitting docker-build job"
echo "----------------------------------------"
echo "Image tag: $IMAGE_TAG"
echo ""

JOB_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/api/jobs" \
  -H "Content-Type: application/json" \
  -d "{
    \"job_type\": \"docker-build\",
    \"input_data\": {
      \"repo_path\": \"$TEST_DIR\",
      \"dockerfile_path\": \"Dockerfile\",
      \"context_path\": \".\",
      \"image_tag\": \"$IMAGE_TAG\"
    },
    \"timeout_seconds\": 300
  }")

echo "Job Response: $JOB_RESPONSE"
echo ""

# Extract job ID
JOB_ID=$(echo "$JOB_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
if [ -z "$JOB_ID" ]; then
  echo "ERROR: Could not extract job ID"
  exit 1
fi

echo "Job ID: $JOB_ID"
echo ""

# Wait for job to complete
echo "Step 3: Waiting for build to complete..."
echo "----------------------------------------"
for i in {1..60}; do
  sleep 2
  JOB_DATA=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$JOB_ID")
  JOB_STATUS=$(echo "$JOB_DATA" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
  echo "  [$i] Status: $JOB_STATUS"
  
  if [ "$JOB_STATUS" = "completed" ]; then
    echo ""
    echo "Build completed successfully!"
    echo ""
    echo "Job result:"
    echo "$JOB_DATA" | python3 -m json.tool 2>/dev/null || echo "$JOB_DATA"
    
    # Extract and display build log if available
    echo ""
    echo "Extracting build information..."
    RESULT=$(echo "$JOB_DATA" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('result', ''))" 2>/dev/null)
    if [ -n "$RESULT" ]; then
      echo "$RESULT" | python3 -m json.tool 2>/dev/null || echo "$RESULT"
    fi
    
    echo ""
    echo "Step 4: Testing the built image"
    echo "----------------------------------------"
    
    # Test running the built image
    echo "Submitting job to run the built image..."
    RUN_JOB_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/api/jobs" \
      -H "Content-Type: application/json" \
      -d "{
        \"job_type\": \"docker\",
        \"code\": \"$IMAGE_TAG\",
        \"input_data\": {},
        \"timeout_seconds\": 30
      }")
    
    RUN_JOB_ID=$(echo "$RUN_JOB_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    if [ -n "$RUN_JOB_ID" ]; then
      echo "Run job ID: $RUN_JOB_ID"
      echo "Waiting for image to run..."
      
      for j in {1..30}; do
        sleep 1
        RUN_JOB_DATA=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$RUN_JOB_ID")
        RUN_STATUS=$(echo "$RUN_JOB_DATA" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
        if [ "$RUN_STATUS" = "completed" ] || [ "$RUN_STATUS" = "failed" ]; then
          echo ""
          echo "Run job result:"
          echo "$RUN_JOB_DATA" | python3 -m json.tool 2>/dev/null || echo "$RUN_JOB_DATA"
          break
        fi
      done
    fi
    
    echo ""
    echo "✅ Docker build test PASSED"
    exit 0
  elif [ "$JOB_STATUS" = "failed" ]; then
    echo ""
    echo "❌ Build FAILED"
    echo ""
    echo "Job details:"
    echo "$JOB_DATA" | python3 -m json.tool 2>/dev/null || echo "$JOB_DATA"
    exit 1
  fi
done

echo ""
echo "❌ Build TIMEOUT - job did not complete within 2 minutes"
exit 1


