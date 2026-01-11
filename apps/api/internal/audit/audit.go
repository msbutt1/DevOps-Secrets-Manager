package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Action constants define the types of auditable actions
const (
	ActionLoginSuccess      = "LOGIN_SUCCESS"
	ActionLoginFailure      = "LOGIN_FAILURE"
	ActionSecretCreated     = "SECRET_CREATED"
	ActionSecretRevealed    = "SECRET_REVEALED"
	ActionSecretUpdated     = "SECRET_UPDATED"
	ActionSecretDeleted     = "SECRET_DELETED"
	ActionVaultCreated      = "VAULT_CREATED"
	ActionVaultDeleted      = "VAULT_DELETED"
	ActionPermissionChanged = "PERMISSION_CHANGED"
	ActionTokenRefreshed    = "TOKEN_REFRESHED"
)

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	ID             uuid.UUID
	Timestamp      time.Time
	UserID         uuid.UUID
	OrganizationID *uuid.UUID
	VaultID        *uuid.UUID
	Action         string
	ResourceType   string
	ResourceID     *uuid.UUID
	IPAddress      *string
	UserAgent      *string
	Metadata       map[string]interface{}
}

// QueryFilters defines the filters for querying audit logs
type QueryFilters struct {
	UserID       *uuid.UUID
	OrgID        *uuid.UUID
	VaultID      *uuid.UUID
	Action       *string
	ResourceType *string
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Offset       int
}

// Repository defines the interface for audit log data access
type Repository interface {
	Append(ctx context.Context, entry *AuditEntry) error
	Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, error)
}
