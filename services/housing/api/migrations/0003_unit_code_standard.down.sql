-- 0003_unit_code_standard.down.sql
-- Rollback for the unit code standard.
--
-- Restores the 0001 shape: dorm_units.code becomes a free-text column with a
-- global UNIQUE constraint, and all derivation is removed.
--
-- The UNIQUE(code) restore is conditional. This migration removed a constraint
-- that was wrong (two campuses may legitimately share 12A2-1), so data written
-- while 0003 was applied can violate it. Re-adding it blindly would make the
-- rollback fail on exactly the rows this migration exists to allow, and
-- deleting them would be data loss. Warn instead, and leave the schema usable.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Remove the reparenting triggers
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_residences_resync_unit_codes ON residences;
DROP TRIGGER IF EXISTS trg_buildings_resync_unit_codes  ON buildings;
DROP TRIGGER IF EXISTS trg_floors_resync_unit_codes     ON floors;

DROP FUNCTION IF EXISTS housing_trg_resync_residence();
DROP FUNCTION IF EXISTS housing_trg_resync_building();
DROP FUNCTION IF EXISTS housing_trg_resync_floor();
DROP FUNCTION IF EXISTS housing_resync_unit_codes(TEXT, INT);

-- ---------------------------------------------------------------------------
-- 2. Remove the code derivation
-- ---------------------------------------------------------------------------

DROP TRIGGER IF EXISTS trg_dorm_units_set_code ON dorm_units;

DROP FUNCTION IF EXISTS housing_dorm_units_set_code();

-- ---------------------------------------------------------------------------
-- 3. Remove the format guarantees
-- ---------------------------------------------------------------------------

ALTER TABLE dorm_units DROP CONSTRAINT IF EXISTS ck_dorm_unit_number_format;
ALTER TABLE dorm_units DROP CONSTRAINT IF EXISTS ck_dorm_unit_code_format;

-- ---------------------------------------------------------------------------
-- 4. Restore the global unique, if the data still permits it
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM dorm_units GROUP BY code HAVING count(*) > 1) THEN
        RAISE NOTICE 'skipping UNIQUE(code) restore: cross-campus duplicate codes exist';
    ELSE
        ALTER TABLE dorm_units ADD CONSTRAINT dorm_units_code_key UNIQUE (code);
    END IF;
END $$;

-- Function is dropped last: housing_dorm_units_set_code() depends on nothing,
-- but housing_unit_code() is referenced by the (now removed) trigger function.
DROP FUNCTION IF EXISTS housing_unit_code(INT, TEXT);

COMMIT;
