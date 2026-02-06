#!/bin/bash

echo "=== Testing Metrics Endpoint ==="
echo ""
echo "1. Getting /metrics endpoint:"
curl -s 'http://localhost:8081/metrics' | python3 -m json.tool 2>/dev/null || curl -s 'http://localhost:8081/metrics'
echo ""
echo ""

echo "2. Getting DLQ count:"
DLQ_COUNT=$(curl -s 'http://localhost:8081/dlq' | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null || echo "0")
echo "DLQ jobs: $DLQ_COUNT"
echo ""

echo "3. Getting job counts by status:"
echo "PENDING: $(curl -s 'http://localhost:8081/jobs?status=PENDING' | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null || echo '0')"
echo "RUNNING: $(curl -s 'http://localhost:8081/jobs?status=RUNNING' | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null || echo '0')"
echo "DONE: $(curl -s 'http://localhost:8081/jobs?status=DONE' | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null || echo '0')"
echo "FAILED: $(curl -s 'http://localhost:8081/jobs?status=FAILED' | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 0)" 2>/dev/null || echo '0')"
echo ""

echo "=== Expected Calculation ==="
echo "Total should be: (PENDING + RUNNING + DONE + FAILED) + DLQ"
echo ""
