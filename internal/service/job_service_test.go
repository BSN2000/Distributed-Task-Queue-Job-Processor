package service

import (
	"context"
	"database/sql"
	"errors"
	"job-queue/internal/metrics"
	"job-queue/internal/models"
	"testing"
	"time"
)

// mockRepository is a mock implementation of JobRepository
type mockRepository struct {
	jobs              map[string]*models.Job
	dlqJobs           []*models.DeadLetterJob
	runningCount      map[string]int
	createJobError    error
	getJobError       error
	listJobsError     error
	idempotencyJob    *models.Job
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		jobs:         make(map[string]*models.Job),
		dlqJobs:      make([]*models.DeadLetterJob, 0),
		runningCount: make(map[string]int),
	}
}

func (m *mockRepository) CreateJob(ctx context.Context, job *models.Job) error {
	if m.createJobError != nil {
		return m.createJobError
	}
	m.jobs[job.ID] = job
	return nil
}

func (m *mockRepository) GetJobByID(ctx context.Context, id string) (*models.Job, error) {
	if m.getJobError != nil {
		return nil, m.getJobError
	}
	job, exists := m.jobs[id]
	if !exists {
		return nil, sql.ErrNoRows
	}
	return job, nil
}

func (m *mockRepository) GetJobByTenantAndIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*models.Job, error) {
	if m.idempotencyJob != nil {
		return m.idempotencyJob, nil
	}
	return nil, nil
}

func (m *mockRepository) ListJobsByStatus(ctx context.Context, status models.JobStatus) ([]*models.Job, error) {
	if m.listJobsError != nil {
		return nil, m.listJobsError
	}
	var result []*models.Job
	for _, job := range m.jobs {
		if job.Status == status {
			result = append(result, job)
		}
	}
	return result, nil
}

func (m *mockRepository) LeaseJob(ctx context.Context, leaseDuration time.Duration) (*models.Job, error) {
	return nil, nil
}

func (m *mockRepository) UpdateJobStatus(ctx context.Context, id string, status models.JobStatus) error {
	if job, exists := m.jobs[id]; exists {
		job.Status = status
		return nil
	}
	return errors.New("job not found")
}

func (m *mockRepository) IncrementRetryCount(ctx context.Context, id string) error {
	if job, exists := m.jobs[id]; exists {
		job.RetryCount++
		return nil
	}
	return errors.New("job not found")
}

func (m *mockRepository) GetRunningJobsCountByTenant(ctx context.Context, tenantID string) (int, error) {
	return m.runningCount[tenantID], nil
}

func (m *mockRepository) MoveToDeadLetterQueue(ctx context.Context, job *models.Job, failureReason string) error {
	dlqJob := &models.DeadLetterJob{
		ID:           "dlq_" + job.ID,
		JobID:        job.ID,
		TenantID:     job.TenantID,
		Payload:      job.Payload,
		FailureReason: failureReason,
		FailedAt:     time.Now(),
	}
	m.dlqJobs = append(m.dlqJobs, dlqJob)
	delete(m.jobs, job.ID)
	return nil
}

func (m *mockRepository) ListDeadLetterJobs(ctx context.Context) ([]*models.DeadLetterJob, error) {
	return m.dlqJobs, nil
}

