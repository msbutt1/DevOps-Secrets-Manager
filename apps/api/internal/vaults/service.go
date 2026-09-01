package vaults

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/crypto"
)

// VaultService defines the interface for vault operations
type VaultService interface {
	CreateVault(ctx context.Context, orgID uuid.UUID, name string, description *string, createdBy uuid.UUID) (*Vault, error)
	GetVault(ctx context.Context, vaultID uuid.UUID) (*Vault, error)
	ListVaults(ctx context.Context, orgID uuid.UUID) ([]*Vault, error)
	UpdateVault(ctx context.Context, vaultID uuid.UUID, name string, description *string) (*Vault, error)
	DeleteVault(ctx context.Context, vaultID uuid.UUID) error
}

type vaultService struct {
	vaultRepo Repository
	keyring   *crypto.Keyring
}

// NewVaultService creates a new vault service
func NewVaultService(repo Repository, keyring *crypto.Keyring) VaultService {
	return &vaultService{
		vaultRepo: repo,
		keyring:   keyring,
	}
}

// CreateVault creates a new vault with a generated and encrypted DEK
func (s *vaultService) CreateVault(ctx context.Context, orgID uuid.UUID, name string, description *string, createdBy uuid.UUID) (*Vault, error) {
	// Generate 32-byte DEK
	dek, err := crypto.GenerateDEK()
	if err != nil {
		return nil, err
	}

	// Encrypt DEK with the current master key
	encryptedDEK, kekVersion, err := s.keyring.WrapDEK(dek)
	if err != nil {
		return nil, err
	}

	// Create vault struct
	vault := &Vault{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           name,
		Description:    description,
		EncryptedDEK:   encryptedDEK,
		KEKVersion:     kekVersion,
		CreatedBy:      &createdBy,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Store in repository
	if err := s.vaultRepo.Create(ctx, vault); err != nil {
		return nil, err
	}

	return vault, nil
}

// GetVault retrieves a vault by its ID
func (s *vaultService) GetVault(ctx context.Context, vaultID uuid.UUID) (*Vault, error) {
	vault, err := s.vaultRepo.GetByID(ctx, vaultID)
	if err != nil {
		return nil, err
	}

	return vault, nil
}

// ListVaults retrieves all vaults for an organization
func (s *vaultService) ListVaults(ctx context.Context, orgID uuid.UUID) ([]*Vault, error) {
	vaults, err := s.vaultRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return nil, err
	}

	return vaults, nil
}

// UpdateVault updates a vault's name and description
func (s *vaultService) UpdateVault(ctx context.Context, vaultID uuid.UUID, name string, description *string) (*Vault, error) {
	// Get existing vault
	vault, err := s.vaultRepo.GetByID(ctx, vaultID)
	if err != nil {
		return nil, err
	}

	// Update fields
	vault.Name = name
	vault.Description = description
	vault.UpdatedAt = time.Now()

	// Save to repository
	if err := s.vaultRepo.Update(ctx, vault); err != nil {
		return nil, err
	}

	return vault, nil
}

// DeleteVault soft deletes a vault
func (s *vaultService) DeleteVault(ctx context.Context, vaultID uuid.UUID) error {
	if err := s.vaultRepo.Delete(ctx, vaultID); err != nil {
		return err
	}

	return nil
}
