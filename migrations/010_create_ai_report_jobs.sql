BEGIN;

-- Durable queue for v2 AI reports. A job stores the exact statistical snapshot
-- used in its prompt, so an analysis can never be attached to newer data.
CREATE TABLE ai_report_jobs (
    id UUID PRIMARY KEY,
    input_hash VARCHAR(64) NOT NULL UNIQUE,
    snapshot_hash VARCHAR(64) NOT NULL,
    range_start DATE NOT NULL,
    range_end DATE NOT NULL,
    comparison_mode VARCHAR(40) NOT NULL,
    comparison_start DATE,
    comparison_end DATE,
    snapshot JSONB NOT NULL,
    prompt TEXT NOT NULL,
    model TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_expires_at TIMESTAMPTZ,
    content TEXT,
    error_code VARCHAR(80),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ai_report_jobs_range_check CHECK (range_end >= range_start),
    CONSTRAINT ai_report_jobs_comparison_range_check CHECK (
        (comparison_start IS NULL AND comparison_end IS NULL)
        OR (comparison_start IS NOT NULL AND comparison_end IS NOT NULL AND comparison_end >= comparison_start)
    ),
    CONSTRAINT ai_report_jobs_status_check CHECK (status IN ('queued', 'running', 'complete', 'failed')),
    CONSTRAINT ai_report_jobs_attempts_check CHECK (attempts >= 0 AND max_attempts > 0)
);

CREATE INDEX idx_ai_report_jobs_claim
    ON ai_report_jobs (status, next_attempt_at, created_at)
    WHERE status = 'queued';

CREATE INDEX idx_ai_report_jobs_expired_lease
    ON ai_report_jobs (lease_expires_at)
    WHERE status = 'running';

COMMIT;
