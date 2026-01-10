package vaults

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL-backed vault repository
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{
		pool: pool,
	}
}

// Create inserts a new vault into the database
func (r *postgresRepository) Create(ctx context.Context, vault *Vault) error {
	query := `
		INSERT INTO vaults (id, organization_id, name, description, encrypted_dek, created_at, updated_at)
		VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		vault.ID,
		vault.OrganizationID,
		vault.Name,
		vault.Description,
		vault.EncryptedDEK,
		vault.CreatedAt,
		vault.UpdatedAt,
	).Scan(&vault.ID, &vault.CreatedAt, &vault.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return fmt.Errorf("failed to create vault: %w", err)
	}

	return nil
}

// GetByID retrieves a vault by its ID
func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Vault, error) {
	query := `
		SELECT id, organization_id, name, description, encrypted_dek, created_at, updated_at, deleted_at
		FROM vaults
		WHERE id = $1 AND deleted_at IS NULL
	`

	var vault Vault
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&vault.ID,
		&vault.OrganizationID,
		&vault.Name,
		&vault.Description,
		&vault.EncryptedDEK,
		&vault.CreatedAt,
		&vault.UpdatedAt,
		&vault.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get vault by id: %w", err)
	}

	return &vault, nil
}

// GetByOrganizationID retrieves all non-deleted vaults for an organization
func (r *postgresRepository) GetByOrganizationID(ctx context.Context, orgID uuid.UUID) ([]*Vault, error) {
	query := `
		SELECT id, organization_id, name, description, encrypted_dek, created_at, updated_at, deleted_at
		FROM vaults
		WHERE organization_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vaults by organization id: %w", err)
	}
	defer rows.Close()

	var vaults []*Vault
	for rows.Next() {
		var vault Vault
		err := rows.Scan(
			&vault.ID,
			&vault.OrganizationID,
			&vault.Name,
			&vault.Description,
			&vault.EncryptedDEK,
			&vault.CreatedAt,
			&vault.UpdatedAt,
			&vault.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan vault: %w", err)
		}
		vaults = append(vaults, &vault)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating vaults: %w", err)
	}

	return vaults, nil
}

// Update modifies an existing vault in the database
func (r *postgresRepository) Update(ctx context.Context, vault *Vault) error {
	query := `
		UPDATE vaults
		SET name = $2, description = $3, encrypted_dek = $4, updated_at = $5
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		vault.ID,
		vault.Name,
		vault.Description,
		vault.EncryptedDEK,
		vault.UpdatedAt,
	).Scan(&vault.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return fmt.Errorf("failed to update vault: %w", err)
	}

	return nil
}

// Delete soft deletes a vault by setting its deleted_at timestamp
func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE vaults
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete vault: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
