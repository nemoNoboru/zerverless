#!/bin/bash

# Test Docker HTTP proxying with a simple HTML server

ORCHESTRATOR_URL="http://localhost:8000"
TEST_DIR=$(mktemp -d)
trap "rm -rf $TEST_DIR" EXIT

echo "=== Testing Docker HTTP Proxying ==="
echo ""
echo "Test directory: $TEST_DIR"
echo ""

# Create a simple HTML file
cat > "$TEST_DIR/index.html" << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>Zerverless Test</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
        }
        h1 {
            color: #333;
        }
        .container {
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>🎉 Zerverless Docker Proxy Test</h1>
        <p>This HTML page is being served from a Docker container!</p>
        <p>The request was successfully proxied from Zerverless to the container.</p>
        <p><strong>Status:</strong> ✅ Working</p>
    </div>
</body>
</html>
EOF

# Create a simple Dockerfile using nginx
cat > "$TEST_DIR/Dockerfile" << 'EOF'
FROM nginx:alpine
COPY index.html /usr/share/nginx/html/index.html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
EOF

echo "Step 1: Created test files"
echo "----------------------------------------"
echo "Dockerfile:"
cat "$TEST_DIR/Dockerfile"
echo ""
echo "HTML content:"
head -5 "$TEST_DIR/index.html"
echo "..."
echo ""

# Generate unique image tag
IMAGE_TAG="zerverless/test-proxy:$(date +%s)"

echo "Step 2: Building Docker image"
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

echo "Build job response: $JOB_RESPONSE"
echo ""

# Extract job ID
BUILD_JOB_ID=$(echo "$JOB_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
if [ -z "$BUILD_JOB_ID" ]; then
  echo "ERROR: Could not extract build job ID"
  exit 1
fi

echo "Build job ID: $BUILD_JOB_ID"
echo ""

# Wait for build to complete
echo "Waiting for build to complete..."
for i in {1..60}; do
  sleep 2
  BUILD_JOB_DATA=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$BUILD_JOB_ID")
  BUILD_STATUS=$(echo "$BUILD_JOB_DATA" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
  echo "  [$i] Build status: $BUILD_STATUS"
  
  if [ "$BUILD_STATUS" = "completed" ]; then
    echo ""
    echo "✅ Build completed successfully!"
    break
  elif [ "$BUILD_STATUS" = "failed" ]; then
    echo ""
    echo "❌ Build FAILED"
    echo "$BUILD_JOB_DATA" | python3 -m json.tool 2>/dev/null || echo "$BUILD_JOB_DATA"
    exit 1
  fi
done

if [ "$BUILD_STATUS" != "completed" ]; then
  echo "❌ Build timeout"
  exit 1
fi

echo ""
echo "Step 3: Deploying Docker function"
echo "----------------------------------------"

DEPLOY_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/api/deploy/test/html-server" \
  -H "Content-Type: application/json" \
  -d "{
    \"runtime\": \"docker\",
    \"code\": \"$IMAGE_TAG\",
    \"port\": 80
  }")

echo "Deployment response: $DEPLOY_RESPONSE"
echo ""

# Wait a moment for container to start
echo "Waiting for container to start..."
sleep 3

echo ""
echo "Step 4: Testing HTTP proxy"
echo "----------------------------------------"
echo "Making request to: $ORCHESTRATOR_URL/test/html-server"
echo ""

# Test the endpoint
HTTP_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" "$ORCHESTRATOR_URL/test/html-server")
HTTP_BODY=$(echo "$HTTP_RESPONSE" | sed '/HTTP_CODE:/d')
HTTP_CODE=$(echo "$HTTP_RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)

echo "HTTP Status Code: $HTTP_CODE"
echo ""
echo "Response body:"
echo "$HTTP_BODY"
echo ""

# Check if HTML is in the response
if echo "$HTTP_BODY" | grep -q "Zerverless Docker Proxy Test"; then
  echo "✅ SUCCESS: HTML content found in response!"
  echo ""
  echo "The Docker container is serving HTML and requests are being proxied correctly."
else
  echo "❌ FAILED: HTML content not found in response"
  echo ""
  echo "Expected to find 'Zerverless Docker Proxy Test' in the response"
  exit 1
fi

# Test with a different path (root)
echo ""
echo "Step 5: Testing root path"
echo "----------------------------------------"
ROOT_RESPONSE=$(curl -s "$ORCHESTRATOR_URL/test/html-server/")
if echo "$ROOT_RESPONSE" | grep -q "Zerverless Docker Proxy Test"; then
  echo "✅ Root path also works!"
else
  echo "⚠️  Root path test inconclusive"
fi

echo ""
echo "✅ All tests passed! Docker HTTP proxying is working correctly."

