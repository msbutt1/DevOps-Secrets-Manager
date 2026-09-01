package http_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func storedCiphertexts(t *testing.T, f *fixture) map[string][]byte {
	t.Helper()
	rows, err := f.api.Pool.Query(context.Background(), `
		SELECT s.id::text, s.encrypted_value FROM secrets s
		JOIN environments e ON e.id = s.environment_id WHERE e.vault_id = $1`, f.vaultID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][]byte{}
	for rows.Next() {
		var id string
		var value []byte
		if err := rows.Scan(&id, &value); err != nil {
			t.Fatal(err)
		}
		out[id] = value
	}
	return out
}

func TestVaultKeyRotation(t *testing.T) {
	f := newFixture(t) // DATABASE_URL in production
	staging := f.createEnv(t, f.vaultID, "staging")
	stagingSecret := f.createSecret(t, staging, "STAGING_KEY", "staging-value")
	deleted := f.createSecret(t, f.envID, "OLD_KEY", "deleted-value")
	f.api.MustDo(http.StatusNoContent, "DELETE", "/secrets/"+deleted, f.owner.Token, nil)

	var dekBefore []byte
	if err := f.api.Pool.QueryRow(context.Background(), `SELECT encrypted_dek FROM vaults WHERE id = $1`, f.vaultID).Scan(&dekBefore); err != nil {
		t.Fatal(err)
	}
	before := storedCiphertexts(t, f)

	var result struct {
		VaultID            string `json:"vault_id"`
		SecretsReencrypted int    `json:"secrets_reencrypted"`
	}
	f.api.MustDo(http.StatusOK, "POST", "/vaults/"+f.vaultID+"/rotate-key", f.owner.Token, nil).Decode(t, &result)
	if result.VaultID != f.vaultID || result.SecretsReencrypted != 3 {
		t.Fatalf("unexpected rotation result: %+v", result)
	}

	// Every stored value, deleted ones included, and the wrapped key changed.
	after := storedCiphertexts(t, f)
	for id, old := range before {
		if bytes.Equal(old, after[id]) {
			t.Errorf("secret %s was not re-encrypted", id)
		}
	}
	var dekAfter []byte
	_ = f.api.Pool.QueryRow(context.Background(), `SELECT encrypted_dek FROM vaults WHERE id = $1`, f.vaultID).Scan(&dekAfter)
	if bytes.Equal(dekBefore, dekAfter) {
		t.Fatal("vault data key did not change")
	}

	// Values are unchanged for readers, and writes work with the new key.
	if got := revealValue(t, f, f.secretID); got != "postgres://fixture" {
		t.Fatalf("value after rotation: %q", got)
	}
	if got := revealValue(t, f, stagingSecret); got != "staging-value" {
		t.Fatalf("staging value after rotation: %q", got)
	}
	f.api.MustDo(http.StatusOK, "PUT", "/secrets/"+f.secretID, f.owner.Token, map[string]any{"value": "postgres://after-rotation"})
	if got := revealValue(t, f, f.secretID); got != "postgres://after-rotation" {
		t.Fatalf("value written after rotation: %q", got)
	}

	var page struct {
		Data []struct {
			TargetName string         `json:"target_name"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"data"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/audit?action=vault.key_rotated", f.owner.Token, nil).Decode(t, &page)
	if len(page.Data) != 1 || page.Data[0].TargetName != "payments-api" || page.Data[0].Metadata["secrets_reencrypted"] != float64(3) {
		t.Fatalf("rotation not audited: %+v", page.Data)
	}

	// Negative cases: roles without manage permission, outsiders and deleted vaults.
	dev := f.member(t, "Dev Eloper", "developer", f.vaultID, "developer")
	f.api.MustDo(http.StatusForbidden, "POST", "/vaults/"+f.vaultID+"/rotate-key", dev.Token, nil)
	stranger := f.api.CreateUser("Stranger Danger")
	f.api.MustDo(http.StatusNotFound, "POST", "/vaults/"+f.vaultID+"/rotate-key", stranger.Token, nil)
	f.api.MustDo(http.StatusBadRequest, "POST", "/vaults/not-a-uuid/rotate-key", f.owner.Token, nil)
	gone := f.createVault(t, "gone")
	f.api.MustDo(http.StatusNoContent, "DELETE", "/vaults/"+gone, f.owner.Token, nil)
	f.api.MustDo(http.StatusNotFound, "POST", "/vaults/"+gone+"/rotate-key", f.owner.Token, nil)
}

// TestVaultKeyRotationDuringWrites rotates the key repeatedly while other requests create,
// update and reveal secrets, and checks that no write is lost or left encrypted under an old key.
func TestVaultKeyRotationDuringWrites(t *testing.T) {
	f := newFixture(t)
	const writers, rounds = 4, 15

	var wg sync.WaitGroup
	errs := make(chan error, writers*rounds*3)
	final := make([]string, writers)
	ids := make([]string, writers)
	for w := 0; w < writers; w++ {
		ids[w] = f.createSecret(t, f.envID, fmt.Sprintf("RACE_%d", w), "initial")
	}

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				value := fmt.Sprintf("writer-%d-round-%d", w, i)
				if s := f.api.Do("PUT", "/secrets/"+ids[w], f.owner.Token, map[string]any{"value": value}).Status; s != http.StatusOK {
					errs <- fmt.Errorf("update %d/%d: status %d", w, i, s)
					return
				}
				final[w] = value
				key := fmt.Sprintf("NEW_%d_%d", w, i)
				if s := f.api.Do("POST", "/envs/"+f.envID+"/secrets", f.owner.Token, map[string]any{"key_name": key, "value": value}).Status; s != http.StatusCreated {
					errs <- fmt.Errorf("create %s: status %d", key, s)
				}
				if s := f.api.Do("POST", "/secrets/"+ids[w]+"/reveal", f.owner.Token, nil).Status; s != http.StatusOK {
					errs <- fmt.Errorf("reveal %d/%d: status %d", w, i, s)
				}
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < rounds; i++ {
			if s := f.api.Do("POST", "/vaults/"+f.vaultID+"/rotate-key", f.owner.Token, nil).Status; s != http.StatusOK {
				errs <- fmt.Errorf("rotate %d: status %d", i, s)
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	for w := 0; w < writers; w++ {
		if got := revealValue(t, f, ids[w]); got != final[w] {
			t.Errorf("writer %d: revealed %q, last write was %q", w, got, final[w])
		}
	}
	// Every secret in the vault decrypts with the final key.
	var tok struct {
		Token string `json:"token"`
	}
	f.api.MustDo(http.StatusCreated, "POST", "/envs/"+f.envID+"/tokens", f.owner.Token, map[string]any{"name": "check"}).Decode(t, &tok)
	var exported struct {
		Secrets []struct {
			Key string `json:"key"`
		} `json:"secrets"`
	}
	f.api.MustDo(http.StatusOK, "GET", "/token/secrets", tok.Token, nil).Decode(t, &exported)
	if want := 1 + writers + writers*rounds; len(exported.Secrets) != want {
		t.Errorf("exported %d secrets, want %d", len(exported.Secrets), want)
	}
}
