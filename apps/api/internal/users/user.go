package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Domain errors
var (
	ErrNotFound  = errors.New("user not found")
	ErrDuplicate = errors.New("user already exists")
)

// User represents a user entity in the system
type User struct {
	ID            uuid.UUID
	Email         string
	PasswordHash  string
	Name          string
	EmailVerified bool
	// LoginLockedUntil is set after repeated failed logins; logins are refused until then.
	LoginLockedUntil *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

// Repository defines the interface for user data access
type Repository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
	MarkEmailVerified(ctx context.Context, userID uuid.UUID) error
	UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	// RecordLoginFailure counts a failed login and, from the fifth consecutive failure, locks
	// logins for 1, 2, 4, ... minutes (at most 60). It returns the new lock time, if any.
	RecordLoginFailure(ctx context.Context, userID uuid.UUID) (attempts int, lockedUntil *time.Time, err error)
	// ResetLoginFailures clears the counter after a successful login.
	ResetLoginFailures(ctx context.Context, userID uuid.UUID) error
	// WithTx returns a repository bound to the transaction.
	WithTx(tx pgx.Tx) Repository
}
