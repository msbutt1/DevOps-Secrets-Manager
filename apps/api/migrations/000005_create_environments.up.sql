CREATE TABLE environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vault_id UUID NOT NULL REFERENCES vaults(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_environments_vault_id ON environments(vault_id);
CREATE INDEX idx_environments_deleted_at ON environments(deleted_at);
CREATE UNIQUE INDEX idx_environments_vault_name_unique ON environments(vault_id, name) WHERE deleted_at IS NULL;
