package environments

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EnvironmentService defines the interface for environment operations
type EnvironmentService interface {
	CreateEnvironment(ctx context.Context, vaultID uuid.UUID, name string, description *string) (*Environment, error)
	GetEnvironment(ctx context.Context, envID uuid.UUID) (*Environment, error)
	ListEnvironmentsByVault(ctx context.Context, vaultID uuid.UUID) ([]*Environment, error)
	UpdateEnvironment(ctx context.Context, envID uuid.UUID, name string, description *string) (*Environment, error)
	DeleteEnvironment(ctx context.Context, envID uuid.UUID) error
}

type environmentService struct {
	environmentRepo Repository
}

// NewEnvironmentService creates a new environment service
func NewEnvironmentService(repo Repository) EnvironmentService {
	return &environmentService{
		environmentRepo: repo,
	}
}

// CreateEnvironment creates a new environment
func (s *environmentService) CreateEnvironment(ctx context.Context, vaultID uuid.UUID, name string, description *string) (*Environment, error) {
	environment := &Environment{
		ID:          uuid.New(),
		VaultID:     vaultID,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.environmentRepo.Create(ctx, environment); err != nil {
		return nil, err
	}

	return environment, nil
}

// GetEnvironment retrieves an environment by its ID
func (s *environmentService) GetEnvironment(ctx context.Context, envID uuid.UUID) (*Environment, error) {
	environment, err := s.environmentRepo.GetByID(ctx, envID)
	if err != nil {
		return nil, err
	}

	return environment, nil
}

// ListEnvironmentsByVault retrieves all environments for a vault
func (s *environmentService) ListEnvironmentsByVault(ctx context.Context, vaultID uuid.UUID) ([]*Environment, error) {
	environments, err := s.environmentRepo.ListByVaultID(ctx, vaultID)
	if err != nil {
		return nil, err
	}

	return environments, nil
}

// UpdateEnvironment updates an environment's name and description
func (s *environmentService) UpdateEnvironment(ctx context.Context, envID uuid.UUID, name string, description *string) (*Environment, error) {
	// Get existing environment
	environment, err := s.environmentRepo.GetByID(ctx, envID)
	if err != nil {
		return nil, err
	}

	// Update fields
	environment.Name = name
	environment.Description = description
	environment.UpdatedAt = time.Now()

	// Save to repository
	if err := s.environmentRepo.Update(ctx, environment); err != nil {
		return nil, err
	}

	return environment, nil
}

// DeleteEnvironment soft deletes an environment
func (s *environmentService) DeleteEnvironment(ctx context.Context, envID uuid.UUID) error {
	if err := s.environmentRepo.Delete(ctx, envID); err != nil {
		return err
	}

	return nil
}
