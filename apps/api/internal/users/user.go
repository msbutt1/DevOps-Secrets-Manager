package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
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
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
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
}
