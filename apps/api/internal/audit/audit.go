package audit

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Action constants define the types of auditable actions. The same names, in the same order,
// are listed in AUDIT_ACTIONS in apps/web/src/types/api.ts; a test keeps the two in sync.
const (
	ActionLoginSuccess      = "login.success"
	ActionLoginFailure      = "login.failure"
	ActionSecretCreated     = "secret.created"
	ActionSecretUpdated     = "secret.updated"
	ActionSecretDeleted     = "secret.deleted"
	ActionSecretRevealed    = "secret.revealed"
	ActionMemberAdded       = "member.added"
	ActionMemberRemoved     = "member.removed"
	ActionMemberRoleChanged = "member.role_changed"
	ActionVaultCreated      = "vault.created"
	ActionVaultUpdated      = "vault.updated"
	ActionVaultDeleted      = "vault.deleted"
	ActionEnvCreated        = "env.created"
	ActionEnvUpdated        = "env.updated"
	ActionEnvDeleted        = "env.deleted"

	ActionOrgUpdated           = "org.updated"
	ActionOrgMemberRoleChanged = "org.member_role_changed"
	ActionOrgMemberRemoved     = "org.member_removed"
	ActionInviteCreated        = "invite.created"
	ActionInviteRevoked        = "invite.revoked"
	ActionInviteAccepted       = "invite.accepted"
	ActionTokenCreated         = "token.created"
	ActionTokenRevoked         = "token.revoked"
	// ActionEnvExported records reading every value in an environment at once (service tokens).
	ActionEnvExported = "env.exported"
)

// Actions lists every action the API records.
var Actions = []string{
	ActionLoginSuccess,
	ActionLoginFailure,
	ActionSecretCreated,
	ActionSecretUpdated,
	ActionSecretDeleted,
	ActionSecretRevealed,
	ActionMemberAdded,
	ActionMemberRemoved,
	ActionMemberRoleChanged,
	ActionVaultCreated,
	ActionVaultUpdated,
	ActionVaultDeleted,
	ActionEnvCreated,
	ActionEnvUpdated,
	ActionEnvDeleted,
	ActionOrgUpdated,
	ActionOrgMemberRoleChanged,
	ActionOrgMemberRemoved,
	ActionInviteCreated,
	ActionInviteRevoked,
	ActionInviteAccepted,
	ActionTokenCreated,
	ActionTokenRevoked,
	ActionEnvExported,
}

// Event describes something to record in the audit log. The actor is UserID, or ServiceTokenID
// for machine access (UserID is then uuid.Nil).
type Event struct {
	UserID         uuid.UUID
	ServiceTokenID *uuid.UUID
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
	UserID         uuid.UUID // uuid.Nil when a service token acted
	ServiceTokenID *uuid.UUID
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

	// Joined for display; empty when the row does not reference them. For service tokens
	// UserEmail is "token:<name>".
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
	// ExcludeActions drops events with any of these actions (e.g. logins on the dashboard).
	ExcludeActions []string
	ResourceType   *string
	StartTime      *time.Time // inclusive
	EndTime        *time.Time // exclusive
	Limit          int
	Offset         int
}

// Repository defines the interface for audit log data access
type Repository interface {
	Append(ctx context.Context, entry *AuditEntry) error
	Query(ctx context.Context, filters QueryFilters) ([]*AuditEntry, int, error)
}
