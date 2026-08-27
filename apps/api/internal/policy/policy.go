package policy

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Action represents a specific permission action in the system
type Action string

// Vault-scoped actions. Every vault, environment, secret and member operation is checked
// against the caller's role on the vault that owns the resource.
const (
	ActionVaultRead    Action = "vault:read"
	ActionVaultWrite   Action = "vault:write"
	ActionVaultDelete  Action = "vault:delete"
	ActionEnvRead      Action = "env:read"
	ActionEnvWrite     Action = "env:write"
	ActionEnvDelete    Action = "env:delete"
	ActionSecretRead   Action = "secret:read"
	ActionSecretWrite  Action = "secret:write"
	ActionSecretDelete Action = "secret:delete"
	ActionSecretReveal Action = "secret:reveal"
	ActionMemberRead   Action = "member:read"
	ActionMemberManage Action = "member:manage"
	ActionAuditRead    Action = "audit:read"
	// ActionTokenManage covers creating, listing and revoking service tokens, which can read
	// every value in an environment.
	ActionTokenManage Action = "token:manage"
)

// Organization-scoped actions.
const (
	ActionOrgVaultCreate Action = "org:vault_create"
	ActionOrgManage      Action = "org:manage"
	ActionOrgAuditRead   Action = "org:audit_read"
)

// Role constants define all possible roles in the system
const (
	RoleOwner     = "owner"
	RoleAdmin     = "admin"
	RoleDeveloper = "developer"
	RoleOncall    = "oncall"
	RoleViewer    = "viewer"
)

var (
	// ErrNoRole is returned when the user has no role in the organization
	ErrNoRole = errors.New("user has no role in this organization")
	// ErrVaultNotFound is returned when the vault does not exist, is deleted, or the user
	// has no access to it. The two cases are deliberately indistinguishable to callers.
	ErrVaultNotFound = errors.New("vault not found")
)

// Permissions are the capabilities a vault role grants. They mirror ROLE_PERMISSIONS in
// apps/web/src/types/api.ts, which the web app uses to show or hide controls.
type Permissions struct {
	CanRead          bool
	CanWrite         bool
	CanReveal        bool
	CanManageMembers bool
	CanDelete        bool
}

var rolePermissions = map[string]Permissions{
	RoleOwner:     {CanRead: true, CanWrite: true, CanReveal: true, CanManageMembers: true, CanDelete: true},
	RoleAdmin:     {CanRead: true, CanWrite: true, CanReveal: true, CanManageMembers: true, CanDelete: false},
	RoleDeveloper: {CanRead: true, CanWrite: true, CanReveal: false, CanManageMembers: false, CanDelete: false},
	RoleOncall:    {CanRead: true, CanWrite: false, CanReveal: true, CanManageMembers: false, CanDelete: false},
	RoleViewer:    {CanRead: true, CanWrite: false, CanReveal: false, CanManageMembers: false, CanDelete: false},
}

// IsValidRole reports whether role is one of the five known roles.
func IsValidRole(role string) bool {
	_, ok := rolePermissions[role]
	return ok
}

// PermissionsFor returns the permissions of a vault role; unknown roles get none.
func PermissionsFor(role string) Permissions {
	return rolePermissions[role]
}

// RoleAllows reports whether a vault role may perform a vault-scoped action.
//
//   - read: view the vault, its environments, secret names and metadata, and members
//   - write: rename the vault, and create, update or delete environments and secrets
//   - reveal: decrypt secret values
//   - manage members: add, remove and change members, read the vault's audit log, and manage
//     service tokens
//   - delete: delete the vault
func RoleAllows(role string, action Action) bool {
	p := PermissionsFor(role)
	switch action {
	case ActionVaultRead, ActionEnvRead, ActionSecretRead, ActionMemberRead:
		return p.CanRead
	case ActionVaultWrite, ActionEnvWrite, ActionEnvDelete, ActionSecretWrite, ActionSecretDelete:
		return p.CanWrite
	case ActionSecretReveal:
		return p.CanReveal
	case ActionMemberManage, ActionAuditRead, ActionTokenManage:
		return p.CanManageMembers
	case ActionVaultDelete:
		return p.CanDelete
	default:
		return false
	}
}

// OrgRoleAllows reports whether an organization role may perform an organization action.
func OrgRoleAllows(role string, action Action) bool {
	switch action {
	case ActionOrgVaultCreate:
		return role == RoleOwner || role == RoleAdmin || role == RoleDeveloper
	case ActionOrgManage, ActionOrgAuditRead:
		return role == RoleOwner || role == RoleAdmin
	default:
		return false
	}
}

// EffectiveVaultRole combines a user's vault membership role and organization role.
// Organization owners and admins inherit that role on every vault in the organization;
// everyone else needs an explicit vault membership. A vault membership without organization
// membership grants nothing. An empty result means no access.
func EffectiveVaultRole(vaultRole, orgRole string) string {
	switch {
	case !IsValidRole(orgRole):
		return ""
	case orgRole == RoleOwner || vaultRole == RoleOwner:
		return RoleOwner
	case orgRole == RoleAdmin || vaultRole == RoleAdmin:
		return RoleAdmin
	case IsValidRole(vaultRole):
		return vaultRole
	default:
		return ""
	}
}

// PolicyService defines the interface for RBAC policy enforcement
type PolicyService interface {
	// VaultRole returns the caller's effective role on a vault, or ErrVaultNotFound when the
	// vault does not exist or the caller has no access to it.
	VaultRole(ctx context.Context, userID, vaultID uuid.UUID) (string, error)

	// CanOnVault checks a vault-scoped action. It returns ErrVaultNotFound when the caller
	// cannot see the vault at all, and false when their role lacks the permission.
	CanOnVault(ctx context.Context, userID, vaultID uuid.UUID, action Action) (bool, error)

	// GetUserRole retrieves the user's role in a specific organization
	GetUserRole(ctx context.Context, userID, orgID uuid.UUID) (string, error)

	// CanOnOrg checks an organization-scoped action.
	CanOnOrg(ctx context.Context, userID, orgID uuid.UUID, action Action) (bool, error)
}
