package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/audit"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/crypto"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/environments"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/vaults"
)

// SecretService defines the interface for secret operations
type SecretService interface {
	CreateSecret(ctx context.Context, envID uuid.UUID, keyName, plaintextValue string, description *string, rotationDays *int, expiresAt *time.Time, metadata map[string]interface{}, createdBy uuid.UUID) (*Secret, error)
	GetSecretMetadata(ctx context.Context, secretID uuid.UUID) (*Secret, error)
	ListSecretsByEnvironment(ctx context.Context, envID uuid.UUID) ([]*Secret, error)
	UpdateSecret(ctx context.Context, secretID uuid.UUID, newPlaintextValue *string, description *string, rotationDays *int, expiresAt *time.Time, metadata map[string]interface{}, updatedBy uuid.UUID) (*Secret, error)
	DeleteSecret(ctx context.Context, secretID, deletedBy uuid.UUID) error
	RevealSecret(ctx context.Context, secretID, requestedBy uuid.UUID) (string, error)
	// ExportEnvironment decrypts every secret in an environment. It records one env.exported
	// audit event for the given actor (a user, or a service token with uuid.Nil as user).
	ExportEnvironment(ctx context.Context, envID uuid.UUID, actor audit.Event) ([]KeyValue, error)
}

// KeyValue is a decrypted secret.
type KeyValue struct {
	Key   string
	Value string
}

type secretService struct {
	secretRepo   Repository
	envRepo      environments.Repository
	vaultRepo    vaults.Repository
	auditService audit.AuditService
	keyring      *crypto.Keyring
}

// NewSecretService creates a new secret service instance
func NewSecretService(
	secretRepo Repository,
	envRepo environments.Repository,
	vaultRepo vaults.Repository,
	auditService audit.AuditService,
	keyring *crypto.Keyring,
) SecretService {
	return &secretService{
		secretRepo:   secretRepo,
		envRepo:      envRepo,
		vaultRepo:    vaultRepo,
		auditService: auditService,
		keyring:      keyring,
	}
}

