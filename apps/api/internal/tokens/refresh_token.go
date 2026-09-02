package tokens

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Domain errors
var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenRevoked  = errors.New("token has been revoked")
	ErrTokenExpired  = errors.New("token has expired")
)

// RefreshToken represents a refresh token entity in the system
type RefreshToken struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	TokenHash   string
	TokenFamily uuid.UUID
	ExpiresAt   time.Time
	RevokedAt   *time.Time
	CreatedAt   time.Time
}

// RefreshTokenRepository defines the interface for refresh token data access
type RefreshTokenRepository interface {
	// RevokeAllByUserIDExceptFamily revokes the user's tokens outside one family and returns
	// how many sessions (families) were ended.
	RevokeAllByUserIDExceptFamily(ctx context.Context, userID, keepFamily uuid.UUID) (int64, error)
	Create(ctx context.Context, token *RefreshToken) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeByID(ctx context.Context, id uuid.UUID) error
	RevokeAllByUserID(ctx context.Context, userID uuid.UUID) error
	RevokeAllByTokenFamily(ctx context.Context, tokenFamily uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}
