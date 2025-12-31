#!/bin/bash

set -e

ORCHESTRATOR_URL="http://localhost:8000"
REPO_URL="https://github.com/nemoNoboru/small-zerverless-test-flask-docker"
NAMESPACE="example"
APP_NAME="flask-example"
FUNCTION_PATH="/flask-example"  # Matches the path in zerverless.yaml manifest

echo "=========================================="
echo "Zerverless GitOps Deployment Test"
echo "=========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if orchestrator is running
echo "1. Checking orchestrator health..."
if ! curl -s "${ORCHESTRATOR_URL}/health" > /dev/null; then
    echo -e "${RED}Error: Orchestrator is not running at ${ORCHESTRATOR_URL}${NC}"
    echo "Please start it with: ./bin/zerverless --workers 2"
    exit 1
fi
echo -e "${GREEN}✓ Orchestrator is healthy${NC}"
echo ""

# Get initial stats
echo "2. Initial system stats:"
INITIAL_STATS=$(curl -s "${ORCHESTRATOR_URL}/stats")
echo "$INITIAL_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(f\"  Jobs: {d['jobs']['pending']} pending, {d['jobs']['running']} running\"); print(f\"  Volunteers: {d['volunteers']['idle']} idle, {d['volunteers']['busy']} busy\")"
echo ""

# Register application
echo "3. Registering GitOps application..."
REGISTER_RESPONSE=$(curl -s -X POST "${ORCHESTRATOR_URL}/api/gitops/applications" \
    -H "Content-Type: application/json" \
    -d "{
        \"name\": \"${APP_NAME}\",
        \"namespace\": \"${NAMESPACE}\",
        \"source\": {
            \"repoURL\": \"${REPO_URL}\",
            \"branch\": \"main\"
        }
    }")

if echo "$REGISTER_RESPONSE" | grep -q "error"; then
    echo -e "${RED}✗ Failed to register application:${NC}"
    echo "$REGISTER_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$REGISTER_RESPONSE"
    exit 1
fi

echo -e "${GREEN}✓ Application registered${NC}"
echo "$REGISTER_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$REGISTER_RESPONSE"
echo ""

# Wait for jobs to be created
echo "4. Waiting for jobs to be created..."
sleep 3

# Find the build-deploy job
echo "5. Checking job status..."
JOBS=$(curl -s "${ORCHESTRATOR_URL}/api/jobs?status=pending&limit=10")
JOB_ID=$(echo "$JOBS" | python3 -c "import sys, json; data=json.load(sys.stdin); jobs=[j for j in data.get('jobs', []) if j.get('job_type')=='docker-build-deploy']; print(jobs[0]['id'] if jobs else '')" 2>/dev/null)

if [ -z "$JOB_ID" ]; then
    echo -e "${YELLOW}⚠ No docker-build-deploy job found. Checking all jobs...${NC}"
    curl -s "${ORCHESTRATOR_URL}/api/jobs?limit=5" | python3 -c "import sys, json; data=json.load(sys.stdin); [print(f\"  {j['job_type']:20} {j['id'][:8]} - {j.get('status')}\") for j in data.get('jobs', [])[:5]]"
    exit 1
fi

echo -e "${GREEN}✓ Found job: ${JOB_ID:0:8}${NC}"
echo ""

# Monitor job progress
echo "6. Monitoring job progress (this may take a few minutes)..."
MAX_WAIT=300  # 5 minutes
ELAPSED=0
INTERVAL=5

while [ $ELAPSED -lt $MAX_WAIT ]; do
    JOB_STATUS=$(curl -s "${ORCHESTRATOR_URL}/api/jobs/${JOB_ID}" 2>/dev/null | python3 -c "import sys, json; j=json.load(sys.stdin); print(j.get('status', 'unknown'))" 2>/dev/null || echo "unknown")
    
    if [ "$JOB_STATUS" = "completed" ]; then
        echo -e "${GREEN}✓ Job completed!${NC}"
        break
    elif [ "$JOB_STATUS" = "failed" ]; then
        echo -e "${RED}✗ Job failed!${NC}"
        curl -s "${ORCHESTRATOR_URL}/api/jobs/${JOB_ID}" | python3 -m json.tool 2>/dev/null | grep -A 5 "error\|result" || true
        exit 1
    elif [ "$JOB_STATUS" = "running" ]; then
        echo "  Job is running... (${ELAPSED}s elapsed)"
    else
        echo "  Job status: $JOB_STATUS (${ELAPSED}s elapsed)"
    fi
    
    sleep $INTERVAL
    ELAPSED=$((ELAPSED + INTERVAL))
