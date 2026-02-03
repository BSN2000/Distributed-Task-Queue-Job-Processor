-- Jobs table
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

-- Dead letter queue table
CREATE TABLE IF NOT EXISTS dead_letter_jobs (
    id TEXT PRIMARY KEY,
    job_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    payload TEXT NOT NULL,
    failure_reason TEXT NOT NULL,
    failed_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dlq_tenant_id ON dead_letter_jobs(tenant_id);
