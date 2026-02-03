#!/bin/bash

API_URL="http://localhost:8081"

echo "=========================================="
echo "Job Queue API Test Script"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print section headers
section() {
    echo ""
    echo -e "${BLUE}=== $1 ===${NC}"
    echo ""
}

# Function to create a job
create_job() {
    local tenant=$1
    local payload=$2
    local max_retries=${3:-3}
    local idempotency_key=${4:-""}
    
    if [ -z "$idempotency_key" ]; then
        local response=$(curl -s -X POST $API_URL/jobs \
        -H "Content-Type: application/json" \
        -d "{\"tenant_id\": \"$tenant\", \"payload\": \"$payload\", \"max_retries\": $max_retries}" 2>/dev/null)
    
    if command -v jq &> /dev/null; then
        echo "$response" | jq -r '.id // empty' 2>/dev/null
    else
        # Extract ID using grep and cut
        local job_id=$(echo "$response" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
        if [ -n "$job_id" ] && [ "$job_id" != "null" ]; then
            echo "$job_id"
        else
            echo "created"
        fi
    fi
    else
        local response=$(curl -s -X POST $API_URL/jobs \
            -H "Content-Type: application/json" \
            -d "{\"tenant_id\": \"$tenant\", \"payload\": \"$payload\", \"max_retries\": $max_retries, \"idempotency_key\": \"$idempotency_key\"}" 2>/dev/null)
        
        if command -v jq &> /dev/null; then
            echo "$response" | jq -r '.id // empty' 2>/dev/null
        else
            local job_id=$(echo "$response" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
            if [ -n "$job_id" ] && [ "$job_id" != "null" ]; then
                echo "$job_id"
            else
                echo "created"
            fi
        fi
    fi
}

section "1. Create Successful Jobs"
echo "Creating 10 successful jobs..."
for i in {1..10}; do
    job_id=$(create_job "tenant-1" "success-job-$i")
    echo -e "${GREEN}✓${NC} Job $i created: $job_id"
    sleep 0.1
done

section "2. Create Failing Jobs"
echo "Creating 5 failing jobs (will retry then go to DLQ)..."
for i in {1..5}; do
    job_id=$(create_job "tenant-2" "fail" 2)
    echo -e "${YELLOW}⚠${NC} Failing job $i created: $job_id"
    sleep 0.1
done

section "3. Create Mixed Jobs"
echo "Creating 15 mixed jobs (some fail, some succeed)..."
for i in {1..15}; do
    if [ $((i % 4)) -eq 0 ]; then
        payload="fail"
        tenant="tenant-3"
        max_retries=2
        icon="${YELLOW}⚠${NC}"
    else
        payload="mixed-job-$i"
        tenant="tenant-$((i % 3 + 1))"
        max_retries=3
        icon="${GREEN}✓${NC}"
    fi
    
    job_id=$(create_job "$tenant" "$payload" $max_retries)
    echo -e "$icon Job $i created: $job_id"
    sleep 0.1
done

section "4. Create Job with Idempotency Key"
echo "Creating job with idempotency key..."
job_id1=$(create_job "tenant-1" "idempotent-job" 3 "key-123")
echo -e "${GREEN}✓${NC} First job: $job_id1"

echo "Creating duplicate with same idempotency key..."
job_id2=$(create_job "tenant-1" "different-payload" 3 "key-123")
echo -e "${BLUE}ℹ${NC} Duplicate job (should return same ID): $job_id2"

if [ "$job_id1" == "$job_id2" ] && [ "$job_id1" != "error" ]; then
    echo -e "${GREEN}✓${NC} Idempotency works correctly!"
else
    echo -e "${YELLOW}⚠${NC} Idempotency check: IDs differ or error occurred"
fi

section "5. Check Job Statuses"
echo "PENDING jobs:"
if command -v jq &> /dev/null; then
    pending_count=$(curl -s "$API_URL/jobs?status=PENDING" 2>/dev/null | jq 'length' 2>/dev/null || echo "0")
else
    pending_count=$(curl -s "$API_URL/jobs?status=PENDING" 2>/dev/null | grep -o '"id"' | wc -l | tr -d ' ')
    [ -z "$pending_count" ] && pending_count="0"
fi
echo -e "  ${YELLOW}Count: $pending_count${NC}"

echo ""
echo "RUNNING jobs:"
if command -v jq &> /dev/null; then
    running_count=$(curl -s "$API_URL/jobs?status=RUNNING" 2>/dev/null | jq 'length' 2>/dev/null || echo "0")
    done_count=$(curl -s "$API_URL/jobs?status=DONE" 2>/dev/null | jq 'length' 2>/dev/null || echo "0")
    failed_count=$(curl -s "$API_URL/jobs?status=FAILED" 2>/dev/null | jq 'length' 2>/dev/null || echo "0")
else
    running_count=$(curl -s "$API_URL/jobs?status=RUNNING" 2>/dev/null | grep -o '"id"' | wc -l | tr -d ' ')
    done_count=$(curl -s "$API_URL/jobs?status=DONE" 2>/dev/null | grep -o '"id"' | wc -l | tr -d ' ')
    failed_count=$(curl -s "$API_URL/jobs?status=FAILED" 2>/dev/null | grep -o '"id"' | wc -l | tr -d ' ')
    [ -z "$running_count" ] && running_count="0"
    [ -z "$done_count" ] && done_count="0"
    [ -z "$failed_count" ] && failed_count="0"
fi
echo -e "  ${BLUE}Count: $running_count${NC}"

echo ""
echo "DONE jobs:"
echo -e "  ${GREEN}Count: $done_count${NC}"

echo ""
echo "FAILED jobs:"
echo -e "  ${YELLOW}Count: $failed_count${NC}"

section "6. Check Dead Letter Queue"
if command -v jq &> /dev/null; then
    dlq_count=$(curl -s "$API_URL/dlq" 2>/dev/null | jq 'length' 2>/dev/null || echo "0")
else
    dlq_count=$(curl -s "$API_URL/dlq" 2>/dev/null | grep -o '"id"' | wc -l | tr -d ' ')
    [ -z "$dlq_count" ] && dlq_count="0"
fi
echo "DLQ jobs: $dlq_count"
if [ "$dlq_count" -gt 0 ]; then
    echo ""
    echo "DLQ entries:"
    if command -v jq &> /dev/null; then
        curl -s "$API_URL/dlq" 2>/dev/null | jq '.[] | {job_id, tenant_id, failure_reason, failed_at}' 2>/dev/null || echo "Error parsing DLQ"
    else
        curl -s "$API_URL/dlq" 2>/dev/null | head -20
    fi
fi

section "7. View Metrics"
echo "System metrics:"
if command -v jq &> /dev/null; then
    curl -s "$API_URL/metrics" 2>/dev/null | jq 2>/dev/null || curl -s "$API_URL/metrics" 2>/dev/null
else
    curl -s "$API_URL/metrics" 2>/dev/null
fi

section "8. Sample Job Details"
echo "Getting first PENDING job details (if any)..."
if command -v jq &> /dev/null; then
    first_job=$(curl -s "$API_URL/jobs?status=PENDING" 2>/dev/null | jq -r '.[0].id // empty' 2>/dev/null)
else
    first_job=$(curl -s "$API_URL/jobs?status=PENDING" 2>/dev/null | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
fi
if [ -n "$first_job" ] && [ "$first_job" != "null" ]; then
    echo "Job ID: $first_job"
    if command -v jq &> /dev/null; then
        curl -s "$API_URL/jobs/$first_job" 2>/dev/null | jq 2>/dev/null || curl -s "$API_URL/jobs/$first_job" 2>/dev/null
    else
        curl -s "$API_URL/jobs/$first_job" 2>/dev/null
    fi
else
    echo "No PENDING jobs found"
fi

section "9. Rate Limit Test"
echo "Testing rate limiting (submitting 12 jobs quickly to same tenant)..."
rate_limit_hits=0
for i in {1..12}; do
    http_code=$(curl -s -X POST $API_URL/jobs \
        -H "Content-Type: application/json" \
        -d "{\"tenant_id\": \"tenant-rate-test\", \"payload\": \"rate-test-$i\"}" \
        -w "%{http_code}" -o /dev/null 2>/dev/null)
    
    if [ "$http_code" == "429" ]; then
        rate_limit_hits=$((rate_limit_hits + 1))
        echo -e "${YELLOW}⚠${NC} Job $i: Rate limited (HTTP 429)"
    else
        echo -e "${GREEN}✓${NC} Job $i: Created (HTTP $http_code)"
    fi
    sleep 0.1
done
echo ""
echo "Rate limit hits: $rate_limit_hits"

echo ""
echo "=========================================="
echo -e "${GREEN}Test Complete!${NC}"
echo "=========================================="
echo ""
echo "Summary:"
echo "  - Created ~30 jobs (mix of success and failure)"
echo "  - Check dashboard at: http://localhost:3001"
echo "  - Wait 30-60 seconds for worker to process"
echo "  - Failed jobs will appear in DLQ after retries"
echo ""
echo "Quick status check:"
echo "  curl -s '$API_URL/jobs?status=PENDING' | jq length"
echo "  curl -s '$API_URL/jobs?status=DONE' | jq length"
echo "  curl -s '$API_URL/dlq' | jq length"
