-- Rollback: 0005_session_management

DROP INDEX IF EXISTS idx_sessions_revoked_at;
DROP INDEX IF EXISTS idx_sessions_expires_at_all;
DROP INDEX IF EXISTS idx_sessions_user_created_active;

ALTER TABLE sessions
    DROP CONSTRAINT IF EXISTS sessions_replaced_by_fkey;

ALTER TABLE sessions
    ADD CONSTRAINT sessions_replaced_by_fkey
    FOREIGN KEY (replaced_by) REFERENCES sessions(id);
