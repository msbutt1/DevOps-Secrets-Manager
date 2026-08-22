-- Email addresses are case-insensitive: store them trimmed and lower-cased, and allow only one
-- account per address regardless of case. If two existing accounts differ only by case this
-- migration fails on the unique index; merge or rename them first.
UPDATE users SET email = lower(trim(email)) WHERE email <> lower(trim(email));

CREATE UNIQUE INDEX idx_users_email_lower_unique ON users (lower(email));
