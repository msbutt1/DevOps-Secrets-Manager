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
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	DeletedAt            *time.Time             `json:"deleted_at,omitempty"`
}

type Repository interface {
	Create(ctx context.Context, secret *Secret) error
	GetByID(ctx context.Context, secretID uuid.UUID) (*Secret, error)
	ListByEnvironmentID(ctx context.Context, envID uuid.UUID) ([]*Secret, error)
	Update(ctx context.Context, secret *Secret) error
	Delete(ctx context.Context, secretID uuid.UUID) error
}
