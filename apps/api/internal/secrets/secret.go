package secrets

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound  = errors.New("secret not found")
	ErrDuplicate = errors.New("secret with this key already exists in the environment")
	// ErrKeyChanged means the vault's data key was rotated after the value was encrypted; the
	// caller should read the vault again and retry.
	ErrKeyChanged = errors.New("vault data key changed during the write")
)

type Secret struct {
	ID                   uuid.UUID              `json:"id"`
	EnvironmentID        uuid.UUID              `json:"environment_id"`
	KeyName              string                 `json:"key_name"`
	EncryptedValue       []byte                 `json:"-"`
	Nonce                []byte                 `json:"-"`
	Description          *string                `json:"description,omitempty"`
	RotationIntervalDays *int                   `json:"rotation_interval_days,omitempty"`
	LastRotatedAt        *time.Time             `json:"last_rotated_at,omitempty"`
	ExpiresAt            *time.Time             `json:"expires_at,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
	CreatedBy            uuid.UUID              `json:"created_by"`
	UpdatedBy            *uuid.UUID             `json:"updated_by,omitempty"`
	UpdatedByName        string                 `json:"-"` // joined from users on read
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	DeletedAt            *time.Time             `json:"deleted_at,omitempty"`
}

type Repository interface {
	// Create and Update store the secret only if the vault's wrapped data key still equals
	// wrappedDEK, the key the value was encrypted under; otherwise they return ErrKeyChanged.
	Create(ctx context.Context, secret *Secret, wrappedDEK []byte) error
	GetByID(ctx context.Context, secretID uuid.UUID) (*Secret, error)
	ListByEnvironmentID(ctx context.Context, envID uuid.UUID) ([]*Secret, error)
	Update(ctx context.Context, secret *Secret, wrappedDEK []byte) error
	Delete(ctx context.Context, secretID uuid.UUID) error
}
