BEGIN;

-- Reconciliation is a ledger adjustment, not ordinary spending or income.
INSERT INTO categories (name, type)
SELECT seed.name, seed.type
FROM (
    VALUES
        ('Penyesuaian Saldo', 'expense'),
        ('Penyesuaian Saldo', 'income')
) AS seed(name, type)
WHERE NOT EXISTS (
    SELECT 1
    FROM categories existing
    WHERE LOWER(existing.name) = LOWER(seed.name)
      AND existing.type = seed.type
);

ALTER TABLE transactions
    DROP CONSTRAINT IF EXISTS transactions_source_check;

ALTER TABLE transactions
    ADD CONSTRAINT transactions_source_check
    CHECK (source IN ('manual', 'notification', 'reprocess', 'import', 'reconcile'));

COMMIT;
