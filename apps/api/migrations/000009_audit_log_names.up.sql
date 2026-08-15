-- Record which environment an event happened in and the name of its target at the time,
-- so the audit log can show names without depending on metadata.
ALTER TABLE audit_logs ADD COLUMN environment_id UUID REFERENCES environments(id);
ALTER TABLE audit_logs ADD COLUMN target_name TEXT;

CREATE INDEX idx_audit_logs_environment_id ON audit_logs(environment_id);

-- Backfill from the metadata written by earlier versions.
UPDATE audit_logs a
SET environment_id = e.id
FROM environments e
WHERE a.environment_id IS NULL
  AND a.metadata ? 'environment_id'
  AND e.id::text = a.metadata->>'environment_id';

UPDATE audit_logs
SET target_name = metadata->>'key_name'
WHERE target_name IS NULL AND metadata ? 'key_name';

UPDATE audit_logs a
SET target_name = v.name
FROM vaults v
WHERE a.target_name IS NULL AND a.resource_type = 'vault' AND a.resource_id = v.id;
