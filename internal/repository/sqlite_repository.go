package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"job-queue/internal/models"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteRepository implements JobRepository using SQLite
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &SQLiteRepository{db: db}
	if err := repo.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return repo, nil
}

// Close closes the database connection
func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

// initSchema initializes the database schema
func (r *SQLiteRepository) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		tenant_id TEXT NOT NULL,
		idempotency_key TEXT,
		payload TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'PENDING',
		max_retries INTEGER NOT NULL DEFAULT 3,
		retry_count INTEGER NOT NULL DEFAULT 0,
		leased_at INTEGER,
		lease_expires_at INTEGER,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		UNIQUE(tenant_id, idempotency_key)
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_tenant_id ON jobs(tenant_id);
	CREATE INDEX IF NOT EXISTS idx_jobs_lease_expires ON jobs(lease_expires_at);

	CREATE TABLE IF NOT EXISTS dead_letter_jobs (
		id TEXT PRIMARY KEY,
		job_id TEXT NOT NULL,
		tenant_id TEXT NOT NULL,
		payload TEXT NOT NULL,
		failure_reason TEXT NOT NULL,
		failed_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_dlq_tenant_id ON dead_letter_jobs(tenant_id);
	`

	_, err := r.db.Exec(schema)
	return err
}

// CreateJob creates a new job
func (r *SQLiteRepository) CreateJob(ctx context.Context, job *models.Job) error {
	query := `
		INSERT INTO jobs (id, tenant_id, idempotency_key, payload, status, max_retries, retry_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now

	// Convert empty string to NULL for idempotency_key
	// SQLite allows multiple NULLs in a UNIQUE constraint, but not multiple empty strings
	var idempotencyKey interface{}
	if job.IdempotencyKey == "" {
		idempotencyKey = nil
	} else {
		idempotencyKey = job.IdempotencyKey
	}

	_, err := r.db.ExecContext(ctx, query,
		job.ID,
		job.TenantID,
		idempotencyKey,
		job.Payload,
		job.Status,
		job.MaxRetries,
		job.RetryCount,
		job.CreatedAt.Unix(),
		job.UpdatedAt.Unix(),
	)

	if err != nil {
		// Check if it's a unique constraint violation (idempotency key conflict)
		if errStr := err.Error(); errStr != "" {
			// SQLite returns "UNIQUE constraint failed" for unique violations
			if strings.Contains(errStr, "UNIQUE constraint failed") {
				// Only return duplicate error if idempotency_key was provided (not empty)
				if job.IdempotencyKey != "" {
					return &ErrDuplicateIdempotencyKey{TenantID: job.TenantID, IdempotencyKey: job.IdempotencyKey}
				}
				// If idempotency_key was empty, this shouldn't happen (NULLs are allowed multiple times)
				// But handle it gracefully
				return fmt.Errorf("failed to create job: unique constraint violation (unexpected)")
			}
		}
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// ErrDuplicateIdempotencyKey is returned when a job with the same idempotency key already exists
type ErrDuplicateIdempotencyKey struct {
	TenantID       string
	IdempotencyKey string
}

func (e *ErrDuplicateIdempotencyKey) Error() string {
	return fmt.Sprintf("job with idempotency_key %s already exists for tenant %s", e.IdempotencyKey, e.TenantID)
}

// GetJobByID retrieves a job by ID
func (r *SQLiteRepository) GetJobByID(ctx context.Context, id string) (*models.Job, error) {
	query := `
		SELECT id, tenant_id, idempotency_key, payload, status, max_retries, retry_count,
		       leased_at, lease_expires_at, created_at, updated_at
		FROM jobs
		WHERE id = ?
	`

	var job models.Job
	var idempotencyKeyVal sql.NullString
	var leasedAt, leaseExpiresAt sql.NullInt64
	var createdAt, updatedAt int64

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.TenantID,
		&idempotencyKeyVal,
		&job.Payload,
		&job.Status,
		&job.MaxRetries,
		&job.RetryCount,
		&leasedAt,
		&leaseExpiresAt,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Handle NULL idempotency_key
	if idempotencyKeyVal.Valid {
		job.IdempotencyKey = idempotencyKeyVal.String
	} else {
		job.IdempotencyKey = ""
	}

	job.CreatedAt = time.Unix(createdAt, 0)
	job.UpdatedAt = time.Unix(updatedAt, 0)

	if leasedAt.Valid {
		t := time.Unix(leasedAt.Int64, 0)
		job.LeasedAt = &t
	}

	if leaseExpiresAt.Valid {
		t := time.Unix(leaseExpiresAt.Int64, 0)
		job.LeaseExpiresAt = &t
	}

	return &job, nil
}

// GetJobByTenantAndIdempotencyKey retrieves a job by tenant ID and idempotency key
func (r *SQLiteRepository) GetJobByTenantAndIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*models.Job, error) {
	// Handle NULL idempotency_key (empty string means no idempotency key)
	var query string
	var args []interface{}

	if idempotencyKey == "" {
		query = `
			SELECT id, tenant_id, idempotency_key, payload, status, max_retries, retry_count,
			       leased_at, lease_expires_at, created_at, updated_at
			FROM jobs
			WHERE tenant_id = ? AND idempotency_key IS NULL
		`
		args = []interface{}{tenantID}
	} else {
		query = `
			SELECT id, tenant_id, idempotency_key, payload, status, max_retries, retry_count,
			       leased_at, lease_expires_at, created_at, updated_at
			FROM jobs
			WHERE tenant_id = ? AND idempotency_key = ?
		`
		args = []interface{}{tenantID, idempotencyKey}
	}

	var job models.Job
	var idempotencyKeyVal sql.NullString
	var leasedAt, leaseExpiresAt sql.NullInt64
	var createdAt, updatedAt int64

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&job.ID,
		&job.TenantID,
		&idempotencyKeyVal,
		&job.Payload,
		&job.Status,
		&job.MaxRetries,
		&job.RetryCount,
		&leasedAt,
		&leaseExpiresAt,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Handle NULL idempotency_key
	if idempotencyKeyVal.Valid {
		job.IdempotencyKey = idempotencyKeyVal.String
	} else {
		job.IdempotencyKey = ""
	}

	job.CreatedAt = time.Unix(createdAt, 0)
	job.UpdatedAt = time.Unix(updatedAt, 0)

	if leasedAt.Valid {
		t := time.Unix(leasedAt.Int64, 0)
		job.LeasedAt = &t
	}

	if leaseExpiresAt.Valid {
		t := time.Unix(leaseExpiresAt.Int64, 0)
		job.LeaseExpiresAt = &t
	}

	return &job, nil
}

// ListJobsByStatus retrieves all jobs with a specific status
func (r *SQLiteRepository) ListJobsByStatus(ctx context.Context, status models.JobStatus) ([]*models.Job, error) {
	query := `
		SELECT id, tenant_id, idempotency_key, payload, status, max_retries, retry_count,
		       leased_at, lease_expires_at, created_at, updated_at
		FROM jobs
		WHERE status = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*models.Job
	for rows.Next() {
		var job models.Job
		var idempotencyKeyVal sql.NullString
		var leasedAt, leaseExpiresAt sql.NullInt64
		var createdAt, updatedAt int64

		err := rows.Scan(
			&job.ID,
			&job.TenantID,
			&idempotencyKeyVal,
			&job.Payload,
			&job.Status,
			&job.MaxRetries,
			&job.RetryCount,
			&leasedAt,
			&leaseExpiresAt,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		// Handle NULL idempotency_key
		if idempotencyKeyVal.Valid {
			job.IdempotencyKey = idempotencyKeyVal.String
		} else {
			job.IdempotencyKey = ""
		}

		job.CreatedAt = time.Unix(createdAt, 0)
		job.UpdatedAt = time.Unix(updatedAt, 0)

		if leasedAt.Valid {
			t := time.Unix(leasedAt.Int64, 0)
			job.LeasedAt = &t
		}

		if leaseExpiresAt.Valid {
			t := time.Unix(leaseExpiresAt.Int64, 0)
			job.LeaseExpiresAt = &t
		}

		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate jobs: %w", err)
	}

	return jobs, nil
}

// LeaseJob leases a job for processing using a transaction
func (r *SQLiteRepository) LeaseJob(ctx context.Context, leaseDuration time.Duration) (*models.Job, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()
	nowUnix := now.Unix()
	expiresAt := now.Add(leaseDuration)
	expiresAtUnix := expiresAt.Unix()

	// Find a job that can be leased:
	// - PENDING jobs
	// - RUNNING jobs whose lease has expired
	query := `
		SELECT id, tenant_id, idempotency_key, payload, status, max_retries, retry_count,
		       leased_at, lease_expires_at, created_at, updated_at
		FROM jobs
		WHERE (status = 'PENDING' OR (status = 'RUNNING' AND lease_expires_at < ?))
		ORDER BY created_at ASC
		LIMIT 1
	`

	var job models.Job
	var idempotencyKeyVal sql.NullString
	var leasedAt, leaseExpiresAt sql.NullInt64
	var createdAt, updatedAt int64

	err = tx.QueryRowContext(ctx, query, nowUnix).Scan(
		&job.ID,
		&job.TenantID,
		&idempotencyKeyVal,
		&job.Payload,
		&job.Status,
		&job.MaxRetries,
		&job.RetryCount,
		&leasedAt,
		&leaseExpiresAt,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find leasable job: %w", err)
	}

	// Handle NULL idempotency_key
	if idempotencyKeyVal.Valid {
		job.IdempotencyKey = idempotencyKeyVal.String
	} else {
		job.IdempotencyKey = ""
	}

	job.CreatedAt = time.Unix(createdAt, 0)
	job.UpdatedAt = time.Unix(updatedAt, 0)

	if leasedAt.Valid {
		t := time.Unix(leasedAt.Int64, 0)
		job.LeasedAt = &t
	}

	// Update the job to RUNNING with new lease
	updateQuery := `
		UPDATE jobs
		SET status = 'RUNNING',
		    leased_at = ?,
		    lease_expires_at = ?,
		    updated_at = ?
		WHERE id = ?
	`

	_, err = tx.ExecContext(ctx, updateQuery, nowUnix, expiresAtUnix, nowUnix, job.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update job lease: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	job.Status = models.StatusRunning
	job.LeasedAt = &now
	job.LeaseExpiresAt = &expiresAt
	job.UpdatedAt = now

	return &job, nil
}

// UpdateJobStatus updates the status of a job
func (r *SQLiteRepository) UpdateJobStatus(ctx context.Context, id string, status models.JobStatus) error {
	query := `
		UPDATE jobs
		SET status = ?, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, status, now.Unix(), id)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}

// IncrementRetryCount increments the retry count of a job
func (r *SQLiteRepository) IncrementRetryCount(ctx context.Context, id string) error {
	query := `
		UPDATE jobs
		SET retry_count = retry_count + 1, updated_at = ?
		WHERE id = ?
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, now.Unix(), id)
	if err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}

	return nil
}

// GetRunningJobsCountByTenant returns the count of running jobs for a tenant
func (r *SQLiteRepository) GetRunningJobsCountByTenant(ctx context.Context, tenantID string) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM jobs
		WHERE tenant_id = ? AND status = 'RUNNING'
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count running jobs: %w", err)
	}

	return count, nil
}

// MoveToDeadLetterQueue moves a job to the dead letter queue
func (r *SQLiteRepository) MoveToDeadLetterQueue(ctx context.Context, job *models.Job, failureReason string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert into dead letter queue
	insertQuery := `
		INSERT INTO dead_letter_jobs (id, job_id, tenant_id, payload, failure_reason, failed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	dlqID := fmt.Sprintf("dlq_%s_%d", job.ID, time.Now().Unix())
	_, err = tx.ExecContext(ctx, insertQuery,
		dlqID,
		job.ID,
		job.TenantID,
		job.Payload,
		failureReason,
		time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert into dead letter queue: %w", err)
	}

	// Delete from jobs table
	_, err = tx.ExecContext(ctx, "DELETE FROM jobs WHERE id = ?", job.ID)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListDeadLetterJobs retrieves all dead letter jobs
func (r *SQLiteRepository) ListDeadLetterJobs(ctx context.Context) ([]*models.DeadLetterJob, error) {
	query := `
		SELECT id, job_id, tenant_id, payload, failure_reason, failed_at
		FROM dead_letter_jobs
		ORDER BY failed_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query dead letter jobs: %w", err)
	}
	defer rows.Close()

	var dlqJobs []*models.DeadLetterJob
	for rows.Next() {
		var dlqJob models.DeadLetterJob
		var failedAt int64

		err := rows.Scan(
			&dlqJob.ID,
			&dlqJob.JobID,
			&dlqJob.TenantID,
			&dlqJob.Payload,
			&dlqJob.FailureReason,
			&failedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dead letter job: %w", err)
		}

		dlqJob.FailedAt = time.Unix(failedAt, 0)
		dlqJobs = append(dlqJobs, &dlqJob)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate dead letter jobs: %w", err)
	}

	return dlqJobs, nil
}

// GetTotalJobsCount returns the total count of all jobs (including DLQ)
func (r *SQLiteRepository) GetTotalJobsCount(ctx context.Context) (int, error) {
	// Count jobs in jobs table
	var jobsCount int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs").Scan(&jobsCount)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	// Count jobs in DLQ
	var dlqCount int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dead_letter_jobs").Scan(&dlqCount)
	if err != nil {
		return 0, fmt.Errorf("failed to count DLQ jobs: %w", err)
	}

	return jobsCount + dlqCount, nil
}

// GetCompletedJobsCount returns the count of completed (DONE) jobs
func (r *SQLiteRepository) GetCompletedJobsCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE status = 'DONE'").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count completed jobs: %w", err)
	}
	return count, nil
}

// GetFailedJobsCount returns the count of failed jobs (FAILED status + DLQ)
func (r *SQLiteRepository) GetFailedJobsCount(ctx context.Context) (int, error) {
	// Count FAILED jobs
	var failedCount int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs WHERE status = 'FAILED'").Scan(&failedCount)
	if err != nil {
		return 0, fmt.Errorf("failed to count failed jobs: %w", err)
	}

	// Count DLQ jobs
	var dlqCount int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dead_letter_jobs").Scan(&dlqCount)
	if err != nil {
		return 0, fmt.Errorf("failed to count DLQ jobs: %w", err)
	}

	return failedCount + dlqCount, nil
}

// GetDeadLetterQueueCount returns the count of jobs in DLQ
func (r *SQLiteRepository) GetDeadLetterQueueCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM dead_letter_jobs").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count DLQ jobs: %w", err)
	}
	return count, nil
}
