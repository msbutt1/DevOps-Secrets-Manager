-- A session is one login: the family of refresh tokens that replace each other on refresh.
-- The table keeps what users need to recognise their sessions; a session is active while it
-- has an unrevoked, unexpired refresh token.
CREATE TABLE user_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip_address TEXT,
    user_agent TEXT
);

CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);

-- Sessions that existed before this table have no address or user agent recorded
INSERT INTO user_sessions (id, user_id, created_at, last_used_at)
SELECT token_family, MIN(user_id::text)::uuid, MIN(created_at), MAX(created_at)
FROM refresh_tokens
GROUP BY token_family;
