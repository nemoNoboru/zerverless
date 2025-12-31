#!/bin/bash

# Test Docker image building with a more complex Dockerfile

ORCHESTRATOR_URL="http://localhost:8000"
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

echo "=== Testing Complex Docker Image Building ==="
echo ""
echo "Test directory: $TEST_DIR"
echo ""

# Create a more complex Dockerfile with multiple steps
cat > "$TEST_DIR/Dockerfile" << 'EOF'
FROM python:3.11-slim

WORKDIR /app

# Copy a file
COPY requirements.txt .
COPY app.py .

# Install dependencies (simulated)
RUN echo "Installing dependencies..." && \
    echo "flask==2.3.0" > requirements.txt && \
    echo "Dependencies installed" > /app/deps.txt

# Set environment variable
ENV APP_NAME=test-app
ENV APP_VERSION=1.0.0

# Create a file at build time
RUN echo "Built at $(date)" > /app/build-time.txt

CMD ["python", "-c", "import os; print(f'App: {os.environ.get(\"APP_NAME\")} v{os.environ.get(\"APP_VERSION\")}'); print(open('/app/build-time.txt').read())"]
EOF

# Create app.py
cat > "$TEST_DIR/app.py" << 'EOF'
print("Hello from Python app!")
EOF

# Create requirements.txt
cat > "$TEST_DIR/requirements.txt" << 'EOF'
flask==2.3.0
EOF

echo "Step 1: Created complex Dockerfile with multiple steps"
echo "----------------------------------------"
echo "Dockerfile:"
cat "$TEST_DIR/Dockerfile"
echo ""
echo ""

# Generate unique image tag
IMAGE_TAG="zerverless/test-complex:$(date +%s)"

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
    \"timeout_seconds\": 600
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
BUILD_START=$(date +%s)
for i in {1..120}; do
  sleep 3
  JOB_DATA=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$JOB_ID")
  JOB_STATUS=$(echo "$JOB_DATA" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
  echo "  [$i] Status: $JOB_STATUS"
  
  if [ "$JOB_STATUS" = "completed" ]; then
    BUILD_END=$(date +%s)
    BUILD_TIME=$((BUILD_END - BUILD_START))
    echo ""
    echo "Build completed successfully in ${BUILD_TIME} seconds!"
    echo ""
    
    # Extract build result
    RESULT_JSON=$(echo "$JOB_DATA" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('result', ''))" 2>/dev/null)
    if [ -n "$RESULT_JSON" ]; then
      echo "Build result:"
      echo "$RESULT_JSON" | python3 -m json.tool 2>/dev/null || echo "$RESULT_JSON"
      echo ""
      
      # Extract build log
      BUILD_LOG=$(echo "$RESULT_JSON" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('build_log', ''))" 2>/dev/null)
      if [ -n "$BUILD_LOG" ]; then
        echo "Build log (last 20 lines):"
        echo "$BUILD_LOG" | tail -20
        echo ""
      fi
      
      # Extract image size
      IMAGE_SIZE=$(echo "$RESULT_JSON" | python3 -c "import sys, json; data=json.load(sys.stdin); size=data.get('size', 0); print(f'{size/1024/1024:.2f} MB')" 2>/dev/null)
      echo "Image size: $IMAGE_SIZE"
      echo ""
    fi
    
    echo "Step 4: Testing the built image"
    echo "----------------------------------------"
    
    # Test running the built image
    echo "Running the built image..."
    RUN_JOB_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/api/jobs" \
      -H "Content-Type: application/json" \
      -d "{
        \"job_type\": \"docker\",
        \"code\": \"$IMAGE_TAG\",
        \"input_data\": {},
        \"timeout_seconds\": 60
      }")
    
    RUN_JOB_ID=$(echo "$RUN_JOB_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
    if [ -n "$RUN_JOB_ID" ]; then
      echo "Run job ID: $RUN_JOB_ID"
      echo "Waiting for image to run..."
      
      for j in {1..30}; do
        sleep 2
        RUN_JOB_DATA=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$RUN_JOB_ID")
        RUN_STATUS=$(echo "$RUN_JOB_DATA" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
        if [ "$RUN_STATUS" = "completed" ]; then
          echo ""
          echo "✅ Image ran successfully!"
          RUN_RESULT=$(echo "$RUN_JOB_DATA" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('result', ''))" 2>/dev/null)
          if [ -n "$RUN_RESULT" ]; then
            echo "Output:"
            echo "$RUN_RESULT"
          fi
          break
        elif [ "$RUN_STATUS" = "failed" ]; then
          echo ""
          echo "❌ Image run FAILED"
          echo "$RUN_JOB_DATA" | python3 -m json.tool 2>/dev/null || echo "$RUN_JOB_DATA"
          exit 1
        fi
      done
    fi
    
    echo ""
    echo "✅ Complex Docker build test PASSED"
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
echo "❌ Build TIMEOUT - job did not complete within 6 minutes"
exit 1