func TestJobService_CreateJob_Success(t *testing.T) {
	repo := newMockRepository()
	rateLimiter := NewRateLimiter(5, 10)
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	req := &models.CreateJobRequest{
		TenantID: "tenant-1",
		Payload:  "test payload",
	}

	job, err := service.CreateJob(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if job == nil {
		t.Fatal("expected job to be created")
	}

	if job.TenantID != req.TenantID {
		t.Errorf("expected tenant_id %s, got %s", req.TenantID, job.TenantID)
	}

	if job.Payload != req.Payload {
		t.Errorf("expected payload %s, got %s", req.Payload, job.Payload)
	}

	if job.Status != models.StatusPending {
		t.Errorf("expected status PENDING, got %s", job.Status)
	}

	if job.MaxRetries != 3 {
		t.Errorf("expected max_retries 3, got %d", job.MaxRetries)
	}
}

func TestJobService_CreateJob_WithMaxRetries(t *testing.T) {
	repo := newMockRepository()
	rateLimiter := NewRateLimiter(5, 10)
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	maxRetries := 5
	req := &models.CreateJobRequest{
		TenantID:   "tenant-1",
		Payload:    "test payload",
		MaxRetries: &maxRetries,
	}

	job, err := service.CreateJob(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if job.MaxRetries != 5 {
		t.Errorf("expected max_retries 5, got %d", job.MaxRetries)
	}
}

func TestJobService_CreateJob_RateLimitSubmission(t *testing.T) {
	repo := newMockRepository()
	rateLimiter := NewRateLimiter(5, 2) // Max 2 submissions per minute
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	req := &models.CreateJobRequest{
		TenantID: "tenant-1",
		Payload:  "test payload",
	}

	// Create first job - should succeed
	_, err := service.CreateJob(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error for first job, got %v", err)
	}

	// Create second job - should succeed
	_, err = service.CreateJob(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error for second job, got %v", err)
	}

	// Create third job - should fail rate limit
	_, err = service.CreateJob(context.Background(), req)
	if err != ErrRateLimitExceeded {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestJobService_CreateJob_RateLimitConcurrent(t *testing.T) {
	repo := newMockRepository()
	repo.runningCount["tenant-1"] = 5 // Already at limit
	rateLimiter := NewRateLimiter(5, 10)
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	req := &models.CreateJobRequest{
		TenantID: "tenant-1",
		Payload:  "test payload",
	}

	_, err := service.CreateJob(context.Background(), req)
	if err != ErrRateLimitExceeded {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestJobService_CreateJob_Idempotency(t *testing.T) {
	repo := newMockRepository()
	existingJob := &models.Job{
		ID:             "existing-id",
		TenantID:       "tenant-1",
		IdempotencyKey: "key-123",
		Payload:        "original payload",
		Status:         models.StatusPending,
	}
	repo.idempotencyJob = existingJob

	rateLimiter := NewRateLimiter(5, 10)
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	req := &models.CreateJobRequest{
		TenantID:       "tenant-1",
		Payload:        "different payload",
		IdempotencyKey: "key-123",
	}

	job, err := service.CreateJob(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if job.ID != existingJob.ID {
		t.Errorf("expected existing job ID %s, got %s", existingJob.ID, job.ID)
	}

	if job.Payload != existingJob.Payload {
		t.Errorf("expected original payload, got %s", job.Payload)
	}
}

func TestJobService_GetJob_Success(t *testing.T) {
	repo := newMockRepository()
	expectedJob := &models.Job{
		ID:       "job-1",
		TenantID: "tenant-1",
		Payload:  "test",
		Status:   models.StatusPending,
	}
	repo.jobs["job-1"] = expectedJob

	rateLimiter := NewRateLimiter(5, 10)
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	job, err := service.GetJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if job.ID != expectedJob.ID {
		t.Errorf("expected job ID %s, got %s", expectedJob.ID, job.ID)
	}
}

func TestJobService_GetJob_NotFound(t *testing.T) {
	repo := newMockRepository()
	rateLimiter := NewRateLimiter(5, 10)
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	_, err := service.GetJob(context.Background(), "non-existent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestJobService_ListJobsByStatus(t *testing.T) {
	repo := newMockRepository()
	repo.jobs["job-1"] = &models.Job{ID: "job-1", Status: models.StatusPending}
	repo.jobs["job-2"] = &models.Job{ID: "job-2", Status: models.StatusPending}
	repo.jobs["job-3"] = &models.Job{ID: "job-3", Status: models.StatusDone}

	rateLimiter := NewRateLimiter(5, 10)
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	jobs, err := service.ListJobsByStatus(context.Background(), models.StatusPending)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(jobs) != 2 {
		t.Errorf("expected 2 pending jobs, got %d", len(jobs))
	}
}

func TestJobService_ListDeadLetterJobs(t *testing.T) {
	repo := newMockRepository()
	repo.dlqJobs = []*models.DeadLetterJob{
		{ID: "dlq-1", JobID: "job-1", TenantID: "tenant-1", FailureReason: "test"},
		{ID: "dlq-2", JobID: "job-2", TenantID: "tenant-2", FailureReason: "test"},
	}

	rateLimiter := NewRateLimiter(5, 10)
	metrics := metrics.NewMetrics()
	service := NewJobService(repo, rateLimiter, metrics)

	dlqJobs, err := service.ListDeadLetterJobs(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(dlqJobs) != 2 {
		t.Errorf("expected 2 DLQ jobs, got %d", len(dlqJobs))
	}
}
