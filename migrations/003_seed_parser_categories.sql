BEGIN;

INSERT INTO categories (name, type)
VALUES
    ('Pemasukan', 'income'),
    ('Belum Dikategorikan', 'expense')
ON CONFLICT DO NOTHING;

COMMIT;
