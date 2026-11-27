-- Rollback: 0004_activate_registered_users
--
-- Restore the previous default. Existing accounts are intentionally NOT put
-- back into 'pending_verification': the down migration cannot know which
-- accounts were already active before the up migration, and re-locking every
-- user would be destructive. Re-apply 0002's default instead.

ALTER TABLE users
    ALTER COLUMN status SET DEFAULT 'pending_verification'::user_status;
