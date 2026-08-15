-- Audit actions use dotted lowercase names shared with the web app (e.g. secret.revealed).
UPDATE audit_logs SET action = CASE action
    WHEN 'LOGIN_SUCCESS' THEN 'login.success'
    WHEN 'LOGIN_FAILURE' THEN 'login.failure'
    WHEN 'SECRET_CREATED' THEN 'secret.created'
    WHEN 'SECRET_REVEALED' THEN 'secret.revealed'
    WHEN 'SECRET_UPDATED' THEN 'secret.updated'
    WHEN 'SECRET_DELETED' THEN 'secret.deleted'
    WHEN 'VAULT_CREATED' THEN 'vault.created'
    WHEN 'VAULT_DELETED' THEN 'vault.deleted'
    WHEN 'PERMISSION_CHANGED' THEN 'member.role_changed'
    WHEN 'TOKEN_REFRESHED' THEN 'token.refreshed'
    ELSE action
END;
