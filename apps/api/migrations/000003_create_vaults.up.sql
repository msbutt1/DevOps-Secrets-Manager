CREATE TABLE vaults (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    encrypted_dek BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_vaults_organization_id ON vaults(organization_id);
CREATE INDEX idx_vaults_deleted_at ON vaults(deleted_at);
CREATE UNIQUE INDEX idx_vaults_org_name_unique ON vaults(organization_id, name) WHERE deleted_at IS NULL;
