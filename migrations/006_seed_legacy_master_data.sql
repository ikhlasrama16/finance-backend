BEGIN;

-- Legacy owned financial accounts. Marketplace sources such as Shopee and
-- Tokopedia are intentionally not accounts and are not seeded here.
INSERT INTO accounts (name, provider, type, opening_balance, is_active)
SELECT seed.name, seed.provider, seed.type, 0, TRUE
FROM (
    VALUES
        ('SeaBank', 'SeaBank', 'bank'),
        ('ShopeePay', 'ShopeePay', 'ewallet'),
        ('Bank Jago', 'Bank Jago', 'bank'),
        ('Mandiri', 'Mandiri', 'bank'),
        ('BRI', 'BRI', 'bank'),
        ('Flip', 'Flip', 'ewallet')
) AS seed(name, provider, type)
WHERE NOT EXISTS (
    SELECT 1
    FROM accounts existing
    WHERE LOWER(existing.name) = LOWER(seed.name)
);

-- Legacy income and expense categories required by the transaction importer.
-- Transfers remain uncategorized in the normalized transaction model.
INSERT INTO categories (name, type)
SELECT seed.name, seed.type
FROM (
    VALUES
        ('Belum Dikategorikan', 'expense'),
        ('Makanan & Minuman', 'expense'),
        ('Laundry', 'expense'),
        ('Transportasi', 'expense'),
        ('Belanja', 'expense'),
        ('Hiburan', 'expense'),
        ('Tagihan', 'expense'),
        ('Kesehatan', 'expense'),
        ('Kos', 'expense'),
        ('Lainnya', 'expense'),
        ('Pemasukan', 'income')
) AS seed(name, type)
WHERE NOT EXISTS (
    SELECT 1
    FROM categories existing
    WHERE LOWER(existing.name) = LOWER(seed.name)
      AND existing.type = seed.type
);

COMMIT;
