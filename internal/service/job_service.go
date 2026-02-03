package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"job-queue/internal/metrics"
	"job-queue/internal/models"
	"job-queue/internal/repository"
	"log"

	"github.com/google/uuid"
)

var (
	ErrJobNotFound       = errors.New("job not found")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrDuplicateJob      = errors.New("job with same idempotency key already exists")
)

// JobService handles job business logic
type JobService struct {
	repo        repository.JobRepository
	rateLimiter *RateLimiter
	metrics     *metrics.Metrics
}

// NewJobService creates a new job service
func NewJobService(repo repository.JobRepository, rateLimiter *RateLimiter, metrics *metrics.Metrics) *JobService {
	return &JobService{
		repo:        repo,
		rateLimiter: rateLimiter,
		metrics:     metrics,
	}
}

// CreateJob creates a new job
func (s *JobService) CreateJob(ctx context.Context, req *models.CreateJobRequest) (*models.Job, error) {
	// Check submission rate limit
	if err := s.rateLimiter.CheckSubmissionRate(ctx, req.TenantID); err != nil {
		return nil, err
	}

	// Check idempotency
	if req.IdempotencyKey != "" {
		existing, err := s.repo.GetJobByTenantAndIdempotencyKey(ctx, req.TenantID, req.IdempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
		if existing != nil {
			log.Printf("job_id=%s: duplicate job detected with idempotency_key=%s", existing.ID, req.IdempotencyKey)
			return existing, nil
		}
	}

	// Check concurrent running limit
	runningCount, err := s.repo.GetRunningJobsCountByTenant(ctx, req.TenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get running jobs count: %w", err)
	}

	if err := s.rateLimiter.CheckConcurrentLimit(ctx, req.TenantID, runningCount); err != nil {
		return nil, err
	}

	// Create job
	maxRetries := 3
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}

	job := &models.Job{
		ID:             uuid.New().String(),
		TenantID:       req.TenantID,
		IdempotencyKey: req.IdempotencyKey,
		Payload:        req.Payload,
		Status:         models.StatusPending,
		MaxRetries:     maxRetries,
		RetryCount:     0,
	}

	if err := s.repo.CreateJob(ctx, job); err != nil {
		// Handle duplicate idempotency key (race condition)
		if dupErr, ok := err.(*repository.ErrDuplicateIdempotencyKey); ok {
			// Fetch the existing job
			existing, fetchErr := s.repo.GetJobByTenantAndIdempotencyKey(ctx, dupErr.TenantID, dupErr.IdempotencyKey)
			if fetchErr != nil {
				return nil, fmt.Errorf("failed to fetch existing job: %w", fetchErr)
			}
			if existing != nil {
				log.Printf("job_id=%s: duplicate job detected with idempotency_key=%s (race condition)", existing.ID, dupErr.IdempotencyKey)
				return existing, nil
			}
		}
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	s.metrics.IncrementTotalJobs()
	log.Printf("job_id=%s: job submitted, tenant_id=%s, payload=%s", job.ID, job.TenantID, job.Payload)

	return job, nil
}

// GetJob retrieves a job by ID
func (s *JobService) GetJob(ctx context.Context, id string) (*models.Job, error) {
	job, err := s.repo.GetJobByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return job, nil
}

// ListJobsByStatus retrieves jobs by status
func (s *JobService) ListJobsByStatus(ctx context.Context, status models.JobStatus) ([]*models.Job, error) {
	jobs, err := s.repo.ListJobsByStatus(ctx, status)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	return jobs, nil
}

// ListDeadLetterJobs retrieves all dead letter jobs
func (s *JobService) ListDeadLetterJobs(ctx context.Context) ([]*models.DeadLetterJob, error) {
	dlqJobs, err := s.repo.ListDeadLetterJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list dead letter jobs: %w", err)
	}
	return dlqJobs, nil
}
