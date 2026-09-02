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

// Session is one login of a user: a refresh token family and where it was last used from.
type Session struct {
	ID         uuid.UUID
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
	IPAddress  *string
	UserAgent  *string
}

// RefreshTokenRepository defines the interface for refresh token data access
type RefreshTokenRepository interface {
	// TouchSession records that a session was created or used, from which address and client.
	TouchSession(ctx context.Context, sessionID, userID uuid.UUID, ipAddress, userAgent string) error
	// ListActiveSessions returns the user's sessions that still have a usable refresh token,
	// most recently used first.
	ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]Session, error)
	// RevokeSession revokes one of the user's sessions; false means it was not active.
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) (bool, error)
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
