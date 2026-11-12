-- 0001_init.up.sql
-- Housing Service Schema Migration
-- Defines spatial hierarchy, occupancy (stays), and cleaning operations aggregates.

-- ---------------------------------------------------------------------------
-- 1. Custom Enums
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'residence_gender_policy') THEN
        CREATE TYPE residence_gender_policy AS ENUM ('male_only', 'female_only');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'dorm_unit_type') THEN
        CREATE TYPE dorm_unit_type AS ENUM (
            'room',
            'apart',
            'corridor',
            'kitchen',
            'laundry'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'stay_status') THEN
        CREATE TYPE stay_status AS ENUM (
            'planned',
            'active',
            'ended',
            'cancelled'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cleaning_request_status') THEN
        CREATE TYPE cleaning_request_status AS ENUM (
            'pending',
            'scheduled',
            'completed',
            'cancelled'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cleaning_shift_status') THEN
        CREATE TYPE cleaning_shift_status AS ENUM (
            'planned',
            'in_progress',
            'completed'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cleaning_task_status') THEN
        CREATE TYPE cleaning_task_status AS ENUM (
            'pending',
            'in_progress',
            'done',
            'skipped'
        );
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- 2. Trigger Function for updated_at
-- ---------------------------------------------------------------------------

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- 3. Campuses Reference Table (Tenancy Boundary)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS campuses (
    id         SERIAL PRIMARY KEY,
    code       VARCHAR(30) UNIQUE NOT NULL,
    name       VARCHAR(60) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT campuses_code_format_ck CHECK (code ~ '^[a-z0-9_-]+$')
);

INSERT INTO campuses (code, name) VALUES
    ('rabat', 'Rabat'),
    ('benguerir', 'Benguerir'),
    ('laayoune', 'Laayoune')
ON CONFLICT (code) DO NOTHING;

CREATE TRIGGER trg_campuses_updated_at
BEFORE UPDATE ON campuses
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------------
-- 4. Spatial Hierarchy
-- ---------------------------------------------------------------------------

-- Residence: identified by a numeric code (e.g. 8, 12, 20) within a campus
CREATE TABLE IF NOT EXISTS residences (
    id            SERIAL PRIMARY KEY,
    campus_id     INTEGER NOT NULL REFERENCES campuses(id) ON DELETE RESTRICT,
    code          VARCHAR(30) NOT NULL, -- e.g. "8", "12", "20"
    name          VARCHAR(100) NOT NULL,
    gender_policy residence_gender_policy NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_residences_campus_code UNIQUE (campus_id, code),
    CONSTRAINT ck_residence_code_numeric CHECK (code ~ '^[0-9]+$')
);

CREATE TRIGGER trg_residences_updated_at
BEFORE UPDATE ON residences
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_residences_campus_id ON residences (campus_id);

-- Building: identified by a letter code (e.g. A, B, C) within a residence
CREATE TABLE IF NOT EXISTS buildings (
    id           SERIAL PRIMARY KEY,
    residence_id INTEGER NOT NULL REFERENCES residences(id) ON DELETE CASCADE,
    code         VARCHAR(10) NOT NULL, -- e.g. "A", "B", "C"
    name         VARCHAR(100) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_buildings_residence_code UNIQUE (residence_id, code),
    CONSTRAINT ck_building_code_letter CHECK (code ~ '^[A-Za-z]+$')
);

CREATE TRIGGER trg_buildings_updated_at
BEFORE UPDATE ON buildings
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_buildings_residence_id ON buildings (residence_id);

-- Floor: identified by floor number (0, 1, 2...) within a building
CREATE TABLE IF NOT EXISTS floors (
    id           SERIAL PRIMARY KEY,
    building_id  INTEGER NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    floor_number INTEGER NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_floors_building_number UNIQUE (building_id, floor_number),
    CONSTRAINT ck_floor_number_non_negative CHECK (floor_number >= 0)
);

CREATE TRIGGER trg_floors_updated_at
BEFORE UPDATE ON floors
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_floors_building_id ON floors (building_id);

-- Dorm Units: living units and common areas
-- Unit Code Standard: [Residence][Building][Floor]-[Unit] (e.g., 12A2-1, 12A2-CORR)
CREATE TABLE IF NOT EXISTS dorm_units (
    id          SERIAL PRIMARY KEY,
    floor_id    INTEGER NOT NULL REFERENCES floors(id) ON DELETE CASCADE,
    unit_number VARCHAR(30) NOT NULL, -- e.g. "1", "14", "CORR", "KITCH", "LAUND"
    code        VARCHAR(60) NOT NULL UNIQUE, -- Canonical full code e.g. "12A2-1", "12A2-CORR"
    unit_type   dorm_unit_type NOT NULL,
    capacity    INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_dorm_units_floor_unit_number UNIQUE (floor_id, unit_number),
    -- Capacity invariants:
    -- Common areas (corridor, kitchen, laundry) must have capacity = 0
    -- Room must have capacity = 1
    -- Apart must have capacity >= 1
    CONSTRAINT ck_dorm_unit_capacity CHECK (
        (unit_type IN ('corridor', 'kitchen', 'laundry') AND capacity = 0) OR
        (unit_type = 'room' AND capacity = 1) OR
        (unit_type = 'apart' AND capacity >= 1)
    )
);

CREATE TRIGGER trg_dorm_units_updated_at
BEFORE UPDATE ON dorm_units
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_dorm_units_floor_id ON dorm_units (floor_id);
CREATE INDEX idx_dorm_units_unit_type ON dorm_units (unit_type);

-- Building Staff Assignments (Maître d'internat assignments)
CREATE TABLE IF NOT EXISTS building_supervisors (
    id          SERIAL PRIMARY KEY,
    building_id INTEGER NOT NULL REFERENCES buildings(id) ON DELETE CASCADE,
    user_id     INTEGER NOT NULL, -- UserID from Identity service
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_building_supervisors UNIQUE (building_id, user_id)
);

CREATE INDEX idx_building_supervisors_user_id ON building_supervisors (user_id);

-- ---------------------------------------------------------------------------
-- 5. Occupancy: The Stay Aggregate
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS stays (
    id              SERIAL PRIMARY KEY,
    student_id      INTEGER NOT NULL, -- UserID of student from Identity service
    dorm_unit_id    INTEGER NOT NULL REFERENCES dorm_units(id) ON DELETE RESTRICT,
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    status          stay_status NOT NULL DEFAULT 'planned',
    assigned_by     INTEGER NOT NULL, -- UserID of Housing Manager
    checked_in_by   INTEGER,          -- UserID of Maître d'internat
    checked_in_at   TIMESTAMPTZ,
    checked_out_by  INTEGER,          -- UserID of Maître d'internat
    checked_out_at  TIMESTAMPTZ,
    cancelled_by    INTEGER,
    cancelled_at    TIMESTAMPTZ,
    cancellation_reason TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_stay_dates CHECK (end_date >= start_date)
);

CREATE TRIGGER trg_stays_updated_at
BEFORE UPDATE ON stays
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_stays_student_id ON stays (student_id);
CREATE INDEX idx_stays_dorm_unit_id ON stays (dorm_unit_id);
CREATE INDEX idx_stays_status ON stays (status);

-- Invariant 1: Single Active Stay across all campuses per student
CREATE UNIQUE INDEX uq_student_active_stay ON stays (student_id)
WHERE status = 'active';

-- ---------------------------------------------------------------------------
-- 6. Cleaning Operations
-- ---------------------------------------------------------------------------

-- Student on-demand cleaning request
CREATE TABLE IF NOT EXISTS cleaning_requests (
    id              SERIAL PRIMARY KEY,
    dorm_unit_id    INTEGER NOT NULL REFERENCES dorm_units(id) ON DELETE RESTRICT,
    student_id      INTEGER NOT NULL, -- Requesting student UserID
    stay_id         INTEGER NOT NULL REFERENCES stays(id) ON DELETE RESTRICT,
    scheduled_date  DATE NOT NULL,    -- Execution date (strictly tomorrow from request date)
    status          cleaning_request_status NOT NULL DEFAULT 'pending',
    year            INTEGER NOT NULL,
    iso_week        INTEGER NOT NULL, -- ISO week number (1-53)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Enforce 1 cleaning request per unit per calendar week (Monday–Sunday)
    CONSTRAINT uq_unit_weekly_cleaning_quota UNIQUE (dorm_unit_id, year, iso_week)
);

CREATE TRIGGER trg_cleaning_requests_updated_at
BEFORE UPDATE ON cleaning_requests
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_cleaning_requests_dorm_unit_id ON cleaning_requests (dorm_unit_id);
CREATE INDEX idx_cleaning_requests_scheduled_date ON cleaning_requests (scheduled_date);
CREATE INDEX idx_cleaning_requests_student_id ON cleaning_requests (student_id);

-- Cleaning Shifts for Housekeeper Operators
CREATE TABLE IF NOT EXISTS cleaning_shifts (
    id            SERIAL PRIMARY KEY,
    operator_id   INTEGER NOT NULL, -- UserID of Housekeeper Operator
    supervisor_id INTEGER NOT NULL, -- UserID of Housekeeper Supervisor
    campus_id     INTEGER NOT NULL REFERENCES campuses(id) ON DELETE RESTRICT,
    shift_date    DATE NOT NULL,
    start_time    TIME NOT NULL DEFAULT '08:00:00',
    end_time      TIME NOT NULL DEFAULT '17:00:00',
    status        cleaning_shift_status NOT NULL DEFAULT 'planned',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ck_shift_operating_hours CHECK (
        start_time >= '08:00:00' AND end_time <= '17:00:00' AND end_time > start_time
    )
);

CREATE TRIGGER trg_cleaning_shifts_updated_at
BEFORE UPDATE ON cleaning_shifts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_cleaning_shifts_operator_date ON cleaning_shifts (operator_id, shift_date);
CREATE INDEX idx_cleaning_shifts_supervisor ON cleaning_shifts (supervisor_id);

-- Cleaning Tasks within a Shift
CREATE TABLE IF NOT EXISTS cleaning_tasks (
    id                  SERIAL PRIMARY KEY,
    shift_id            INTEGER NOT NULL REFERENCES cleaning_shifts(id) ON DELETE CASCADE,
    dorm_unit_id        INTEGER NOT NULL REFERENCES dorm_units(id) ON DELETE RESTRICT,
    cleaning_request_id INTEGER REFERENCES cleaning_requests(id) ON DELETE SET NULL, -- Null for common areas
    task_order          INTEGER NOT NULL DEFAULT 1,
    status              cleaning_task_status NOT NULL DEFAULT 'pending',
    completed_at        TIMESTAMPTZ,
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_cleaning_tasks_updated_at
BEFORE UPDATE ON cleaning_tasks
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_cleaning_tasks_shift_id ON cleaning_tasks (shift_id);
CREATE INDEX idx_cleaning_tasks_dorm_unit_id ON cleaning_tasks (dorm_unit_id);
CREATE INDEX idx_cleaning_tasks_status ON cleaning_tasks (status);
