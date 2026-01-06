-- Add email_verified column to users table
ALTER TABLE users ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT false;

-- Create email_verification_tokens table
CREATE TABLE email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index on user_id for faster lookups
CREATE INDEX idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);

-- Create index on token_hash for faster lookups
CREATE INDEX idx_email_verification_tokens_hash ON email_verification_tokens(token_hash);

-- Create index on expires_at for cleanup queries
CREATE INDEX idx_email_verification_tokens_expires_at ON email_verification_tokens(expires_at);
