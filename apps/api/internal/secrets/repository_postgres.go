package secrets

import (
	"context"
	"encoding/json"
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

// NewPostgresRepository creates a new PostgreSQL-backed secrets repository
func NewPostgresRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{
		pool: pool,
	}
}

// Create inserts a new secret into the database
func (r *postgresRepository) Create(ctx context.Context, secret *Secret) error {
	query := `
		INSERT INTO secrets (
			id, environment_id, key_name, encrypted_value, nonce,
			description, rotation_interval_days, last_rotated_at, expires_at,
			metadata, created_by, created_at, updated_at
		)
		VALUES (
			COALESCE($1, gen_random_uuid()), $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13
		)
		RETURNING id, created_at, updated_at
	`

	// Convert metadata map to JSONB
	var metadataJSON []byte
	var err error
	if secret.Metadata != nil {
		metadataJSON, err = json.Marshal(secret.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	err = r.pool.QueryRow(
		ctx,
		query,
		secret.ID,
		secret.EnvironmentID,
		secret.KeyName,
		secret.EncryptedValue,
		secret.Nonce,
		secret.Description,
		secret.RotationIntervalDays,
		secret.LastRotatedAt,
		secret.ExpiresAt,
		metadataJSON,
		secret.CreatedBy,
		secret.CreatedAt,
		secret.UpdatedAt,
	).Scan(&secret.ID, &secret.CreatedAt, &secret.UpdatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

// GetByID retrieves a secret by its ID
func (r *postgresRepository) GetByID(ctx context.Context, secretID uuid.UUID) (*Secret, error) {
	query := `
		SELECT
			id, environment_id, key_name, encrypted_value, nonce,
			description, rotation_interval_days, last_rotated_at, expires_at,
			metadata, created_by, created_at, updated_at, deleted_at
		FROM secrets
		WHERE id = $1 AND deleted_at IS NULL
	`

	var secret Secret
	var metadataJSON []byte

	err := r.pool.QueryRow(ctx, query, secretID).Scan(
		&secret.ID,
		&secret.EnvironmentID,
		&secret.KeyName,
		&secret.EncryptedValue,
		&secret.Nonce,
		&secret.Description,
		&secret.RotationIntervalDays,
		&secret.LastRotatedAt,
		&secret.ExpiresAt,
		&metadataJSON,
		&secret.CreatedBy,
		&secret.CreatedAt,
		&secret.UpdatedAt,
		&secret.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get secret by id: %w", err)
	}

	// Unmarshal JSONB metadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &secret.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &secret, nil
}

// ListByEnvironmentID retrieves all non-deleted secrets for an environment
func (r *postgresRepository) ListByEnvironmentID(ctx context.Context, envID uuid.UUID) ([]*Secret, error) {
	query := `
		SELECT
			id, environment_id, key_name, encrypted_value, nonce,
			description, rotation_interval_days, last_rotated_at, expires_at,
			metadata, created_by, created_at, updated_at, deleted_at
		FROM secrets
		WHERE environment_id = $1 AND deleted_at IS NULL
		ORDER BY created_at
	`

	rows, err := r.pool.Query(ctx, query, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get secrets by environment id: %w", err)
	}
	defer rows.Close()

	var secrets []*Secret
	for rows.Next() {
		var secret Secret
		var metadataJSON []byte

		err := rows.Scan(
			&secret.ID,
			&secret.EnvironmentID,
			&secret.KeyName,
			&secret.EncryptedValue,
			&secret.Nonce,
			&secret.Description,
			&secret.RotationIntervalDays,
			&secret.LastRotatedAt,
			&secret.ExpiresAt,
			&metadataJSON,
			&secret.CreatedBy,
			&secret.CreatedAt,
			&secret.UpdatedAt,
			&secret.DeletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan secret: %w", err)
		}

		// Unmarshal JSONB metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &secret.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		secrets = append(secrets, &secret)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating secrets: %w", err)
	}

	return secrets, nil
}

// Update modifies an existing secret in the database
func (r *postgresRepository) Update(ctx context.Context, secret *Secret) error {
	query := `
		UPDATE secrets
		SET
			encrypted_value = $2,
			nonce = $3,
			description = $4,
			rotation_interval_days = $5,
			last_rotated_at = $6,
			expires_at = $7,
			metadata = $8,
			updated_at = $9
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at
	`

	// Convert metadata map to JSONB
	var metadataJSON []byte
	var err error
	if secret.Metadata != nil {
		metadataJSON, err = json.Marshal(secret.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	err = r.pool.QueryRow(
		ctx,
		query,
		secret.ID,
		secret.EncryptedValue,
		secret.Nonce,
		secret.Description,
		secret.RotationIntervalDays,
		secret.LastRotatedAt,
		secret.ExpiresAt,
		metadataJSON,
		secret.UpdatedAt,
	).Scan(&secret.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		return fmt.Errorf("failed to update secret: %w", err)
	}

	return nil
}

// Delete soft deletes a secret by setting its deleted_at timestamp
func (r *postgresRepository) Delete(ctx context.Context, secretID uuid.UUID) error {
	query := `
		UPDATE secrets
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, secretID)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}
