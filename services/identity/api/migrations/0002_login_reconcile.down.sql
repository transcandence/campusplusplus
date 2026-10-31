-- 0002_login_reconcile.down.sql
--
-- Reverses 0002_login_reconcile.up.sql. Runs in a transaction (the
-- migration runner wraps it; this file does NOT open BEGIN/COMMIT).
--
-- TWO IRREVERSIBLE LOSSES:
--
--   1. sessions.token (the old plaintext refresh token) is NOT restored.
--      The up migration TRUNCATEd the table before dropping the column,
--      so the data is gone. The down recreates an empty `token TEXT
--      UNIQUE NOT NULL` column, which means the restored schema cannot
--      hold any rows without a token — and every row that existed before
--      the up is lost. This is by design; there is no way to recover it.
--
--   2. users.is_active backfill is lossy. The up maps:
--        is_active = TRUE  -> 'active'
--        is_active = FALSE -> 'pending_verification'
--      The down maps:
--        'active'              -> TRUE
--        'pending_verification' -> FALSE
--        'suspended'            -> FALSE
--      So 'suspended' and 'pending_verification' both become FALSE. You
--      cannot tell them apart after a round-trip. If you need that
--      distinction preserved, do not run the down on real data.
--
-- Order: reverse of the up. Triggers first (they reference columns that
-- are about to disappear), then indexes, then columns, then types, then
-- the extension, then the constraint changes that were last in the up.

-- ---------------------------------------------------------------------------
-- 9. Drop triggers, then the trigger function.
-- ---------------------------------------------------------------------------
--
-- Triggers must be dropped before the columns/tables they reference.
-- Drop in reverse order of creation is not required (DROP TRIGGER IF
-- EXISTS is order-independent), but the function must go last, after
-- every trigger that uses it.

DROP TRIGGER IF EXISTS trg_campuses_updated_at ON campuses;
DROP TRIGGER IF EXISTS trg_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS trg_settings_updated_at ON settings;
DROP TRIGGER IF EXISTS trg_profiles_updated_at ON profiles;
DROP TRIGGER IF EXISTS trg_users_updated_at    ON users;

DROP FUNCTION IF EXISTS set_updated_at();

-- ---------------------------------------------------------------------------
-- 8. Drop the partial indexes from step 8.
-- ---------------------------------------------------------------------------
--
-- idx_sessions_family_id was created in step 7 (after the family_id
-- column), not step 8 — but it's an index on a column we're about to
-- drop, so it goes here. Dropping it explicitly is clearer than relying
-- on DROP COLUMN to cascade.

DROP INDEX IF EXISTS idx_sessions_expires_at_active;
DROP INDEX IF EXISTS idx_sessions_user_id_active;
DROP INDEX IF EXISTS idx_sessions_family_id;

-- ---------------------------------------------------------------------------
-- 7. Sessions: remove lifecycle columns, restore plaintext token.
-- ---------------------------------------------------------------------------
--
-- The up did, in order:
--   TRUNCATE sessions;
--   DROP COLUMN token;
--   ADD COLUMN token_hash BYTEA NOT NULL UNIQUE;
--   ADD CONSTRAINT sessions_expiry_ck;
--   DROP CONSTRAINT sessions_user_id_key;
--   CREATE INDEX idx_sessions_user_id;
--   ADD COLUMN revoked_at, last_used_at, family_id, rotated_at, replaced_by;
--   CREATE INDEX idx_sessions_family_id;
--
-- The down must reverse this. The tricky part: `token` was UNIQUE NOT
-- NULL, and `token_hash` is UNIQUE NOT NULL. We can't have both UNIQUE
-- NOT NULL columns populated at once if we want to allow rows without
-- hashes. Since the up TRUNCATEd, there are no rows to preserve — so we
-- drop token_hash entirely, re-add token as nullable first, then tighten
-- to NOT NULL after a (zero-row) backfill. This mirrors the up's
-- TRUNCATE-then-recreate approach.

ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_expiry_ck;

-- Drop the unique constraint on token_hash before dropping the column.
-- (DROP COLUMN would cascade, but being explicit documents intent.)
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_token_hash_uniq;
ALTER TABLE sessions DROP COLUMN IF EXISTS token_hash;

-- Restore the plaintext token column. NOT NULL UNIQUE, as in 0001.
-- The table is empty (the up TRUNCATEd it), so adding NOT NULL is safe.
ALTER TABLE sessions ADD COLUMN token TEXT NOT NULL;
ALTER TABLE sessions
    ADD CONSTRAINT sessions_token_key UNIQUE (token);

-- Restore the one-session-per-user UNIQUE constraint that the up dropped.
-- The up created idx_sessions_user_id as a non-unique index; the down
-- drops the index and re-adds the UNIQUE constraint (which creates its
-- own index).
DROP INDEX IF EXISTS idx_sessions_user_id;
ALTER TABLE sessions
    ADD CONSTRAINT sessions_user_id_key UNIQUE (user_id);

