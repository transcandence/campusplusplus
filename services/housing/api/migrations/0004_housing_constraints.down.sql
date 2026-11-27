-- 0004_housing_constraints.down.sql
-- Rollback for the housing structural constraints.
--
-- Restores the 0003 shape: no living-unit guard on stays, and a plain
-- UNIQUE(dorm_unit_id, year, iso_week) on cleaning_requests.
--
-- The UNIQUE restore is conditional. Under 0004 a unit may hold a cancelled
-- request and a live one in the same ISO week, which the plain constraint
-- cannot represent. Re-adding it blindly would fail on exactly the rows 0004
-- exists to allow; deleting them would be data loss during a rollback.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Drop the living-unit guard
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_stays_living_unit_only ON stays;
DROP FUNCTION IF EXISTS housing_stays_living_unit_only();

-- ---------------------------------------------------------------------------
-- 2. Restore the blanket weekly quota, if the data still permits it
-- ---------------------------------------------------------------------------

DROP INDEX IF EXISTS uq_unit_weekly_cleaning_quota;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM cleaning_requests
        GROUP BY dorm_unit_id, year, iso_week
        HAVING count(*) > 1
    ) THEN
        RAISE NOTICE 'skipping uq_unit_weekly_cleaning_quota restore: duplicate weekly requests exist';
    ELSE
        ALTER TABLE cleaning_requests
            ADD CONSTRAINT uq_unit_weekly_cleaning_quota
            UNIQUE (dorm_unit_id, year, iso_week);
    END IF;
END $$;

COMMIT;
