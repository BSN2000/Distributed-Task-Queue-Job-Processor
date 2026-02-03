# Test Cases - Job Queue System

## Overview
Comprehensive test cases covering all edge cases, error scenarios, and normal operations.

---

## 1. Job Creation Tests

### 1.1 Successful Job Creation

**Test Case ID:** TC-JOB-001  
**Description:** Create a job with valid data  
**Preconditions:** API server running  
**Steps:**
1. POST `/jobs` with valid tenant_id and payload
2. Verify response status 201
3. Verify response contains job ID
4. Verify job status is PENDING
5. Verify max_retries defaults to 3

**Expected Result:**
- Status: 201 Created
- Response contains: id, tenant_id, payload, status=PENDING, max_retries=3

---

### 1.2 Job Creation with Custom Max Retries

**Test Case ID:** TC-JOB-002  
**Description:** Create job with custom max_retries  
**Steps:**
1. POST `/jobs` with max_retries=5
2. Verify max_retries is set to 5

**Expected Result:** Job created with max_retries=5

---

### 1.3 Job Creation with Idempotency Key

**Test Case ID:** TC-JOB-003  
**Description:** Create job with idempotency_key  
**Steps:**
1. POST `/jobs` with idempotency_key="key-123"
2. Note the job ID
3. POST `/jobs` again with same tenant_id and idempotency_key but different payload
4. Verify same job ID is returned

**Expected Result:** Second request returns existing job (same ID)

---

### 1.4 Job Creation - Missing Tenant ID

**Test Case ID:** TC-JOB-004  
**Description:** Attempt to create job without tenant_id  
**Steps:**
1. POST `/jobs` without tenant_id field
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request
- Error message: "tenant_id is required"

---

### 1.5 Job Creation - Missing Payload

**Test Case ID:** TC-JOB-005  
**Description:** Attempt to create job without payload  
**Steps:**
1. POST `/jobs` without payload field
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request
- Error message: "payload is required"

---

### 1.6 Job Creation - Empty Tenant ID

**Test Case ID:** TC-JOB-006  
**Description:** Create job with empty tenant_id string  
**Steps:**
1. POST `/jobs` with tenant_id=""
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request
- Error message: "tenant_id is required"

---

### 1.7 Job Creation - Empty Payload

**Test Case ID:** TC-JOB-007  
**Description:** Create job with empty payload string  
**Steps:**
1. POST `/jobs` with payload=""
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request
- Error message: "payload is required"

---

### 1.8 Job Creation - Invalid JSON

**Test Case ID:** TC-JOB-008  
**Description:** Send malformed JSON  
**Steps:**
1. POST `/jobs` with invalid JSON syntax
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request
- Error message: "invalid request body"

---

### 1.9 Job Creation - Failing Job (payload="fail")

**Test Case ID:** TC-JOB-009  
**Description:** Create job that will fail (payload="fail")  
**Steps:**
1. POST `/jobs` with payload="fail"
2. Wait for worker to process
3. Verify job moves to FAILED status
4. Verify retry_count increments
5. After max_retries, verify job moves to DLQ

**Expected Result:**
- Job created successfully
- Worker processes and fails job
- Job retries up to max_retries
- Job moves to DLQ after max retries

---

### 1.10 Job Creation - Zero Max Retries

**Test Case ID:** TC-JOB-010  
**Description:** Create job with max_retries=0  
**Steps:**
1. POST `/jobs` with max_retries=0
2. Create failing job (payload="fail")
3. Verify job goes to DLQ immediately on first failure

**Expected Result:** Job moves to DLQ after first failure (no retries)

---

### 1.11 Job Creation - Negative Max Retries

**Test Case ID:** TC-JOB-011  
**Description:** Create job with negative max_retries  
**Steps:**
1. POST `/jobs` with max_retries=-1
2. Verify job is created (system may accept but behavior undefined)

**Expected Result:** Job created (system handles gracefully)

---

### 1.12 Job Creation - Very Large Max Retries

**Test Case ID:** TC-JOB-012  
**Description:** Create job with very large max_retries  
**Steps:**
1. POST `/jobs` with max_retries=1000
2. Verify job is created

**Expected Result:** Job created successfully

---

### 1.13 Job Creation - Very Long Payload