-- Drop the lifecycle columns from step 7.
ALTER TABLE sessions DROP COLUMN IF EXISTS replaced_by;
ALTER TABLE sessions DROP COLUMN IF EXISTS rotated_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS family_id;
ALTER TABLE sessions DROP COLUMN IF EXISTS last_used_at;
ALTER TABLE sessions DROP COLUMN IF EXISTS revoked_at;

-- ---------------------------------------------------------------------------
-- 6. Revert TIMESTAMPTZ back to TIMESTAMP (no timezone).
-- ---------------------------------------------------------------------------
--
-- AT TIME ZONE 'UTC' on a TIMESTAMPTZ produces a TIMESTAMP in UTC. That
-- round-trips the up's `USING ... AT TIME ZONE 'UTC'` exactly, provided
-- every value was written in UTC. It was (Go's time.Now() is UTC), so
-- this is lossless for data written by the app.

ALTER TABLE sessions
    ALTER COLUMN expires_at TYPE TIMESTAMP USING expires_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE settings
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE profiles
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

ALTER TABLE users
    ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN created_at TYPE TIMESTAMP USING created_at AT TIME ZONE 'UTC';

-- ---------------------------------------------------------------------------
-- 5. Revert email from CITEXT to VARCHAR(150).
-- ---------------------------------------------------------------------------
--
-- The extension is left in place (CREATE EXTENSION IF NOT EXISTS in the
-- up; dropping it could break other databases/schemas that use it, and
-- the down has no way to know whether the up created it or it was
-- already there). If you want a fully clean revert, drop the extension
-- manually after confirming nothing else uses it.

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
ALTER TABLE users
    ALTER COLUMN email TYPE VARCHAR(150)
    USING email::text;
ALTER TABLE users
    ADD CONSTRAINT users_email_key UNIQUE (email);

-- NOTE: citext extension is NOT dropped. See comment above.

-- ---------------------------------------------------------------------------
-- 4. Drop campuses and users.campus_id.
-- ---------------------------------------------------------------------------
--
-- ON DELETE RESTRICT on users.campus_id means we must drop the FK
-- (implicitly by dropping the column) before dropping the table.
-- Dropping the column removes the FK; dropping the index first is
-- explicit.

DROP INDEX IF EXISTS idx_users_campus_id;
ALTER TABLE users DROP COLUMN IF EXISTS campus_id;
DROP TABLE IF EXISTS campuses;

-- ---------------------------------------------------------------------------
-- 3. Restore users.is_active from users.status, then drop status.
-- ---------------------------------------------------------------------------
--
-- Lossy: 'suspended' and 'pending_verification' both become FALSE.
-- See the header comment.

ALTER TABLE users ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT TRUE;
UPDATE users SET is_active = (status = 'active');
ALTER TABLE users DROP COLUMN status;

-- ---------------------------------------------------------------------------
-- 2. Revert users.affiliation, users.role, users.gender to VARCHAR + CHECK.
-- ---------------------------------------------------------------------------
--
-- Reverse of the up's enum conversion:
--   1. Change the type back to VARCHAR with USING <enum>::text.
--   2. Re-add the CHECK constraint (the up dropped it).
--   3. For `role`, the up set DEFAULT 'student'::user_role; drop it
--      before the type change (varchar default on an enum column would
--      fail), then re-set it as a varchar default.

ALTER TABLE users
    ALTER COLUMN gender TYPE VARCHAR(30)
    USING gender::text;
ALTER TABLE users
    ADD CONSTRAINT users_gender_check
    CHECK (gender IN ('male', 'female', 'other'));

ALTER TABLE users ALTER COLUMN role DROP DEFAULT;
ALTER TABLE users
    ALTER COLUMN role TYPE VARCHAR(30)
    USING role::text;
ALTER TABLE users
    ALTER COLUMN role SET DEFAULT 'student';
ALTER TABLE users
    ADD CONSTRAINT users_role_check
    CHECK (role IN ('student', 'staff', 'admin', 'guest', 'worker'));

ALTER TABLE users
    ALTER COLUMN affiliation TYPE VARCHAR(30)
    USING affiliation::text;
ALTER TABLE users
    ADD CONSTRAINT users_affiliation_check
    CHECK (affiliation IN (
        '1337', 'ABS', 'FGSES', 'SHBM',
        'SAS', 'CC', 'AEM', 'um6p'
    ));

-- ---------------------------------------------------------------------------
-- 1. Drop the enum types.
-- ---------------------------------------------------------------------------
--
-- Must come after every column that uses them has been converted back to
-- VARCHAR (steps 2 and 3 above). DROP TYPE IF EXISTS is safe even if the
-- up's DO-block guard skipped creation (e.g. a partial previous run).

DROP TYPE IF EXISTS user_status;
DROP TYPE IF EXISTS gender;
DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS affiliation;
