BEGIN;

ALTER TABLE raw_notifications
    DROP CONSTRAINT IF EXISTS raw_notifications_status_check;

ALTER TABLE raw_notifications
    ADD CONSTRAINT raw_notifications_status_check
    CHECK (status IN ('pending', 'parsed', 'ignored', 'failed', 'detached'));

COMMIT;
