package policy

import (
	"context"

	"github.com/google/uuid"
)

// Action represents a specific permission action in the system
type Action string

// Action constants define all possible actions in the system
const (
	ActionVaultRead      Action = "vault:read"
	ActionVaultWrite     Action = "vault:write"
	ActionVaultDelete    Action = "vault:delete"
	ActionEnvRead        Action = "env:read"
	ActionEnvWrite       Action = "env:write"
	ActionEnvDelete      Action = "env:delete"
	ActionSecretRead     Action = "secret:read"
	ActionSecretWrite    Action = "secret:write"
	ActionSecretDelete   Action = "secret:delete"
	ActionSecretReveal   Action = "secret:reveal"
	ActionAuditRead      Action = "audit:read"
	ActionMemberManage   Action = "member:manage"
)

// Role constants define all possible roles in the system
const (
	RoleOwner     = "owner"
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleOncall    = "oncall"
	RoleViewer    = "viewer"
)

// PolicyService defines the interface for RBAC policy enforcement
type PolicyService interface {
	// Can checks if a user has permission to perform an action within an organization
	Can(ctx context.Context, userID uuid.UUID, action Action, orgID uuid.UUID) (bool, error)

	// GetUserRole retrieves the user's role in a specific organization
	GetUserRole(ctx context.Context, userID, orgID uuid.UUID) (string, error)
}
