package environments

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

// NewPostgresRepository creates a new PostgreSQL-backed environment repository
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{
		pool: pool,
	}
}

// Create inserts a new environment into the database
func (r *postgresRepository) Create(ctx context.Context, environment *Environment) error {
	query := `
		INSERT INTO environments (id, vault_id, name, description, created_at, updated_at)
		VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		environment.ID,
		environment.VaultID,
		environment.Name,
		environment.Description,
		environment.CreatedAt,
		environment.UpdatedAt,
	).Scan(&environment.ID, &environment.CreatedAt, &environment.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return fmt.Errorf("failed to create environment: %w", err)
	}

	return nil
}

// GetByID retrieves an environment by its ID
func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Environment, error) {
	query := `
		SELECT id, vault_id, name, description, created_at, updated_at, deleted_at
		FROM environments
		WHERE id = $1 AND deleted_at IS NULL
	`

	var environment Environment
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&environment.ID,
		&environment.VaultID,
		&environment.Name,
		&environment.Description,
		&environment.CreatedAt,
		&environment.UpdatedAt,
		&environment.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get environment by id: %w", err)
	}

	return &environment, nil
}

// ListByVaultID retrieves all non-deleted environments for a vault
func (r *postgresRepository) ListByVaultID(ctx context.Context, vaultID uuid.UUID) ([]*Environment, error) {
	query := `
		SELECT e.id, e.vault_id, e.name, e.description, e.created_at, e.updated_at, e.deleted_at,
			(SELECT COUNT(*) FROM secrets s WHERE s.environment_id = e.id AND s.deleted_at IS NULL)
		FROM environments e
		WHERE e.vault_id = $1 AND e.deleted_at IS NULL
		ORDER BY e.created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, vaultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environments by vault id: %w", err)
	}
	defer rows.Close()

	var environments []*Environment
	for rows.Next() {
		var environment Environment
		err := rows.Scan(
			&environment.ID,
			&environment.VaultID,
			&environment.Name,
			&environment.Description,
			&environment.CreatedAt,
			&environment.UpdatedAt,
			&environment.DeletedAt,
			&environment.SecretCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan environment: %w", err)
		}
		environments = append(environments, &environment)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating environments: %w", err)
	}

	return environments, nil
}

// Update modifies an existing environment in the database
func (r *postgresRepository) Update(ctx context.Context, environment *Environment) error {
	query := `
		UPDATE environments
		SET name = $2, description = $3, updated_at = $4
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		environment.ID,
		environment.Name,
		environment.Description,
		environment.UpdatedAt,
	).Scan(&environment.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return fmt.Errorf("failed to update environment: %w", err)
	}

	return nil
}

// Delete soft deletes an environment by setting its deleted_at timestamp
func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE environments
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
