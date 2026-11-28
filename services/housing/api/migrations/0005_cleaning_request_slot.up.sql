-- 0005_cleaning_request_slot.up.sql
--
-- Adds the half-day slot chosen by the student to a cleaning request.
--
-- Why an enum and not a TIMESTAMPTZ:
--   "morning" / "afternoon" are discrete booking buckets, not instants. A
--   TIMESTAMPTZ cannot hold those values, and the concrete execution hours
--   remain on cleaning_shifts.start_time / end_time. The slot is therefore a
--   small enum column on cleaning_requests.
--
-- Interaction with the weekly quota:
--   The quota stays UNIQUE(dorm_unit_id, year, iso_week) WHERE status <>
--   'cancelled'. It is "one request per unit per week" regardless of the
--   chosen slot; the slot does not create a second weekly allowance.
--
-- Transaction handling: this file opens its own transaction because the
-- golang-migrate pgx driver executes the file as one statement batch. The
-- enum creation, column add and backfill must be atomic.

BEGIN;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cleaning_slot') THEN
        CREATE TYPE cleaning_slot AS ENUM ('morning', 'afternoon');
    END IF;
END
$$;

ALTER TABLE cleaning_requests
    ADD COLUMN IF NOT EXISTS slot cleaning_slot;

-- Existing rows predate the slot. Morning is the one-time safe default; new
-- rows have no column default and must choose explicitly (the service
-- validates the value and the enum rejects anything else).
UPDATE cleaning_requests
   SET slot = 'morning'
 WHERE slot IS NULL;

ALTER TABLE cleaning_requests
    ALTER COLUMN slot SET NOT NULL;

COMMENT ON TYPE cleaning_slot IS
    'Half-day booking bucket for a cleaning request: morning or afternoon.';

COMMENT ON COLUMN cleaning_requests.slot IS
    'Requested half-day slot. The actual execution hours are on cleaning_shifts.';

COMMIT;
