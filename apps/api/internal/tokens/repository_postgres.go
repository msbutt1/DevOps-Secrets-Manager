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

func (r *postgresRepository) RevokeAllByUserIDExceptFamily(ctx context.Context, userID, keepFamily uuid.UUID) (int64, error) {
	var sessions int64
	err := r.pool.QueryRow(ctx, `
		WITH revoked AS (
			UPDATE refresh_tokens
			SET revoked_at = CURRENT_TIMESTAMP
			WHERE user_id = $1 AND token_family <> $2 AND revoked_at IS NULL
			RETURNING token_family, expires_at
		)
		SELECT COUNT(DISTINCT token_family) FROM revoked WHERE expires_at > CURRENT_TIMESTAMP
	`, userID, keepFamily).Scan(&sessions)
	if err != nil {
		return 0, fmt.Errorf("failed to revoke other sessions: %w", err)
	}
	return sessions, nil
}

func (r *postgresRepository) TouchSession(ctx context.Context, sessionID, userID uuid.UUID, ipAddress, userAgent string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_sessions (id, user_id, ip_address, user_agent)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''))
		ON CONFLICT (id) DO UPDATE SET
			last_used_at = CURRENT_TIMESTAMP,
			ip_address = COALESCE(EXCLUDED.ip_address, user_sessions.ip_address),
			user_agent = COALESCE(EXCLUDED.user_agent, user_sessions.user_agent)
		WHERE user_sessions.user_id = EXCLUDED.user_id
	`, sessionID, userID, ipAddress, userAgent)
	if err != nil {
		return fmt.Errorf("failed to record session: %w", err)
	}
	return nil
}

func (r *postgresRepository) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.created_at, s.last_used_at, MAX(t.expires_at), s.ip_address, s.user_agent
		FROM user_sessions s
		JOIN refresh_tokens t ON t.token_family = s.id AND t.user_id = s.user_id
		WHERE s.user_id = $1 AND t.revoked_at IS NULL AND t.expires_at > CURRENT_TIMESTAMP
		GROUP BY s.id
		ORDER BY s.last_used_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	sessions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Session, error) {
		var s Session
		return s, row.Scan(&s.ID, &s.CreatedAt, &s.LastUsedAt, &s.ExpiresAt, &s.IPAddress, &s.UserAgent)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	return sessions, nil
}

func (r *postgresRepository) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) (bool, error) {
	result, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND token_family = $2 AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP
	`, userID, sessionID)
	if err != nil {
		return false, fmt.Errorf("failed to revoke session: %w", err)
	}
	return result.RowsAffected() > 0, nil
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
