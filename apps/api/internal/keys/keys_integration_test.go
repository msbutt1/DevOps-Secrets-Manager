package keys_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/crypto"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/keys"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/vaults"
)

type idResponse struct {
	ID string `json:"id"`
}

func createSecret(t *testing.T, api *apitest.Server, token, vaultName, value string) (vaultID, secretID string) {
	t.Helper()
	var vault, env, secret idResponse
	api.MustDo(http.StatusCreated, "POST", "/vaults", token, map[string]any{"name": vaultName}).Decode(t, &vault)
	api.MustDo(http.StatusCreated, "POST", "/vaults/"+vault.ID+"/envs", token, map[string]any{"name": "production"}).Decode(t, &env)
	api.MustDo(http.StatusCreated, "POST", "/envs/"+env.ID+"/secrets", token, map[string]any{"key_name": "API_KEY", "value": value}).Decode(t, &secret)
	return vault.ID, secret.ID
}

func reveal(t *testing.T, api *apitest.Server, token, secretID string) string {
	t.Helper()
	var body struct {
		Value string `json:"value"`
	}
	api.MustDo(http.StatusOK, "POST", "/secrets/"+secretID+"/reveal", token, nil).Decode(t, &body)
	return body.Value
}

func kekVersion(t *testing.T, api *apitest.Server, vaultID string) (int, []byte) {
	t.Helper()
	var version int
	var wrapped []byte
	if err := api.Pool.QueryRow(context.Background(), `SELECT kek_version, encrypted_dek FROM vaults WHERE id = $1`, vaultID).Scan(&version, &wrapped); err != nil {
		t.Fatal(err)
	}
	return version, wrapped
}

func TestMasterKeyRotation(t *testing.T) {
	ctx := context.Background()
	api := apitest.New(t)
	user := api.CreateUser("Key Keeper")

	vaultID, secretID := createSecret(t, api, user.Token, "payments", "value-under-key-1")
	deletedVault, _ := createSecret(t, api, user.Token, "retired", "deleted but still encrypted")
	api.MustDo(http.StatusNoContent, "DELETE", "/vaults/"+deletedVault, user.Token, nil)
	if v, _ := kekVersion(t, api, vaultID); v != 1 {
		t.Fatalf("new vault has kek_version %d, want 1", v)
	}

	// Step 1: deploy with a new current key, keeping the old one as a previous version.
	newKEK, _ := crypto.GenerateDEK()
	during, err := crypto.NewKeyring(2, newKEK, map[int][]byte{1: apitest.TestKEK})
	if err != nil {
		t.Fatal(err)
	}
	api2 := api.Restart(apitest.Options{Keyring: during})
	if got := reveal(t, api2, user.Token, secretID); got != "value-under-key-1" {
		t.Fatalf("old vault unreadable during rotation: %q", got)
	}
	newVault, newSecret := createSecret(t, api2, user.Token, "created-mid-rotation", "value-under-key-2")
	if v, _ := kekVersion(t, api2, newVault); v != 2 {
		t.Fatalf("vault created during rotation has kek_version %d, want 2", v)
	}

	// Without the old key, the check reports the vaults still on version 1.
	onlyNew, err := crypto.NewKeyring(2, newKEK, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := keys.CheckKeyring(ctx, api.Pool, onlyNew); !errors.Is(err, crypto.ErrUnknownKEKVersion) {
		t.Fatalf("CheckKeyring before rotation: %v", err)
	}

	// Step 2: re-wrap. A vault read before the rotation and saved afterwards must not undo it.
	repo := vaults.NewPostgresRepository(api.Pool)
	stale, err := repo.GetByID(ctx, uuid.MustParse(vaultID))
	if err != nil {
		t.Fatal(err)
	}
	_, beforeWrapped := kekVersion(t, api, vaultID)

	result, err := keys.RotateKEK(ctx, api.Pool, during)
	if err != nil || result.Rewrapped != 2 || result.Current != 2 {
		t.Fatalf("RotateKEK: %+v %v", result, err)
	}
	if again, err := keys.RotateKEK(ctx, api.Pool, during); err != nil || again.Rewrapped != 0 {
		t.Fatalf("second RotateKEK should be a no-op: %+v %v", again, err)
	}

	stale.Name = "payments-renamed"
	if err := repo.Update(ctx, stale); err != nil {
		t.Fatal(err)
	}
	version, afterWrapped := kekVersion(t, api, vaultID)
	if version != 2 || bytes.Equal(afterWrapped, beforeWrapped) {
		t.Fatalf("vault update reverted the rotation: version %d", version)
	}

	// Step 3: drop the old key. Everything is readable with the new key alone.
	if err := keys.CheckKeyring(ctx, api.Pool, onlyNew); err != nil {
		t.Fatalf("CheckKeyring after rotation: %v", err)
	}
	api3 := api.Restart(apitest.Options{Keyring: onlyNew})
	if got := reveal(t, api3, user.Token, secretID); got != "value-under-key-1" {
		t.Fatalf("rotated vault unreadable with the new key: %q", got)
	}
	if got := reveal(t, api3, user.Token, newSecret); got != "value-under-key-2" {
		t.Fatalf("vault created mid-rotation unreadable: %q", got)
	}

	// A server still configured with only the old key can no longer read the data.
	oldOnly := api.Restart(apitest.Options{})
	oldOnly.MustDo(http.StatusInternalServerError, "POST", "/secrets/"+secretID+"/reveal", user.Token, nil)
}
