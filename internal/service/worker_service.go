package service

import (
	"context"
	"fmt"
	"job-queue/internal/metrics"
	"job-queue/internal/models"
	"job-queue/internal/repository"
	"log"
	"time"
)

// WorkerService handles worker operations
type WorkerService struct {
	repo    repository.JobRepository
	metrics *metrics.Metrics
}

// NewWorkerService creates a new worker service
func NewWorkerService(repo repository.JobRepository, metrics *metrics.Metrics) *WorkerService {
	return &WorkerService{
		repo:    repo,
		metrics: metrics,
	}
}

// ProcessJobs continuously processes jobs
func (s *WorkerService) ProcessJobs(ctx context.Context, leaseDuration time.Duration) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			job, err := s.repo.LeaseJob(ctx, leaseDuration)
			if err != nil {
				log.Printf("error leasing job: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if job == nil {
				// No jobs available
				time.Sleep(1 * time.Second)
				continue
			}

			log.Printf("job_id=%s: job leased, tenant_id=%s, payload=%s", job.ID, job.TenantID, job.Payload)

			// Process the job
			s.processJob(ctx, job)
		}
	}
}

// processJob processes a single job
func (s *WorkerService) processJob(ctx context.Context, job *models.Job) {
	// Simulate processing
	time.Sleep(2 * time.Second)

	// Check if job should fail
	if job.Payload == "fail" {
		s.handleJobFailure(ctx, job, "payload is 'fail'")
		return
	}

	// Job succeeded
	if err := s.repo.UpdateJobStatus(ctx, job.ID, models.StatusDone); err != nil {
		log.Printf("job_id=%s: error updating job status to DONE: %v", job.ID, err)
		return
	}

	s.metrics.IncrementCompletedJobs()
	log.Printf("job_id=%s: job completed successfully", job.ID)
}

// handleJobFailure handles a failed job
func (s *WorkerService) handleJobFailure(ctx context.Context, job *models.Job, failureReason string) {
	// Check if we should retry
	if job.RetryCount < job.MaxRetries {
		// Reset to PENDING for retry
		if err := s.repo.IncrementRetryCount(ctx, job.ID); err != nil {
			log.Printf("job_id=%s: error incrementing retry count: %v", job.ID, err)
			return
		}

		if err := s.repo.UpdateJobStatus(ctx, job.ID, models.StatusPending); err != nil {
			log.Printf("job_id=%s: error resetting job status to PENDING: %v", job.ID, err)
			return
		}

		s.metrics.IncrementRetriedJobs()
		log.Printf("job_id=%s: job failed, retrying (attempt %d/%d), reason: %s", job.ID, job.RetryCount+1, job.MaxRetries, failureReason)
		return
	}

	// Max retries exceeded, move to DLQ
	if err := s.repo.MoveToDeadLetterQueue(ctx, job, fmt.Sprintf("max retries exceeded: %s", failureReason)); err != nil {
		log.Printf("job_id=%s: error moving job to DLQ: %v", job.ID, err)
		return
	}

	s.metrics.IncrementFailedJobs()
	log.Printf("job_id=%s: job moved to dead letter queue, reason: %s", job.ID, failureReason)
}
