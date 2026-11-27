-- 0004_activate_registered_users.up.sql
--
-- Registration no longer waits for an email-verification step that does not
-- exist yet. New accounts are inserted with status = 'active' by the service,
-- and this migration makes the database default consistent with that rule.
--
-- It also activates accounts created by the previous registration code so
-- they can log in and be tested without a manual SQL update.

ALTER TABLE users
    ALTER COLUMN status SET DEFAULT 'active'::user_status;

UPDATE users
   SET status = 'active'::user_status
 WHERE status = 'pending_verification'::user_status;
