BEGIN;

-- =========================================================
-- ACCOUNTS
-- Rekening / wallet yang kita monitor.
-- Contoh: SeaBank, ShopeePay, Jago, BRImo, Livin, Cash
-- =========================================================

CREATE TABLE accounts (
    id BIGSERIAL PRIMARY KEY,

    name VARCHAR(100) NOT NULL,
    provider VARCHAR(100),
    type VARCHAR(30) NOT NULL,

    opening_balance BIGINT NOT NULL DEFAULT 0,

    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT accounts_type_check
        CHECK (type IN ('bank', 'ewallet', 'cash', 'other'))
);

CREATE UNIQUE INDEX idx_accounts_name
    ON accounts (LOWER(name));


-- =========================================================
-- CATEGORIES
-- Kategori income / expense.
-- Transfer tidak membutuhkan kategori.
-- =========================================================

CREATE TABLE categories (
    id BIGSERIAL PRIMARY KEY,

    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT categories_type_check
        CHECK (type IN ('income', 'expense'))
);

CREATE UNIQUE INDEX idx_categories_name_type
    ON categories (LOWER(name), type);


-- =========================================================
-- RAW NOTIFICATIONS
-- Semua notification MacroDroid masuk sini terlebih dahulu.
-- =========================================================

CREATE TABLE raw_notifications (
    id BIGSERIAL PRIMARY KEY,

    source_app VARCHAR(100) NOT NULL,
    title TEXT,
    body TEXT NOT NULL,

    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    status VARCHAR(20) NOT NULL DEFAULT 'pending',

    parser_name VARCHAR(100),

    raw_payload JSONB,

    fingerprint VARCHAR(64),

    error_message TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT raw_notifications_status_check
        CHECK (
            status IN (
                'pending',
                'parsed',
                'ignored',
                'failed'
            )
        )
);

CREATE UNIQUE INDEX idx_raw_notifications_fingerprint
    ON raw_notifications (fingerprint)
    WHERE fingerprint IS NOT NULL;

CREATE INDEX idx_raw_notifications_status
    ON raw_notifications (status);

CREATE INDEX idx_raw_notifications_received_at
    ON raw_notifications (received_at DESC);


-- =========================================================
-- TRANSACTIONS
-- Source of truth seluruh pergerakan uang.
-- =========================================================

CREATE TABLE transactions (
    id BIGSERIAL PRIMARY KEY,

    type VARCHAR(20) NOT NULL,

    amount BIGINT NOT NULL,

    source_account_id BIGINT
        REFERENCES accounts(id),

    destination_account_id BIGINT
        REFERENCES accounts(id),

    category_id BIGINT
        REFERENCES categories(id),

    description TEXT,

    occurred_at TIMESTAMPTZ NOT NULL,

    raw_notification_id BIGINT
        REFERENCES raw_notifications(id),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT transactions_type_check
        CHECK (
            type IN (
                'income',
                'expense',
                'transfer'
            )
        ),

    CONSTRAINT transactions_amount_positive
        CHECK (amount > 0),

    CONSTRAINT transactions_accounts_different
        CHECK (
            source_account_id IS NULL
            OR destination_account_id IS NULL
            OR source_account_id <> destination_account_id
        )
);


CREATE INDEX idx_transactions_occurred_at
    ON transactions (occurred_at DESC);

CREATE INDEX idx_transactions_source_account
    ON transactions (source_account_id);

CREATE INDEX idx_transactions_destination_account
    ON transactions (destination_account_id);

CREATE INDEX idx_transactions_category
    ON transactions (category_id);

CREATE UNIQUE INDEX idx_transactions_raw_notification
    ON transactions (raw_notification_id)
    WHERE raw_notification_id IS NOT NULL;


COMMIT;