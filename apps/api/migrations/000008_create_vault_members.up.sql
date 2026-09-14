-- Per-vault membership and roles, used by the vault, member, and secret handlers.
CREATE TABLE IF NOT EXISTS vault_members (
    vault_id UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'developer', 'oncall', 'viewer')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (vault_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_vault_members_user_id ON vault_members(user_id);

-- Vaults created before this table existed: give their organization's members access with their organization role.
INSERT INTO vault_members (vault_id, user_id, role, created_at)
SELECT v.id, uo.user_id, uo.role, v.created_at
FROM vaults v
JOIN user_organizations uo ON uo.organization_id = v.organization_id
WHERE v.deleted_at IS NULL
ON CONFLICT DO NOTHING;
