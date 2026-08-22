package email

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/storage"
)

// Domain errors
var (
	ErrTokenNotFound = errors.New("verification token not found")
	ErrTokenExpired  = errors.New("verification token has expired")
	ErrTokenUsed     = errors.New("verification token already used")
)

// VerificationToken represents an email verification token
type VerificationToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// VerificationTokenRepository defines the interface for verification token data access
type VerificationTokenRepository interface {
	Create(ctx context.Context, token *VerificationToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*VerificationToken, error)
	MarkAsUsed(ctx context.Context, id uuid.UUID) error
	// WithTx returns a repository bound to the transaction.
	WithTx(tx pgx.Tx) VerificationTokenRepository
}

type verificationTokenRepository struct {
	pool storage.Querier
}

// NewVerificationTokenRepository creates a new verification token repository
func NewVerificationTokenRepository(pool *pgxpool.Pool) VerificationTokenRepository {
	return &verificationTokenRepository{
		pool: pool,
	}
}

// WithTx returns a repository that runs its queries in the transaction.
func (r *verificationTokenRepository) WithTx(tx pgx.Tx) VerificationTokenRepository {
	return &verificationTokenRepository{pool: tx}
}

// Create inserts a new verification token into the database
func (r *verificationTokenRepository) Create(ctx context.Context, token *VerificationToken) error {
	query := `
		INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(
		ctx,
		query,
		token.ID,
		token.UserID,
		token.TokenHash,
		token.ExpiresAt,
		token.CreatedAt,
	).Scan(&token.ID, &token.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create verification token: %w", err)
	}

	return nil
}

// GetByTokenHash retrieves a verification token by its hash
func (r *verificationTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*VerificationToken, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM email_verification_tokens
		WHERE token_hash = $1
	`

	var token VerificationToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("failed to get verification token: %w", err)
	}

	return &token, nil
}

// MarkAsUsed marks a verification token as used
func (r *verificationTokenRepository) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE email_verification_tokens
		SET used_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark token as used: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrTokenNotFound
	}

	return nil
}
