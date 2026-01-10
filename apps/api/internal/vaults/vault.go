package vaults

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Domain errors
var (
	ErrNotFound  = errors.New("vault not found")
	ErrDuplicate = errors.New("vault already exists")
)

// Vault represents a vault entity in the system
type Vault struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Description    *string
	EncryptedDEK   []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

// Repository defines the interface for vault data access
type Repository interface {
	Create(ctx context.Context, vault *Vault) error
	GetByID(ctx context.Context, id uuid.UUID) (*Vault, error)
	GetByOrganizationID(ctx context.Context, orgID uuid.UUID) ([]*Vault, error)
	Update(ctx context.Context, vault *Vault) error
	Delete(ctx context.Context, id uuid.UUID) error
}
