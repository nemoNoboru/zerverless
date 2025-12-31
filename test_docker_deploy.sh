#!/bin/bash

# Test Docker deployment and invocation

ORCHESTRATOR_URL="http://localhost:8000"

echo "=== Testing Docker Deployment ==="
echo ""

# Deploy hello-world as a function
echo "Step 1: Deploying hello-world image as a function"
echo "----------------------------------------"
DEPLOY_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/api/deploy/test/hello-world" \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "docker",
    "code": "hello-world:latest"
  }')

echo "Deployment response: $DEPLOY_RESPONSE"
echo ""

# Wait a moment for deployment to be ready
sleep 2

# Invoke the deployed function
echo "Step 2: Invoking the deployed function"
echo "----------------------------------------"
INVOKE_RESPONSE=$(curl -s "$ORCHESTRATOR_URL/test/hello-world")
echo "Invocation response: $INVOKE_RESPONSE"
echo ""

# Test with busybox echo
echo "Step 3: Deploying busybox echo function"
echo "----------------------------------------"
DEPLOY_RESPONSE2=$(curl -s -X POST "$ORCHESTRATOR_URL/api/deploy/test/echo" \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "docker",
    "code": "busybox:latest"
  }')

echo "Deployment response: $DEPLOY_RESPONSE2"
echo ""

sleep 2

echo "Step 4: Invoking busybox function"
echo "----------------------------------------"
# Note: For busybox, we'd need to pass command via query params or body
# But since we're using the default CMD, let's just test the endpoint
INVOKE_RESPONSE2=$(curl -s "$ORCHESTRATOR_URL/test/echo")
echo "Invocation response: $INVOKE_RESPONSE2"
echo ""




