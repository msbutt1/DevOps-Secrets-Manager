-- Read-only, environment-scoped tokens for CI pipelines and servers. Only a SHA-256 hash of
-- each token is stored; the prefix is kept to help people recognise a token.
CREATE TABLE service_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    environment_id UUID NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    token_prefix VARCHAR(16) NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_service_tokens_environment_id ON service_tokens(environment_id);

-- Audit events can now be performed by a service token instead of a user.
ALTER TABLE audit_logs ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE audit_logs ADD COLUMN service_token_id UUID REFERENCES service_tokens(id) ON DELETE SET NULL;
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_actor_present CHECK (user_id IS NOT NULL OR service_token_id IS NOT NULL);
CREATE INDEX idx_audit_logs_service_token_id ON audit_logs(service_token_id);
