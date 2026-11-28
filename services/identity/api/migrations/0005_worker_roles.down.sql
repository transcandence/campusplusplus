-- 0004_worker_roles.down.sql
-- Revert the worker sub-role dimension: drop the CHECK, the column, and the
-- enum type, in that order (the type can only be dropped once nothing
-- references it).

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_worker_role_ck;
ALTER TABLE users DROP COLUMN IF EXISTS worker_role;
DROP TYPE IF EXISTS worker_role;