**Test Case ID:** TC-JOB-013  
**Description:** Create job with very long payload string  
**Steps:**
1. POST `/jobs` with payload containing 10KB+ of data
2. Verify job is created

**Expected Result:** Job created successfully

---

### 1.14 Job Creation - SQL Injection Attempt

**Test Case ID:** TC-JOB-014  
**Description:** Attempt SQL injection in tenant_id or payload  
**Steps:**
1. POST `/jobs` with tenant_id="tenant-1'; DROP TABLE jobs; --"
2. Verify system handles safely (parameterized queries)

**Expected Result:** Job created safely, no SQL injection

---

### 1.15 Job Creation - XSS Attempt

**Test Case ID:** TC-JOB-015  
**Description:** Attempt XSS in payload  
**Steps:**
1. POST `/jobs` with payload="<script>alert('xss')</script>"
2. Verify payload is stored as-is (not executed)

**Expected Result:** Payload stored safely, no XSS execution

---

## 2. Rate Limiting Tests

### 2.1 Submission Rate Limit

**Test Case ID:** TC-RATE-001  
**Description:** Test submission rate limit (10 per minute)  
**Preconditions:** Rate limiter configured for 10 jobs/minute  
**Steps:**
1. Submit 10 jobs rapidly to same tenant
2. All should succeed
3. Submit 11th job
4. Verify rate limit error

**Expected Result:**
- First 10 jobs: Status 201
- 11th job: Status 429 (Too Many Requests)

---

### 2.2 Concurrent Running Limit

**Test Case ID:** TC-RATE-002  
**Description:** Test concurrent running limit (5 per tenant)  
**Preconditions:** 5 jobs already RUNNING for tenant  
**Steps:**
1. Attempt to create new job for same tenant
2. Verify rate limit error

**Expected Result:**
- Status: 429 (Too Many Requests)
- Error: "rate limit exceeded"

---

### 2.3 Rate Limit Window Expiry

**Test Case ID:** TC-RATE-003  
**Description:** Test rate limit window resets after 1 minute  
**Steps:**
1. Exhaust submission rate limit (submit 10 jobs)
2. Wait 61 seconds
3. Submit another job
4. Verify job is accepted

**Expected Result:** Job accepted after window expiry

---

### 2.4 Rate Limit Per Tenant

**Test Case ID:** TC-RATE-004  
**Description:** Verify rate limits are per-tenant  
**Steps:**
1. Exhaust rate limit for tenant-1
2. Submit job for tenant-2
3. Verify tenant-2 job succeeds

**Expected Result:** Different tenants have independent rate limits

---

## 3. Job Retrieval Tests

### 3.1 Get Job by ID - Success

**Test Case ID:** TC-GET-001  
**Description:** Retrieve existing job by ID  
**Steps:**
1. Create a job and note the ID
2. GET `/jobs/{id}`
3. Verify job details

**Expected Result:**
- Status: 200 OK
- Response contains complete job details

---

### 3.2 Get Job by ID - Not Found

**Test Case ID:** TC-GET-002  
**Description:** Retrieve non-existent job  
**Steps:**
1. GET `/jobs/non-existent-id`
2. Verify error response

**Expected Result:**
- Status: 404 Not Found
- Error message: "job not found"

---

### 3.3 Get Job by ID - Invalid Format

**Test Case ID:** TC-GET-003  
**Description:** Retrieve job with invalid ID format  
**Steps:**
1. GET `/jobs/invalid-format-123`
2. Verify error response

**Expected Result:**
- Status: 404 Not Found or 400 Bad Request

---

### 3.4 Get Job by ID - Empty ID

**Test Case ID:** TC-GET-004  
**Description:** Retrieve job with empty ID  
**Steps:**
1. GET `/jobs/`
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request
- Error message: "job id is required"

---

## 4. Job Listing Tests

### 4.1 List Jobs by Status - PENDING

**Test Case ID:** TC-LIST-001  
**Description:** List all PENDING jobs  
**Steps:**
1. Create multiple jobs
2. GET `/jobs?status=PENDING`
3. Verify all returned jobs have status PENDING

**Expected Result:**
- Status: 200 OK
- Response is array of jobs with status PENDING

---

### 4.2 List Jobs by Status - RUNNING

**Test Case ID:** TC-LIST-002  
**Description:** List all RUNNING jobs  
**Steps:**
1. Create jobs and let worker lease some
2. GET `/jobs?status=RUNNING`
3. Verify all returned jobs have status RUNNING

