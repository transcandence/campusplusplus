-- 0005_cleaning_request_slot.down.sql

BEGIN;

ALTER TABLE cleaning_requests
    DROP COLUMN IF EXISTS slot;

DROP TYPE IF EXISTS cleaning_slot;

COMMIT;
