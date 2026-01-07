package tokens

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

// NewPostgresRepository creates a new PostgreSQL-backed refresh token repository
func NewPostgresRepository(pool *pgxpool.Pool) RefreshTokenRepository {
	return &postgresRepository{
		pool: pool,
	}
}

// Create inserts a new refresh token into the database
func (r *postgresRepository) Create(ctx context.Context, token *RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (id, user_id, token_hash, token_family, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.TokenFamily,
		token.ExpiresAt,
		token.CreatedAt,
	).Scan(&token.ID, &token.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}

	return nil
}

// GetByTokenHash retrieves a refresh token by its token hash
func (r *postgresRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, token_family, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`

	var token RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.TokenFamily,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("failed to get token by hash: %w", err)
	}

	return &token, nil
}

// RevokeByID revokes a refresh token by setting its revoked_at timestamp
func (r *postgresRepository) RevokeByID(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND revoked_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to revoke token by id: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrTokenNotFound
	}

	return nil
}

// RevokeAllByUserID revokes all refresh tokens for a specific user
func (r *postgresRepository) RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND revoked_at IS NULL
	`

	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke tokens by user id: %w", err)
	}

	return nil
}

// RevokeAllByTokenFamily revokes all refresh tokens in a specific token family
func (r *postgresRepository) RevokeAllByTokenFamily(ctx context.Context, tokenFamily uuid.UUID) error {
	query := `
		UPDATE refresh_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE token_family = $1 AND revoked_at IS NULL
	`

	_, err := r.pool.Exec(ctx, query, tokenFamily)
	if err != nil {
		return fmt.Errorf("failed to revoke tokens by family: %w", err)
	}

	return nil
}

// DeleteExpired permanently deletes expired refresh tokens from the database
func (r *postgresRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM refresh_tokens
		WHERE expires_at < CURRENT_TIMESTAMP
	`

	result, err := r.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	return result.RowsAffected(), nil
}
