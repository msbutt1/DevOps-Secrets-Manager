-- Every value a secret has had, encrypted with the vault's data key, so a bad update can be
-- rolled back. secrets.version is the number of the current value.
ALTER TABLE secrets ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

CREATE TABLE secret_versions (
    secret_id UUID NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
    version INTEGER NOT NULL CHECK (version > 0),
    encrypted_value BYTEA NOT NULL,
    nonce BYTEA NOT NULL,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Set when the version was made by restoring an earlier one
    restored_from INTEGER,
    PRIMARY KEY (secret_id, version)
);

-- The value each existing secret has now becomes its first recorded version
INSERT INTO secret_versions (secret_id, version, encrypted_value, nonce, created_by, created_at)
SELECT id, 1, encrypted_value, nonce, COALESCE(updated_by, created_by), updated_at
FROM secrets;
