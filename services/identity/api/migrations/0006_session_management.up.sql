-- 0005_session_management.up.sql
--
-- Supports session listing, per-user session limits, and stale-session GC.
--
-- replaced_by is a self-reference. Without ON DELETE SET NULL, deleting an
-- old session that is still pointed to by a newer row would violate the FK
-- and block the cleanup job.

ALTER TABLE sessions
    DROP CONSTRAINT IF EXISTS sessions_replaced_by_fkey;

ALTER TABLE sessions
    ADD CONSTRAINT sessions_replaced_by_fkey
    FOREIGN KEY (replaced_by) REFERENCES sessions(id)
    ON DELETE SET NULL;

-- Listing and per-user limit queries filter on active sessions and sort by
-- creation date.
CREATE INDEX IF NOT EXISTS idx_sessions_user_created_active
    ON sessions (user_id, created_at DESC, id DESC)
    WHERE revoked_at IS NULL;

-- Cleanup queries scan expired and revoked rows. The existing
-- idx_sessions_expires_at_active is partial on revoked_at IS NULL and does
-- not help the cleanup job.
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at_all
    ON sessions (expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_revoked_at
    ON sessions (revoked_at)
    WHERE revoked_at IS NOT NULL;
