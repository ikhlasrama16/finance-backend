BEGIN;

-- Cached, generated explanations. Financial figures remain derived from
-- transactions at request time; this table only avoids repeating LLM calls.
-- A future multi-user migration can add user_id and extend this unique key.
CREATE TABLE ai_reports (
    id BIGSERIAL PRIMARY KEY,
    period_type VARCHAR(20) NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    summary_hash VARCHAR(64) NOT NULL,
    content TEXT NOT NULL,
    model TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'complete',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ai_reports_period_type_check
        CHECK (period_type IN ('daily', 'weekly', 'monthly', 'custom')),
    CONSTRAINT ai_reports_period_check
        CHECK (period_end >= period_start),
    CONSTRAINT ai_reports_status_check
        CHECK (status = 'complete')
);

CREATE UNIQUE INDEX idx_ai_reports_period_summary_model
    ON ai_reports (period_type, period_start, period_end, summary_hash, model);

COMMIT;