// CreateSecret creates a new secret with encrypted value
func (s *secretService) CreateSecret(
	ctx context.Context,
	envID uuid.UUID,
	keyName, plaintextValue string,
	description *string,
	rotationDays *int,
	expiresAt *time.Time,
	metadata map[string]interface{},
	createdBy uuid.UUID,
) (*Secret, error) {
	// Get environment
	env, err := s.envRepo.GetByID(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	// Get vault
	vault, err := s.vaultRepo.GetByID(ctx, env.VaultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}

	// Decrypt vault's DEK
	dek, err := s.keyring.UnwrapDEK(vault.EncryptedDEK, vault.KEKVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt vault DEK: %w", err)
	}

	// Encrypt the plaintext value
	ciphertext, nonce, err := s.encryptValue([]byte(plaintextValue), dek)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt value: %w", err)
	}

	// Create secret entity
	secret := &Secret{
		ID:                   uuid.New(),
		EnvironmentID:        envID,
		KeyName:              keyName,
		EncryptedValue:       ciphertext,
		Nonce:                nonce,
		Description:          description,
		RotationIntervalDays: rotationDays,
		ExpiresAt:            expiresAt,
		Metadata:             metadata,
		CreatedBy:            createdBy,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	// Persist to repository
	if err := s.secretRepo.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	// Audit failures are logged by the audit service and do not fail the operation
	s.record(ctx, audit.Event{
		UserID:         createdBy,
		Action:         audit.ActionSecretCreated,
		TargetType:     "secret",
		TargetID:       &secret.ID,
		TargetName:     keyName,
		OrganizationID: &vault.OrganizationID,
		VaultID:        &vault.ID,
		EnvironmentID:  &envID,
	})

	return secret, nil
}

// GetSecretMetadata retrieves secret metadata (without decrypted value)
func (s *secretService) GetSecretMetadata(ctx context.Context, secretID uuid.UUID) (*Secret, error) {
	secret, err := s.secretRepo.GetByID(ctx, secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	return secret, nil
}

// ListSecretsByEnvironment lists all secrets in an environment
func (s *secretService) ListSecretsByEnvironment(ctx context.Context, envID uuid.UUID) ([]*Secret, error) {
	secrets, err := s.secretRepo.ListByEnvironmentID(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	return secrets, nil
}

// UpdateSecret updates an existing secret with new encrypted value
func (s *secretService) UpdateSecret(
	ctx context.Context,
	secretID uuid.UUID,
	newPlaintextValue *string,
	description *string,
	rotationDays *int,
	expiresAt *time.Time,
	metadata map[string]interface{},
	updatedBy uuid.UUID,
) (*Secret, error) {
	// Get existing secret
	secret, err := s.secretRepo.GetByID(ctx, secretID)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	// Get environment
	env, err := s.envRepo.GetByID(ctx, secret.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	// Get vault
	vault, err := s.vaultRepo.GetByID(ctx, env.VaultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}

	// Re-encrypt only when a new value was supplied; otherwise keep the stored ciphertext
	if newPlaintextValue != nil {
		dek, err := s.keyring.UnwrapDEK(vault.EncryptedDEK, vault.KEKVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt vault DEK: %w", err)
		}

		ciphertext, nonce, err := s.encryptValue([]byte(*newPlaintextValue), dek)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt value: %w", err)
		}

		secret.EncryptedValue = ciphertext
		secret.Nonce = nonce
		now := time.Now()
		secret.LastRotatedAt = &now
	}

	// Update secret fields
	secret.Description = description
	secret.RotationIntervalDays = rotationDays
	secret.ExpiresAt = expiresAt
	secret.Metadata = metadata
	secret.UpdatedAt = time.Now()
	secret.UpdatedBy = &updatedBy

	// Persist updates
	if err := s.secretRepo.Update(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to update secret: %w", err)
	}

	// Audit failures are logged by the audit service and do not fail the operation
	s.record(ctx, audit.Event{
		UserID:         updatedBy,
		Action:         audit.ActionSecretUpdated,
		TargetType:     "secret",
		TargetID:       &secret.ID,
		TargetName:     secret.KeyName,
		OrganizationID: &vault.OrganizationID,
		VaultID:        &vault.ID,
		EnvironmentID:  &secret.EnvironmentID,
		Metadata:       map[string]interface{}{"value_changed": newPlaintextValue != nil},
	})

	return secret, nil
}

// DeleteSecret soft deletes a secret
func (s *secretService) DeleteSecret(ctx context.Context, secretID, deletedBy uuid.UUID) error {
	// Get secret for audit logging
	secret, err := s.secretRepo.GetByID(ctx, secretID)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Get environment for audit context
	env, err := s.envRepo.GetByID(ctx, secret.EnvironmentID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Get vault for audit context
	vault, err := s.vaultRepo.GetByID(ctx, env.VaultID)
	if err != nil {
		return fmt.Errorf("failed to get vault: %w", err)
	}

	// Delete the secret
	if err := s.secretRepo.Delete(ctx, secretID); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	// Audit failures are logged by the audit service and do not fail the operation
	s.record(ctx, audit.Event{
		UserID:         deletedBy,
		Action:         audit.ActionSecretDeleted,
		TargetType:     "secret",
		TargetID:       &secretID,
		TargetName:     secret.KeyName,
		OrganizationID: &vault.OrganizationID,
		VaultID:        &vault.ID,
		EnvironmentID:  &secret.EnvironmentID,
	})

	return nil
}

// RevealSecret decrypts and returns the plaintext value of a secret
func (s *secretService) RevealSecret(ctx context.Context, secretID, requestedBy uuid.UUID) (string, error) {
	// Get secret
	secret, err := s.secretRepo.GetByID(ctx, secretID)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	// Get environment
	env, err := s.envRepo.GetByID(ctx, secret.EnvironmentID)
	if err != nil {
		return "", fmt.Errorf("failed to get environment: %w", err)
	}

	// Get vault
	vault, err := s.vaultRepo.GetByID(ctx, env.VaultID)
	if err != nil {
		return "", fmt.Errorf("failed to get vault: %w", err)
	}

	// Decrypt vault's DEK
	dek, err := s.keyring.UnwrapDEK(vault.EncryptedDEK, vault.KEKVersion)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt vault DEK: %w", err)
	}

	// Decrypt the secret value
	plaintext, err := s.decryptValue(secret.EncryptedValue, secret.Nonce, dek)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret value: %w", err)
	}

	// Audit failures are logged by the audit service and do not fail the operation
	s.record(ctx, audit.Event{
		UserID:         requestedBy,
		Action:         audit.ActionSecretRevealed,
		TargetType:     "secret",
		TargetID:       &secretID,
		TargetName:     secret.KeyName,
		OrganizationID: &vault.OrganizationID,
		VaultID:        &vault.ID,
		EnvironmentID:  &secret.EnvironmentID,
	})

	return string(plaintext), nil
}

func (s *secretService) ExportEnvironment(ctx context.Context, envID uuid.UUID, actor audit.Event) ([]KeyValue, error) {
	env, err := s.envRepo.GetByID(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}
	vault, err := s.vaultRepo.GetByID(ctx, env.VaultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vault: %w", err)
	}
	dek, err := s.keyring.UnwrapDEK(vault.EncryptedDEK, vault.KEKVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt vault DEK: %w", err)
	}
	secrets, err := s.secretRepo.ListByEnvironmentID(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	values := make([]KeyValue, 0, len(secrets))
	keys := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		plaintext, err := s.decryptValue(secret.EncryptedValue, secret.Nonce, dek)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt secret %s: %w", secret.KeyName, err)
		}
		values = append(values, KeyValue{Key: secret.KeyName, Value: string(plaintext)})
		keys = append(keys, secret.KeyName)
	}

	actor.Action = audit.ActionEnvExported
	actor.TargetType = "environment"
	actor.TargetID = &env.ID
	actor.TargetName = env.Name
	actor.OrganizationID = &vault.OrganizationID
	actor.VaultID = &vault.ID
	actor.EnvironmentID = &env.ID
	if actor.Metadata == nil {
		actor.Metadata = map[string]interface{}{}
	}
	actor.Metadata["keys"] = keys
	s.record(ctx, actor)

	return values, nil
}

// encryptValue encrypts plaintext using AES-256-GCM with the provided DEK
func (s *secretService) encryptValue(plaintext, dek []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate 12-byte nonce
	nonce = make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt plaintext
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)

	return ciphertext, nonce, nil
}

// decryptValue decrypts ciphertext using AES-256-GCM with the provided DEK and nonce
func (s *secretService) decryptValue(ciphertext, nonce, dek []byte) (plaintext []byte, err error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt ciphertext
	plaintext, err = gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// record writes an audit event; the audit service logs failures itself.
func (s *secretService) record(ctx context.Context, event audit.Event) {
	_ = s.auditService.Record(ctx, event)
}
