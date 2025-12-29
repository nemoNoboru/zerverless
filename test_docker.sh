#!/bin/bash

# Test Docker functionality with busybox and hello-world

ORCHESTRATOR_URL="http://localhost:8000"

echo "=== Testing Docker Jobs ==="
echo ""

# Test 1: Busybox echo "hello world"
echo "Test 1: Running busybox echo 'hello world'"
echo "----------------------------------------"
JOB_RESPONSE=$(curl -s -X POST "$ORCHESTRATOR_URL/api/jobs" \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "docker",
    "code": "busybox:latest",
    "input_data": {
      "command": ["echo", "hello world"]
    },
    "timeout_seconds": 30
  }')

JOB_ID=$(echo $JOB_RESPONSE | jq -r '.id')
echo "Job submitted: $JOB_ID"
echo "Response: $JOB_RESPONSE"
echo ""

# Wait for job to complete
echo "Waiting for job to complete..."
for i in {1..30}; do
  sleep 1
  JOB_STATUS=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$JOB_ID" | jq -r '.status')
  echo "  Status: $JOB_STATUS"
  if [ "$JOB_STATUS" = "completed" ] || [ "$JOB_STATUS" = "failed" ]; then
    break
  fi
done

echo ""
echo "Final job result:"
curl -s "$ORCHESTRATOR_URL/api/jobs/$JOB_ID" | jq '.'
echo ""
echo ""

# Test 2: Hello-world image
echo "Test 2: Running hello-world image"
echo "----------------------------------------"
JOB_RESPONSE2=$(curl -s -X POST "$ORCHESTRATOR_URL/api/jobs" \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "docker",
    "code": "hello-world:latest",
    "input_data": {},
    "timeout_seconds": 30
  }')

JOB_ID2=$(echo $JOB_RESPONSE2 | jq -r '.id')
echo "Job submitted: $JOB_ID2"
echo "Response: $JOB_RESPONSE2"
echo ""

# Wait for job to complete
echo "Waiting for job to complete..."
for i in {1..30}; do
  sleep 1
  JOB_STATUS2=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$JOB_ID2" | jq -r '.status')
  echo "  Status: $JOB_STATUS2"
  if [ "$JOB_STATUS2" = "completed" ] || [ "$JOB_STATUS2" = "failed" ]; then
    break
  fi
done

echo ""
echo "Final job result:"
curl -s "$ORCHESTRATOR_URL/api/jobs/$JOB_ID2" | jq '.'
echo ""


