-- Record who added each vault member.
ALTER TABLE vault_members ADD COLUMN added_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Existing members: the member.added audit event where there is one; the vault creator's own
-- owner membership was created by the creator.
UPDATE vault_members vm
SET added_by = a.user_id
FROM (
    SELECT DISTINCT ON (vault_id, resource_id) vault_id, resource_id, user_id
    FROM audit_logs
    WHERE action = 'member.added'
    ORDER BY vault_id, resource_id, timestamp DESC
) a
WHERE vm.added_by IS NULL AND a.vault_id = vm.vault_id AND a.resource_id = vm.user_id;

UPDATE vault_members vm
SET added_by = v.created_by
FROM vaults v
WHERE vm.added_by IS NULL AND v.id = vm.vault_id AND v.created_by = vm.user_id;
