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

// Event describes something to record in the audit log.
type Event struct {
	UserID         uuid.UUID
	Action         string
	TargetType     string
	TargetID       *uuid.UUID
	TargetName     string
	OrganizationID *uuid.UUID
	VaultID        *uuid.UUID
	EnvironmentID  *uuid.UUID
	Metadata       map[string]interface{}
}

// AuditEntry represents a single audit log entry
type AuditEntry struct {
	ID             uuid.UUID
	Timestamp      time.Time
	UserID         uuid.UUID
	OrganizationID *uuid.UUID
	VaultID        *uuid.UUID
	EnvironmentID  *uuid.UUID
	Action         string
	ResourceType   string
	ResourceID     *uuid.UUID
	TargetName     *string
	IPAddress      *string
	UserAgent      *string
	Metadata       map[string]interface{}

	// Joined for display; empty when the row does not reference them.
	UserEmail       string
	VaultName       *string
	EnvironmentName *string
}

// QueryFilters defines the filters for querying audit logs
type QueryFilters struct {
	// ViewerID limits results to events the viewer may see: their own events, every event in
	// organizations they own or administer, and events in vaults where they are owner or admin.
	ViewerID      *uuid.UUID
	UserID        *uuid.UUID
	UserEmail     *string // case-insensitive substring
	OrgID         *uuid.UUID
	VaultID       *uuid.UUID
	EnvironmentID *uuid.UUID
	Action        *string
	ResourceType  *string
	StartTime     *time.Time // inclusive
	EndTime       *time.Time // exclusive
	Limit         int
	Offset        int
}

// Repository defines the interface for audit log data access
type Repository interface {
	Append(ctx context.Context, entry *AuditEntry) error
	Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, int, error)
}
