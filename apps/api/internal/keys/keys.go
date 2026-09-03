// Package keys implements master key maintenance: checking that every stored vault data key
// can be unwrapped with the configured keyring, and re-wrapping data keys after a rotation.
package keys

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/crypto"
)

// VersionCounts returns how many vaults (including soft-deleted ones, whose keys must also be
// re-wrapped before an old master key is retired) use each master key version.
func VersionCounts(ctx context.Context, pool *pgxpool.Pool) (map[int]int, error) {
	rows, err := pool.Query(ctx, `SELECT kek_version, COUNT(*) FROM vaults GROUP BY kek_version`)
	if err != nil {
		return nil, fmt.Errorf("count vaults by master key version: %w", err)
	}
	counts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (struct{ v, n int }, error) {
		var c struct{ v, n int }
		return c, row.Scan(&c.v, &c.n)
	})
	if err != nil {
		return nil, fmt.Errorf("count vaults by master key version: %w", err)
	}
	result := make(map[int]int, len(counts))
	for _, c := range counts {
		result[c.v] = c.n
	}
	return result, nil
}

// CheckKeyring fails when some vault's data key was wrapped with a version the keyring lacks,
// which would make those vaults unreadable.
func CheckKeyring(ctx context.Context, pool *pgxpool.Pool, keyring *crypto.Keyring) error {
	counts, err := VersionCounts(ctx, pool)
	if err != nil {
		return err
	}
	var missing []string
	for version, n := range counts {
		if !keyring.Has(version) {
			missing = append(missing, fmt.Sprintf("version %d (%d vaults)", version, n))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("%w: %s; add the old key to MASTER_KEK_PREVIOUS", crypto.ErrUnknownKEKVersion, strings.Join(missing, ", "))
	}
	return nil
}

// RotationResult summarises a RotateKEK run.
type RotationResult struct {
	Rewrapped int
	Current   int
}

// RotateKEK re-wraps every vault data key that is not yet under the current master key. Each
// vault is handled in its own short transaction holding the vault row lock, so the API can keep
// serving requests, and a run that stops part-way can simply be repeated. The data keys
// themselves do not change, so secret values are not re-encrypted.
func RotateKEK(ctx context.Context, pool *pgxpool.Pool, keyring *crypto.Keyring) (RotationResult, error) {
	result := RotationResult{Current: keyring.CurrentVersion()}

	rows, err := pool.Query(ctx, `SELECT id FROM vaults WHERE kek_version <> $1 ORDER BY id`, keyring.CurrentVersion())
	if err != nil {
		return result, fmt.Errorf("list vaults to rotate: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return result, fmt.Errorf("list vaults to rotate: %w", err)
	}

	for _, id := range ids {
		rewrapped, err := rewrapVault(ctx, pool, keyring, id)
		if err != nil {
			return result, fmt.Errorf("vault %s: %w", id, err)
		}
		if rewrapped {
			result.Rewrapped++
		}
	}
	return result, nil
}

func rewrapVault(ctx context.Context, pool *pgxpool.Pool, keyring *crypto.Keyring, id uuid.UUID) (bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	var wrapped []byte
	var version int
	err = tx.QueryRow(ctx, `SELECT encrypted_dek, kek_version FROM vaults WHERE id = $1 FOR UPDATE`, id).Scan(&wrapped, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // removed since listing
	}
	if err != nil {
		return false, err
	}
	if version == keyring.CurrentVersion() {
		return false, nil // done by a concurrent run
	}

	dek, err := keyring.UnwrapDEK(wrapped, version)
	if err != nil {
		return false, err
	}
	rewrapped, current, err := keyring.WrapDEK(dek)
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE vaults SET encrypted_dek = $2, kek_version = $3 WHERE id = $1`, id, rewrapped, current); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// ErrVaultNotFound is returned by RotateVaultDEK for a missing or deleted vault.
var ErrVaultNotFound = errors.New("vault not found")

// VaultRotation summarises a RotateVaultDEK run.
type VaultRotation struct {
	SecretsReencrypted int
}

// RotateVaultDEK gives a vault a new data key and re-encrypts all of its secret values,
// including deleted ones, in one transaction. The vault row is locked first, so secret writes
// (which take a share lock on it and check the data key) wait and then retry with the new key.
// The new data key is wrapped with the current master key.
func RotateVaultDEK(ctx context.Context, pool *pgxpool.Pool, keyring *crypto.Keyring, vaultID uuid.UUID) (VaultRotation, error) {
	var result VaultRotation
	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	var wrapped []byte
	var version int
	err = tx.QueryRow(ctx, `SELECT encrypted_dek, kek_version FROM vaults WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, vaultID).Scan(&wrapped, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, ErrVaultNotFound
	}
	if err != nil {
		return result, fmt.Errorf("lock vault: %w", err)
	}
	oldDEK, err := keyring.UnwrapDEK(wrapped, version)
	if err != nil {
		return result, err
	}
	newDEK, err := crypto.GenerateDEK()
	if err != nil {
		return result, err
	}

	rows, err := tx.Query(ctx, `
		SELECT s.id, s.key_name, s.encrypted_value, s.nonce
		FROM secrets s
		JOIN environments e ON e.id = s.environment_id
		WHERE e.vault_id = $1
		FOR UPDATE OF s
	`, vaultID)
	if err != nil {
		return result, fmt.Errorf("list secrets: %w", err)
	}
	type encrypted struct {
		id         uuid.UUID
		key        string
		ciphertext []byte
		nonce      []byte
	}
	secrets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (encrypted, error) {
		var e encrypted
		return e, row.Scan(&e.id, &e.key, &e.ciphertext, &e.nonce)
	})
	if err != nil {
		return result, fmt.Errorf("list secrets: %w", err)
	}

	for _, secret := range secrets {
		plaintext, err := crypto.DecryptValue(secret.ciphertext, secret.nonce, oldDEK)
		if err != nil {
			// Nothing has been written yet that the rollback would not undo
			return result, fmt.Errorf("decrypt %s: %w", secret.key, err)
		}
		ciphertext, nonce, err := crypto.EncryptValue(plaintext, newDEK)
		if err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `UPDATE secrets SET encrypted_value = $2, nonce = $3 WHERE id = $1`, secret.id, ciphertext, nonce); err != nil {
			return result, fmt.Errorf("update %s: %w", secret.key, err)
		}
	}

	// Earlier values are kept for rollback and must stay readable under the new key
	versionRows, err := tx.Query(ctx, `
		SELECT v.secret_id, v.version, v.encrypted_value, v.nonce
		FROM secret_versions v
		JOIN secrets s ON s.id = v.secret_id
		JOIN environments e ON e.id = s.environment_id
		WHERE e.vault_id = $1
		FOR UPDATE OF v
	`, vaultID)
	if err != nil {
		return result, fmt.Errorf("list secret versions: %w", err)
	}
	type encryptedVersion struct {
		secretID   uuid.UUID
		version    int
		ciphertext []byte
		nonce      []byte
	}
	versions, err := pgx.CollectRows(versionRows, func(row pgx.CollectableRow) (encryptedVersion, error) {
		var v encryptedVersion
		return v, row.Scan(&v.secretID, &v.version, &v.ciphertext, &v.nonce)
	})
	if err != nil {
		return result, fmt.Errorf("list secret versions: %w", err)
	}
	for _, v := range versions {
		plaintext, err := crypto.DecryptValue(v.ciphertext, v.nonce, oldDEK)
		if err != nil {
			return result, fmt.Errorf("decrypt version %d of secret %s: %w", v.version, v.secretID, err)
		}
		ciphertext, nonce, err := crypto.EncryptValue(plaintext, newDEK)
		if err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `UPDATE secret_versions SET encrypted_value = $3, nonce = $4 WHERE secret_id = $1 AND version = $2`, v.secretID, v.version, ciphertext, nonce); err != nil {
			return result, fmt.Errorf("update version %d of secret %s: %w", v.version, v.secretID, err)
		}
	}

	newWrapped, newVersion, err := keyring.WrapDEK(newDEK)
	if err != nil {
		return result, err
	}
	if _, err := tx.Exec(ctx, `UPDATE vaults SET encrypted_dek = $2, kek_version = $3 WHERE id = $1`, vaultID, newWrapped, newVersion); err != nil {
		return result, fmt.Errorf("update vault: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	result.SecretsReencrypted = len(secrets)
	return result, nil
}
