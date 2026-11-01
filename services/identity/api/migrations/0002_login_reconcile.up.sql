-- 0002_login_reconcile.up.sql
--
-- This is the second and final migration for the login slice. It takes the
-- 0001 schema (which was written before we knew what login needed) and
-- reshapes it to what the auth code actually uses. After this runs, the
-- schema is final: no 0003.
--
-- What it does, in order:
--   1. Turns the old VARCHAR+CHECK columns into native Postgres enums.
--   2. Replaces users.is_active with users.status (an enum).
--   3. Adds a campuses table and users.campus_id (every user belongs to one campus).
--   4. Makes email case-insensitive (Sara@x == sara@x).
--   5. Converts every timestamp to TIMESTAMPTZ (timezone-aware).
--   6. Replaces sessions.token (plaintext) with sessions.token_hash (SHA-256, bytes).
--   7. Adds session lifecycle columns: revoked_at, last_used_at, family_id,
--      rotated_at, replaced_by. These make logout, rotation, and reuse
--      detection possible.
--   8. Adds the indexes those queries actually use.
--   9. Adds updated_at triggers so updated_at actually changes on UPDATE.
--
-- Transaction handling: this file does NOT open its own BEGIN/COMMIT. The
-- migration runner (cmd/migrate) wraps it in a transaction. If you ever
-- switch to a runner that doesn't, wrap this file in BEGIN; ... COMMIT; —
-- never both.
--
-- WARNING — this migration TRUNCATEs the sessions table. Every session that
-- exists before this runs is deleted. That's on purpose: the old column
-- held plaintext tokens, which cannot be turned into SHA-256 hashes, and
-- which may not be hex at all. There is no safe conversion. In dev, the
-- table is empty, so nothing happens. In any shared environment, this logs
-- everyone out once. Plan accordingly.

-- ---------------------------------------------------------------------------
-- 1. Create the enum types.
-- ---------------------------------------------------------------------------
--
-- Enums are better than VARCHAR+CHECK because the database itself rejects
-- bad values (the CHECK can be forgotten on a new column; the enum cannot).
-- The IF NOT EXISTS guard makes this safe to re-run if a previous attempt
-- failed halfway and the transaction was rolled back after the CREATE TYPE.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'affiliation') THEN
        CREATE TYPE affiliation AS ENUM (
            '1337', 'ABS', 'FGSES', 'SHBM',
            'SAS', 'CC', 'AEM', 'um6p'
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        CREATE TYPE user_role AS ENUM (
            'student', 'staff', 'admin', 'guest', 'worker'
        );
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'gender') THEN
        CREATE TYPE gender AS ENUM ('male', 'female', 'other');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_status') THEN
        CREATE TYPE user_status AS ENUM (
            'pending_verification', 'active', 'suspended'
        );
    END IF;
END
$$;

