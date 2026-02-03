package models

import "time"

// JobStatus represents the state of a job
type JobStatus string

const (
	StatusPending JobStatus = "PENDING"
	StatusRunning JobStatus = "RUNNING"
	StatusDone    JobStatus = "DONE"
	StatusFailed  JobStatus = "FAILED"
)

// Job represents a job in the system
type Job struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	IdempotencyKey string     `json:"idempotency_key,omitempty"`
	Payload        string     `json:"payload"`
	Status         JobStatus  `json:"status"`
	MaxRetries     int        `json:"max_retries"`
	RetryCount     int        `json:"retry_count"`
	LeasedAt       *time.Time `json:"leased_at,omitempty"`
	LeaseExpiresAt *time.Time `json:"lease_expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateJobRequest represents a request to create a job
type CreateJobRequest struct {
	TenantID       string `json:"tenant_id"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	Payload        string `json:"payload"`
	MaxRetries     *int   `json:"max_retries,omitempty"`
}

// DeadLetterJob represents a job that has permanently failed
type DeadLetterJob struct {
	ID           string    `json:"id"`
	JobID        string    `json:"job_id"`
	TenantID     string    `json:"tenant_id"`
	Payload      string    `json:"payload"`
	FailureReason string   `json:"failure_reason"`
	FailedAt     time.Time `json:"failed_at"`
}
