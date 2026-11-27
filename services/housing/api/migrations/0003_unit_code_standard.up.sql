-- 0003_unit_code_standard.up.sql
-- Housing Service: enforce the unit code standard [Residence][Building][Floor]-[Unit].
--
-- 0001 stored dorm_units.code as a free-text column: nothing tied it to the
-- parent residence/building/floor, and UNIQUE(code) was global even though the
-- campus is the tenancy boundary (Res 12 / Bldg A / Floor 2 / Unit 1 legitimately
-- exists in more than one campus). This migration derives the code from the
-- hierarchy, keeps it in sync when a parent is renamed, and constrains its shape.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Canonical code builder
-- ---------------------------------------------------------------------------
-- Single source of truth for the code layout: <residence><building><floor>-<unit>
-- e.g. residence "12", building "A", floor 2, unit "14"  ->  12A2-14

CREATE OR REPLACE FUNCTION housing_unit_code(p_floor_id INT, p_unit_number TEXT)
RETURNS TEXT AS $$
    SELECT r.code || b.code || f.floor_number::text || '-' || p_unit_number
    FROM floors f
    JOIN buildings  b ON b.id = f.building_id
    JOIN residences r ON r.id = b.residence_id
    WHERE f.id = p_floor_id;
$$ LANGUAGE sql STABLE;

-- ---------------------------------------------------------------------------
-- 2. Derive the code on write
-- ---------------------------------------------------------------------------
-- code is never trusted from the client: it is recomputed from the parent
-- hierarchy on every insert, and whenever a unit is reparented (floor_id) or
-- renumbered (unit_number). Because BEFORE ROW triggers run ahead of the
-- NOT NULL check, an insert may simply omit the column.
--
-- housing_unit_code() returns NULL when floor_id does not resolve, so the
-- trigger raises instead of storing an unbuildable code.

CREATE OR REPLACE FUNCTION housing_dorm_units_set_code()
RETURNS TRIGGER AS $$
BEGIN
    NEW.code := housing_unit_code(NEW.floor_id, NEW.unit_number);
    IF NEW.code IS NULL THEN
        RAISE EXCEPTION 'dorm_units.floor_id % does not exist', NEW.floor_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_dorm_units_set_code ON dorm_units;
CREATE TRIGGER trg_dorm_units_set_code
BEFORE INSERT OR UPDATE OF floor_id, unit_number, code ON dorm_units
FOR EACH ROW EXECUTE FUNCTION housing_dorm_units_set_code();

-- ---------------------------------------------------------------------------
-- 3. Keep codes in sync when a parent is renamed
-- ---------------------------------------------------------------------------
-- A code is derived, so renaming a residence, a building, or a floor silently
-- invalidates every descendant code unless we resync. `SET unit_number =
-- unit_number` is intentional: touching the column is what re-fires
-- trg_dorm_units_set_code for each row.

CREATE OR REPLACE FUNCTION housing_resync_unit_codes(p_scope TEXT, p_id INT)
RETURNS VOID AS $$
BEGIN
    IF p_scope = 'floor' THEN
        UPDATE dorm_units SET unit_number = unit_number WHERE floor_id = p_id;
    ELSIF p_scope = 'building' THEN
        UPDATE dorm_units d SET unit_number = d.unit_number
        FROM floors f
        WHERE f.id = d.floor_id AND f.building_id = p_id;
    ELSIF p_scope = 'residence' THEN
        UPDATE dorm_units d SET unit_number = d.unit_number
        FROM floors f JOIN buildings b ON b.id = f.building_id
        WHERE f.id = d.floor_id AND b.residence_id = p_id;
    ELSE
        RAISE EXCEPTION 'unknown resync scope %', p_scope;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION housing_trg_resync_floor() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.floor_number <> OLD.floor_number THEN
        PERFORM housing_resync_unit_codes('floor', NEW.id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_floors_resync_unit_codes ON floors;
CREATE TRIGGER trg_floors_resync_unit_codes
AFTER UPDATE ON floors
FOR EACH ROW EXECUTE FUNCTION housing_trg_resync_floor();

CREATE OR REPLACE FUNCTION housing_trg_resync_building() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.code <> OLD.code THEN
        PERFORM housing_resync_unit_codes('building', NEW.id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_buildings_resync_unit_codes ON buildings;
CREATE TRIGGER trg_buildings_resync_unit_codes
AFTER UPDATE ON buildings
FOR EACH ROW EXECUTE FUNCTION housing_trg_resync_building();

CREATE OR REPLACE FUNCTION housing_trg_resync_residence() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.code <> OLD.code THEN
        PERFORM housing_resync_unit_codes('residence', NEW.id);
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_residences_resync_unit_codes ON residences;
CREATE TRIGGER trg_residences_resync_unit_codes
AFTER UPDATE ON residences
FOR EACH ROW EXECUTE FUNCTION housing_trg_resync_residence();

-- ---------------------------------------------------------------------------
-- 4. Drop the global unique, backfill, then constrain
-- ---------------------------------------------------------------------------
-- code was UNIQUE across the whole table, but the campus is the tenancy
-- boundary: residence "12" / building "A" / floor 2 / unit 1 exists in Rabat
-- *and* in Benguerir, so both units legitimately share the code 12A2-1.
-- Uniqueness within a campus is already guaranteed by
-- uq_dorm_units_floor_unit_number plus the unique parent codes, so the global
-- constraint is dropped rather than rebuilt.
--
-- Order matters: backfill first, then constrain. Adding the CHECK before
-- re-deriving codes would reject any row 0001 stored with a hand-written code.

ALTER TABLE dorm_units DROP CONSTRAINT IF EXISTS dorm_units_code_key;

-- Re-derive every stored code through trg_dorm_units_set_code.
UPDATE dorm_units SET unit_number = unit_number;

-- Last line of defence for a code written by anything that bypasses the
-- trigger (trigger disabled, replication, a future migration).
ALTER TABLE dorm_units DROP CONSTRAINT IF EXISTS ck_dorm_unit_code_format;
ALTER TABLE dorm_units ADD CONSTRAINT ck_dorm_unit_code_format
    CHECK (code ~ '^[0-9]+[A-Za-z]+[0-9]+-[A-Za-z0-9]+$');

-- Living units are numbered; common areas use the reserved masks.
-- Reserved masks match docs/domain-model.md: CORR / KITCH / LAUND.
ALTER TABLE dorm_units DROP CONSTRAINT IF EXISTS ck_dorm_unit_number_format;
ALTER TABLE dorm_units ADD CONSTRAINT ck_dorm_unit_number_format
    CHECK (
        (unit_type IN ('room', 'apart') AND unit_number ~ '^[0-9]+$') OR
        (unit_type = 'corridor' AND unit_number = 'CORR')  OR
        (unit_type = 'kitchen'  AND unit_number = 'KITCH') OR
        (unit_type = 'laundry'  AND unit_number = 'LAUND')
    );

COMMIT;
