package environments

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Domain errors
var (
	ErrNotFound  = errors.New("environment not found")
	ErrDuplicate = errors.New("environment already exists")
)

// Environment represents an environment entity in the system
type Environment struct {
	ID          uuid.UUID
	VaultID     uuid.UUID
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// Repository defines the interface for environment data access
type Repository interface {
	Create(ctx context.Context, environment *Environment) error
	GetByID(ctx context.Context, id uuid.UUID) (*Environment, error)
	ListByVaultID(ctx context.Context, vaultID uuid.UUID) ([]*Environment, error)
	Update(ctx context.Context, environment *Environment) error
	Delete(ctx context.Context, id uuid.UUID) error
}
