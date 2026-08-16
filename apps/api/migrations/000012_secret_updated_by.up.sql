-- Record who last changed each secret.
ALTER TABLE secrets ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Existing secrets: the most recent secret.updated audit event, otherwise the creator.
UPDATE secrets s
SET updated_by = a.user_id
FROM (
    SELECT DISTINCT ON (resource_id) resource_id, user_id
    FROM audit_logs
    WHERE action = 'secret.updated' AND resource_type = 'secret'
    ORDER BY resource_id, timestamp DESC
) a
WHERE a.resource_id = s.id;

UPDATE secrets SET updated_by = created_by WHERE updated_by IS NULL;
