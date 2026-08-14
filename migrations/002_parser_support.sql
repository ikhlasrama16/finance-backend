BEGIN;

-- =========================================================
-- ENRICH TRANSACTIONS
-- Metadata yang sebelumnya ada di sheet transactions GAS
-- =========================================================

ALTER TABLE transactions
    ADD COLUMN merchant TEXT,
    ADD COLUMN parse_status VARCHAR(30) NOT NULL DEFAULT 'MANUAL',
    ADD COLUMN confidence NUMERIC(4,3),
    ADD COLUMN source VARCHAR(30) NOT NULL DEFAULT 'manual';

ALTER TABLE transactions
    ADD CONSTRAINT transactions_parse_status_check
    CHECK (
        parse_status IN (
            'AUTO',
            'RULE',
            'MANUAL',
            'NEEDS_REVIEW',
            'REPROCESS'
        )
    );

ALTER TABLE transactions
    ADD CONSTRAINT transactions_source_check
    CHECK (
        source IN (
            'manual',
            'notification',
            'reprocess',
            'import'
        )
    );

ALTER TABLE transactions
    ADD CONSTRAINT transactions_confidence_check
    CHECK (
        confidence IS NULL
        OR (
            confidence >= 0
            AND confidence <= 1
        )
    );


-- =========================================================
-- PARSER RULES
-- Pengganti sheet "rules"
-- =========================================================

CREATE TABLE parser_rules (
    id BIGSERIAL PRIMARY KEY,

    source_app VARCHAR(100),
    keyword TEXT,

    action VARCHAR(20) NOT NULL,

    transaction_type VARCHAR(20),

    category_id BIGINT
        REFERENCES categories(id),

    merchant TEXT,

    confidence NUMERIC(4,3) NOT NULL DEFAULT 1,

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    priority INTEGER NOT NULL DEFAULT 100,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT parser_rules_action_check
        CHECK (
            action IN (
                'PARSE',
                'IGNORE'
            )
        ),

    CONSTRAINT parser_rules_transaction_type_check
        CHECK (
            transaction_type IS NULL
            OR transaction_type IN (
                'income',
                'expense',
                'transfer'
            )
        ),

    CONSTRAINT parser_rules_confidence_check
        CHECK (
            confidence >= 0
            AND confidence <= 1
        )
);

CREATE INDEX idx_parser_rules_active_priority
    ON parser_rules (
        is_active,
        priority
    );


-- =========================================================
-- CATEGORY RULES
-- Pengganti sheet "category_rules"
-- =========================================================

CREATE TABLE category_rules (
    id BIGSERIAL PRIMARY KEY,

    keyword TEXT NOT NULL,

    category_id BIGINT NOT NULL
        REFERENCES categories(id),

    confidence NUMERIC(4,3) NOT NULL DEFAULT 0.95,

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    priority INTEGER NOT NULL DEFAULT 100,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT category_rules_confidence_check
        CHECK (
            confidence >= 0
            AND confidence <= 1
        )
);

CREATE INDEX idx_category_rules_active_priority
    ON category_rules (
        is_active,
        priority
    );


-- =========================================================
-- LINK RAW NOTIFICATION → TRANSACTION
-- Agar satu raw notification bisa ditelusuri langsung
-- =========================================================

ALTER TABLE raw_notifications
    ADD COLUMN transaction_id BIGINT
        REFERENCES transactions(id);

CREATE UNIQUE INDEX idx_raw_notifications_transaction_id
    ON raw_notifications (transaction_id)
    WHERE transaction_id IS NOT NULL;


COMMIT;