**Expected Result:**
- Status: 200 OK
- Response contains only RUNNING jobs

---

### 4.3 List Jobs by Status - DONE

**Test Case ID:** TC-LIST-003  
**Description:** List all DONE jobs  
**Steps:**
1. Create jobs and let worker complete them
2. GET `/jobs?status=DONE`
3. Verify all returned jobs have status DONE

**Expected Result:**
- Status: 200 OK
- Response contains only DONE jobs

---

### 4.4 List Jobs by Status - FAILED

**Test Case ID:** TC-LIST-004  
**Description:** List all FAILED jobs  
**Steps:**
1. Create failing jobs
2. GET `/jobs?status=FAILED`
3. Verify all returned jobs have status FAILED

**Expected Result:**
- Status: 200 OK
- Response contains only FAILED jobs

---

### 4.5 List Jobs - Missing Status Parameter

**Test Case ID:** TC-LIST-005  
**Description:** List jobs without status parameter  
**Steps:**
1. GET `/jobs` (no query parameter)
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request
- Error message: "status query parameter is required"

---

### 4.6 List Jobs - Invalid Status

**Test Case ID:** TC-LIST-006  
**Description:** List jobs with invalid status value  
**Steps:**
1. GET `/jobs?status=INVALID`
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request
- Error message: "invalid status"

---

### 4.7 List Jobs - Empty Status

**Test Case ID:** TC-LIST-007  
**Description:** List jobs with empty status parameter  
**Steps:**
1. GET `/jobs?status=`
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request

---

### 4.8 List Jobs - SQL Injection in Status

**Test Case ID:** TC-LIST-008  
**Description:** Attempt SQL injection via status parameter  
**Steps:**
1. GET `/jobs?status=PENDING' OR '1'='1`
2. Verify system handles safely

**Expected Result:**
- Status: 400 Bad Request (invalid status validation)

---

### 4.9 List Jobs - Special Characters

**Test Case ID:** TC-LIST-009  
**Description:** List jobs with special characters in status  
**Steps:**
1. GET `/jobs?status=PENDING<script>`
2. Verify error response

**Expected Result:**
- Status: 400 Bad Request

---

## 5. Dead Letter Queue Tests

### 5.1 Get Dead Letter Queue - Empty

**Test Case ID:** TC-DLQ-001  
**Description:** Retrieve DLQ when empty  
**Steps:**
1. GET `/dlq`
2. Verify empty array response

**Expected Result:**
- Status: 200 OK
- Response: [] (empty array)

---

### 5.2 Get Dead Letter Queue - With Jobs

**Test Case ID:** TC-DLQ-002  
**Description:** Retrieve DLQ with failed jobs  
**Steps:**
1. Create failing jobs (payload="fail")
2. Wait for max retries
3. GET `/dlq`
4. Verify DLQ contains failed jobs

**Expected Result:**
- Status: 200 OK
- Response contains DLQ jobs with failure_reason

---

### 5.3 DLQ Job Details

**Test Case ID:** TC-DLQ-003  
**Description:** Verify DLQ job contains all required fields  
**Steps:**
1. Get DLQ jobs
2. Verify each job has: id, job_id, tenant_id, payload, failure_reason, failed_at

**Expected Result:**
- All required fields present
- failure_reason explains why job failed

---

## 6. Worker Processing Tests

### 6.1 Worker Leases Job

**Test Case ID:** TC-WORKER-001  
**Description:** Worker successfully leases a PENDING job  
**Steps:**
1. Create a PENDING job
2. Start worker
3. Verify job status changes to RUNNING
4. Verify lease_expires_at is set

**Expected Result:**
- Job status: RUNNING
- leased_at and lease_expires_at are set
- Lease duration: 30 seconds

---

### 6.2 Worker Processes Successful Job

**Test Case ID:** TC-WORKER-002  
**Description:** Worker processes job successfully  
**Steps:**
1. Create job with payload != "fail"
2. Wait for worker to process
3. Verify job status changes to DONE

**Expected Result:**
- Job status: DONE
- Processing time: ~2 seconds
- Metrics: completed_jobs incremented

---

### 6.3 Worker Processes Failing Job

