#!/bin/bash
set -e

echo "Testing CopilotKit sidecar..."

# Test 1: Health check
echo "1. Health check..."
HEALTH=$(curl -s http://localhost:3001/health)
if [[ $HEALTH == *"ok"* ]]; then
  echo "✓ Health check passed"
else
  echo "✗ Health check failed"
  exit 1
fi

# Test 2: GraphQL endpoint exists
echo "2. GraphQL endpoint check..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:3001/copilot)
if [[ $STATUS == "200" ]] || [[ $STATUS == "400" ]]; then
  echo "✓ GraphQL endpoint responding"
else
  echo "✗ GraphQL endpoint not responding (HTTP $STATUS)"
  exit 1
fi

# Test 3: Backend connectivity
echo "3. Backend connectivity..."
cd ../
BACKEND_INFO=$(curl -s http://localhost:8080/api/copilot/info)
if [[ $BACKEND_INFO == *"models"* ]]; then
  echo "✓ Backend reachable"
else
  echo "✗ Backend not reachable"
  exit 1
fi

echo ""
echo "All tests passed! ✓"
echo "Frontend should now work at http://localhost:5173"
