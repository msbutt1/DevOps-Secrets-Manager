package secrets

import (
	"context"
	"errors"
	"fmt"
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
	// ListVersions lists a secret's earlier and current values without decrypting them.
	ListVersions(ctx context.Context, secretID uuid.UUID) ([]Version, error)
	// RevealVersion decrypts one version of a secret and audits it like a reveal.
	RevealVersion(ctx context.Context, secretID uuid.UUID, version int, requestedBy uuid.UUID) (string, error)
	// RestoreVersion makes an earlier value current again as a new version.
	RestoreVersion(ctx context.Context, secretID uuid.UUID, version int, restoredBy uuid.UUID) (*Secret, error)
}

// KeyValue is a decrypted secret.
type KeyValue struct {
	Key       string
	Value     string
	ExpiresAt *time.Time
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

	secret := &Secret{
		ID:                   uuid.New(),
		EnvironmentID:        envID,
		KeyName:              keyName,
		Description:          description,
		RotationIntervalDays: rotationDays,
		ExpiresAt:            expiresAt,
		Metadata:             metadata,
		CreatedBy:            createdBy,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	var vault *vaults.Vault
	err = retryOnKeyChange(func() error {
		if vault, err = s.vaultRepo.GetByID(ctx, env.VaultID); err != nil {
			return fmt.Errorf("failed to get vault: %w", err)
		}
		if secret.EncryptedValue, secret.Nonce, err = s.encrypt(vault, plaintextValue); err != nil {
			return err
		}
		return s.secretRepo.Create(ctx, secret, vault.EncryptedDEK)
	})
	if err != nil {
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
	var secret *Secret
	var vault *vaults.Vault
	err := retryOnKeyChange(func() error {
		// Read again on every attempt: a data key rotation also replaces the stored ciphertext
		var err error
		if secret, err = s.secretRepo.GetByID(ctx, secretID); err != nil {
			return fmt.Errorf("failed to get secret: %w", err)
		}
		env, err := s.envRepo.GetByID(ctx, secret.EnvironmentID)
		if err != nil {
			return fmt.Errorf("failed to get environment: %w", err)
		}
		if vault, err = s.vaultRepo.GetByID(ctx, env.VaultID); err != nil {
			return fmt.Errorf("failed to get vault: %w", err)
		}

		// Re-encrypt only when a new value was supplied; otherwise keep the stored ciphertext
		if newPlaintextValue != nil {
			if secret.EncryptedValue, secret.Nonce, err = s.encrypt(vault, *newPlaintextValue); err != nil {
				return err
			}
			now := time.Now()
			secret.LastRotatedAt = &now
		}

		secret.Description = description
		secret.RotationIntervalDays = rotationDays
		secret.ExpiresAt = expiresAt
		secret.Metadata = metadata
		secret.UpdatedAt = time.Now()
		secret.UpdatedBy = &updatedBy

		if err := s.secretRepo.Update(ctx, secret, vault.EncryptedDEK, newPlaintextValue != nil, nil); err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
		Metadata:       map[string]interface{}{"value_changed": newPlaintextValue != nil, "version": secret.Version},
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
	var secret *Secret
	var vault *vaults.Vault
	var plaintext []byte
	err := retryOnKeyChange(func() error {
		var err error
		if secret, err = s.secretRepo.GetByID(ctx, secretID); err != nil {
			return fmt.Errorf("failed to get secret: %w", err)
		}
		env, err := s.envRepo.GetByID(ctx, secret.EnvironmentID)
		if err != nil {
			return fmt.Errorf("failed to get environment: %w", err)
		}
		if vault, err = s.vaultRepo.GetByID(ctx, env.VaultID); err != nil {
			return fmt.Errorf("failed to get vault: %w", err)
		}
		plaintext, err = s.decrypt(vault, secret)
		return err
	})
	if err != nil {
		return "", err
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

	var vault *vaults.Vault
	var values []KeyValue
	var keys []string
	err = retryOnKeyChange(func() error {
		var err error
		if vault, err = s.vaultRepo.GetByID(ctx, env.VaultID); err != nil {
			return fmt.Errorf("failed to get vault: %w", err)
		}
		secrets, err := s.secretRepo.ListByEnvironmentID(ctx, envID)
		if err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}
		values = make([]KeyValue, 0, len(secrets))
		keys = make([]string, 0, len(secrets))
		for _, secret := range secrets {
			plaintext, err := s.decrypt(vault, secret)
			if err != nil {
				return err
			}
			values = append(values, KeyValue{Key: secret.KeyName, Value: string(plaintext), ExpiresAt: secret.ExpiresAt})
			keys = append(keys, secret.KeyName)
		}
		return nil
	})
	if err != nil {
		return nil, err
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

func (s *secretService) ListVersions(ctx context.Context, secretID uuid.UUID) ([]Version, error) {
	if _, err := s.secretRepo.GetByID(ctx, secretID); err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	return s.secretRepo.ListVersions(ctx, secretID)
}

func (s *secretService) RevealVersion(ctx context.Context, secretID uuid.UUID, version int, requestedBy uuid.UUID) (string, error) {
	var secret *Secret
	var vault *vaults.Vault
	var plaintext []byte
	err := retryOnKeyChange(func() error {
		var err error
		if secret, vault, err = s.secretWithVault(ctx, secretID); err != nil {
			return err
		}
		v, err := s.secretRepo.GetVersion(ctx, secretID, version)
		if err != nil {
			return err
		}
		plaintext, err = s.decrypt(vault, &Secret{KeyName: secret.KeyName, EncryptedValue: v.EncryptedValue, Nonce: v.Nonce})
		return err
	})
	if err != nil {
		return "", err
	}
	s.record(ctx, audit.Event{
		UserID:         requestedBy,
		Action:         audit.ActionSecretRevealed,
		TargetType:     "secret",
		TargetID:       &secretID,
		TargetName:     secret.KeyName,
		OrganizationID: &vault.OrganizationID,
		VaultID:        &vault.ID,
		EnvironmentID:  &secret.EnvironmentID,
		Metadata:       map[string]interface{}{"version": version},
	})
	return string(plaintext), nil
}

func (s *secretService) RestoreVersion(ctx context.Context, secretID uuid.UUID, version int, restoredBy uuid.UUID) (*Secret, error) {
	var secret *Secret
	var vault *vaults.Vault
	err := retryOnKeyChange(func() error {
		var err error
		if secret, vault, err = s.secretWithVault(ctx, secretID); err != nil {
			return err
		}
		if version == secret.Version {
			return ErrVersionIsCurrent
		}
		v, err := s.secretRepo.GetVersion(ctx, secretID, version)
		if err != nil {
			return err
		}
		// Versions are kept under the vault's current data key, so the ciphertext is reused as is
		now := time.Now()
		secret.EncryptedValue, secret.Nonce = v.EncryptedValue, v.Nonce
		secret.LastRotatedAt = &now
		secret.UpdatedAt = now
		secret.UpdatedBy = &restoredBy
		return s.secretRepo.Update(ctx, secret, vault.EncryptedDEK, true, &version)
	})
	if err != nil {
		return nil, err
	}
	s.record(ctx, audit.Event{
		UserID:         restoredBy,
		Action:         audit.ActionSecretRestored,
		TargetType:     "secret",
		TargetID:       &secretID,
		TargetName:     secret.KeyName,
		OrganizationID: &vault.OrganizationID,
		VaultID:        &vault.ID,
		EnvironmentID:  &secret.EnvironmentID,
		Metadata:       map[string]interface{}{"restored_version": version, "version": secret.Version},
	})
	return secret, nil
}

// secretWithVault reads a secret and the vault that owns it.
func (s *secretService) secretWithVault(ctx context.Context, secretID uuid.UUID) (*Secret, *vaults.Vault, error) {
	secret, err := s.secretRepo.GetByID(ctx, secretID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get secret: %w", err)
	}
	env, err := s.envRepo.GetByID(ctx, secret.EnvironmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get environment: %w", err)
	}
	vault, err := s.vaultRepo.GetByID(ctx, env.VaultID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get vault: %w", err)
	}
	return secret, vault, nil
}

// maxKeyAttempts bounds how often a write is retried when a data key rotation races with it.
const maxKeyAttempts = 3

// retryOnKeyChange runs fn again when it fails because the vault's data key changed.
func retryOnKeyChange(fn func() error) error {
	var err error
	for attempt := 0; attempt < maxKeyAttempts; attempt++ {
		if err = fn(); !errors.Is(err, ErrKeyChanged) {
			return err
		}
	}
	return err
}

// encrypt encrypts a value under the vault's data key.
func (s *secretService) encrypt(vault *vaults.Vault, plaintext string) (ciphertext, nonce []byte, err error) {
	dek, err := s.keyring.UnwrapDEK(vault.EncryptedDEK, vault.KEKVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt vault DEK: %w", err)
	}
	ciphertext, nonce, err = crypto.EncryptValue([]byte(plaintext), dek)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to encrypt value: %w", err)
	}
	return ciphertext, nonce, nil
}

// decrypt decrypts a secret's value. The vault and the secret are read in separate queries, so a
// data key rotation committing in between makes decryption fail; that is reported as
// ErrKeyChanged so the caller reads both again.
func (s *secretService) decrypt(vault *vaults.Vault, secret *Secret) ([]byte, error) {
	dek, err := s.keyring.UnwrapDEK(vault.EncryptedDEK, vault.KEKVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt vault DEK: %w", err)
	}
	plaintext, err := crypto.DecryptValue(secret.EncryptedValue, secret.Nonce, dek)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret %s: %w (%w)", secret.KeyName, err, ErrKeyChanged)
	}
	return plaintext, nil
}

// record writes an audit event; the audit service logs failures itself.
func (s *secretService) record(ctx context.Context, event audit.Event) {
	_ = s.auditService.Record(ctx, event)
}
