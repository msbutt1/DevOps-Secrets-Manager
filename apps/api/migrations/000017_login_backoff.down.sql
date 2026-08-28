ALTER TABLE users DROP COLUMN IF EXISTS login_locked_until;
ALTER TABLE users DROP COLUMN IF EXISTS failed_login_attempts;
