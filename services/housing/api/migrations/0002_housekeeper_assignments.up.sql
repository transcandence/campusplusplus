-- 0002_housekeeper_assignments.up.sql
-- Housing Service: housekeeper operator -> residence assignment.
--
-- The scheduling algorithm treats each housekeeper operator as indivisible
-- and fixed to one residence ("an operator never leaves their residence").
-- This table records that assignment.
--
-- user_id is the Identity user id. It is NOT a foreign key because the row
-- lives in the identity service's database; the assignment must point to a
-- worker with worker_role = 'housekeeper_operator', which the service layer
-- enforces via the JWT claim (not here, since cross-database FKs are not
-- possible).
--
-- UNIQUE(user_id) enforces one residence per operator, matching the
-- algorithm's "never leaves their residence" rule. A supervisor is
-- campus/region-wide and therefore has no assignment row.

CREATE TABLE IF NOT EXISTS housekeeper_assignments (
    id           SERIAL PRIMARY KEY,
    user_id      INTEGER NOT NULL,
    residence_id INTEGER NOT NULL REFERENCES residences(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_housekeeper_assignments UNIQUE (user_id)
);

CREATE INDEX idx_housekeeper_assignments_residence_id
    ON housekeeper_assignments (residence_id);
