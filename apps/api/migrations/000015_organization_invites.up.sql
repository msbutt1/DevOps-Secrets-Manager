-- Email invitations to join an organization. Only a SHA-256 hash of each token is stored.
CREATE TABLE organization_invites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'developer', 'oncall', 'viewer')),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    invited_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    accepted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_organization_invites_organization_id ON organization_invites(organization_id);

-- At most one open invite per address per organization.
CREATE UNIQUE INDEX idx_organization_invites_open_unique
    ON organization_invites (organization_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;
