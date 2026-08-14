package policy

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// CanOnOrg checks an organization-scoped action; users outside the organization get ErrNoRole.
func (s *policyService) CanOnOrg(ctx context.Context, userID, orgID uuid.UUID, action Action) (bool, error) {
	role, err := s.GetUserRole(ctx, userID, orgID)
	if err != nil {
		return false, err
	}
	return OrgRoleAllows(role, action), nil
}

// VaultRole returns the caller's effective role on a live vault.
func (s *policyService) VaultRole(ctx context.Context, userID, vaultID uuid.UUID) (string, error) {
	query := `
		SELECT COALESCE(vm.role, ''), COALESCE(uo.role, '')
		FROM vaults v
		LEFT JOIN vault_members vm ON vm.vault_id = v.id AND vm.user_id = $1
		LEFT JOIN user_organizations uo ON uo.organization_id = v.organization_id AND uo.user_id = $1
		WHERE v.id = $2 AND v.deleted_at IS NULL
	`
	var vaultRole, orgRole string
	if err := s.pool.QueryRow(ctx, query, userID, vaultID).Scan(&vaultRole, &orgRole); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrVaultNotFound
		}
		return "", err
	}

	role := EffectiveVaultRole(vaultRole, orgRole)
	if role == "" {
		return "", ErrVaultNotFound
	}
	return role, nil
}

// CanOnVault checks a vault-scoped action against the caller's effective vault role.
func (s *policyService) CanOnVault(ctx context.Context, userID, vaultID uuid.UUID, action Action) (bool, error) {
	role, err := s.VaultRole(ctx, userID, vaultID)
	if err != nil {
		return false, err
	}
	return RoleAllows(role, action), nil
}