done

if [ $ELAPSED -ge $MAX_WAIT ]; then
    echo -e "${RED}✗ Job did not complete within ${MAX_WAIT} seconds${NC}"
    exit 1
fi

echo ""

# Check deployment
echo "7. Checking deployment status..."
DEPLOYMENTS=$(curl -s "${ORCHESTRATOR_URL}/api/deploy")
DEPLOYMENT_EXISTS=$(echo "$DEPLOYMENTS" | python3 -c "import sys, json; data=json.load(sys.stdin); deps=data.get('deployments', []); print('yes' if any(d['user']=='${NAMESPACE}' and d['path']=='${FUNCTION_PATH}' for d in deps) else 'no')" 2>/dev/null || echo "no")

if [ "$DEPLOYMENT_EXISTS" = "yes" ]; then
    echo -e "${GREEN}✓ Deployment found: ${NAMESPACE}${FUNCTION_PATH}${NC}"
    echo "$DEPLOYMENTS" | python3 -c "import sys, json; data=json.load(sys.stdin); deps=[d for d in data.get('deployments', []) if d.get('user')=='${NAMESPACE}' and d.get('path')=='${FUNCTION_PATH}']; [print(f\"  User: {d['user']}\"); print(f\"  Path: {d['path']}\"); print(f\"  Runtime: {d['runtime']}\"); print(f\"  Image: {d.get('code', 'N/A')[:60]}\") for d in deps]" 2>/dev/null || true
else
    echo -e "${YELLOW}⚠ Deployment not found yet${NC}"
    echo "$DEPLOYMENTS" | python3 -m json.tool 2>/dev/null || echo "$DEPLOYMENTS"
fi
echo ""

# Test the endpoint
echo "8. Testing deployed endpoint..."
ENDPOINT="${ORCHESTRATOR_URL}/${NAMESPACE}${FUNCTION_PATH}/hello"
echo "  Testing: ${ENDPOINT}"

RESPONSE=$(curl -s -w "\n%{http_code}" "${ENDPOINT}" 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "200" ]; then
    echo -e "${GREEN}✓ Endpoint responded successfully${NC}"
    echo "  Response:"
    echo "$BODY" | python3 -m json.tool 2>/dev/null | head -10 || echo "$BODY" | head -5
else
    echo -e "${YELLOW}⚠ Endpoint returned HTTP ${HTTP_CODE}${NC}"
    echo "  Response: $BODY"
fi
echo ""

# Test static files
echo "9. Testing static files..."
STATIC_URL="${ORCHESTRATOR_URL}/static/${NAMESPACE}/index.html"
STATIC_RESPONSE=$(curl -s -w "\n%{http_code}" "${STATIC_URL}" 2>&1)
STATIC_CODE=$(echo "$STATIC_RESPONSE" | sed -n '$p')

if [ "$STATIC_CODE" = "200" ]; then
    echo -e "${GREEN}✓ Static files are accessible${NC}"
else
    echo -e "${YELLOW}⚠ Static files returned HTTP ${STATIC_CODE}${NC}"
fi
echo ""

# Final stats
echo "10. Final system stats:"
FINAL_STATS=$(curl -s "${ORCHESTRATOR_URL}/stats")
echo "$FINAL_STATS" | python3 -c "import sys, json; d=json.load(sys.stdin); print(f\"  Jobs: {d['jobs']['pending']} pending, {d['jobs']['running']} running, {d['jobs']['completed_total']} completed, {d['jobs']['failed_total']} failed\"); print(f\"  Volunteers: {d['volunteers']['idle']} idle, {d['volunteers']['busy']} busy\")"
echo ""

echo "=========================================="
echo -e "${GREEN}Test completed!${NC}"
echo "=========================================="

