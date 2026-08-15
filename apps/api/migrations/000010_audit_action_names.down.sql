UPDATE audit_logs SET action = CASE action
    WHEN 'login.success' THEN 'LOGIN_SUCCESS'
    WHEN 'login.failure' THEN 'LOGIN_FAILURE'
    WHEN 'secret.created' THEN 'SECRET_CREATED'
    WHEN 'secret.revealed' THEN 'SECRET_REVEALED'
    WHEN 'secret.updated' THEN 'SECRET_UPDATED'
    WHEN 'secret.deleted' THEN 'SECRET_DELETED'
    WHEN 'vault.created' THEN 'VAULT_CREATED'
    WHEN 'vault.deleted' THEN 'VAULT_DELETED'
    WHEN 'member.role_changed' THEN 'PERMISSION_CHANGED'
    WHEN 'token.refreshed' THEN 'TOKEN_REFRESHED'
    ELSE action
END;
