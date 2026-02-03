package repository

import (
	"context"
	"job-queue/internal/models"
	"time"
)

// JobRepository defines the interface for job persistence
type JobRepository interface {
	CreateJob(ctx context.Context, job *models.Job) error
	GetJobByID(ctx context.Context, id string) (*models.Job, error)
	GetJobByTenantAndIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*models.Job, error)
	ListJobsByStatus(ctx context.Context, status models.JobStatus) ([]*models.Job, error)
	LeaseJob(ctx context.Context, leaseDuration time.Duration) (*models.Job, error)
	UpdateJobStatus(ctx context.Context, id string, status models.JobStatus) error
	IncrementRetryCount(ctx context.Context, id string) error
	GetRunningJobsCountByTenant(ctx context.Context, tenantID string) (int, error)
	MoveToDeadLetterQueue(ctx context.Context, job *models.Job, failureReason string) error
	ListDeadLetterJobs(ctx context.Context) ([]*models.DeadLetterJob, error)
}
