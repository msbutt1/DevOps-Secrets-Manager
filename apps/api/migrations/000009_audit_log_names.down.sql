DROP INDEX IF EXISTS idx_audit_logs_environment_id;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS target_name;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS environment_id;
