#!/bin/bash

# Simple Docker test without jq dependency

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

echo "Job Response: $JOB_RESPONSE"
echo ""

# Extract job ID manually (simple approach)
JOB_ID=$(echo "$JOB_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
if [ -z "$JOB_ID" ]; then
  echo "ERROR: Could not extract job ID. Is the orchestrator running?"
  exit 1
fi

echo "Job ID: $JOB_ID"
echo ""

# Wait for job to complete
echo "Waiting for job to complete..."
for i in {1..30}; do
  sleep 1
  JOB_DATA=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$JOB_ID")
  JOB_STATUS=$(echo "$JOB_DATA" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
  echo "  [$i] Status: $JOB_STATUS"
  if [ "$JOB_STATUS" = "completed" ] || [ "$JOB_STATUS" = "failed" ]; then
    echo ""
    echo "Final job result:"
    echo "$JOB_DATA" | python3 -m json.tool 2>/dev/null || echo "$JOB_DATA"
    break
  fi
done

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

echo "Job Response: $JOB_RESPONSE2"
echo ""

JOB_ID2=$(echo "$JOB_RESPONSE2" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
if [ -z "$JOB_ID2" ]; then
  echo "ERROR: Could not extract job ID"
  exit 1
fi

echo "Job ID: $JOB_ID2"
echo ""

# Wait for job to complete
echo "Waiting for job to complete..."
for i in {1..30}; do
  sleep 1
  JOB_DATA2=$(curl -s "$ORCHESTRATOR_URL/api/jobs/$JOB_ID2")
  JOB_STATUS2=$(echo "$JOB_DATA2" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
  echo "  [$i] Status: $JOB_STATUS2"
  if [ "$JOB_STATUS2" = "completed" ] || [ "$JOB_STATUS2" = "failed" ]; then
    echo ""
    echo "Final job result:"
    echo "$JOB_DATA2" | python3 -m json.tool 2>/dev/null || echo "$JOB_DATA2"
    break
  fi
done

echo ""



