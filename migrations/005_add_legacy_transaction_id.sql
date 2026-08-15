BEGIN;

ALTER TABLE transactions
    ADD COLUMN legacy_id TEXT;

CREATE UNIQUE INDEX idx_transactions_legacy_id
    ON transactions (legacy_id)
    WHERE legacy_id IS NOT NULL;

COMMIT;
