BEGIN;

-- SeaBank is an explicitly supported owned account used by the notification parser.
INSERT INTO accounts (name, provider, type, opening_balance)
VALUES ('SeaBank', 'SeaBank', 'bank', 0)
ON CONFLICT (LOWER(name)) DO NOTHING;

COMMIT;
