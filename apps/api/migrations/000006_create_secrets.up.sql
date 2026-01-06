CREATE TABLE secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    key_name VARCHAR(255) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    description TEXT,
    rotation_interval_days INTEGER,
    last_rotated_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    metadata JSONB,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_secrets_environment_id ON secrets(environment_id);
CREATE INDEX idx_secrets_deleted_at ON secrets(deleted_at);

CREATE UNIQUE INDEX idx_secrets_environment_key_unique
ON secrets(environment_id, key_name)
WHERE deleted_at IS NULL;
