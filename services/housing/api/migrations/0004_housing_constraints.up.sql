-- 0004_housing_constraints.up.sql
-- Housing Service: two structural invariants that 0001 left to the service layer.
--
-- 1. Stays may only target living units (rooms and apartments). A corridor,
--    kitchen or laundry has capacity 0 and cannot legally be occupied, but
--    nothing in 0001 stopped a stay from pointing at one.
--    Ref: docs/domain-model.md, Occupancy invariants, "Living Units Only".
--
-- 2. A cancelled cleaning request must release its weekly slot. 0001 put a
--    plain UNIQUE(dorm_unit_id, year, iso_week) on cleaning_requests, so once a
--    request was created the unit could never book that ISO week again, even
--    after the request was cancelled. The quota is "max 1 *live* request per
--    unit per week", which is a partial unique index.
--
-- Both are structural, not time-dependent, so they belong in the database. The
-- time-dependent rules (active stays <= capacity, gender policy, the 00:00
-- booking cut-off, next-day scheduling) stay in the service layer.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Stays are for living units only
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION housing_stays_living_unit_only()
RETURNS TRIGGER AS $$
DECLARE
    v_unit_type dorm_unit_type;
BEGIN
    SELECT unit_type INTO v_unit_type FROM dorm_units WHERE id = NEW.dorm_unit_id;

    -- A missing unit is left to the foreign key constraint.
    IF v_unit_type IN ('corridor', 'kitchen', 'laundry') THEN
        RAISE EXCEPTION 'stays cannot target common area % (unit_type=%)',
            NEW.dorm_unit_id, v_unit_type
            USING ERRCODE = 'check_violation';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_stays_living_unit_only ON stays;
CREATE TRIGGER trg_stays_living_unit_only
BEFORE INSERT OR UPDATE OF dorm_unit_id ON stays
FOR EACH ROW EXECUTE FUNCTION housing_stays_living_unit_only();

-- ---------------------------------------------------------------------------
-- 2. Only live cleaning requests count against the weekly quota
-- ---------------------------------------------------------------------------

ALTER TABLE cleaning_requests
    DROP CONSTRAINT IF EXISTS uq_unit_weekly_cleaning_quota;

DROP INDEX IF EXISTS uq_unit_weekly_cleaning_quota;

CREATE UNIQUE INDEX IF NOT EXISTS uq_unit_weekly_cleaning_quota
    ON cleaning_requests (dorm_unit_id, year, iso_week)
    WHERE status <> 'cancelled';

COMMIT;
