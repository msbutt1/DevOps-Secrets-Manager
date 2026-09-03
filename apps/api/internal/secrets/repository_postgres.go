package secrets

import (
	"bytes"
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
func (r *postgresRepository) Create(ctx context.Context, secret *Secret, wrappedDEK []byte) error {
	query := `
		INSERT INTO secrets (
			id, environment_id, key_name, encrypted_value, nonce,
			description, rotation_interval_days, last_rotated_at, expires_at,
			metadata, created_by, updated_by, created_at, updated_at
		)
		VALUES (
			COALESCE($1, gen_random_uuid()), $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $11, $12, $13
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

	err = r.withVaultKey(ctx, secret.EnvironmentID, wrappedDEK, func(tx pgx.Tx) error {
		err := tx.QueryRow(
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
			return err
		}
		secret.Version = 1
		_, err = tx.Exec(ctx, `
			INSERT INTO secret_versions (secret_id, version, encrypted_value, nonce, created_by, created_at)
			VALUES ($1, 1, $2, $3, $4, $5)
		`, secret.ID, secret.EncryptedValue, secret.Nonce, secret.CreatedBy, secret.CreatedAt)
		return err
	})

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		if errors.Is(err, ErrKeyChanged) {
			return err
		}
		return fmt.Errorf("failed to create secret: %w", err)
	}

	return nil
}

// GetByID retrieves a secret by its ID
func (r *postgresRepository) GetByID(ctx context.Context, secretID uuid.UUID) (*Secret, error) {
	query := `
		SELECT
			s.id, s.environment_id, s.key_name, s.encrypted_value, s.nonce,
			s.description, s.rotation_interval_days, s.last_rotated_at, s.expires_at,
			s.metadata, s.created_by, s.updated_by, COALESCE(u.name, ''), s.created_at, s.updated_at, s.deleted_at, s.version
		FROM secrets s
		LEFT JOIN users u ON u.id = s.updated_by
		WHERE s.id = $1 AND s.deleted_at IS NULL
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
		&secret.UpdatedBy,
		&secret.UpdatedByName,
		&secret.CreatedAt,
		&secret.UpdatedAt,
		&secret.DeletedAt,
		&secret.Version,
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
			s.id, s.environment_id, s.key_name, s.encrypted_value, s.nonce,
			s.description, s.rotation_interval_days, s.last_rotated_at, s.expires_at,
			s.metadata, s.created_by, s.updated_by, COALESCE(u.name, ''), s.created_at, s.updated_at, s.deleted_at, s.version
		FROM secrets s
		LEFT JOIN users u ON u.id = s.updated_by
		WHERE s.environment_id = $1 AND s.deleted_at IS NULL
		ORDER BY s.created_at
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
			&secret.UpdatedBy,
			&secret.UpdatedByName,
			&secret.CreatedAt,
			&secret.UpdatedAt,
			&secret.DeletedAt,
			&secret.Version,
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
func (r *postgresRepository) Update(ctx context.Context, secret *Secret, wrappedDEK []byte, valueChanged bool, restoredFrom *int) error {
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
			updated_at = $9,
			updated_by = $10,
			version = version + CASE WHEN $11 THEN 1 ELSE 0 END
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING updated_at, version
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

	err = r.withVaultKey(ctx, secret.EnvironmentID, wrappedDEK, func(tx pgx.Tx) error {
		err := tx.QueryRow(
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
			secret.UpdatedBy,
			valueChanged,
		).Scan(&secret.UpdatedAt, &secret.Version)
		if err != nil || !valueChanged {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO secret_versions (secret_id, version, encrypted_value, nonce, created_by, created_at, restored_from)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, secret.ID, secret.Version, secret.EncryptedValue, secret.Nonce, secret.UpdatedBy, secret.UpdatedAt, restoredFrom)
		return err
	})

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicate
		}
		if errors.Is(err, ErrKeyChanged) {
			return err
		}
		return fmt.Errorf("failed to update secret: %w", err)
	}

	return nil
}

func (r *postgresRepository) ListVersions(ctx context.Context, secretID uuid.UUID) ([]Version, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT v.version, v.created_by, COALESCE(u.name, ''), v.created_at, v.restored_from
		FROM secret_versions v
		LEFT JOIN users u ON u.id = v.created_by
		WHERE v.secret_id = $1
		ORDER BY v.version DESC
	`, secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to list secret versions: %w", err)
	}
	versions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Version, error) {
		var v Version
		return v, row.Scan(&v.Version, &v.CreatedBy, &v.CreatedByName, &v.CreatedAt, &v.RestoredFrom)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list secret versions: %w", err)
	}
	return versions, nil
}

func (r *postgresRepository) GetVersion(ctx context.Context, secretID uuid.UUID, version int) (*Version, error) {
	v := Version{Version: version}
	err := r.pool.QueryRow(ctx, `
		SELECT v.encrypted_value, v.nonce, v.created_by, COALESCE(u.name, ''), v.created_at, v.restored_from
		FROM secret_versions v
		LEFT JOIN users u ON u.id = v.created_by
		WHERE v.secret_id = $1 AND v.version = $2
	`, secretID, version).Scan(&v.EncryptedValue, &v.Nonce, &v.CreatedBy, &v.CreatedByName, &v.CreatedAt, &v.RestoredFrom)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVersionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get secret version: %w", err)
	}
	return &v, nil
}

// withVaultKey runs fn in a transaction that holds a share lock on the environment's vault and
// has checked that the vault's wrapped data key is still wrappedDEK. Data key rotation takes
// the vault row lock exclusively, so a write either commits before the rotation starts (and is
// re-encrypted by it) or sees the new key afterwards and gets ErrKeyChanged.
func (r *postgresRepository) withVaultKey(ctx context.Context, envID uuid.UUID, wrappedDEK []byte, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var current []byte
	err = tx.QueryRow(ctx, `
		SELECT v.encrypted_dek
		FROM vaults v
		JOIN environments e ON e.vault_id = v.id
		WHERE e.id = $1
		FOR SHARE OF v
	`, envID).Scan(&current)
	if err != nil {
		return fmt.Errorf("failed to lock vault: %w", err)
	}
	if !bytes.Equal(current, wrappedDEK) {
		return ErrKeyChanged
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