**Test Case ID:** TC-WORKER-003  
**Description:** Worker processes failing job  
**Steps:**
1. Create job with payload="fail"
2. Wait for worker to process
3. Verify job retries
4. Verify retry_count increments

**Expected Result:**
- Job fails processing
- retry_count increments
- Job status resets to PENDING for retry

---

### 6.4 Worker Max Retries Exceeded

**Test Case ID:** TC-WORKER-004  
**Description:** Worker moves job to DLQ after max retries  
**Steps:**
1. Create failing job with max_retries=2
2. Wait for worker to retry 2 times
3. Verify job moves to DLQ
4. Verify job removed from jobs table

**Expected Result:**
- Job appears in DLQ
- Job removed from jobs table
- failure_reason explains failure
- Metrics: failed_jobs incremented

---

### 6.5 Worker Lease Expiry

**Test Case ID:** TC-WORKER-005  
**Description:** Worker can lease expired RUNNING jobs  
**Steps:**
1. Create job and let worker lease it
2. Simulate worker crash (stop worker)
3. Wait 31 seconds (lease expires)
4. Start worker again
5. Verify job is re-leased

**Expected Result:**
- Expired RUNNING job can be leased again
- Prevents job from being stuck

---

### 6.6 Worker Concurrent Processing

**Test Case ID:** TC-WORKER-006  
**Description:** Multiple workers don't double-process jobs  
**Steps:**
1. Start 2 workers
2. Create multiple jobs
3. Verify each job processed only once

**Expected Result:**
- No duplicate processing
- All jobs processed exactly once

---

## 7. Metrics Tests

### 7.1 Get Metrics

**Test Case ID:** TC-METRICS-001  
**Description:** Retrieve system metrics  
**Steps:**
1. GET `/metrics`
2. Verify all metric fields present

**Expected Result:**
- Status: 200 OK
- Response contains: total_jobs, completed_jobs, failed_jobs, retried_jobs

---

### 7.2 Metrics Increment on Job Creation

**Test Case ID:** TC-METRICS-002  
**Description:** Verify total_jobs increments on job creation  
**Steps:**
1. Get initial metrics
2. Create a job
3. Get metrics again
4. Verify total_jobs incremented

**Expected Result:**
- total_jobs increases by 1

---

### 7.3 Metrics Increment on Completion

**Test Case ID:** TC-METRICS-003  
**Description:** Verify completed_jobs increments  
**Steps:**
1. Create a job
2. Wait for completion
3. Get metrics
4. Verify completed_jobs incremented

**Expected Result:**
- completed_jobs increases by 1

---

### 7.4 Metrics Increment on Failure

**Test Case ID:** TC-METRICS-004  
**Description:** Verify failed_jobs increments  
**Steps:**
1. Create failing job
2. Wait for max retries and DLQ
3. Get metrics
4. Verify failed_jobs incremented

**Expected Result:**
- failed_jobs increases by 1

---

### 7.5 Metrics Increment on Retry

**Test Case ID:** TC-METRICS-005  
**Description:** Verify retried_jobs increments  
**Steps:**
1. Create failing job
2. Wait for retry
3. Get metrics
4. Verify retried_jobs incremented

**Expected Result:**
- retried_jobs increases by 1

---

## 8. HTTP Method Tests

### 8.1 POST /jobs - Wrong Method

**Test Case ID:** TC-HTTP-001  
**Description:** Use wrong HTTP method for POST endpoint  
**Steps:**
1. GET `/jobs` (should be POST)
2. Verify error response

**Expected Result:**
- Status: 405 Method Not Allowed or 400 Bad Request

---

### 8.2 GET /jobs/{id} - Wrong Method

**Test Case ID:** TC-HTTP-002  
**Description:** Use wrong HTTP method for GET endpoint  
**Steps:**
1. POST `/jobs/{id}` (should be GET)
2. Verify error response

**Expected Result:**
- Status: 405 Method Not Allowed

---

### 8.3 OPTIONS Request (CORS)

**Test Case ID:** TC-HTTP-003  
**Description:** Verify CORS preflight works  
**Steps:**
1. OPTIONS `/jobs` with Origin header
2. Verify CORS headers in response

**Expected Result:**
- Status: 200 OK
- Headers: Access-Control-Allow-Origin, Access-Control-Allow-Methods

---

## 9. Edge Cases & Security Tests

### 9.1 Concurrent Job Creation

