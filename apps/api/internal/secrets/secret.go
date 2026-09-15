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
	// ErrVersionNotFound means the secret has no version with that number.
	ErrVersionNotFound = errors.New("secret version not found")
	// ErrSameEnvironment means a copy named one environment as both source and target.
	ErrSameEnvironment = errors.New("source and target environments are the same")
	// ErrDifferentVaults means a copy crossed vaults, which is not allowed.
	ErrDifferentVaults = errors.New("environments belong to different vaults")
	// ErrNoMatchingKeys means a copy matched no secrets in the source environment.
	ErrNoMatchingKeys = errors.New("no matching secrets in the source environment")
	// ErrVersionIsCurrent means a restore named the version that is already current.
	ErrVersionIsCurrent = errors.New("that version is already the current value")
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
	// Version is the number of the current value; it goes up each time the value changes.
	Version   int        `json:"version"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type Repository interface {
	// Create and Update store the secret only if the vault's wrapped data key still equals
	// wrappedDEK, the key the value was encrypted under; otherwise they return ErrKeyChanged.
	Create(ctx context.Context, secret *Secret, wrappedDEK []byte) error
	GetByID(ctx context.Context, secretID uuid.UUID) (*Secret, error)
	ListByEnvironmentID(ctx context.Context, envID uuid.UUID) ([]*Secret, error)
	// Update stores the secret; with valueChanged its version goes up and the new ciphertext is
	// kept as a version (restoredFrom names the version it was copied from, if any).
	Update(ctx context.Context, secret *Secret, wrappedDEK []byte, valueChanged bool, restoredFrom *int) error
	// ListVersions returns a secret's versions, newest first, without ciphertext.
	ListVersions(ctx context.Context, secretID uuid.UUID) ([]Version, error)
	// GetVersion returns one version including its ciphertext.
	GetVersion(ctx context.Context, secretID uuid.UUID, version int) (*Version, error)
	Delete(ctx context.Context, secretID uuid.UUID) error
}

// Version is one value a secret has had.
type Version struct {
	Version        int
	EncryptedValue []byte
	Nonce          []byte
	CreatedBy      *uuid.UUID
	CreatedByName  string
	CreatedAt      time.Time
	RestoredFrom   *int
}
