package organizations

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a PostgreSQL-backed organization repository
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

const organizationSelect = `
	SELECT o.id, o.name, uo.role, o.created_at, o.updated_at,
		(SELECT COUNT(*) FROM user_organizations m WHERE m.organization_id = o.id),
		(SELECT COUNT(*) FROM vaults v WHERE v.organization_id = o.id AND v.deleted_at IS NULL)
	FROM organizations o
	JOIN user_organizations uo ON uo.organization_id = o.id AND uo.user_id = $1
	WHERE o.deleted_at IS NULL
`

func scanOrganization(row pgx.Row) (*Organization, error) {
	var o Organization
	if err := row.Scan(&o.ID, &o.Name, &o.Role, &o.CreatedAt, &o.UpdatedAt, &o.MemberCount, &o.VaultCount); err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *postgresRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]*Organization, error) {
	rows, err := r.pool.Query(ctx, organizationSelect+` ORDER BY o.name, o.created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}
	defer rows.Close()

	orgs := make([]*Organization, 0)
	for rows.Next() {
		o, err := scanOrganization(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

func (r *postgresRepository) GetForUser(ctx context.Context, orgID, userID uuid.UUID) (*Organization, error) {
	o, err := scanOrganization(r.pool.QueryRow(ctx, organizationSelect+` AND o.id = $2`, userID, orgID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return o, nil
}

func (r *postgresRepository) Rename(ctx context.Context, orgID uuid.UUID, name string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE organizations SET name = $2, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, orgID, name)
	if err != nil {
		return fmt.Errorf("failed to rename organization: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

const memberSelect = `
	SELECT u.id, u.email, u.name, uo.role, uo.created_at,
		(SELECT MAX(al.timestamp) FROM audit_logs al WHERE al.user_id = u.id AND al.action = 'login.success')
	FROM user_organizations uo
	JOIN users u ON u.id = uo.user_id AND u.deleted_at IS NULL
	WHERE uo.organization_id = $1
`

func scanMember(row pgx.Row) (*Member, error) {
	var m Member
	if err := row.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.JoinedAt, &m.LastLoginAt); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *postgresRepository) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*Member, error) {
	rows, err := r.pool.Query(ctx, memberSelect+` ORDER BY uo.created_at, u.email`, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization members: %w", err)
	}
	defer rows.Close()

	members := make([]*Member, 0)
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *postgresRepository) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*Member, error) {
	m, err := scanMember(r.pool.QueryRow(ctx, memberSelect+` AND uo.user_id = $2`, orgID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMemberNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get member: %w", err)
	}
	return m, nil
}

func (r *postgresRepository) CountOwners(ctx context.Context, orgID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_organizations WHERE organization_id = $1 AND role = 'owner'`, orgID).Scan(&n)
	return n, err
}

func (r *postgresRepository) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	tag, err := r.pool.Exec(ctx, `UPDATE user_organizations SET role = $3 WHERE organization_id = $1 AND user_id = $2`, orgID, userID, role)
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return nil
}

func (r *postgresRepository) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		DELETE FROM vault_members vm USING vaults v
		WHERE vm.vault_id = v.id AND v.organization_id = $1 AND vm.user_id = $2`, orgID, userID); err != nil {
		return fmt.Errorf("failed to remove vault memberships: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM user_organizations WHERE organization_id = $1 AND user_id = $2`, orgID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrMemberNotFound
	}
	return tx.Commit(ctx)
}