-- ---------------------------------------------------------------------------
-- 2. Convert users.affiliation, users.role, users.gender to the new enums.
-- ---------------------------------------------------------------------------
--
-- The old columns were VARCHAR with a CHECK constraint. To convert them:
--   1. Drop the CHECK (it would reject enum values otherwise).
--   2. Change the column type with USING <old>::<new>, which casts each
--      existing value to the enum (fails loudly if a value isn't in the enum).
--   3. For `role`, the old column had a DEFAULT 'student' (varchar). A
--      varchar default cannot be auto-cast to an enum, so drop the default
--      before the type change and set it again afterwards with the enum
--      type. This is the standard Postgres dance.

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_affiliation_check;
ALTER TABLE users
    ALTER COLUMN affiliation TYPE affiliation
    USING affiliation::affiliation;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ALTER COLUMN role DROP DEFAULT;
ALTER TABLE users
    ALTER COLUMN role TYPE user_role
    USING role::user_role;
ALTER TABLE users
    ALTER COLUMN role SET DEFAULT 'student'::user_role;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_gender_check;
ALTER TABLE users
    ALTER COLUMN gender TYPE gender
    USING gender::gender;

-- ---------------------------------------------------------------------------
-- 3. Add users.status, backfill from users.is_active, then drop is_active.
-- ---------------------------------------------------------------------------
--
-- is_active was a boolean (true = can log in). The login slice needs three
-- states, not two:
--   pending_verification — account exists but email is not verified yet
--   active               — can log in
--   suspended            — cannot log in (banned / locked)
--
-- Backfill rule:
--   * is_active = TRUE  -> 'active' (keep existing users able to log in)
--   * is_active = FALSE -> 'pending_verification' (we can't tell "never
--     verified" from "banned", so the safe default is "not yet active";
--     if this is wrong for your data, fix it BEFORE running the migration).
--
-- New rows default to 'pending_verification'. The registration flow
-- (future work) must explicitly UPDATE to 'active' after email verification.

ALTER TABLE users
    ADD COLUMN status user_status NOT NULL DEFAULT 'pending_verification';

UPDATE users SET status = 'active' WHERE is_active = TRUE;

ALTER TABLE users DROP COLUMN is_active;

-- ---------------------------------------------------------------------------
-- 4. Campuses: the tenancy boundary.
-- ---------------------------------------------------------------------------
--
-- Every user belongs to exactly one campus. The JWT carries the campus
-- *code* (a short string like 'rabat') rather than the numeric id, so logs
-- and traces are readable. The codes are lowercase ASCII, immutable, and
-- enforced by a CHECK below.
--
-- These rows are REFERENCE DATA, not demo data. They must exist before
-- users.campus_id can be NOT NULL. Do not move them to a seed script.

CREATE TABLE IF NOT EXISTS campuses (
    id         SERIAL PRIMARY KEY,
    code       VARCHAR(30) UNIQUE NOT NULL,
    name       VARCHAR(60) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE campuses
    ADD CONSTRAINT campuses_code_format_ck
    CHECK (code ~ '^[a-z0-9_-]+$');

INSERT INTO campuses (code, name) VALUES
    ('rabat',    'Rabat'),
    ('benguerir', 'Benguerir'),
    ('laayoune', 'Laayoune')
ON CONFLICT (code) DO NOTHING;

-- Add the FK to users. The column is NOT NULL because every user must
-- belong to a campus. ON DELETE RESTRICT means you cannot delete a campus
-- that still has users — that's intentional; deleting a campus should be
-- an explicit, considered operation, not a cascade that silently orphans
-- or deletes users.

ALTER TABLE users
    ADD COLUMN campus_id INTEGER NOT NULL
        REFERENCES campuses(id) ON DELETE RESTRICT;

-- Existing users (if any) get 'rabat'. If your real users belong
-- elsewhere, change this backfill BEFORE running the migration.
-- After this UPDATE, the column has no DEFAULT, so future INSERTs must
-- choose a campus explicitly (the INSERT will fail with a NOT NULL
-- violation if they don't — loud, not silent).
UPDATE users
   SET campus_id = (SELECT id FROM campuses WHERE code = 'rabat')
 WHERE campus_id IS NULL;

CREATE INDEX idx_users_campus_id ON users (campus_id);

-- ---------------------------------------------------------------------------
-- 5. Email becomes case-insensitive.
-- ---------------------------------------------------------------------------
--
-- The old UNIQUE(email) treated 'Sara@campus.ma' and 'sara@campus.ma' as
-- different users. That's wrong for login: the same person typing their
-- email with different capitalisation should reach the same account.
--
-- CITEXT is a Postgres type that compares case-insensitively. UNIQUE on a
-- CITEXT column means two emails that differ only by case collide — which
-- is exactly what we want. The upside over `lower(email)` with a
-- functional index: every existing query like `WHERE email = $1` keeps
-- using the index and stays case-insensitive without remembering to call
-- lower() everywhere.
--
-- The app should STILL normalize (trim + lowercase) before storing or
-- querying. CITEXT only handles comparison, not whitespace, not Unicode
-- normalization, not display. Normalization is the app's job; CITEXT is
-- the safety net.

CREATE EXTENSION IF NOT EXISTS citext;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
ALTER TABLE users
    ALTER COLUMN email TYPE CITEXT
    USING email::citext;
ALTER TABLE users
    ADD CONSTRAINT users_email_key UNIQUE (email);

-- ---------------------------------------------------------------------------
-- 6. Every timestamp becomes TIMESTAMPTZ (timezone-aware).
-- ---------------------------------------------------------------------------
--
-- The old columns were TIMESTAMP (no timezone). A timestamp without a zone
-- is ambiguous: "2024-01-01 12:00" is different times in different places.
-- TIMESTAMPTZ stores an absolute instant, which is what session expiry and
-- audit trails need.
--
-- Conversion: the existing values had no zone, so we interpret them as UTC
-- (Go's time.Now() writes UTC by default, so this is the correct
-- assumption). AT TIME ZONE 'UTC' says "this timestamp was in UTC" and
-- produces a TIMESTAMPTZ.

ALTER TABLE users
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE profiles
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE settings
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC';

ALTER TABLE sessions
    ALTER COLUMN created_at TYPE TIMESTAMPTZ USING created_at AT TIME ZONE 'UTC',
    ALTER COLUMN updated_at TYPE TIMESTAMPTZ USING updated_at AT TIME ZONE 'UTC',
    ALTER COLUMN expires_at TYPE TIMESTAMPTZ USING expires_at AT TIME ZONE 'UTC';

-- ---------------------------------------------------------------------------
-- 7. Sessions: replace plaintext token with a SHA-256 hash, add lifecycle.
-- ---------------------------------------------------------------------------
--
-- WHY THE TRUNCATE:
--   The old `sessions.token` column held the raw refresh token (a string).
--   We now want to store only SHA-256(token), as raw bytes (BYTEA, 32 bytes).
--   You cannot convert a plaintext token to its hash in SQL in a way that
--   works for any input (the old values might be JWTs, base64url, UUIDs, or
--   already-hex — there is no single decode). The only safe options are
--   (a) drop the data, or (b) re-hash every row using pgcrypto and hope the
--   values were raw tokens. (a) is correct. The table is empty in dev; in
--   shared environments, this logs everyone out once.
--
-- WHY BYTEA AND NOT TEXT:
--   Storing the hash as BYTEA (raw 32 bytes) is smaller and removes any
--   ambiguity about hex encoding vs. base64. The Go code does
--   `h := sha256.Sum256([]byte(token))` and passes `h[:]` to the driver.
--
-- WHY SHA-256 AND NOT ARGON2/BCRYPT:
--   Refresh tokens are 256 bits of random data. There is nothing to slow
--   down. A slow hash would just make every refresh take 100ms. Fast hash
--   is correct for high-entropy inputs. Password hashing (which DOES need
--   to be slow) is a separate concern and lives in the app's password code.

TRUNCATE sessions;

ALTER TABLE sessions DROP COLUMN token;
ALTER TABLE sessions ADD COLUMN token_hash BYTEA NOT NULL;
ALTER TABLE sessions
    ADD CONSTRAINT sessions_token_hash_uniq UNIQUE (token_hash);

ALTER TABLE sessions
    ADD CONSTRAINT sessions_expiry_ck CHECK (expires_at > created_at);

COMMENT ON COLUMN sessions.token_hash IS
    'SHA-256 of the opaque refresh token. 32 bytes. Never store the token itself.';

-- Drop the one-session-per-user constraint. Login needs one session per
-- device (laptop + phone + tablet), and rotation needs the old and new
-- rows to coexist briefly. user_id is indexed (below), but not UNIQUE.
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_user_id_key;
CREATE INDEX idx_sessions_user_id ON sessions (user_id);

-- Lifecycle columns:
--
--   revoked_at    - NULL means the session is active. Set to now() on
--                   logout, on rotation (old row), and on suspend. Rows are
--                   NEVER deleted immediately; they're kept for reuse
--                   detection (see family_id below) and audit. A GC job
--                   hard-deletes them after both revoked_at and expires_at
--                   are older than the refresh-token lifetime (30 days).
--
--   last_used_at  - updated on each /auth/refresh only, not on every
--                   authenticated request. Updating on every request would
--                   put a write on the hot path for no benefit; 10-minute
--                   accuracy is enough. NULL until the first refresh.
--
--   family_id     - Every login creates one family. Every rotation creates
--                   a new row in the SAME family. If a token that has
--                   already been rotated is presented (i.e. someone is
--                   replaying an old refresh token), we revoke the ENTIRE
--                   family. This is what makes "stolen refresh token" a
--                   one-time weapon instead of a 30-day one.
--
--   rotated_at    - When this row was rotated. NULL = never rotated.
--                   Non-NULL + revoked_at IS NOT NULL = a rotation
--                   happened, so a replay is detectable.
--
--   replaced_by   - The id of the row that replaced this one. Informational,
--                   and useful for debugging a chain of rotations.

ALTER TABLE sessions
    ADD COLUMN revoked_at   TIMESTAMPTZ NULL,
    ADD COLUMN last_used_at TIMESTAMPTZ NULL,
    ADD COLUMN family_id    UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN rotated_at   TIMESTAMPTZ NULL,
    ADD COLUMN replaced_by  INTEGER REFERENCES sessions(id);

COMMENT ON COLUMN sessions.revoked_at IS
    'NULL = active. Set on logout, rotation, suspend. Soft delete; GC removes after reuse window.';
COMMENT ON COLUMN sessions.last_used_at IS
    'Updated on refresh only (every ~10 min), not on every request.';

CREATE INDEX idx_sessions_family_id ON sessions (family_id);

-- ---------------------------------------------------------------------------
-- 8. Partial indexes for the queries the auth code actually runs.
-- ---------------------------------------------------------------------------
--
-- Each query below is a hot path, and each index is tailored to it:
--
--   "find a session by token hash"       -> sessions_token_hash_uniq (already)
--   "active sessions for a user"         -> idx_sessions_user_id_active
--   "cleanup expired sessions"           -> idx_sessions_expires_at_active
--   "revoke all sessions for a user"     -> idx_sessions_user_id_active (same)
--   "revoke family on reuse"             -> idx_sessions_family_id (already)
--
-- The `WHERE revoked_at IS NULL` clause makes these indexes smaller and
-- faster: the auth code only ever looks at active rows. If you forget the
-- `revoked_at IS NULL` filter in a query, Postgres won't use the index and
-- you'll notice the slowness — that's a feature.
--
-- NOTE: do not add an index on (user_id, created_at) for the "list active
-- sessions" screen yet. That endpoint doesn't exist. Add it when it does.

CREATE INDEX idx_sessions_user_id_active
    ON sessions (user_id)    WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_expires_at_active
    ON sessions (expires_at) WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- 9. updated_at triggers.
-- ---------------------------------------------------------------------------
--
-- Postgres has no ON UPDATE clause; the standard idiom is a trigger. One
-- function, attached to every table that has an updated_at column. Without
-- this, updated_at never changes after INSERT, which makes it useless for
-- debugging ("when did this row last change?").
--
-- NOTE: now() returns the transaction start time, not the statement time.
-- If one transaction updates a row twice, updated_at reflects the start of
-- the transaction. That's fine for audit purposes; if you ever need
-- per-statement time, switch to clock_timestamp().

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_users_updated_at    ON users;
DROP TRIGGER IF EXISTS trg_profiles_updated_at ON profiles;
DROP TRIGGER IF EXISTS trg_settings_updated_at ON settings;
DROP TRIGGER IF EXISTS trg_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS trg_campuses_updated_at ON campuses;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_profiles_updated_at
    BEFORE UPDATE ON profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_settings_updated_at
    BEFORE UPDATE ON settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_sessions_updated_at
    BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_campuses_updated_at
    BEFORE UPDATE ON campuses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
