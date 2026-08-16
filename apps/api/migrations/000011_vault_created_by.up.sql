-- Record who created each vault.
ALTER TABLE vaults ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Existing vaults: use the audit log where it has the creation event, otherwise the
-- earliest owner membership (the creator is added as owner when a vault is created).
UPDATE vaults v
SET created_by = a.user_id
FROM (
    SELECT DISTINCT ON (vault_id) vault_id, user_id
    FROM audit_logs
    WHERE action IN ('vault.created', 'VAULT_CREATED')
    ORDER BY vault_id, timestamp
) a
WHERE v.created_by IS NULL AND a.vault_id = v.id;

UPDATE vaults v
SET created_by = m.user_id
FROM (
    SELECT DISTINCT ON (vault_id) vault_id, user_id
    FROM vault_members
    WHERE role = 'owner'
    ORDER BY vault_id, created_at
) m
WHERE v.created_by IS NULL AND m.vault_id = v.id;
