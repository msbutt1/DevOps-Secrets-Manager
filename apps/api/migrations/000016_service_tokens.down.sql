-- Token-performed events have no user and cannot satisfy the old NOT NULL constraint.
DELETE FROM audit_logs WHERE user_id IS NULL;
DROP INDEX IF EXISTS idx_audit_logs_service_token_id;
ALTER TABLE audit_logs DROP CONSTRAINT IF EXISTS audit_logs_actor_present;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS service_token_id;
ALTER TABLE audit_logs ALTER COLUMN user_id SET NOT NULL;
DROP TABLE IF EXISTS service_tokens;
