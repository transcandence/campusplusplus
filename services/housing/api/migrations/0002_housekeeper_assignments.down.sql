-- 0002_housekeeper_assignments.down.sql
-- Rollback for the housekeeper operator -> residence assignment table.

DROP TABLE IF EXISTS housekeeper_assignments CASCADE;
