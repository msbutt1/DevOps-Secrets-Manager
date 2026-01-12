package policy

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNoRole is returned when the user has no role in the organization
	ErrNoRole = errors.New("user has no role in this organization")
)

// policyService implements the PolicyService interface
type policyService struct {
	pool *pgxpool.Pool
}

// NewPolicyService creates a new instance of PolicyService
func NewPolicyService(pool *pgxpool.Pool) PolicyService {
	return &policyService{
		pool: pool,
	}
}

// GetUserRole retrieves the user's role in a specific organization
func (s *policyService) GetUserRole(ctx context.Context, userID, orgID uuid.UUID) (string, error) {
	query := `SELECT role FROM user_organizations WHERE user_id = $1 AND organization_id = $2`
	var role string
	err := s.pool.QueryRow(ctx, query, userID, orgID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNoRole
		}
		return "", err
	}
	return role, nil
}

// Can checks if a user has permission to perform an action within an organization
func (s *policyService) Can(ctx context.Context, userID uuid.UUID, action Action, orgID uuid.UUID) (bool, error) {
	// Get the user's role
	role, err := s.GetUserRole(ctx, userID, orgID)
	if err != nil {
		return false, err
	}

	// Check permission based on role and action
	return s.hasPermission(role, action), nil
}

// hasPermission checks if a role has permission to perform an action
func (s *policyService) hasPermission(role string, action Action) bool {
	// Permission matrix
	switch role {
	case RoleOwner:
		// Owner has ALL permissions
		return true

	case RoleAdmin:
		// Admin has ALL permissions
		return true

	case RoleDeveloper:
		// Developer: vault/env/secret read+write+reveal, NO delete, NO member manage
		switch action {
		case ActionVaultRead, ActionVaultWrite:
			return true
		case ActionEnvRead, ActionEnvWrite:
			return true
		case ActionSecretRead, ActionSecretWrite, ActionSecretReveal:
			return true
		case ActionVaultDelete, ActionEnvDelete, ActionSecretDelete:
			return false
		case ActionAuditRead, ActionMemberManage:
			return false
		default:
			return false
		}

	case RoleOncall:
		// Oncall: read ALL, reveal secrets, NO write, NO delete, NO member manage
		switch action {
		case ActionVaultRead, ActionEnvRead, ActionSecretRead, ActionSecretReveal:
			return true
		case ActionVaultWrite, ActionVaultDelete:
			return false
		case ActionEnvWrite, ActionEnvDelete:
			return false
		case ActionSecretWrite, ActionSecretDelete:
			return false
		case ActionAuditRead, ActionMemberManage:
			return false
		default:
			return false
		}

	case RoleViewer:
		// Viewer: read-only (no reveal, no write, no delete, no member manage)
		switch action {
		case ActionVaultRead, ActionEnvRead, ActionSecretRead:
			return true
		case ActionVaultWrite, ActionVaultDelete:
			return false
		case ActionEnvWrite, ActionEnvDelete:
			return false
		case ActionSecretWrite, ActionSecretDelete, ActionSecretReveal:
			return false
		case ActionAuditRead, ActionMemberManage:
			return false
		default:
			return false
		}

	default:
		// Unknown role: deny all
		return false
	}
}
