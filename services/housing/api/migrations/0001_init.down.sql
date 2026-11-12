-- 0001_init.down.sql
-- Rollback for Housing Service Schema Migration

-- Drop triggers and tables in reverse dependency order

DROP TRIGGER IF EXISTS trg_cleaning_tasks_updated_at ON cleaning_tasks;
DROP TABLE IF EXISTS cleaning_tasks CASCADE;

DROP TRIGGER IF EXISTS trg_cleaning_shifts_updated_at ON cleaning_shifts;
DROP TABLE IF EXISTS cleaning_shifts CASCADE;

DROP TRIGGER IF EXISTS trg_cleaning_requests_updated_at ON cleaning_requests;
DROP TABLE IF EXISTS cleaning_requests CASCADE;

DROP TRIGGER IF EXISTS trg_stays_updated_at ON stays;
DROP TABLE IF EXISTS stays CASCADE;

DROP TABLE IF EXISTS building_supervisors CASCADE;

DROP TRIGGER IF EXISTS trg_dorm_units_updated_at ON dorm_units;
DROP TABLE IF EXISTS dorm_units CASCADE;

DROP TRIGGER IF EXISTS trg_floors_updated_at ON floors;
DROP TABLE IF EXISTS floors CASCADE;

DROP TRIGGER IF EXISTS trg_buildings_updated_at ON buildings;
DROP TABLE IF EXISTS buildings CASCADE;

DROP TRIGGER IF EXISTS trg_residences_updated_at ON residences;
DROP TABLE IF EXISTS residences CASCADE;

DROP TRIGGER IF EXISTS trg_campuses_updated_at ON campuses;
DROP TABLE IF EXISTS campuses CASCADE;

DROP FUNCTION IF EXISTS set_updated_at();

-- Drop Custom Enums
DROP TYPE IF EXISTS cleaning_task_status;
DROP TYPE IF EXISTS cleaning_shift_status;
DROP TYPE IF EXISTS cleaning_request_status;
DROP TYPE IF EXISTS stay_status;
DROP TYPE IF EXISTS dorm_unit_type;
DROP TYPE IF EXISTS residence_gender_policy;