**Test Case ID:** TC-EDGE-001  
**Description:** Create multiple jobs concurrently  
**Steps:**
1. Submit 50 jobs simultaneously
2. Verify all jobs created
3. Verify no data corruption

**Expected Result:**
- All jobs created successfully
- No duplicate IDs
- Database integrity maintained

---

### 9.2 Idempotency Race Condition

**Test Case ID:** TC-EDGE-002  
**Description:** Test idempotency with concurrent requests  
**Steps:**
1. Submit 2 jobs with same idempotency_key simultaneously
2. Verify only one job created
3. Verify both requests return same job

**Expected Result:**
- Only one job in database
- Both requests return same job ID

---

### 9.3 Large Number of Jobs

**Test Case ID:** TC-EDGE-003  
**Description:** Create and process large number of jobs  
**Steps:**
1. Create 1000 jobs
2. Verify all jobs processed
3. Verify system performance

**Expected Result:**
- All jobs processed
- System remains responsive
- No memory leaks

---

### 9.4 Database Connection Loss

**Test Case ID:** TC-EDGE-004  
**Description:** Handle database connection loss gracefully  
**Steps:**
1. Create jobs
2. Simulate database disconnect
3. Attempt to create job
4. Verify error handling

**Expected Result:**
- Error returned gracefully
- System doesn't crash
- Connection recovery works

---

### 9.5 Worker Crash Recovery

**Test Case ID:** TC-EDGE-005  
**Description:** Verify jobs recover after worker crash  
**Steps:**
1. Create jobs
2. Let worker lease some jobs
3. Kill worker process
4. Wait for lease expiry
5. Restart worker
6. Verify jobs are re-leased and processed

**Expected Result:**
- Expired leases are re-leased
- No jobs lost
- All jobs eventually processed

---

### 9.6 Empty Database

**Test Case ID:** TC-EDGE-006  
**Description:** Test system with empty database  
**Steps:**
1. Start with fresh database
2. List jobs by status
3. Verify empty arrays returned

**Expected Result:**
- No errors
- Empty arrays returned
- System works normally

---

## 10. Integration Tests

### 10.1 Complete Job Lifecycle

**Test Case ID:** TC-INT-001  
**Description:** Test complete job lifecycle  
**Steps:**
1. Create job
2. Verify PENDING status
3. Wait for worker to lease
4. Verify RUNNING status
5. Wait for completion
6. Verify DONE status
7. Verify metrics updated

**Expected Result:**
- Job transitions: PENDING → RUNNING → DONE
- All timestamps set correctly
- Metrics updated

---

### 10.2 Failed Job Lifecycle

**Test Case ID:** TC-INT-002  
**Description:** Test failed job lifecycle  
**Steps:**
1. Create failing job (payload="fail")
2. Verify retry cycle
3. Verify DLQ after max retries
4. Verify metrics updated

**Expected Result:**
- Job retries up to max_retries
- Job moves to DLQ
- Metrics: retried_jobs and failed_jobs updated

---

### 10.3 Multiple Tenants

**Test Case ID:** TC-INT-003  
**Description:** Test system with multiple tenants  
**Steps:**
1. Create jobs for tenant-1, tenant-2, tenant-3
2. Verify rate limits are per-tenant
3. Verify jobs processed correctly
4. Verify tenant isolation

**Expected Result:**
- Rate limits independent per tenant
- Jobs processed correctly
- No cross-tenant data leakage

---

## Test Execution

### Run Unit Tests
```bash
go test ./internal/service/... -v
go test ./internal/metrics/... -v
```

### Run Integration Tests
```bash
# Start services
docker-compose up -d

# Run test script
./test-api.sh

# Check results
docker-compose logs worker
```

### Import Postman Collection
1. Open Postman
2. Import → `postman_collection.json`
3. Update `base_url` variable if needed
4. Run collection

---

## Test Coverage Summary

- **Job Creation:** 15 test cases
- **Rate Limiting:** 4 test cases
- **Job Retrieval:** 4 test cases
- **Job Listing:** 9 test cases
- **Dead Letter Queue:** 3 test cases
- **Worker Processing:** 6 test cases
- **Metrics:** 5 test cases
- **HTTP Methods:** 3 test cases
- **Edge Cases:** 6 test cases
- **Integration:** 3 test cases

**Total: 61 test cases**
