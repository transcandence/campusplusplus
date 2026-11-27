-- 0004_worker_roles.up.sql
--
-- Adds the worker sub-role dimension to users. `role` stays coarse
-- (student / worker / staff / admin / guest); a new nullable `worker_role`
-- column carries the concrete function a worker performs, ONLY when
-- role = 'worker'.
--
-- Why a separate column instead of more enum values on `user_role`:
--   * `role` is the tenancy-level identity classification. Keeping it to the
--     five coarse values means JWT, rate-limiting, and every service that
--     only cares "is this a student or a worker?" doesn't have to enumerate
--     six extra strings.
--   * `worker_role` is the functional team within the worker bucket. It is
--     NULL for non-workers, so the two axes can never disagree silently.
--
-- The fine-grained functions (from docs/domain-model.md and the housing
-- actor model):
--   housing_manager       - assigns/plans stays, manages inventory
--   housekeeper_supervisor - monitors shift execution, handles exceptions
--   housekeeper_operator   - executes cleaning shifts
--
-- Why enums (not VARCHAR + CHECK): same reasoning as 0002 — the database
-- itself rejects bad values; a CHECK can be forgotten on a new column.
--
-- Transaction handling: like 0002, this file does NOT open its own
-- BEGIN/COMMIT. The migration runner (cmd/migrate) wraps it in a
-- transaction.

-- ---------------------------------------------------------------------------
-- 1. Create the enum type (guarded, safe to re-run).
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'worker_role') THEN
        CREATE TYPE worker_role AS ENUM (
            'housing_manager',
            'housekeeper_supervisor',
            'housekeeper_operator'
        );
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- 2. Add the column (nullable, no default).
-- ---------------------------------------------------------------------------
ALTER TABLE users ADD COLUMN worker_role worker_role;

-- ---------------------------------------------------------------------------
-- 3. Backfill existing workers before the CHECK can be added.
-- ---------------------------------------------------------------------------
--
-- Any row that already has role = 'worker' (dev seed: worker@campus.ma)
-- must get a non-NULL worker_role or the CHECK below would reject it.
-- 'housekeeper_operator' is the safest default: it is the least privileged
-- of the worker functions. If real environments have workers with other
-- functions, override this backfill BEFORE running the migration.
UPDATE users
   SET worker_role = 'housekeeper_operator'::worker_role
 WHERE role = 'worker'
   AND worker_role IS NULL;

-- ---------------------------------------------------------------------------
-- 4. Enforce the role <-> worker_role invariant.
-- ---------------------------------------------------------------------------
--
-- Exactly one of two states must hold:
--   * a worker has a worker_role, or
--   * a non-worker has NULL worker_role.
-- There is no state where a non-worker carries a worker_role, nor a worker
-- with none. This is the single source of truth; the app's Go code mirrors
-- it but the DB guarantees it.
ALTER TABLE users
    ADD CONSTRAINT users_worker_role_ck CHECK (
        (role = 'worker' AND worker_role IS NOT NULL)
        OR
        (role <> 'worker' AND worker_role IS NULL)
    );
